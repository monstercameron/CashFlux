// SPDX-License-Identifier: MIT

package nwchange

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func usd(minor int64) money.Money { return money.New(minor, "USD") }

func rates() currency.Rates { return currency.Rates{Base: "USD"} }

// day builds a UTC calendar date the way the app stores transaction dates.
func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func checking(id string, opening int64) domain.Account {
	return domain.Account{
		ID: id, Name: id, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: usd(opening),
	}
}

func txn(acct string, d time.Time, minor int64) domain.Transaction {
	return domain.Transaction{ID: acct + d.String(), AccountID: acct, Date: d, Amount: usd(minor)}
}

func TestMonthToDateBounds(t *testing.T) {
	now := time.Date(2026, time.July, 3, 14, 30, 0, 0, time.UTC)
	w := MonthToDate(now)
	if want := day(2026, time.July, 1); !w.Since.Equal(want) {
		t.Errorf("Since = %s, want %s", w.Since, want)
	}
	// Until is tomorrow so today's postings count and tomorrow's do not.
	if want := day(2026, time.July, 4); !w.Until.Equal(want) {
		t.Errorf("Until = %s, want %s", w.Until, want)
	}
}

func TestPriorMonthBounds(t *testing.T) {
	now := time.Date(2026, time.July, 3, 14, 30, 0, 0, time.UTC)
	w := PriorMonth(now)
	if want := day(2026, time.June, 1); !w.Since.Equal(want) {
		t.Errorf("Since = %s, want %s", w.Since, want)
	}
	if want := day(2026, time.July, 1); !w.Until.Equal(want) {
		t.Errorf("Until = %s, want %s", w.Until, want)
	}
}

func TestMonthsWindowSpansWholeMonths(t *testing.T) {
	now := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		n     int
		since time.Time
	}{
		{1, day(2026, time.July, 1)},
		{6, day(2026, time.February, 1)},
		{12, day(2025, time.August, 1)},
		{0, day(2026, time.July, 1)}, // clamped to 1
	} {
		if got := Months(now, tc.n).Since; !got.Equal(tc.since) {
			t.Errorf("Months(%d).Since = %s, want %s", tc.n, got, tc.since)
		}
	}
}

func TestComputeExcludesFutureDatedRows(t *testing.T) {
	now := time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC)
	accts := []domain.Account{checking("a", 100_000)}
	txns := []domain.Transaction{
		txn("a", day(2026, time.June, 20), -5_000),  // before the window
		txn("a", day(2026, time.July, 1), 470_000),  // the 1st counts
		txn("a", day(2026, time.July, 3), -12_000),  // today counts
		txn("a", day(2026, time.July, 20), -32_000), // scheduled — must not count
	}

	c, err := MonthToDateChange(accts, txns, rates(), now)
	if err != nil {
		t.Fatalf("MonthToDateChange: %v", err)
	}
	if !c.Known {
		t.Fatal("Known = false, want true")
	}
	if got, want := c.StartMinor, int64(95_000); got != want {
		t.Errorf("StartMinor = %d, want %d", got, want)
	}
	if got, want := c.EndMinor, int64(553_000); got != want {
		t.Errorf("EndMinor = %d, want %d", got, want)
	}
	if got, want := c.DeltaMinor(), int64(458_000); got != want {
		t.Errorf("DeltaMinor = %d, want %d (a future-dated row leaked in)", got, want)
	}
}

// The reason this package exists: the dashboard, /accounts, /networth and
// /reports must not be able to answer "how much did net worth move this month?"
// differently. Each surface passes its own account scope; the arithmetic is
// identical, so an unscoped read from any of them matches to the cent.
func TestEverySurfaceReadsTheSameMonthToDateDelta(t *testing.T) {
	now := time.Date(2026, time.July, 3, 9, 0, 0, 0, time.UTC)
	accts := []domain.Account{
		checking("chk", 340_000),
		{ID: "card", Name: "card", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			Currency: "USD", OpeningBalance: usd(-80_000)},
	}
	txns := []domain.Transaction{
		txn("chk", day(2026, time.June, 28), 20_000),
		txn("chk", day(2026, time.July, 2), 284_000),
		txn("card", day(2026, time.July, 2), -6_000),
	}

	want, err := MonthToDateChange(accts, txns, rates(), now)
	if err != nil {
		t.Fatalf("MonthToDateChange: %v", err)
	}
	// Reading the same window through the general entry point must agree.
	got, err := Compute(accts, txns, rates(), MonthToDate(now))
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if got.DeltaMinor() != want.DeltaMinor() {
		t.Fatalf("Compute delta = %d, MonthToDateChange delta = %d", got.DeltaMinor(), want.DeltaMinor())
	}
	if got, want := want.DeltaMinor(), int64(278_000); got != want {
		t.Errorf("delta = %d, want %d", got, want)
	}
}

func TestPriorMonthIsNotMonthToDate(t *testing.T) {
	now := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	accts := []domain.Account{checking("a", 0)}
	txns := []domain.Transaction{
		txn("a", day(2026, time.June, 10), 100_000),
		txn("a", day(2026, time.July, 2), 5_000),
	}
	mtd, err := Compute(accts, txns, rates(), MonthToDate(now))
	if err != nil {
		t.Fatalf("mtd: %v", err)
	}
	prior, err := Compute(accts, txns, rates(), PriorMonth(now))
	if err != nil {
		t.Fatalf("prior: %v", err)
	}
	if mtd.DeltaMinor() != 5_000 {
		t.Errorf("month-to-date delta = %d, want 5000", mtd.DeltaMinor())
	}
	if prior.DeltaMinor() != 100_000 {
		t.Errorf("prior-month delta = %d, want 100000", prior.DeltaMinor())
	}
}

func TestPercentChangeUnknownWhenWindowOpensAtZero(t *testing.T) {
	now := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	accts := []domain.Account{checking("a", 0)}
	txns := []domain.Transaction{txn("a", day(2026, time.July, 2), 5_000)}
	c, err := Compute(accts, txns, rates(), MonthToDate(now))
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if _, ok := c.PercentChange(); ok {
		t.Error("PercentChange ok = true for a window opening at zero, want false")
	}
	if c.Flat() {
		t.Error("Flat = true, want false (money moved)")
	}
}

func TestUnknownChangeReportsNoPercent(t *testing.T) {
	var c Change
	if _, ok := c.PercentChange(); ok {
		t.Error("PercentChange ok = true on an unknown change")
	}
	if c.Flat() {
		t.Error("Flat = true on an unknown change — an unread window is not a flat one")
	}
}

func TestComputeSurfacesLedgerErrors(t *testing.T) {
	now := time.Date(2026, time.July, 3, 0, 0, 0, 0, time.UTC)
	accts := []domain.Account{{
		ID: "eu", Name: "eu", Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "EUR", OpeningBalance: money.New(10_000, "EUR"),
	}}
	if _, err := Compute(accts, nil, rates(), MonthToDate(now)); err == nil {
		t.Fatal("Compute error = nil for a missing EUR rate, want an error")
	}
	c, _ := Compute(accts, nil, rates(), MonthToDate(now))
	if c.Known {
		t.Error("Known = true after a failed read")
	}
}
