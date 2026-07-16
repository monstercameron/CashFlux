// SPDX-License-Identifier: MIT

package txntemplate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestUpsert(t *testing.T) {
	t.Run("insert assigns an ID when empty", func(t *testing.T) {
		var s Store
		s.Upsert(domain.TxnTemplate{Name: "Coffee", AmountMinor: 450, AccountID: "a1", CategoryID: "c1"})
		if len(s.Items) != 1 {
			t.Fatalf("want 1 item, got %d", len(s.Items))
		}
		if s.Items[0].ID == "" {
			t.Fatal("want a generated ID, got empty")
		}
	})

	t.Run("insert keeps a caller-supplied ID", func(t *testing.T) {
		var s Store
		s.Upsert(domain.TxnTemplate{ID: "t1", Name: "Coffee", AmountMinor: 450})
		if s.Items[0].ID != "t1" {
			t.Fatalf("want ID t1, got %q", s.Items[0].ID)
		}
	})

	t.Run("update by ID replaces in place", func(t *testing.T) {
		var s Store
		s.Upsert(domain.TxnTemplate{ID: "t1", Name: "Coffee", AmountMinor: 450})
		s.Upsert(domain.TxnTemplate{ID: "t2", Name: "Lunch", AmountMinor: 1200})
		s.Upsert(domain.TxnTemplate{ID: "t1", Name: "Coffee XL", AmountMinor: 500})
		if len(s.Items) != 2 {
			t.Fatalf("update must not append; want 2 items, got %d", len(s.Items))
		}
		if s.Items[0].Name != "Coffee XL" || s.Items[0].AmountMinor != 500 {
			t.Fatalf("update did not replace fields: %+v", s.Items[0])
		}
		if s.Items[1].Name != "Lunch" {
			t.Fatalf("update disturbed sibling: %+v", s.Items[1])
		}
	})
}

func TestDelete(t *testing.T) {
	var s Store
	s.Upsert(domain.TxnTemplate{ID: "t1", Name: "Coffee", AmountMinor: 450})
	s.Upsert(domain.TxnTemplate{ID: "t2", Name: "Lunch", AmountMinor: 1200})

	s.Delete("t1")
	if len(s.Items) != 1 || s.Items[0].ID != "t2" {
		t.Fatalf("delete t1 failed: %+v", s.Items)
	}
	// Deleting a missing / empty ID is a no-op.
	s.Delete("nope")
	s.Delete("")
	if len(s.Items) != 1 {
		t.Fatalf("no-op delete changed the store: %+v", s.Items)
	}
}

func TestValidate(t *testing.T) {
	valid := domain.TxnTemplate{Name: "Coffee", AmountMinor: 450, AccountID: "a1", CategoryID: "c1"}

	tests := []struct {
		name string
		tmpl domain.TxnTemplate
		want error
	}{
		{"valid", valid, nil},
		{"blank name", domain.TxnTemplate{Name: "  ", AmountMinor: 450, AccountID: "a1", CategoryID: "c1"}, ErrNameRequired},
		{"zero amount", domain.TxnTemplate{Name: "Coffee", AmountMinor: 0, AccountID: "a1", CategoryID: "c1"}, ErrAmountZero},
		{"no account", domain.TxnTemplate{Name: "Coffee", AmountMinor: 450, AccountID: "", CategoryID: "c1"}, ErrAccountRequired},
		{"no category", domain.TxnTemplate{Name: "Coffee", AmountMinor: 450, AccountID: "a1", CategoryID: ""}, ErrCategoryRequired},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.tmpl); !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	var s Store
	s.Upsert(domain.TxnTemplate{
		ID: "t1", Name: "Coffee", Payee: "Blue Bottle", CategoryID: "c1",
		AccountID: "a1", AmountMinor: 450, Currency: "USD",
		Direction: domain.DirectionExpense, Note: "morning", Tags: []string{"treat"},
	})
	s.Upsert(domain.TxnTemplate{
		ID: "t2", Name: "Paycheck", AccountID: "a1", AmountMinor: 250000,
		Currency: "USD", Direction: domain.DirectionIncome, CategoryID: "c2",
	})

	raw, err := Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("round-trip lost items: %+v", got.Items)
	}
	if got.Items[0].Name != "Coffee" || got.Items[0].AmountMinor != 450 ||
		got.Items[0].Direction != domain.DirectionExpense || len(got.Items[0].Tags) != 1 {
		t.Fatalf("template 0 mismatch after round-trip: %+v", got.Items[0])
	}
	if got.Items[1].Direction != domain.DirectionIncome {
		t.Fatalf("template 1 direction lost: %+v", got.Items[1])
	}
}

