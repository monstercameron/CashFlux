package store

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// TestStoreErrorsOnClosedDB exercises the error branches of the store helpers by
// running them against a closed database, where every query fails. No production
// seam is needed — Close() is part of the public API, and a closed *sql.DB makes
// Exec/Query/QueryRow return errors.
func TestStoreErrorsOnClosedDB(t *testing.T) {
	s, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	s.Close()

	// crud.go generic helpers: putJSON / getJSON / deleteRow / loadRows / queryRows.
	if err := s.PutMember(domain.Member{ID: "m1", Name: "A"}); err == nil {
		t.Error("PutMember on a closed DB should error (putJSON)")
	}
	if _, _, err := s.GetMember("m1"); err == nil {
		t.Error("GetMember on a closed DB should error (getJSON)")
	}
	if _, err := s.DeleteMember("m1"); err == nil {
		t.Error("DeleteMember on a closed DB should error (deleteRow)")
	}
	if _, err := s.ListMembers(); err == nil {
		t.Error("ListMembers on a closed DB should error (loadRows)")
	}
	if _, err := s.TransactionsByAccount("a1"); err == nil {
		t.Error("TransactionsByAccount on a closed DB should error (queryRows)")
	}

	// manage.go: settings + wipe.
	if _, err := s.GetSettings(); err == nil {
		t.Error("GetSettings on a closed DB should error")
	}
	if err := s.PutSettings(Settings{}); err == nil {
		t.Error("PutSettings on a closed DB should error")
	}
	if err := s.Wipe(); err == nil {
		t.Error("Wipe on a closed DB should error")
	}

	// sqlitestore.go: Snapshot reads every table; Load replaces them in a tx.
	if _, err := s.Snapshot(); err == nil {
		t.Error("Snapshot on a closed DB should error")
	}
	if err := s.Load(Dataset{}); err == nil {
		t.Error("Load on a closed DB should error")
	}
}
