// SPDX-License-Identifier: MIT

package docexpiry

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func dt(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestDueReminderWithinLead(t *testing.T) {
	docs := []domain.AccountDocRef{
		{ArtifactID: "pol", Label: "Auto policy", AttachedAt: dt(2026, 1, 1), ExpiresAt: dt(2026, 4, 1)},
	}
	// now is 20 days before expiry with a 30-day lead: due.
	got := DueReminders("ac1", docs, 30, dt(2026, 3, 12))
	if len(got) != 1 {
		t.Fatalf("want 1 reminder, got %d", len(got))
	}
	if got[0].Key != "docexpiry:ac1:auto policy" {
		t.Errorf("key = %q", got[0].Key)
	}
	if !got[0].DueAt.Equal(dt(2026, 3, 2)) {
		t.Errorf("DueAt = %v, want Mar 2", got[0].DueAt)
	}
}

func TestNotYetDue(t *testing.T) {
	docs := []domain.AccountDocRef{
		{ArtifactID: "pol", Label: "Auto policy", ExpiresAt: dt(2026, 4, 1)},
	}
	got := DueReminders("ac1", docs, 30, dt(2026, 2, 1)) // 60 days out
	if len(got) != 0 {
		t.Errorf("should not be due yet: %+v", got)
	}
}

func TestSupersededByNewerSameLabel(t *testing.T) {
	docs := []domain.AccountDocRef{
		{ArtifactID: "old", Label: "Auto policy", AttachedAt: dt(2026, 1, 1), ExpiresAt: dt(2026, 4, 1)},
		{ArtifactID: "new", Label: "auto POLICY", AttachedAt: dt(2026, 3, 15), ExpiresAt: dt(2027, 4, 1)},
	}
	got := DueReminders("ac1", docs, 30, dt(2026, 3, 20))
	// Old is superseded (case-insensitive label match, newer AttachedAt); new expires 2027 so not due.
	if len(got) != 0 {
		t.Errorf("old should be superseded and new not-yet-due: %+v", got)
	}
}

func TestNoExpiryNoReminder(t *testing.T) {
	docs := []domain.AccountDocRef{{ArtifactID: "x", Label: "Statement"}}
	if got := DueReminders("ac1", docs, 30, dt(2026, 3, 20)); len(got) != 0 {
		t.Errorf("no expiry should produce no reminder: %+v", got)
	}
}

func TestActiveKeysAcrossAccounts(t *testing.T) {
	accts := []domain.Account{
		{ID: "ac1", DocRefs: []domain.AccountDocRef{{ArtifactID: "p", Label: "Policy", ExpiresAt: dt(2026, 3, 25)}}},
		{ID: "ac2"},
	}
	keys := ActiveKeys(accts, 30, dt(2026, 3, 20))
	if len(keys) != 1 {
		t.Fatalf("want 1 active key, got %d", len(keys))
	}
	if _, ok := keys["docexpiry:ac1:policy"]; !ok {
		t.Errorf("missing expected key: %v", keys)
	}
}

func TestDefaultLeadWhenNonPositive(t *testing.T) {
	docs := []domain.AccountDocRef{{ArtifactID: "p", Label: "P", ExpiresAt: dt(2026, 4, 1)}}
	got := DueReminders("ac1", docs, 0, dt(2026, 3, 5)) // default 30-day lead → due Mar 2
	if len(got) != 1 {
		t.Errorf("default lead should apply: %+v", got)
	}
}
