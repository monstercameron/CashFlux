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

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/rules"
)

// consistency is the minimum share of a key's transactions that must agree on one
// category before it's worth suggesting (so the odd miscategorized txn doesn't
// block a clear pattern, but a genuinely mixed key is skipped).
const consistency = 0.8

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

	// Per normalized key: the first-seen original text, total count, and per-category counts.
	type agg struct {
		display string
		total   int
		byCat   map[string]int
	}
	keys := map[string]*agg{}
	for _, t := range txns {
		if t.IsTransfer() || t.CategoryID == "" {
			continue
		}
		text := t.Payee
		if strings.TrimSpace(text) == "" {
			text = t.Desc
		}
		text = strings.TrimSpace(text)
		norm := strings.ToLower(text)
		if len(norm) < 3 {
			continue // too short to be a meaningful match
		}
		a := keys[norm]
		if a == nil {
			a = &agg{display: text, byCat: map[string]int{}}
			keys[norm] = a
		}
		a.total++
		a.byCat[t.CategoryID]++
	}

	var out []Suggestion
	for _, a := range keys {
		if a.total < minCount {
			continue
		}
		// Find the dominant category for this key.
		bestCat, bestN := "", 0
		for cat, n := range a.byCat {
			if n > bestN || (n == bestN && cat < bestCat) {
				bestCat, bestN = cat, n
			}
		}
		if bestCat == "" || float64(bestN)/float64(a.total) < consistency {
			continue
		}
		// Skip keys an existing rule already covers.
		if rules.FirstMatch(existing, a.display) != nil {
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
