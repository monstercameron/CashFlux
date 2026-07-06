// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestAddDebtVarsSurface covers the per-debt variable surface (debt_<slug>_*) and the
// debt aggregate atoms + the credit_utilization molecule, so the engine — not inline
// view code — is the source of the debt figures.
func TestAddDebtVarsSurface(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	accounts := []domain.Account{
		// A credit card: $2,000 owed against a $5,000 limit (40% utilization), 19.99% APR,
		// $50 minimum. Opening balance is negative (a liability owes).
		{ID: "cc", Name: "Visa", Class: domain.ClassLiability, Type: domain.TypeCreditCard, Currency: "USD",
			OpeningBalance: usd(-200000), CreditLimit: usd(500000), InterestRateAPR: 19.99, MinPayment: usd(5000)},
		// A car loan: $10,000 owed, 6% APR, $200 minimum, no credit limit (installment).
		{ID: "car", Name: "Car Loan", Class: domain.ClassLiability, Type: domain.TypeLoan, Currency: "USD",
			OpeningBalance: usd(-1000000), InterestRateAPR: 6, MinPayment: usd(20000)},
		// An asset — must NOT produce debt_* vars.
		{ID: "chk", Name: "Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: usd(300000)},
	}
	vars := Vars(Data{Accounts: accounts, Rates: currency.Rates{Base: "USD"}, Now: now})

	want := map[string]float64{
		// per-debt (credit card)
		"debt_visa_balance":     2000,
		"debt_visa_apr":         19.99,
		"debt_visa_min_payment": 50,
		"debt_visa_limit":       5000,
		"debt_visa_available":   3000,
		"debt_visa_utilization": 40,
		// per-debt (installment loan — no limit, so utilization/available are 0)
		"debt_car_loan_balance":     10000,
		"debt_car_loan_apr":         6,
		"debt_car_loan_min_payment": 200,
		"debt_car_loan_limit":       0,
		"debt_car_loan_utilization": 0,
		// aggregate atoms
		"debt_count":         2,
		"revolving_balance":  2000, // only the credit card
		"credit_limit_total": 5000, // only the credit card
		"min_payments_total": 250,  // 50 + 200
		// molecule (formula, not code): 2000 / 5000 * 100
		"credit_utilization": 40,
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
	// An asset account must not leak into the debt surface.
	if _, ok := vars["debt_checking_balance"]; ok {
		t.Error("asset account should not produce debt_* variables")
	}
}

func TestDebtVarBasesSkipsArchivedAndAssets(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a", Name: "Card A", Class: domain.ClassLiability, Type: domain.TypeCreditCard},
		{ID: "b", Name: "Card A", Class: domain.ClassLiability, Type: domain.TypeCreditCard}, // name collision
		{ID: "c", Name: "Old Card", Class: domain.ClassLiability, Archived: true},            // archived → skipped
		{ID: "d", Name: "Savings", Class: domain.ClassAsset},                                 // asset → skipped
	}
	bases := DebtVarBases(accounts)
	if len(bases) != 2 || bases[0].Prefix != "debt_card_a_" || bases[1].Prefix != "debt_card_a_2_" {
		t.Errorf("debt var bases wrong: %+v", bases)
	}
}
