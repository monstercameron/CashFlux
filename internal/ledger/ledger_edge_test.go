// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// badOpening has an opening-balance currency that disagrees with its account
// currency, so openingBalance (and everything built on it) errors.
func badOpening() domain.Account {
	return domain.Account{ID: "a1", Currency: "USD", OpeningBalance: money.New(10000, "EUR")}
}

// foreignAcct holds a balance in a currency the base-only rate table can't
// convert, so the Convert step errors.
func foreignAcct() domain.Account {
	return domain.Account{ID: "a2", Currency: "EUR", OpeningBalance: money.New(10000, "EUR")}
}

func TestClearedBalanceErrors(t *testing.T) {
	if _, err := ClearedBalance(badOpening(), nil); err == nil {
		t.Error("ClearedBalance should error on opening-currency mismatch")
	}
	// A cleared transaction in a different currency than the account fails the Add.
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}
	mixed := []domain.Transaction{{ID: "t1", AccountID: "a1", Amount: money.New(500, "EUR"), Cleared: true}}
	if _, err := ClearedBalance(acc, mixed); err == nil {
		t.Error("ClearedBalance should error when a cleared txn currency differs from the account")
	}
}

func TestRunningBalancesErrors(t *testing.T) {
	if _, err := RunningBalances(badOpening(), nil); err == nil {
		t.Error("RunningBalances should error on opening-currency mismatch")
	}
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}
	mixed := []domain.Transaction{{ID: "t1", AccountID: "a1", Amount: money.New(500, "EUR")}}
	if _, err := RunningBalances(acc, mixed); err == nil {
		t.Error("RunningBalances should error when a txn currency differs from the account")
	}
}

func TestPeriodTotalsConvertError(t *testing.T) {
	start, end := dateutil.MonthRange(mustDate("2026-06-15"))
	rates := currency.Rates{Base: "USD"} // no JPY rate
	// A non-transfer, in-range transaction in an unconvertible currency.
	all := []domain.Transaction{{ID: "t1", AccountID: "a1", Amount: money.New(-1000, "JPY"), Date: mustDate("2026-06-10")}}
	if _, _, err := PeriodTotals(all, start, end, rates); err == nil {
		t.Error("PeriodTotals should error when a transaction can't be converted")
	}
}

func TestNetWorthErrors(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	if _, _, _, err := NetWorth([]domain.Account{badOpening()}, nil, rates); err == nil {
		t.Error("NetWorth should error when an account balance can't be computed")
	}
	if _, _, _, err := NetWorth([]domain.Account{foreignAcct()}, nil, rates); err == nil {
		t.Error("NetWorth should error when an account balance can't be converted")
	}
}

func TestNetByOwnerErrors(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	if _, err := NetByOwner([]domain.Account{badOpening()}, nil, rates); err == nil {
		t.Error("NetByOwner should error when an account balance can't be computed")
	}
	if _, err := NetByOwner([]domain.Account{foreignAcct()}, nil, rates); err == nil {
		t.Error("NetByOwner should error when an account balance can't be converted")
	}
}

// TestNetByOwnerAggregatesAndSkipsArchived covers the same-owner accumulation
// branch and the archived-account skip.
func TestNetByOwnerAggregatesAndSkipsArchived(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{
		{ID: "a1", Currency: "USD", OwnerID: "m1", OpeningBalance: usd(10000)},
		{ID: "a2", Currency: "USD", OwnerID: "m1", OpeningBalance: usd(5000)},
		{ID: "a3", Currency: "USD", OwnerID: "m2", OpeningBalance: usd(7000), Archived: true},
	}
	got, err := NetByOwner(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetByOwner error: %v", err)
	}
	if !got["m1"].Equal(usd(15000)) {
		t.Errorf("m1 net = %v, want 15000 USD (two accounts summed)", got["m1"])
	}
	if _, ok := got["m2"]; ok {
		t.Error("archived account's owner should not appear")
	}
}
