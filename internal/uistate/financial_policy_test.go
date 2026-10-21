// SPDX-License-Identifier: MIT

package uistate

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/inflation"
)

func TestFinancialPolicyParsersPreserveDefaultsAndStoredIntent(t *testing.T) {
	debt, err := ParseDebtConfig(`{"warnUtilizationPct":80,"highUtilizationPct":20,"defaultStrategy":"invalid","defaultExtraMinor":9223372036854775807}`)
	if err != nil || debt.HighUtilizationPct != 80 || debt.DefaultStrategy != "avalanche" || !debt.ExcludeMortgage || debt.DefaultExtraMinor != math.MaxInt64 || debt.UtilizationBand(80) != "high" {
		t.Fatal(debt, err)
	}
	sweep, err := ParseSweepConfig(`{"enabled":true,"budgetIds":["archived","archived"],"targetGoalId":"old-goal"}`)
	if err != nil || !reflect.DeepEqual(sweep.Domain().BudgetIDs, []string{"archived", "archived"}) || sweep.Domain().TargetGoalID != "old-goal" {
		t.Fatal(sweep, err)
	}
	roundup, err := ParseRoundUpConfig("")
	if err != nil || roundup.Enabled || roundup.Cadence != RoundUpCadenceWeekly {
		t.Fatal(roundup, err)
	}
	roundup, err = ParseRoundUpConfig(`{"enabled":true,"cadence":"legacy","accountIds":["removed"],"lastSweepStamp":"not-a-date"}`)
	fallback := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err != nil || roundup.Cadence != "legacy" || roundup.EffectiveCadence() != RoundUpCadenceWeekly || !roundup.ParticipatingSet()["removed"] || !roundup.SinceOr(fallback).Equal(fallback) {
		t.Fatal(roundup, err)
	}
	tax, err := ParseTaxPlan(`{"priorYearTaxMinor":9223372036854775807,"effectiveRatePct":0}`)
	if err != nil || tax.PriorYearTaxMinor != math.MaxInt64 || tax.EffectiveRatePct != 0 {
		t.Fatal("unset rate was invented or exact monetary integer lost", tax, err)
	}
	retirement, err := ParseRetirementPlan(`{"contribution":9223372036854775807,"inflationPct":99}`)
	if err != nil || retirement.AnnualContributionMinor != math.MaxInt64 || retirement.YearsToRetirement != 0 || retirement.ReturnPct != DefaultReturnPct || retirement.InflationPct != 0 {
		t.Fatal("invented horizon or duplicate inflation assumption", retirement, err)
	}
	if pct, err := ParseInflationPct(""); err != nil || pct != inflation.DefaultRatePct {
		t.Fatal(pct, err)
	}
	if pct, err := ParseInflationPct("4.125"); err != nil || pct != 4.125 {
		t.Fatal(pct, err)
	}
	routes, err := ParseNotifyRoutes(`[{"prefix":"same","route":"/first"},{"prefix":"same","route":"/second"}]`)
	if err != nil || len(routes) != 2 || routes[0].Route != "/first" || routes[1].Route != "/second" {
		t.Fatal("route precedence changed", routes, err)
	}
	if routes, err := ParseNotifyRoutes("[]"); err != nil || !reflect.DeepEqual(routes, defaultNotifyRoutes()) {
		t.Fatal(routes, err)
	}
}

func TestFinancialPolicyParsersRejectCorruptSourcesWithoutInventingDefaults(t *testing.T) {
	parsers := map[string]func(string) error{
		"debt":       func(s string) error { _, e := ParseDebtConfig(s); return e },
		"sweep":      func(s string) error { _, e := ParseSweepConfig(s); return e },
		"roundup":    func(s string) error { _, e := ParseRoundUpConfig(s); return e },
		"tax":        func(s string) error { _, e := ParseTaxPlan(s); return e },
		"retirement": func(s string) error { _, e := ParseRetirementPlan(s); return e },
	}
	for name, parse := range parsers {
		for _, raw := range []string{"{broken", " ", "null", "[]", "true", `"{}"`} {
			if err := parse(raw); err == nil {
				t.Errorf("%s accepted %q", name, raw)
			}
		}
	}
	for _, raw := range []string{"{broken", " ", "null", "{}", "true"} {
		if _, err := ParseNotifyRoutes(raw); err == nil {
			t.Errorf("routes accepted %q", raw)
		}
	}
	for _, raw := range []string{"NaN", "+Inf", "-100", "broken", " ", "null"} {
		if _, err := ParseInflationPct(raw); err == nil {
			t.Errorf("inflation accepted %q", raw)
		}
	}
}
