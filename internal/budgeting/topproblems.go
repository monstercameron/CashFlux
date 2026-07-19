// SPDX-License-Identifier: MIT

package budgeting

import "sort"

// Problem is one budget flagged as needing attention: its evaluated Status plus
// whether a PACE projection (rather than the state itself) is what flagged it —
// i.e. the budget is trending over though it hasn't yet crossed the limit.
type Problem struct {
	Status   Status
	PaceRisk bool
}

// problemSeverity ranks how much a budget needs attention: over-budget is the
// most urgent, then near the limit, then merely trending over (pace risk). A
// healthy budget scores 0 and is never surfaced. paceRisk is the raw pace flag;
// the state always wins over it (an over/near budget isn't downgraded to "pace").
func problemSeverity(s Status, paceRisk bool) int {
	switch {
	case s.State == StateOver:
		return 3
	case s.State == StateNear:
		return 2
	case paceRisk:
		return 1
	}
	return 0
}

// TopProblems ranks the budgets that need attention worst-first and returns at
// most n of them, for a compact "Needs attention" strip. Ranking: over-budget
// first (by how far over in money, then by percent used), then near-limit or
// pace-risk budgets (by percent used). Healthy budgets are never returned.
//
// paceRisk is the set of budget IDs a pace projection flags as trending over
// though not yet over (a nil map is treated as empty). n <= 0 returns nil. The
// input order is preserved for equal-severity, equal-figure ties (stable sort),
// so a caller that pre-sorts by its own tiebreak keeps that order.
func TopProblems(statuses []Status, paceRisk map[string]bool, n int) []Problem {
	if n <= 0 {
		return nil
	}
	probs := make([]Problem, 0, len(statuses))
	for _, s := range statuses {
		raw := paceRisk[s.Budget.ID]
		if problemSeverity(s, raw) == 0 {
			continue
		}
		// PaceRisk marks a budget surfaced ONLY because of its pace — over/near
		// budgets are flagged by their state, not the pace projection.
		paceOnly := raw && s.State != StateOver && s.State != StateNear
		probs = append(probs, Problem{Status: s, PaceRisk: paceOnly})
	}
	overBy := func(s Status) int64 {
		if s.Remaining.IsNegative() {
			return -s.Remaining.Amount
		}
		return 0
	}
	sort.SliceStable(probs, func(i, j int) bool {
		si := problemSeverity(probs[i].Status, probs[i].PaceRisk)
		sj := problemSeverity(probs[j].Status, probs[j].PaceRisk)
		if si != sj {
			return si > sj
		}
		if si == 3 { // both over — the deepest overspend (in money) leads
			if oi, oj := overBy(probs[i].Status), overBy(probs[j].Status); oi != oj {
				return oi > oj
			}
		}
		return probs[i].Status.Percent > probs[j].Status.Percent
	})
	if len(probs) > n {
		probs = probs[:n]
	}
	return probs
}
