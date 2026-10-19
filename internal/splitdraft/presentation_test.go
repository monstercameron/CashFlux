// SPDX-License-Identifier: MIT

package splitdraft_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
)

func TestView(t *testing.T) {
	for _, tc := range []struct {
		name               string
		amount             int64
		percent, attempted bool
		lines              []splitdraft.Line
		total, remainder   int64
		parse, save        bool
		tone, key, value   string
	}{
		{"empty", -100, false, false, nil, 0, 100, false, false, "split-remainder-neutral", "splitEditor.left", "$1.00"},
		{"amount balanced", -100, false, false, []splitdraft.Line{{Cat: "a", Amt: "0.50"}, {Cat: "b", Amt: "0.50"}}, 100, 0, false, true, "pos", "splitEditor.balanced", ""},
		{"percent balanced", -100, true, true, []splitdraft.Line{{Cat: "a", Amt: "50"}, {Cat: "b", Amt: "50"}}, 10000, 0, false, true, "pos", "splitEditor.balanced", ""},
		{"percent left", -100, true, false, []splitdraft.Line{{Cat: "a", Amt: "30"}}, 3000, 7000, false, false, "split-remainder-neutral", "splitEditor.pctLeft", "70.00"},
		{"percent over", -100, true, true, []splitdraft.Line{{Cat: "a", Amt: "130"}}, 13000, -3000, false, false, "neg", "splitEditor.pctOver", "30.00"},
		{"amount over", -100, false, false, []splitdraft.Line{{Cat: "a", Amt: "2"}}, 200, -100, false, false, "split-remainder-neutral", "splitEditor.over", "$1.00"},
		{"attempted left", -100, false, true, nil, 0, 100, false, false, "neg", "splitEditor.left", "$1.00"},
		{"bad before save", -100, false, false, []splitdraft.Line{{Cat: "a", Amt: "bad"}}, 0, 100, true, false, "split-remainder-neutral", "splitEditor.left", "$1.00"},
		{"bad after save", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "bad"}}, 0, 100, true, false, "neg", "splitEditor.badAmount", ""},
		{"bad percent", -100, true, true, []splitdraft.Line{{Cat: "a", Amt: "bad"}}, 0, 10000, true, false, "neg", "splitEditor.badPercent", ""},
		{"negative", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "-1"}}, 0, 100, true, false, "neg", "splitEditor.badAmount", ""},
		{"zero line", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "0"}}, 0, 100, true, false, "neg", "splitEditor.badAmount", ""},
		{"incomplete", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "1"}, {Cat: "b"}}, 100, 0, false, false, "neg", "splitEditor.incomplete", ""},
		{"one row", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "1"}, {}}, 100, 0, false, false, "neg", "splitEditor.needTwoShort", ""},
		{"zero parent", 0, false, false, nil, 0, 0, false, false, "pos", "splitEditor.balanced", ""},
		{"minimum parent", math.MinInt64, false, true, nil, 0, 0, true, false, "neg", "splitEditor.badAmount", ""},
		{"total overflow", -100, false, true, []splitdraft.Line{{Cat: "a", Amt: "92233720368547758.07"}, {Cat: "b", Amt: "0.01"}}, math.MaxInt64, 100 - math.MaxInt64, true, false, "neg", "splitEditor.badAmount", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := splitdraft.State{Lines: tc.lines, Percent: tc.percent, SaveAttempted: tc.attempted, PickerLine: -1}
			v := s.View(transaction(tc.amount, "USD"))
			if v.Total != tc.total || v.Remainder != tc.remainder || v.ParseError != tc.parse || v.CanSave != tc.save || v.Tone != tc.tone || v.Key != tc.key || v.Value != tc.value || v.PickerOpen {
				t.Fatalf("View = %+v; want total %d remainder %d parse %v save %v text (%q,%q,%q)", v, tc.total, tc.remainder, tc.parse, tc.save, tc.tone, tc.key, tc.value)
			}
		})
	}
	for _, index := range []int{-1, 0, 1, math.MaxInt} {
		s := splitdraft.State{Lines: []splitdraft.Line{{Cat: "a", Amt: ""}}, PickerLine: index}
		v := s.View(transaction(-100, "EUR"))
		if v.PickerOpen != (index == 0) || v.Hint != "€1.00" || (index == 0 && v.PickerCategory != "a") {
			t.Fatalf("picker/hint view = %+v", v)
		}
	}
}

