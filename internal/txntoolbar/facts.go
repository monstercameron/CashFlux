// SPDX-License-Identifier: MIT

package txntoolbar

import (
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"strings"
	"time"
)

// Facts describes the secondary summary independently of any DOM.
type Facts struct {
	Shown, MovedRows, UnpairedRows, Unconverted, Excluded int
	Moved                                                 money.Money
	Issues                                                []txnfilter.ConversionIssue
	Lens                                                  string
}

// TransactionCount chooses the existing localized singular/plural label.
func (p Presentation) TransactionCount(n int) string {
	key := "r3n.transactionCountMany"
	if n == 1 {
		key = "r3n.transactionCountOne"
	}
	return p.Text(key, CountNumber(n))
}

// SummaryFacts produces an ordered, explainable account of omitted rows.
func (p Presentation) SummaryFacts(f Facts) []string {
	result := []string{p.TransactionCount(f.Shown)}
	if f.MovedRows > 0 {
		result = append(result, p.Text("transactions.scopeMoved", p.Money(f.Moved)))
		if f.UnpairedRows > 0 {
			result = append(result, p.Text("transactions.scopeUnpaired", f.UnpairedRows))
		}
	}
	if f.Unconverted > 0 {
		result = append(result, p.Text("transactions.scopeUnconverted", f.Unconverted))
	}
	for _, issue := range ConversionIssueCounts(f.Issues) {
		result = append(result, p.Text("tc2e.rowsCannotCompare", issue.Count, issue.Currency))
	}
	if f.Excluded > 0 {
		result = append(result, p.Text("transactions.scopeExcluded", f.Excluded))
	}
	if f.Lens != "" {
		result = append(result, p.Text("transactions.scopeLens", f.Lens))
	}
	return result
}

// WidenSearch offers all dates only for a search constrained by a named period.
func WidenSearch(f txnfilter.Criteria, now time.Time) (txnfilter.Criteria, bool) {
	preset := f.PeriodPreset
	if preset == "" {
		preset = txnfilter.InferPeriodPreset(f.From, f.To, now)
	}
	if strings.TrimSpace(f.Text) == "" || preset == txnfilter.PeriodAllDates || preset == txnfilter.PeriodCustom {
		return f, false
	}
	f.From, f.To, f.PeriodPreset = "", "", txnfilter.PeriodAllDates
	return f, true
}
