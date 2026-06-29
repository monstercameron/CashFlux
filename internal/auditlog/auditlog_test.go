// SPDX-License-Identifier: MIT

package auditlog_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
)

// TestClear verifies a data wipe can empty the session feed and that it's reusable
// afterward (a new entry appends cleanly).
func TestClear(t *testing.T) {
	l := auditlog.New(500)
	l.Append(mkEntry("ae-1", "user", "added", "transaction", "t1", "Added transaction t1", time.Unix(1000, 0)))
	l.Append(mkEntry("ae-2", "user", "deleted", "account", "a1", "Deleted account a1", time.Unix(2000, 0)))
	if l.Len() != 2 {
		t.Fatalf("Len before clear = %d, want 2", l.Len())
	}
	l.Clear()
	if l.Len() != 0 {
		t.Errorf("Len after clear = %d, want 0", l.Len())
	}
	if got := l.Recent(10); got != nil {
		t.Errorf("Recent after clear = %v, want nil", got)
	}
	// Still usable after clearing.
	l.Append(mkEntry("ae-3", "user", "added", "budget", "b1", "Added budget b1", time.Unix(3000, 0)))
	if l.Len() != 1 {
		t.Errorf("Len after re-append = %d, want 1", l.Len())
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func mkEntry(id, actor, action, eType, eID, summary string, at time.Time) auditlog.Entry {
	return auditlog.Entry{
		ID:         id,
		At:         at,
		Actor:      actor,
		Action:     action,
		EntityType: eType,
		EntityID:   eID,
		Summary:    summary,
	}
}

var t0 = time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)

// ─── Append / Recent ──────────────────────────────────────────────────────────

func TestRecent_empty(t *testing.T) {
	l := auditlog.New(0)
	if got := l.Recent(10); got != nil {
		t.Fatalf("expected nil for empty log, got %v", got)
	}
}

func TestRecent_reverseChronological(t *testing.T) {
	l := auditlog.New(0)
	for i := 0; i < 5; i++ {
		l.Append(mkEntry(fmt.Sprintf("e%d", i), "system", "added", "transaction", fmt.Sprintf("t%d", i), "tx added", t0.Add(time.Duration(i)*time.Minute)))
	}
	got := l.Recent(0) // all
	if len(got) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(got))
	}
	for i := 0; i < len(got)-1; i++ {
		if !got[i].At.After(got[i+1].At) {
			t.Errorf("entry %d (%v) not after entry %d (%v) — not reverse-chronological", i, got[i].At, i+1, got[i+1].At)
		}
	}
}

func TestRecent_limitsToN(t *testing.T) {
	l := auditlog.New(0)
	for i := 0; i < 10; i++ {
		l.Append(mkEntry(fmt.Sprintf("e%d", i), "system", "added", "account", fmt.Sprintf("a%d", i), "acc added", t0.Add(time.Duration(i)*time.Minute)))
	}
	got := l.Recent(3)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	// The 3 most recent (i=9, 8, 7) should be first.
	if got[0].ID != "e9" {
		t.Errorf("expected e9 first, got %s", got[0].ID)
	}
}

// ─── Cap enforcement ──────────────────────────────────────────────────────────

func TestAppend_dropOldestWhenAtCap(t *testing.T) {
	l := auditlog.New(3)
	for i := 0; i < 5; i++ {
		l.Append(mkEntry(fmt.Sprintf("e%d", i), "user", "updated", "budget", "b1", "budget changed", t0.Add(time.Duration(i)*time.Minute)))
	}
	if l.Len() != 3 {
		t.Fatalf("expected len 3 after overflow, got %d", l.Len())
	}
	got := l.Recent(0)
	// Should retain e4, e3, e2 (the newest 3).
	if got[0].ID != "e4" || got[1].ID != "e3" || got[2].ID != "e2" {
		t.Errorf("unexpected order after cap: %v %v %v", got[0].ID, got[1].ID, got[2].ID)
	}
}

