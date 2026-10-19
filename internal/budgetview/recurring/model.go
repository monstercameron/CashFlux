// SPDX-License-Identifier: MIT

package recurring

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Input is the immutable household snapshot and the date context of the tile.
type Input struct {
	Recurring              []domain.Recurring
	Budgets                []domain.Budget
	Categories             []domain.Category
	Accounts               []domain.Account
	BaseCurrency, MemberID string
	Rates                  map[string]float64
	Now, Start, End        time.Time
}

// Category is an unbudgeted category's total and the seed for Budget this.
type Category struct {
	Amount                   string
	CategoryID, CategoryName string
	Monthly                  money.Money
	LimitMajor               string
	CanBudget                bool
}

// Model is a checked, scoped summary. A missing FX rate excludes the named bill;
// an overflowing aggregate fails the entire summary rather than showing a part.
type Model struct {
	Base                                            string
	Covered                                         []domain.Recurring
	Categories                                      map[string]string
	Unbudgeted                                      []Category
	Excluded                                        []string
	CoveredMonthly, UnbudgetedMonthly, TotalMonthly money.Money
	Count                                           int
	Visible                                         bool
}

// Coverage includes descendants of each tracked category in the member's budgets.
func Coverage(budgets []domain.Budget, categories []domain.Category, memberID string) map[string]bool {
	roots := make([]string, 0)
	for _, budget := range budgets {
		if memberID != "" && budget.Scope == domain.ScopeIndividual && budget.OwnerID != memberID {
			continue
		}
		roots = append(roots, budget.TrackedCategoryIDs()...)
	}
	return categorytree.DescendantsOfAll(categories, roots)
}

// InMemberLens retains shared, group-owned and accountless bills in member views.
// A linked account that cannot be found is hidden in a member view.
func InMemberLens(r domain.Recurring, memberID string, accounts []domain.Account) bool {
	if memberID == "" || r.AccountID == "" {
		return true
	}
	for _, account := range accounts {
		if account.ID == r.AccountID {
			return account.Scope != domain.ScopeIndividual || account.OwnerID == memberID || account.OwnerID == domain.GroupOwnerID
		}
	}
	return false
}

// ExcludedLabel trims a bill's label, falling back to the caller's localized text.
func ExcludedLabel(r domain.Recurring, unnamed string) string {
	if label := strings.TrimSpace(r.Label); label != "" {
		return label
	}
	return unnamed
}

// Build filters active outflows, sorts stably by native monthly magnitude, then
// partitions them by budget coverage and converts each bill before aggregation.
// Equal category totals use category ID as a deterministic tie-breaker.
func Build(in Input, uncategorized, unnamed string) (Model, error) {
	base := in.BaseCurrency
	if base == "" {
		base = "USD"
	}
	out := Model{Base: base, Categories: make(map[string]string, len(in.Categories))}
	for _, c := range in.Categories {
		out.Categories[c.ID] = c.Name
	}
	recs := make([]domain.Recurring, 0, len(in.Recurring))
	for _, r := range in.Recurring {
		if r.Active() && r.Amount.IsNegative() && InMemberLens(r, in.MemberID, in.Accounts) {
			recs = append(recs, r)
		}
	}
	sort.SliceStable(recs, func(i, j int) bool { return monthlyMagnitude(recs[i]) > monthlyMagnitude(recs[j]) })
	out.Visible = len(recs) > 0
	covered := Coverage(in.Budgets, in.Categories, in.MemberID)
	byCategory := make(map[string]int64)
	var coveredTotal, unbudgetedTotal int64
	rates := currency.Rates{Base: base, Rates: in.Rates}
	for _, r := range recs {
		amount, err := MonthlyBase(r, base, rates)
		if err != nil {
			out.Excluded = append(out.Excluded, ExcludedLabel(r, unnamed))
			continue
		}
		out.Count++
		if covered[r.CategoryID] {
			out.Covered = append(out.Covered, r)
			coveredTotal, err = money.Add(coveredTotal, amount)
		} else {
			byCategory[r.CategoryID], err = money.Add(byCategory[r.CategoryID], amount)
			if err == nil {
				unbudgetedTotal, err = money.Add(unbudgetedTotal, amount)
			}
		}
		if err != nil {
			return Model{}, fmt.Errorf("recurring section total: %w", err)
		}
	}
	total, err := money.Add(coveredTotal, unbudgetedTotal)
	if err != nil {
		return Model{}, fmt.Errorf("recurring total: %w", err)
	}
	out.CoveredMonthly = money.New(coveredTotal, base)
	out.UnbudgetedMonthly = money.New(unbudgetedTotal, base)
	out.TotalMonthly = money.New(total, base)
	out.Unbudgeted = categoryRows(byCategory, out.Categories, base, uncategorized)
	sort.Strings(out.Excluded)
	return out, nil
}

func categoryRows(amounts map[string]int64, names map[string]string, base, fallback string) []Category {
	ids := make([]string, 0, len(amounts))
	for id := range amounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if amounts[ids[i]] == amounts[ids[j]] {
			return ids[i] < ids[j]
		}
		return amounts[ids[i]] > amounts[ids[j]]
	})
	rows := make([]Category, 0, len(ids))
	for _, id := range ids {
		name := names[id]
		if name == "" {
			name = fallback
		}
		rows = append(rows, Category{CategoryID: id, CategoryName: name,
			Monthly: money.New(amounts[id], base), Amount: formatMoney(money.New(amounts[id], base)), LimitMajor: money.FormatMinor(amounts[id], currency.Decimals(base)), CanBudget: id != ""})
	}
	return rows
}
