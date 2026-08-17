// SPDX-License-Identifier: MIT

// Package splitsuggest proposes a category breakdown for a new charge from how
// the household has split the SAME merchant before (SM-3).
//
// A warehouse club or a supermarket run is rarely one category, and re-typing the
// same three-way split every time is the kind of chore the app should already know.
// Given the merchant's prior split charges, this package averages each category's
// SHARE of those charges and applies the averaged shares to the new amount.
//
// Two design rules carry the weight:
//
//   - Shares are averaged per PRECEDENT, not pooled across all their money. One
//     unusually large past run would otherwise dictate the shape of every future
//     one, which is the opposite of "typical".
//   - The proposed lines sum to the charge EXACTLY. Money is integer minor units
//     and the rounding remainder is handed out by largest fractional part, so a
//     proposal can be applied without a reconciling line.
//
// The caller resolves merchant identity (payee aliases) and passes in only the
// precedents it considers the same merchant, exactly as internal/catsuggest takes
// the rules engine's answer rather than running it. Pure Go, no syscall/js,
// unit-tested on native Go.
package splitsuggest

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// DefaultMinPrecedents is how many past split charges a merchant needs before a
// proposal is offered. One precedent is an anecdote: repeating a single past
// breakdown as "typical" is how a wrong guess becomes a habit.
const DefaultMinPrecedents = 2

// dominantShareBP is the point past which a breakdown stops being a split. When
// one category already takes 95%+ of the typical charge, the honest suggestion is
// a category (SM-2), not a split, and offering one here would be noise.
const dominantShareBP = 9500

// bpScale is the basis-point denominator shares are expressed in (10000 = 100%).
const bpScale = 10000

// Line is one proposed split line: a category, the amount it takes of THIS
// charge, and the share of the merchant's typical charge it represents.
type Line struct {
	CategoryID string
	// AmountMinor carries the same sign as the charge being split.
	AmountMinor int64
	// ShareBP is the averaged share in basis points (2500 = 25%). It is evidence,
	// not decoration: the UI states it so the user can judge the proposal rather
	// than trust it.
	ShareBP int
}

// Suggestion is a proposed breakdown plus the evidence behind it. Evidence is
// DATA, never a sentence — the UI formats it through en_*.go.
type Suggestion struct {
	// Lines are ordered by descending share, then category id, so the same inputs
	// always render in the same order.
	Lines []Line
	// Precedents is how many past charges the shares were averaged over.
	Precedents int
}

// Input is everything a proposal needs.
type Input struct {
	// AmountMinor is the charge to split, signed as the ledger stores it. Zero
	// yields no suggestion (there is nothing to divide).
	AmountMinor int64
	// History is the merchant's PRIOR charges — the caller has already resolved
	// payee aliases and excluded the charge being split. Entries without a split
	// are ignored, so a caller may pass the merchant's whole history.
	History []domain.Transaction
	// MinPrecedents overrides DefaultMinPrecedents when > 0.
	MinPrecedents int
}

// minPrecedents resolves the threshold, defaulting when unset.
func (in Input) minPrecedents() int {
	if in.MinPrecedents > 0 {
		return in.MinPrecedents
	}
	return DefaultMinPrecedents
}

