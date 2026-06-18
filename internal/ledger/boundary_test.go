package ledger

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// inc is an income transaction (positive amount) dated day "YYYY-MM-DD".
func inc(amount int64, day string) domain.Transaction {
	return domain.Transaction{ID: "t" + day, AccountID: "a1", Amount: usd(amount), Date: mustDate(day)}
}

// incomeIn returns the income total for the half-open window [start, end).
func incomeIn(t *testing.T, txns []domain.Transaction, rates currency.Rates, start, end time.Time) int64 {
	t.Helper()
	income, _, err := PeriodTotals(txns, start, end, rates)
	if err != nil {
		t.Fatalf("PeriodTotals: %v", err)
	}
	return income.Amount
}

// TestPeriodTotalsMonthBoundary pins that a transaction on the first or last day
// of a month lands in exactly one month — no drop, no double-count — exercising
// the half-open [start, end) window. This is the regression home for C1 (day-1
// transactions were dropped) at the totals level.
func TestPeriodTotalsMonthBoundary(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		inc(100, "2026-05-31"), // last day of May
		inc(200, "2026-06-01"), // first day of June
		inc(400, "2026-06-30"), // last day of June
		inc(800, "2026-07-01"), // first day of July
	}
	month := func(d string) (time.Time, time.Time) { return dateutil.MonthRange(mustDate(d)) }

	ms, me := month("2026-05-15")
	js, je := month("2026-06-15")
	ls, le := month("2026-07-15")

	if got := incomeIn(t, txns, rates, ms, me); got != 100 {
		t.Errorf("May income = %d, want 100 (only May 31)", got)
	}
	if got := incomeIn(t, txns, rates, js, je); got != 600 {
		t.Errorf("June income = %d, want 600 (Jun 1 + Jun 30)", got)
	}
	if got := incomeIn(t, txns, rates, ls, le); got != 800 {
		t.Errorf("July income = %d, want 800 (only Jul 1)", got)
	}
	// No drop, no double-count: each txn counted in exactly one of the three
	// consecutive windows → their sum equals every amount once.
	total := incomeIn(t, txns, rates, ms, me) + incomeIn(t, txns, rates, js, je) + incomeIn(t, txns, rates, ls, le)
	if total != 1500 {
		t.Errorf("sum across May+June+July = %d, want 1500 (100+200+400+800)", total)
	}
}

// TestPeriodTotalsWeekBoundary checks the half-open week window honors the
// week-start: the first day of the week is in, the day before the next week-start
// is in, and the next week-start rolls to the following week.
func TestPeriodTotalsWeekBoundary(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	// 2026-06-14 is a Sunday. Sunday-start week: [Jun 14, Jun 21).
	txns := []domain.Transaction{
		inc(10, "2026-06-13"), // Sat — previous week
		inc(20, "2026-06-14"), // Sun — week start, in
		inc(40, "2026-06-20"), // Sat — last day, in
		inc(80, "2026-06-21"), // Sun — next week start, out
	}
	weekSun := func(d string) (time.Time, time.Time) {
		s := dateutil.WeekStart(mustDate(d), time.Sunday)
		return s, s.AddDate(0, 0, 7)
	}
	s, e := weekSun("2026-06-16") // a date inside the Jun 14 week
	if got := incomeIn(t, txns, rates, s, e); got != 60 {
		t.Errorf("week income = %d, want 60 (Jun 14 + Jun 20)", got)
	}
}

// TestPeriodTotalsQuarterBoundary checks the half-open quarter window: Q2 2026 is
// [Apr 1, Jul 1), so Apr 1 and Jun 30 are in but Jul 1 rolls to Q3.
func TestPeriodTotalsQuarterBoundary(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	txns := []domain.Transaction{
		inc(10, "2026-03-31"), // Q1
		inc(20, "2026-04-01"), // Q2 start, in
		inc(40, "2026-06-30"), // Q2 last day, in
		inc(80, "2026-07-01"), // Q3 start, out
	}
	q2s, q2e := mustDate("2026-04-01"), mustDate("2026-07-01")
	if got := incomeIn(t, txns, rates, q2s, q2e); got != 60 {
		t.Errorf("Q2 income = %d, want 60 (Apr 1 + Jun 30)", got)
	}
}
