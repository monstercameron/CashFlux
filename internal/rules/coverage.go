// SPDX-License-Identifier: MIT

package rules

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/payeeclean"
)

// CoverageSummary is the prepared, full-context view of the current rule list
// against a transaction slice. Counts has one entry per rule in storage order.
// Conflicts retains the canonical conflict indexes into that same order.
type CoverageSummary struct {
	Covered       int
	MaxMatchCount int
	Counts        []int
	Conflicts     []Conflict
}

// SummarizeCoverage evaluates full-context rule counts and first-match coverage
// in one native pass boundary. It does not modify either input. Rule order is
// significant for Covered and is preserved for Counts and Conflicts.
func SummarizeCoverage(rs []Rule, txns []TxnCtx) CoverageSummary {
	summary := CoverageSummary{
		Counts:    make([]int, len(rs)),
		Conflicts: Conflicts(rs),
	}
	phraseRules := make([]bool, len(rs))
	phrasePatterns := make([]string, len(rs))
	hasPhraseRules := false
	for i, rule := range rs {
		if len(rule.Conditions) == 0 {
			phraseRules[i] = true
			phrasePatterns[i] = strings.ToLower(strings.TrimSpace(rule.Match))
			hasPhraseRules = true
		}
	}
	for _, txn := range txns {
		phraseText := ""
		if hasPhraseRules {
			phraseText = strings.ToLower(txn.Payee + " " + txn.Desc + " " + payeeclean.Suggest(txn.Payee))
		}
		firstMatched := false
		for i, rule := range rs {
			var matched bool
			if phraseRules[i] {
				matched = matchesLowered(phraseText, phrasePatterns[i])
			} else {
				matched = rule.matchesFull(txn)
			}
			if matched {
				summary.Counts[i]++
				if !firstMatched {
					firstMatched = true
				}
			}
		}
		if firstMatched {
			summary.Covered++
		}
	}
	for _, count := range summary.Counts {
		if count > summary.MaxMatchCount {
			summary.MaxMatchCount = count
		}
	}
	return summary
}
