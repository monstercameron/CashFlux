// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// r3pFixture builds a sample-household-sized ledger (3,292 rows, matching the
// browser measurement in ticket R3P): mostly expenses, some income, some
// transfer legs, spread over merchants, accounts and categories.
func r3pFixture(n int) []domain.Transaction {
	txns := make([]domain.Transaction, n)
	for i := range txns {
		t := domain.Transaction{
			ID:         fmt.Sprintf("r3p-%04d", i),
			Desc:       fmt.Sprintf("Market purchase %04d", i),
			Payee:      fmt.Sprintf("market-%02d", i%40),
			AccountID:  fmt.Sprintf("acct-%d", i%4),
			CategoryID: fmt.Sprintf("cat-%d", i%12),
			Amount:     money.New(int64(-100-i), "USD"),
			Date:       time.Unix(1_700_000_000+int64(i*3600), 0).UTC(),
		}
		switch {
		case i%10 == 9: // transfer leg
			t.TransferAccountID = "acct-9"
		case i%5 == 4: // income
			t.Amount = money.New(int64(5000+i), "USD")
		}
		txns[i] = t
	}
	return txns
}

func r3pTime(t *testing.T, label string, iters int, fn func()) {
	t.Helper()
	start := time.Now()
	for i := 0; i < iters; i++ {
		fn()
	}
	t.Logf("R3P-PERF %-44s %8.3f ms/op (%d iters)", label, float64(time.Since(start).Microseconds())/1000/float64(iters), iters)
}

// TestR3PPerfTimings3292 logs the per-render pure-work costs on a 3,292-row
// ledger. Timings are logged as evidence; wall-time assertions would flake.
func TestR3PPerfTimings3292(t *testing.T) {
	txns := r3pFixture(3292)
	filter := txnfilter.Criteria{Sort: "amount", Dir: txnfilter.Asc}

	shown := deriveTxnRows(txns, filter, txnfilter.Labels{}, nil, func(s string) string { return s })

	r3pTime(t, "filter+amount-sort 3292 (deriveTxnRows)", 20, func() {
		_ = deriveTxnRows(txns, filter, txnfilter.Labels{}, nil, func(s string) string { return s })
	})
	r3pTime(t, "toolbar cold scans: review+dupes+presets", 5, func() {
		_ = reviewqueue.Count(txns)
		_ = dedupe.Count(dedupe.FindDuplicates(txns))
		_ = txnfilter.CountPresets(txns, filter, txnfilter.PresetScope{
			ReviewTag: reviewqueue.ReviewTag,
			LargeFor:  func(txn domain.Transaction) int64 { return currency.MinorFromMajor(100, txn.Amount.Currency) },
		})
	})
	catNames := map[string]string{"cat-1": "Transfer"}
	r3pTime(t, "OLD suspect-scan over shown 3292", 20, func() {
		n := 0
		for _, txn := range shown {
			if _, ok := txnclassify.Suspected(txn, catNames[txn.CategoryID]); ok {
				n++
			}
		}
		_ = n
	})
	r3pTime(t, "NEW suspect check per visible page (25)", 200, func() {
		n := 0
		for _, txn := range shown[:25] {
			if _, ok := txnclassify.Suspected(txn, catNames[txn.CategoryID]); ok {
				n++
			}
		}
		_ = n
	})
	r3pTime(t, "txByID map build over shown 3292", 50, func() {
		m := make(map[string]domain.Transaction, len(shown))
		for _, txn := range shown {
			m[txn.ID] = txn
		}
		_ = m
	})
}

