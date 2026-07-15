// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/savedtxnview"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

func TestSaveTxnViewRoundTrip(t *testing.T) {
	a := newApp(t, false)

	v, err := a.SaveTxnView("Amazon this month", txnfilter.Criteria{Text: "amazon"}, 50000)
	if err != nil {
		t.Fatalf("SaveTxnView: %v", err)
	}
	if v.ID == "" {
		t.Fatal("expected an assigned id")
	}

	got := a.SavedTxnViews()
	if len(got) != 1 {
		t.Fatalf("SavedTxnViews len = %d, want 1", len(got))
	}
	if got[0].Name != "Amazon this month" || got[0].Criteria.Text != "amazon" || got[0].Threshold != 50000 {
		t.Fatalf("round-trip mismatch: %+v", got[0])
	}

	// Re-read via the single-view lookup.
	if fetched, ok := a.SavedTxnView(v.ID); !ok || fetched.Name != v.Name {
		t.Fatalf("SavedTxnView lookup failed: %+v, ok=%v", fetched, ok)
	}
}

func TestSaveTxnViewValidation(t *testing.T) {
	a := newApp(t, false)

	if _, err := a.SaveTxnView("  ", txnfilter.Criteria{}, 0); err != savedtxnview.ErrNameRequired {
		t.Fatalf("blank name err = %v, want ErrNameRequired", err)
	}

	if _, err := a.SaveTxnView("Fees", txnfilter.Criteria{Category: "fees"}, 0); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if _, err := a.SaveTxnView("fees", txnfilter.Criteria{Category: "other"}, 0); err != savedtxnview.ErrNameTaken {
		t.Fatalf("duplicate name err = %v, want ErrNameTaken", err)
	}
}

func TestUpdateAndDeleteTxnView(t *testing.T) {
	a := newApp(t, false)
	v, err := a.SaveTxnView("Cash over 100", txnfilter.Criteria{AmountMin: "100"}, 0)
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	v.Threshold = 20000
	v.Name = "Cash over $100"
	if err := a.UpdateTxnView(v); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := a.SavedTxnView(v.ID)
	if got.Threshold != 20000 || got.Name != "Cash over $100" {
		t.Fatalf("update not persisted: %+v", got)
	}

	// Updating an unknown id is rejected, not a silent insert.
	if err := a.UpdateTxnView(savedtxnview.SavedTxnView{ID: "nope", Name: "x"}); err != savedtxnview.ErrNotFound {
		t.Fatalf("update unknown err = %v, want ErrNotFound", err)
	}

	if err := a.DeleteTxnView(v.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(a.SavedTxnViews()) != 0 {
		t.Fatal("view not deleted")
	}
}
