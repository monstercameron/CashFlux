// SPDX-License-Identifier: MIT

package goals

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func reservationGoal(id, account, code string, amounts ...int64) domain.Goal {
	g := domain.Goal{ID: id, Name: id}
	for _, amount := range amounts {
		g.Allocations = append(g.Allocations, domain.GoalAllocation{AccountID: account, Amount: money.New(amount, code)})
	}
	return g
}

func TestAccountReservationsConvertRetainedClaimsAndDeduplicateIdentities(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "USD"}
	archived := reservationGoal("archived", a.ID, "EUR", 3000, 3000)
	archived.Archived = true
	goals := []domain.Goal{archived, reservationGoal("active", a.ID, "USD", 1000), reservationGoal("active", a.ID, "USD", 0)}
	balances := map[string]money.Money{a.ID: money.New(10000, "USD")}
	before, _ := json.Marshal([]any{a, goals, balances})
	index := BuildAccountReservations(goals, []domain.Account{a}, balances, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	p := index.ByAccount[a.ID]
	if !p.Known || !p.ReservedKnown || !p.BalanceKnown || p.Reserved.Amount != 13000 || p.Free.Amount != -3000 || p.Over.Amount != 3000 || !p.Overbooked || p.Err != nil {
		t.Fatalf("position=%+v", p)
	}
	if !reflect.DeepEqual(p.GoalRefs, []ReservationGoal{{"archived", "archived"}, {"active", "active"}}) {
		t.Fatalf("identities=%v", p.GoalRefs)
	}
	if index.Incomplete || index.ByCurrency[a.ID]["EUR"] != 6000 {
		t.Fatalf("index=%+v", index)
	}
	after, _ := json.Marshal([]any{a, goals, balances})
	if string(before) != string(after) {
		t.Fatal("source mutated")
	}
}

func TestAccountReservationsAggregateBeforeRoundingAndRetainArchivedSources(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "USD", Archived: true}
	index := BuildAccountReservations([]domain.Goal{reservationGoal("same", a.ID, "EUR", 1, 1)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(1, "USD")}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": .4}})
	p := index.ByAccount[a.ID]
	if !p.Known || p.Reserved.Amount != 1 || p.Free.Amount != 0 || len(p.GoalRefs) != 1 || !p.Account.Archived {
		t.Fatalf("rounded retained position=%+v", p)
	}
	zero := BuildAccountReservations(nil, []domain.Account{a}, map[string]money.Money{a.ID: money.Zero("USD")}, currency.Rates{Base: "USD"}).ByAccount[a.ID]
	if !zero.Known || !zero.ReservedKnown || !zero.BalanceKnown || zero.Free.Amount != 0 || zero.Overbooked || len(zero.GoalRefs) != 0 {
		t.Fatalf("valid zero=%+v", zero)
	}
}

func TestAccountReservationsUnavailableNeverMeansHealthyZero(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "USD"}
	for _, tc := range []struct {
		name          string
		goals         []domain.Goal
		accounts      []domain.Account
		balances      map[string]money.Money
		cause         error
		reservedKnown bool
	}{
		{"missing FX", []domain.Goal{reservationGoal("g", a.ID, "EUR", 10)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(100, "USD")}, currency.ErrUnknownRate, false},
		{"negative reservation", []domain.Goal{reservationGoal("g", a.ID, "USD", -1)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(100, "USD")}, ErrInvalidReservation, false},
		{"native sum overflow", []domain.Goal{reservationGoal("g", a.ID, "USD", math.MaxInt64, 1)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(100, "USD")}, money.ErrOverflow, false},
		{"missing source", []domain.Goal{reservationGoal("g", a.ID, "USD", 10)}, nil, nil, ErrReservationUnavailable, false},
		{"missing balance", []domain.Goal{reservationGoal("g", a.ID, "USD", 10)}, []domain.Account{a}, nil, ErrReservationUnavailable, true},
		{"wrong balance currency", []domain.Goal{reservationGoal("g", a.ID, "USD", 10)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(100, "EUR")}, ErrReservationUnavailable, true},
		{"headroom overflow", []domain.Goal{reservationGoal("g", a.ID, "USD", 1)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(math.MinInt64, "USD")}, money.ErrOverflow, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := BuildAccountReservations(tc.goals, tc.accounts, tc.balances, currency.Rates{Base: "USD"})
			p := index.ByAccount[a.ID]
			if !index.Incomplete || p.Known || p.ReservedKnown != tc.reservedKnown || !errors.Is(p.Err, tc.cause) || len(p.GoalRefs) != 1 {
				t.Fatalf("unknown=%+v", p)
			}
			if !p.ReservedKnown && p.Reserved.Amount != 0 {
				t.Fatalf("unavailable reservation exposed partial sum=%+v", p)
			}
		})
	}
}

func TestReservationWarningsRequireDistinctGoalsAndKeepUnknownPositions(t *testing.T) {
	a := domain.Account{ID: "a", Name: "Bank", Currency: "USD"}
	balances := map[string]money.Money{a.ID: money.New(100, "USD")}
	all := []domain.Goal{reservationGoal("same", a.ID, "USD", 100, 100), reservationGoal("same", a.ID, "USD", 1)}
	index := BuildAccountReservations(all, []domain.Account{a}, balances, currency.Rates{Base: "USD"})
	if len(index.Warnings(1)) != 1 || len(index.Warnings(2)) != 0 {
		t.Fatalf("duplicate rows became multiple goals: %+v", index)
	}
	all = append(all, reservationGoal("different", a.ID, "EUR", 1))
	index = BuildAccountReservations(all, []domain.Account{a}, balances, currency.Rates{Base: "USD"})
	warnings := index.Warnings(2)
	if len(warnings) != 1 || warnings[0].Known || len(warnings[0].GoalRefs) != 2 {
		t.Fatalf("unknown shared position disappeared: %+v", warnings)
	}
}

func TestAccountReservationsCheckForeignRangeAndRates(t *testing.T) {
	a := domain.Account{ID: "bank", Currency: "USD"}
	for _, tc := range []struct {
		name    string
		rate    float64
		amounts []int64
		cause   error
	}{
		{"foreign sum overflow", 1, []int64{math.MaxInt64, 1}, money.ErrOverflow},
		{"foreign conversion overflow", 2, []int64{math.MaxInt64}, money.ErrOverflow},
		{"invalid FX", math.NaN(), []int64{10}, currency.ErrInvalidRate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := BuildAccountReservations([]domain.Goal{reservationGoal("g", a.ID, "EUR", tc.amounts...)}, []domain.Account{a}, map[string]money.Money{a.ID: money.New(100, "USD")}, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": tc.rate}})
			p := index.ByAccount[a.ID]
			if p.Known || p.ReservedKnown || !index.Incomplete || !errors.Is(p.Err, tc.cause) {
				t.Fatalf("invalid FX/range became known: %+v", p)
			}
		})
	}
}
