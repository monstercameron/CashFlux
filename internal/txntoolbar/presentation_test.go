// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestChipLabelEveryDimension(t *testing.T) {
	names := Indexes{Accounts: map[string]string{"a": "Checking"}, Categories: map[string]string{"c": "Dining"}, Members: map[string]string{"m": "Cam"}}
	for _, tt := range []struct {
		field       txnfilter.FilterField
		value, want string
	}{
		{txnfilter.FieldText, "coffee", "transactions.chipFilteringcoffee"},
		{txnfilter.FieldAccount, "a", "transactions.chipAccountChecking"},
		{txnfilter.FieldCategory, "c", "transactions.chipCategoryDining"},
		{txnfilter.FieldMember, "m", "transactions.chipMemberCam"},
		{txnfilter.FieldSource, "manual", "transactions.chipSource"},
		{txnfilter.FieldTag, "tag", "transactions.chipTagtag"},
		{txnfilter.FieldAmountMin, "1", "transactions.chipAmountBound"},
		{txnfilter.FieldAmountMax, "2", "transactions.chipAmountBound"},
		{txnfilter.FieldFrom, "2026-10-01", "transactions.chipFrom2026-10-01"},
		{txnfilter.FieldTo, "2026-10-31", "transactions.chipTo2026-10-31"},
		{txnfilter.FieldCleared, "yes", "transactions.cleared"},
		{txnfilter.FieldCleared, "no", "transactions.notCleared"},
		{txnfilter.FieldFlow, "in", "transactions.chipFlowIn"},
		{txnfilter.FieldFlow, "out", "transactions.chipFlowOut"},
		{txnfilter.FieldUncategorized, "1", "transactions.chipUncategorized"},
		{txnfilter.FieldCustom, "blue", "transactions.chipCustomProject blue"},
		{txnfilter.FilterField("unknown"), "raw", "raw"},
	} {
		if got := presentation.FilterChipLabel(txnfilter.ActiveFilter{Field: tt.field, Value: tt.value}, names, "USD", "Project"); !strings.HasPrefix(got, tt.want) {
			t.Fatalf("%s=%q want prefix %q", tt.field, got, tt.want)
		}
	}
	if got := presentation.FilterChipLabel(txnfilter.ActiveFilter{Field: txnfilter.FieldAccount, Value: "missing"}, Indexes{}, "USD", ""); got != "transactions.chipAccount" {
		t.Fatal("nil label map", got)
	}
}

func TestCategoryLayoutReconcilesAndPreservesSelection(t *testing.T) {
	categories := []CategoryTotal{{ID: "", Minor: 25}, {ID: "a", Minor: 25}, {ID: "b", Minor: 25}, {ID: "__other__", Minor: 25}, {ID: "d", Minor: 0}, {ID: "e", Minor: -5}}
	original := append([]CategoryTotal(nil), categories...)
	for _, selected := range []string{"", "a", "__uncategorized__"} {
		got := presentation.CategoryLayout(categories, selected)
		if len(got.All) != 6 || len(got.Visible) != 3 || got.More != 3 || got.Visible[2].Category.ID != "__other__" {
			t.Fatalf("layout=%+v", got)
		}
		if selected == "a" && (!got.All[1].Selected || !got.All[0].Dimmed) {
			t.Fatal("ordinary selection")
		}
		if selected == "__uncategorized__" && (!got.All[0].Selected || !got.All[1].Dimmed) {
			t.Fatal("uncategorized selection")
		}
		if got.All[0].Category.Share != 25 || got.All[5].Category.Share != 0 {
			t.Fatal("category share")
		}
	}
	if !reflect.DeepEqual(categories, original) {
		t.Fatal("layout mutated category input")
	}
	if got := presentation.CategoryLayout(nil, ""); len(got.All) != 0 || len(got.Visible) != 0 || got.More != 0 {
		t.Fatal(got)
	}
	if got := presentation.CategoryLayout([]CategoryTotal{{Minor: math.MaxInt64}, {Minor: 1}}, ""); got.All[0].Category.Share != 0 {
		t.Fatal("overflow exposed percentage")
	}
}

func TestCategoryAndFlowTransitions(t *testing.T) {
	original := txnfilter.Criteria{Text: "search", Category: "a", Flow: "out"}
	if got := ChooseCategory(original, "a"); got.Category != "" || got.Text != "search" {
		t.Fatal(got)
	}
	if got := ChooseCategory(original, "b"); got.Category != "b" {
		t.Fatal(got)
	}
	if got := ChooseCategory(original, ""); !got.Uncategorized || got.Category != "" {
		t.Fatal(got)
	}
	if got := ChooseCategory(txnfilter.Criteria{Uncategorized: true}, ""); got.Uncategorized {
		t.Fatal(got)
	}
	if got := ToggleFlow(original, "out"); got.Flow != "" || got.Category != "a" {
		t.Fatal(got)
	}
	if got := ToggleFlow(original, "in"); got.Flow != "in" {
		t.Fatal(got)
	}
}

