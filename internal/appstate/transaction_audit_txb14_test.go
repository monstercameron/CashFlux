// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
)

func TestTransactionAuditIncludesOldEntriesAndKeepsNewestFirst(t *testing.T) {
	a, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 505; i++ {
		entry := auditlog.Entry{ID: fmt.Sprintf("e-%03d", i), At: base.Add(time.Duration(i) * time.Minute),
			EntityType: "account", EntityID: "other"}
		if i == 1 || i == 503 || i == 504 {
			entry.EntityType, entry.EntityID = "transaction", "tx-old"
		}
		if err := a.Store().PutAuditEntry(entry); err != nil {
			t.Fatalf("PutAuditEntry(%s): %v", entry.ID, err)
		}
	}
	got := a.TransactionAudit("tx-old")
	if len(got) != 3 {
		t.Fatalf("got %d transaction entries, want all 3 (including one beyond newest 500)", len(got))
	}
	if got[0].ID != "e-504" || got[1].ID != "e-503" || got[2].ID != "e-001" {
		t.Fatalf("transaction audit IDs = %s, %s, %s; want newest first", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestTransactionAuditDoesNotMixSameIDFromOtherEntity(t *testing.T) {
	a, err := New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []auditlog.Entry{
		{ID: "tx", EntityType: "transaction", EntityID: "same"},
		{ID: "acct", EntityType: "account", EntityID: "same"},
	} {
		if err := a.Store().PutAuditEntry(entry); err != nil {
			t.Fatal(err)
		}
	}
	got := a.TransactionAudit("same")
	if len(got) != 1 || got[0].EntityType != "transaction" {
		t.Fatalf("TransactionAudit returned %+v, want only transaction event", got)
	}
}
