// SPDX-License-Identifier: MIT

package goalfunded

import (
	"math/big"
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// BudgetContribution is a goal's already-recorded contribution to one budget,
// in non-negative minor units. It is used only as a deterministic split weight.
type BudgetContribution struct {
	GoalID      string
	BudgetID    string
	AmountMinor int64
}

// BudgetFunding is one goal's share of available funding assigned to a budget.
// AmountMinor is in the caller's chosen base currency. PurposeMismatch is true
// when the goal's linked category is outside this budget's tracked category
// tree; it lets a surface flag an unusual association without guessing at intent.
type BudgetFunding struct {
	GoalID          string
	GoalName        string
	BudgetID        string
	AmountMinor     int64
	PurposeMismatch bool
}

// AllocateAvailable assigns each goal's available amount to budgets once.
//
// The deterministic priority is: split across the goal's explicitly linked
// BudgetIDs (using observed contributions as weights, or equally if none); else
// assign to the lexicographically first budget tracking the goal's CategoryID;
// else split by positive observed goal contributions. A goal with no explicit
// association, category match, or observed contribution is left unassigned.
// Scope and owner must match exactly, so this function cannot move funding
// between household and individual budgets. Integer remainders go to budgets
// with the largest fractional remainder, ties by budget ID.
func AllocateAvailable(goals []domain.Goal, budgets []domain.Budget, categories []domain.Category,
	availableByGoal map[string]int64, contributions []BudgetContribution) []BudgetFunding {
	budgetByID := make(map[string]domain.Budget, len(budgets))
	for _, budget := range budgets {
		if strings.TrimSpace(budget.ID) != "" {
			budgetByID[budget.ID] = budget
		}
	}
	weights := make(map[string]map[string]int64)
	for _, contribution := range contributions {
		if contribution.AmountMinor <= 0 || contribution.GoalID == "" || contribution.BudgetID == "" {
			continue
		}
		byBudget := weights[contribution.GoalID]
		if byBudget == nil {
			byBudget = make(map[string]int64)
			weights[contribution.GoalID] = byBudget
		}
		// Saturate duplicate contributions rather than allowing overflow to
		// reverse the weight or make it negative.
		if contribution.AmountMinor > int64(^uint64(0)>>1)-byBudget[contribution.BudgetID] {
			byBudget[contribution.BudgetID] = int64(^uint64(0) >> 1)
		} else {
			byBudget[contribution.BudgetID] += contribution.AmountMinor
		}
	}

	var result []BudgetFunding
	for _, goal := range goals {
		available := availableByGoal[goal.ID]
		if goal.ID == "" || available <= 0 {
			continue
		}
		eligible := make([]domain.Budget, 0, len(budgets))
		for _, budget := range budgets {
			if budget.Scope == goal.Scope && budget.OwnerID == goal.OwnerID {
				eligible = append(eligible, budget)
			}
		}
		sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
		if len(eligible) == 0 {
			continue
		}

		eligibleIDs := make(map[string]bool, len(eligible))
		for _, budget := range eligible {
			eligibleIDs[budget.ID] = true
		}
		selected := make([]string, 0)
		for _, id := range goal.BudgetIDs {
			if eligibleIDs[id] && !containsID(selected, id) {
				selected = append(selected, id)
			}
		}
		if len(selected) == 0 && strings.TrimSpace(goal.CategoryID) != "" {
			for _, budget := range eligible {
				if tracksPurpose(budget, goal.CategoryID, categories) {
					selected = append(selected, budget.ID)
				}
			}
			if len(selected) > 0 {
				selected = selected[:1]
			}
		}
		if len(selected) == 0 {
			for _, budget := range eligible {
				if weights[goal.ID][budget.ID] > 0 {
					selected = append(selected, budget.ID)
				}
			}
		}
		if len(selected) == 0 {
			continue
		}

		selectedWeights := make(map[string]int64, len(selected))
		anyWeight := false
		for _, id := range selected {
			selectedWeights[id] = weights[goal.ID][id]
			anyWeight = anyWeight || selectedWeights[id] > 0
		}
		if !anyWeight {
			for _, id := range selected {
				selectedWeights[id] = 1
			}
		}
		for id, amount := range splitMinor(available, selectedWeights) {
			if amount == 0 {
				continue
			}
			budget := budgetByID[id]
			result = append(result, BudgetFunding{
				GoalID: goal.ID, GoalName: strings.TrimSpace(goal.Name), BudgetID: id,
				AmountMinor: amount, PurposeMismatch: goal.CategoryID != "" && !tracksPurpose(budget, goal.CategoryID, categories),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BudgetID != result[j].BudgetID {
			return result[i].BudgetID < result[j].BudgetID
		}
		return result[i].GoalID < result[j].GoalID
	})
	return result
}

func containsID(ids []string, wanted string) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func tracksPurpose(budget domain.Budget, categoryID string, categories []domain.Category) bool {
	return categorytree.DescendantsOfAll(categories, budget.TrackedCategoryIDs())[categoryID]
}

type fractionalPart struct {
	id        string
	remainder *big.Int
}

// splitMinor performs exact integer proportional allocation without float
// arithmetic. The map iteration is normalized before remainder assignment.
func splitMinor(amount int64, weights map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(weights))
	ids := make([]string, 0, len(weights))
	total := new(big.Int)
	for id, weight := range weights {
		if weight <= 0 {
			continue
		}
		ids = append(ids, id)
		total.Add(total, big.NewInt(weight))
	}
	if len(ids) == 0 || amount <= 0 {
		return result
	}
	sort.Strings(ids)
	amountBig := big.NewInt(amount)
	parts := make([]fractionalPart, 0, len(ids))
	var assigned int64
	for _, id := range ids {
		product := new(big.Int).Mul(amountBig, big.NewInt(weights[id]))
		quotient, remainder := new(big.Int), new(big.Int)
		quotient.QuoRem(product, total, remainder)
		value := quotient.Int64()
		result[id] = value
		assigned += value
		parts = append(parts, fractionalPart{id: id, remainder: remainder})
	}
	sort.Slice(parts, func(i, j int) bool {
		if cmp := parts[i].remainder.Cmp(parts[j].remainder); cmp != 0 {
			return cmp > 0
		}
		return parts[i].id < parts[j].id
	})
	for i := int64(0); i < amount-assigned; i++ {
		result[parts[i%int64(len(parts))].id]++
	}
	return result
}
