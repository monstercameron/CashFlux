// SPDX-License-Identifier: MIT

// Package txntoolbar contains platform-independent transaction toolbar decisions.
package txntoolbar

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

// Derived holds the uncollapsed scope, its conversion issues and summary. It must
// be derived from the ledger, never from a previous render's collapsed rows.
type Derived struct {
	Rows    []domain.Transaction
	Issues  []txnfilter.ConversionIssue
	View    txnview.View
	Presets txnfilter.PresetCounts
}

// Snapshot is the immutable handoff from a host derivation to toolbar children.
// Scope must include app, workspace, filter and conversion identity.
type Snapshot struct {
	Revision uint64
	Scope    string
	Value    Derived
}

// Matches rejects stale render props as well as values from another scope.
func (s *Snapshot) Matches(revision uint64, scope string) bool {
	return s != nil && s.Revision == revision && s.Scope == scope
}

// SummaryScope removes ordering and pagination, which cannot affect totals.
func SummaryScope(f txnfilter.Criteria) txnfilter.Criteria {
	f.Sort, f.Dir, f.Page, f.PageSize = "", "", 0, 0
	return f
}

// CategoryScope keeps the contextual scope but removes every category and flow
// selection. Multi-category picks and budget union scopes follow the same rule.
func CategoryScope(f txnfilter.Criteria) txnfilter.Criteria {
	f = SummaryScope(f)
	f.Flow, f.Category, f.Categories, f.Uncategorized, f.ScopeAny = "", "", "", false, false
	return f
}

// CategoryFiltered reports whether either category control is active.
func CategoryFiltered(f txnfilter.Criteria) bool {
	return f.Category != "" || f.Categories != "" || f.Uncategorized
}

// Derive filters a current immutable ledger snapshot and folds the uncollapsed
// rows in the requested order, without pagination. Filtering conversion failures
// are retained rather than recalculated. On arithmetic failure, rows and issues
// remain available for correction, but no partial monetary totals are returned.
func Derive(rows []domain.Transaction, f txnfilter.Criteria, labels txnfilter.Labels, rates currency.Rates, base string, now time.Time) (Derived, error) {
	f.Page, f.PageSize = 0, 0
	visible, issues := txnfilter.ApplyWithLabelsChecked(rows, f, labels)
	categoryRows := visible
	if categoryScope := CategoryScope(f); categoryScope != SummaryScope(f) {
		categoryRows = txnfilter.ApplyWithLabels(rows, categoryScope, labels)
	}
	counts := PresetCounts(rows, f, labels, base, now)
	view, err := txnview.SummarizeWithCategories(visible, categoryRows, rates, base)
	if err != nil {
		return Derived{Rows: visible, Issues: issues, Presets: counts, View: txnview.View{Count: len(visible), Categories: []txnview.CategoryTotal{}}}, fmt.Errorf("derive transaction toolbar: %w", err)
	}
	return Derived{Rows: visible, Issues: issues, View: view, Presets: counts}, nil
}

// PresetCounts retains the preset's widening semantics, which cannot be computed
// from already-filtered rows when the toggled dimension excluded some matches.
func PresetCounts(rows []domain.Transaction, f txnfilter.Criteria, labels txnfilter.Labels, base string, now time.Time) txnfilter.PresetCounts {
	start := dateutil.MonthStart(now)
	return txnfilter.CountPresetsWithLabels(rows, f, txnfilter.PresetScope{
		ReviewTag: reviewqueue.ReviewTag, LargeMinor: currency.MinorFromMajor(100, base),
		MonthStart: start, MonthEnd: dateutil.AddMonths(start, 1),
	}, labels)
}

// ToggleSelection selects exactly the displayed set, or clears a fully selected set.
func ToggleSelection(shown []domain.Transaction, selected map[string]bool) map[string]bool {
	all := len(shown) > 0
	for _, row := range shown {
		all = all && selected[row.ID]
	}
	result := make(map[string]bool)
	if !all {
		for _, row := range shown {
			result[row.ID] = true
		}
	}
	return result
}

// FutureTotal sums scheduled non-transfer cashflow using checked minor units.
// A failed conversion or an unrepresentable addition counts as omitted.
func FutureTotal(amounts []money.Money, rates currency.Rates, base string) (money.Money, int) {
	total, omitted := money.New(0, base), 0
	for _, amount := range amounts {
		converted, err := rates.Convert(amount, base)
		if err != nil {
			omitted++
			continue
		}
		next, err := money.Add(total.Amount, converted.Amount)
		if err != nil {
			omitted++
			continue
		}
		total.Amount = next
	}
	return total, omitted
}
