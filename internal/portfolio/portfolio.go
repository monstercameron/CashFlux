// Package portfolio provides pure financial calculations for an investment
// portfolio: holding valuation, unrealized gain/loss, return percentages, and
// allocation weights by holding or asset class.
//
// All monetary amounts are expressed in minor currency units (e.g. cents for
// USD) as int64. Share quantities and prices per share use float64 because
// fractional shares are common. No external dependencies are required; the
// package is safe to test natively on any platform.
package portfolio

import (
	"math"
	"sort"
)

// Holding is the local input type describing a single investment position.
// Money values are in minor units (e.g. cents); Shares is fractional.
type Holding struct {
	Ticker                    string
	Name                      string
	Shares                    float64
	CostBasisMinor            int64
	CurrentPriceMinorPerShare int64
	AssetClass                string
	// SecurityType is the position's category (stock/etf/bond/…) as a plain string,
	// so the pure package stays free of the domain enum. Empty maps to "other".
	SecurityType string
	// Sector and Region are the two allocation dimensions the benchmark adds
	// beyond asset class and security type (C377). Both are free text the
	// household types, not a controlled vocabulary: the app has no market-data
	// feed to classify a position from, so pretending to a taxonomy it cannot
	// verify would be worse than letting people name their own buckets.
	//
	// Empty is not "other" here, it is UNCLASSIFIED — a meaningful state on a
	// dimension nothing fills in automatically, and one the view has to show
	// rather than hide, or a portfolio with two labelled holdings would read as
	// 100% classified.
	Sector string
	Region string
	// ExpenseRatioBps is the fund's annual expense ratio in basis points
	// (100 bps = 1.00%), zero when unknown or not applicable (C378).
	ExpenseRatioBps int
}

// HoldingValueMinor returns the current market value of h in minor units,
// computed as round(Shares × CurrentPriceMinorPerShare).
func HoldingValueMinor(h Holding) int64 {
	return int64(math.Round(h.Shares * float64(h.CurrentPriceMinorPerShare)))
}

// UnrealizedGainMinor returns the unrealized gain (or loss, if negative) of h
// in minor units: current value minus cost basis.
func UnrealizedGainMinor(h Holding) int64 {
	return HoldingValueMinor(h) - h.CostBasisMinor
}

// ReturnPct returns the percentage return of h relative to its cost basis.
// Returns 0 if CostBasisMinor is zero to avoid division by zero.
func ReturnPct(h Holding) float64 {
	if h.CostBasisMinor == 0 {
		return 0
	}
	gain := float64(UnrealizedGainMinor(h))
	return gain / float64(h.CostBasisMinor) * 100
}

// Summary aggregates portfolio-level totals.
type Summary struct {
	TotalValueMinor int64
	TotalCostMinor  int64
	TotalGainMinor  int64
	ReturnPct       float64
}

// PortfolioSummary computes the aggregate Summary over all holdings.
// Returns a zero Summary for an empty slice.
func PortfolioSummary(hs []Holding) Summary {
	var s Summary
	for _, h := range hs {
		s.TotalValueMinor += HoldingValueMinor(h)
		s.TotalCostMinor += h.CostBasisMinor
	}
	s.TotalGainMinor = s.TotalValueMinor - s.TotalCostMinor
	if s.TotalCostMinor != 0 {
		s.ReturnPct = float64(s.TotalGainMinor) / float64(s.TotalCostMinor) * 100
	}
	return s
}

// Weight represents the value and percentage weight of a labelled group within
// the portfolio.
type Weight struct {
	Label      string
	ValueMinor int64
	Pct        float64
}

// AllocationByHolding returns one Weight per holding, labelled by Ticker (or
// Name if Ticker is blank), sorted by ValueMinor descending. Pct is the
// holding's share of total portfolio value. Returns nil for an empty slice.
func AllocationByHolding(hs []Holding) []Weight {
	if len(hs) == 0 {
		return nil
	}
	var total int64
	weights := make([]Weight, len(hs))
	for i, h := range hs {
		v := HoldingValueMinor(h)
		label := h.Ticker
		if label == "" {
			label = h.Name
		}
		weights[i] = Weight{Label: label, ValueMinor: v}
		total += v
	}
	for i := range weights {
		if total != 0 {
			weights[i].Pct = float64(weights[i].ValueMinor) / float64(total) * 100
		}
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].ValueMinor > weights[j].ValueMinor
	})
	return weights
}