// Suggest returns the proposed breakdown, or ok=false when the merchant has no
// usable split history, when the typical breakdown is really a single category,
// or when the charge is zero.
func Suggest(in Input) (Suggestion, bool) {
	if in.AmountMinor == 0 {
		return Suggestion{}, false
	}

	// Average each category's share ACROSS precedents: sum of per-precedent
	// shares, divided by the precedent count at the end.
	shareSum := map[string]int64{}
	precedents := 0
	for _, t := range in.History {
		lines, total := splitShape(t)
		if len(lines) < 2 || total == 0 {
			continue
		}
		precedents++
		for cat, amt := range lines {
			shareSum[cat] += amt * bpScale / total
		}
	}
	if precedents < in.minPrecedents() || len(shareSum) < 2 {
		return Suggestion{}, false
	}

	// Order the categories deterministically before any remainder is handed out,
	// so an exact tie in share resolves the same way on every run.
	cats := make([]string, 0, len(shareSum))
	for cat := range shareSum {
		cats = append(cats, cat)
	}
	avg := make(map[string]int64, len(shareSum))
	for _, cat := range cats {
		avg[cat] = shareSum[cat] / int64(precedents)
	}
	sort.Slice(cats, func(i, j int) bool {
		if avg[cats[i]] != avg[cats[j]] {
			return avg[cats[i]] > avg[cats[j]]
		}
		return cats[i] < cats[j]
	})

	weights := make([]int64, len(cats))
	for i, cat := range cats {
		weights[i] = avg[cat]
	}
	// A category that averaged out to nothing cannot claim a line; dropping it
	// here keeps a $0 line out of the proposal.
	kept, keptWeights := cats[:0:0], weights[:0:0]
	for i, w := range weights {
		if w > 0 {
			kept = append(kept, cats[i])
			keptWeights = append(keptWeights, w)
		}
	}
	if len(kept) < 2 {
		return Suggestion{}, false
	}

	amounts := Distribute(in.AmountMinor, keptWeights)
	totalW := int64(0)
	for _, w := range keptWeights {
		totalW += w
	}
	out := Suggestion{Precedents: precedents, Lines: make([]Line, 0, len(kept))}
	for i, cat := range kept {
		out.Lines = append(out.Lines, Line{
			CategoryID:  cat,
			AmountMinor: amounts[i],
			ShareBP:     int(keptWeights[i] * bpScale / totalW),
		})
	}
	// One category swallowing the charge is a categorization, not a split.
	if out.Lines[0].ShareBP >= dominantShareBP {
		return Suggestion{}, false
	}
	return out, true
}

// splitShape reduces one transaction to its per-category split amounts (in
// absolute minor units) and their total. A transaction without a multi-line split
// returns nil. Lines are keyed by category, so a breakdown that names the same
// category twice is folded into one share rather than counted twice.
func splitShape(t domain.Transaction) (map[string]int64, int64) {
	if len(t.Splits) < 2 {
		return nil, 0
	}
	lines := make(map[string]int64, len(t.Splits))
	total := int64(0)
	for _, s := range t.Splits {
		if s.CategoryID == "" {
			continue
		}
		amt := s.Amount.Amount
		if amt < 0 {
			amt = -amt
		}
		if amt == 0 {
			continue
		}
		lines[s.CategoryID] += amt
		total += amt
	}
	return lines, total
}

// Distribute apportions total across weights so the parts sum to total EXACTLY,
// preserving total's sign. The remainder goes to the largest fractional parts
// (ties broken by earlier index), which is the standard largest-remainder
// apportionment — no floats, no reconciling line.
//
// A zero weight total, or no weights, returns nil.
func Distribute(total int64, weights []int64) []int64 {
	if len(weights) == 0 {
		return nil
	}
	sum := int64(0)
	for _, w := range weights {
		if w > 0 {
			sum += w
		}
	}
	if sum == 0 {
		return nil
	}
	neg := total < 0
	mag := total
	if neg {
		mag = -mag
	}

	out := make([]int64, len(weights))
	// rem[i] is the fractional part of weight i's exact share, scaled by sum, so
	// the comparison stays in integers.
	type frac struct {
		idx int
		rem int64
	}
	fracs := make([]frac, 0, len(weights))
	assigned := int64(0)
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		exact := mag * w
		out[i] = exact / sum
		assigned += out[i]
		fracs = append(fracs, frac{idx: i, rem: exact % sum})
	}
	// Largest fractional part first; SliceStable keeps an exact tie with the
	// earlier index, so the extra unit lands deterministically.
	sort.SliceStable(fracs, func(a, b int) bool { return fracs[a].rem > fracs[b].rem })
	// Each exact share contributes less than one whole unit of remainder, so the
	// shortfall is always smaller than the number of positive weights; the bound
	// is belt-and-braces, never load-bearing.
	for i := 0; int64(i) < mag-assigned && i < len(fracs); i++ {
		out[fracs[i].idx]++
	}
	if neg {
		for i := range out {
			out[i] = -out[i]
		}
	}
	return out
}

// AsSplits renders a suggestion as the domain split lines a transaction stores,
// in the suggestion's own order. currency is the charge's currency code.
func AsSplits(s Suggestion, currency string) []domain.CategorySplit {
	if len(s.Lines) == 0 {
		return nil
	}
	out := make([]domain.CategorySplit, 0, len(s.Lines))
	for _, l := range s.Lines {
		out = append(out, domain.CategorySplit{
			CategoryID: l.CategoryID,
			Amount:     money.New(l.AmountMinor, currency),
		})
	}
	return out
}
