package txnfilter

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func tx(acct, cat, mem string, tags ...string) domain.Transaction {
	return domain.Transaction{AccountID: acct, CategoryID: cat, MemberID: mem, Tags: tags}
}

func TestMultiCriteriaMatches(t *testing.T) {
	groceries := tx("checking", "food", "maya", "weekly")
	dining := tx("credit", "dining", "devon", "treat", "weekly")
	rent := tx("checking", "housing", "maya")

	tests := []struct {
		name string
		m    MultiCriteria
		txn  domain.Transaction
		want bool
	}{
		{"empty matches everything", MultiCriteria{}, dining, true},
		{"OR within accounts — first", MultiCriteria{Accounts: []string{"checking", "credit"}}, groceries, true},
		{"OR within accounts — second", MultiCriteria{Accounts: []string{"checking", "credit"}}, dining, true},
		{"account not in set fails", MultiCriteria{Accounts: []string{"savings"}}, groceries, false},
		{"OR within categories", MultiCriteria{Categories: []string{"food", "dining"}}, dining, true},
		{"AND across dimensions — both match", MultiCriteria{Accounts: []string{"checking"}, Categories: []string{"food"}}, groceries, true},
		{"AND across — one fails", MultiCriteria{Accounts: []string{"checking"}, Categories: []string{"dining"}}, groceries, false},
		{"member matches", MultiCriteria{Members: []string{"devon"}}, dining, true},
		{"tags OR — shared tag", MultiCriteria{Tags: []string{"treat", "fun"}}, dining, true},
		{"tags — none shared", MultiCriteria{Tags: []string{"fun"}}, groceries, false},
		{"tags against a txn with no tags", MultiCriteria{Tags: []string{"weekly"}}, rent, false},
		{"all dimensions engaged and pass", MultiCriteria{Accounts: []string{"credit"}, Categories: []string{"dining"}, Members: []string{"devon"}, Tags: []string{"weekly"}}, dining, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Matches(tc.txn); got != tc.want {
				t.Errorf("Matches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMultiCriteriaFilterPreservesOrder(t *testing.T) {
	txns := []domain.Transaction{tx("a", "food", ""), tx("b", "food", ""), tx("a", "rent", "")}
	got := MultiCriteria{Categories: []string{"food"}}.Filter(txns)
	if len(got) != 2 || got[0].AccountID != "a" || got[1].AccountID != "b" {
		t.Errorf("Filter = %+v, want the two food rows in input order", got)
	}
}

func TestMultiCriteriaNormalizeAndEqual(t *testing.T) {
	a := MultiCriteria{Accounts: []string{"b", "a", "a"}, Tags: []string{"y", "x"}}
	n := a.Normalize()
	if !reflect.DeepEqual(n.Accounts, []string{"a", "b"}) {
		t.Errorf("Normalize accounts = %v, want [a b] deduped+sorted", n.Accounts)
	}
	// Equal is order- and duplicate-insensitive.
	b := MultiCriteria{Accounts: []string{"a", "b"}, Tags: []string{"x", "y"}}
	if !a.Equal(b) {
		t.Error("Equal should hold across order/dup differences")
	}
	if a.Equal(MultiCriteria{Accounts: []string{"a"}, Tags: []string{"x", "y"}}) {
		t.Error("Equal should fail when a value is missing")
	}
}

func TestMultiCriteriaAddToggleWithout(t *testing.T) {
	m := MultiCriteria{}
	m = m.Add(FieldCategory, "food")
	m = m.Add(FieldCategory, "food") // idempotent
	if !reflect.DeepEqual(m.Categories, []string{"food"}) {
		t.Fatalf("Add not idempotent: %v", m.Categories)
	}
	m = m.Toggle(FieldCategory, "dining") // adds
	if !contains(m.Categories, "dining") {
		t.Fatal("Toggle should have added dining")
	}
	m = m.Toggle(FieldCategory, "food") // removes
	if contains(m.Categories, "food") {
		t.Fatal("Toggle should have removed food")
	}
	m = m.Without(FieldCategory, "dining")
	if len(m.Categories) != 0 {
		t.Fatalf("Without should have emptied categories: %v", m.Categories)
	}
}

func TestMultiCriteriaActiveValues(t *testing.T) {
	m := MultiCriteria{Accounts: []string{"checking"}, Tags: []string{"y", "x"}}
	got := m.ActiveValues()
	want := []ActiveFilter{
		{Field: FieldAccount, Value: "checking"},
		{Field: FieldTags, Value: "x"},
		{Field: FieldTags, Value: "y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActiveValues = %+v, want %+v (account then sorted tags)", got, want)
	}
	if !(MultiCriteria{}).IsEmpty() {
		t.Error("zero value should be empty")
	}
	if m.IsEmpty() {
		t.Error("engaged filter should not be empty")
	}
}
