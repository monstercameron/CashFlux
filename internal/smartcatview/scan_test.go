// SPDX-License-Identifier: MIT

package smartcatview

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartai"
)

func fixtureCats() []domain.Category {
	return []domain.Category{{ID: "food", Name: "Food", Kind: domain.KindExpense}, {ID: "gas", Name: "Gas", ParentID: "food", Kind: domain.KindExpense}, {ID: "pay", Name: "Pay", Kind: domain.KindIncome}}
}
func transaction(id, cat string, amount int64) domain.Transaction {
	return domain.Transaction{ID: id, Payee: " Store ", Desc: "receipt", CategoryID: cat, Amount: money.New(amount, "USD")}
}

func TestCatalog(t *testing.T) {
	for _, tc := range []struct {
		name string
		cats []domain.Category
		want string
	}{
		{"nil", nil, ""},
		{"qualified", fixtureCats(), "Food | expense\nFood > Gas | expense\nPay | income\n"},
		{"self", []domain.Category{{ID: "a", Name: "Self", ParentID: "a"}}, "Self | expense\n"},
		{"orphan", []domain.Category{{ID: "a", Name: "Orphan", ParentID: "missing"}}, "Orphan | expense\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Catalog(tc.cats).Prompt(); got != tc.want {
				t.Fatalf("%q != %q", got, tc.want)
			}
		})
	}
}

func TestBuildScan(t *testing.T) {
	txns := []domain.Transaction{transaction("zero", "", 0), transaction("negative", "", -1), transaction("positive", "", 1), transaction("filed", "food", math.MinInt64), transaction("missing", "lost", math.MaxInt64), transaction("transfer", "", 20)}
	txns[1].Amount.Currency = "EUR"
	txns[2].Amount.Currency = "JPY"
	txns[5].TransferAccountID = "other"
	for _, tc := range []struct {
		mode   string
		want   []string
		system string
	}{
		{Suggest, []string{"zero", "negative", "positive"}, smartai.SuggestCategoriesSystem},
		{Auto, []string{"zero", "negative", "positive"}, smartai.AutoCategorizeSystem},
		{Recat, []string{"filed", "missing"}, smartai.RecategorizeSystem},
		{"unknown", []string{"zero", "negative", "positive"}, smartai.AutoCategorizeSystem},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			got := BuildScan(tc.mode, txns, fixtureCats())
			var ids []string
			for _, x := range got.Sample {
				ids = append(ids, x.ID)
			}
			if !reflect.DeepEqual(ids, tc.want) || got.Request.System != tc.system {
				t.Fatalf("%+v", got)
			}
			if tc.mode == Recat && (!strings.Contains(got.Request.User, "currently: Food") || !strings.Contains(got.Request.User, "currently: (uncategorized)")) {
				t.Fatal(got.Request.User)
			}
			if !strings.Contains(got.Request.User, "Store  \u2014 receipt") {
				t.Fatal(got.Request.User)
			}
		})
	}
	if len(BuildScan(Suggest, nil, nil).Sample) != 0 {
		t.Fatal("empty input")
	}
	huge := make([]domain.Transaction, 100000)
	for i := range huge {
		huge[i] = transaction(fmt.Sprint(i), "", -1)
	}
	got := BuildScan(Auto, huge, nil)
	if len(got.Sample) != ScanCap || got.Sample[39].ID != "39" || strings.Contains(got.Request.User, "41 |") {
		t.Fatal("cap/order")
	}
}

