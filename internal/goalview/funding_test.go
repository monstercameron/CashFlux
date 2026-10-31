// SPDX-License-Identifier: MIT

package goalview

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
)

func fundingAccount(id, code string, amount int64) domain.Account {
	return domain.Account{ID: id, Name: id, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: code, OpeningBalance: money.New(amount, code)}
}

func TestFundingSuggestionPreservesLinkedPreferenceAndStoredTies(t *testing.T) {
	accounts := []domain.Account{fundingAccount("linked", "USD", 100), fundingAccount("largest", "USD", 200), fundingAccount("tie", "USD", 200)}
	g := goal("g", "USD", 0, 80)
	g.AccountID = "linked"
	for _, tt := range []struct {
		target  int64
		account string
		amount  int64
	}{{80, "linked", 80}, {150, "largest", 150}, {300, "largest", 200}} {
		g.TargetAmount.Amount = tt.target
		balances, _ := ledger.Balances(accounts, nil)
		f := BuildFunding([]domain.Goal{g}, accounts, balances, currency.Rates{Base: "USD"}, "USD")
		if got := f.Suggestions[g.ID]; got.AccountID != tt.account || got.Amount.Amount != tt.amount {
			t.Fatalf("target %d: %+v", tt.target, got)
		}
	}
}

func TestFundingCountsOwnAndForeignReservations(t *testing.T) {
	a := fundingAccount("bank", "USD", 100)
	g := goal("g", "USD", 0, 150)
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(30, "USD")}}
	other := goal("other", "EUR", 0, 100)
	other.Archived = true // archived reservations still consume available money
	other.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(10, "EUR")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	f := BuildFunding([]domain.Goal{g, other}, []domain.Account{a}, balances, rates, "USD")
	if got := f.Suggestions[g.ID]; got.Amount != money.New(50, "USD") {
		t.Fatalf("existing reservations ignored: %+v", got)
	}
	if _, ok := f.Suggestions[other.ID]; ok {
		t.Fatal("archived goal offered funding")
	}
	if f.Earmarks.TotalReserved.Amount != 50 || f.Earmarks.Free.Amount != 50 {
		t.Fatalf("map did not reconcile: %+v", f.Earmarks)
	}
}

func TestFundingConvertsThisGoalsOwnForeignAllocation(t *testing.T) {
	a := fundingAccount("bank", "USD", 10000)
	g := goal("g", "EUR", 0, 10000)
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	gap, err := fundingGap(g, rates)
	if err != nil || gap != money.New(9500, "EUR") {
		t.Fatalf("own foreign allocation gap = %v, %v; want 9500 EUR", gap, err)
	}
	f := BuildFunding([]domain.Goal{g}, []domain.Account{a}, balances, rates, "USD")
	if got := f.Suggestions[g.ID]; got.AccountID != a.ID || got.Amount != money.New(4500, "EUR") {
		t.Fatalf("own foreign allocation suppressed funding: %+v", got)
	}
	merged, err := AddReservationAmount(g.Allocations[0].Amount, f.Suggestions[g.ID].Amount, rates)
	if err != nil || merged != money.New(10000, "USD") {
		t.Fatalf("foreign reservation relabeled or added wrong units: %v, %v", merged, err)
	}
	g.Allocations[0].Amount = merged
	after := BuildFunding([]domain.Goal{g}, []domain.Account{a}, balances, rates, "USD")
	if after.Overbooked[g.ID] || len(after.Suggestions) != 0 {
		t.Fatalf("merged allocation did not consume exact headroom: %+v", after)
	}
	v := Build(Input{Goals: []domain.Goal{g}, Rates: rates, Base: "USD"})
	if v.SetAsideMin != money.New(10000, "USD") || len(v.ExcludedCurrencies) != 0 {
		t.Fatalf("valid foreign reservation excluded from summary: %+v", v)
	}
}

