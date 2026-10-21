// SPDX-License-Identifier: MIT

package goalview

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAllocationDraftPreservesForeignRecordsAndUsesNativeHeadroom(t *testing.T) {
	a := fundingAccount("bank", "USD", 10000)
	g := goal("g", "EUR", 0, 10000)
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}
	other := goal("other", "USD", 0, 10000)
	other.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	d := BuildAllocationDraft(g, []domain.Goal{g, other}, []domain.Account{a}, map[string]money.Money{a.ID: a.OpeningBalance}, rates)
	r, _ := d.Row(a.ID)
	if !r.AmountKnown || r.Amount.Amount != 500 || !r.CanEdit || r.Available.Amount != 4500 || d.Free.Amount != 4500 {
		t.Fatalf("foreign row/headroom: %+v draft %+v", r, d)
	}
	before := append([]domain.GoalAllocation(nil), g.Allocations...)
	for _, changes := range []map[string]AllocationChange{nil, {a.ID: {Selected: true}}} {
		got, err := d.Apply(changes)
		if err != nil || !reflect.DeepEqual(got, before) {
			t.Fatalf("untouched record changed: %+v %v", got, err)
		}
	}
	got, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "45.00"}})
	if err != nil || len(got) != 1 || got[0].Amount != money.New(4500, "EUR") {
		t.Fatalf("explicit edit: %+v %v", got, err)
	}
	if !reflect.DeepEqual(g.Allocations, before) {
		t.Fatal("raw goal modified")
	}
	if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "45.01"}}); !errors.Is(err, ErrAllocationHeadroom) {
		t.Fatalf("over headroom accepted: %v", err)
	}
}

func TestAllocationDraftUnavailableOriginalSourcesCanStayOrBeRemoved(t *testing.T) {
	archived := fundingAccount("archived", "USD", 10000)
	archived.Archived = true
	liability := fundingAccount("debt", "USD", 10000)
	liability.Class = domain.ClassLiability
	g := goal("g", "EUR", 0, 10000)
	g.Allocations = []domain.GoalAllocation{
		{AccountID: "missing", Amount: money.New(1000, "GBP")},
		{AccountID: archived.ID, Amount: money.New(2000, "USD")},
		{AccountID: liability.ID, Amount: money.New(3000, "EUR")},
		{AccountID: "missing", Amount: money.New(1, "USD")},
	}
	d := BuildAllocationDraft(g, []domain.Goal{g}, []domain.Account{archived, liability}, nil, currency.Rates{Base: "USD"})
	got, err := d.Apply(nil)
	if err != nil || !reflect.DeepEqual(got, g.Allocations) {
		t.Fatalf("unknown original changed: %+v %v", got, err)
	}
	if len(d.Rows) != 3 || !d.Incomplete || d.Preview(nil, time.Now()).Err == nil {
		t.Fatalf("unavailable legacy records hidden: %+v", d)
	}
	for _, r := range d.Rows {
		if r.CanEdit || !r.Selected {
			t.Fatalf("unavailable source editable: %+v", r)
		}
	}
	got, err = d.Apply(map[string]AllocationChange{"missing": {Selected: false}})
	if err != nil || len(got) != 2 || got[0].AccountID != archived.ID {
		t.Fatalf("remove unavailable: %+v %v", got, err)
	}
	if _, err := d.Apply(map[string]AllocationChange{archived.ID: {Selected: true, AmountChanged: true, AmountText: "1.00"}}); !errors.Is(err, ErrAllocationUnavailable) {
		t.Fatalf("archived source can be increased: %v", err)
	}
}

func TestAllocationDraftUnknownFXAndInvalidBalancesNeverBecomeParity(t *testing.T) {
	a := fundingAccount("bank", "USD", 10000)
	g := goal("g", "EUR", 0, 10000)
	for _, tc := range []struct {
		name     string
		balances map[string]money.Money
		other    []domain.Goal
	}{
		{"missing-rate", map[string]money.Money{a.ID: a.OpeningBalance}, nil},
		{"invalid-balance", map[string]money.Money{}, nil},
		{"unknown-reservation", map[string]money.Money{a.ID: a.OpeningBalance}, []domain.Goal{{ID: "other", Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(10, "GBP")}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := BuildAllocationDraft(g, tc.other, []domain.Account{a}, tc.balances, currency.Rates{Base: "USD"})
			r, _ := d.Row(a.ID)
			if r.CanEdit || r.AvailableKnown || !d.Incomplete {
				t.Fatalf("unavailable source got headroom: %+v", r)
			}
			if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "1.00"}}); !errors.Is(err, ErrAllocationUnavailable) {
				t.Fatalf("unknown FX edit allowed: %v", err)
			}
		})
	}
}

