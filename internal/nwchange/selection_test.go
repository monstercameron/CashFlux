// SPDX-License-Identifier: MIT

package nwchange

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestChangeMatchesHeadlineWithChosenExclusions(t *testing.T) {
	now := day(2026, time.July, 15)
	accounts := []domain.Account{checking("own", 10000), checking("custodial", 1000000), {ID: "unrated", Currency: "EUR", OpeningBalance: money.New(100, "EUR"), ExcludeFromNetWorth: true}}
	accounts[1].ExcludeFromNetWorth = true
	txns := []domain.Transaction{txn("own", day(2026, time.July, 1), 200), txn("custodial", day(2026, time.July, 2), 900000)}
	got, err := MonthToDateChange(accounts, txns, currency.Rates{Base: "USD"}, now)
	if err != nil || !got.Known || got.StartMinor != 10000 || got.EndMinor != 10200 || got.DeltaMinor() != 200 {
		t.Fatalf("change=%+v err=%v", got, err)
	}
	headline, err := ledger.NetWorthExplained(accounts, txns, currency.Rates{Base: "USD"})
	if err != nil || got.EndMinor != headline.Net.Amount {
		t.Fatalf("headline=%+v err=%v", headline, err)
	}
	accounts[2].ExcludeFromNetWorth = false
	unknown, err := MonthToDateChange(accounts, txns, currency.Rates{Base: "USD"}, now)
	if err == nil || unknown.Known {
		t.Fatalf("included missing rate produced known change=%+v err=%v", unknown, err)
	}
}
