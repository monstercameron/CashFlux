// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestBudgetPeriodNotesRoundTrip verifies the BG16 per-period notes journal on a
// budget survives export/import and the SQLite snapshot path losslessly.
func TestBudgetPeriodNotesRoundTrip(t *testing.T) {
	ds := Dataset{
		Budgets: []domain.Budget{
			{
				ID: "b1", Name: "Groceries", CategoryID: "cat1",
				Period: domain.PeriodMonthly, Limit: money.New(40000, "USD"),
				PeriodNotes: map[string]string{
					"2026-12-01": "December was high because we hosted",
					"2026-11-01": "quiet month",
				},
			},
		},
	}

	exported, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(exported)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	reexported, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(exported, reexported) {
		t.Errorf("round-trip not lossless:\nfirst:\n%s\nsecond:\n%s", exported, reexported)
	}
	if got := imported.Budgets[0].PeriodNotes["2026-12-01"]; got != "December was high because we hosted" {
		t.Errorf("imported Dec note = %q", got)
	}

	// SQLite snapshot path.
	st, err := NewMemory()
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer st.Close()
	if err := st.Load(imported); err != nil {
		t.Fatalf("load: %v", err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Budgets) != 1 {
		t.Fatalf("want 1 budget in snapshot, got %d", len(snap.Budgets))
	}
	if got := snap.Budgets[0].PeriodNotes["2026-11-01"]; got != "quiet month" {
		t.Errorf("snapshot Nov note = %q", got)
	}
}
