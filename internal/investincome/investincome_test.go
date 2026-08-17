// SPDX-License-Identifier: MIT

package investincome

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

var usd = currency.Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "EUR": 1.1}}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func txn(acct, holding string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{
		AccountID: acct, HoldingID: holding, Date: when,
		Amount: money.New(minor, "USD"),
	}
}

func invAccounts() map[string]bool { return map[string]bool{"brokerage": true} }

func TestGatherAttributesIncomeToItsHolding(t *testing.T) {
	txns := []domain.Transaction{
		txn("brokerage", "h1", day(2026, time.March, 1), 5_000),
		txn("brokerage", "h1", day(2026, time.June, 1), 5_500),
		txn("brokerage", "h2", day(2026, time.April, 1), 2_000),
	}
	s := Gather(txns, invAccounts(), day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.TotalMinor != 12_500 {
		t.Errorf("total = %d, want 12500", s.TotalMinor)
	}
	h1 := s.ByHolding["h1"]
	if h1.TotalMinor != 10_500 || h1.Count != 2 {
		t.Errorf("h1 = %+v, want 10500 over 2 payments", h1)
	}
	if !h1.FirstDate.Equal(day(2026, time.March, 1)) || !h1.LastDate.Equal(day(2026, time.June, 1)) {
		t.Errorf("h1 span = %v..%v, want Mar..Jun", h1.FirstDate, h1.LastDate)
	}
}

func TestExpensesAndTransfersAreNotIncome(t *testing.T) {
	// A fee posted against a holding is real, and it is not a payout. A transfer
	// in is money moved, not money earned.
	fee := txn("brokerage", "h1", day(2026, time.March, 1), -1_000)
	transfer := txn("brokerage", "h1", day(2026, time.March, 2), 50_000)
	transfer.TransferAccountID = "checking"
	s := Gather([]domain.Transaction{fee, transfer}, invAccounts(),
		day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.TotalMinor != 0 {
		t.Errorf("total = %d, want 0 — neither a fee nor a transfer is a payout", s.TotalMinor)
	}
}

func TestUntaggedIncomeIsReportedNotSpread(t *testing.T) {
	// The honesty property: a household whose dividends are mostly untagged must
	// be able to see that the attributed figure is incomplete.
	txns := []domain.Transaction{
		txn("brokerage", "h1", day(2026, time.March, 1), 5_000),
		txn("brokerage", "", day(2026, time.March, 2), 9_000),
	}
	s := Gather(txns, invAccounts(), day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.TotalMinor != 5_000 {
		t.Errorf("attributed total = %d, want only the tagged 5000", s.TotalMinor)
	}
	if s.UntaggedMinor != 9_000 || s.UntaggedCount != 1 {
		t.Errorf("untagged = %d over %d, want 9000 over 1", s.UntaggedMinor, s.UntaggedCount)
	}
	if got := s.ByHolding["h1"].TotalMinor; got != 5_000 {
		t.Errorf("h1 = %d — untagged income must not be apportioned onto a position", got)
	}
}

func TestSalaryIntoACheckingAccountIsNotPortfolioIncome(t *testing.T) {
	pay := txn("checking", "", day(2026, time.March, 1), 400_000)
	s := Gather([]domain.Transaction{pay}, invAccounts(),
		day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.UntaggedMinor != 0 {
		t.Errorf("untagged = %d, want 0 — a paycheck is not a portfolio payout", s.UntaggedMinor)
	}
}

func TestExcludedFromReportsIsExcludedHere(t *testing.T) {
	tx := txn("brokerage", "h1", day(2026, time.March, 1), 5_000)
	tx.ExcludeFromReports = true
	s := Gather([]domain.Transaction{tx}, invAccounts(),
		day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.TotalMinor != 0 {
		t.Errorf("total = %d, want 0 for an excluded transaction", s.TotalMinor)
	}
}

func TestWindowBoundsAreInclusiveStartExclusiveEnd(t *testing.T) {
	from, to := day(2026, time.January, 1), day(2027, time.January, 1)
	on := txn("brokerage", "h1", from, 1_000)
	off := txn("brokerage", "h1", to, 2_000)
	s := Gather([]domain.Transaction{on, off}, invAccounts(), from, to, usd)
	if s.TotalMinor != 1_000 {
		t.Errorf("total = %d, want only the payment on the start date", s.TotalMinor)
	}
}

func TestYieldIsOnCostNotOnCurrentValue(t *testing.T) {
	// $100 of income against $1,000 paid is 10%, whatever the position is worth
	// today. Yield on current value would reprice the past.
	got, ok := YieldOnCostPct(10_000, 100_000)
	if !ok || got != 10 {
		t.Errorf("YieldOnCostPct = %v (ok=%v), want 10", got, ok)
	}
}

func TestYieldRefusesAMissingCost(t *testing.T) {
	// An infinite yield is not a spectacular result, it is an absent basis.
	if _, ok := YieldOnCostPct(10_000, 0); ok {
		t.Error("a zero basis must refuse rather than divide")
	}
	if _, ok := YieldOnCostPct(10_000, -5); ok {
		t.Error("a negative basis must refuse")
	}
}

func TestAnnualizingRefusesAWindowTooShortToMeanAnything(t *testing.T) {
	// Most positions pay quarterly. A 40-day window either catches one payment
	// and multiplies it ninefold or catches none and reports zero.
	if _, ok := AnnualizedYieldPct(10_000, 100_000, 40); ok {
		t.Errorf("a %d-day span is shorter than the %d-day floor and must refuse", 40, MinIncomeDays)
	}
	got, ok := AnnualizedYieldPct(10_000, 100_000, 365)
	if !ok || got != 10 {
		t.Errorf("a full year = %v (ok=%v), want the unscaled 10", got, ok)
	}
	// Half a year of the same income annualizes to roughly double.
	half, ok := AnnualizedYieldPct(10_000, 100_000, 182)
	if !ok || half < 19 || half > 21 {
		t.Errorf("half a year = %v (ok=%v), want about 20", half, ok)
	}
}

func TestSpanClosesThePeriodTheLastPaymentStandsFor(t *testing.T) {
	// Four quarterly payments run Jan→Oct, a 273-day gap. The last one still
	// covers its own quarter, so the span is a year, not nine months.
	h := HoldingIncome{
		Count:     4,
		FirstDate: day(2026, time.January, 1),
		LastDate:  day(2026, time.October, 1),
	}
	got := Span(h)
	if got < 355 || got > 375 {
		t.Errorf("Span = %d, want roughly a year", got)
	}
}

func TestOnePaymentSaysNothingAboutCadence(t *testing.T) {
	h := HoldingIncome{Count: 1, FirstDate: day(2026, time.March, 1), LastDate: day(2026, time.March, 1)}
	if got := Span(h); got != 0 {
		t.Errorf("Span = %d, want 0 — a single payment implies no interval", got)
	}
	// And the refusal propagates: no span means no annualized yield.
	if _, ok := AnnualizedYieldPct(10_000, 100_000, Span(h)); ok {
		t.Error("a single payment must not produce an annualized yield")
	}
}

func TestForeignCurrencyIncomeConvertsToBase(t *testing.T) {
	tx := domain.Transaction{
		AccountID: "brokerage", HoldingID: "h1", Date: day(2026, time.March, 1),
		Amount: money.New(1_000, "EUR"),
	}
	s := Gather([]domain.Transaction{tx}, invAccounts(),
		day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.TotalMinor != 1_100 {
		t.Errorf("total = %d, want 1100 (1000 EUR at 1.1)", s.TotalMinor)
	}
}

func TestNoIncomeIsAnEmptySummaryNotANilMap(t *testing.T) {
	s := Gather(nil, invAccounts(), day(2026, time.January, 1), day(2027, time.January, 1), usd)
	if s.ByHolding == nil {
		t.Error("ByHolding must be usable without a nil check by every caller")
	}
	if s.TotalMinor != 0 || s.UntaggedMinor != 0 {
		t.Errorf("empty summary = %+v, want zeroes", s)
	}
}