// AllocationByAssetClass groups holdings by AssetClass (blank maps to "other"),
// summing values, and returns one Weight per class sorted by ValueMinor
// descending. Pct is each class's share of total portfolio value. Returns nil
// for an empty slice.
func AllocationByAssetClass(hs []Holding) []Weight {
	if len(hs) == 0 {
		return nil
	}
	totals := make(map[string]int64)
	var total int64
	for _, h := range hs {
		cls := h.AssetClass
		if cls == "" {
			cls = "other"
		}
		v := HoldingValueMinor(h)
		totals[cls] += v
		total += v
	}
	weights := make([]Weight, 0, len(totals))
	for cls, v := range totals {
		var pct float64
		if total != 0 {
			pct = float64(v) / float64(total) * 100
		}
		weights = append(weights, Weight{Label: cls, ValueMinor: v, Pct: pct})
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].ValueMinor > weights[j].ValueMinor
	})
	return weights
}

// Unclassified is the bucket label for a holding with no value on a dimension
// that nothing fills in automatically (C377). It is deliberately distinct from
// "other": "other" says the position was classified and did not fit, while this
// says nobody has said yet — and a portfolio where most of the pie is
// unclassified should read that way rather than looking diversified.
const Unclassified = "unclassified"

// AllocationBySector groups holdings by Sector, and AllocationByRegion by
// Region; both bucket the unset ones under Unclassified (C377).
func AllocationBySector(hs []Holding) []Weight {
	return allocationBy(hs, func(h Holding) string { return h.Sector }, Unclassified)
}

// AllocationByRegion is AllocationBySector's twin for geography.
func AllocationByRegion(hs []Holding) []Weight {
	return allocationBy(hs, func(h Holding) string { return h.Region }, Unclassified)
}

// allocationBy is the shared grouping the four allocation views use: sum market
// value per bucket, express each as a percentage of the whole, and order by
// value. blank is the label an empty key falls into.
func allocationBy(hs []Holding, key func(Holding) string, blank string) []Weight {
	if len(hs) == 0 {
		return nil
	}
	totals := make(map[string]int64)
	var total int64
	for _, h := range hs {
		k := key(h)
		if k == "" {
			k = blank
		}
		v := HoldingValueMinor(h)
		totals[k] += v
		total += v
	}
	weights := make([]Weight, 0, len(totals))
	for k, v := range totals {
		var pct float64
		if total != 0 {
			pct = float64(v) / float64(total) * 100
		}
		weights = append(weights, Weight{Label: k, ValueMinor: v, Pct: pct})
	}
	sort.Slice(weights, func(i, j int) bool {
		if weights[i].ValueMinor != weights[j].ValueMinor {
			return weights[i].ValueMinor > weights[j].ValueMinor
		}
		return weights[i].Label < weights[j].Label // stable for equal values
	})
	return weights
}

// AllocationBySecurityType groups holdings by SecurityType (blank maps to "other"),
// summing market values, and returns one Weight per type sorted by ValueMinor descending.
// Pct is each type's share of total portfolio value. Returns nil for an empty slice.
func AllocationBySecurityType(hs []Holding) []Weight {
	if len(hs) == 0 {
		return nil
	}
	totals := make(map[string]int64)
	var total int64
	for _, h := range hs {
		st := h.SecurityType
		if st == "" {
			st = "other"
		}
		v := HoldingValueMinor(h)
		totals[st] += v
		total += v
	}
	weights := make([]Weight, 0, len(totals))
	for st, v := range totals {
		var pct float64
		if total != 0 {
			pct = float64(v) / float64(total) * 100
		}
		weights = append(weights, Weight{Label: st, ValueMinor: v, Pct: pct})
	}
	sort.Slice(weights, func(i, j int) bool {
		return weights[i].ValueMinor > weights[j].ValueMinor
	})
	return weights
}
