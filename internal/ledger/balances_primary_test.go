// SPDX-License-Identifier: MIT

package ledger

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBulkBalancesDropAccountAfterFirstTransactionError(t *testing.T) {
	tests := []struct {
		name    string
		account domain.Account
		bad     domain.Transaction
		later   domain.Transaction
	}{
		{
			name:    "currency mismatch",
			account: domain.Account{ID: "bad", Currency: "USD", OpeningBalance: money.New(100, "USD")},
			bad:     domain.Transaction{AccountID: "bad", Amount: money.New(1, "EUR"), Cleared: true},
			later:   domain.Transaction{AccountID: "bad", Amount: money.New(-1, "USD"), Cleared: true},
		},
		{
			name:    "overflow",
			account: domain.Account{ID: "bad", Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")},
			bad:     domain.Transaction{AccountID: "bad", Amount: money.New(1, "USD"), Cleared: true},
			later:   domain.Transaction{AccountID: "bad", Amount: money.New(-1, "USD"), Cleared: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			good := domain.Account{ID: "good", Currency: "USD", OpeningBalance: money.New(10, "USD")}
			goodTxn := domain.Transaction{AccountID: good.ID, Amount: money.New(2, "USD"), Cleared: true}
			txns := []domain.Transaction{tt.bad, goodTxn, tt.later}
			accounts := []domain.Account{tt.account, good}

			for _, variant := range []struct {
				name string
				bulk func([]domain.Account, []domain.Transaction) (map[string]money.Money, error)
				one  func(domain.Account, []domain.Transaction) (money.Money, error)
			}{
				{"all", Balances, Balance},
				{"cleared", ClearedBalances, ClearedBalance},
			} {
				t.Run(variant.name, func(t *testing.T) {
					got, err := variant.bulk(accounts, txns)
					if err == nil {
						t.Fatal("bulk calculation should report the corrupt account")
					}
					wantBad, oneErr := variant.one(tt.account, txns)
					if oneErr == nil || err.Error() != oneErr.Error() {
						t.Fatalf("bulk error %v, per-account error %v", err, oneErr)
					}
					if _, ok := got[tt.account.ID]; ok {
						t.Fatalf("corrupt account retained in bulk result: %+v (single result %+v)", got[tt.account.ID], wantBad)
					}
					wantGood, goodErr := variant.one(good, txns)
					if goodErr != nil {
						t.Fatal(goodErr)
					}
					if gotGood, ok := got[good.ID]; !ok || gotGood != wantGood {
						t.Fatalf("unaffected account = %+v, present %v; want %+v", gotGood, ok, wantGood)
					}
				})
			}
		})
	}
}

func TestNetWorthExplainedSkipsIrrelevantCorruptAccounts(t *testing.T) {
	accounts := []domain.Account{
		{ID: "included", Name: "Included", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(100, "USD")},
		{ID: "archived", Name: "Archived", Class: domain.ClassAsset, Currency: "USD", Archived: true, OpeningBalance: money.New(1, "EUR")},
		{ID: "excluded", Name: "Excluded", Class: domain.ClassAsset, Currency: "USD", ExcludeFromNetWorth: true, OpeningBalance: money.New(1, "EUR")},
	}
	got, err := NetWorthExplained(accounts, nil, currency.Rates{Base: "USD"})
	if err != nil {
		t.Fatalf("irrelevant malformed account caused error: %v", err)
	}
	if got.Net.Amount != 100 || len(got.ExcludedByChoice) != 1 || got.ExcludedByChoice[0] != "Excluded" {
		t.Fatalf("result = %+v", got)
	}
}

func TestNetWorthExplainedPreservesBalanceErrorAndDuplicateIDSemantics(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	bad := domain.Account{ID: "bad", Name: "Bad", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(1, "EUR")}
	_, wantErr := Balance(bad, nil)
	_, gotErr := NetWorthExplained([]domain.Account{bad}, nil, rates)
	if wantErr == nil || gotErr == nil || gotErr.Error() != wantErr.Error() {
		t.Fatalf("NetWorthExplained error = %v, Balance error = %v", gotErr, wantErr)
	}

	// A map keyed by ID cannot encode the two distinct balances. NetWorthExplained
	// must retain its historical per-account behavior for duplicate IDs.
	accounts := []domain.Account{
		{ID: "same", Name: "First", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(10, "USD")},
		{ID: "same", Name: "Second", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(20, "USD")},
	}
	got, err := NetWorthExplained(accounts, nil, rates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Assets.Amount != 30 {
		t.Fatalf("assets = %d, want 30", got.Assets.Amount)
	}

	// The duplicate-ID fallback must also preserve wrapped arithmetic errors.
	first := domain.Account{ID: "overflow", Name: "Overflow", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.New(math.MaxInt64, "USD")}
	txns := []domain.Transaction{{AccountID: first.ID, Amount: money.New(1, "USD")}}
	_, wantErr = Balance(first, txns)
	_, gotErr = NetWorthExplained([]domain.Account{
		first,
		{ID: first.ID, Name: "Duplicate", Class: domain.ClassAsset, Currency: "USD", OpeningBalance: money.Zero("USD")},
	}, txns, rates)
	if wantErr == nil || gotErr == nil || gotErr.Error() != wantErr.Error() || !errors.Is(gotErr, money.ErrOverflow) {
		t.Fatalf("duplicate-ID NetWorthExplained error = %v, Balance error = %v", gotErr, wantErr)
	}
}

func BenchmarkNetWorthExplained(b *testing.B) {
	const accountCount, transactionCount = 100, 10_000
	accounts := make([]domain.Account, accountCount)
	for i := range accounts {
		accounts[i] = domain.Account{
			ID: fmt.Sprintf("a-%03d", i), Class: domain.ClassAsset, Currency: "USD",
			OpeningBalance: money.New(100_000, "USD"),
		}
	}
	txns := make([]domain.Transaction, transactionCount)
	for i := range txns {
		txns[i] = domain.Transaction{
			AccountID: accounts[i%accountCount].ID,
			Amount:    money.New(1, "USD"),
		}
	}
	rates := currency.Rates{Base: "USD"}

	b.Run("old-per-account", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := netWorthAccumulate(accounts, rates, func(a domain.Account) (money.Money, error) {
				return Balance(a, txns)
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("optimized", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := NetWorthExplained(accounts, txns, rates)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
