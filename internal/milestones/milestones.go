// SPDX-License-Identifier: MIT

// Package milestones detects positive financial milestones worth celebrating —
// the counterweight to a notification feed that is otherwise all warnings. It is
// pure Go (no syscall/js), deterministic, and table-tested; the UI layer supplies
// the already-computed primitives (reached goals, net worth, a no-spend streak,
// budgets kept last month) and renders the localized copy. Detection stays here so
// the thresholds and dedupe keys are proven once.
package milestones

import (
	"strconv"
	"time"
)

// Kind classifies a milestone so the UI can pick its icon and copy.
type Kind string

const (
	KindGoalReached   Kind = "goal"     // a savings goal fully funded
	KindNetWorth      Kind = "networth" // net worth crossed a round-number rung
	KindNoSpendStreak Kind = "nospend"  // a run of days with no spending
	KindKeptBudgets   Kind = "kept"     // budgets finished last month within limit
)

// Milestone is one thing going right. Key is a stable identity used by the UI to
// remember which wins it has already celebrated (so the confetti fires once, not
// on every visit).
type Milestone struct {
	Key   string
	Kind  Kind
	Name  string // goal name (goal milestones); empty otherwise
	Value int64  // net-worth rung (minor units), streak days, or kept-budget count
}

// Input is the pre-computed snapshot Detect reasons over.
type Input struct {
	ReachedGoals  []string  // names of goals whose target is fully covered
	NetWorthMinor int64     // current net worth, base-currency minor units
	NoSpendDays   int       // current consecutive days with no expense
	KeptBudgets   int       // budgets kept at/under limit for the last completed period
	KeptPeriodKey string    // e.g. "2026-06" — dedupes the kept-budgets win per period
	Now           time.Time // reserved for future time-based milestones
}

// noSpendMinDays is the shortest no-spend run worth celebrating — below this it's
// noise, not a streak.
const noSpendMinDays = 3

// netWorthLadder are the round-number net-worth rungs worth a nod, in base-currency
// minor units ($10k, $25k, $50k, $100k, $250k, $500k, $1M, $2M, $5M). Ascending.
var netWorthLadder = []int64{
	1_000_000,   // $10,000
	2_500_000,   // $25,000
	5_000_000,   // $50,000
	10_000_000,  // $100,000
	25_000_000,  // $250,000
	50_000_000,  // $500,000
	100_000_000, // $1,000,000
	200_000_000, // $2,000,000
	500_000_000, // $5,000,000
}

// highestRungBelow returns the largest ladder rung at or below netWorth, or 0 when
// net worth hasn't reached the first rung (nothing to celebrate yet).
func highestRungBelow(netWorth int64) int64 {
	var best int64
	for _, rung := range netWorthLadder {
		if netWorth >= rung {
			best = rung
		}
	}
	return best
}

// Detect returns the milestones present in the snapshot, in a stable order (goals,
// then net worth, then no-spend streak, then kept budgets). It reports the current
// state — the UI decides which are newly reached (via Key) and worth a celebration
// versus a calm listing.
func Detect(in Input) []Milestone {
	var out []Milestone
	for _, name := range in.ReachedGoals {
		if name == "" {
			continue
		}
		out = append(out, Milestone{Key: "goal:" + name, Kind: KindGoalReached, Name: name})
	}
	if rung := highestRungBelow(in.NetWorthMinor); rung > 0 {
		out = append(out, Milestone{Key: "networth:" + strconv.FormatInt(rung, 10), Kind: KindNetWorth, Value: rung})
	}
	if in.NoSpendDays >= noSpendMinDays {
		out = append(out, Milestone{Key: "nospend:" + strconv.Itoa(in.NoSpendDays), Kind: KindNoSpendStreak, Value: int64(in.NoSpendDays)})
	}
	if in.KeptBudgets > 0 && in.KeptPeriodKey != "" {
		out = append(out, Milestone{Key: "kept:" + in.KeptPeriodKey, Kind: KindKeptBudgets, Value: int64(in.KeptBudgets)})
	}
	return out
}
