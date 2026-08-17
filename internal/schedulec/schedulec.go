// SPDX-License-Identifier: MIT

// Package schedulec groups business spending by the tax line it belongs on
// (FP-T1e, part 1).
//
// A category carried one `Deductible` bool, which answers "does this reduce my
// tax" and nothing else. At tax time the question is narrower and harder: WHICH
// LINE does it go on. "Deductible: $18,400" is a number nobody can transcribe
// onto a form, and the work of splitting it back out by hand is exactly the work
// the app should have been doing all year.
//
// The taxonomy is deliberately the real form's, not a tidier invention of our
// own. A neat internal scheme would have to be mapped to the form eventually,
// and that mapping is where the categories nobody thought about go missing.
package schedulec

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// Line describes one line of Schedule C.
type Line struct {
	// Code is the form's own line number ("8", "16b", "24b"). Kept as a string
	// because the form's numbering is not numeric — 16a and 16b are different
	// lines, and normalizing them to 16 would merge two deductions the form keeps
	// apart on purpose.
	Code string
	// Label is the form's wording, near enough to be recognizable beside it.
	Label string
	// Note is a plain-English gloss where the form's own wording is the part
	// people get wrong.
	Note string
}

// Lines is the taxonomy, in form order.
//
// Not exhaustive: depletion, depreciation and the cost-of-goods worksheet are
// omitted because they are computed on their own schedules from facts this app
// does not hold, and offering a bucket the household could drop receipts into
// would produce a number that looks filed-ready and is not.
var Lines = []Line{
	{Code: "8", Label: "Advertising"},
	{Code: "9", Label: "Car and truck expenses",
		Note: "Only if you deduct actual costs. Mileage is claimed separately and is not tracked here."},
	{Code: "10", Label: "Commissions and fees"},
	{Code: "11", Label: "Contract labor",
		Note: "People you paid who are not employees. Payments over the annual threshold may need a 1099."},
	{Code: "14", Label: "Employee benefit programs"},
	{Code: "15", Label: "Insurance (other than health)"},
	{Code: "16a", Label: "Interest — mortgage"},
	{Code: "16b", Label: "Interest — other"},
	{Code: "17", Label: "Legal and professional services"},
	{Code: "18", Label: "Office expense"},
	{Code: "19", Label: "Pension and profit-sharing plans"},
	{Code: "20a", Label: "Rent or lease — vehicles, machinery, equipment"},
	{Code: "20b", Label: "Rent or lease — other business property"},
	{Code: "21", Label: "Repairs and maintenance"},
	{Code: "22", Label: "Supplies"},
	{Code: "23", Label: "Taxes and licenses"},
	{Code: "24a", Label: "Travel"},
	{Code: "24b", Label: "Deductible meals",
		Note: "Meals are usually deductible at 50%. This reports what you spent, not the deductible share."},
	{Code: "25", Label: "Utilities"},
	{Code: "26", Label: "Wages"},
	{Code: "27a", Label: "Other expenses"},
	{Code: "30", Label: "Business use of your home",
		Note: "Computed on its own form from square footage; what is reported here is the spending you tagged."},
}

// Valid reports whether a code is one of the taxonomy's lines.
func Valid(code string) bool {
	_, ok := ByCode(code)
	return ok
}

// ByCode returns the line for a code.
func ByCode(code string) (Line, bool) {
	for _, l := range Lines {
		if l.Code == code {
			return l, true
		}
	}
	return Line{}, false
}

// Row is one tax line's total for the period.
type Row struct {
	Line Line
	// AmountMinor is absolute expense in base-currency minor units.
	AmountMinor int64
	// CategoryIDs are the categories that fed it, so a reader who disputes a
	// figure can find out what is in it. A total nobody can decompose is a total
	// nobody can check.
	CategoryIDs []string
}

