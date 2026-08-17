// SPDX-License-Identifier: MIT

package roundups

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func expense(id, acct string, cents int64, day int) domain.Transaction {
	return domain.Transaction{
		ID: id, AccountID: acct, Payee: id,
		Amount: money.New(-cents, "USD"),
		Date:   time.Date(2026, time.July, day, 12, 0, 0, 0, time.UTC),
	}
}

func TestAccrue(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)

	txns := []domain.Transaction{
		expense("a", "chk", 347, 2),  // spare 53
		expense("b", "chk", 1299, 3), // spare 1
		expense("c", "chk", 500, 4),  // exact dollar → 0, skipped
		{ID: "inc", AccountID: "chk", Amount: money.New(20000, "USD"), Date: time.Date(2026, time.July, 5, 0, 0, 0, 0, time.UTC)},
		{ID: "xfer", AccountID: "chk", TransferAccountID: "sav", Amount: money.New(-10000, "USD"), Date: time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC)},
	}

	got := Accrue(txns, nil, nil, since, now)
	if got.TotalCents != 54 {
		t.Fatalf("total = %d, want 54", got.TotalCents)
	}
	if len(got.Contributions) != 2 {
		t.Fatalf("contributions = %d, want 2", len(got.Contributions))
	}
	if got.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", got.Currency)
	}
}

func TestAccrueParticipatingFilter(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		expense("a", "chk", 347, 2),   // 53
		expense("b", "other", 111, 3), // 89, excluded
	}
	got := Accrue(txns, map[string]bool{"chk": true}, nil, since, now)
	if got.TotalCents != 53 {
		t.Fatalf("total = %d, want 53 (only participating account)", got.TotalCents)
	}
}

func TestAccrueSkipsRefundPaired(t *testing.T) {
	since := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 31, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{expense("orig", "chk", 347, 2)} // would be 53
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"orig", "refund"}}}
	got := Accrue(txns, nil, links, since, now)
	if got.TotalCents != 0 {
		t.Fatalf("total = %d, want 0 (refund-paired skipped)", got.TotalCents)
	}
}

func TestAccrueWindowBounds(t *testing.T) {
	since := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		expense("before", "chk", 347, 5), // before since → excluded
		expense("in", "chk", 347, 15),    // in window → 53
		expense("after", "chk", 347, 25), // after now → excluded
	}
	got := Accrue(txns, nil, nil, since, now)
	if got.TotalCents != 53 {
		t.Fatalf("total = %d, want 53 (window bounds)", got.TotalCents)
	}
}

// EC-13: "this goal finishes seven weeks sooner" is a promise, and a promise
// extrapolated from a fortnight of spare change is a fantasy.
func TestMonthlyRateNeedsASustainedWindow(t *testing.T) {
	a := Accrual{TotalCents: 4_000, Currency: "USD"}
	for i := range 12 {
		a.Contributions = append(a.Contributions, Contribution{TxnID: string(rune('a' + i))})
	}
	if _, ok := a.MonthlyRateCents(14); ok {
		t.Error("a fortnight produced a projectable rate")
	}
	rate, ok := a.MonthlyRateCents(90)
	if !ok {
		t.Fatal("90 days of round-ups produced no rate")
	}
	// $40 over 90 days is about $13.33 a month.
	if rate < 1_300 || rate > 1_360 {
		t.Errorf("rate = %d, want about 1333", rate)
	}
}

func TestAnEmptyJarHasNoRate(t *testing.T) {
	if _, ok := (Accrual{}).MonthlyRateCents(120); ok {
		t.Error("an empty jar produced a rate")
	}
	// Plenty of days but barely any activity is not a habit either.
	thin := Accrual{TotalCents: 300, Contributions: []Contribution{{TxnID: "a"}, {TxnID: "b"}}}
	if _, ok := thin.MonthlyRateCents(120); ok {
		t.Error("two contributions in four months produced a rate")
	}
}

func TestMonthsSoonerIsTheDifferenceInWholeMonths(t *testing.T) {
	// $1,200 to go at $100/mo is 12 months; at $150/mo it is 8.
	got, ok := MonthsSooner(120_000, 10_000, 5_000)
	if !ok {
		t.Fatal("no acceleration reported")
	}
	if got != 4 {
		t.Errorf("months sooner = %d, want 4", got)
	}
}

// "0 months sooner" dressed up as an accelerator is worse than saying nothing.
func TestNoRealAccelerationSaysNothing(t *testing.T) {
	if _, ok := MonthsSooner(120_000, 10_000, 10); ok {
		t.Error("a rate that changes nothing was reported as acceleration")
	}
	// Nothing being contributed means there is no date to bring forward.
	if _, ok := MonthsSooner(120_000, 0, 5_000); ok {
		t.Error("a goal with no contribution reported a date brought forward")
	}
	if _, ok := MonthsSooner(0, 10_000, 5_000); ok {
		t.Error("a finished goal reported acceleration")
	}
}
