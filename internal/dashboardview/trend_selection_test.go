// SPDX-License-Identifier: MIT

package dashboardview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNetWorthTrendKeepsChosenAccountsOutOfSparkAndDelta(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "own", Currency: "USD", OpeningBalance: money.New(10000, "USD")},
		{ID: "chosen", Currency: "USD", OpeningBalance: money.New(1000000, "USD"), ExcludeFromNetWorth: true},
		{ID: "unrated", Currency: "EUR", OpeningBalance: money.New(100, "EUR"), ExcludeFromNetWorth: true},
	}
	txns := []domain.Transaction{
		{AccountID: "own", Date: now.AddDate(0, 0, -1), Amount: money.New(200, "USD")},
		{AccountID: "chosen", Date: now.AddDate(0, 0, -1), Amount: money.New(900000, "USD")},
	}
	got := BuildNetWorthTrend(accounts, txns, currency.Rates{Base: "USD"}, now)
	if len(got.Spark) != 7 || got.Spark[0] != 10000 || got.Spark[6] != 10200 || !got.Change.Known || got.Change.DeltaMinor() != 200 {
		t.Fatalf("trend=%+v", got)
	}
	accounts[2].ExcludeFromNetWorth = false
	got = BuildNetWorthTrend(accounts, txns, currency.Rates{Base: "USD"}, now)
	if len(got.Spark) != 0 || got.Change.Known {
		t.Fatalf("included unknown FX supplied a trend: %+v", got)
	}
}