// Summary is the grouped report.
type Summary struct {
	Rows  []Row
	Total int64
	// UnassignedMinor is deductible spending in categories with no tax line, and
	// UnassignedIDs names those categories.
	//
	// Surfaced, never folded into "Other expenses". Line 27a is a real line with
	// real rules, and quietly sweeping every unclassified category into it would
	// turn "we do not know where this goes" into a filed position.
	UnassignedMinor int64
	UnassignedIDs   []string
}

// Group totals deductible expense by tax line over [start, end).
//
// Only DEDUCTIBLE categories count, even if a category carries a tax line: the
// two flags answer different questions, and a category marked not-deductible has
// already been excluded by the household on purpose.
func Group(txns []domain.Transaction, cats []domain.Category,
	start, end time.Time, rates currency.Rates) Summary {

	lineOf := make(map[string]string, len(cats))
	deductible := make(map[string]bool, len(cats))
	for _, c := range cats {
		deductible[c.ID] = c.Deductible
		if c.Deductible && Valid(string(c.TaxLine)) {
			lineOf[c.ID] = string(c.TaxLine)
		}
	}

	byLine := map[string]int64{}
	catsOf := map[string]map[string]bool{}
	var s Summary
	unassigned := map[string]bool{}

	for _, t := range txns {
		if !t.IsExpense() || !t.CountsInReports() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		if !deductible[t.CategoryID] {
			continue
		}
		amt, err := rates.ToBase(t.Amount)
		if err != nil {
			continue
		}
		v := amt.Amount
		if v < 0 {
			v = -v
		}
		code, ok := lineOf[t.CategoryID]
		if !ok {
			s.UnassignedMinor += v
			unassigned[t.CategoryID] = true
			continue
		}
		byLine[code] += v
		if catsOf[code] == nil {
			catsOf[code] = map[string]bool{}
		}
		catsOf[code][t.CategoryID] = true
	}

	// Rows come out in FORM ORDER, not by size. This report is transcribed onto a
	// document that runs in that order, and re-sorting it by amount would make the
	// reader hunt for each line.
	for _, l := range Lines {
		if byLine[l.Code] == 0 {
			continue
		}
		s.Rows = append(s.Rows, Row{Line: l, AmountMinor: byLine[l.Code], CategoryIDs: sortedKeys(catsOf[l.Code])})
		s.Total += byLine[l.Code]
	}
	s.UnassignedIDs = sortedKeys(unassigned)
	return s
}

// sortedKeys returns a set's keys in a stable order.
func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// CSV renders the grouped report for a spreadsheet or a preparer.
//
// The unassigned total is written as its own final row rather than omitted. A
// export that silently drops spending the app could not classify would be read
// as complete by whoever opens it next, which is usually not the person who knew.
func CSV(s Summary, name func(id string) string, amount func(int64) string) []byte {
	var b bytes.Buffer
	b.WriteString("Line,Description,Amount,Categories\n")
	for _, r := range s.Rows {
		names := make([]string, 0, len(r.CategoryIDs))
		for _, id := range r.CategoryIDs {
			names = append(names, name(id))
		}
		fmt.Fprintf(&b, "%s,%s,%s,%s\n",
			csvField(r.Line.Code), csvField(r.Line.Label), csvField(amount(r.AmountMinor)),
			csvField(strings.Join(names, "; ")))
	}
	fmt.Fprintf(&b, "%s,%s,%s,\n", csvField(""), csvField("Total"), csvField(amount(s.Total)))
	if s.UnassignedMinor > 0 {
		names := make([]string, 0, len(s.UnassignedIDs))
		for _, id := range s.UnassignedIDs {
			names = append(names, name(id))
		}
		fmt.Fprintf(&b, "%s,%s,%s,%s\n", csvField(""), csvField("Not assigned to a line"),
			csvField(amount(s.UnassignedMinor)), csvField(strings.Join(names, "; ")))
	}
	return b.Bytes()
}

// csvField quotes a field when it contains a comma, a quote or a newline.
func csvField(v string) string {
	if !strings.ContainsAny(v, ",\"\n") {
		return v
	}
	return `"` + strings.ReplaceAll(v, `"`, `""`) + `"`
}
