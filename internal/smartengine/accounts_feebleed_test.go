// SPDX-License-Identifier: MIT

package smartengine

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

func TestA9FeeBleed(t *testing.T) {
	in := baseInput()
	old := ref.AddDate(0, -10, 0) // last real activity 10 months ago
	acc := acct("a1", "Old Brokerage Cash", domain.TypeSavings, 100000, old)
	// Monthly $5 fee for the last 4 months → otherwise-dormant fee bleed.
	var txns []domain.Transaction
	for i := 1; i <= 4; i++ {
		txns = append(txns, txn("f"+string(rune('0'+i)), "a1", ref.AddDate(0, -i, 0), -500))
	}
	in.Accounts = []domain.Account{acc}
	in.Transactions = txns

	got := a9FeeBleed(in)
	ins, ok := findInsight(got, "SMART-A9")
	if !ok {
		t.Fatal("expected a SMART-A9 fee-bleed insight")
	}
	if ins.Key != "SMART-A9:a1" {
		t.Errorf("key = %q, want SMART-A9:a1", ins.Key)
	}
	if ins.Action == nil || ins.Action.Kind != smart.ActionCreateTask || ins.Action.RelatedID != "a1" {
		t.Errorf("expected a create-task action tied to the account, got %+v", ins.Action)
	}
}

func TestA9NoFlagWithRealActivity(t *testing.T) {
	in := baseInput()
	acc := acct("a1", "Active", domain.TypeChecking, 100000, ref)
	in.Accounts = []domain.Account{acc}
	in.Transactions = []domain.Transaction{
		txn("f1", "a1", ref.AddDate(0, -1, 0), -500),   // a fee
		txn("d1", "a1", ref.AddDate(0, -1, 0), 200000), // but also a real deposit → not dormant
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Error("account with real activity should not be flagged as fee-bleed")
	}
}

func TestA9NoFlagLargeDebit(t *testing.T) {
	in := baseInput()
	acc := acct("a1", "Spender", domain.TypeChecking, 100000, ref)
	in.Accounts = []domain.Account{acc}
	in.Transactions = []domain.Transaction{
		txn("f1", "a1", ref.AddDate(0, -1, 0), -500),  // a fee
		txn("b1", "a1", ref.AddDate(0, -2, 0), -8000), // a real $80 purchase → not just fees
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Error("account with a non-trivial debit should not be flagged as fee-bleed")
	}
}

func TestA9ConvertsEURFeesToHouseholdBase(t *testing.T) {
	in := baseInput()
	in.Rates = currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	in.Accounts = []domain.Account{acct("eur", "Euro account", domain.TypeChecking, 10000, ref)}
	for i := 1; i <= 4; i++ {
		in.Transactions = append(in.Transactions, domain.Transaction{
			ID: string(rune('a' + i)), AccountID: "eur", Date: ref.AddDate(0, -i, 0),
			Amount: money.New(-500, "EUR"),
		})
	}
	ins, ok := findInsight(a9FeeBleed(in), "SMART-A9")
	if !ok {
		t.Fatal("expected fee-bleed insight for four small EUR fees")
	}
	if !ins.HasAmount || ins.Amount.Currency != "USD" || ins.Amount.Amount != 4400 || ins.AmountCadence != smart.AmountAnnual {
		t.Fatalf("annual impact = %+v cadence=%v, want $44.00/year USD", ins.Amount, ins.AmountCadence)
	}
}

func TestA9UsesZeroDecimalJPYThreshold(t *testing.T) {
	in := baseInput()
	in.Base = "JPY"
	in.Rates = currency.Rates{Base: "JPY"}
	yenAccount := acct("jpy", "Yen account", domain.TypeChecking, 10000, ref)
	yenAccount.Currency = "JPY"
	yenAccount.OpeningBalance = money.New(10000, "JPY")
	in.Accounts = []domain.Account{yenAccount}
	in.Transactions = []domain.Transaction{
		{ID: "fee", AccountID: "jpy", Date: ref.AddDate(0, -1, 0), Amount: money.New(-10, "JPY")},
		{ID: "large", AccountID: "jpy", Date: ref.AddDate(0, -2, 0), Amount: money.New(-31, "JPY")},
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Fatal("31 JPY is above the 30 JPY threshold and should count as real activity")
	}
}

func TestA9OmitsAccountWhenFeeCurrencyCannotConvert(t *testing.T) {
	in := baseInput() // no EUR rate
	in.Accounts = []domain.Account{acct("eur", "Unpriced euro account", domain.TypeChecking, 10000, ref)}
	in.Transactions = []domain.Transaction{
		{ID: "fee1", AccountID: "eur", Date: ref.AddDate(0, -1, 0), Amount: money.New(-500, "EUR")},
		{ID: "fee2", AccountID: "eur", Date: ref.AddDate(0, -2, 0), Amount: money.New(-500, "EUR")},
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Fatal("account with an unconvertible in-window debit must be omitted")
	}
}

func TestA9MinInt64ExpenseDoesNotWrapOrPanic(t *testing.T) {
	in := baseInput()
	in.Accounts = []domain.Account{acct("min", "Extreme account", domain.TypeChecking, 10000, ref)}
	in.Transactions = []domain.Transaction{
		{ID: "min-int", AccountID: "min", Date: ref.AddDate(0, -1, 0), Amount: money.New(math.MinInt64, "USD")},
		{ID: "fee", AccountID: "min", Date: ref.AddDate(0, -2, 0), Amount: money.New(-500, "USD")},
	}
	if _, ok := findInsight(a9FeeBleed(in), "SMART-A9"); ok {
		t.Fatal("MinInt64 debit must not wrap into a fee-sized magnitude")
	}
}