// ─── ByEntity ─────────────────────────────────────────────────────────────────

func TestByEntity_filtersCorrectly(t *testing.T) {
	l := auditlog.New(0)
	l.Append(mkEntry("e1", "user", "added", "transaction", "txA", "tx A added", t0))
	l.Append(mkEntry("e2", "user", "updated", "account", "accX", "acc updated", t0.Add(time.Minute)))
	l.Append(mkEntry("e3", "user", "deleted", "transaction", "txA", "tx A deleted", t0.Add(2*time.Minute)))
	l.Append(mkEntry("e4", "user", "added", "transaction", "txB", "tx B added", t0.Add(3*time.Minute)))

	got := l.ByEntity("transaction", "txA")
	if len(got) != 2 {
		t.Fatalf("expected 2 entries for txA, got %d", len(got))
	}
	// Reverse-chronological: e3 before e1.
	if got[0].ID != "e3" || got[1].ID != "e1" {
		t.Errorf("unexpected order: %v %v", got[0].ID, got[1].ID)
	}
}

func TestByEntity_missingType(t *testing.T) {
	l := auditlog.New(0)
	l.Append(mkEntry("e1", "user", "added", "transaction", "t1", "added", t0))
	if got := l.ByEntity("goal", "t1"); got != nil {
		t.Errorf("expected nil for missing entityType, got %v", got)
	}
}

// ─── Redact ───────────────────────────────────────────────────────────────────

var redactTests = []struct {
	name  string
	input string
	want  string
}{
	{
		name:  "no secret",
		input: "Added transaction Coffee for $4.50",
		want:  "Added transaction Coffee for $4.50",
	},
	{
		name:  "openai key",
		input: "Settings updated: apiKey=sk-abcDEF1234567890",
		want:  "Settings updated: apiKey=[REDACTED]",
	},
	{
		name:  "openai key mid-sentence",
		input: "key sk-XYZ789 saved",
		want:  "key [REDACTED] saved",
	},
	{
		name:  "bearer token",
		input: "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9",
		want:  "Authorization: [REDACTED]",
	},
	{
		name:  "multiple patterns",
		input: "sk-abc123 and Bearer tok456",
		want:  "[REDACTED] and [REDACTED]",
	},
	{
		name:  "key at end of string",
		input: "saved key sk-end",
		want:  "saved key [REDACTED]",
	},
}

func TestRedact(t *testing.T) {
	for _, tc := range redactTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := auditlog.Redact(tc.input)
			if got != tc.want {
				t.Errorf("Redact(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ─── Concurrency smoke ────────────────────────────────────────────────────────

func TestLog_concurrentAppend(t *testing.T) {
	l := auditlog.New(0)
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		i := i
		go func() {
			l.Append(mkEntry(fmt.Sprintf("c%d", i), "system", "added", "task", fmt.Sprintf("tk%d", i), "task added", t0.Add(time.Duration(i)*time.Second)))
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if l.Len() != 50 {
		t.Errorf("expected 50 entries after concurrent appends, got %d", l.Len())
	}
}

func TestFilterByEntityType(t *testing.T) {
	entries := []auditlog.Entry{
		{EntityType: "transaction", EntityID: "t1"},
		{EntityType: "account", EntityID: "a1"},
		{EntityType: "transaction", EntityID: "t2"},
		{EntityType: "budget", EntityID: "b1"},
	}
	cases := []struct {
		name       string
		entityType string
		wantIDs    []string
	}{
		{"transactions only", "transaction", []string{"t1", "t2"}},
		{"single account", "account", []string{"a1"}},
		{"no match", "goal", []string{}},
		{"empty type returns all", "", []string{"t1", "a1", "t2", "b1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := auditlog.FilterByEntityType(entries, tc.entityType)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.wantIDs))
			}
			for i, id := range tc.wantIDs {
				if got[i].EntityID != id {
					t.Errorf("[%d] = %q, want %q", i, got[i].EntityID, id)
				}
			}
		})
	}
}