// TestR3PRowRendersOnSelectionChange renders a page of REAL txnFrameRow
// components through the row-props cache, then flips one row's Selected flag
// (what selecting a checkbox does) and a sort-like reorder, measuring the
// wall time of each testkit re-render. DOM node count is reported as the
// mock-DOM proxy for DOM size; the mock DOM exposes no MutationObserver, so
// mutation records cannot be counted here (browser-only measurement).
func TestR3PRowRendersOnSelectionChange(t *testing.T) {
	const page = 25
	cache := &txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)}
	rowList := func(selectedID string, reversed bool) ui.Node {
		kids := make([]any, 0, page)
		for i := 0; i < page; i++ {
			idx := i
			if reversed {
				idx = page - 1 - i
			}
			p := r4cRow(fmt.Sprintf("row-%02d", idx))
			p.Selected = p.ID == selectedID
			kids = append(kids, ui.CreateElement(txnFrameRow, cache.reuse(p)))
		}
		return shorthand.Div(kids...)
	}
	f := render.New(t)
	defer f.Cleanup()

	start := time.Now()
	f.Render(rowList("", false))
	t.Logf("R3P-PERF mount 25 real rows: %v, dom text bytes=%d", time.Since(start), len(f.Text()))

	start = time.Now()
	f.Render(rowList("row-03", false))
	t.Logf("R3P-PERF select one row re-render: %v", time.Since(start))

	start = time.Now()
	f.Render(rowList("row-03", true))
	t.Logf("R3P-PERF sort-like reorder re-render: %v", time.Since(start))

	start = time.Now()
	f.Render(rowList("", true))
	t.Logf("R3P-PERF deselect re-render: %v", time.Since(start))
}

// TestR3PCountLineUsesProvidedCashflow verifies the line renders its caller's fold.
func TestR3PCountLineUsesProvidedCashflow(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 1250, Total: 3292, Net: money.New(200, "USD"),
		Spent: money.New(700, "USD"), Income: money.New(900, "USD"), CashflowProvided: true,
	}))
	got := f.Text()
	for _, want := range []string{"Spent $7.00", "Income $9.00"} {
		if !strings.Contains(got, want) {
			t.Errorf("provided cashflow: count line = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "Spent −$100.00") {
		t.Errorf("provided cashflow: count line = %q, ledger's own total leaked in (second pass still running)", got)
	}
	f.Cleanup()

}

// TestR3PSummaryCacheSurvivesSortAndPage proves sort/page changes no longer
// invalidate the toolbar's whole-ledger scans, while a scope change does.
func TestR3PSummaryCacheSurvivesSortAndPage(t *testing.T) {
	cache := &txnToolbarSummaryCache{}
	calls := 0
	compute := func() txnToolbarSummary { calls++; return txnToolbarSummary{review: calls} }
	base := txnfilter.Criteria{Text: "coffee", Sort: "date", Dir: txnfilter.Desc, Page: 1, PageSize: 25}
	if got := cache.get(1, txnSummaryScope(base), compute); got.review != 1 || calls != 1 {
		t.Fatalf("first summary = %+v after %d calls, want 1/1", got, calls)
	}
	sorted := base
	sorted.Sort, sorted.Dir, sorted.Page = "amount", txnfilter.Asc, 3
	if got := cache.get(1, txnSummaryScope(sorted), compute); got.review != 1 || calls != 1 {
		t.Fatalf("sort/page change recomputed summary: %+v after %d calls, want cached 1/1", got, calls)
	}
	scoped := base
	scoped.Text = "rent"
	if got := cache.get(1, txnSummaryScope(scoped), compute); got.review != 2 || calls != 2 {
		t.Fatalf("scope change kept stale summary: %+v after %d calls, want 2/2", got, calls)
	}
	if got := cache.get(2, txnSummaryScope(scoped), compute); got.review != 3 || calls != 3 {
		t.Fatalf("data revision kept stale summary: %+v after %d calls, want 3/3", got, calls)
	}
}

// TestR3PCollectGroupMembersWholeLedger proves a group's members resolve
// against the whole ledger, including members no filter currently shows.
func TestR3PCollectGroupMembersWholeLedger(t *testing.T) {
	all := []domain.Transaction{
		{ID: "a", Amount: money.New(-1000, "USD")},
		{ID: "b", Amount: money.New(-2000, "USD")},
		{ID: "c", Amount: money.New(-3000, "USD")},
		{ID: "other", Amount: money.New(-9000, "USD")},
	}
	link := domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"a", "b", "c"}}
	byTxn := map[string]domain.TxnLink{"a": link, "b": link, "c": link}
	got := collectTxnGroupMembers(byTxn, all)
	if len(got) != 3 {
		t.Fatalf("collected %d members, want 3 (whole group): %v", len(got), got)
	}
	if _, ok := got["other"]; ok {
		t.Errorf("collected a non-member row")
	}
	if got := collectTxnGroupMembers(nil, all); len(got) != 0 {
		t.Errorf("no groups collected %d members, want 0", len(got))
	}
}