func TestFundingGapAggregatesOwnForeignAllocationsBeforeFX(t *testing.T) {
	g := goal("g", "EUR", 0, 100)
	g.Allocations = []domain.GoalAllocation{{Amount: money.New(1, "USD")}, {Amount: money.New(1, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	gap, err := fundingGap(g, rates)
	if err != nil || gap != money.New(99, "EUR") {
		t.Fatalf("allocation FX rounded before aggregation: %v, %v", gap, err)
	}
	g.Allocations[0].Amount = money.New(1, "GBP")
	if _, err := fundingGap(g, rates); err == nil {
		t.Fatal("missing own-allocation FX silently assumed parity")
	}
	g.Allocations = []domain.GoalAllocation{{Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(1, "USD")}}
	if _, err := fundingGap(g, rates); err == nil {
		t.Fatal("foreign allocation sum overflow accepted")
	}
}

func TestFundingAggregatesSmallForeignReservationsBeforeRounding(t *testing.T) {
	a := fundingAccount("bank", "USD", 100)
	first, second, receiver := goal("first", "EUR", 0, 100), goal("second", "EUR", 0, 100), goal("receiver", "USD", 0, 100)
	for _, g := range []*domain.Goal{&first, &second} {
		g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1, "EUR")}}
	}
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	f := BuildFunding([]domain.Goal{first, second, receiver}, []domain.Account{a}, balances, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.4}}, "USD")
	if got := f.Suggestions[receiver.ID].Amount.Amount; got != 99 {
		t.Fatalf("rounded reservations disappeared: headroom = %d, want 99", got)
	}
}

func TestFundingSuggestionChecksExistingForeignRoundingCarry(t *testing.T) {
	a := fundingAccount("bank", "USD", 1)
	g := goal("g", "EUR", 0, 5)
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1, "EUR")}}
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.4}}
	f := BuildFunding([]domain.Goal{g}, []domain.Account{a}, balances, rates, "USD")
	if got := f.Suggestions[g.ID].Amount.Amount; got != 2 {
		t.Fatalf("rounded existing reservation allowed overbooking: got %d, want 2", got)
	}
	g.Allocations[0].Amount.Amount += f.Suggestions[g.ID].Amount.Amount
	after := BuildFunding([]domain.Goal{g}, []domain.Account{a}, balances, rates, "USD")
	if after.Overbooked[g.ID] {
		t.Fatal("applying funding suggestion breached native account balance")
	}
}

func TestFundingNeverInventsMissingFXOrInvalidHeadroom(t *testing.T) {
	for _, tt := range []struct {
		name        string
		reservation money.Money
		balance     money.Money
	}{
		{"unknown reservation", money.New(20, "GBP"), money.New(100, "USD")},
		{"invalid balance", money.New(20, "USD"), money.Money{}},
		{"negative reservation", money.New(-20, "USD"), money.New(100, "USD")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := fundingAccount("bank", "USD", 100)
			g, other := goal("g", "USD", 0, 100), goal("other", "USD", 0, 100)
			other.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: tt.reservation}}
			f := BuildFunding([]domain.Goal{g, other}, []domain.Account{a}, map[string]money.Money{a.ID: tt.balance}, currency.Rates{Base: "USD"}, "USD")
			if len(f.Suggestions) != 0 || !f.Earmarks.Incomplete || len(f.Earmarks.Accounts) != 0 || f.Earmarks.Free.Amount != 0 {
				t.Fatalf("unsafe headroom: %+v", f)
			}
		})
	}
	a := fundingAccount("bank", "EUR", 100)
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	f := BuildFunding([]domain.Goal{goal("usd", "USD", 0, 100), goal("eur", "EUR", 0, 100)}, []domain.Account{a}, balances, currency.Rates{Base: "USD"}, "USD")
	if _, ok := f.Suggestions["usd"]; ok {
		t.Fatal("unknown EUR/USD rate treated as parity")
	}
	if f.Suggestions["eur"].Amount != money.New(100, "EUR") {
		t.Fatal("native-currency funding unnecessarily required FX")
	}
}

