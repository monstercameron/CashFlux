// SPDX-License-Identifier: MIT

// Package txnview derives the shared, explainable transaction summary used by
// the transaction toolbar, count line, and table.
package txnview

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
)

// CategoryTotal is a net amount in the selected base currency. Positive values
// are spending; negative values are split credits or returns.
type CategoryTotal struct {
	ID    string
	Minor int64
}

// View contains figures and pairing facts derived from one visible row set.
type View struct {
	Spent, Income, Net, Transferred int64
	Count, Unconverted, Excluded    int
	MovedRows, UnpairedRows         int
	CountedTransfers                []bool
	Categories                      []CategoryTotal
	UnconvertedByCurrency           map[string]int
}

// Summarize derives totals, category amounts, and transfer pairing facts once.
// Excluded rows remain in Count and pairing facts, but do not affect analytical
// money totals or category amounts.
func Summarize(rows []domain.Transaction, rates currency.Rates, base string) (View, error) {
	return SummarizeWithCategories(rows, rows, rates, base)
}

// SummarizeWithCategories derives cashflow from rows and category spend from
// categoryRows. The latter normally clears category/flow filters while keeping
// the same date, search, member, and account scope.
func SummarizeWithCategories(rows, categoryRows []domain.Transaction, rates currency.Rates, base string) (View, error) {
	return SummarizeWithAccounts(rows, categoryRows, nil, rates, base)
}

// SummarizeWithAccounts derives cashflow and category spend while honoring each
// account's fixed booking sign convention for transfer pairing.
func SummarizeWithAccounts(rows, categoryRows []domain.Transaction, accounts []domain.Account, rates currency.Rates, base string) (View, error) {
	view := View{Count: len(rows), CountedTransfers: make([]bool, len(rows)), UnconvertedByCurrency: make(map[string]int)}
	pairRates := rates
	if strings.TrimSpace(base) != "" {
		pairRates.Base = base
	}
	// Pair only non-excluded rows so an excluded leg never suppresses a
	// non-excluded partner or inflates UnpairedRows.
	includedRows := make([]domain.Transaction, 0, len(rows))
	includedOrigIdx := make([]int, 0, len(rows))
	for i, row := range rows {
		if row.ExcludeFromReports {
			continue
		}
		includedRows = append(includedRows, row)
		includedOrigIdx = append(includedOrigIdx, i)
	}
	transfers := txnclassify.SummarizeVisibleTransfersWithAccounts(includedRows, accounts, pairRates)
	view.UnpairedRows = transfers.UnpairedRows
	for j, origIdx := range includedOrigIdx {
		if j < len(transfers.CountedRows) && transfers.CountedRows[j] {
			view.CountedTransfers[origIdx] = true
		}
	}
	for i, row := range rows {
		if row.ExcludeFromReports {
			view.Excluded++
			continue
		}
		converted, err := rates.Convert(row.Amount, base)
		if err != nil {
			if !errors.Is(err, currency.ErrUnknownRate) {
				return View{}, fmt.Errorf("convert transaction %q: %w", row.ID, err)
			}
			view.Unconverted++
			code := strings.ToUpper(strings.TrimSpace(row.Amount.Currency))
			if code == "" {
				code = "unknown"
			}
			view.UnconvertedByCurrency[code]++
			continue
		}
		if row.IsTransfer() {
			if view.CountedTransfers[i] {
				amount, absErr := money.Abs(converted.Amount)
				if absErr != nil {
					return View{}, fmt.Errorf("transfer %q magnitude: %w", row.ID, absErr)
				}
				view.Transferred, err = money.Add(view.Transferred, amount)
				if err != nil {
					return View{}, fmt.Errorf("sum transfers: %w", err)
				}
				view.MovedRows++
			}
			continue
		}
		if converted.Amount < 0 {
			amount, negErr := money.Neg(converted.Amount)
			if negErr != nil {
				return View{}, fmt.Errorf("expense %q magnitude: %w", row.ID, negErr)
			}
			view.Spent, err = money.Add(view.Spent, amount)
		} else {
			view.Income, err = money.Add(view.Income, converted.Amount)
		}
		if err != nil {
			return View{}, fmt.Errorf("sum transaction %q: %w", row.ID, err)
		}
	}
	net, err := money.Sub(view.Income, view.Spent)
	if err != nil {
		return View{}, fmt.Errorf("calculate net: %w", err)
	}
	view.Net = net
	view.Categories, err = categoryTotals(categoryRows, rates, base)
	if err != nil {
		return View{}, err
	}
	return view, nil
}

