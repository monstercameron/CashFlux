// SPDX-License-Identifier: MIT

package store

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestHoldingCRUD(t *testing.T) {
	s := newStore(t)

	h := domain.Holding{
		ID:                        "h1",
		AccountID:                 "acct1",
		Ticker:                    "AAPL",
		Name:                      "Apple Inc.",
		Shares:                    10.5,
		CostBasisMinor:            150000, // $1,500.00
		CurrentPriceMinorPerShare: 17500,  // $175.00
		AssetClass:                "Stocks",
	}

	if err := s.PutHolding(h); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, ok, err := s.GetHolding("h1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Ticker != "AAPL" || got.Name != "Apple Inc." || got.Shares != 10.5 ||
		got.CostBasisMinor != 150000 || got.CurrentPriceMinorPerShare != 17500 ||
		got.AssetClass != "Stocks" || got.AccountID != "acct1" {
		t.Errorf("Get returned wrong values: %+v", got)
	}

	list, err := s.ListHoldings()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: len=%d err=%v", len(list), err)
	}

	// Update: change price.
	h.CurrentPriceMinorPerShare = 18000
	if err := s.PutHolding(h); err != nil {
		t.Fatalf("Put (update): %v", err)
	}
	got2, _, _ := s.GetHolding("h1")
	if got2.CurrentPriceMinorPerShare != 18000 {
		t.Errorf("Update not persisted: price = %d, want 18000", got2.CurrentPriceMinorPerShare)
	}

	deleted, err := s.DeleteHolding("h1")
	if err != nil || !deleted {
		t.Fatalf("Delete: deleted=%v err=%v", deleted, err)
	}
	if _, ok, _ := s.GetHolding("h1"); ok {
		t.Error("holding still present after delete")
	}
}

func TestDatasetHoldingRoundTrip(t *testing.T) {
	ds := sampleDataset()
	ds.Holdings = []domain.Holding{
		{
			ID:                        "h1",
			AccountID:                 "inv1",
			Ticker:                    "VTI",
			Name:                      "Vanguard Total Stock Market ETF",
			Shares:                    25.0,
			CostBasisMinor:            500000,
			CurrentPriceMinorPerShare: 22000,
			AssetClass:                "Stocks",
		},
	}

	first, err := Export(ds)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	imported, err := Import(first)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	second, err := Export(imported)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("holding round-trip not lossless")
	}

	if len(imported.Holdings) != 1 {
		t.Fatalf("holdings lost: got %d", len(imported.Holdings))
	}
	h := imported.Holdings[0]
	if h.ID != "h1" || h.Ticker != "VTI" || h.Shares != 25.0 ||
		h.CostBasisMinor != 500000 || h.CurrentPriceMinorPerShare != 22000 ||
		h.AssetClass != "Stocks" {
		t.Errorf("holding field mismatch: %+v", h)
	}
}
