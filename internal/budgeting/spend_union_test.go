// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSpendUnionPreservesTagsOverlapsAndLineOwnership(t *testing.T) {
	categories := []domain.Category{{ID: "food"}, {ID: "grocery", ParentID: "food"}, {ID: "discount", ParentID: "food"}, {ID: "home"}}
	food := domain.Budget{ID: "food", CategoryIDs: []string{"food", "grocery"}}
	home := domain.Budget{ID: "home", CategoryID: "home"}
	tag := domain.Budget{ID: "tag", TrackedTags: []string{" Trip ", "trip"}}
	alice := domain.Budget{ID: "alice", CategoryID: "food", Scope: domain.ScopeIndividual, OwnerID: "alice"}
	tagAlice := tag
	tagAlice.Scope, tagAlice.OwnerID = domain.ScopeIndividual, "alice"
	split := domain.Transaction{ID: "receipt", MemberID: "bob", Amount: money.New(-100, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "grocery", MemberID: "alice", Amount: money.New(-70, "USD")},
		{CategoryID: "grocery", Amount: money.New(-20, "USD")},
		{CategoryID: "discount", MemberID: "alice", Amount: money.New(10, "USD")},
		{CategoryID: "home", Amount: money.New(-20, "USD")},
	}}
	tagged := split
	tagged.Tags = []string{"TRIP"}
	excluded := split
	excluded.ExcludeFromReports = true
	transfer := split
	transfer.TransferAccountID = "other"
	for _, tc := range []struct {
		name    string
		budgets []domain.Budget
		txn     domain.Transaction
		want    int64
		matched bool
	}{
		{"overlapping roots and duplicate lines", []domain.Budget{food, food}, split, 80, true},
		{"disjoint categories union", []domain.Budget{food, home}, split, 100, true},
		{"tag counts whole before category", []domain.Budget{food, tag, home, tag}, tagged, 100, true},
		{"line owner overrides payer and keeps discount", []domain.Budget{alice}, split, 60, true},
		{"unowned tag does not widen category scope", []domain.Budget{tagAlice, alice}, tagged, 60, true},
		{"report exclusion", []domain.Budget{food, tag}, excluded, 0, false},
		{"transfer exclusion", []domain.Budget{food, tag}, transfer, 0, false},
		{"unsplit overlap once", []domain.Budget{food, food}, domain.Transaction{CategoryID: "grocery", Amount: money.New(-123, "USD")}, 123, true},
		{"income exclusion", []domain.Budget{food}, domain.Transaction{CategoryID: "food", Amount: money.New(123, "USD")}, 0, false},
		{"no scopes", nil, split, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, matched, issues := PrepareSpendUnion(categories, tc.budgets).Contribution(tc.txn)
			if got.Amount != tc.want || matched != tc.matched || len(issues) != 0 {
				t.Fatalf("got %v matched=%v issues=%v; want %d matched=%v", got, matched, issues, tc.want, tc.matched)
			}
		})
	}
}

func TestSpendUnionCheckedFailuresRetainDiagnosticOrder(t *testing.T) {
	budgets := []domain.Budget{{ID: "first", CategoryID: "food"}, {ID: "second", CategoryID: "food"}}
	u := PrepareSpendUnion(nil, budgets)
	_, matched, issues := u.Contribution(domain.Transaction{ID: "minimum", CategoryID: "food", Amount: money.New(math.MinInt64, "USD")})
	if matched || len(issues) != 2 || issues[0].BudgetID != "first" || issues[1].BudgetID != "second" || !errors.Is(issues[0].Err, money.ErrOverflow) {
		t.Fatalf("lost contribution error/order: matched=%v issues=%+v", matched, issues)
	}
	for _, tc := range []struct {
		name string
		row  domain.Transaction
		err  error
	}{
		{"mixed split currency", domain.Transaction{Amount: money.New(-10, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-10, "EUR")}}}, money.ErrCurrencyMismatch},
		{"unset split currency preserves canonical mismatch", domain.Transaction{Amount: money.New(-10, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-10, "")}}}, money.ErrCurrencyMismatch},
		{"split total overflow", domain.Transaction{Amount: money.New(-10, "USD"), Splits: []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "food", Amount: money.New(-1, "USD")}}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, matched, issues := u.Contribution(tc.row)
			if matched || len(issues) != 1 || !errors.Is(issues[0].Err, tc.err) || issues[0].Operation != "sum selected-period split spending" {
				t.Fatalf("matched=%v issues=%+v", matched, issues)
			}
		})
	}
}

func TestSpendUnionPreparationIsImmutableAndMatchesCategoryClosures(t *testing.T) {
	categories := []domain.Category{{ID: "root"}, {ID: "leaf", ParentID: "root"}, {ID: "self", ParentID: "self"}, {ID: "a", ParentID: "b"}, {ID: "b", ParentID: "a"}}
	budgets := []domain.Budget{{ID: "one", CategoryIDs: []string{"root", "a", "missing", ""}, TrackedTags: []string{"trip"}}, {ID: "two", CategoryIDs: []string{"root", "a", "missing", ""}}}
	u := PrepareSpendUnion(categories, budgets)
	for i, b := range budgets {
		if !reflect.DeepEqual(u.scopes[i].covers, categorytree.DescendantsOfAll(categories, b.TrackedCategoryIDs())) {
			t.Fatal("prepared descendants changed missing/cyclic category rules")
		}
	}
	// Preparation owns both matching slices and its descendant maps.
	budgets[0].CategoryIDs[0], budgets[0].TrackedTags[0] = "unrelated", "changed"
	categories[1].ParentID = "unrelated"
	row := domain.Transaction{ID: "one", CategoryID: "leaf", Tags: []string{"trip"}, Amount: money.New(-42, "USD"), Splits: []domain.CategorySplit{{CategoryID: "unrelated", Amount: money.New(-42, "USD")}}}
	before := row.Splits[0]
	for range 2 {
		got, matched, issues := u.Contribution(row)
		if got.Amount != 42 || !matched || len(issues) != 0 || row.Splits[0] != before || row.Tags[0] != "trip" {
			t.Fatalf("immutable prepared union changed: %v %v %+v", got, matched, issues)
		}
	}
	plain := row
	plain.Splits, plain.Tags = nil, nil
	if got, matched, _ := u.Contribution(plain); !matched || got.Amount != 42 {
		t.Fatal("caller mutated prepared categories", got, matched)
	}
}
