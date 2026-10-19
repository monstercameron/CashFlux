// SPDX-License-Identifier: MIT

package store

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestSOLP3LedgerReaderTracksOnlyCommittedTransactions(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.NewLedgerReader()
	if err != nil {
		t.Fatal(err)
	}
	before, err := r.Revision()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO transactions VALUES('rollback','{"id":"rollback"}')`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got, err := r.Revision(); err != nil || got != before {
		t.Fatalf("rollback invalidated reader: %d %v", got, err)
	}
	if err := s.SetKV("key", "value"); err != nil {
		t.Fatal(err)
	}
	if got, err := r.Revision(); err != nil || got != before {
		t.Fatalf("KV invalidated reader: %d %v", got, err)
	}
	if err := s.PutTransaction(domain.Transaction{ID: "one", Desc: "first"}); err != nil {
		t.Fatal(err)
	}
	target, err := r.Revision()
	if err != nil || target <= before {
		t.Fatalf("write revision=%d %v", target, err)
	}
	if err := s.PutTransaction(domain.Transaction{ID: "one", Desc: "second"}); err != nil {
		t.Fatal(err)
	}
	changes, err := r.Changes(before, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || string(changes[0].After) == "" {
		t.Fatalf("fixed target changes=%+v", changes)
	}
	var fixed domain.Transaction
	if err := json.Unmarshal(changes[0].After, &fixed); err != nil {
		t.Fatal(err)
	}
	if fixed.Desc != "first" {
		t.Fatalf("fixed target included later edit: %+v", fixed)
	}

	if err := s.PruneChanges(target); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Changes(before, target); !errors.Is(err, ErrCaptureExpired) {
		t.Fatalf("missing prune error: %v", err)
	}
	after, err := r.Revision()
	if err != nil || after <= target {
		t.Fatalf("pruning lost watermark=%d %v", after, err)
	}
	for _, span := range [][2]uint64{{after, after - 1}, {after, after + 1}} {
		if _, err := r.Changes(span[0], span[1]); err == nil {
			t.Fatalf("invalid change range accepted: %v", span)
		}
	}

	again, err := s.NewLedgerReader()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := again.Revision(); err != nil || got != after {
		t.Fatalf("second reader reset watermark: %d %v", got, err)
	}
}
