// SPDX-License-Identifier: MIT

package accountview

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBuildMatchesLedgerIncludingErrors(t *testing.T) {
	accounts := []domain.Account{
		{ID: "valid", Currency: "USD", OpeningBalance: money.New(100, "USD")},
		{ID: "wrong-opening", Currency: "USD", OpeningBalance: money.New(100, "EUR")},
		{ID: "wrong-txn", Currency: "USD", OpeningBalance: money.New(100, "USD")},
		{ID: "empty", Currency: "USD"},
	}
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	txns := []domain.Transaction{
		{AccountID: "valid", Amount: money.New(50, "USD"), Cleared: true, Date: now},
		{AccountID: "valid", Amount: money.New(-20, "USD"), Date: now.AddDate(0, 0, -2)},
		{AccountID: "wrong-txn", Amount: money.New(20, "EUR"), Cleared: true},
		{AccountID: "wrong-txn", Amount: money.New(10, "USD"), Cleared: true},
	}
	s := Build(accounts, txns)
	for _, a := range accounts {
		want, _ := ledger.Balance(a, txns)
		if s.Balances[a.ID] != want {
			t.Errorf("%s balance = %v, want %v", a.ID, s.Balances[a.ID], want)
		}
		want, _ = ledger.ClearedBalance(a, txns)
		if s.Cleared[a.ID] != want {
			t.Errorf("%s cleared = %v, want %v", a.ID, s.Cleared[a.ID], want)
		}
	}
	if !s.Earliest["valid"].Equal(now.AddDate(0, 0, -2)) {
		t.Fatal("earliest date lost")
	}
	if !reflect.DeepEqual(s.Transactions["valid"], txns[:2]) {
		t.Fatal("transaction order changed")
	}
}

func TestBuildBillPaymentsUsesBillLinkRatherThanBookingAccount(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}, {ID: "card", Currency: "USD"}}
	txns := []domain.Transaction{
		{ID: "old", AccountID: "checking", BillAccountID: "card", Amount: money.New(-100, "USD"), Date: now.AddDate(0, 0, -1)},
		{ID: "first-tie", AccountID: "checking", BillAccountID: "card", Amount: money.New(-200, "USD"), Date: now},
		{ID: "last-tie", AccountID: "checking", BillAccountID: "card", Amount: money.New(-300, "USD"), Date: now},
	}
	s := Build(accounts, txns)
	for _, account := range accounts {
		if got, want := s.BillPayments[account.ID], ledger.BillPaymentForAccount(account.ID, txns); got != want {
			t.Fatalf("%s payment = %v, want %v", account.ID, got, want)
		}
	}
}

func TestBuildIndexedReusesRowsAndMatchesStandaloneSnapshot(t *testing.T) {
	accounts, txns := benchmarkData()
	want := Build(accounts, txns)
	got := BuildIndexed(accounts, txns, want.Transactions)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("existing transaction index changed the snapshot")
	}
	for _, a := range accounts {
		if &got.Transactions[a.ID][0] != &want.Transactions[a.ID][0] {
			t.Fatalf("%s transaction rows copied", a.ID)
		}
	}
}

func TestAssetsScopeOrderAndStableTies(t *testing.T) {
	accounts := []domain.Account{
		{ID: "a", OwnerID: "m", Currency: "USD"},
		{ID: "b", OwnerID: domain.GroupOwnerID, Currency: "EUR"},
		{ID: "c", OwnerID: "m", Currency: "USD"},
		{ID: "other", OwnerID: "other"},
		{ID: "debt", OwnerID: "m", Class: domain.ClassLiability},
		{ID: "archived", OwnerID: "m", Archived: true},
	}
	balances := map[string]money.Money{"a": money.New(200, "USD"), "b": money.New(100, "EUR"), "c": money.New(200, "USD")}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	active, archived := Assets(accounts, balances, rates, "USD", "m")
	if got := ids(active); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("active = %v", got)
	}
	if got := ids(archived); !reflect.DeepEqual(got, []string{"archived"}) {
		t.Fatalf("archived = %v", got)
	}
	if accounts[0].ID != "a" {
		t.Fatal("mutated input")
	}
}

