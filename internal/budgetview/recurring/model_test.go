// SPDX-License-Identifier: MIT

package recurring

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCoverage(t *testing.T) {
	categories := []domain.Category{{ID: "home"}, {ID: "rent", ParentID: "home"}, {ID: "power", ParentID: "home"}, {ID: "food"}}
	budgets := []domain.Budget{{CategoryID: "home", Scope: domain.ScopeShared}, {CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "alice"}}
	for _, tc := range []struct {
		member string
		food   bool
	}{{"", true}, {"alice", true}, {"bob", false}} {
		got := Coverage(budgets, categories, tc.member)
		if !got["home"] || !got["rent"] || !got["power"] || got["food"] != tc.food {
			t.Fatalf("member=%q coverage=%v", tc.member, got)
		}
	}
	if len(Coverage(nil, nil, "")) != 0 {
		t.Fatal("empty coverage")
	}
	got := Coverage([]domain.Budget{{CategoryID: "home", CategoryIDs: []string{"food"}}}, categories, "")
	if !got["food"] {
		t.Fatalf("tracked categories: %v", got)
	}
}

func TestInMemberLens(t *testing.T) {
	accounts := []domain.Account{{ID: "alice", Scope: domain.ScopeIndividual, OwnerID: "alice"}, {ID: "bob", Scope: domain.ScopeIndividual, OwnerID: "bob"}, {ID: "shared", Scope: domain.ScopeShared}, {ID: "group", Scope: domain.ScopeIndividual, OwnerID: domain.GroupOwnerID}}
	for _, tc := range []struct {
		account, member string
		want            bool
	}{{"alice", "bob", false}, {"bob", "bob", true}, {"shared", "bob", true}, {"group", "bob", true}, {"missing", "bob", false}, {"", "bob", true}, {"missing", "", true}} {
		if got := InMemberLens(domain.Recurring{AccountID: tc.account}, tc.member, accounts); got != tc.want {
			t.Fatalf("%+v got %v", tc, got)
		}
	}
	if InMemberLens(domain.Recurring{AccountID: "x"}, "bob", nil) {
		t.Fatal("nil accounts leaked bill")
	}
}

func TestExcludedLabel(t *testing.T) {
	for _, tc := range []struct{ label, want string }{{" Rent \n", "Rent"}, {" \t", "Unnamed bill"}, {"", "Unnamed bill"}, {"電気", "電気"}} {
		if got := ExcludedLabel(domain.Recurring{Label: tc.label}, "Unnamed bill"); got != tc.want {
			t.Fatalf("%q => %q", tc.label, got)
		}
	}
}

func TestBuildPartitionsAndDoesNotMutate(t *testing.T) {
	recs := []domain.Recurring{bill("first", -100, "USD", domain.CadenceMonthly), bill("second", -100, "USD", domain.CadenceMonthly), bill("yearly", -2400, "USD", domain.CadenceYearly), bill("unbudgeted", -300, "EUR", domain.CadenceMonthly), bill("zeta", -500, "ZZZ", domain.CadenceMonthly), bill(" alpha ", -500, "ZZZ", domain.CadenceMonthly), bill("", -500, "ZZZ", domain.CadenceMonthly), bill("income", 1000, "USD", domain.CadenceMonthly), bill("zero", 0, "USD", domain.CadenceMonthly), bill("paused", -1000, "USD", domain.CadenceMonthly), bill("foreign", -1000, "USD", domain.CadenceMonthly)}
	for i := 0; i < 3; i++ {
		recs[i].CategoryID = "rent"
	}
	recs[3].CategoryID = "food"
	recs[9].Paused = true
	recs[10].AccountID = "alice"
	before := append([]domain.Recurring(nil), recs...)
	in := Input{Recurring: recs, MemberID: "bob", Accounts: []domain.Account{{ID: "alice", Scope: domain.ScopeIndividual, OwnerID: "alice"}}, Budgets: []domain.Budget{{CategoryID: "home", Scope: domain.ScopeShared}}, Categories: []domain.Category{{ID: "home"}, {ID: "rent", ParentID: "home"}, {ID: "food", Name: "Food"}}, Rates: map[string]float64{"EUR": 2}}
	got, err := Build(in, "Uncategorized", "Unnamed bill")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Visible || got.Base != "USD" || got.Count != 4 || got.CoveredMonthly.Amount != 400 || got.UnbudgetedMonthly.Amount != 600 || got.TotalMonthly.Amount != 1000 {
		t.Fatalf("summary=%+v", got)
	}
	if len(got.Covered) != 3 || got.Covered[0].ID != "yearly" || got.Covered[1].ID != "first" || got.Covered[2].ID != "second" {
		t.Fatalf("stable order=%v", got.Covered)
	}
	if !reflect.DeepEqual(got.Excluded, []string{"Unnamed bill", "alpha", "zeta"}) {
		t.Fatalf("excluded=%v", got.Excluded)
	}
	if len(got.Unbudgeted) != 1 || got.Unbudgeted[0].CategoryName != "Food" || got.Unbudgeted[0].LimitMajor != "6.00" || !got.Unbudgeted[0].CanBudget {
		t.Fatalf("category=%+v", got.Unbudgeted)
	}
	if !reflect.DeepEqual(recs, before) {
		t.Fatal("input mutated")
	}
}

func TestBuildEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name              string
		in                Input
		count, excluded   int
		visible, overflow bool
	}{
		{"empty", Input{}, 0, 0, false, false},
		{"income", Input{Recurring: []domain.Recurring{bill("pay", 100, "USD", "")}}, 0, 0, false, false},
		{"tiny annual", Input{Recurring: []domain.Recurring{bill("tiny", -1, "USD", domain.CadenceYearly)}}, 1, 0, true, false},
		{"all missing FX", Input{Recurring: []domain.Recurring{bill("x", -1, "EUR", "")}}, 0, 1, true, false},
		{"monthly min excluded", Input{Recurring: []domain.Recurring{bill("min", math.MinInt64, "USD", "")}}, 0, 1, true, false},
		{"weekly min excluded", Input{Recurring: []domain.Recurring{bill("min", math.MinInt64, "USD", domain.CadenceWeekly)}}, 0, 1, true, false},
		{"category overflow", overflowInput(false, false), 0, 0, false, true},
		{"unbudgeted section overflow", overflowInput(false, true), 0, 0, false, true},
		{"covered overflow", overflowInput(true, false), 0, 0, false, true},
		{"combined overflow", overflowInput(true, true), 0, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(tc.in, "Uncategorized", "Unnamed")
			if tc.overflow {
				if !errors.Is(err, money.ErrOverflow) || !reflect.DeepEqual(got, Model{}) {
					t.Fatalf("partial overflow: %+v %v", got, err)
				}
				return
			}
			if err != nil || got.Count != tc.count || len(got.Excluded) != tc.excluded || got.Visible != tc.visible {
				t.Fatalf("model=%+v err=%v", got, err)
			}
			if len(got.Unbudgeted) > 0 && (got.Unbudgeted[0].CanBudget || got.Unbudgeted[0].CategoryName != "Uncategorized") {
				t.Fatalf("uncategorized=%+v", got.Unbudgeted)
			}
		})
	}
}

func overflowInput(covered, separate bool) Input {
	a, b := bill("a", -math.MaxInt64, "USD", ""), bill("b", -1, "USD", "")
	a.CategoryID = "a"
	b.CategoryID = "a"
	if separate {
		b.CategoryID = "b"
	}
	in := Input{Recurring: []domain.Recurring{a, b}}
	if covered {
		in.Budgets = []domain.Budget{{CategoryID: "a"}}
	}
	return in
}

func TestBuildLargeInputAndCategoryOrder(t *testing.T) {
	in := Input{BaseCurrency: "JPY", Recurring: make([]domain.Recurring, 20000)}
	for i := range in.Recurring {
		in.Recurring[i] = bill("bill", -1, "JPY", "")
		if i%2 == 0 {
			in.Recurring[i].CategoryID = "z"
		} else {
			in.Recurring[i].CategoryID = "a"
		}
	}
	got, err := Build(in, "Uncategorized", "Unnamed")
	if err != nil || got.Count != 20000 || got.TotalMonthly.Amount != 20000 || got.Unbudgeted[0].CategoryID != "a" || got.Unbudgeted[0].LimitMajor != "10000" {
		t.Fatalf("large summary=%+v %v", got, err)
	}
	for i := 0; i < 20; i++ {
		again, err := Build(in, "Uncategorized", "Unnamed")
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatal("nondeterministic equal category totals")
		}
	}
}

func TestBuildUsesNativeRankAndKeepsOverflowExclusions(t *testing.T) {
	a, b := bill("converted bigger", -1000, "EUR", ""), bill("native bigger", -2000, "USD", "")
	a.CategoryID, b.CategoryID = "eur", "usd"
	in := Input{Recurring: []domain.Recurring{
		a, b, bill("min", math.MinInt64, "USD", ""), bill("weekly min", math.MinInt64, "USD", domain.CadenceWeekly),
	}, Rates: map[string]float64{"EUR": 100}}
	m, err := Build(in, "Uncategorized", "Unnamed")
	if err != nil || m.Count != 2 || len(m.Excluded) != 2 || len(m.Unbudgeted) != 2 || m.Unbudgeted[0].CategoryID != "eur" || m.Unbudgeted[1].CategoryID != "usd" {
		t.Fatalf("category ranking: %+v %v", m, err)
	}
	in.Budgets = []domain.Budget{{CategoryIDs: []string{"eur", "usd"}}}
	m, err = Build(in, "Uncategorized", "Unnamed")
	if err != nil || len(m.Covered) != 2 || m.Covered[0].ID != b.ID || m.Covered[1].ID != a.ID {
		t.Fatalf("native bill ranking: %+v %v", m, err)
	}
}

func FuzzBuildTotalsReconcile(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5})
	f.Add([]byte{})
	f.Add([]byte{255, 255})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 2048 {
			data = data[:2048]
		}
		in := Input{Budgets: []domain.Budget{{CategoryID: "covered"}}}
		var total int64
		for i, v := range data {
			r := bill("bill", -int64(v), "USD", domain.CadenceMonthly)
			if i%2 == 0 {
				r.CategoryID = "covered"
			}
			in.Recurring = append(in.Recurring, r)
			total += int64(v)
		}
		m, err := Build(in, "Uncategorized", "Unnamed")
		if err != nil {
			t.Fatal(err)
		}
		if m.TotalMonthly.Amount != total || m.CoveredMonthly.Amount+m.UnbudgetedMonthly.Amount != total {
			t.Fatalf("unreconciled %+v want %d", m, total)
		}
		var categories int64
		for _, c := range m.Unbudgeted {
			categories += c.Monthly.Amount
			roundtrip, err := money.ParseMinor(c.LimitMajor, 2)
			if err != nil || roundtrip != c.Monthly.Amount {
				t.Fatal("draft round trip")
			}
		}
		if categories != m.UnbudgetedMonthly.Amount {
			t.Fatal("category totals do not reconcile")
		}
	})
}
