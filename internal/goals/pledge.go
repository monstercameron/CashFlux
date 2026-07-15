// SPDX-License-Identifier: MIT

package goals

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// PledgeStanding is one member's position on a shared goal (GL5): what they
// pledged per month, what the household expects them to have contributed by now,
// and what they actually have. Delta is Actual − Expected (positive = ahead), and
// AheadMonths expresses that gap in whole pledged-months so the readout can say
// "2 months ahead of pledge". It is attribution, NEVER blame — the copy layer
// renders even a negative delta neutrally ("on pace", "a bit behind").
type PledgeStanding struct {
	MemberID       string
	Pledged        money.Money // pledged monthly amount
	ExpectedToDate money.Money
	Actual         money.Money
	Delta          money.Money // Actual − Expected (may be negative)
	// AheadMonths is Delta divided by the monthly pledge, truncated toward zero: a
	// positive value reads as months ahead, negative as months behind, zero as on
	// pace. Zero when the member has no positive pledge to divide by.
	AheadMonths int
}

// Pace classifies the standing into a small, non-judgmental set for the split-bar
// readout. The tolerance keeps a member who is within a fraction of a month of
// their pledge reading as simply "on pace".
func (s PledgeStanding) Pace() PledgePace {
	if s.Pledged.Amount <= 0 {
		return PledgePaceNone
	}
	// On pace when actual is within one month's pledge of expected, either way.
	tol := s.Pledged.Amount
	switch {
	case s.Delta.Amount > tol:
		return PledgePaceAhead
	case s.Delta.Amount < -tol:
		return PledgePaceBehind
	default:
		return PledgePaceOnPace
	}
}

// PledgePace is a glanceable, blame-free classification of a member's standing.
type PledgePace string

const (
	// PledgePaceNone applies when the member made no positive pledge.
	PledgePaceNone PledgePace = ""
	// PledgePaceAhead means the member is comfortably ahead of their pledge.
	PledgePaceAhead PledgePace = "ahead"
	// PledgePaceOnPace means the member is within a month of their pledge.
	PledgePaceOnPace PledgePace = "onpace"
	// PledgePaceBehind means the member has contributed less than pledged so far.
	// The UI renders this gently — an FYI, never a scold.
	PledgePaceBehind PledgePace = "behind"
)

// PledgeReadout is the whole shared-goal fairness picture: one standing per
// pledging member plus household totals, ready for a BG13-style split bar.
type PledgeReadout struct {
	Standings     []PledgeStanding
	TotalPledged  money.Money // sum of monthly pledges
	TotalActual   money.Money // sum of attributed actual contributions
	TotalExpected money.Money // sum of expected-to-date
	Currency      string
	MonthsElapsed int
}

// MonthsElapsed returns whole months between since and now (>= 0), the horizon
// pledges are measured over. A zero or future `since` yields 0.
func MonthsElapsed(since, now time.Time) int {
	if since.IsZero() || !since.Before(now) {
		return 0
	}
	m := (now.Year()-since.Year())*12 + int(now.Month()) - int(since.Month())
	if now.Day() < since.Day() {
		m-- // not a whole month yet
	}
	if m < 0 {
		return 0
	}
	return m
}

// BuildPledgeReadout computes each member's pledge standing on a shared goal. It
// is pure and deterministic. Pledges come from goal.Pledges; actual contribution
// amounts are summed per member from goal.Contributions using each entry's
// MemberID (a contribution with an empty MemberID is attributed to
// fallbackMember — the contributing member's context the caller supplies — or
// dropped into the unassigned bucket when fallbackMember is empty). Expected-to-
// date is the monthly pledge times monthsElapsed. All amounts must share the
// goal's target currency (the pledge UI stores them in it); a mismatch is
// ignored (the entry is skipped) rather than erroring, so one bad row can't blank
// the whole readout.
//
// Standings are sorted largest pledge first, then member id, for a stable order.
// Members who neither pledged nor contributed are omitted. monthsElapsed is
// typically MonthsElapsed(goal-start, now); the caller owns the start reference.
func BuildPledgeReadout(goal domain.Goal, fallbackMember string, monthsElapsed int) PledgeReadout {
	cur := goal.TargetAmount.Currency
	if monthsElapsed < 0 {
		monthsElapsed = 0
	}

	pledged := map[string]int64{}
	for m, p := range goal.Pledges {
		if p.Currency != "" && p.Currency != cur {
			continue
		}
		if p.Amount > 0 {
			pledged[m] += p.Amount
		}
	}

	actual := map[string]int64{}
	for _, c := range goal.Contributions {
		if c.Amount.Currency != "" && c.Amount.Currency != cur {
			continue
		}
		member := c.MemberID
		if member == "" {
			member = fallbackMember
		}
		actual[member] += c.Amount.Amount
	}

	// Union of members who pledged or contributed.
	seen := map[string]bool{}
	var members []string
	for m := range pledged {
		if !seen[m] {
			seen[m] = true
			members = append(members, m)
		}
	}
	for m := range actual {
		if !seen[m] {
			seen[m] = true
			members = append(members, m)
		}
	}

	var totPledged, totActual, totExpected int64
	standings := make([]PledgeStanding, 0, len(members))
	for _, m := range members {
		p := pledged[m]
		a := actual[m]
		exp := p * int64(monthsElapsed)
		delta := a - exp
		ahead := 0
		if p > 0 {
			ahead = int(delta / p)
		}
		standings = append(standings, PledgeStanding{
			MemberID:       m,
			Pledged:        money.New(p, cur),
			ExpectedToDate: money.New(exp, cur),
			Actual:         money.New(a, cur),
			Delta:          money.New(delta, cur),
			AheadMonths:    ahead,
		})
		totPledged += p
		totActual += a
		totExpected += exp
	}

	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].Pledged.Amount != standings[j].Pledged.Amount {
			return standings[i].Pledged.Amount > standings[j].Pledged.Amount
		}
		return standings[i].MemberID < standings[j].MemberID
	})

	return PledgeReadout{
		Standings:     standings,
		TotalPledged:  money.New(totPledged, cur),
		TotalActual:   money.New(totActual, cur),
		TotalExpected: money.New(totExpected, cur),
		Currency:      cur,
		MonthsElapsed: monthsElapsed,
	}
}

// IsShared reports whether the goal has any positive per-member pledge — the
// signal that its card should render the pledge split bar.
func IsShared(goal domain.Goal) bool {
	for _, p := range goal.Pledges {
		if p.Amount > 0 {
			return true
		}
	}
	return false
}

// PledgeStartFrom picks the reference date pledges are measured from for a goal:
// the earliest recorded contribution when there is one, else `created` (the
// caller's best available creation anchor, e.g. LastReviewedAt or now). It keeps
// the "months elapsed" honest instead of assuming pledges started today.
func PledgeStartFrom(goal domain.Goal, created time.Time) time.Time {
	earliest := time.Time{}
	for _, c := range goal.Contributions {
		if c.At.IsZero() {
			continue
		}
		if earliest.IsZero() || c.At.Before(earliest) {
			earliest = c.At
		}
	}
	if !earliest.IsZero() {
		return earliest
	}
	return created
}
