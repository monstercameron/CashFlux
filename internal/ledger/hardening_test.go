// SPDX-License-Identifier: MIT

package ledger

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestNetByOwnerHandlesPositiveOwedLiabilitiesAndExcludedAccounts(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	accounts := []domain.Account{
		{
			ID: "chk", OwnerID: "alice", Class: domain.ClassAsset, Type: domain.TypeChecking,
			Currency: "USD", OpeningBalance: money.New(100000, "USD"),
		},
		{
			ID: "cc-pos", OwnerID: "alice", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			LiabilitySign: domain.LiabilityPositiveOwed,
			Currency:      "USD", OpeningBalance: money.New(25000, "USD"),
		},
		{
			ID: "cc-neg", OwnerID: "bob", Class: domain.ClassLiability, Type: domain.TypeCreditCard,
			LiabilitySign: domain.LiabilityNegativeOwed,
			Currency:      "USD", OpeningBalance: money.New(-10000, "USD"),
		},
		{
			ID: "excl", OwnerID: "alice", Class: domain.ClassAsset, Type: domain.TypeSavings,
			Currency: "USD", OpeningBalance: money.New(500000, "USD"), ExcludeFromNetWorth: true,
		},
	}
	byOwner, err := NetByOwner(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetByOwner: %v", err)
	}
	if got := byOwner["alice"].Amount; got != 75000 {
		t.Fatalf("alice net = %d, want 75000 (100000 - 25000, excluding ExcludeFromNetWorth)", got)
	}
	if got := byOwner["bob"].Amount; got != -10000 {
		t.Fatalf("bob net = %d, want -10000", got)
	}
	nwNet, _, _, err := NetWorth(accounts, nil, rates)
	if err != nil {
		t.Fatalf("NetWorth: %v", err)
	}
	if sum := byOwner["alice"].Amount + byOwner["bob"].Amount; sum != nwNet.Amount {
		t.Fatalf("sum(NetByOwner)=%d != NetWorth=%d", sum, nwNet.Amount)
	}
}

func TestAnalyticsAndPaymentHelpersExtremeInt64Safe(t *testing.T) {
	// BillPaymentForAccount and SubscriptionPaymentForName must not panic on MinInt64
	// and must not let an earlier row overwrite a later row with an empty ID.
	d1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{ID: "", BillAccountID: "cc", SubscriptionName: "Net", Date: d2, Amount: money.New(math.MinInt64, "USD")},
		{ID: "old", BillAccountID: "cc", SubscriptionName: "Net", Date: d1, Amount: money.New(-500, "USD")},
	}
	bp := BillPaymentForAccount("cc", txns)
	if !bp.HasAny || bp.Count != 2 || bp.Latest.Amount != math.MaxInt64 || bp.LatestTxn != "" {
		t.Fatalf("BillPaymentForAccount = %+v, want latest from d2 with MaxInt64", bp)
	}
	sp := SubscriptionPaymentForName("Net", txns)
	if !sp.HasAny || sp.Count != 2 || sp.Latest.Amount != math.MaxInt64 || sp.LatestTxn != "" {
		t.Fatalf("SubscriptionPaymentForName = %+v, want latest from d2 with MaxInt64", sp)
	}

	// Overflow-safe SavingsRate, Utilization, PercentChange, Delta, RankSpending.
	if got := SavingsRate(math.MaxInt64, 0); got != 100 {
		t.Fatalf("SavingsRate(MaxInt64, 0) = %d, want 100", got)
	}
	if got, ok := Utilization(math.MinInt64, math.MaxInt64); !ok || got != 100 {
		t.Fatalf("Utilization(MinInt64, MaxInt64) = %d,%v, want 100,true", got, ok)
	}
	if got, ok := PercentChange(0, math.MinInt64); !ok || got != 100 {
		t.Fatalf("PercentChange(0, MinInt64) = %d,%v, want 100,true", got, ok)
	}
	if got := Delta(math.MaxInt64, math.MinInt64); got.Kind != DeltaPctKind || got.Pct != 199 {
		t.Fatalf("Delta(MaxInt64, MinInt64) = %+v, want +199%%", got)
	}
	_, other := RankSpending(map[string]int64{"a": math.MaxInt64, "b": math.MaxInt64, "c": 100}, 1)
	if other != math.MaxInt64 {
		t.Fatalf("RankSpending overflow other = %d, want MaxInt64", other)
	}
}