func TestParseAssignments(t *testing.T) {
	sample := []domain.Transaction{transaction("expense", "", -1), transaction("income", "", 1), transaction("noop", "food", -3), transaction("zero", "", 0)}
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"valid", "1 => Food | high\n2 => Pay | high\n3 => Food | high\n4 => Food | high", []string{"expense", "income", "zero"}},
		{"mismatch", "1 => Pay | high\n2 => Food | high", nil},
		{"invalid", "0 => Food\n99 => Food\n1 => Imaginary\nbroken", nil},
		{"nil", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseAssignments(tc.text, sample, fixtureCats())
			var ids []string
			for _, x := range got {
				ids = append(ids, x.TxnID)
				if x.Label != "Store  \u2014 receipt" || x.Amount == "" {
					t.Fatal(x)
				}
			}
			if !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("%+v want %v", got, tc.want)
			}
		})
	}
	if got := ParseAssignments("1 => Food", nil, nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestSuggestionsAndNewCategory(t *testing.T) {
	got := ParseSuggestions("Food | expense\nTravel | expense | FOOD\nSalary | income | absent\nTravel | income", fixtureCats())
	if len(got) != 2 || got[0].ParentName != "FOOD" || got[1].ParentName != "" {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		s      smartai.SuggestedCategory
		parent string
	}{
		{got[0], "food"}, {got[1], ""}, {smartai.SuggestedCategory{Name: "Other", Kind: "expense", ParentName: "absent"}, ""},
	} {
		c := NewCategory(tc.s, fixtureCats(), "new")
		if c.ID != "new" || c.Name != tc.s.Name || string(c.Kind) != tc.s.Kind || c.ParentID != tc.parent {
			t.Fatal(c)
		}
	}
	if len(ParseSuggestions("", nil)) != 0 {
		t.Fatal("empty")
	}
}

func TestReviewState(t *testing.T) {
	for _, mode := range []string{Suggest, Auto, Recat} {
		t.Run(mode, func(t *testing.T) {
			s := Reset(mode)
			s.Picked[2] = true
			started := s.Start()
			if !started.Loading || started.Scanned || started.Error != "" || !started.Picked[2] {
				t.Fatal(started)
			}
			failed := started.Fail("failed")
			if failed.Loading || !failed.Scanned || failed.Error != "failed" || !failed.Picked[2] {
				t.Fatal(failed)
			}
			a := []Assignment{{TxnID: "x"}}
			c := []smartai.SuggestedCategory{{Name: "New"}, {Name: "Other"}}
			done := started.Finish(mode, c, a)
			n := 1
			if mode == Suggest {
				n = 2
			}
			if done.Loading || !done.Scanned || len(done.Picked) != n || !done.Picked[0] {
				t.Fatal(done)
			}
			toggled := done.Toggle(0)
			if toggled.Picked[0] || !done.Picked[0] {
				t.Fatal("selection alias")
			}
			nilState := State{}.Toggle(-1)
			if !nilState.Picked[-1] {
				t.Fatal("nil map")
			}
		})
	}
}

func TestPresentReview(t *testing.T) {
	rows := []Assignment{{TxnID: "x", Label: "Store", CatName: "Food", Current: "Old", Amount: "$1"}}
	suggestions := []smartai.SuggestedCategory{{Name: "Food", Kind: "expense"}, {Name: "Pay", Kind: "income"}}
	for _, tc := range []struct {
		name                      string
		s                         State
		provider                  bool
		result, hint, scan, apply string
		show                      bool
		n                         int
	}{
		{"provider", State{Mode: Suggest, Loading: true}, false, "provider", "smartcat.hintSuggest", "smartcat.scan", "smartcat.createSelected", false, 0},
		{"loading", State{Mode: Auto, Loading: true, Error: "bad"}, true, "loading", "smartcat.hintAuto", "smartcat.scan", "smartcat.applySelected", false, 0},
		{"error", State{Mode: Recat, Error: "bad", Scanned: true}, true, "error", "smartcat.hintRecat", "smartcat.rescan", "smartcat.applySelected", false, 0},
		{"empty", State{Mode: Suggest, Scanned: true}, true, "empty", "smartcat.hintSuggest", "smartcat.rescan", "smartcat.createSelected", false, 0},
		{"idle", Reset(Auto), true, "", "smartcat.hintAuto", "smartcat.scan", "smartcat.applySelected", false, 0},
		{"suggestions", State{Mode: Suggest, Scanned: true, Suggestions: suggestions, Picked: map[int]bool{0: true, 1: false}}, true, "rows", "smartcat.hintSuggest", "smartcat.rescan", "smartcat.createSelected", true, 1},
		{"assignments", State{Mode: Recat, Scanned: true, Assignments: rows, Picked: map[int]bool{0: true}}, true, "rows", "smartcat.hintRecat", "smartcat.rescan", "smartcat.applySelected", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PresentReview(tc.s, tc.provider)
			if got.Result != tc.result || got.HintKey != tc.hint || got.ScanKey != tc.scan || got.ApplyKey != tc.apply || got.ShowApply != tc.show || got.Selected != tc.n || got.ApplyDisabled != (tc.n == 0) {
				t.Fatal(got)
			}
		})
	}
	for _, mode := range []string{Auto, Recat} {
		got := PresentReview(State{Mode: mode, Assignments: rows}, true).Rows[0]
		want := "\u2192 Food \u00b7 $1"
		if mode == Recat {
			want = "Old " + want
		}
		if got.Sub != want || got.TestID != "smartcat-row-0" || got.Key != "x" {
			t.Fatal(got)
		}
	}
	dup := PresentReview(State{Mode: Suggest, Suggestions: append(suggestions, suggestions[0]), Picked: map[int]bool{2: true}}, true)
	if dup.Rows[0].Index != 2 || !dup.Rows[0].Checked || dup.Rows[1].KindKey != "smartcat.kindIncome" || dup.Rows[0].KindKey != "smartcat.kindExpense" {
		t.Fatal(dup)
	}
}

