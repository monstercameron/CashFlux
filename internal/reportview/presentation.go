// SPDX-License-Identifier: MIT

package reportview

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// CustomFieldRow contains the existing ranked custom-field total and its bar
// width, expressed as an integer percentage of the largest displayed value.
type CustomFieldRow struct {
	Spend        reports.CustomFieldSpend
	WidthPercent int
}

// CustomFieldReview contains the custom-field summary facts used by its report
// section. Rows preserves the full ranked list for export; DisplayRows omits the
// dominant unvalued bucket when that would drown the valued rows.
type CustomFieldReview struct {
	Rows             []reports.CustomFieldSpend
	DisplayRows      []CustomFieldRow
	TotalMinor       int64
	NoValueMinor     int64
	NoValuePercent   int
	AllUnvalued      bool
	DominantUnvalued bool
}

// BuildCustomFieldReview checks summary totals and prepares the exact integer
// bar/share percentages while preserving the source row order and floor rounding.
func BuildCustomFieldReview(rows []reports.CustomFieldSpend) (CustomFieldReview, error) {
	out := CustomFieldReview{
		Rows:        append([]reports.CustomFieldSpend(nil), rows...),
		DisplayRows: make([]CustomFieldRow, 0, len(rows)),
		AllUnvalued: len(rows) > 0,
	}
	for _, row := range rows {
		if row.Amount < 0 {
			return CustomFieldReview{}, fmt.Errorf("custom field value %q has negative spend: %w", row.Value, money.ErrOverflow)
		}
		var err error
		out.TotalMinor, err = money.Add(out.TotalMinor, row.Amount)
		if err != nil {
			return CustomFieldReview{}, fmt.Errorf("custom field total: %w", err)
		}
		if row.Value == "" {
			out.NoValueMinor, err = money.Add(out.NoValueMinor, row.Amount)
			if err != nil {
				return CustomFieldReview{}, fmt.Errorf("custom field unvalued total: %w", err)
			}
		} else {
			out.AllUnvalued = false
		}
	}
	if out.TotalMinor > 0 {
		share, err := reportRatio(out.NoValueMinor, out.TotalMinor)
		if err != nil {
			return CustomFieldReview{}, fmt.Errorf("custom field unvalued share: %w", err)
		}
		out.NoValuePercent = int(share)
		out.DominantUnvalued = !out.AllUnvalued && share >= 70
	}
	var maxAmount int64
	for _, row := range rows {
		if out.DominantUnvalued && row.Value == "" {
			continue
		}
		maxAmount = max(maxAmount, row.Amount)
	}
	for _, row := range rows {
		if out.DominantUnvalued && row.Value == "" {
			continue
		}
		width, err := reportRatio(row.Amount, maxAmount)
		if err != nil {
			return CustomFieldReview{}, fmt.Errorf("custom field value %q bar width: %w", row.Value, err)
		}
		if width > 100 {
			width = 100
		}
		out.DisplayRows = append(out.DisplayRows, CustomFieldRow{Spend: row, WidthPercent: int(width)})
	}
	return out, nil
}

// DeductibleRow contains the source row and its bar width relative to the
// largest deductible category amount.
type DeductibleRow struct {
	Row          reports.DeductibleRow
	WidthPercent int
}

// DeductibleReview contains checked chart rows alongside the checked service
// summary. Rows retain the service's descending amount and ID tie-break order.
type DeductibleReview struct {
	Summary reports.DeductibleSummary
	Rows    []DeductibleRow
}

// BuildDeductibleReview validates the summary total and prepares bounded bar
// widths using the same truncation as the prior report presentation.
func BuildDeductibleReview(summary reports.DeductibleSummary) (DeductibleReview, error) {
	ownedSummary := summary
	ownedSummary.Rows = append([]reports.DeductibleRow(nil), summary.Rows...)
	out := DeductibleReview{Summary: ownedSummary, Rows: make([]DeductibleRow, 0, len(summary.Rows))}
	var total, maxAmount int64
	for _, row := range summary.Rows {
		if row.Amount < 0 {
			return DeductibleReview{}, fmt.Errorf("deductible category %q has negative spend: %w", row.CategoryID, money.ErrOverflow)
		}
		var err error
		total, err = money.Add(total, row.Amount)
		if err != nil {
			return DeductibleReview{}, fmt.Errorf("deductible rows total: %w", err)
		}
		maxAmount = max(maxAmount, row.Amount)
	}
	if total != summary.Total {
		return DeductibleReview{}, fmt.Errorf("deductible row total %d differs from summary total %d", total, summary.Total)
	}
	for _, row := range summary.Rows {
		width, err := reportRatio(row.Amount, maxAmount)
		if err != nil {
			return DeductibleReview{}, fmt.Errorf("deductible category %q bar width: %w", row.CategoryID, err)
		}
		if width > 100 {
			width = 100
		}
		out.Rows = append(out.Rows, DeductibleRow{Row: row, WidthPercent: int(width)})
	}
	return out, nil
}

// InvestmentRow contains one checked investment performance row and the safe
// magnitude used to describe a negative cost basis.
type InvestmentRow struct {
	Performance       reports.AccountPerformance
	InvestedMagnitude int64
}

// InvestmentReview contains checked totals and the rounded total return in
// tenths of a percent, ready for formatting by the view.
type InvestmentReview struct {
	Rows                 []InvestmentRow
	TotalInvested        int64
	TotalCurrent         int64
	TotalGain            int64
	TotalReturnTenthsPct int64
	HasReturn            bool
}

// BuildInvestmentReview aggregates the service's rows once and checks every
// total and ratio before a section is allowed to present its headline figures.
func BuildInvestmentReview(rows []reports.AccountPerformance) (InvestmentReview, error) {
	out := InvestmentReview{Rows: make([]InvestmentRow, 0, len(rows))}
	for _, row := range rows {
		var err error
		out.TotalInvested, err = money.Add(out.TotalInvested, row.Invested)
		if err != nil {
			return InvestmentReview{}, fmt.Errorf("investment cost basis total: %w", err)
		}
		out.TotalCurrent, err = money.Add(out.TotalCurrent, row.Current)
		if err != nil {
			return InvestmentReview{}, fmt.Errorf("investment current value total: %w", err)
		}
		out.TotalGain, err = money.Add(out.TotalGain, row.Gain)
		if err != nil {
			return InvestmentReview{}, fmt.Errorf("investment gain total: %w", err)
		}
		magnitude := row.Invested
		if magnitude < 0 {
			magnitude, err = money.Abs(magnitude)
			if err != nil {
				return InvestmentReview{}, fmt.Errorf("investment %q cost basis magnitude: %w", row.AccountID, err)
			}
		}
		out.Rows = append(out.Rows, InvestmentRow{Performance: row, InvestedMagnitude: magnitude})
	}
	if out.TotalInvested > 0 {
		// The section previously formatted its total with %.1f, whose exact
		// halfway cases round to even. Keep that rule without a float ratio.
		ret, err := money.MulDiv(out.TotalGain, 1000, out.TotalInvested, money.RoundHalfEven)
		if err != nil {
			return InvestmentReview{}, fmt.Errorf("investment total return: %w", err)
		}
		out.TotalReturnTenthsPct = ret
		out.HasReturn = true
	}
	return out, nil
}
