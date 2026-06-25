package portfolio

import (
	"math"
	"testing"
)

// almostEqual compares two float64 values within a small tolerance.
func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// ---------------------------------------------------------------------------
// HoldingValueMinor
// ---------------------------------------------------------------------------

func TestHoldingValueMinor(t *testing.T) {
	tests := []struct {
		name   string
		h      Holding
		wantV  int64
	}{
		{
			name:  "whole shares",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 5000},
			wantV: 50000,
		},
		{
			name:  "fractional shares rounds up",
			h:     Holding{Shares: 1.5, CurrentPriceMinorPerShare: 3333},
			wantV: 5000, // 1.5 * 3333 = 4999.5 → round → 5000
		},
		{
			name:  "fractional shares rounds down",
			h:     Holding{Shares: 2.3, CurrentPriceMinorPerShare: 100},
			wantV: 230, // 2.3 * 100 = 230.0 exact
		},
		{
			name:  "zero shares",
			h:     Holding{Shares: 0, CurrentPriceMinorPerShare: 9999},
			wantV: 0,
		},
		{
			name:  "zero price",
			h:     Holding{Shares: 100, CurrentPriceMinorPerShare: 0},
			wantV: 0,
		},
		{
			name:  "banker rounding — 0.5 rounds to nearest even (2)",
			h:     Holding{Shares: 0.5, CurrentPriceMinorPerShare: 3},
			// 0.5 * 3 = 1.5 → math.Round → 2
			wantV: 2,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := HoldingValueMinor(tc.h)
			if got != tc.wantV {
				t.Errorf("HoldingValueMinor(%+v) = %d; want %d", tc.h, got, tc.wantV)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UnrealizedGainMinor
// ---------------------------------------------------------------------------

func TestUnrealizedGainMinor(t *testing.T) {
	tests := []struct {
		name  string
		h     Holding
		wantG int64
	}{
		{
			name:  "positive gain",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 6000, CostBasisMinor: 50000},
			wantG: 10000, // value=60000, cost=50000
		},
		{
			name:  "loss (negative gain)",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 4000, CostBasisMinor: 50000},
			wantG: -10000, // value=40000, cost=50000
		},
		{
			name:  "breakeven",
			h:     Holding{Shares: 5, CurrentPriceMinorPerShare: 2000, CostBasisMinor: 10000},
			wantG: 0,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := UnrealizedGainMinor(tc.h)
			if got != tc.wantG {
				t.Errorf("UnrealizedGainMinor(%+v) = %d; want %d", tc.h, got, tc.wantG)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ReturnPct
// ---------------------------------------------------------------------------

func TestReturnPct(t *testing.T) {
	const tol = 1e-9
	tests := []struct {
		name  string
		h     Holding
		wantP float64
	}{
		{
			name:  "20% gain",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 6000, CostBasisMinor: 50000},
			wantP: 20.0,
		},
		{
			name:  "20% loss",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 4000, CostBasisMinor: 50000},
			wantP: -20.0,
		},
		{
			name:  "zero cost basis returns 0",
			h:     Holding{Shares: 10, CurrentPriceMinorPerShare: 5000, CostBasisMinor: 0},
			wantP: 0,
		},
		{
			name:  "breakeven is 0%",
			h:     Holding{Shares: 5, CurrentPriceMinorPerShare: 2000, CostBasisMinor: 10000},
			wantP: 0.0,
		},
		{
			name:  "100% gain",
			h:     Holding{Shares: 1, CurrentPriceMinorPerShare: 20000, CostBasisMinor: 10000},
			wantP: 100.0,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ReturnPct(tc.h)
			if !almostEqual(got, tc.wantP, tol) {
				t.Errorf("ReturnPct(%+v) = %f; want %f", tc.h, got, tc.wantP)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PortfolioSummary
// ---------------------------------------------------------------------------

func TestPortfolioSummary(t *testing.T) {
	const tol = 1e-9

	t.Run("empty slice returns zero summary", func(t *testing.T) {
		s := PortfolioSummary(nil)
		if s.TotalValueMinor != 0 || s.TotalCostMinor != 0 || s.TotalGainMinor != 0 || s.ReturnPct != 0 {
			t.Errorf("expected zero Summary, got %+v", s)
		}
	})

	t.Run("single holding", func(t *testing.T) {
		hs := []Holding{
			{Shares: 10, CurrentPriceMinorPerShare: 6000, CostBasisMinor: 50000},
		}
		s := PortfolioSummary(hs)
		if s.TotalValueMinor != 60000 {
			t.Errorf("TotalValueMinor = %d; want 60000", s.TotalValueMinor)
		}
		if s.TotalCostMinor != 50000 {
			t.Errorf("TotalCostMinor = %d; want 50000", s.TotalCostMinor)
		}
		if s.TotalGainMinor != 10000 {
			t.Errorf("TotalGainMinor = %d; want 10000", s.TotalGainMinor)
		}
		if !almostEqual(s.ReturnPct, 20.0, tol) {
			t.Errorf("ReturnPct = %f; want 20.0", s.ReturnPct)
		}
	})

	t.Run("multiple holdings mixed gain and loss", func(t *testing.T) {
		hs := []Holding{
			// value=60000, cost=50000, gain=+10000
			{Shares: 10, CurrentPriceMinorPerShare: 6000, CostBasisMinor: 50000},
			// value=40000, cost=50000, gain=-10000
			{Shares: 10, CurrentPriceMinorPerShare: 4000, CostBasisMinor: 50000},
			// value=20000, cost=10000, gain=+10000
			{Shares: 2, CurrentPriceMinorPerShare: 10000, CostBasisMinor: 10000},
		}
		s := PortfolioSummary(hs)
		if s.TotalValueMinor != 120000 {
			t.Errorf("TotalValueMinor = %d; want 120000", s.TotalValueMinor)
		}
		if s.TotalCostMinor != 110000 {
			t.Errorf("TotalCostMinor = %d; want 110000", s.TotalCostMinor)
		}
		if s.TotalGainMinor != 10000 {
			t.Errorf("TotalGainMinor = %d; want 10000", s.TotalGainMinor)
		}
		wantPct := float64(10000) / float64(110000) * 100
		if !almostEqual(s.ReturnPct, wantPct, tol) {
			t.Errorf("ReturnPct = %f; want %f", s.ReturnPct, wantPct)
		}
	})

	t.Run("zero total cost guards ReturnPct", func(t *testing.T) {
		hs := []Holding{
			{Shares: 5, CurrentPriceMinorPerShare: 1000, CostBasisMinor: 0},
		}
		s := PortfolioSummary(hs)
		if s.ReturnPct != 0 {
			t.Errorf("ReturnPct = %f; want 0 when TotalCostMinor=0", s.ReturnPct)
		}
	})
}

// ---------------------------------------------------------------------------
// AllocationByHolding
// ---------------------------------------------------------------------------

func TestAllocationByHolding(t *testing.T) {
	const tol = 1e-6

	t.Run("nil slice returns nil", func(t *testing.T) {
		w := AllocationByHolding(nil)
		if w != nil {
			t.Errorf("expected nil, got %v", w)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		w := AllocationByHolding([]Holding{})
		if w != nil {
			t.Errorf("expected nil, got %v", w)
		}
	})

	t.Run("percentages sum to ~100", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "AAPL", Shares: 10, CurrentPriceMinorPerShare: 18000, CostBasisMinor: 150000},
			{Ticker: "MSFT", Shares: 5, CurrentPriceMinorPerShare: 40000, CostBasisMinor: 180000},
			{Ticker: "GOOG", Shares: 2, CurrentPriceMinorPerShare: 150000, CostBasisMinor: 250000},
		}
		weights := AllocationByHolding(hs)
		var sum float64
		for _, w := range weights {
			sum += w.Pct
		}
		if !almostEqual(sum, 100.0, tol) {
			t.Errorf("Pct sum = %f; want ~100", sum)
		}
	})

	t.Run("sorted by value descending", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "A", Shares: 1, CurrentPriceMinorPerShare: 100},
			{Ticker: "B", Shares: 1, CurrentPriceMinorPerShare: 300},
			{Ticker: "C", Shares: 1, CurrentPriceMinorPerShare: 200},
		}
		weights := AllocationByHolding(hs)
		if len(weights) != 3 {
			t.Fatalf("expected 3 weights, got %d", len(weights))
		}
		if weights[0].Label != "B" || weights[1].Label != "C" || weights[2].Label != "A" {
			t.Errorf("unexpected order: %v", weights)
		}
	})

	t.Run("label falls back to Name when Ticker empty", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "", Name: "Cash Fund", Shares: 1, CurrentPriceMinorPerShare: 10000},
		}
		weights := AllocationByHolding(hs)
		if len(weights) != 1 || weights[0].Label != "Cash Fund" {
			t.Errorf("expected label 'Cash Fund', got %v", weights)
		}
	})

	t.Run("all zero values — Pct stays 0", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "X", Shares: 0, CurrentPriceMinorPerShare: 0},
		}
		weights := AllocationByHolding(hs)
		if weights[0].Pct != 0 {
			t.Errorf("expected Pct=0 for zero total, got %f", weights[0].Pct)
		}
	})
}

