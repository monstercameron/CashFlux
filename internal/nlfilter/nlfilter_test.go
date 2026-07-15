// SPDX-License-Identifier: MIT

package nlfilter

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// testCtx is a fixed vocabulary + clock (mid-June 2026) so the relative date
// words have deterministic expected ranges.
func testCtx() Context {
	return Context{
		Now:       time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
		WeekStart: time.Monday,
		Categories: []NameID{
			{Name: "Coffee", ID: "cat-coffee"},
			{Name: "Coffee Shops", ID: "cat-coffee-shops"},
			{Name: "Groceries", ID: "cat-groceries"},
		},
		Tags:   []string{"business", "tax"},
		Payees: []string{"Amazon", "Starbucks"},
		ResolvePayee: func(raw string) string {
			if raw == "amzn" {
				return "Amazon"
			}
			return raw
		},
	}
}

func TestParse(t *testing.T) {
	ctx := testCtx()
	tests := []struct {
		name string
		q    string
		want txnfilter.Criteria
		ok   bool
	}{
		{"empty", "", txnfilter.Criteria{}, false},
		{"whitespace", "   ", txnfilter.Criteria{}, false},
		{"plain text only", "lunch run", txnfilter.Criteria{Text: "lunch run"}, false},
		{"over dollars", "over $20", txnfilter.Criteria{AmountMin: "20"}, true},
		{"gt symbol glued", ">$50", txnfilter.Criteria{AmountMin: "50"}, true},
		{"gt symbol number", "> 30", txnfilter.Criteria{AmountMin: "30"}, true},
		{"under plain", "under 50", txnfilter.Criteria{AmountMax: "50"}, true},
		{"below decimal", "below 12.50", txnfilter.Criteria{AmountMax: "12.50"}, true},
		{"more than", "more than 100", txnfilter.Criteria{AmountMin: "100"}, true},
		{"less than", "less than 5", txnfilter.Criteria{AmountMax: "5"}, true},
		{"between and", "between 10 and 50", txnfilter.Criteria{AmountMin: "10", AmountMax: "50"}, true},
		{"thousands separator", "over $1,000", txnfilter.Criteria{AmountMin: "1000"}, true},
		{"this month", "this month", txnfilter.Criteria{From: "2026-06-01", To: "2026-06-30"}, true},
		{"last month", "last month", txnfilter.Criteria{From: "2026-05-01", To: "2026-05-31"}, true},
		{"this year", "this year", txnfilter.Criteria{From: "2026-01-01", To: "2026-12-31"}, true},
		{"yesterday", "yesterday", txnfilter.Criteria{From: "2026-06-14", To: "2026-06-14"}, true},
		{"today", "today", txnfilter.Criteria{From: "2026-06-15", To: "2026-06-15"}, true},
		{"in june", "in june", txnfilter.Criteria{From: "2026-06-01", To: "2026-06-30"}, true},
		{"bare month with year", "june 2025", txnfilter.Criteria{From: "2025-06-01", To: "2025-06-30"}, true},
		{"since march", "since march", txnfilter.Criteria{From: "2026-03-01"}, true},
		{"cleared", "cleared", txnfilter.Criteria{Cleared: "yes"}, true},
		{"uncleared word", "uncleared", txnfilter.Criteria{Cleared: "no"}, true},
		{"not cleared", "not cleared", txnfilter.Criteria{Cleared: "no"}, true},
		{"pending", "pending", txnfilter.Criteria{Cleared: "no"}, true},
		{"spent flow", "spent", txnfilter.Criteria{Flow: "out"}, true},
		{"received flow", "received", txnfilter.Criteria{Flow: "in"}, true},
		{"category word", "groceries", txnfilter.Criteria{Categories: "cat-groceries"}, true},
		{"longest category wins", "coffee shops", txnfilter.Criteria{Categories: "cat-coffee-shops"}, true},
		{"tag word", "business", txnfilter.Criteria{Tags: "business"}, true},
		{"payee canonicalized text", "amzn", txnfilter.Criteria{Text: "Amazon"}, false},
		{"garbage passthrough", "asdf qwerty", txnfilter.Criteria{Text: "asdf qwerty"}, false},
		{
			"mixed clause",
			"coffee last month over $20",
			txnfilter.Criteria{Categories: "cat-coffee", From: "2026-05-01", To: "2026-05-31", AmountMin: "20"},
			true,
		},
		{
			"mixed with residue text",
			"lunch spent over 15 this month",
			txnfilter.Criteria{Flow: "out", AmountMin: "15", From: "2026-06-01", To: "2026-06-30", Text: "lunch"},
			true,
		},
		{
			"payee plus amount",
			"starbucks over 5",
			txnfilter.Criteria{AmountMin: "5", Text: "Starbucks"},
			true,
		},
		{
			"range and category",
			"groceries between 20 and 80",
			txnfilter.Criteria{Categories: "cat-groceries", AmountMin: "20", AmountMax: "80"},
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Parse(tc.q, ctx)
			if ok != tc.ok {
				t.Errorf("ok = %v, want %v", ok, tc.ok)
			}
			if got != tc.want {
				t.Errorf("Parse(%q)\n  got  %+v\n  want %+v", tc.q, got, tc.want)
			}
		})
	}
}

// TestParseThisWeek checks the week bounds against dateutil so the expectation
// tracks the same week math the parser uses.
func TestParseThisWeek(t *testing.T) {
	ctx := testCtx()
	ws := dateutil.WeekStart(ctx.Now, time.Monday)
	got, ok := Parse("this week", ctx)
	if !ok {
		t.Fatal("expected ok")
	}
	if got.From != dateutil.FormatDate(ws) || got.To != dateutil.FormatDate(ws.AddDate(0, 0, 6)) {
		t.Errorf("this week = %s..%s, want %s..%s", got.From, got.To,
			dateutil.FormatDate(ws), dateutil.FormatDate(ws.AddDate(0, 0, 6)))
	}
}

// TestParseZeroContextStillParses confirms an empty Context (no vocabulary, no
// clock) still recognizes amounts and dates.
func TestParseZeroContextStillParses(t *testing.T) {
	got, ok := Parse("over $40", Context{})
	if !ok || got.AmountMin != "40" {
		t.Errorf("got %+v ok=%v, want AmountMin=40 ok=true", got, ok)
	}
}