func categoryTotals(rows []domain.Transaction, rates currency.Rates, base string) ([]CategoryTotal, error) {
	categories := make(map[string]int64)
	for _, row := range rows {
		if row.ExcludeFromReports || row.IsTransfer() || row.Amount.Amount >= 0 {
			continue
		}
		if len(row.Splits) == 0 {
			converted, err := rates.Convert(row.Amount, base)
			if err != nil {
				if !errors.Is(err, currency.ErrUnknownRate) {
					return nil, fmt.Errorf("convert category expense %q: %w", row.ID, err)
				}
				continue
			}
			amount, err := money.Neg(converted.Amount)
			if err != nil {
				return nil, fmt.Errorf("category expense %q magnitude: %w", row.ID, err)
			}
			categories, err = addCategoryAmount(categories, row.CategoryID, amount)
			if err != nil {
				return nil, fmt.Errorf("category expense %q: %w", row.ID, err)
			}
			continue
		}
		splitTotals := make(map[string]int64, len(row.Splits))
		for _, split := range row.Splits {
			converted, err := rates.Convert(split.Amount, base)
			if err != nil {
				if !errors.Is(err, currency.ErrUnknownRate) {
					return nil, fmt.Errorf("convert category split for %q: %w", row.ID, err)
				}
				continue
			}
			splitTotals[split.CategoryID], err = money.Add(splitTotals[split.CategoryID], converted.Amount)
			if err != nil {
				return nil, fmt.Errorf("net category splits for %q (%q): %w", row.ID, split.CategoryID, err)
			}
		}
		for categoryID, net := range splitTotals {
			if net == 0 {
				continue
			}
			amount, err := money.Neg(net)
			if err != nil {
				return nil, fmt.Errorf("category split for %q magnitude: %w", row.ID, err)
			}
			nextCategories, err := addCategoryAmount(categories, categoryID, amount)
			if err != nil {
				return nil, fmt.Errorf("category split for %q: %w", row.ID, err)
			}
			categories = nextCategories
		}
	}
	result := make([]CategoryTotal, 0, len(categories))
	for id, amount := range categories {
		result = append(result, CategoryTotal{ID: id, Minor: amount})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Minor == result[j].Minor {
			return result[i].ID < result[j].ID
		}
		return result[i].Minor > result[j].Minor
	})
	return result, nil
}

// Scope returns a stable key for a filtered row set, ignoring ordering and page.
func Scope(criteria txnfilter.Criteria) string {
	criteria.Sort, criteria.Dir, criteria.Page, criteria.PageSize = "", "", 0, 0
	return fmt.Sprintf("%#v", criteria)
}

func addCategoryAmount(totals map[string]int64, id string, amount int64) (map[string]int64, error) {
	next, err := money.Add(totals[id], amount)
	if err != nil {
		return nil, fmt.Errorf("sum category %q: %w", id, err)
	}
	totals[id] = next
	return totals, nil
}

// Cache memoizes one derived view for a revision, normalized filter scope, exact
// ordered row identities, and conversion inputs. Row values are immutable within
// a revision; changed values must advance the revision.
type Cache struct {
	revision   uint64
	scope      string
	rows       string
	categories string
	conversion string
	view       View
	valid      bool
}

// Get returns the cached view for matching inputs, or derives it.
func (c *Cache) Get(revision uint64, scope string, rows []domain.Transaction, rates currency.Rates, base string) (View, error) {
	return c.GetWithCategories(revision, scope, rows, rows, rates, base)
}

// GetWithCategories returns or derives summary totals and category data for a
// single revision, filter scope, row identities, and conversion inputs.
func (c *Cache) GetWithCategories(revision uint64, scope string, rows, categoryRows []domain.Transaction, rates currency.Rates, base string) (View, error) {
	rowKey, categoryKey := rowIDsKey(rows), rowIDsKey(categoryRows)
	// fmt orders map keys and preserves float values, making this independent
	// of map identity and sensitive to in-place rate changes.
	conversionKey := fmt.Sprintf("%q|%#v", base, rates)
	if c.valid && c.revision == revision && c.scope == scope && c.rows == rowKey && c.categories == categoryKey && c.conversion == conversionKey {
		return cloneView(c.view), nil
	}
	view, err := SummarizeWithCategories(rows, categoryRows, rates, base)
	if err != nil {
		return View{}, err
	}
	c.revision, c.scope, c.view, c.valid = revision, scope, cloneView(view), true
	c.rows, c.categories, c.conversion = rowKey, categoryKey, conversionKey
	return view, nil
}

func cloneView(v View) View {
	if v.CountedTransfers != nil {
		v.CountedTransfers = append([]bool(nil), v.CountedTransfers...)
	}
	if v.Categories != nil {
		v.Categories = append([]CategoryTotal(nil), v.Categories...)
	}
	if v.UnconvertedByCurrency != nil {
		cp := make(map[string]int, len(v.UnconvertedByCurrency))
		for k, n := range v.UnconvertedByCurrency {
			cp[k] = n
		}
		v.UnconvertedByCurrency = cp
	}
	return v
}

// rowIDsKey is collision-free even when IDs contain separators. Order matters:
// CountedTransfers indexes the input slice, rather than an unordered set.
func rowIDsKey(rows []domain.Transaction) string {
	var key strings.Builder
	for _, row := range rows {
		key.WriteString(strconv.Itoa(len(row.ID)))
		key.WriteByte(':')
		key.WriteString(row.ID)
	}
	return key.String()
}

// Lookup returns a previously derived view when its revision and scope match.
func (c *Cache) Lookup(revision uint64, scope string) (View, bool) {
	if !c.valid || c.revision != revision || c.scope != scope {
		return View{}, false
	}
	return cloneView(c.view), true
}