func TestAllocationDraftChecksAggregateRoundingCarry(t *testing.T) {
	a := fundingAccount("bank", "USD", 1)
	g := goal("g", "EUR", 0, 100)
	other := domain.Goal{ID: "other", Allocations: []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1, "EUR")}}}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.4}}
	d := BuildAllocationDraft(g, []domain.Goal{other}, []domain.Account{a}, map[string]money.Money{a.ID: a.OpeningBalance}, rates)
	r, _ := d.Row(a.ID)
	if !r.AvailableKnown || r.Available.Amount != 2 {
		t.Fatalf("rounding carry cap: %+v", r)
	}
	if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "0.03"}}); !errors.Is(err, ErrAllocationHeadroom) {
		t.Fatalf("unsafe rounding allowed: %v", err)
	}
	if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "0.02"}}); err != nil {
		t.Fatal(err)
	}
}

func TestAllocationDraftRebuildRevalidatesAndKeepsLatestUneditedData(t *testing.T) {
	a := fundingAccount("bank", "USD", 10000)
	g := goal("g", "USD", 0, 10000)
	g.Allocations = []domain.GoalAllocation{{AccountID: a.ID, Amount: money.New(1000, "USD")}}
	changes := map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "75.00"}}
	d := BuildAllocationDraft(g, nil, []domain.Account{a}, map[string]money.Money{a.ID: a.OpeningBalance}, currency.Rates{})
	if _, err := d.Apply(changes); err != nil {
		t.Fatal(err)
	}
	d = BuildAllocationDraft(g, nil, []domain.Account{a}, map[string]money.Money{a.ID: money.New(5000, "USD")}, currency.Rates{})
	if _, err := d.Apply(changes); !errors.Is(err, ErrAllocationHeadroom) {
		t.Fatalf("stale headroom accepted: %v", err)
	}
	g.Allocations[0].Amount = money.New(1300, "USD")
	d = BuildAllocationDraft(g, nil, []domain.Account{a}, map[string]money.Money{a.ID: a.OpeningBalance}, currency.Rates{})
	got, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true}})
	if err != nil || got[0].Amount.Amount != 1300 {
		t.Fatalf("unedited latest amount replaced by stale seed: %+v %v", got, err)
	}
}

func TestAllocationDraftChecksTotalsAndTypedInputs(t *testing.T) {
	a, b := fundingAccount("a", "USD", math.MaxInt64), fundingAccount("b", "USD", math.MaxInt64)
	g := goal("g", "USD", 0, 10000)
	d := BuildAllocationDraft(g, nil, []domain.Account{a, b}, map[string]money.Money{a.ID: a.OpeningBalance, b.ID: b.OpeningBalance}, currency.Rates{})
	if !errors.Is(d.FreeErr, money.ErrOverflow) || !d.PrefillKnown || d.Prefill.Amount != 10000 {
		t.Fatalf("overflow total fabricated: %+v", d)
	}
	for _, value := range []string{"", "-1.00", "nope", "0"} {
		if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: value}}); !errors.Is(err, ErrAllocationAmount) {
			t.Fatalf("invalid selected amount %q: %v", value, err)
		}
	}
	if _, err := d.Apply(map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "92233720368547758.08"}}); !errors.Is(err, money.ErrOverflow) || !errors.Is(err, ErrAllocationAmount) {
		t.Fatalf("typed range error lost cause: %v", err)
	}
	changes := map[string]AllocationChange{a.ID: {Selected: true, AmountChanged: true, AmountText: "92233720368547758.07"}, b.ID: {Selected: true, AmountChanged: true, AmountText: "92233720368547758.07"}}
	if _, err := d.Apply(changes); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("draft total overflow accepted: %v", err)
	}
}

func TestAllocationDraftSplitUsesLiquidDefaultsAndExplicitHeldSources(t *testing.T) {
	cash, held := fundingAccount("cash", "USD", 10000), fundingAccount("held", "USD", 30000)
	held.Type = domain.TypeInvestment
	g := goal("g", "USD", 0, 40000)
	d := BuildAllocationDraft(g, nil, []domain.Account{held, cash}, map[string]money.Money{cash.ID: cash.OpeningBalance, held.ID: held.OpeningBalance}, currency.Rates{})
	changes, err := d.Split("400.00", nil, goals.SplitProportional)
	if err != nil || !changes[cash.ID].Selected || changes[held.ID].Selected || changes[cash.ID].AmountText != "100.00" {
		t.Fatalf("automatic split reserved held asset: %+v %v", changes, err)
	}
	changes, err = d.Split("200.00", map[string]AllocationChange{held.ID: {Selected: true}}, goals.SplitEven)
	if err != nil || !changes[held.ID].Selected || changes[held.ID].AmountText != "200.00" || changes[cash.ID].Selected {
		t.Fatalf("explicit held source ignored: %+v %v", changes, err)
	}
	if _, err := d.Apply(map[string]AllocationChange{cash.ID: {Selected: true, AmountChanged: true, AmountText: "1.00", Currency: "EUR"}}); !errors.Is(err, ErrAllocationCurrency) {
		t.Fatalf("stale currency edit relabeled: %v", err)
	}
}
