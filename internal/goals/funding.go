// SPDX-License-Identifier: MIT

package goals

import (
	"math"
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// funding.go orders goals for payday funding (#65): an explicit per-goal
// FundingOrder (1 = first) set by the reorder control, falling back to the
// planning Priority (high → low) and then stored order for goals never
// explicitly ordered. The payday waterfall consumes this order, so reordering
// here directly changes which goal the next paycheck fills first.

// fundingRank returns a goal's sortable funding position: its explicit
// FundingOrder when set, else a sentinel past any real position.
func fundingRank(g domain.Goal) int {
	if g.FundingOrder > 0 {
		return g.FundingOrder
	}
	return math.MaxInt
}

// FundingOrdered returns a copy of gs sorted into payday-funding order:
// explicit FundingOrder first (ascending), then Priority (high first), then
// the incoming (stored) order. The input is never mutated.
func FundingOrdered(gs []domain.Goal) []domain.Goal {
	out := append([]domain.Goal(nil), gs...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := fundingRank(out[i]), fundingRank(out[j])
		if ri != rj {
			return ri < rj
		}
		return out[i].PriorityRank() < out[j].PriorityRank()
	})
	return out
}

// MoveFunding moves the goal with the given id one step up (delta -1) or down
// (delta +1) within the funding order of gs, and returns the complete id →
// FundingOrder renumbering (1..n over FundingOrdered(gs) after the move) so a
// caller can persist every affected goal. ok is false when the id is absent or
// the move would fall off either end (nothing to persist).
func MoveFunding(gs []domain.Goal, id string, delta int) (map[string]int, bool) {
	ordered := FundingOrdered(gs)
	idx := -1
	for i, g := range ordered {
		if g.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, false
	}
	to := idx + delta
	if to < 0 || to >= len(ordered) {
		return nil, false
	}
	ordered[idx], ordered[to] = ordered[to], ordered[idx]
	out := make(map[string]int, len(ordered))
	for i, g := range ordered {
		out[g.ID] = i + 1
	}
	return out, true
}
