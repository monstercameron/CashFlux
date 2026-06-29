// SPDX-License-Identifier: MIT

package widgetengine

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(n int64) money.Money { return money.New(n, "USD") }

func mustDate(s string) time.Time {
	t, err := dateutil.ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

// TestHydrateKPI verifies a ScalarBind hydrates to a formatted figure plus a
// templated sub-label, both evaluated over the variable surface.
func TestHydrateKPI(t *testing.T) {
	sc := Scope{Vars: map[string]float64{"net_worth": 114314.04, "accounts": 14}, Base: "USD"}
	view, err := HydrateKPI(&domain.ScalarBind{
		Expr:   "net_worth",
		Format: "currency",
		Sub:    "{{accounts|plural:account}}",
	}, sc)
	if err != nil {
		t.Fatalf("HydrateKPI: %v", err)
	}
	if view.Value != 114314.04 {
		t.Errorf("value = %v, want 114314.04", view.Value)
	}
	if view.Text == "" || view.Text == "—" {
		t.Errorf("text = %q, want a formatted currency figure", view.Text)
	}
	if view.Sub != "14 accounts" {
		t.Errorf("sub = %q, want %q", view.Sub, "14 accounts")
	}

	if _, err := HydrateKPI(nil, sc); err == nil {
		t.Error("nil binding: want error")
	}
	if _, err := HydrateKPI(&domain.ScalarBind{Expr: "this is not valid +"}, sc); err == nil {
		t.Error("bad formula: want error")
	}
}

// TestRenderTemplate covers literals, string tokens, every format verb, graceful
// degradation, and no-token fast paths.
func TestRenderTemplate(t *testing.T) {
	sc := Scope{
		Vars: map[string]float64{
			"income": 6982, "savings_rate": 23, "cashflow_net": 1659.33,
			"income_count": 3, "expense_count": 1, "delta": -200, "zero": 0,
		},
		Strs: map[string]string{"period": "Jun 2026"},
		Base: "USD",
	}
	cases := []struct{ tmpl, want string }{
		{"", ""},
		{"plain text", "plain text"},
		{"{{period}}", "Jun 2026"},                               // string token
		{"{{savings_rate|percent}} saved", "23% saved"},          // percent
		{"{{income|currency}} in", "$6,982.00 in"},               // currency
		{"{{income}} count", "6982 count"},                       // default number
		{"cash flow {{cashflow_net|signed}}", "cash flow +$1,659.33"}, // signed +
		{"{{delta|signed}}", "-$200.00"},                         // signed -
		{"{{income_count|plural:deposit}}", "3 deposits"},        // plural many
		{"{{expense_count|plural:transaction}}", "1 transaction"}, // plural one
		{"{{savings_rate|arrow}} up", "▲ up"},                    // arrow +
		{"{{delta|arrow}} down", "▼ down"},                       // arrow -
		{"{{zero|arrow}}none", "none"},                           // arrow 0 → ""
		{"{{nonexistent + }}", "—"},                              // bad expr → graceful
		{"a {{unterminated", "a {{unterminated"},
	}
	for _, c := range cases {
		if got := RenderTemplate(c.tmpl, sc); got != c.want {
			t.Errorf("RenderTemplate(%q) = %q, want %q", c.tmpl, got, c.want)
		}
	}
}

func budgetFixtures() (budgets []domain.Budget, cats []domain.Category, txns []domain.Transaction) {
	cats = []domain.Category{{ID: "food", Name: "Food"}, {ID: "rent", Name: "Rent"}, {ID: "fun", Name: "Fun"}}
	budgets = []domain.Budget{
		{Name: "Food", CategoryID: "food", Scope: domain.ScopeShared, Limit: usd(10000)}, // 90% near
		{Name: "Rent", CategoryID: "rent", Scope: domain.ScopeShared, Limit: usd(10000)}, // 200% over
		{Name: "Fun", CategoryID: "fun", Scope: domain.ScopeShared, Limit: usd(10000)},   // 10% ok
	}
	txns = []domain.Transaction{
		{Amount: usd(-9000), CategoryID: "food", Date: mustDate("2026-06-03")},
		{Amount: usd(-20000), CategoryID: "rent", Date: mustDate("2026-06-04")},
		{Amount: usd(-1000), CategoryID: "fun", Date: mustDate("2026-06-05")},
	}
	return
}

// TestHydrateFrameCollectionAndTransforms verifies a Pipeline resolves a collection
// source and applies filter/sort/limit transforms in order.
func TestHydrateFrameCollectionAndTransforms(t *testing.T) {
	budgets, cats, txns := budgetFixtures()
	dc := DataCtx{
		Budgets: budgets, Categories: cats, Transactions: txns,
		Rates: currency.Rates{Base: "USD"}, Start: mustDate("2026-06-01"), End: mustDate("2026-07-01"),
	}

	// Raw source: 3 budgets.
	all, err := HydrateFrame(&domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "budgets"}}, dc)
	if err != nil {
		t.Fatalf("HydrateFrame: %v", err)
	}
	if all.Rows != 3 {
		t.Fatalf("source rows = %d, want 3", all.Rows)
	}

	// filter atrisk → sort -percent → limit 1 should yield Rent (200%, over).
	p := &domain.Pipeline{
		Source: domain.Source{Kind: domain.SourceCollection, Collection: "budgets"},
		Transform: []domain.Transform{
			{Kind: domain.TransformFilter, Arg: "atrisk"},
			{Kind: domain.TransformSort, Arg: "-percent"},
			{Kind: domain.TransformLimit, N: 1},
		},
	}
	fr, err := HydrateFrame(p, dc)
	if err != nil {
		t.Fatalf("HydrateFrame pipeline: %v", err)
	}
	if fr.Rows != 1 {
		t.Fatalf("piped rows = %d, want 1", fr.Rows)
	}
	nameCol, _ := fr.Column("name")
	if got := nameCol.Str(0); got != "Rent" {
		t.Errorf("top at-risk = %q, want Rent", got)
	}
}

