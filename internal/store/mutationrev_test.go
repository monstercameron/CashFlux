// SPDX-License-Identifier: MIT

package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMutationRevAdvancesOnWriteAndDelete(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	r0 := s.Rev()
	if err := s.PutAccount(domain.Account{ID: "a1", Name: "Checking"}); err != nil {
		t.Fatalf("put: %v", err)
	}
	r1 := s.Rev()
	if r1 <= r0 {
		t.Fatalf("rev did not advance on write: %d -> %d", r0, r1)
	}
	// A second write advances again.
	if err := s.PutAccount(domain.Account{ID: "a2", Name: "Savings"}); err != nil {
		t.Fatalf("put2: %v", err)
	}
	if s.Rev() <= r1 {
		t.Fatalf("rev did not advance on second write")
	}
	r2 := s.Rev()
	// A delete that removes a row advances.
	if _, err := s.DeleteAccount("a1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if s.Rev() <= r2 {
		t.Fatalf("rev did not advance on delete")
	}
	r3 := s.Rev()
	// A delete that removes nothing does NOT advance.
	if _, err := s.DeleteAccount("nonexistent"); err != nil {
		t.Fatalf("delete-missing: %v", err)
	}
	if s.Rev() != r3 {
		t.Fatalf("rev advanced on a no-op delete: %d -> %d", r3, s.Rev())
	}
}
