// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAddReportsVars(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	ps := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	pe := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 10, 0, 0, 0, time.UTC) }

	d := Data{
		Accounts: []domain.Account{
			{ID: "a1", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
				OpeningBalance: money.New(600000, "USD"), BalanceAsOf: asOf}, // $6,000 liquid
		},
		Transactions: []domain.Transaction{
			// Current window (July): $4,000 in, spend $100 + $300 at two payees.
			{ID: "t1", AccountID: "a1", Date: day(time.July, 2), Amount: money.New(400000, "USD")},
			{ID: "t2", AccountID: "a1", Date: day(time.July, 5), Amount: money.New(-10000, "USD"), Desc: "Cafe"},
			{ID: "t3", AccountID: "a1", Date: day(time.July, 9), Amount: money.New(-30000, "USD"), Desc: "Grocer"},
			// Prior window (June): $2,000 in, spend $200. Also feeds the 6-month burn.
			{ID: "t4", AccountID: "a1", Date: day(time.June, 3), Amount: money.New(200000, "USD")},
			{ID: "t5", AccountID: "a1", Date: day(time.June, 10), Amount: money.New(-20000, "USD"), Desc: "Grocer"},
			// Earlier months of spending so the burn average is non-zero.
			{ID: "t6", AccountID: "a1", Date: day(time.May, 10), Amount: money.New(-40000, "USD"), Desc: "Rent"},
			{ID: "t7", AccountID: "a1", Date: day(time.April, 10), Amount: money.New(-40000, "USD"), Desc: "Rent"},
		},
		Rates: currency.Rates{Base: "USD"}, Now: now, PeriodStart: ps, PeriodEnd: pe,
	}
	v := Vars(d)

	// Prior-window figures (June): income $2,000, spend $200, net $1,800.
	if v["report_prev_income"] != 2000 {
		t.Errorf("report_prev_income = %v, want 2000", v["report_prev_income"])
	}
	if v["report_prev_spend"] != 200 {
		t.Errorf("report_prev_spend = %v, want 200", v["report_prev_spend"])
	}
	if v["report_prev_net"] != 1800 {
		t.Errorf("report_prev_net = %v, want 1800", v["report_prev_net"])
	}
	// Deltas: income doubled (+100%), spending doubled (+100%).
	if v["report_income_delta_pct"] != 100 {
		t.Errorf("report_income_delta_pct = %v, want 100", v["report_income_delta_pct"])
	}
	if v["report_spend_delta_pct"] != 100 {
		t.Errorf("report_spend_delta_pct = %v, want 100", v["report_spend_delta_pct"])
	}
	// Spending stats over July: two expenses $100 + $300 → avg $200, median $200.
	if v["report_avg_expense"] != 200 {
		t.Errorf("report_avg_expense = %v, want 200", v["report_avg_expense"])
	}
	if v["report_median_expense"] != 200 {
		t.Errorf("report_median_expense = %v, want 200", v["report_median_expense"])
	}
	// 14 elapsed days (Jul 1–14 complete before "now" Jul 15 midday) minus 2 spend days ≥ 10.
	if v["report_no_spend_days"] < 10 {
		t.Errorf("report_no_spend_days = %v, want >= 10", v["report_no_spend_days"])
	}
	// Top payee: Grocer $300 of $400 total → 75%.
	if v["report_top_payee_spend"] != 300 {
		t.Errorf("report_top_payee_spend = %v, want 300", v["report_top_payee_spend"])
	}
	if v["report_top_payee_pct"] != 75 {
		t.Errorf("report_top_payee_pct = %v, want 75", v["report_top_payee_pct"])
	}
	// Burn: only the ACTIVE full months count (AverageMonthlyExpense skips
	// inactive buckets) — Apr $400 + May $400 + Jun $200 = $1,000 / 3 ≈ $333.33.
	if v["report_burn"] < 330 || v["report_burn"] > 336 {
		t.Errorf("report_burn = %v, want ~333.33", v["report_burn"])
	}
	// Runway: positive and finite at that burn with $6,000 opening + flows.
	if v["report_runway_months"] <= 0 {
		t.Errorf("report_runway_months = %v, want > 0", v["report_runway_months"])
	}
}

// TestAddReportsVarsEmpty asserts every report_* variable is always present —
// even over an empty dataset — so formulas referencing them never hit
// undefined-variable errors.
func TestAddReportsVarsEmpty(t *testing.T) {
	v := Vars(Data{Rates: currency.Rates{Base: "USD"}, Now: time.Now()})
	for _, k := range ReportsVarNames {
		if _, ok := v[k]; !ok {
			t.Errorf("%s should always be present", k)
		}
	}
}
