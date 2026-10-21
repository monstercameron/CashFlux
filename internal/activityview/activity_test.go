// SPDX-License-Identifier: MIT

package activityview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/auditlog"
	"github.com/monstercameron/CashFlux/internal/checkpoint"
	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestBuildAuditFeedOrdersHumanizesFiltersAndPreparesHero(t *testing.T) {
	newer := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	input := []auditlog.Entry{
		{ID: "older", At: older, Actor: "user", EntityType: "placements", Summary: "Added 20 placements records"},
		{ID: "newer-a", At: newer, Actor: "system", EntityType: "transaction", Summary: "Added transaction"},
		{ID: "newer-b", At: newer, Actor: "user", EntityType: "transaction", Summary: "Added transaction"},
		{ID: "undated", Actor: "user", EntityType: "settingsState", Summary: "Added settingsState legacy"},
	}
	model := Build(Input{AuditEntries: input, AuditAvailable: true, SnapshotAvailable: true, Limit: 3, Filter: "transaction"})
	if model.Source != SourceAuditFeed || !model.Available || !model.AuditAvailable || !model.SnapshotAvailable {
		t.Fatalf("source availability = %+v", model)
	}
	if len(model.Entries) != 4 { // audit-feed snapshots are already bounded by Feed.Recent; Limit applies only to fallback.
		t.Fatalf("audit entries got %d, want all supplied bounded entries", len(model.Entries))
	}
	if got := []string{model.Entries[0].ID, model.Entries[1].ID, model.Entries[2].ID, model.Entries[3].ID}; got[0] != "newer-a" || got[1] != "newer-b" || got[2] != "older" || got[3] != "undated" {
		t.Fatalf("stable dated order = %v", got)
	}
	if model.Entries[2].Summary != "Added 20 dashboard layout records" || model.Entries[3].EntityType != "settings" || model.Entries[3].Summary != "Added settings" {
		t.Fatalf("legacy humanization = %+v", model.Entries)
	}
	if model.Hero.Count != 4 || model.Hero.YouCount != 3 || model.Hero.OtherCount != 1 || model.Hero.Latest.ID != "newer-a" {
		t.Fatalf("hero = %+v", model.Hero)
	}
	if len(model.Rows) != 2 || model.Rows[0].Entry.ID != "newer-a" || model.Rows[1].Entry.ID != "newer-b" {
		t.Fatalf("filtered rows = %+v", model.Rows)
	}
	if len(model.Days) != 1 || len(model.Days[0].Rows) != 2 || model.Days[0].Key != "2026-10-21" {
		t.Fatalf("filtered day groups = %+v", model.Days)
	}
	if input[0].EntityType != "placements" || input[0].Summary != "Added 20 placements records" {
		t.Fatal("Build mutated caller-owned entries")
	}
}

func TestBuildFallbackSortsRedactsAndCaps(t *testing.T) {
	date := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	transactions := []domain.Transaction{
		{ID: "old", Date: date, Desc: "old"},
		{ID: "new", Date: date.Add(time.Hour), Payee: "vendor sk-secret"},
	}
	tasks := []domain.Task{{ID: "due", Title: "Due soon", Due: date.Add(2 * time.Hour)}, {ID: "undated", Title: "No due date"}}
	model := Build(Input{Transactions: transactions, Tasks: tasks, SnapshotAvailable: true, Limit: 2})
	if model.Source != SourceEntitySnapshot || !model.Available || model.AuditAvailable || !model.SnapshotAvailable {
		t.Fatalf("source availability = %+v", model)
	}
	if len(model.Entries) != 2 || model.Entries[0].ID != "task-due" || model.Entries[1].ID != "txn-new" {
		t.Fatalf("fallback cap/order = %+v", model.Entries)
	}
	if model.Entries[1].Summary != "Transaction: vendor [REDACTED]" {
		t.Fatalf("fallback summary = %q", model.Entries[1].Summary)
	}
}

func TestBuildAvailabilityIsIndependentAndUndoFactsArePrepared(t *testing.T) {
	when := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	entries := []auditlog.Entry{
		{ID: "earlier", At: when.Add(-time.Hour), EntityType: "transaction"},
		{ID: "latest", At: when, EntityType: "transaction"},
	}
	model := Build(Input{
		AuditEntries: entries, AuditAvailable: true, SnapshotAvailable: false,
		Checkpoints: []checkpoint.Checkpoint{{ID: "before", At: when.Add(-2 * time.Hour)}},
	})
	if !model.AuditAvailable || model.SnapshotAvailable || !model.Available {
		t.Fatalf("availability flags = %+v", model)
	}
	if len(model.Rows) != 2 || model.Rows[0].Entry.ID != "latest" {
		t.Fatalf("prepared rows = %+v", model.Rows)
	}
	if model.Rows[1].Undo.Route != "checkpoint" || model.Rows[1].Undo.AlsoReverses != 1 {
		t.Fatalf("prepared undo assessment = %+v", model.Rows[1].Undo)
	}
	empty := Build(Input{AuditAvailable: true, SnapshotAvailable: false})
	if empty.Available || empty.Source != SourceEntitySnapshot || len(empty.Entries) != 0 {
		t.Fatalf("unavailable empty snapshot = %+v", empty)
	}
}

func TestBuildFallbackPreservesStableTiesAndUndatedLast(t *testing.T) {
	date := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	model := Build(Input{
		Transactions: []domain.Transaction{{ID: "first", Date: date}, {ID: "second", Date: date}},
		Tasks:        []domain.Task{{ID: "undated"}}, SnapshotAvailable: true, Limit: 10,
	})
	got := []string{model.Entries[0].ID, model.Entries[1].ID, model.Entries[2].ID}
	if got[0] != "txn-first" || got[1] != "txn-second" || got[2] != "task-undated" {
		t.Fatalf("fallback stable ordering = %v", got)
	}
}