// ---------------------------------------------------------------------------
// AllocationByAssetClass
// ---------------------------------------------------------------------------

func TestAllocationByAssetClass(t *testing.T) {
	const tol = 1e-6

	t.Run("nil slice returns nil", func(t *testing.T) {
		w := AllocationByAssetClass(nil)
		if w != nil {
			t.Errorf("expected nil, got %v", w)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		w := AllocationByAssetClass([]Holding{})
		if w != nil {
			t.Errorf("expected nil, got %v", w)
		}
	})

	t.Run("blank AssetClass maps to other", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "CASH", Shares: 1, CurrentPriceMinorPerShare: 5000, AssetClass: ""},
		}
		weights := AllocationByAssetClass(hs)
		if len(weights) != 1 || weights[0].Label != "other" {
			t.Errorf("expected label 'other', got %v", weights)
		}
	})

	t.Run("groups and sums by asset class", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "AAPL", Shares: 10, CurrentPriceMinorPerShare: 10000, AssetClass: "equity"},
			{Ticker: "MSFT", Shares: 5, CurrentPriceMinorPerShare: 10000, AssetClass: "equity"},
			{Ticker: "BND", Shares: 20, CurrentPriceMinorPerShare: 5000, AssetClass: "bond"},
			{Ticker: "CASH", Shares: 1, CurrentPriceMinorPerShare: 100000, AssetClass: ""},
		}
		// equity: 100000+50000=150000, bond: 100000, other: 100000
		// total: 350000
		weights := AllocationByAssetClass(hs)

		byLabel := make(map[string]Weight)
		for _, w := range weights {
			byLabel[w.Label] = w
		}

		if byLabel["equity"].ValueMinor != 150000 {
			t.Errorf("equity ValueMinor = %d; want 150000", byLabel["equity"].ValueMinor)
		}
		if byLabel["bond"].ValueMinor != 100000 {
			t.Errorf("bond ValueMinor = %d; want 100000", byLabel["bond"].ValueMinor)
		}
		if byLabel["other"].ValueMinor != 100000 {
			t.Errorf("other ValueMinor = %d; want 100000", byLabel["other"].ValueMinor)
		}

		var sum float64
		for _, w := range weights {
			sum += w.Pct
		}
		if !almostEqual(sum, 100.0, tol) {
			t.Errorf("Pct sum = %f; want ~100", sum)
		}
	})

	t.Run("sorted by value descending", func(t *testing.T) {
		hs := []Holding{
			{Ticker: "A", Shares: 1, CurrentPriceMinorPerShare: 100, AssetClass: "small"},
			{Ticker: "B", Shares: 1, CurrentPriceMinorPerShare: 300, AssetClass: "large"},
			{Ticker: "C", Shares: 1, CurrentPriceMinorPerShare: 200, AssetClass: "medium"},
		}
		weights := AllocationByAssetClass(hs)
		if weights[0].Label != "large" || weights[1].Label != "medium" || weights[2].Label != "small" {
			t.Errorf("unexpected order: %v", weights)
		}
	})
}
