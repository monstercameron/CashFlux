// SPDX-License-Identifier: MIT

//go:build js && wasm

package store

import (
	"fmt"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestTV8AWasmCapturePruneSoak(t *testing.T) {
	const writes = 120
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var tempStore int
	if err := s.db.QueryRow("PRAGMA temp_store").Scan(&tempStore); err != nil {
		t.Fatal(err)
	}
	if tempStore != 2 {
		t.Fatalf("PRAGMA temp_store = %d, want MEMORY (2)", tempStore)
	}

	var through uint64
	for n := 1; n <= writes; n++ {
		description := fmt.Sprintf("write-%03d", n)
		if err := s.PutTransaction(domain.Transaction{ID: "soak", Desc: description}); err != nil {
			t.Fatalf("write %d/%d: %v", n, writes, err)
		}
		target, err := s.ChangeCursor()
		if err != nil {
			t.Fatalf("cursor after write %d/%d: %v", n, writes, err)
		}
		page, err := s.CaptureChanges(through, target, 64)
		if err != nil {
			t.Fatalf("capture after write %d/%d: %v", n, writes, err)
		}
		if len(page.Changes) != 1 || page.Changes[0].ID != "soak" {
			t.Fatalf("capture after write %d/%d = %+v, want one soak record", n, writes, page.Changes)
		}
		if err := s.PruneChanges(target); err != nil {
			t.Fatalf("prune after write %d/%d: %v", n, writes, err)
		}
		var remaining int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM store_change_journal").Scan(&remaining); err != nil {
			t.Fatalf("journal count after write %d/%d: %v", n, writes, err)
		}
		if remaining != 0 {
			t.Fatalf("journal retained %d rows after pruning write %d/%d", remaining, n, writes)
		}
		through = target
	}

	got, found, err := s.GetTransaction("soak")
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.Desc != fmt.Sprintf("write-%03d", writes) {
		t.Fatalf("last write did not persist: found=%t transaction=%+v", found, got)
	}
	t.Logf("TV8A wasm soak passed: %d writes, %d captures, %d prunes; no journal errors or failed writes", writes, writes, writes)
}
