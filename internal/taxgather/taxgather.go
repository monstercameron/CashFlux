// SPDX-License-Identifier: MIT

// Package taxgather assembles a year-end tax-prep GATHERING for the assistant's
// "get me ready for taxes" tool (AG14). It sweeps a tax year for the figures a
// preparer asks for — deductible-category spending, charitable donations, and
// interest paid — and lists the GAPS that would slow a filing (deductible or
// donation entries with no receipt attached).
//
// It is deliberately humble: it GATHERS and organizes the user's own records for
// their own filing. It gives no tax advice, claims no completeness, and applies no
// jurisdiction rules — the user (or their preparer) decides what is actually
// deductible. It reuses the tested reports.DeductibleTotals / reports.YearTax so
// the figures match the app's own reports.
//
// Pure Go, no syscall/js: unit-tested on native Go.
package taxgather

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// Bucket is a summed line of gathered activity — a count of matching transactions
// and their absolute total in base-currency minor units.
type Bucket struct {
	Count int
	Total int64 // absolute, base-currency minor units
}

// Gap is one thing missing from the gathered records that the user should fix
// before filing — currently a deductible/donation entry with no receipt attached.
type Gap struct {
	TxnID  string
	Label  string // payee or description, for showing the user
	Amount int64  // absolute base-currency minor units
	Date   time.Time
	Reason string // plain-English, e.g. "no receipt attached"
}

// Summary is the full gathering for one tax year.
type Summary struct {
	Year         int
	Deductible   reports.DeductibleSummary // per deductible-category totals
	Charitable   Bucket                    // donations by tag/category keyword
	InterestPaid Bucket                    // interest by tag/category keyword
	Gaps         []Gap
}

// donationKeywords / interestKeywords match a category name or tag (case-insensitive
// substring) when a category isn't explicitly flagged deductible. Kept small and
// obvious so the classification is explainable.
var donationKeywords = []string{"charit", "donat", "tithe", "giving"}
var interestKeywords = []string{"interest"}

// Gather sweeps txns dated in [start, end) into a tax-year Summary: deductible
// totals (via the shared report), charitable donations and interest paid (matched
// by category-name / tag keyword), and a gap list of deductible-or-donation entries
// missing a receipt. year labels the summary; the bounds define the actual period.
func Gather(txns []domain.Transaction, cats []domain.Category, year int, start, end time.Time, rates currency.Rates) (Summary, error) {
	ded, err := reports.DeductibleTotals(txns, cats, start, end, rates)
	if err != nil {
		return Summary{}, err
	}
	catName := make(map[string]string, len(cats))
	deductibleCat := make(map[string]bool, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		if c.Deductible {
			deductibleCat[c.ID] = true
		}
	}

	s := Summary{Year: year, Deductible: ded}
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return Summary{}, err
		}
		amt := conv.Abs().Amount
		hay := strings.ToLower(catName[t.CategoryID] + " " + strings.Join(t.Tags, " "))

		isDonation := containsAny(hay, donationKeywords)
		if isDonation {
			s.Charitable.Count++
			s.Charitable.Total += amt
		}
		if containsAny(hay, interestKeywords) {
			s.InterestPaid.Count++
			s.InterestPaid.Total += amt
		}
		// Gap: a deductible-flagged or donation entry with no receipt attached is
		// the thing most likely to hold up a filing.
		if (deductibleCat[t.CategoryID] || isDonation) && len(t.Attachments) == 0 {
			s.Gaps = append(s.Gaps, Gap{
				TxnID:  t.ID,
				Label:  txnLabel(t),
				Amount: amt,
				Date:   t.Date,
				Reason: "no receipt attached",
			})
		}
	}
	return s, nil
}

// GatherCSV renders a Summary as CSV bytes: a deductible block (reusing the shared
// report's shape), then charitable and interest lines, then a gaps block. name
// resolves a category id to a label; amount renders minor-unit integers.
func GatherCSV(s Summary, name func(id string) string, amount func(int64) string) []byte {
	// Deductible section from the shared report, then append our extra lines.
	out := reports.DeductibleCSV(s.Deductible, name, amount)
	var b strings.Builder
	b.Write(out)
	b.WriteString("\n")
	b.WriteString("Charitable donations," + amount(s.Charitable.Total) + "\n")
	b.WriteString("Interest paid," + amount(s.InterestPaid.Total) + "\n")
	if len(s.Gaps) > 0 {
		b.WriteString("\nGaps to resolve,Date,Amount\n")
		for _, g := range s.Gaps {
			b.WriteString(csvField(g.Label+" ("+g.Reason+")") + "," + g.Date.Format("2006-01-02") + "," + amount(g.Amount) + "\n")
		}
	}
	return []byte(b.String())
}

// containsAny reports whether hay contains any of the (lowercase) needles.
func containsAny(hay string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

// txnLabel picks the best human label for a transaction.
func txnLabel(t domain.Transaction) string {
	if s := strings.TrimSpace(t.Payee); s != "" {
		return s
	}
	return strings.TrimSpace(t.Desc)
}

// csvField quotes a field that contains a comma or quote so the CSV stays valid.
func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
