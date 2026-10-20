// SPDX-License-Identifier: MIT

package dashboardview

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/nwchange"
)

func TestSummarizeBudgetsUsesEffectiveLimitsAndLivePace(t *testing.T) {
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	makeStatus := func(id, name string, pct int) budgeting.Status {
		return budgeting.Status{Budget: domain.Budget{ID: id, Name: name, CategoryID: "food", Period: domain.PeriodMonthly, Limit: money.New(10000, "USD")},
			Start: start, Spent: money.New(int64(pct)*200, "USD"), Limit: money.New(20000, "USD"), Percent: pct}
	}
	rows := []budgeting.Status{makeStatus("calm", "Calm", 80), makeStatus("hot", "Hot", 40), makeStatus("early", "Early", 99), makeStatus("named", "", 50)}
	for _, kind := range []string{"weekly", "saving", "failed", "zero"} {
		st := makeStatus(kind, kind, 999)
		switch kind {
		case "weekly":
			st.Budget.Period = domain.PeriodWeekly
		case "saving":
			st.Budget.Direction = domain.DirectionSave
		case "failed":
			st.Error = &budgeting.BudgetEvaluationError{BudgetID: kind, Err: errors.New("missing rate")}
		case "zero":
			st.Limit.Amount = 0
		}
		rows = append(rows, st)
	}
	got := SummarizeBudgets(rows, map[string]string{"food": "Groceries"}, map[string]bool{"calm": false, "hot": true, "named": false})
	if got.Total != 4 || got.Measured != 3 || got.OnPace != 2 || !got.Period.Equal(start) {
		t.Fatalf("snapshot = %+v", got)
	}
	if len(got.Limits) != 3 || got.Limits[0].Name != "Hot" || got.Limits[1].Name != "Early" || got.Limits[2].Name != "Calm" {
		t.Fatalf("ranking = %+v", got.Limits)
	}
	if got.Limits[0].Limit.Amount != 20000 || rows[0].Budget.Limit.Amount != 10000 {
		t.Fatal("effective cap was lost or source status was mutated")
	}
	unnamed := SummarizeBudgets(rows[3:4], map[string]string{"food": "Groceries"}, nil)
	if unnamed.Limits[0].Name != "Groceries" || unnamed.OnPace != 0 {
		t.Fatalf("unnamed/unmeasured = %+v", unnamed)
	}
}

func TestBuildNetWorthTrendMatchesCanonicalMovementAndSpark(t *testing.T) {
	now := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.FixedZone("local", -4*60*60))
	accounts := []domain.Account{
		{ID: "usd", Currency: "USD", OpeningBalance: money.New(200000, "USD")},
		{ID: "eur", Currency: "EUR", OpeningBalance: money.New(-100000, "EUR")},
	}
	txns := []domain.Transaction{
		{AccountID: "usd", Date: time.Date(2026, time.October, 20, 20, 0, 0, 0, time.UTC), Amount: money.New(5000, "USD")},
		{AccountID: "usd", Date: time.Date(2026, time.October, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(7000, "USD")},
		{AccountID: "eur", Date: time.Date(2026, time.September, 2, 0, 0, 0, 0, time.UTC), Amount: money.New(10000, "EUR")},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	got := BuildNetWorthTrend(accounts, txns, rates, now)
	want, err := nwchange.MonthToDateChange(accounts, txns, rates, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Change, want) || len(got.Spark) != 7 {
		t.Fatalf("trend = %+v, canonical change %+v", got, want)
	}
	current, err := ledger.NetWorthSeries(accounts, txns, []time.Time{now}, rates)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spark[6] != float64(current[0].Amount) || got.Change.DeltaMinor() != 12000 {
		t.Fatalf("instant spark / end-of-day delta = %+v", got)
	}
	// A narrower account scope changes both outputs together.
	scoped := BuildNetWorthTrend(accounts[:1], txns, rates, now)
	scopedWant, _ := nwchange.MonthToDateChange(accounts[:1], txns, rates, now)
	if !reflect.DeepEqual(scoped.Change, scopedWant) || scoped.Spark[6] == got.Spark[6] {
		t.Fatalf("scope inconsistent: %+v", scoped)
	}
}

func TestBuildNetWorthTrendUnavailableOnMalformedBalance(t *testing.T) {
	got := BuildNetWorthTrend([]domain.Account{{ID: "bad", Currency: "USD", OpeningBalance: money.New(100, "EUR")}}, nil, currency.Rates{Base: "USD"}, time.Now())
	if got.Change.Known || len(got.Spark) != 0 {
		t.Fatalf("invalid balance produced a confident trend: %+v", got)
	}
}
