// SPDX-License-Identifier: MIT

package txnfilter

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func compiledRows() []domain.Transaction {
	return []domain.Transaction{
		{ID: "a", AccountID: "a", BillAccountID: "bill", CategoryID: "food", MemberID: "m1", Source: "manual", Payee: "Raw Café", Desc: "Merchant coffee", Date: d("2026-10-01"), Amount: money.New(-1000, "USD"), Cleared: true, Tags: []string{"Coffee", "needs-review"}, Custom: map[string]any{"done": true}},
		{ID: "b", AccountID: "b", CategoryID: "other", MemberIDs: []string{"m1", "m2"}, Source: "imported", Payee: "SQ *CAFE", Desc: "Merchant #2", Date: d("2026-10-02"), Amount: money.New(-500, "EUR"), Tags: []string{"COFFEE"}, Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-500, "EUR")}}},
		{ID: "c", AccountID: "a", Source: "manual", Desc: "Merchant $10.00", Date: d("2026-10-03"), Amount: money.New(-1000, "GBP"), Tags: []string{"needs-review"}},
		{ID: "d", AccountID: "b", Source: "recurring", Desc: "Merchant payment", Date: d("2026-09-30"), Amount: money.New(25000, "USD"), Tags: []string{" coffee "}},
		{ID: "e", AccountID: "a", TransferAccountID: "b", Desc: "Movement", Date: d("2026-10-05"), Amount: money.New(-1000, "USD"), Tags: []string{"Coffee"}},
		{ID: "f", AccountID: "a", CategoryID: "food", MemberIDs: []string{"", "m2", "m2"}, Desc: "Merchant extreme", Date: d("2026-10-06"), Amount: money.New(math.MinInt64, "USD")},
		{ID: "g", AccountID: "a", CategoryID: "", Desc: "Merchant late", Date: time.Date(2026, 10, 31, 23, 0, 0, 0, time.FixedZone("west", -5*3600)), Amount: money.New(-1250, "USD")},
	}
}

func compiledLabels(trace *[]string) Labels {
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	return Labels{
		Account: map[string]string{"a": "Zebra", "b": "Apple"}, Category: map[string]string{"food": "Food", "other": "Other"}, BaseCurrency: "USD",
		Payee: func(raw string) string {
			*trace = append(*trace, "alias:"+raw)
			if raw == "SQ *CAFE" {
				return "Clean Café"
			}
			return raw
		},
		ToBase: func(value money.Money) (money.Money, error) {
			*trace = append(*trace, fmt.Sprintf("convert:%s:%d", value.Currency, value.Amount))
			return rates.ToBase(value)
		},
		OnConversionError: func(row domain.Transaction, err error) { *trace = append(*trace, "error:"+row.ID+":"+err.Error()) },
	}
}

func compiledIssueKeys(issues []ConversionIssue) []string {
	out := make([]string, len(issues))
	for i, issue := range issues {
		out[i] = issue.Transaction.ID + ":" + issue.Err.Error()
	}
	return out
}

func TestCompiledPreparationMatchesLegacyResultsIssuesAndCallbacks(t *testing.T) {
	rows := compiledRows()
	before := append([]domain.Transaction(nil), rows...)
	for i, row := range rows {
		before[i].Tags = append([]string(nil), row.Tags...)
		before[i].MemberIDs = append([]string(nil), row.MemberIDs...)
		before[i].Splits = append([]domain.CategorySplit(nil), row.Splits...)
		if row.Custom != nil {
			before[i].Custom = make(map[string]any, len(row.Custom))
			for key, value := range row.Custom {
				before[i].Custom[key] = value
			}
		}
	}
	cases := []Criteria{
		{}, {Text: "merchant"}, {Text: " Clean CAFÉ "}, {Text: "#cof"}, {Text: "$10.00"}, {Text: "10.00"}, {Text: "$"}, {Text: "12."}, {Text: "--1"},
		{Text: "$10.00", Account: "a", AmountMin: "1"}, {Text: "$10.00", Account: "missing", AmountMin: "1"},
		{Accounts: " a, b, a, ", Account: "missing"}, {Accounts: " bill ", Account: "missing"}, {Accounts: " , ", Account: "a"},
		{Categories: " food,other,food ", Category: "missing"}, {Categories: " , ", Category: "food"},
		{Members: "m2," + UnassignedMemberID, Member: "missing"}, {Members: " , ", Member: "m1"}, {Member: " m1, m2 "},
		{Sources: "manual, imported", Source: "missing"}, {Sources: " , ", Source: "manual"},
		{Tags: " coffee,NEEDS-REVIEW, coffee ", Tag: "missing"}, {Tags: " , ", Tag: "coffee"}, {Tag: " CoFFee "},
		{ScopeAny: true, Categories: " food ", Tags: " needs-review "}, {ScopeAny: true, Categories: " , ", Tags: " , "},
		{ScopeAny: true, Category: "food", Tag: "coffee"}, {ScopeAny: true},
		{From: "2026-10-01", To: "2026-10-31"}, {From: "invalid", To: "2026-02-30"},
		{Text: "$10.00", Categories: "food", From: "2026-10-01", To: "2026-10-04", AmountMax: "20"},
		{CustomKey: "done", CustomVal: "true"}, {Uncategorized: true}, {Flow: "out", Cleared: "yes"},
		{Text: "merchant", Accounts: "a,b", Categories: "food,other", Members: "m1,m2," + UnassignedMemberID, Sources: "manual,imported", Tags: "coffee,needs-review", ScopeAny: true},
	}
	for i, criteria := range cases {
		for _, sortKey := range []string{"date", "amount", "payee", "account", "category", "source"} {
			for _, direction := range []string{Asc, Desc} {
				for _, checked := range []bool{false, true} {
					t.Run(fmt.Sprintf("case-%02d/%s/%s/checked=%t", i, sortKey, direction, checked), func(t *testing.T) {
						criteria.Sort, criteria.Dir = sortKey, direction
						original := criteria
						var actualTrace, legacyTrace []string
						labels := compiledLabels(&actualTrace)
						var got []domain.Transaction
						var issues []ConversionIssue
						if checked {
							got, issues = ApplyWithLabelsChecked(rows, criteria, labels)
						} else {
							got = ApplyWithLabels(rows, criteria, labels)
						}
						want, wantIssues := legacyApply(rows, criteria, compiledLabels(&legacyTrace), checked)
						if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(compiledIssueKeys(issues), compiledIssueKeys(wantIssues)) || !reflect.DeepEqual(actualTrace, legacyTrace) {
							t.Fatalf("got IDs=%q issues=%v calls=%v; want IDs=%q issues=%v calls=%v", ids(got), compiledIssueKeys(issues), actualTrace, ids(want), compiledIssueKeys(wantIssues), legacyTrace)
						}
						if criteria != original {
							t.Fatal("filter criteria mutated")
						}
					})
				}
			}
		}
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("filter preparation mutated raw rows, owner fields, splits, tags or custom values")
	}
}

