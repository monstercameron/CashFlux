// SPDX-License-Identifier: MIT

package runway

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSuggestCoverCheckedMatchesLegacyAndSurfacesFXErrors(t *testing.T) {
	now := time.Date(2026, time.July, 12, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "checking", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD", OpeningBalance: money.New(4_000, "USD"), BalanceAsOf: now},
		{ID: "savings", Name: "Savings", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeSavings, Currency: "EUR", OpeningBalance: money.New(9_000, "EUR"), BalanceAsOf: now},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	got, err := SuggestCoverChecked(5_000, accounts, nil, rates)
	if err != nil {
		t.Fatal(err)
	}
	want := SuggestCover(5_000, accounts, nil, rates)
	if got != want || got.SourceID != "savings" || got.AmountMinor != 5_000 {
		t.Fatalf("checked suggestion %+v, legacy %+v", got, want)
	}
	_, err = SuggestCoverChecked(5_000, accounts, nil, currency.Rates{Base: "USD"})
	if err == nil || !strings.Contains(err.Error(), "cover source \"savings\" currency") {
		t.Fatalf("missing FX must remain visible, got %v", err)
	}
}
