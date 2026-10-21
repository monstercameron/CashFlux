// SPDX-License-Identifier: MIT
package engineenv

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"testing"
	"time"
)

func reportCheckedData() Data {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	return Data{Rates: currency.Rates{Base: "USD"}, Now: now, PeriodStart: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
}
func reportCheckedTxn(id string, amount int64, code string, on time.Time) domain.Transaction {
	return domain.Transaction{ID: id, Amount: money.New(amount, code), Date: on, Desc: "Shop"}
}
func TestReportsVarsMinimumAmountDoesNotPanicOrBecomeZero(t *testing.T) {
	d := reportCheckedData()
	d.Transactions = []domain.Transaction{reportCheckedTxn("minimum", math.MinInt64, "USD", d.PeriodStart)}
	for _, name := range []string{"report_avg_expense", "report_median_expense", "report_top_payee_spend", "report_top_payee_pct", "report_spend_delta_pct"} {
		d.Molecules = append(d.Molecules, domain.Molecule{Name: name, Formula: "999"})
	}
	vars := Vars(d)
	for _, name := range []string{"report_avg_expense", "report_median_expense", "report_top_payee_spend", "report_top_payee_pct", "report_spend_delta_pct"} {
		if got, ok := vars[name]; ok {
			t.Errorf("unavailable %s fabricated as%v", name, got)
		}
	}
	if _, ok := vars["report_no_spend_days"]; !ok {
		t.Error("independent day count disappeared")
	}
	if got, ok := vars["report_prev_spend"]; !ok || got != 0 {
		t.Errorf("known empty prior lost: %v %v", got, ok)
	}
}
func TestReportsVarsUnknownFXByWindowIsUnavailableAndReserved(t *testing.T) {
	for _, window := range []string{"current", "prior", "burn"} {
		t.Run(window, func(t *testing.T) {
			d := reportCheckedData()
			when := d.PeriodStart
			omitted := []string{"report_avg_expense", "report_median_expense", "report_top_payee_spend", "report_top_payee_pct", "report_income_delta_pct", "report_spend_delta_pct"}
			if window == "prior" {
				when = when.AddDate(0, 0, -1)
				omitted = []string{"report_prev_income", "report_prev_spend", "report_prev_net", "report_income_delta_pct", "report_spend_delta_pct", "report_burn", "report_runway_months"}
			}
			if window == "burn" {
				when = when.AddDate(0, -2, 0)
				omitted = []string{"report_burn", "report_runway_months"}
			}
			d.Transactions = []domain.Transaction{reportCheckedTxn("unknown", -100, "EUR", when)}
			for _, name := range omitted {
				d.Molecules = append(d.Molecules, domain.Molecule{Name: name, Formula: "999"})
			}
			vars := Vars(d)
			for _, name := range omitted {
				if got, ok := vars[name]; ok {
					t.Errorf("%s unexpectedly%v", name, got)
				}
			}
			if _, ok := vars["report_no_spend_days"]; !ok {
				t.Error("independent count unavailable")
			}
		})
	}
}
func TestReportsVarsBurnOverflowAndLiquidErrorAreUnavailable(t *testing.T) {
	d := reportCheckedData()
	d.Transactions = []domain.Transaction{reportCheckedTxn("may", -math.MaxInt64, "USD", d.PeriodStart.AddDate(0, -2, 0)), reportCheckedTxn("june", -math.MaxInt64, "USD", d.PeriodStart.AddDate(0, -1, 0))}
	d.Molecules = []domain.Molecule{{Name: "report_burn", Formula: "999"}, {Name: "report_runway_months", Formula: "999"}}
	vars := Vars(d)
	for _, name := range []string{"report_burn", "report_runway_months"} {
		if got, ok := vars[name]; ok {
			t.Errorf("overflow %s=%v", name, got)
		}
	}
	// A known burn does not turn a failed liquid-balance calculation into zero runway.
	d = reportCheckedData()
	d.Transactions = []domain.Transaction{reportCheckedTxn("burn", -100, "USD", d.PeriodStart.AddDate(0, -1, 0))}
	out := map[string]float64{}
	addReportsVars(out, d, func(v int64) float64 { return float64(v) / 100 }, 0, errors.New("unknown liquid"))
	if out["report_burn"] != 1 {
		t.Errorf("known burn lost: %v", out)
	}
	if _, ok := out["report_runway_months"]; ok {
		t.Error("unknown liquid became zero runway")
	}
}
func TestReportsVarsWideSharesAndDeltaDoNotWrap(t *testing.T) {
	d := reportCheckedData()
	d.Transactions = []domain.Transaction{reportCheckedTxn("max", -math.MaxInt64, "USD", d.PeriodStart), reportCheckedTxn("prior", -1, "USD", d.PeriodStart.AddDate(0, 0, -1))}
	vars := Vars(d)
	if got, ok := vars["report_top_payee_pct"]; !ok || got != 100 {
		t.Errorf("wide share=%v present%v", got, ok)
	}
	if _, ok := vars["report_spend_delta_pct"]; ok {
		t.Error("unrepresentable percent should be omitted")
	}
	if got, ok := reportDeltaPct(math.MaxInt64/2, math.MaxInt64/4); !ok || got != 100 {
		t.Errorf("representable wide percent %v %v", got, ok)
	}
	if got, ok := vars["report_avg_expense"]; !ok || got != float64(math.MaxInt64)/100 {
		t.Errorf("valid maximum average=%v %v", got, ok)
	}
}
