// SPDX-License-Identifier: MIT

// Package capgains reports what a year's disposals realized (FP-T1e, part 2).
//
// FP-T1d computed the gain on each sale; this is the year-level view — what has
// to be carried onto a tax return, split the way the return splits it, with the
// individual sales still visible underneath so a figure can be checked rather
// than only trusted.
//
// The split between short-term and long-term is the whole reason this is not one
// number: they are taxed at different rates, and a household that nets them into
// a single "capital gains" figure has lost the fact that decides what they owe.
package capgains

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Summary is a period's realized gains and losses.
type Summary struct {
	// Sales are the disposals in the window, newest first.
	Sales []domain.RealizedSale
	// ShortTermMinor and LongTermMinor are the netted gains for each holding
	// period. Either can be negative — a net loss is a real and useful outcome,
	// not an error to clamp away.
	ShortTermMinor, LongTermMinor int64
	// NetMinor is the two together.
	NetMinor int64
	// ProceedsMinor and BasisMinor are the year's totals, which is what a return
	// asks for alongside the gain.
	ProceedsMinor, BasisMinor int64
	// Methods are the basis methods used across the window, sorted.
	//
	// Surfaced because MIXING them across a year is legitimate but worth knowing:
	// the figures are then not reproducible from one rule, and a preparer asking
	// "how did you pick the shares" deserves a truthful answer.
	Methods []string
}

// MaxLossOffsetMinor is the amount of net capital loss usually deductible
// against ordinary income in one year, in minor units ($3,000).
//
// A convention with a real cutoff, not a computation: it is stated as a stated
// assumption wherever it is shown, because it is a rule that changes and that
// does not apply to everyone.
const MaxLossOffsetMinor = 300_000

// Gather summarizes disposals dated in [start, end).
func Gather(sales []domain.RealizedSale, start, end time.Time) Summary {
	var s Summary
	methods := map[string]bool{}
	for _, r := range sales {
		if !dateutil.InRange(r.Date, start, end) {
			continue
		}
		s.Sales = append(s.Sales, r)
		s.ShortTermMinor += r.ShortTermGainMinor
		s.LongTermMinor += r.LongTermGainMinor
		s.ProceedsMinor += r.ProceedsMinor
		s.BasisMinor += r.BasisMinor
		if r.Method != "" {
			methods[r.Method] = true
		}
	}
	s.NetMinor = s.ShortTermMinor + s.LongTermMinor
	// Newest first: the sales a reader is reconciling are the recent ones, and
	// ties break on ID so the same year always renders in the same order.
	sort.SliceStable(s.Sales, func(i, j int) bool {
		if !s.Sales[i].Date.Equal(s.Sales[j].Date) {
			return s.Sales[i].Date.After(s.Sales[j].Date)
		}
		return s.Sales[i].ID < s.Sales[j].ID
	})
	for m := range methods {
		s.Methods = append(s.Methods, m)
	}
	sort.Strings(s.Methods)
	return s
}

// DeductibleLossMinor is how much of a net loss can offset ordinary income this
// year, and how much carries forward.
//
// Reports ok=false when the year is not a net loss, so a surface says nothing
// rather than showing "0 deductible, 0 carried forward" on a profitable year —
// two zeroes that read as a failed calculation.
func (s Summary) DeductibleLossMinor() (deduct, carry int64, ok bool) {
	if s.NetMinor >= 0 {
		return 0, 0, false
	}
	loss := -s.NetMinor
	if loss <= MaxLossOffsetMinor {
		return loss, 0, true
	}
	return MaxLossOffsetMinor, loss - MaxLossOffsetMinor, true
}

// MixedMethods reports whether the window used more than one basis method.
func (s Summary) MixedMethods() bool { return len(s.Methods) > 1 }