// TestHydrateFrameFilterEquality verifies the "<col>=<val>" and "!=" filter forms.
func TestHydrateFrameFilterEquality(t *testing.T) {
	budgets, cats, txns := budgetFixtures()
	dc := DataCtx{Budgets: budgets, Categories: cats, Transactions: txns,
		Rates: currency.Rates{Base: "USD"}, Start: mustDate("2026-06-01"), End: mustDate("2026-07-01")}

	over, err := HydrateFrame(&domain.Pipeline{
		Source:    domain.Source{Kind: domain.SourceCollection, Collection: "budgets"},
		Transform: []domain.Transform{{Kind: domain.TransformFilter, Arg: "state=over"}},
	}, dc)
	if err != nil || over.Rows != 1 {
		t.Fatalf("state=over rows = %d err=%v, want 1", over.Rows, err)
	}
	notOk, err := HydrateFrame(&domain.Pipeline{
		Source:    domain.Source{Kind: domain.SourceCollection, Collection: "budgets"},
		Transform: []domain.Transform{{Kind: domain.TransformFilter, Arg: "state!=ok"}},
	}, dc)
	if err != nil || notOk.Rows != 2 {
		t.Fatalf("state!=ok rows = %d err=%v, want 2", notOk.Rows, err)
	}
}

// TestHydrateFrameSeries verifies the series source resolves a net-worth chart Frame.
func TestHydrateFrameSeries(t *testing.T) {
	dc := DataCtx{
		Accounts: []domain.Account{{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}},
		Rates:    currency.Rates{Base: "USD"},
		Now:      mustDate("2026-06-15"),
	}
	fr, err := HydrateFrame(&domain.Pipeline{
		Source: domain.Source{Kind: domain.SourceSeries, Series: domain.SeriesSpec{Metric: "networth", Months: 6}},
	}, dc)
	if err != nil {
		t.Fatalf("HydrateFrame series: %v", err)
	}
	if fr.Rows != 6 {
		t.Fatalf("series rows = %d, want 6", fr.Rows)
	}
	if _, ok := fr.Column("value"); !ok {
		t.Error("series frame missing value column")
	}
}

// TestHydrateFrameErrors verifies unknown sources/transforms and the explicitly
// unsupported aggregate/paginate steps error rather than silently passing through.
func TestHydrateFrameErrors(t *testing.T) {
	dc := DataCtx{Rates: currency.Rates{Base: "USD"}}
	if _, err := HydrateFrame(nil, dc); err == nil {
		t.Error("nil pipeline: want error")
	}
	if _, err := HydrateFrame(&domain.Pipeline{Source: domain.Source{Kind: domain.SourceCollection, Collection: "nope"}}, dc); err == nil {
		t.Error("unknown collection: want error")
	}
	if _, err := HydrateFrame(&domain.Pipeline{
		Source:    domain.Source{Kind: domain.SourceCollection, Collection: "accounts"},
		Transform: []domain.Transform{{Kind: domain.TransformAggregate, Arg: "sum"}},
	}, dc); err == nil {
		t.Error("aggregate: want unimplemented error")
	}
	if _, err := HydrateFrame(&domain.Pipeline{
		Source:    domain.Source{Kind: domain.SourceCollection, Collection: "accounts"},
		Transform: []domain.Transform{{Kind: domain.TransformPaginate, N: 1}},
	}, dc); err == nil {
		t.Error("paginate: want unimplemented error")
	}
}
