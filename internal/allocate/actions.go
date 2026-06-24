// SPDX-License-Identifier: MIT

package allocate

import "strings"

// ActionKind classifies what an Action does to its destination.
type ActionKind int

const (
	// GoalContribution adds money to a goal's CurrentAmount (earmark-only: no
	// cash moves between accounts; the goal balance is updated in place).
	GoalContribution ActionKind = iota
	// AccountEarmark tags an asset account with an earmarked amount.
	AccountEarmark
	// DebtPaydownEarmark tags a liability account with an earmarked paydown amount.
	DebtPaydownEarmark
)

// Action is one destination's concrete commitment derived from a Plan. The
// Amount is in minor currency units and is always positive (zero-amount plans
// are dropped by PlanActions).
type Action struct {
	Kind            ActionKind
	DestinationID   string // goal ID (prefix stripped) or account ID
	DestinationName string
	Amount          int64
}

// PlanActions converts a set of Plans into concrete Actions, using the
// "goal:" ID prefix to detect goal contributions and the caller-supplied
// isLiability classifier to distinguish debt paydown earmarks from asset
// earmarks. Plans with Amount ≤ 0 are dropped.
//
// INVARIANT: sum(Action.Amount) == sum(plan.Amount for plan.Amount > 0).
//
// The isLiability func receives the raw account ID (never a goal-prefixed ID);
// it is not called for goal candidates.
func PlanActions(plans []Plan, isLiability func(id string) bool) []Action {
	out := make([]Action, 0, len(plans))
	for _, p := range plans {
		if p.Amount <= 0 {
			continue
		}
		id := p.Candidate.ID
		name := p.Candidate.Name
		if goalID, ok := strings.CutPrefix(id, "goal:"); ok {
			out = append(out, Action{
				Kind:            GoalContribution,
				DestinationID:   goalID,
				DestinationName: name,
				Amount:          p.Amount,
			})
			continue
		}
		kind := AccountEarmark
		if isLiability != nil && isLiability(id) {
			kind = DebtPaydownEarmark
		}
		out = append(out, Action{
			Kind:            kind,
			DestinationID:   id,
			DestinationName: name,
			Amount:          p.Amount,
		})
	}
	return out
}