func TestFundingEligibilityAndMoneyMapParity(t *testing.T) {
	accounts := []domain.Account{fundingAccount("cash", "USD", 100), fundingAccount("held", "USD", 500), fundingAccount("archived", "USD", 1000), fundingAccount("debt", "USD", 1000)}
	accounts[1].Type, accounts[2].Archived, accounts[3].Class = domain.TypeProperty, true, domain.ClassLiability
	g := goal("g", "USD", 0, 200)
	g.Allocations = []domain.GoalAllocation{{AccountID: "cash", Amount: money.New(20, "USD")}, {AccountID: "held", Amount: money.New(50, "USD")}}
	balances, _ := ledger.Balances(accounts, nil)
	f := BuildFunding([]domain.Goal{g}, accounts, balances, currency.Rates{Base: "USD"}, "USD")
	v := f.Earmarks
	if f.Suggestions[g.ID].AccountID != "cash" || f.Suggestions[g.ID].Amount.Amount != 80 {
		t.Fatalf("ineligible funding source: %+v", f.Suggestions)
	}
	if v.TotalBalance.Amount != 600 || v.TotalReserved.Amount != 70 || v.Free.Amount != 80 || v.Held.Amount != 500 || v.Percent != 11 || len(v.Accounts) != 2 || v.Incomplete {
		t.Fatalf("money map = %+v", v)
	}
	if v.Accounts[0].Percent != 20 || v.Accounts[1].Percent != 10 {
		t.Fatalf("exposure percentages = %+v", v.Accounts)
	}
}

func TestFundingOverflowDisclosesOmissionsAndKeepsOverbooking(t *testing.T) {
	a := fundingAccount("bank", "USD", math.MaxInt64)
	first, second := goal("first", "USD", 0, 100), goal("second", "USD", 0, 100)
	first.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(math.MaxInt64, "USD")}}
	second.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1, "USD")}}
	balances, _ := ledger.Balances([]domain.Account{a}, nil)
	f := BuildFunding([]domain.Goal{first, second}, []domain.Account{a}, balances, currency.Rates{Base: "USD"}, "USD")
	if !f.Overbooked[first.ID] || !f.Overbooked[second.ID] || !f.Earmarks.Incomplete || len(f.Suggestions) != 0 || f.Earmarks.TotalReserved.Amount < 0 {
		t.Fatalf("overflow hidden: %+v", f)
	}
	if sharePercent(math.MaxInt64/2, math.MaxInt64) != 49 {
		t.Fatal("exposure percentage overflowed")
	}
}

func TestFundTargetsUsesFinancialConvertibleSubset(t *testing.T) {
	usd, eur, unknown := goal("usd", "USD", 0, 100), goal("eur", "EUR", 0, 100), goal("unknown", "GBP", 0, 10000)
	check, archived := goal("check", "USD", 0, 999), goal("archived", "USD", 0, 999)
	check.Kind, archived.Archived = domain.GoalKindChecklist, true
	total, excluded := FundTargets([]domain.Goal{usd, eur, unknown, check, archived}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, "USD")
	if total != money.New(300, "USD") || !reflect.DeepEqual(excluded, []string{"GBP"}) {
		t.Fatalf("fund targets = %v, %v", total, excluded)
	}
	usd.TargetAmount.Amount, eur.TargetAmount = math.MaxInt64, money.New(1, "USD")
	total, excluded = FundTargets([]domain.Goal{usd, eur}, currency.Rates{Base: "USD"}, "USD")
	if total.Amount != math.MaxInt64 || !reflect.DeepEqual(excluded, []string{"USD"}) {
		t.Fatalf("overflowed fund targets = %v, %v", total, excluded)
	}
}

func BenchmarkFundingSuggestions(b *testing.B) {
	accounts := make([]domain.Account, 80)
	for i := range accounts {
		accounts[i] = fundingAccount(fmt.Sprint(i), "USD", 10000)
	}
	all := make([]domain.Goal, 100)
	for i := range all {
		all[i] = goal(fmt.Sprint(i), "USD", 0, 1000)
	}
	txns := make([]domain.Transaction, 20000)
	for i := range txns {
		txns[i] = domain.Transaction{AccountID: accounts[i%len(accounts)].ID, Amount: money.New(1, "USD")}
	}
	balances, _ := ledger.Balances(accounts, txns)
	b.Run("repeated-ledger-scan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for range all {
				for _, a := range accounts {
					if _, err := ledger.Balance(a, txns); err != nil {
						b.Fatal(err)
					}
				}
			}
		}
	})
	b.Run("shared-index", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			f := BuildFunding(all, accounts, balances, currency.Rates{Base: "USD"}, "USD")
			if len(f.Suggestions) != len(all) {
				b.Fatal("missing suggestions")
			}
		}
	})
}
