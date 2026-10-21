// SPDX-License-Identifier: MIT

package balancesheet

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestComposedSeriesMatchesNetWorthWithChosenExclusions(t *testing.T) {
	accounts := []domain.Account{acct("own", domain.ClassAsset, domain.TypeChecking, 10000), acct("chosen", domain.ClassAsset, domain.TypeInvestment, 900000), acct("chosen-debt", domain.ClassLiability, domain.TypeLoan, -70000), {ID: "unrated", Currency: "EUR", OpeningBalance: money.New(100, "EUR"), ExcludeFromNetWorth: true}}
	accounts[1].ExcludeFromNetWorth, accounts[2].ExcludeFromNetWorth = true, true
	txns := []domain.Transaction{tx("own-txn", "own", 3, 200), tx("chosen-txn", "chosen", 3, 500000)}
	cutoffs := []time.Time{day(1), day(5)}
	rates := currency.Rates{Base: "USD"}
	got, err := Series(accounts, txns, cutoffs, rates)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range got {
		if p.NetMinor != want[i].Amount || p.AssetsMinor != []int64{10000, 10200}[i] || p.LiabilitiesMinor != 0 || p.Assets[BucketInvested] != 0 {
			t.Fatalf("point=%+v canonical=%v", p, want[i])
		}
	}
}
