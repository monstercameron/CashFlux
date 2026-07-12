// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestApplyMultiAccount: the Accounts (multi) set matches OR-within the dimension and
// takes precedence over the single Account.
func TestApplyMultiAccount(t *testing.T) {
	txns := sample() // a:acc1, b:acc2, c:acc1
	if got := ids(Apply(txns, Criteria{Accounts: "acc2"})); got != "b" {
		t.Fatalf("Accounts=acc2 => %q, want b", got)
	}
	if got := ids(Apply(txns, Criteria{Accounts: "acc1,acc2"})); len(got) != 3 {
		t.Fatalf("Accounts=acc1,acc2 => %q, want all 3", got)
	}
	// Multi takes precedence over the single Account.
	if got := ids(Apply(txns, Criteria{Account: "acc1", Accounts: "acc2"})); got != "b" {
		t.Fatalf("Accounts=acc2 (single acc1) => %q, want b (multi wins)", got)
	}
}

// TestApplyMultiMemberSourceTag exercises the other multi dimensions.
func TestApplyMultiMemberSourceTag(t *testing.T) {
	txns := []domain.Transaction{
		{ID: "a", MemberID: "m1", Source: domain.TxnSourceManual, Tags: []string{"food", "treat"}, Amount: money.New(-100, "USD"), Date: d("2026-06-01")},
		{ID: "b", MemberID: "m2", Source: domain.TxnSourceImported, Tags: []string{"bill"}, Amount: money.New(-200, "USD"), Date: d("2026-06-02")},
		{ID: "c", MemberID: "m3", Source: domain.TxnSourceRecurring, Amount: money.New(-300, "USD"), Date: d("2026-06-03")},
	}
	// Results come back sorted newest-first (date desc), so c (06-03) precedes a (06-01).
	if got := ids(Apply(txns, Criteria{Members: "m1,m3"})); got != "ca" {
		t.Fatalf("Members=m1,m3 => %q, want ca", got)
	}
	if got := ids(Apply(txns, Criteria{Sources: string(domain.TxnSourceManual) + "," + string(domain.TxnSourceImported)})); got != "ba" {
		t.Fatalf("Sources=manual,import => %q, want ba", got)
	}
	if got := ids(Apply(txns, Criteria{Tags: "treat,bill"})); got != "ba" {
		t.Fatalf("Tags=treat,bill => %q, want ba", got)
	}
}

// TestToggleValue: toggling adds/removes and folds+clears the single counterpart.
func TestToggleValue(t *testing.T) {
	c := Criteria{}.ToggleValue(FieldCategory, "food")
	if c.Categories != "food" || c.Category != "" {
		t.Fatalf("toggle food => Categories=%q Category=%q", c.Categories, c.Category)
	}
	c = c.ToggleValue(FieldCategory, "food") // toggle off
	if c.Categories != "" {
		t.Fatalf("toggle food off => Categories=%q, want empty", c.Categories)
	}
	// A pre-existing single value is folded into the multi set, then cleared.
	c = Criteria{Category: "food"}.ToggleValue(FieldCategory, "rent")
	if c.Category != "" {
		t.Fatalf("single not cleared: Category=%q", c.Category)
	}
	if vals := c.SelectedValues(FieldCategory); len(vals) != 2 {
		t.Fatalf("SelectedValues=%v, want food+rent", vals)
	}
}

// TestSelectedValuesAndRemoveValue: SelectedValues merges single+multi; RemoveValue
// drops one value without touching the rest.
func TestSelectedValuesAndRemoveValue(t *testing.T) {
	c := Criteria{Categories: "food,rent"}
	if vals := c.SelectedValues(FieldCategory); len(vals) != 2 {
		t.Fatalf("SelectedValues=%v, want 2", vals)
	}
	c = c.RemoveValue(FieldCategory, "food")
	if c.Categories != "rent" {
		t.Fatalf("RemoveValue food => %q, want rent", c.Categories)
	}
	// Single fallback shows up in SelectedValues.
	if vals := (Criteria{Member: "m1"}).SelectedValues(FieldMember); len(vals) != 1 || vals[0] != "m1" {
		t.Fatalf("single SelectedValues=%v, want [m1]", vals)
	}
}

// TestActiveFiltersPerValueChips: each selected multi value is one removable chip.
func TestActiveFiltersPerValueChips(t *testing.T) {
	c := Criteria{Accounts: "acc1,acc2", Members: "m1"}
	af := c.ActiveFilters()
	nAcct, nMem := 0, 0
	for _, a := range af {
		switch a.Field {
		case FieldAccount:
			nAcct++
		case FieldMember:
			nMem++
		}
	}
	if nAcct != 2 || nMem != 1 {
		t.Fatalf("chips: account=%d member=%d, want 2 and 1 (%v)", nAcct, nMem, af)
	}
	if c.ActiveCount() != 3 {
		t.Fatalf("ActiveCount=%d, want 3", c.ActiveCount())
	}
}