func TestUnmarshalTolerant(t *testing.T) {
	tests := []string{"", "   ", "not json", "{", "[1,2,3]", "null"}
	for _, raw := range tests {
		got, err := Unmarshal(raw)
		if err != nil {
			t.Fatalf("Unmarshal(%q) returned error %v, want nil", raw, err)
		}
		if len(got.Items) != 0 {
			t.Fatalf("Unmarshal(%q) yielded items %+v, want empty", raw, got.Items)
		}
	}
}

func TestApply(t *testing.T) {
	now := time.Date(2026, 7, 15, 9, 30, 0, 0, time.UTC)

	t.Run("expense builds a negative signed draft", func(t *testing.T) {
		tmpl := domain.TxnTemplate{
			ID: "t1", Name: "Coffee", Payee: "  Blue Bottle  ", CategoryID: "c1",
			AccountID: "a1", AmountMinor: 450, Currency: "usd",
			Direction: domain.DirectionExpense, Note: "  morning  ", Tags: []string{"treat"},
		}
		got := Apply(tmpl, now)

		if got.ID != "" {
			t.Fatalf("draft ID must be blank, got %q", got.ID)
		}
		if !got.Date.Equal(now) {
			t.Fatalf("draft date = %v, want %v", got.Date, now)
		}
		if got.AccountID != "a1" || got.CategoryID != "c1" {
			t.Fatalf("account/category not carried: %+v", got)
		}
		if got.Payee != "Blue Bottle" {
			t.Fatalf("payee not trimmed: %q", got.Payee)
		}
		if got.Desc != "morning" {
			t.Fatalf("note not mapped to trimmed Desc: %q", got.Desc)
		}
		if got.Amount.Amount != -450 {
			t.Fatalf("expense amount = %d, want -450", got.Amount.Amount)
		}
		if got.Amount.Currency != "USD" {
			t.Fatalf("currency not normalised: %q", got.Amount.Currency)
		}
		if len(got.Tags) != 1 || got.Tags[0] != "treat" {
			t.Fatalf("tags not carried: %+v", got.Tags)
		}
		if got.Source != domain.TxnSourceManual {
			t.Fatalf("source = %q, want manual", got.Source)
		}
	})

	t.Run("income builds a positive signed draft", func(t *testing.T) {
		tmpl := domain.TxnTemplate{
			Name: "Paycheck", AccountID: "a1", AmountMinor: 250000,
			Currency: "USD", Direction: domain.DirectionIncome,
		}
		got := Apply(tmpl, now)
		if got.Amount.Amount != 250000 {
			t.Fatalf("income amount = %d, want 250000", got.Amount.Amount)
		}
	})

	t.Run("blank direction defaults to expense", func(t *testing.T) {
		got := Apply(domain.TxnTemplate{AmountMinor: 999, Currency: "USD"}, now)
		if got.Amount.Amount != -999 {
			t.Fatalf("default-direction amount = %d, want -999", got.Amount.Amount)
		}
	})

	t.Run("stored sign is treated as magnitude", func(t *testing.T) {
		got := Apply(domain.TxnTemplate{AmountMinor: -450, Direction: domain.DirectionExpense, Currency: "USD"}, now)
		if got.Amount.Amount != -450 {
			t.Fatalf("magnitude handling wrong: got %d, want -450", got.Amount.Amount)
		}
	})
}
