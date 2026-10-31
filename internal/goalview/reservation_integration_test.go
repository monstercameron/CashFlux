// SPDX-License-Identifier: MIT

package goalview

import (
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestFundingReusesCanonicalReservationPosition(t *testing.T) {
	a := fundingAccount("bank", "USD", 10000)
	first, second := goal("first", "EUR", 0, 20000), goal("second", "USD", 0, 20000)
	first.Archived = true
	first.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(6000, "EUR")}}
	second.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}
	all, accounts := []domain.Goal{first, second}, []domain.Account{a}
	balances := map[string]money.Money{a.ID: a.OpeningBalance}
	for _, rate := range []float64{2, 1, 0} {
		rates := currency.Rates{Base: "USD", Rates: map[string]float64{}}
		if rate > 0 {
			rates.Rates["EUR"] = rate
		}
		index := goals.BuildAccountReservations(all, accounts, balances, rates)
		got := BuildFundingWithReservations(all, accounts, index, rates, "USD")
		want := BuildFunding(all, accounts, balances, rates, "USD")
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("reused index diverged at rate%v: %+v vs %+v", rate, got, want)
		}
		p := index.ByAccount[a.ID]
		if rate == 2 && (!p.Overbooked || !got.Overbooked[second.ID] || got.Earmarks.TotalReserved != p.Reserved) {
			t.Fatalf("breach disagrees: %+v %+v", p, got)
		}
		if rate == 1 && (p.Overbooked || got.Suggestions[second.ID].Amount.Amount != 3000) {
			t.Fatalf("headroom disagrees: %+v %+v", p, got)
		}
		if rate == 0 && (p.Known || !got.Earmarks.Incomplete || len(got.Suggestions) != 0) {
			t.Fatalf("unknown reservation was fundable: %+v %+v", p, got)
		}
	}
}