func TestSortRiskFirstKeepsStaleAndDebtOrdering(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "fresh", Type: domain.TypeChecking, BalanceAsOf: now},
		{ID: "stale", Type: domain.TypeChecking, BalanceAsOf: now.AddDate(0, 0, -45)},
		{ID: "debt", Type: domain.TypeChecking, Class: domain.ClassLiability, BalanceAsOf: now},
	}
	balances := map[string]money.Money{"fresh": money.New(1000, "USD"), "stale": money.New(1, "USD"), "debt": money.New(500, "USD")}
	SortRiskFirst(accounts, balances, freshness.DefaultWindows(), currency.Rates{Base: "USD"}, "USD", now)
	if got := ids(accounts); !reflect.DeepEqual(got, []string{"stale", "fresh", "debt"}) {
		t.Fatal(got)
	}
}

func ids(accounts []domain.Account) []string {
	out := make([]string, len(accounts))
	for i, a := range accounts {
		out[i] = a.ID
	}
	return out
}

func TestGroupsClaimOnceAndKeepUngroupedOrder(t *testing.T) {
	accounts := []domain.Account{{ID: "a", Currency: "USD"}, {ID: "b", Class: domain.ClassLiability, Currency: "USD"}, {ID: "c", Currency: "USD"}, {ID: "d", Currency: "USD"}}
	groups := []domain.AccountGroup{{ID: "first", AccountIDs: []string{"missing", "b", "b", "a"}}, {ID: "empty", AccountIDs: []string{"b"}}}
	balances := map[string]money.Money{"a": money.New(500, "USD"), "b": money.New(-100, "USD"), "c": money.New(30, "USD"), "d": money.New(20, "USD")}
	got := Groups(accounts, groups, balances, currency.Rates{Base: "USD"}, "USD")
	if len(got) != 2 || got[0].Group.ID != "first" || got[0].Subtotal.Amount != 400 || got[1].Subtotal.Amount != 50 {
		t.Fatalf("groups = %+v", got)
	}
	if !reflect.DeepEqual(ids(got[0].Accounts), []string{"b", "a"}) || !reflect.DeepEqual(ids(got[1].Accounts), []string{"c", "d"}) {
		t.Fatalf("account assignment = %+v", got)
	}
}

func TestGroupSubtotalDisclosesMissingFXAndNeverAssumesParity(t *testing.T) {
	accounts := []domain.Account{{ID: "usd", Currency: "USD"}, {ID: "gbp", Currency: "GBP"}}
	balances := map[string]money.Money{"usd": money.New(100, "USD"), "gbp": money.New(10000, "GBP")}
	got := Groups(accounts, nil, balances, currency.Rates{Base: "USD"}, "USD")
	if len(got) != 1 || got[0].Subtotal.Amount != 100 || !got[0].Incomplete || !reflect.DeepEqual(got[0].ExcludedCurrencies, []string{"GBP"}) {
		t.Fatalf("missing FX subtotal = %+v", got)
	}
}

func benchmarkData() ([]domain.Account, []domain.Transaction) {
	accounts := make([]domain.Account, 80)
	for i := range accounts {
		accounts[i] = domain.Account{ID: fmt.Sprint(i), Currency: "USD", OpeningBalance: money.New(int64(i*100), "USD")}
	}
	txns := make([]domain.Transaction, 20000)
	for i := range txns {
		txns[i] = domain.Transaction{AccountID: accounts[i%len(accounts)].ID, Amount: money.New(int64(i%37), "USD"), Cleared: i%2 == 0}
	}
	return accounts, txns
}

func BenchmarkAssetSelection(b *testing.B) {
	accounts, txns := benchmarkData()
	rates := currency.Rates{Base: "USD"}
	b.Run("scan-comparator", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			shown := append([]domain.Account(nil), accounts...)
			sort.SliceStable(shown, func(i, j int) bool {
				a, _ := ledger.Balance(shown[i], txns)
				z, _ := ledger.Balance(shown[j], txns)
				return a.Amount > z.Amount
			})
		}
	})
	b.Run("indexed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			s := Build(accounts, txns)
			Assets(accounts, s.Balances, rates, "USD", "")
		}
	})
}

