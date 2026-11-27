// SPDX-License-Identifier: MIT

package txntemplate

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTemplateHardening(t *testing.T) {
	if err := Validate(domain.TxnTemplate{Name: "Bad", AmountMinor: -100, AccountID: "a1", CategoryID: "c1"}); !errors.Is(err, ErrAmountZero) {
		t.Fatalf("negative amount Validate = %v, want ErrAmountZero", err)
	}

	// Delete must not corrupt a shallow copy of Store.
	var s Store
	s.Upsert(domain.TxnTemplate{ID: " t1 ", Name: "A", AmountMinor: 100, Tags: []string{"x"}})
	s.Upsert(domain.TxnTemplate{ID: "t2", Name: "B", AmountMinor: 200})
	if s.Items[0].ID != "t1" {
		t.Fatalf("ID not trimmed: %q", s.Items[0].ID)
	}
	snapshot := s
	s.Delete("t1")
	if len(snapshot.Items) != 2 || snapshot.Items[0].ID != "t1" {
		t.Fatalf("Delete mutated shallow copy backing array: %+v", snapshot.Items)
	}
}