func TestSummaryFactsAndWidening(t *testing.T) {
	facts := presentation.SummaryFacts(Facts{Shown: 2, MovedRows: 1, UnpairedRows: 1, Unconverted: 1, Excluded: 1, Moved: money.New(100, "USD"), Issues: []txnfilter.ConversionIssue{{}}, Lens: "Cam"})
	if len(facts) != 7 || facts[0] != "r3n.transactionCountMany2" || facts[6] != "transactions.scopeLensCam" {
		t.Fatal(facts)
	}
	if got := presentation.SummaryFacts(Facts{Shown: 1}); len(got) != 1 || got[0] != "r3n.transactionCountOne1" {
		t.Fatal(got)
	}
	if got := presentation.SummaryFacts(Facts{MovedRows: 1}); len(got) != 2 {
		t.Fatal(got)
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("local", -7*3600))
	for _, tt := range []struct {
		f    txnfilter.Criteria
		want bool
	}{
		{txnfilter.Criteria{}, false},
		{txnfilter.Criteria{Text: "   ", PeriodPreset: txnfilter.PeriodThisMonth}, false},
		{txnfilter.Criteria{Text: "coffee", PeriodPreset: txnfilter.PeriodCustom}, false},
		{txnfilter.Criteria{Text: "coffee", PeriodPreset: txnfilter.PeriodAllDates}, false},
		{txnfilter.Criteria{Text: "coffee", From: "2026-10-01", To: "2026-10-31"}, true},
		{txnfilter.Criteria{Text: "coffee", PeriodPreset: txnfilter.PeriodThisMonth}, true},
	} {
		got, ok := WidenSearch(tt.f, now)
		if ok != tt.want {
			t.Fatal(tt, got, ok)
		}
		if ok && (got.From != "" || got.To != "" || got.PeriodPreset != txnfilter.PeriodAllDates || got.Text != tt.f.Text) {
			t.Fatal(got)
		}
	}
}

func TestPickerSearchAndKeyboardTransitions(t *testing.T) {
	options := []PickerOption{{Label: "placeholder"}, {Value: "a", Label: "Dining"}, {Value: "b", Label: "Car"}}
	for _, tt := range []struct {
		query   string
		create  bool
		want    int
		created bool
	}{{"", false, 2, false}, {" din ", false, 1, false}, {"missing", false, 0, false}, {"missing", true, 1, true}} {
		got := PickerOptions(options, tt.query, tt.create, func(name string) string { return "Create " + name })
		if len(got) != tt.want {
			t.Fatal(got)
		}
		if tt.created && (got[0].Value != "__create_category__" || !got[0].Create || got[0].Label != "Create missing") {
			t.Fatal(got)
		}
	}
	for _, tt := range []struct{ index, count, step, want int }{{0, 0, 1, 0}, {-1, 3, 0, 0}, {9, 3, 0, 0}, {1, 3, 0, 1}, {1, 3, 1, 2}, {2, 3, 1, 0}, {0, 3, -1, 2}, {1, 3, -1, 0}} {
		if got := PickerIndex(tt.index, tt.count, tt.step); got != tt.want {
			t.Fatal(tt, got)
		}
	}
}

func TestSnapshotRequiresExactRevisionAndScope(t *testing.T) {
	var nilSnapshot *Snapshot
	if nilSnapshot.Matches(1, "scope") {
		t.Fatal("nil snapshot matched")
	}
	snapshot := Snapshot{Revision: 1, Scope: "app/workspace/filter/currency"}
	if !snapshot.Matches(1, snapshot.Scope) || snapshot.Matches(2, snapshot.Scope) || snapshot.Matches(1, "other-workspace") {
		t.Fatal("snapshot identity not checked")
	}
}

func TestPageRangeDoesNotOverflowOnHugeInput(t *testing.T) {
	for _, size := range []int{1, 2, 100, math.MaxInt} {
		got := presentation.PageRange(math.MaxInt, size, math.MaxInt)
		if strings.Contains(got, "-") {
			t.Fatal("range overflow", got)
		}
	}
}

func TestChipAndPickerProjections(t *testing.T) {
	active := []txnfilter.ActiveFilter{{Field: txnfilter.FieldFrom, Value: "2026-10-01"}, {Field: txnfilter.FieldText, Value: "coffee"}, {Field: txnfilter.FieldTo, Value: "2026-10-31"}}
	got := ActiveChips(active, func(f txnfilter.ActiveFilter) string { return f.Value })
	if len(got) != 1 || got[0].Key != string(txnfilter.FieldText)+"\x1fcoffee" || got[0].Label != "coffee" {
		t.Fatal(got)
	}
	if len(ActiveChips(nil, nil)) != 0 {
		t.Fatal("nil chips")
	}
	categories := []domain.Category{{ID: "parent", Name: "Home"}, {ID: "child", Name: "Rent", ParentID: "parent"}}
	if got := CategoryLabel(categories, "child"); !strings.Contains(got, "Home") || !strings.Contains(got, "Rent") {
		t.Fatal(got)
	}
	if CategoryLabel(nil, "missing") != "" {
		t.Fatal("missing category")
	}
	owners := OwnerOptions([]domain.Member{{ID: "m", Name: "Cam"}}, "Unassigned")
	if len(owners) != 2 || owners[0].Value != UnassignedOwner || owners[0].Label != "Unassigned" || owners[1].Label != "Cam" || len(OwnerOptions(nil, "Unassigned")) != 1 {
		t.Fatal(owners)
	}
}
