// SPDX-License-Identifier: MIT

package txnview

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestTxnViewHardening(t *testing.T) {
	d := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "chk", Class: domain.ClassAsset, Currency: "USD"},
		{ID: "cc", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed, Currency: "USD", OpeningBalance: money.New(50000, "USD")},
	}
	// Positive-owed liability legacy transfer pairs two negative legs without double-counting.
	rows := []domain.Transaction{
		{ID: "out", AccountID: "chk", TransferAccountID: "cc", Date: d, Amount: money.New(-10000, "USD")},
		{ID: "in", AccountID: "cc", TransferAccountID: "chk", Date: d, Amount: money.New(-10000, "USD")},
	}
	v, err := SummarizeWithAccounts(rows, rows, accounts, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatalf("SummarizeWithAccounts: %v", err)
	}
	if v.Transferred != 10000 || v.MovedRows != 1 || v.UnpairedRows != 0 {
		t.Fatalf("positive-owed pair summary = %+v, want Transferred=10000 MovedRows=1 Unpaired=0", v)
	}

	// Split income transaction must not leak negative expense entries into Categories.
	incSplit := domain.Transaction{
		ID: "inc", Date: d, Amount: money.New(10000, "USD"),
		Splits: []domain.CategorySplit{{CategoryID: "salary", Amount: money.New(10000, "USD")}},
	}
	v2, err := SummarizeWithCategories([]domain.Transaction{incSplit}, []domain.Transaction{incSplit}, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatalf("split income: %v", err)
	}
	if len(v2.Categories) != 0 {
		t.Fatalf("split income leaked into expense categories: %+v", v2.Categories)
	}

	// Invalid FX rate (<=0) must fail rather than being swallowed as Unconverted.
	badRates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": -1}}
	eurRow := domain.Transaction{ID: "e", Date: d, Amount: money.New(-100, "EUR")}
	_, err = SummarizeWithCategories([]domain.Transaction{eurRow}, []domain.Transaction{eurRow}, badRates, "USD")
	if !errors.Is(err, currency.ErrInvalidRate) {
		t.Fatalf("invalid rate error = %v, want ErrInvalidRate", err)
	}

	// Mixed ExcludeFromReports transfer pair: non-excluded leg must still count.
	mixed := []domain.Transaction{
		{ID: "a", AccountID: "chk", TransferAccountID: "sav", Date: d, Amount: money.New(-5000, "USD"), ExcludeFromReports: true},
		{ID: "b", AccountID: "sav", TransferAccountID: "chk", Date: d, Amount: money.New(5000, "USD")},
	}
	v3, err := SummarizeWithCategories(mixed, mixed, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatalf("mixed exclusion: %v", err)
	}
	if v3.Transferred != 5000 || v3.MovedRows != 1 {
		t.Fatalf("mixed exclusion dropped non-excluded leg: %+v", v3)
	}
}