func TestAssignmentWrites(t *testing.T) {
	live := IndexTransactions([]domain.Transaction{transaction("one", "", -1), transaction("transfer", "", -2)})
	transfer := live["transfer"]
	transfer.TransferAccountID = "other"
	live["transfer"] = transfer
	rows := []Assignment{{TxnID: "one", CatID: "food"}, {TxnID: "missing"}, {TxnID: "transfer"}, {TxnID: "unpicked"}}
	lookup := func(id string) (domain.Transaction, bool) { x, ok := live[id]; return x, ok }
	for _, tc := range []struct {
		name   string
		picked map[int]bool
		ids    []string
	}{
		{"live row plus missing and transfer", map[int]bool{0: true, 1: true, 2: true}, []string{"one"}},
		{"nil map", nil, nil}, {"unselected", map[int]bool{0: false}, nil},
		{"missing row", map[int]bool{1: true}, nil}, {"transfer", map[int]bool{2: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, ids := AssignmentWrites(rows, tc.picked, lookup)
			if !reflect.DeepEqual(ids, tc.ids) || len(writes) != len(tc.ids) {
				t.Fatal(writes, ids)
			}
			for _, w := range writes {
				if w.CategoryID != "food" || !w.Reviewed {
					t.Fatal(w)
				}
			}
			if live["one"].CategoryID != "" {
				t.Fatal("mutated live transaction")
			}
		})
	}
}

func FuzzSelectionToggle(f *testing.F) {
	f.Add(0, true)
	f.Add(-1, false)
	f.Add(100000, true)
	f.Fuzz(func(t *testing.T, index int, selected bool) {
		s := State{Picked: map[int]bool{index: selected, 42: true}}
		before := s.Picked[index]
		a := s.Toggle(index)
		b := a.Toggle(index)
		if s.Picked[index] != before || a.Picked[index] == before || b.Picked[index] != before {
			t.Fatal("toggle invariant")
		}
	})
}

func TestEmptyAndInFlightScans(t *testing.T) {
	for _, mode := range []string{Suggest, Auto} {
		s := State{Mode: Recat, Loading: true, Suggestions: []smartai.SuggestedCategory{{Name: "old"}}, Assignments: []Assignment{{TxnID: "old"}}, Picked: map[int]bool{9: true}}
		empty := s.NoSample(mode)
		if empty.Loading || !empty.Scanned || !empty.Picked[9] || empty.Mode != Recat {
			t.Fatal(empty)
		}
		if mode == Suggest && len(empty.Suggestions) != 0 || mode == Auto && len(empty.Assignments) != 0 {
			t.Fatal(empty)
		}
		done := s.Finish(mode, []smartai.SuggestedCategory{{Name: "new"}}, []Assignment{{TxnID: "new"}})
		if done.Mode != Recat || !done.Picked[0] {
			t.Fatal(done)
		}
		if mode == Suggest && done.Suggestions[0].Name != "new" || mode == Auto && done.Assignments[0].TxnID != "new" {
			t.Fatal(done)
		}
	}
}

func TestCategoryDrafts(t *testing.T) {
	suggestions := []smartai.SuggestedCategory{{Name: "Travel", ParentName: "food"}, {Name: "Other"}, {Name: "Pay", Kind: "income"}}
	for _, tc := range []struct {
		name        string
		suggestions []smartai.SuggestedCategory
		picked      map[int]bool
		names       []string
	}{
		{"nil", nil, nil, nil}, {"unselected", suggestions, nil, nil},
		{"selected", suggestions, map[int]bool{0: true, 2: true, 99: true}, []string{"Travel", "Pay"}},
		{"false entries", suggestions, map[int]bool{1: false}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := CategoryDrafts(tc.suggestions, tc.picked, fixtureCats())
			var names []string
			for _, c := range got {
				names = append(names, c.Name)
				if c.ID != "" {
					t.Fatal(c)
				}
			}
			if !reflect.DeepEqual(names, tc.names) {
				t.Fatal(got)
			}
			if len(got) > 0 && got[0].ParentID != "food" {
				t.Fatal(got)
			}
		})
	}
}

func TestParseSuggestionsCases(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		cats       []domain.Category
		want       []string
	}{
		{"empty", "", nil, nil},
		{"existing", "Food | expense", fixtureCats(), nil},
		{"duplicate", "Travel | expense\ntravel | income", nil, []string{"Travel"}},
		{"unknown parent", "Travel | expense | missing", fixtureCats(), []string{"Travel"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSuggestions(tc.text, tc.cats)
			var names []string
			for _, s := range got {
				names = append(names, s.Name)
				if s.ParentName != "" {
					t.Fatal("invented parent", s)
				}
			}
			if !reflect.DeepEqual(names, tc.want) {
				t.Fatal(got)
			}
		})
	}
}
