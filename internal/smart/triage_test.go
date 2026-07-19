// SPDX-License-Identifier: MIT

package smart

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func triageFixture() []Insight {
	return []Insight{
		{Key: "a", Page: PageAccounts, Title: "Stale balance on Checking", Detail: "not updated", Severity: SeverityNudge},
		{Key: "b", Page: PageBills, Title: "A bill looks missed", Detail: "car loan", Severity: SeverityAlert,
			Amount: money.New(30000, "USD"), HasAmount: true},
		{Key: "c", Page: PageBudgets, Title: "Groceries near its limit", Detail: "92% used", Severity: SeverityWarn,
			Amount: money.New(1500, "USD"), HasAmount: true},
		{Key: "d", Page: PageAccounts, Title: "Idle cash", Detail: "consider moving", Severity: SeverityInfo},
	}
}

func TestFilterInsights(t *testing.T) {
	in := triageFixture()
	tests := []struct {
		name  string
		query string
		sev   Severity
		page  Page
		want  []string
	}{
		{"all pass-through", "", SeverityInfo, "", []string{"a", "b", "c", "d"}},
		{"query matches title case-insensitively", "BILL", SeverityInfo, "", []string{"b"}},
		{"query matches detail", "moving", SeverityInfo, "", []string{"d"}},
		{"min severity", "", SeverityWarn, "", []string{"b", "c"}},
		{"page filter", "", SeverityInfo, PageAccounts, []string{"a", "d"}},
		{"combined", "cash", SeverityInfo, PageAccounts, []string{"d"}},
		{"no match", "zebra", SeverityInfo, "", nil},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := FilterInsights(in, tc.query, tc.sev, tc.page)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d insights, want %d", len(got), len(tc.want))
			}
			for i, w := range tc.want {
				if got[i].Key != w {
					t.Errorf("got[%d] = %q, want %q", i, got[i].Key, w)
				}
			}
		})
	}
	if len(in) != 4 {
		t.Error("input must not be mutated")
	}
}

func TestNeedsAttention(t *testing.T) {
	cases := map[Severity]bool{
		SeverityInfo:  false,
		SeverityNudge: false,
		SeverityWarn:  true,
		SeverityAlert: true,
	}
	for sev, want := range cases {
		if got := NeedsAttention(Insight{Severity: sev}); got != want {
			t.Errorf("NeedsAttention(%v) = %v, want %v", sev, got, want)
		}
	}
}

func TestDedupeInsights(t *testing.T) {
	in := []Insight{
		{Key: "a", Title: "Low balance before payday", Detail: "Checking dips to -$40"},
		{Key: "b", Title: "Low balance before payday", Detail: "Checking dips to -$40"}, // dup of a
		{Key: "c", Title: "Low balance before payday", Detail: "Savings dips to -$5"},   // same title, diff detail
		{Key: "d", Title: "Idle cash", Detail: "consider moving"},
		{Key: "e", Title: "Low balance before payday", Detail: "Checking dips to -$40"}, // dup of a
	}
	got := DedupeInsights(in)
	want := []string{"a", "c", "d"}
	if len(got) != len(want) {
		t.Fatalf("DedupeInsights returned %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Key != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i].Key, w)
		}
	}
	if len(in) != 5 {
		t.Error("input must not be mutated")
	}
}

func TestSortInsightsBy(t *testing.T) {
	byKeys := func(in []Insight) []string {
		out := make([]string, len(in))
		for i, ins := range in {
			out[i] = ins.Key
		}
		return out
	}

	in := triageFixture()
	SortInsightsBy(in, SortBySeverity)
	if got := byKeys(in); got[0] != "b" || got[1] != "c" {
		t.Errorf("severity order = %v, want alert (b) then warn (c) first", got)
	}

	in = triageFixture()
	SortInsightsBy(in, SortByAmount)
	if got := byKeys(in); got[0] != "b" || got[1] != "c" {
		t.Errorf("amount order = %v, want b ($300) then c ($15) first", got)
	}
	// Amount-less findings trail in severity order.
	in = triageFixture()
	SortInsightsBy(in, SortByAmount)
	if got := byKeys(in); got[2] != "a" || got[3] != "d" {
		t.Errorf("amount order tail = %v, want nudge (a) before info (d)", got)
	}

	in = triageFixture()
	SortInsightsBy(in, SortByPage)
	if got := byKeys(in); got[0] != "a" || got[1] != "d" {
		t.Errorf("page order = %v, want accounts first (a nudge before d info)", got)
	}
}
