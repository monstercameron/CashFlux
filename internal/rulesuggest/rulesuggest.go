// SPDX-License-Identifier: MIT

// Package rulesuggest proposes auto-categorization rules from how a household has
// already categorized its transactions. It's a deterministic, explainable
// heuristic (no AI): group transactions by their payee/description key, and where
// a key reliably maps to one category — appearing often enough and consistently
// enough, and not already covered by an existing rule — suggest a rule for it.
//
// Pure Go, no platform dependencies; unit-tested on native Go.
package rulesuggest

import (
	"sort"
	"strings"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/payeeclean"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// consistency is the minimum share of a key's transactions that must agree on one
// category before it's worth suggesting (so the odd miscategorized txn doesn't
// block a clear pattern, but a genuinely mixed key is skipped).
const consistency = 0.8

type suggestionAggregate struct {
	key     string
	display string
	total   int
	covered int
	byCat   map[string]int
}

// Suggestion is a proposed rule plus the evidence behind it: how many of the
// matching transactions support it and the total seen for that key.
type Suggestion struct {
	Rule    rules.Rule // Match (the payee/description key) + SetCategoryID
	Support int        // transactions for this key in the dominant category
	Total   int        // transactions seen for this key
}

// Suggest analyzes categorized, non-transfer transactions and returns proposed
// rules, most-supported first. A key (the payee, or the description when there's
// no payee) is suggested when it appears in at least minCount transactions, at
// least `consistency` of them share one category, and no existing rule already
// matches it. minCount below 2 is treated as 2.
func Suggest(txns []domain.Transaction, existing []rules.Rule, minCount int) []Suggestion {
	if minCount < 2 {
		minCount = 2
	}

	// Per normalized key: display text, counts, and existing-rule coverage.
	keys := map[string]*suggestionAggregate{}
	for _, t := range txns {
		if t.IsTransfer() || t.CategoryID == "" {
			continue
		}
		text := strings.TrimSpace(t.Payee)
		if text == "" {
			text = strings.TrimSpace(t.Desc)
		} else {
			text = strings.TrimSpace(payeeclean.Suggest(text))
		}
		norm := strings.ToLower(text)
		if len(norm) < 3 {
			continue // too short to be a meaningful match
		}
		a := keys[norm]
		if a == nil {
			a = &suggestionAggregate{key: norm, display: text, byCat: map[string]int{}}
			keys[norm] = a
		}
		a.total++
		a.byCat[t.CategoryID]++
		if rules.FirstMatchFullWhereCurrency(existing, t.Payee, t.Desc, t.Amount.Amount, t.Amount.Currency, t.AccountID, rules.NewTxnDate(t.Date), nil) != nil {
			a.covered++
		}
	}

	dominant := make(map[string]string, len(keys))
	for key, a := range keys {
		bestCat, bestN := "", 0
		for cat, n := range a.byCat {
			if n > bestN || (n == bestN && cat < bestCat) {
				bestCat, bestN = cat, n
			}
		}
		if a.total >= minCount && bestCat != "" && float64(bestN)/float64(a.total) >= consistency {
			dominant[key] = bestCat
		}
	}
	conflicting := conflictingKeys(keys, dominant)
	var out []Suggestion
	for _, a := range keys {
		if a.total < minCount {
			continue
		}
		// Find the dominant category for this key.
		bestCat, bestN := dominant[a.key], 0
		for cat, n := range a.byCat {
			if cat == bestCat {
				bestN = n
			}
		}
		if bestCat == "" || float64(bestN)/float64(a.total) < consistency {
			continue
		}
		// Skip keys an existing rule already covers: the legacy substring check
		// (a Match rule catches every occurrence of the key text), plus the
		// conditions-aware share — when >= `consistency` of the key's own
		// transactions are already caught by ANY rule under full matching,
		// suggesting a duplicate would be noise.
		if rules.FirstMatch(existing, a.display) != nil {
			continue
		}
		if float64(a.covered)/float64(a.total) >= consistency {
			continue
		}
		if conflicting[a.key] {
			continue
		}
		out = append(out, Suggestion{
			Rule:    rules.Rule{Match: a.display, SetCategoryID: bestCat},
			Support: bestN,
			Total:   a.total,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Support != out[j].Support {
			return out[i].Support > out[j].Support
		}
		return out[i].Rule.Match < out[j].Rule.Match
	})
	return out
}

// conflictingKeys rejects broad short keys contained as whole phrases in
// longer keys with a different dominant category. It indexes substrings of each
// merchant key instead of comparing every pair of keys.
func conflictingKeys(keys map[string]*suggestionAggregate, dominant map[string]string) map[string]bool {
	conflicting := make(map[string]bool)
	for longer, otherCategory := range dominant {
		if otherCategory == "" {
			continue
		}
		runes := []rune(longer)
		for start := 0; start < len(runes); start++ {
			if start > 0 && merchantWordRune(runes[start-1]) {
				continue
			}
			for end := start + 1; end <= len(runes); end++ {
				if end-start == len(runes) {
					continue
				}
				short := string(runes[start:end])
				if _, exists := keys[short]; exists && dominant[short] != otherCategory {
					conflicting[short] = true
				}
			}
		}
	}
	return conflicting
}

func merchantWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// ForPayee returns the rule suggestion for ONE payee, or ok=false when that
// payee does not clear the same bar a bulk suggestion has to (LF-10).
//
// It exists so a surface that has already identified a merchant — a discovered
// recurring candidate, say — can ask "would you also suggest a rule for this
// one?" without re-deriving what "confident enough" means. Two definitions of a
// confident rule, one here and one at the call site, is how a merchant comes to
// be offered a rule on one screen and refused it on another.
//
// minCount is the same threshold Suggest applies, and below 2 is treated as 2
// for the same reason: one transaction is not a pattern.
func ForPayee(txns []domain.Transaction, existing []rules.Rule, payee string, minCount int) (Suggestion, bool) {
	want := strings.ToLower(strings.TrimSpace(payee))
	if want == "" {
		return Suggestion{}, false
	}
	for _, s := range Suggest(txns, existing, minCount) {
		if strings.ToLower(strings.TrimSpace(s.Rule.Match)) == want {
			return s, true
		}
	}
	return Suggestion{}, false
}