func TestPresentationDecisions(t *testing.T) {
	for _, tc := range []struct {
		flag                        bool
		columns, tone, amount, mode string
	}{
		{false, "minmax(180px, 1fr) 120px 32px", "split-remainder-neutral", "splitEditor.amount", "btn btn-sm"},
		{true, "minmax(180px, 1fr) 160px 120px 32px", "neg", "splitEditor.percent", "btn btn-sm btn-primary"},
	} {
		if splitdraft.Columns(tc.flag) != tc.columns || splitdraft.RemainderTone(tc.flag) != tc.tone || splitdraft.AmountKey(tc.flag) != tc.amount || splitdraft.ModeClass(tc.flag) != tc.mode {
			t.Errorf("presentation decisions for %v changed", tc.flag)
		}
	}
	for _, tc := range []struct {
		open, splits bool
		want         string
	}{
		{false, false, "splitEditor.toggle"}, {false, true, "splitEditor.edit"},
		{true, false, "splitEditor.hide"}, {true, true, "splitEditor.hide"},
	} {
		if got := splitdraft.ToggleKey(tc.open, tc.splits); got != tc.want {
			t.Errorf("ToggleKey = %q", got)
		}
	}
	for _, tc := range []struct{ value, want int64 }{
		{0, 0}, {1, 1}, {-1, 1}, {math.MaxInt64, math.MaxInt64}, {math.MinInt64, math.MinInt64},
	} {
		if got := splitdraft.Magnitude(tc.value); got != tc.want {
			t.Errorf("Magnitude(%d) = %d", tc.value, got)
		}
	}
	for _, tc := range []struct {
		amount int64
		want   domain.CategoryKind
	}{
		{0, domain.KindExpense}, {-1, domain.KindExpense}, {1, domain.KindIncome}, {math.MinInt64, domain.KindExpense},
	} {
		if got := splitdraft.CategoryKind(transaction(tc.amount, "USD")); got != tc.want {
			t.Errorf("CategoryKind = %s", got)
		}
	}
	for _, members := range [][]domain.Member{nil, {}, {{ID: "a"}}, {{ID: "a"}, {ID: "b"}}} {
		if got := splitdraft.ShowOwner(members); got != (len(members) > 0) {
			t.Errorf("ShowOwner(%v) = %v", members, got)
		}
	}
}

func TestCategoriesForKind(t *testing.T) {
	input := []domain.Category{{ID: "a", Kind: domain.KindExpense}, {ID: "b", Kind: domain.KindIncome}, {ID: "c", Kind: domain.KindExpense}}
	for _, tc := range []struct {
		name  string
		input []domain.Category
		kind  domain.CategoryKind
		ids   []string
	}{
		{"nil", nil, domain.KindExpense, nil},
		{"expense stable", input, domain.KindExpense, []string{"a", "c"}},
		{"income", input, domain.KindIncome, []string{"b"}},
		{"none", input, "unknown", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := splitdraft.CategoriesForKind(tc.input, tc.kind)
			var ids []string
			for _, c := range out {
				ids = append(ids, c.ID)
			}
			if !reflect.DeepEqual(ids, tc.ids) {
				t.Fatalf("filtered = %v, want %v", ids, tc.ids)
			}
			if len(out) > 0 {
				out[0].ID = "changed"
				if input[0].ID != "a" || input[1].ID != "b" {
					t.Fatal("filter aliased input")
				}
			}
		})
	}
}

func TestCellSummary(t *testing.T) {
	// There is deliberately no FX map: a tooltip formats each currency in place,
	// without pretending that its amounts can be added across currencies.
	for _, tc := range []struct {
		name    string
		lines   []domain.CategorySplit
		count   int
		tooltip string
	}{
		{"nil", nil, 0, ""},
		{"duplicates", []domain.CategorySplit{{CategoryID: "a", Amount: money.New(-100, "USD")}, {CategoryID: "a", Amount: money.New(-200, "USD")}}, 1, "($1.00), ($2.00)"},
		{"mixed missing FX", []domain.CategorySplit{{CategoryID: "a", Amount: money.New(-100, "USD")}, {CategoryID: "b", Amount: money.New(200, "EUR")}, {CategoryID: "b", Amount: money.New(300, "JPY")}}, 2, "($1.00), €2.00, ¥300"},
		{"unset currency", []domain.CategorySplit{{CategoryID: "", Amount: money.New(0, "")}}, 1, "0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count, tooltip := splitdraft.CellSummary(domain.Transaction{Splits: tc.lines})
			if count != tc.count || tooltip != tc.tooltip {
				t.Fatalf("summary = %d, %q; want %d, %q", count, tooltip, tc.count, tc.tooltip)
			}
		})
	}
}
