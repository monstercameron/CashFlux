// SPDX-License-Identifier: MIT

package reportview

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// CategoryRow holds one unformatted category's table and histogram readings.
// AverageMinor uses the configured window length, including inactive months.
type CategoryRow struct {
	Spend                                                              reports.CategorySpend
	Amount, AverageMinor, SharePercent, WidthPercent, HistogramPercent int64
	Spark                                                              []int64
}

// CategoryReview supplies table rows, the first twelve histogram rows and their
// collapsed remainder. The same rollup drives amounts and monthly sparklines.
type CategoryReview struct {
	// Error is available to callers retaining a failed model in a view memo.
	Error                                         error
	Spending                                      []reports.CategorySpend
	Rows, Histogram                               []CategoryRow
	MaxMinor                                      int64
	RestMinor, RestSharePercent, RestWidthPercent int64
	RestCount                                     int
	UncategorizedMinor, UncategorizedPercent      int64
}

// BuildCategoryReview prepares report category readings without formatting or
// browser state. months is the full selected bucket count, not active months.
// Failed arithmetic returns an error rather than wrapping financial figures.
func BuildCategoryReview(spending []reports.CategorySpend, trends []reports.CategoryTrend, categories []domain.Category, rollup bool, months int, expense int64) (CategoryReview, error) {
	out := CategoryReview{Spending: spending}
	var series map[string][]int64
	if rollup {
		var err error
		out.Spending, err = reports.RollUpByParentChecked(spending, categories)
		if err != nil {
			return CategoryReview{}, err
		}
		series, err = reports.RollUpCategorySeries(trends, categories)
		if err != nil {
			return CategoryReview{}, err
		}
	} else {
		series = make(map[string][]int64, len(trends))
		for _, trend := range trends {
			series[trend.CategoryID] = trend.Spend
		}
	}
	for _, spend := range out.Spending {
		amount, err := money.Abs(spend.Amount)
		if err != nil {
			return CategoryReview{}, fmt.Errorf("category %q amount: %w", spend.CategoryID, err)
		}
		out.MaxMinor = max(out.MaxMinor, amount)
	}
	for _, spend := range out.Spending {
		if spend.Amount == 0 && spend.Prior == 0 {
			continue
		}
		amount, _ := money.Abs(spend.Amount) // checked in the maximum pass above
		row := CategoryRow{Spend: spend, Amount: amount, Spark: series[spend.CategoryID]}
		if months > 0 {
			row.AverageMinor = amount / int64(months)
		}
		var err error
		row.SharePercent, err = reportRatio(amount, expense)
		if err != nil {
			return CategoryReview{}, fmt.Errorf("category %q share: %w", spend.CategoryID, err)
		}
		row.WidthPercent, err = reportRatio(amount, out.MaxMinor)
		if err != nil {
			return CategoryReview{}, fmt.Errorf("category %q width: %w", spend.CategoryID, err)
		}
		row.HistogramPercent = max(row.WidthPercent, 1)
		out.Rows = append(out.Rows, row)
		if spend.CategoryID == "" {
			out.UncategorizedMinor, out.UncategorizedPercent = amount, row.SharePercent
		}
		if amount == 0 {
			continue
		}
		if len(out.Histogram) < 12 {
			out.Histogram = append(out.Histogram, row)
		} else {
			out.RestMinor, err = money.Add(out.RestMinor, amount)
			if err != nil {
				return CategoryReview{}, fmt.Errorf("category histogram remainder: %w", err)
			}
			out.RestCount++
		}
	}
	var err error
	out.RestSharePercent, err = reportRatio(out.RestMinor, expense)
	if err != nil {
		return CategoryReview{}, err
	}
	out.RestWidthPercent, err = reportRatio(out.RestMinor, out.MaxMinor)
	out.RestWidthPercent = max(out.RestWidthPercent, 1)
	return out, err
}

// TagRow adds the report's comparison and histogram readings to one tag total.
// Tags share the category scale; a tag's width can therefore exceed 100 percent.
type TagRow struct {
	reports.TagSpend
	HasDelta, PriorZero        bool
	DeltaPercent, WidthPercent int64
}

// BuildTagRows prepares all tag rows. A zero prior has no percentage change;
// positive current spend instead carries the existing "new" indication.
func BuildTagRows(tags []reports.TagSpend, maxCategory int64) ([]TagRow, error) {
	rows := make([]TagRow, 0, len(tags))
	for _, tag := range tags {
		row := TagRow{TagSpend: tag, HasDelta: tag.Prior > 0, PriorZero: tag.Prior == 0 && tag.Amount > 0}
		var err error
		if row.HasDelta {
			delta, subErr := money.Sub(tag.Amount, tag.Prior)
			if subErr != nil {
				return nil, subErr
			}
			row.DeltaPercent, err = money.MulDiv(delta, 100, tag.Prior, money.RoundTowardZero)
			if err != nil {
				return nil, err
			}
		}
		row.WidthPercent, err = reportRatio(tag.Amount, maxCategory)
		if err != nil {
			return nil, err
		}
		row.WidthPercent = max(row.WidthPercent, 1)
		rows = append(rows, row)
	}
	return rows, nil
}

func reportRatio(amount, total int64) (int64, error) {
	if total <= 0 {
		return 0, nil
	}
	return money.MulDiv(amount, 100, total, money.RoundTowardZero)
}

// DepositGroup combines equal resolved descriptions and amounts. Last is the
// most recent deposit date, while group ordering follows first appearance.
type DepositGroup struct {
	Description string
	Amount      int64
	Count       int
	Last        time.Time
}

// GroupDeposits folds repeat deposits without changing the source list. nameOf
// resolves a blank description exactly as the UI does, including translation.
// A structured key keeps punctuation and numeric descriptions unambiguous.
func GroupDeposits(deposits []reports.ExpenseItem, nameOf func(string) string) []DepositGroup {
	type key struct {
		description string
		amount      int64
	}
	index := make(map[key]int, len(deposits))
	var groups []DepositGroup
	for _, deposit := range deposits {
		description := deposit.Desc
		if description == "" && nameOf != nil {
			description = nameOf(deposit.CategoryID)
		}
		k := key{description, deposit.Amount}
		if i, ok := index[k]; ok {
			groups[i].Count++
			if deposit.Date.After(groups[i].Last) {
				groups[i].Last = deposit.Date
			}
		} else {
			index[k] = len(groups)
			groups = append(groups, DepositGroup{Description: description, Amount: deposit.Amount, Count: 1, Last: deposit.Date})
		}
	}
	return groups
}
