// SPDX-License-Identifier: MIT

package dashboardview

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestCashForecastUsesEligibleScopeAndConvertsEveryHorizon(t *testing.T) {
	now := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "usd", Type: domain.TypeChecking, Currency: "USD"},
		{ID: "eur", Type: domain.TypeSavings, Currency: "EUR"},
		{ID: "no-rate", Type: domain.TypeCash, Currency: "GBP"},
		{ID: "archived", Type: domain.TypeChecking, Archived: true},
		{ID: "debt", Type: domain.TypeChecking, Class: domain.ClassLiability},
		{ID: "locked", Type: domain.TypeSavings, LockUntil: now.AddDate(0, 1, 0)},
		{ID: "investment", Type: domain.TypeInvestment},
	}
	var calls []string
	project := func(id string, asOf time.Time, horizon int) (acctproject.Projection, error) {
		if !asOf.Equal(now) {
			t.Fatalf("implicit clock used: %s", asOf)
		}
		calls = append(calls, fmt.Sprintf("%s/%d", id, horizon))
		return acctproject.Projection{Start: 10000, End: 10000 - int64(horizon)*100}, nil
	}
	got := BuildCashForecast(accounts, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, now, project)
	if got.Today != 30000 || got.Totals != [3]int64{21000, 12000, 3000} || got.Counted != 2 {
		t.Fatalf("forecast = %+v", got)
	}
	wantCalls := []string{"usd/30", "usd/60", "usd/90", "eur/30", "eur/60", "eur/90", "no-rate/30"}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("projection calls = %v, want %v", calls, wantCalls)
	}
	scoped := BuildCashForecast(accounts[:1], currency.Rates{Base: "USD"}, now, project)
	if scoped.Today != 10000 || scoped.Counted != 1 || scoped.Totals != [3]int64{7000, 4000, 1000} {
		t.Fatalf("scope leaked: %+v", scoped)
	}
}

func TestCashForecastRejectsOverflowingTotalsAndConvertedAmounts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		start, end int64
		code       string
		rate       float64
	}{
		{"today", math.MaxInt64, 0, "USD", 1},
		{"horizon", 0, math.MaxInt64, "USD", 1},
		{"negative horizon", 0, math.MinInt64, "USD", 1},
		{"converted start", math.MaxInt64, 0, "EUR", 2},
		{"converted end", 0, math.MaxInt64, "EUR", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			accounts := []domain.Account{{ID: "a", Currency: tc.code, Type: domain.TypeChecking}, {ID: "b", Currency: tc.code, Type: domain.TypeSavings}}
			got := BuildCashForecast(accounts, currency.Rates{Base: "USD", Rates: map[string]float64{tc.code: tc.rate}}, time.Now(), func(string, time.Time, int) (acctproject.Projection, error) {
				return acctproject.Projection{Start: tc.start, End: tc.end}, nil
			})
			if !errors.Is(got.Error, money.ErrOverflow) || got.Today != 0 || got.Totals != [3]int64{} || got.Counted != 0 {
				t.Fatalf("overflow exposed apparently valid forecast: %+v", got)
			}
		})
	}
}

func TestCashForecastPropagatesAccountProjectionErrors(t *testing.T) {
	want := errors.New("account projection unavailable")
	got := BuildCashForecast([]domain.Account{{ID: "checking", Currency: "USD", Type: domain.TypeChecking}}, currency.Rates{Base: "USD"}, time.Now(),
		func(string, time.Time, int) (acctproject.Projection, error) { return acctproject.Projection{}, want })
	if !errors.Is(got.Error, want) || got.Counted != 0 || got.Today != 0 || got.Totals != [3]int64{} {
		t.Fatalf("projection error became a plausible cash forecast: %+v", got)
	}
}
