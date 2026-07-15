// SPDX-License-Identifier: MIT

package ledger

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestRegisterBalances(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(10000)}
	all := []domain.Transaction{
		{ID: "t2", AccountID: "a1", Amount: usd(-3000), Date: mustDate("2026-01-10")},
		{ID: "t1", AccountID: "a1", Amount: usd(5000), Date: mustDate("2026-01-05")},
		{ID: "t3", AccountID: "other", Amount: usd(9999), Date: mustDate("2026-01-06")}, // ignored
		{ID: "t4", AccountID: "a1", Amount: usd(2000), Date: mustDate("2026-01-20")},
	}
	got, err := RegisterBalances(acc, all)
	if err != nil {
		t.Fatalf("RegisterBalances error: %v", err)
	}
	// Chronological fold from opening 10000: t1(+5000)=15000, t2(-3000)=12000, t4(+2000)=14000.
	want := map[string]money.Money{
		"t1": usd(15000),
		"t2": usd(12000),
		"t4": usd(14000),
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for id, w := range want {
		if !got[id].Equal(w) {
			t.Errorf("running balance[%s] = %v, want %v", id, got[id], w)
		}
	}
	if _, ok := got["t3"]; ok {
		t.Error("other-account txn t3 must not appear")
	}
}

// TestRegisterBalancesSliceTruth: a filtered/paginated subset still reports the
// TRUE running balance because the fold covers the account's full history.
func TestRegisterBalancesSliceTruth(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(0)}
	all := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: usd(1000), Date: mustDate("2026-01-01")},
		{ID: "t2", AccountID: "a1", Amount: usd(1000), Date: mustDate("2026-01-02")},
		{ID: "t3", AccountID: "a1", Amount: usd(1000), Date: mustDate("2026-01-03")},
	}
	got, err := RegisterBalances(acc, all)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Even if the view only shows t3, its running balance reflects t1 and t2 before it.
	if !got["t3"].Equal(usd(3000)) {
		t.Errorf("running balance[t3] = %v, want 3000", got["t3"])
	}
}

// TestRegisterBalancesTieBreak: same-date txns fold in ID order deterministically.
func TestRegisterBalancesTieBreak(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(0)}
	all := []domain.Transaction{
		{ID: "b", AccountID: "a1", Amount: usd(200), Date: mustDate("2026-01-01")},
		{ID: "a", AccountID: "a1", Amount: usd(100), Date: mustDate("2026-01-01")},
	}
	got, err := RegisterBalances(acc, all)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// a folds first (ID asc): a=100, b=300.
	if !got["a"].Equal(usd(100)) || !got["b"].Equal(usd(300)) {
		t.Errorf("tie-break wrong: a=%v b=%v, want 100/300", got["a"], got["b"])
	}
}

// TestRegisterBalancesCurrencyMismatch: a foreign-currency txn on the account errors
// so the caller hides the column rather than showing a wrong figure.
func TestRegisterBalancesCurrencyMismatch(t *testing.T) {
	acc := domain.Account{ID: "a1", Currency: "USD", OpeningBalance: usd(1000)}
	all := []domain.Transaction{
		{ID: "t1", AccountID: "a1", Amount: money.New(500, "EUR"), Date: mustDate("2026-01-01")},
	}
	if _, err := RegisterBalances(acc, all); err == nil {
		t.Error("expected an error for a mismatched-currency transaction")
	}
}
