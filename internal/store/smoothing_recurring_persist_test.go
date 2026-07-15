// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestSmoothingRecurringRoundTrip verifies the XC3 SmoothIntoBudgets flag on a
// recurring survives export/import and the SQLite snapshot path losslessly.
func TestSmoothingRecurringRoundTrip(t *testing.T) {
	due := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	ds := Dataset{
		Recurring: []domain.Recurring{
			{
				ID: "r-ins", Label: "Insurance",
				Amount: money.New(-60000, "USD"), Cadence: domain.CadenceYearly,
				NextDue: due, CategoryID: "cat1", SmoothIntoBudgets: true,
			},
			{
				ID: "r-net", Label: "Netflix",
				Amount: money.New(-1600, "USD"), Cadence: domain.CadenceMonthly,
				NextDue: due, CategoryID: "cat1", SmoothIntoBudgets: false,
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

	var ins domain.Recurring
	for _, r := range imported.Recurring {
		if r.ID == "r-ins" {
			ins = r
		}
	}
	if !ins.SmoothIntoBudgets {
		t.Error("SmoothIntoBudgets flag lost on import")
	}
	if !ins.Smooths() {
		t.Error("imported yearly recurring should smooth")
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
	var found bool
	for _, r := range snap.Recurring {
		if r.ID == "r-ins" {
			found = true
			if !r.SmoothIntoBudgets {
				t.Error("snapshot lost SmoothIntoBudgets flag")
			}
		}
	}
	if !found {
		t.Fatal("recurring r-ins missing from snapshot")
	}
}
