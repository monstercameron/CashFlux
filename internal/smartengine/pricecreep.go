// SPDX-License-Identifier: MIT

package smartengine

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func init() {
	register("SMART-BL16", bl16PriceCreep)
}

const (
	// priceCreepCycles is N — the number of consecutive cycles a recurring must
	// post above its expected amount before the creep is trusted (not one-off).
	priceCreepCycles = 2
	// priceCreepTolBps is the tolerance band above expected that still counts as
	// "on price" — 1% (100 basis points) absorbs rounding and tiny variation.
	priceCreepTolBps = 100
	// priceCreepMinExpected ignores trivial recurrings where a few cents of
	// creep isn't worth a flag.
	priceCreepMinExpected = 5_00
	// priceCreepLookback is how many cycles back the detector scans.
	priceCreepLookback = priceCreepCycles + 2
	// priceCreepMaxRatio caps a plausible creep at 3× the expected amount: a
	// "charge" that far above the set price is almost certainly a different
	// purchase mis-matched into the bill's window, not the bill's new price.
	priceCreepMaxRatio = 3
)

// priceCreepAdvice and priceCreepActionLabel are the engine's user-facing copy,
// kept as constants (this pure package cannot call uistate.T).
const (
	priceCreepAdvice      = "Accept the new price or make it a task to cancel or downgrade."
	priceCreepActionLabel = "Review the price"
)

// Creep is one detected price-creep finding: a recurring whose matched actual
// charges have run above its expected amount for priceCreepCycles cycles. All
// amounts are base-currency minor units, magnitudes (positive).
type Creep struct {
	RecurringID   string
	Label         string
	ExpectedMinor int64
	NewMinor      int64 // most recent cycle's actual charge
	Cycles        int   // consecutive cycles above expected (>= priceCreepCycles)
	CategoryID    string
}

