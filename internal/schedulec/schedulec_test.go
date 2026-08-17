// SPDX-License-Identifier: MIT

package schedulec

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var rates = currency.Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "EUR": 1.1}}

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func year2026() (time.Time, time.Time) {
	return d(2026, time.January, 1), d(2027, time.January, 1)
}

func cat(id string, deductible bool, line string) domain.Category {
	return domain.Category{ID: id, Name: id, Deductible: deductible, TaxLine: domain.TaxLine(line)}
}

func spend(catID string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{CategoryID: catID, Date: when, Amount: money.New(-minor, "USD")}
}

func TestGroupTotalsByTaxLine(t *testing.T) {
	cats := []domain.Category{cat("office", true, "18"), cat("supplies", true, "22")}
	txns := []domain.Transaction{
		spend("office", d(2026, time.March, 1), 10_000),
		spend("office", d(2026, time.April, 1), 5_000),
		spend("supplies", d(2026, time.May, 1), 2_500),
	}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if len(s.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(s.Rows))
	}
	if s.Total != 17_500 {
		t.Errorf("total = %d, want 17500", s.Total)
	}
	if s.Rows[0].Line.Code != "18" || s.Rows[0].AmountMinor != 15_000 {
		t.Errorf("first row = %+v, want line 18 at 15000", s.Rows[0])
	}
}

func TestRowsComeOutInFormOrderNotBySize(t *testing.T) {
	// The report is transcribed onto a document that runs in form order; sorting
	// by amount would make the reader hunt for each line.
	cats := []domain.Category{cat("meals", true, "24b"), cat("ads", true, "8")}
	txns := []domain.Transaction{
		spend("meals", d(2026, time.March, 1), 90_000),
		spend("ads", d(2026, time.March, 1), 1_000),
	}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if s.Rows[0].Line.Code != "8" {
		t.Errorf("first row = %s, want line 8 despite being the smaller amount", s.Rows[0].Line.Code)
	}
}

func TestUnassignedIsSurfacedNotSweptIntoOtherExpenses(t *testing.T) {
	// Line 27a is a real line with real rules. Treating "we do not know where
	// this goes" as "put it there" turns an open question into a filed position.
	cats := []domain.Category{cat("mystery", true, "")}
	txns := []domain.Transaction{spend("mystery", d(2026, time.March, 1), 4_200)}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if len(s.Rows) != 0 {
		t.Errorf("rows = %d, want none — nothing was classified", len(s.Rows))
	}
	if s.UnassignedMinor != 4_200 || len(s.UnassignedIDs) != 1 {
		t.Errorf("unassigned = %d over %v, want 4200 for one category", s.UnassignedMinor, s.UnassignedIDs)
	}
	if s.Total != 0 {
		t.Errorf("total = %d — unclassified spending must not be counted as classified", s.Total)
	}
}

func TestANonDeductibleCategoryIsExcludedEvenWithATaxLine(t *testing.T) {
	// The two flags answer different questions, and not-deductible is a decision
	// the household already made on purpose.
	cats := []domain.Category{cat("personal", false, "18")}
	txns := []domain.Transaction{spend("personal", d(2026, time.March, 1), 9_000)}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if s.Total != 0 || s.UnassignedMinor != 0 {
		t.Errorf("summary = %+v, want everything excluded", s)
	}
}

func TestIncomeAndTransfersAreNotExpenses(t *testing.T) {
	cats := []domain.Category{cat("office", true, "18")}
	income := domain.Transaction{CategoryID: "office", Date: d(2026, time.March, 1), Amount: money.New(5_000, "USD")}
	transfer := spend("office", d(2026, time.March, 2), 5_000)
	transfer.TransferAccountID = "savings"
	from, to := year2026()
	s := Group([]domain.Transaction{income, transfer}, cats, from, to, rates)
	if s.Total != 0 {
		t.Errorf("total = %d, want 0", s.Total)
	}
}

func TestExcludedFromReportsIsExcludedHere(t *testing.T) {
	cats := []domain.Category{cat("office", true, "18")}
	tx := spend("office", d(2026, time.March, 1), 5_000)
	tx.ExcludeFromReports = true
	from, to := year2026()
	if s := Group([]domain.Transaction{tx}, cats, from, to, rates); s.Total != 0 {
		t.Errorf("total = %d, want 0 for an excluded transaction", s.Total)
	}
}

