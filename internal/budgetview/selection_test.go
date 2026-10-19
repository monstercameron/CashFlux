package budgetview

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
func usd() currency.Rates                      { return currency.Rates{Base: "USD"} }
func budget(id string, amount int64) domain.Budget {
	return domain.Budget{ID: id, Name: id, CategoryID: "c", Period: domain.PeriodMonthly, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Limit: money.New(amount, "USD")}
}
func tr(key string, args ...any) string { return fmt.Sprint(append([]any{key}, args...)...) }

func TestSelectionAndDraft(t *testing.T) {
	cats := []domain.Category{{ID: "a", Kind: domain.KindExpense}, {ID: "b", Kind: domain.KindExpense}, {ID: "i", Kind: domain.KindIncome}}
	b := budget("b", 10)
	b.CategoryIDs = []string{"b", "a"}
	b.TrackedTags = []string{" X ", "x", ""}
	selected, tags := SelectionSeed(b)
	if !reflect.DeepEqual(selected, map[string]bool{"a": true, "b": true}) || !reflect.DeepEqual(tags, map[string]bool{"x": true}) {
		t.Fatal(selected, tags)
	}
	if got, ok := FindBudget([]domain.Budget{b, budget("b", 50)}, "b"); !ok || got.Limit.Amount != 10 {
		t.Fatal(got, ok)
	}
	if _, ok := FindBudget(nil, "b"); ok {
		t.Fatal("found missing budget")
	}
	for _, tc := range []struct {
		name         string
		picked, tags map[string]bool
		ids          []string
		primary      string
		wantTags     []string
		valid        bool
	}{
		{"empty", nil, nil, nil, "", nil, false},
		{"stale", map[string]bool{"missing": true}, nil, nil, "", nil, false},
		{"single", map[string]bool{"b": true, "i": true}, nil, nil, "b", nil, true},
		{"multi", selected, nil, []string{"a", "b"}, "a", nil, true},
		{"tags", nil, map[string]bool{"z": true, "a": true, "": true, "off": false}, nil, "", []string{"a", "z"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := TrackingDraft(b, cats, tc.picked, tc.tags)
			if ok != tc.valid {
				t.Fatal(ok)
			}
			if ok && (got.CategoryID != tc.primary || !reflect.DeepEqual(got.CategoryIDs, tc.ids) || !reflect.DeepEqual(got.TrackedTags, tc.wantTags)) {
				t.Fatalf("%+v", got)
			}
		})
	}
	for _, tc := range []struct {
		p    map[string]bool
		key  string
		tag  bool
		want int
	}{
		{nil, "a", false, 1}, {selected, "a", false, 1}, {tags, " X ", true, 0}, {nil, " ", true, 0}, {nil, "", false, 0},
	} {
		got := ToggleSelection(tc.p, tc.key, tc.tag)
		if SelectionCount(got) != tc.want {
			t.Fatal(got)
		}
		if got != nil {
			got["private"] = true
		}
		if tc.p["private"] {
			t.Fatal("mutated input")
		}
	}
}

func TestCategoryAndTagOptions(t *testing.T) {
	cats := []domain.Category{{ID: "parent", Name: "Auto", Kind: domain.KindExpense}, {ID: "gas", Name: "Gas", ParentID: "parent", Kind: domain.KindExpense}, {ID: "i", Name: "Income", Kind: domain.KindIncome}}
	b := budget("b", 0)
	b.CategoryID = "gas"
	b.Name = "Travel"
	for _, tc := range []struct {
		query     string
		saving    bool
		exclude   string
		n, hidden int
	}{{"", false, "", 2, 1}, {" AUTO ", false, "", 1, 1}, {"gas", false, "", 1, 1}, {"income", false, "", 0, 1}, {"income", true, "", 1, 0}, {"gas", false, "b", 1, 1}} {
		got, hidden := CategoryOptions(cats, []domain.Budget{b, budget("c", 0)}, tc.exclude, tc.query, tc.saving)
		if len(got) != tc.n || hidden != tc.hidden {
			t.Fatal(tc, got, hidden)
		}
		if tc.query == "gas" && tc.exclude == "" && got[0].AlsoIn != "Travel" {
			t.Fatal(got)
		}
	}
	tags := DistinctTags([]domain.Transaction{{Tags: []string{" Z ", "a", "z", "", " A "}}})
	if !reflect.DeepEqual(tags, []string{"a", "z"}) {
		t.Fatal(tags)
	}
	if DistinctTags(nil) != nil {
		t.Fatal("non-nil empty tags")
	}
	for _, tc := range []struct {
		query string
		shown []string
		exact bool
	}{{"", []string{"a", "custom", "z"}, false}, {" A ", []string{"a"}, true}, {"u", []string{"custom"}, false}, {"new", nil, false}} {
		got, q, exact := TagOptions([]string{"z", "a", "a"}, map[string]bool{"custom": true, "off": false}, tc.query)
		if !reflect.DeepEqual(got, tc.shown) || q != strings.ToLower(strings.TrimSpace(tc.query)) || exact != tc.exact {
			t.Fatal(got, q, exact)
		}
	}
	for _, tc := range []struct {
		n    int
		want string
	}{{0, ""}, {-1, ""}, {2, "2 \u00b7 $1"}} {
		if got := CountTotalLabel(tc.n, "$1"); got != tc.want {
			t.Fatal(got)
		}
	}
	if CheckedRowClass(true) != "budgetcat-row is-on" || CheckedRowClass(false) != "budgetcat-row" {
		t.Fatal("row class")
	}
}

