// SPDX-License-Identifier: MIT

package dashboardview

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
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
	project := func(id string, asOf time.Time, horizon int) acctproject.Projection {
		if !asOf.Equal(now) {
			t.Fatalf("implicit clock used: %s", asOf)
		}
		calls = append(calls, fmt.Sprintf("%s/%d", id, horizon))
		return acctproject.Projection{Start: 10000, End: 10000 - int64(horizon)*100}
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