// DetectCreep finds recurrings charging above their expected amount for
// priceCreepCycles consecutive cycles (within priceCreepTolBps tolerance). It is
// pure and deterministic; the accept-flow screen and the engine both read it so
// detection lives in one place. Findings are sorted by recurring id for a stable
// order.
func DetectCreep(in Input) []Creep {
	var out []Creep
	for _, r := range in.Recurring {
		if !r.Amount.IsNegative() {
			continue // only expenses creep
		}
		expected := abs64(in.toBaseMinor(r.Amount.Amount, r.Amount.Currency))
		if expected < priceCreepMinExpected {
			continue
		}
		bounds := recentCycleBoundaries(r, in.Now, priceCreepLookback)
		if len(bounds) < priceCreepCycles+1 {
			continue
		}
		threshold := expected + expected*priceCreepTolBps/10000
		if threshold <= expected {
			threshold = expected + 1
		}
		// Walk completed cycles most-recent-first: cycle k spans [bounds[k], bounds[k-1]).
		consecutive := 0
		var newest int64
		for k := 1; k < len(bounds); k++ {
			actual, has := matchedInWindow(in, r, bounds[k], bounds[k-1])
			if !has {
				break // a cycle with no matched charge breaks the streak
			}
			if actual <= threshold || actual > expected*priceCreepMaxRatio {
				break // on-price, or implausibly high (a mis-match, not a creep)
			}
			if consecutive == 0 {
				newest = actual
			}
			consecutive++
		}
		if consecutive >= priceCreepCycles {
			out = append(out, Creep{
				RecurringID: r.ID, Label: r.Label, ExpectedMinor: expected,
				NewMinor: newest, Cycles: consecutive, CategoryID: r.CategoryID,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RecurringID < out[j].RecurringID })
	return out
}

// SMART-BL16 — Price-creep watch. Surfaces DetectCreep findings as insights. The
// dismissal Key encodes the new price level, so if the price climbs further the
// Key changes and the insight re-appears (a dismissal only silences the level the
// user acknowledged).
func bl16PriceCreep(in Input) []smart.Insight {
	var out []smart.Insight
	for _, c := range DetectCreep(in) {
		delta := c.NewMinor - c.ExpectedMinor
		// Copy is assembled as locals: this package is pure (no syscall/js) and
		// cannot call uistate.T, so its insight strings are built here like every
		// other smartengine engine's.
		title := c.Label + " has crept up to " + in.hmoney(c.NewMinor)
		detail := c.Label + " is set at " + in.hmoney(c.ExpectedMinor) + " but the last " +
			plural(int64(c.Cycles), "cycle") + " charged about " + in.hmoney(c.NewMinor) +
			" — roughly " + in.hmoney(delta) + " more each time. " + priceCreepAdvice
		actionLabel := priceCreepActionLabel
		out = append(out, smart.Insight{
			Feature: "SMART-BL16",
			Page:    smart.PageBills,
			// Level-encoded key: a further increase (new NewMinor) re-flags.
			Key:      "SMART-BL16:" + c.RecurringID + ":" + itoa64(c.NewMinor),
			Title:    title,
			Detail:   detail,
			Severity: smart.SeverityWarn,
		}.WithAmount(in.baseMoney(delta)).
			WithAction(smart.Action{Kind: smart.ActionNavigate, Label: actionLabel,
				Route: "/recurring", RelatedType: "bill", RelatedID: c.RecurringID}))
	}
	return out
}

// matchedInWindow returns the magnitude (base minor) of the single debit that
// best represents the recurring's charge within [from, to), and whether one
// matched. A bill's cycle charge is ONE transaction, so candidates are never
// summed — a shared category (two tax installments, several streaming services)
// must not read as one bill "charging" their total. Label matches (the txn's
// text contains the recurring's label) are trusted first; category matches are
// the fallback. Among candidates, the amount CLOSEST to the expected amount
// wins — the bill's own charge, not a bigger neighbor in the same category.
func matchedInWindow(in Input, r domain.Recurring, from, to time.Time) (int64, bool) {
	label := strings.ToLower(strings.TrimSpace(r.Label))
	expected := abs64(in.toBaseMinor(r.Amount.Amount, r.Amount.Currency))
	best, bestDist := int64(0), int64(-1)
	bestByLabel := false
	consider := func(mag int64, byLabel bool) {
		dist := mag - expected
		if dist < 0 {
			dist = -dist
		}
		// A label match always beats a category-only match; within the same
		// tier, nearest-to-expected wins.
		switch {
		case bestDist < 0,
			byLabel && !bestByLabel,
			byLabel == bestByLabel && dist < bestDist:
			best, bestDist, bestByLabel = mag, dist, byLabel
		}
	}
	for _, t := range in.Transactions {
		if t.IsTransfer() || !t.Amount.IsNegative() {
			continue
		}
		if t.Date.Before(from) || !t.Date.Before(to) {
			continue
		}
		byLabel := label != "" && strings.Contains(strings.ToLower(txnLabel(t)), label)
		byCat := r.CategoryID != "" && t.CategoryID == r.CategoryID
		if !byLabel && !byCat {
			continue
		}
		consider(abs64(in.toBaseMinor(t.Amount.Amount, t.Amount.Currency)), byLabel)
	}
	return best, bestDist >= 0
}

// recentCycleBoundaries returns up to count+1 cycle boundaries for a recurring,
// descending (most recent on-or-before now first), so consecutive completed
// cycles can be bucketed as [bounds[k], bounds[k-1]). It anchors on NextDue (or
// now when unset) and steps back one cadence at a time.
func recentCycleBoundaries(r domain.Recurring, now time.Time, count int) []time.Time {
	anchor := r.NextDue
	if anchor.IsZero() {
		anchor = now
	}
	// Walk the anchor down to the most recent boundary on-or-before now.
	b := anchor
	for b.After(now) {
		b = cadenceBack(r.Cadence, b)
	}
	// If the anchor was already far in the past, walk it up to just-before now.
	for {
		nxt := r.Cadence.Next(b)
		if nxt.After(now) {
			break
		}
		b = nxt
	}
	out := []time.Time{b}
	for i := 0; i < count; i++ {
		b = cadenceBack(r.Cadence, b)
		out = append(out, b)
	}
	return out
}

// cadenceBack steps a date back one cadence period — the inverse of
// RecurringCadence.Next, close enough for bucketing charges into cycle windows.
func cadenceBack(c domain.RecurringCadence, t time.Time) time.Time {
	switch c {
	case domain.CadenceWeekly:
		return t.AddDate(0, 0, -7)
	case domain.CadenceBiweekly:
		return t.AddDate(0, 0, -14)
	case domain.CadenceSemimonthly:
		return t.AddDate(0, 0, -15)
	case domain.CadenceQuarterly:
		return t.AddDate(0, -3, 0)
	case domain.CadenceYearly:
		return t.AddDate(-1, 0, 0)
	default: // monthly
		return t.AddDate(0, -1, 0)
	}
}
