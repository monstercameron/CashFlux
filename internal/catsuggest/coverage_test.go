// SPDX-License-Identifier: MIT

package catsuggest_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/learntally"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/store"
)

// TestColdStartCoverageOverTheSampleDataset measures what the FREE, local
// sources can categorize with no rules and no history at all — the cold start
// C514 exists for. It is the number that decides how much a SMART+ pass is
// asked to do (and therefore what it costs), so it is asserted rather than left
// to anecdote.
//
// Rules are deliberately excluded here: this measures the dictionary alone
// against a household that has just imported its first statement.
func TestColdStartCoverageOverTheSampleDataset(t *testing.T) {
	ds := store.SampleDataset()
	queue := reviewqueue.Queue(ds.Transactions)
	if len(queue) == 0 {
		t.Fatal("sample dataset has nothing needing review; this test measures nothing")
	}

	resolved, bySource := 0, map[string]int{}
	for _, tx := range queue {
		s, ok := catsuggest.Resolve(catsuggest.Input{
			Payee:       tx.Payee,
			Desc:        tx.Desc,
			AmountMinor: tx.Amount.Amount,
			Categories:  ds.Categories,
			// No rules, no history — a brand-new household.
		})
		if !ok {
			continue
		}
		resolved++
		bySource[s.Source.String()]++
	}

	pct := resolved * 100 / len(queue)
	t.Logf("cold start: %d of %d queued charges resolved locally (%d%%) — by source: %v",
		resolved, len(queue), pct, bySource)

	// The dictionary must carry real weight on day one. This is a floor, not a
	// target: if a table edit drops coverage below it, that is a regression worth
	// failing over.
	if pct < 25 {
		t.Errorf("cold-start coverage is %d%%, want at least 25%% — the merchant dictionary "+
			"is meant to make the first import mostly self-categorizing", pct)
	}
	if bySource["dictionary"] == 0 {
		t.Error("no charge resolved via the dictionary; the table is not reaching real descriptors")
	}
}

// TestHistoryLiftsCoverageAboveTheDictionary proves the two local sources
// compound: once a household has corrected a few merchants, the free tier covers
// strictly more than the shipped table alone.
func TestHistoryLiftsCoverageAboveTheDictionary(t *testing.T) {
	ds := store.SampleDataset()
	queue := reviewqueue.Queue(ds.Transactions)

	count := func(tally learntally.Tally) int {
		n := 0
		for _, tx := range queue {
			if _, ok := catsuggest.Resolve(catsuggest.Input{
				Payee:       tx.Payee,
				Desc:        tx.Desc,
				AmountMinor: tx.Amount.Amount,
				Tally:       tally,
				Categories:  ds.Categories,
			}); ok {
				n++
			}
		}
		return n
	}

	cold := count(nil)

	// Teach it the merchants the dictionary does NOT know, the way a user would:
	// by categorizing a few of them. Three corrections clears the threshold.
	taught := learntally.Tally{}
	var target string
	for _, tx := range queue {
		if _, known := catsuggest.Resolve(catsuggest.Input{
			Payee: tx.Payee, Desc: tx.Desc, AmountMinor: tx.Amount.Amount,
			Categories: ds.Categories,
		}); known {
			continue
		}
		payee := tx.Payee
		if payee == "" {
			payee = tx.Desc
		}
		if payee == "" {
			continue
		}
		target = payee
		break
	}
	if target == "" {
		t.Skip("every queued charge is already resolvable; nothing left to learn")
	}
	expenseCat := ""
	for _, c := range ds.Categories {
		if c.Kind == domain.KindExpense {
			expenseCat = c.ID
			break
		}
	}
	for i := 0; i < learntally.DefaultMinCount; i++ {
		taught.Record(target, expenseCat)
	}

	warm := count(taught)
	if warm <= cold {
		t.Errorf("teaching %q did not increase coverage (%d -> %d); history is not being consulted",
			target, cold, warm)
	}
	t.Logf("coverage %d -> %d of %d after learning one merchant", cold, warm, len(queue))
}