func TestContributionAndGroupsUseTheStatedSignForEitherLiabilityConvention(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	cases := []struct {
		name    string
		account domain.Account
		balance int64
		want    int64
	}{
		{"asset keeps its sign", domain.Account{ID: "a", Currency: "USD"}, 700, 700},
		{"debt stored positive reads negative", domain.Account{ID: "a", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(900, "USD")}, 500, -500},
		{"debt stored negative reads negative", domain.Account{ID: "a", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(-900, "USD")}, -500, -500},
		{"overpaid positive-owed card reads positive", domain.Account{ID: "a", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(900, "USD")}, -50, 50},
		{"overpaid negative-owed card reads positive", domain.Account{ID: "a", Currency: "USD", Class: domain.ClassLiability, OpeningBalance: money.New(-900, "USD")}, 50, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			balance := money.New(tc.balance, "USD")
			if got := Contribution(tc.account, balance, rates, "USD"); got != tc.want {
				t.Fatalf("Contribution = %d, want %d", got, tc.want)
			}
			sections := Groups([]domain.Account{tc.account}, nil, map[string]money.Money{"a": balance}, rates, "USD")
			if len(sections) != 1 || sections[0].Subtotal.Amount != tc.want || sections[0].Incomplete {
				t.Fatalf("Groups subtotal = %+v, want %d", sections, tc.want)
			}
		})
	}
}

func TestGroupSectionRejectsBalanceInAnotherCurrencyAndNamesTheBalanceCurrency(t *testing.T) {
	accounts := []domain.Account{{ID: "usd", Currency: "USD"}, {ID: "mismatch", Currency: "USD"}, {ID: "gbp", Currency: "GBP"}}
	balances := map[string]money.Money{"usd": money.New(100, "USD"), "mismatch": money.New(900, "EUR"), "gbp": money.New(5, "GBP")}
	got := Groups(accounts, nil, balances, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1}}, "USD")
	if len(got) != 1 || got[0].Subtotal.Amount != 100 || !got[0].Incomplete {
		t.Fatalf("mismatched-currency balance leaked into the subtotal: %+v", got)
	}
	if !reflect.DeepEqual(got[0].ExcludedCurrencies, []string{"GBP"}) {
		t.Fatalf("ExcludedCurrencies = %v, want only the unconvertible GBP", got[0].ExcludedCurrencies)
	}
}

func TestBuildBillPaymentsSurvivesMinInt64Amount(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{ID: "checking", Currency: "USD"}, {ID: "card", Currency: "USD"}}
	txns := []domain.Transaction{
		{ID: "ok", AccountID: "checking", BillAccountID: "card", Amount: money.New(-100, "USD"), Date: now.AddDate(0, 0, -1)},
		{ID: "min", AccountID: "checking", BillAccountID: "card", Amount: money.New(math.MinInt64, "USD"), Date: now},
	}
	info := Build(accounts, txns).BillPayments["card"]
	if info.Count != 2 || info.LatestTxn != "ok" || info.Latest.Amount != 100 {
		t.Fatalf("BillPayments = %+v, want both counted and the representable payment kept as latest", info)
	}
}

func TestContributionAndGroupsFollowTheDeclaredLiabilitySign(t *testing.T) {
	rates := currency.Rates{Base: "USD"}
	account := domain.Account{
		ID: "a", Currency: "USD", Class: domain.ClassLiability,
		LiabilitySign: domain.LiabilityNegativeOwed, OpeningBalance: money.New(900, "USD"),
	}
	balance := money.New(-500, "USD")
	if got := Contribution(account, balance, rates, "USD"); got != -500 {
		t.Fatalf("Contribution = %d, want -500", got)
	}
	sections := Groups([]domain.Account{account}, nil, map[string]money.Money{"a": balance}, rates, "USD")
	if len(sections) != 1 || sections[0].Subtotal.Amount != -500 {
		t.Fatalf("Groups subtotal = %+v, want -500", sections)
	}
}

func TestGroupSectionReportsOverflowWithoutExcludingACurrency(t *testing.T) {
	accounts := []domain.Account{{ID: "a", Currency: "USD"}, {ID: "b", Currency: "USD"}}
	balances := map[string]money.Money{"a": money.New(math.MaxInt64, "USD"), "b": money.New(1, "USD")}
	got := Groups(accounts, nil, balances, currency.Rates{Base: "USD"}, "USD")
	if len(got) != 1 || !got[0].Incomplete || !got[0].Overflow || len(got[0].ExcludedCurrencies) != 0 {
		t.Fatalf("overflowing subtotal = %+v, want Incomplete+Overflow and no excluded currency", got)
	}
}
