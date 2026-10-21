// SPDX-License-Identifier: MIT

package anomalyattrib

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func purchaseTxn(id string, minor int64, date time.Time) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: "food", Desc: "Market", Amount: money.New(minor, "USD"), Date: date}
}

func TestCategoryPurchasesUsesSplitsAndConsistentMerchantNames(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	prior := purchaseTxn("prior", -10000, now.AddDate(0, -1, 0))
	prior.Splits = []domain.CategorySplit{{CategoryID: "food", Amount: money.New(-5000, "USD")}, {CategoryID: "other", Amount: money.New(-5000, "USD")}}
	current := purchaseTxn("current", -20000, now)
	current.Splits = []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-5000, "USD")},
		{CategoryID: "food", Amount: money.New(-5000, "USD")},
		{CategoryID: "other", Amount: money.New(-11000, "USD")},
		{CategoryID: "food", Amount: money.New(1000, "USD")},
	}
	got := CategoryPurchases("food", []domain.Transaction{current, prior}, currency.Rates{Base: "USD"}, now, "Unnamed")
	if len(got) != 1 || got[0].Minor != 10000 || got[0].Payee != "Market" || !got[0].Larger {
		t.Fatalf("split evidence or merchant baseline incorrect: %+v", got)
	}
	if current.Splits[0].Amount.Amount != -5000 {
		t.Fatal("shared transaction mutated")
	}
}

func TestCategoryPurchasesFiltersAndConverts(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	fx := purchaseTxn("fx", -1000, now)
	fx.Amount.Currency = "EUR"
	missing := purchaseTxn("missing", -900000, now)
	missing.Amount.Currency = "JPY"
	excluded := purchaseTxn("excluded", -900000, now)
	excluded.ExcludeFromReports = true
	transfer := purchaseTxn("transfer", -900000, now)
	transfer.TransferAccountID = "savings"
	got := CategoryPurchases("food", []domain.Transaction{
		fx, missing, excluded, transfer,
		purchaseTxn("future", -900000, now.AddDate(0, 1, 0)),
		purchaseTxn("income", 900000, now),
	}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, now, "Unnamed")
	if len(got) != 1 || got[0].ID != "fx" || got[0].Minor != 2000 || got[0].Larger {
		t.Fatalf("incorrect filters or conversion: %+v", got)
	}
}

func TestCategoryPurchasesLargeComparisonDoesNotOverflow(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		purchaseTxn("prior", -math.MaxInt64/2, now.AddDate(0, -1, 0)),
		purchaseTxn("current", -math.MaxInt64, now),
		purchaseTxn("invalid magnitude", math.MinInt64, now),
	}
	got := CategoryPurchases("food", txns, currency.Rates{Base: "USD"}, now, "Unnamed")
	if len(got) != 1 || !got[0].Larger {
		t.Fatalf("valid huge purchase lost its larger-than-usual flag: %+v", got)
	}
}
