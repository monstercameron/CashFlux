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
// deductible. It reuses reports.SpendingByCategoryChecked so category selection,
// report exclusions and checked split arithmetic match the shared report fold.
//
// Pure Go, no syscall/js: unit-tested on native Go.
package taxgather

import (
	"fmt"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/csvsafe"
	"github.com/monstercameron/CashFlux/internal/money"
	"sort"
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
	return gatherLedger(budgeting.NewNettedLedger(txns), cats, year, start, end, rates)
}

// gatherLedger reuses the checked report fold once per transaction so category
// totals, keyword buckets and receipt gaps share exactly the same selected
// expense lines. Buckets can overlap; counts are distinct matching transactions.
func gatherLedger(all budgeting.NettedLedger, cats []domain.Category, year int, start, end time.Time, rates currency.Rates) (Summary, error) {
	if year < 1 || year > 9999 || !start.Before(end) {
		return Summary{}, fmt.Errorf("invalid tax year or gathering window")
	}
	if strings.TrimSpace(rates.Base) == "" {
		return Summary{}, fmt.Errorf("tax gathering base currency is unavailable")
	}
	catName := map[string]string{}
	deductible := map[string]bool{}
	for _, c := range cats {
		catName[c.ID] = c.Name
		deductible[c.ID] = c.Deductible
	}
	s := Summary{Year: year}
	totals := map[string]int64{}
	for _, t := range all {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		rows, err := reports.SpendingByCategoryChecked(budgeting.NettedLedger{t}, start, end, rates)
		if err != nil {
			return Summary{}, fmt.Errorf("transaction %q tax records unavailable: %w", t.ID, err)
		}
		donationTagged := containsAny(strings.ToLower(strings.Join(t.Tags, " ")), donationKeywords)
		interestTagged := containsAny(strings.ToLower(strings.Join(t.Tags, " ")), interestKeywords)
		var donation, interest, gap int64
		for _, row := range rows {
			isDonation := donationTagged || containsAny(strings.ToLower(catName[row.CategoryID]), donationKeywords)
			isInterest := interestTagged || containsAny(strings.ToLower(catName[row.CategoryID]), interestKeywords)
			if deductible[row.CategoryID] {
				totals[row.CategoryID], err = money.Add(totals[row.CategoryID], row.Amount)
				if err != nil {
					return Summary{}, fmt.Errorf("deductible category %q: %w", row.CategoryID, err)
				}
				s.Deductible.Total, err = money.Add(s.Deductible.Total, row.Amount)
				if err != nil {
					return Summary{}, fmt.Errorf("deductible spending: %w", err)
				}
			}
			if isDonation {
				donation, err = money.Add(donation, row.Amount)
				if err != nil {
					return Summary{}, fmt.Errorf("donation transaction %q: %w", t.ID, err)
				}
			}
			if isInterest {
				interest, err = money.Add(interest, row.Amount)
				if err != nil {
					return Summary{}, fmt.Errorf("interest transaction %q: %w", t.ID, err)
				}
			}
			if deductible[row.CategoryID] || isDonation {
				gap, err = money.Add(gap, row.Amount)
				if err != nil {
					return Summary{}, fmt.Errorf("receipt gap %q: %w", t.ID, err)
				}
			}
		}
		if donation > 0 {
			s.Charitable.Count++
			s.Charitable.Total, err = money.Add(s.Charitable.Total, donation)
			if err != nil {
				return Summary{}, fmt.Errorf("charitable total: %w", err)
			}
		}
		if interest > 0 {
			s.InterestPaid.Count++
			s.InterestPaid.Total, err = money.Add(s.InterestPaid.Total, interest)
			if err != nil {
				return Summary{}, fmt.Errorf("interest total: %w", err)
			}
		}
		if gap > 0 && len(t.Attachments) == 0 {
			s.Gaps = append(s.Gaps, Gap{TxnID: t.ID, Label: txnLabel(t), Amount: gap, Date: t.Date, Reason: "no receipt attached"})
		}
	}
	for id, amount := range totals {
		s.Deductible.Rows = append(s.Deductible.Rows, reports.DeductibleRow{CategoryID: id, Amount: amount})
	}
	sort.Slice(s.Deductible.Rows, func(i, j int) bool {
		if s.Deductible.Rows[i].Amount != s.Deductible.Rows[j].Amount {
			return s.Deductible.Rows[i].Amount > s.Deductible.Rows[j].Amount
		}
		return s.Deductible.Rows[i].CategoryID < s.Deductible.Rows[j].CategoryID
	})
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
			b.WriteString(csvField(csvsafe.Guard(g.Label+" ("+g.Reason+")")) + "," + g.Date.Format("2006-01-02") + "," + amount(g.Amount) + "\n")
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