func TestOutOfWindowSpendingIsIgnored(t *testing.T) {
	cats := []domain.Category{cat("office", true, "18")}
	txns := []domain.Transaction{
		spend("office", d(2025, time.December, 31), 1_000),
		spend("office", d(2026, time.January, 1), 2_000),
		spend("office", d(2027, time.January, 1), 4_000),
	}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if s.Total != 2_000 {
		t.Errorf("total = %d, want only the 2000 inside the year", s.Total)
	}
}

func TestForeignSpendConvertsToBase(t *testing.T) {
	cats := []domain.Category{cat("office", true, "18")}
	tx := domain.Transaction{CategoryID: "office", Date: d(2026, time.March, 1), Amount: money.New(-1_000, "EUR")}
	from, to := year2026()
	if s := Group([]domain.Transaction{tx}, cats, from, to, rates); s.Total != 1_100 {
		t.Errorf("total = %d, want 1100", s.Total)
	}
}

func TestRowsNameTheCategoriesThatFedThem(t *testing.T) {
	// A total nobody can decompose is a total nobody can check.
	cats := []domain.Category{cat("phone", true, "25"), cat("power", true, "25")}
	txns := []domain.Transaction{
		spend("phone", d(2026, time.March, 1), 1_000),
		spend("power", d(2026, time.March, 2), 2_000),
	}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if len(s.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 (both are line 25)", len(s.Rows))
	}
	got := strings.Join(s.Rows[0].CategoryIDs, ",")
	if got != "phone,power" {
		t.Errorf("categories = %q, want a stable \"phone,power\"", got)
	}
}

func TestAnUnknownLineCodeIsTreatedAsUnassigned(t *testing.T) {
	// A category carrying a code the taxonomy does not know must not invent a row
	// for it — the form has no such line to transcribe onto.
	cats := []domain.Category{cat("weird", true, "99z")}
	txns := []domain.Transaction{spend("weird", d(2026, time.March, 1), 3_000)}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	if len(s.Rows) != 0 || s.UnassignedMinor != 3_000 {
		t.Errorf("summary = %+v, want it reported as unassigned", s)
	}
}

func TestCSVKeepsTheUnclassifiedTotalVisible(t *testing.T) {
	// An export that silently drops what the app could not classify reads as
	// complete to whoever opens it next, who is usually not the person who knew.
	cats := []domain.Category{cat("office", true, "18"), cat("mystery", true, "")}
	txns := []domain.Transaction{
		spend("office", d(2026, time.March, 1), 1_000),
		spend("mystery", d(2026, time.March, 2), 500),
	}
	from, to := year2026()
	s := Group(txns, cats, from, to, rates)
	out := string(CSV(s, func(id string) string { return id }, func(v int64) string {
		return money.New(v, "USD").String()
	}))
	if !strings.Contains(out, "Not assigned to a line") {
		t.Errorf("CSV omitted the unclassified row:\n%s", out)
	}
	if !strings.Contains(out, "Office expense") {
		t.Errorf("CSV omitted the classified row:\n%s", out)
	}
}

func TestCSVQuotesFieldsContainingCommas(t *testing.T) {
	s := Summary{Rows: []Row{{
		Line:        Line{Code: "20a", Label: "Rent or lease — vehicles, machinery, equipment"},
		AmountMinor: 1_000, CategoryIDs: []string{"a"},
	}}}
	out := string(CSV(s, func(id string) string { return id }, func(v int64) string { return "1.00" }))
	if !strings.Contains(out, `"Rent or lease — vehicles, machinery, equipment"`) {
		t.Errorf("a label with a comma must be quoted:\n%s", out)
	}
}

func TestTheTaxonomyHasNoDuplicateCodes(t *testing.T) {
	seen := map[string]bool{}
	for _, l := range Lines {
		if seen[l.Code] {
			t.Errorf("line %q appears twice — one of them would silently absorb the other", l.Code)
		}
		seen[l.Code] = true
		if strings.TrimSpace(l.Label) == "" {
			t.Errorf("line %q has no label", l.Code)
		}
	}
}