func TestTrackingMetaAndFormatting(t *testing.T) {
	now := day(2026, 10, 19)
	txns := budgeting.NewNettedLedger([]domain.Transaction{
		{CategoryID: "a", Date: now, Amount: money.New(-100, "USD"), Tags: []string{"T"}},
		{CategoryID: "a", Date: now, Amount: money.New(-100, "EUR"), Tags: []string{"t"}},
		{CategoryID: "a", Date: day(2026, 9, 30), Amount: money.New(-200, "USD")},
	})
	for _, tc := range []struct {
		from, to time.Time
		total    string
		n        int
	}{{time.Time{}, time.Time{}, "1 \u00b7 $1.00", 1}, {day(2026, 9, 1), day(2026, 10, 1), "1 \u00b7 $2.00", 0}, {day(2026, 10, 1), time.Time{}, "1 \u00b7 $1.00", 1}} {
		got := TrackingMeta(txns, tc.from, tc.to, now, "USD", usd())
		if got.Categories["a"] != tc.total || got.TagCount["t"] != tc.n {
			t.Fatal(got)
		}
		if tc.n > 0 && got.TagTotal["t"] != "$1.00" {
			t.Fatal(got)
		}
	}
	empty := TrackingMeta(nil, now, now, now, "USD", usd())
	if len(empty.Categories) != 0 || empty.TagCount == nil {
		t.Fatal(empty)
	}
	for _, tc := range []struct {
		m    money.Money
		want string
	}{{money.New(0, "USD"), "$0.00"}, {money.New(-123456, "USD"), "($1,234.56)"}, {money.New(123, "JPY"), "\u00a5123"}} {
		if got := FormatMoney(tc.m); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
	if FormatMoney(money.New(math.MinInt64, "USD")) == "" {
		t.Fatal("minimum amount")
	}
	if BaseCurrency("") != "USD" || BaseCurrency("EUR") != "EUR" {
		t.Fatal("currency fallback")
	}
}

func FuzzSelectionToggle(t *testing.F) {
	t.Add("a", true)
	t.Add("", false)
	t.Fuzz(func(t *testing.T, key string, on bool) {
		initial := map[string]bool{key: on}
		once := ToggleSelection(initial, key, false)
		twice := ToggleSelection(once, key, false)
		if !reflect.DeepEqual(twice, initial) {
			t.Fatal(initial, twice)
		}
	})
}

func TestHugeTagInput(t *testing.T) {
	txns := make([]domain.Transaction, 20000)
	for i := range txns {
		txns[i].Tags = []string{fmt.Sprintf("tag%05d", i%1000), "common"}
	}
	got := DistinctTags(txns)
	if len(got) != 1001 || got[0] != "common" {
		t.Fatal(len(got))
	}
}

func FuzzDistinctTagsCanonical(f *testing.F) {
	f.Add(" Tag ,tag, Other, ")
	f.Fuzz(func(t *testing.T, raw string) {
		tags := DistinctTags([]domain.Transaction{{Tags: strings.Split(raw, ",")}})
		if !sort.StringsAreSorted(tags) {
			t.Fatal(tags)
		}
		for i, tag := range tags {
			if tag == "" || tag != strings.ToLower(strings.TrimSpace(tag)) || (i > 0 && tags[i-1] == tag) {
				t.Fatal(tags)
			}
		}
		if got := DistinctTags([]domain.Transaction{{Tags: tags}}); !reflect.DeepEqual(got, tags) {
			t.Fatal(got, tags)
		}
	})
}
