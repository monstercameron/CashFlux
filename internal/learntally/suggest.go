// SPDX-License-Identifier: MIT

package learntally

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/payeealias"
)

// MerchantKey returns the tally key for a merchant STEM rather than an exact
// descriptor (C513). NormalizePayee keys on the whole payee, so
// "AMZN MKTP US*2H4RT9" and "AMZN MKTP US*8K1QP2" are separate keys and a
// merchant whose descriptor carries a per-charge reference could never reach the
// suggestion threshold — the exact case this package exists for. Routing through
// payeealias.Normalize collapses processor prefixes and trailing references, so
// every Amazon charge lands on one key.
//
// Keys from this function are namespaced with a "~" prefix so they cannot
// collide with the exact-descriptor keys written by Increment. Both live in one
// Tally, which keeps the persisted shape unchanged.
func MerchantKey(raw string) string {
	clean := NormalizePayee(payeealias.Normalize(raw))
	if clean == "" {
		return ""
	}
	return "~" + clean
}

// Record files one correction under BOTH the exact descriptor and the merchant
// stem, so an exact repeat can outrank a merely-similar one at suggestion time.
// A no-op when the payee or category is empty.
//
// Increment remains for callers that only want the exact key; Record is what new
// callers should use.
func (t Tally) Record(rawPayee, categoryID string) {
	if strings.TrimSpace(rawPayee) == "" || categoryID == "" {
		return
	}
	t.Increment(rawPayee, categoryID)
	if mk := MerchantKey(rawPayee); mk != "" {
		if t[mk] == nil {
			t[mk] = make(map[string]int)
		}
		t[mk][categoryID]++
	}
}

// Match reports how a suggestion was reached. Callers use it to rank and to
// explain; this package deliberately returns DATA, never a display string — all
// user-facing copy goes through the i18n layer.
type Match int

const (
	// MatchNone: no usable history.
	MatchNone Match = iota
	// MatchExact: the identical descriptor has been categorized before.
	MatchExact
	// MatchMerchant: other charges from the same merchant have been categorized,
	// though this exact descriptor is new.
	MatchMerchant
)

// Suggestion is a history-derived category proposal with the evidence behind it.
type Suggestion struct {
	// CategoryID is the proposed category.
	CategoryID string
	// Match says whether the evidence came from the exact descriptor or the merchant.
	Match Match
	// Count is how many times CategoryID was chosen for the matched key.
	Count int
	// Total is how many corrections exist for that key across all categories.
	// Count/Total is the consistency of the history: 6/7 is a strong signal,
	// 4/7 is a coin flip wearing a suggestion's clothes.
	Total int
}

// Consistent reports whether the winning category holds a strict majority of the
// evidence. A split history (4 Groceries / 3 Dining) is NOT consistent, and a
// caller should treat it as low confidence rather than picking a winner.
func (s Suggestion) Consistent() bool {
	return s.Total > 0 && s.Count*2 > s.Total
}

// Suggest proposes a category from past corrections, preferring the exact
// descriptor over the merchant stem. threshold is the minimum number of
// corrections required (<= 0 is treated as DefaultMinCount).
//
// It returns ok=false when neither key clears the threshold. It does NOT refuse
// an inconsistent history — that judgement belongs to the caller, which has the
// Count/Total to make it — but see Consistent.
func (t Tally) Suggest(rawPayee string, threshold int) (Suggestion, bool) {
	if threshold <= 0 {
		threshold = DefaultMinCount
	}
	if s, ok := t.suggestFor(NormalizePayee(rawPayee), MatchExact, threshold); ok {
		return s, true
	}
	return t.suggestFor(MerchantKey(rawPayee), MatchMerchant, threshold)
}

// suggestFor resolves one already-normalized key.
func (t Tally) suggestFor(key string, m Match, threshold int) (Suggestion, bool) {
	if key == "" {
		return Suggestion{}, false
	}
	cats, ok := t[key]
	if !ok || len(cats) == 0 {
		return Suggestion{}, false
	}
	var winner string
	var best, total int
	for cat, n := range cats {
		total += n
		// Ties break on the lexicographically smallest id, matching TopCategory,
		// so a tied history always yields the same answer across runs.
		if n > best || (n == best && (winner == "" || cat < winner)) {
			winner, best = cat, n
		}
	}
	if winner == "" || best < threshold {
		return Suggestion{}, false
	}
	return Suggestion{CategoryID: winner, Match: m, Count: best, Total: total}, true
}
