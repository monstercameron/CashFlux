// SPDX-License-Identifier: MIT

package budgeting

import (
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func bcs2CoverInput() ViewInput {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	return ViewInput{Base: "USD", Rates: currency.Rates{Base: "USD"}, Period: ViewPeriod{Anchor: now, Now: now, WeekStart: time.Monday}}
}

func bcs2CoverBudget(id string, amount int64) domain.Budget {
	return domain.Budget{ID: id, Name: id, CategoryID: id, Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID, Period: domain.PeriodMonthly, Limit: money.New(amount, "USD")}
}

func TestBCS2PlanCoverUsesEffectiveCapAndProtectsBills(t *testing.T) {
	in := bcs2CoverInput()
	from, to := bcs2CoverBudget("from", 30000), bcs2CoverBudget("to", 10000)
	from.Rollover, from.RolloverStart = true, in.Period.Anchor.AddDate(0, -1, 0)
	in.Transactions = []domain.Transaction{
		{ID: "last", CategoryID: "from", Date: in.Period.Anchor.AddDate(0, -1, 0), Amount: money.New(-10000, "USD")},
		{ID: "this", CategoryID: "from", Date: in.Period.Anchor, Amount: money.New(-5000, "USD")},
	}
	funds, err := CoverSourceFunds(from, in)
	if err != nil {
		t.Fatal(err)
	}
	if funds.Limit.Amount != 50000 || funds.Free.Amount != 45000 {
		t.Fatalf("funds=%+v", funds)
	}
	before, _ := json.Marshal([]domain.Budget{from, to})
	writes, err := PlanCover([]domain.Budget{from, to}, []CoverRequest{{DestinationID: "to", Shares: map[string]money.Money{"from": money.New(40000, "USD")}}}, in)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal([]domain.Budget{from, to})
	if string(before) != string(after) {
		t.Fatal("planner mutated its input")
	}
	if writes[0].PeriodBoosts["2026-10-01"] != -40000 || writes[1].PeriodBoosts["2026-10-01"] != 40000 {
		t.Fatalf("writes=%+v", writes)
	}
	for _, b := range writes {
		if len(b.PeriodTopups) != 0 {
			t.Fatal("cover became copyable")
		}
	}
	in.Recurring = []domain.Recurring{{ID: "bill", CategoryID: "from", Amount: money.New(-10000, "USD"), Cadence: domain.CadenceMonthly, NextDue: in.Period.Anchor.AddDate(0, 0, 2)}}
	if _, err := PlanCover([]domain.Budget{from, to}, []CoverRequest{{DestinationID: "to", Shares: map[string]money.Money{"from": money.New(40000, "USD")}}}, in); !errors.Is(err, ErrInsufficientSource) {
		t.Fatalf("bill money moved: %v", err)
	}
}

func TestBCS2PlanCoverRoomAndAtomicFailure(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		amount                    int64
		saving, permanent, second bool
		wantErr                   error
	}{
		{"spend remainder may be exhausted", 5000, false, false, false, nil},
		{"cap keeps one cent", 10000, false, false, false, ErrInsufficientSource},
		{"saving source", 100, true, false, false, ErrInsufficientSource},
		{"permanent keeps positive base", 10000, false, true, false, ErrInsufficientSource},
		{"aggregate source exhaustion", 3000, false, false, true, ErrInsufficientSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := bcs2CoverInput()
			from, to, other := bcs2CoverBudget("from", 10000), bcs2CoverBudget("to", 10000), bcs2CoverBudget("other", 10000)
			if tc.saving {
				from.Direction = domain.DirectionSave
			}
			in.Transactions = []domain.Transaction{{ID: "spent", CategoryID: "from", Date: in.Period.Anchor, Amount: money.New(-5000, "USD")}}
			requests := []CoverRequest{{DestinationID: "to", Shares: map[string]money.Money{"from": money.New(tc.amount, "USD")}, Permanent: tc.permanent}}
			if tc.second {
				requests = append(requests, CoverRequest{DestinationID: "other", Shares: map[string]money.Money{"from": money.New(tc.amount, "USD")}})
			}
			got, err := PlanCover([]domain.Budget{from, to, other}, requests, in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if err != nil && got != nil {
				t.Fatal("partial plan escaped")
			}
		})
	}
}

func TestBCS2CoverSplitLargeWeightsAndTotals(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		shares, short := CoverSplit(math.MaxInt64, []CoverSplitInput{{ID: "a", Cap: math.MaxInt64 / 2, Weight: math.MaxInt, Pinned: pinned}, {ID: "b", Cap: math.MaxInt64, Weight: math.MaxInt}})
		if short != 0 || shares["a"] != math.MaxInt64/2 || shares["b"] != math.MaxInt64/2+1 {
			t.Fatalf("pinned=%v shares=%v short=%d", pinned, shares, short)
		}
	}
	if got, short := CoverSplit(100, []CoverSplitInput{{ID: "same", Cap: 100}, {ID: "same", Cap: 100}}); got != nil || short != 100 {
		t.Fatal("duplicate source accepted")
	}
	if got := SourceFundsOf(money.New(math.MinInt64, "USD"), money.New(100, "USD"), money.Zero("USD")); got.Movable.Amount != 0 {
		t.Fatal("negative cap wrapped")
	}
}

func TestBCS2PlanCoverPermanentUsesViewedHistory(t *testing.T) {
	in := bcs2CoverInput()
	in.Period.Anchor = in.Period.Anchor.AddDate(0, -2, 0)
	from, to := bcs2CoverBudget("from", 60000), bcs2CoverBudget("to", 20000)
	from.LimitHistory = map[string]int64{"2026-01-01": 30000, "2026-10-01": 60000}
	to.LimitHistory = map[string]int64{"2026-01-01": 10000, "2026-10-01": 20000}
	writes, err := PlanCover([]domain.Budget{from, to}, []CoverRequest{{DestinationID: "to", Permanent: true, Shares: map[string]money.Money{"from": money.New(5000, "USD")}}}, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 2 || writes[0].Limit.Amount != 25000 || writes[1].Limit.Amount != 15000 {
		t.Fatalf("writes=%+v", writes)
	}
	if len(writes[0].PeriodBoosts) != 0 || len(writes[1].PeriodBoosts) != 0 {
		t.Fatal("standing move wrote a period boost")
	}
	if from.LimitHistory["2026-08-01"] != 0 {
		t.Fatal("input history mutated")
	}
}

func TestBCS2PlanCoverBorrowAndDestinationOverflow(t *testing.T) {
	in := bcs2CoverInput()
	to := bcs2CoverBudget("to", 10000)
	writes, err := PlanCover([]domain.Budget{to}, []CoverRequest{{DestinationID: "to", BorrowNext: money.New(3000, "USD")}}, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || writes[0].PeriodBoosts["2026-10-01"] != 3000 || writes[0].PeriodBoosts["2026-11-01"] != -3000 || len(writes[0].PeriodTopups) != 0 {
		t.Fatalf("borrow=%+v", writes)
	}
	to.Rollover = true
	if _, err := PlanCover([]domain.Budget{to}, []CoverRequest{{DestinationID: "to", BorrowNext: money.New(3000, "USD")}}, in); err == nil {
		t.Fatal("rollover borrowed from future carry")
	}
	from := bcs2CoverBudget("from", 10000)
	to = bcs2CoverBudget("to", math.MaxInt64)
	if _, err := PlanCover([]domain.Budget{from, to}, []CoverRequest{{DestinationID: "to", Shares: map[string]money.Money{"from": money.New(100, "USD")}}}, in); err == nil {
		t.Fatal("effective destination cap overflow accepted")
	}
}