func TestCompiledPresetCountsPreserveWideningAndCallbackOrder(t *testing.T) {
	rows := compiledRows()
	scope := PresetScope{ReviewTag: "needs-review", LargeMinor: 1000, MonthStart: d("2026-10-01"), MonthEnd: d("2026-11-01")}
	for _, text := range []string{"", "merchant", "$10.00", "clean café", "#cof"} {
		for _, scopeAny := range []bool{false, true} {
			for _, ownDimension := range []Criteria{{}, {Uncategorized: true}, {Tag: "Coffee"}, {AmountMin: "100"}, {From: "2026-09-01", To: "2026-09-30"}} {
				c := ownDimension
				c.Text, c.ScopeAny, c.Accounts, c.Members, c.Categories = text, scopeAny, "a,b", "m1,m2,"+UnassignedMemberID, "food,other"
				var actualTrace, legacyTrace []string
				got := CountPresetsWithLabels(rows, c, scope, compiledLabels(&actualTrace))
				want := legacyPresetCounts(rows, c, scope, compiledLabels(&legacyTrace))
				if got != want || !reflect.DeepEqual(actualTrace, legacyTrace) {
					t.Fatalf("criteria=%+v got=%+v calls=%v, want=%+v calls=%v", c, got, actualTrace, want, legacyTrace)
				}
			}
		}
	}
}

func TestCompiledNumericSearchKeepsConversionFailureTextFallback(t *testing.T) {
	row := domain.Transaction{ID: "stateful", Desc: "10", Amount: money.New(-1000, "EUR")}
	for _, legacy := range []bool{false, true} {
		calls, reports := 0, 0
		labels := Labels{BaseCurrency: "USD", ToBase: func(value money.Money) (money.Money, error) {
			calls++
			if calls == 2 {
				return money.Money{}, fmt.Errorf("second conversion unavailable")
			}
			return money.New(value.Amount, "USD"), nil
		}, OnConversionError: func(domain.Transaction, error) { reports++ }}
		c := Criteria{Text: "10", AmountMin: "1"}
		var got []domain.Transaction
		var issues []ConversionIssue
		if legacy {
			got, issues = legacyApply([]domain.Transaction{row}, c, labels, true)
		} else {
			got, issues = ApplyWithLabelsChecked([]domain.Transaction{row}, c, labels)
		}
		if len(got) != 1 || calls != 2 || reports != 1 || len(issues) != 1 || !strings.Contains(issues[0].Err.Error(), "second conversion unavailable") {
			t.Fatalf("legacy=%t rows=%v calls=%d reports=%d issues=%v", legacy, ids(got), calls, reports, issues)
		}
	}
}

func TestCompiledTagFacetsPreserveStoredWhitespaceAndCase(t *testing.T) {
	rows := []domain.Transaction{{ID: "trimmed", Tags: []string{"CoFFee"}}, {ID: "spaces", Tags: []string{" coffee "}}}
	if got := Apply(rows, Criteria{Tags: " coffee,COFFEE "}); ids(got) != "trimmed" {
		t.Fatalf("exact tag facet changed whitespace/case semantics: %q", ids(got))
	}
	if got := Apply(rows, Criteria{Text: "#coffee"}); len(got) != 2 {
		t.Fatal("search must continue trimming stored tags while exact facets do not")
	}
}
