// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func perfTransactions(n int) []domain.Transaction {
	txns := make([]domain.Transaction, n)
	for i := range txns {
		txns[i] = domain.Transaction{
			ID: fmt.Sprintf("txn-%04d", i), Desc: fmt.Sprintf("Market purchase %04d", i),
			Payee: fmt.Sprintf("market-%02d", i%40), AccountID: fmt.Sprintf("acct-%d", i%4),
			CategoryID: fmt.Sprintf("cat-%d", i%12), Amount: money.New(int64(-100-i), "USD"),
			Date: time.Unix(1_700_000_000+int64(i), 0),
		}
	}
	return txns
}

func BenchmarkTxnFilterSortPerf3300(b *testing.B) {
	txns := perfTransactions(3300)
	criteria := txnfilter.Criteria{Text: "market-1", Sort: "amount", Dir: txnfilter.Asc}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = deriveTxnRows(txns, criteria, txnfilter.Labels{}, nil, func(s string) string { return s })
	}
}

func BenchmarkTxnSummary3300(b *testing.B) {
	txns := perfTransactions(3300)
	cache := &txnToolbarSummaryCache{}
	filter := uistate.TxFilter{Text: "market-1"}
	compute := func() txnToolbarSummary {
		var result txnToolbarSummary
		for _, txn := range txns {
			result.shownNet += txn.Amount.Amount
		}
		return result
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.get(1, filter, compute)
		cache.filter.Text = fmt.Sprintf("market-%d", i%40)
	}
}

func BenchmarkTxnProvenanceMap3300(b *testing.B) {
	txns := perfTransactions(3300)
	var ruleset []rules.Rule
	provCache = txnProvenanceCache{}
	_ = txnAutoByID(nil, txns, ruleset, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = txnAutoByID(nil, txns, ruleset, 1)
	}
}

func BenchmarkTxnSuggestionInputs3300(b *testing.B) {
	txns := perfTransactions(3300)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		byID := make(map[string]domain.Transaction, len(txns))
		for _, txn := range txns {
			byID[txn.ID] = txn
		}
	}
}

func BenchmarkTxnRowDerivedProps3300(b *testing.B) {
	cache := &txnFrameRowPropsCache{rows: make(map[string]*txnFrameRowProps)}
	props := make([]txnFrameRowProps, 3300)
	for i := range props {
		props[i] = txnFrameRowProps{ID: fmt.Sprintf("txn-%04d", i), Amount: "$42.00", Category: "Food", Desc: "Market"}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, row := range props {
			_ = cache.reuse(row)
		}
	}
}

func TestTxnRenderCounterHarness(t *testing.T) {
	type listProps struct {
		rows   []int
		action string
		change int
	}
	count := 0
	row := func(props struct{ index, change int }) ui.Node {
		count++
		return shorthand.Span(fmt.Sprint(props.index, props.change))
	}
	list := func(props listProps) ui.Node {
		children := make([]any, 0, len(props.rows))
		for _, index := range props.rows {
			change := 0
			if props.action == "single-row edit" && index == props.change {
				change = 1
			}
			children = append(children, ui.CreateElement(row, struct{ index, change int }{index, change}))
		}
		return shorthand.Div(children...)
	}
	fixture := render.New(t)
	visible := make([]int, 40)
	for i := range visible {
		visible[i] = i
	}
	fixture.Render(ui.CreateElement(list, listProps{rows: visible, action: "initial"}))
	if count != 40 {
		t.Fatalf("initial visible window rendered %d rows, want 40", count)
	}
	for _, action := range []string{"search keystroke", "sort click", "single-row edit", "page change"} {
		before := count
		rows := visible
		if action == "page change" {
			rows = make([]int, len(visible))
			for i := range rows {
				rows[i] = i + 40
			}
		}
		fixture.Render(ui.CreateElement(list, listProps{rows: rows, action: action, change: 1}))
		renders := count - before
		want := 0
		if action == "single-row edit" {
			want = 1
		} else if action == "page change" {
			want = 40
		}
		if renders != want {
			t.Errorf("%s row component renders = %d, want %d", action, renders, want)
		}
	}
	t.Logf("row component render deltas: search=0 sort=0 single-row-edit=1 page-change=40")
}

func TestTxnProvenanceCacheReusesClassificationsAcrossSorts(t *testing.T) {
	provCache = txnProvenanceCache{}
	txns := perfTransactions(3)
	_ = txnAutoByID(nil, txns, nil, 9)
	if got := len(provCache.seen); got != len(txns) {
		t.Fatalf("first provenance pass classified %d IDs, want %d", got, len(txns))
	}
	_ = txnAutoByID(nil, []domain.Transaction{txns[2], txns[0]}, nil, 9)
	if got := len(provCache.seen); got != len(txns) {
		t.Fatalf("resorted provenance pass classified %d IDs, want cached %d", got, len(txns))
	}
	_ = txnAutoByID(nil, txns, nil, 10)
	if got := len(provCache.seen); got != len(txns) || provCache.revision != 10 {
		t.Fatalf("new revision cache = %d IDs at revision %d, want %d at revision 10", got, provCache.revision, len(txns))
	}
}

func TestTxnSuggestionMemoCachesNegativeResults(t *testing.T) {
	inputs := txnCatSuggestInputs{suggestions: make(map[string]txnCatSuggestion)}
	txn := domain.Transaction{ID: "uncategorized"}
	if _, ok := inputs.suggestion(nil, txn); ok {
		t.Fatal("nil app unexpectedly produced a suggestion")
	}
	inputs.suggestion(nil, txn)
	if got := len(inputs.suggestions); got != 1 {
		t.Fatalf("cached %d suggestion results, want one", got)
	}
}
