// SPDX-License-Identifier: MIT

package engineenv

// This file exposes the financial-health model as engine variables: each of the
// six factor scores (0–100) and its EXACT post-renormalization weight as atoms,
// plus the deficit penalty and two informative raw values. The overall score is
// deliberately NOT an atom — it is the health_score MOLECULE in DefaultMolecules,
// a real formula over these atoms (round(Σ score×weight) − penalty, clamped) —
// so the headline number is auditable via Explain, referenceable in any formula
// or dashboard widget, and even re-weightable by editing the molecule. The
// inputs derivation (HealthInputs) is the single source shared by the /health
// screen, the dashboard tile, and these variables, so they can never disagree.

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/periodage"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// HealthVarNames are the fixed health atoms addHealthVars exposes, in a stable
// order: a score + weight pair per factor, the penalty, and two raw values.
var HealthVarNames = []string{
	"health_savings", "health_savings_weight",
	"health_emergency", "health_emergency_weight",
	"health_debt", "health_debt_weight",
	"health_budget", "health_budget_weight",
	"health_utilization", "health_utilization_weight",
	"health_trend", "health_trend_weight",
	"health_penalty",          // flat deduction when spending exceeds income (else 0)
	"health_emergency_months", // liquid cash ÷ average monthly spending
	"health_obligation_pct",   // Σ liability minimum payments ÷ monthly income (%)
}

func init() { Names = append(Names, HealthVarNames...) }

// healthLookbackMonths is the trailing window used to derive savings rate,
// average monthly spending, and monthly income — three full months, excluding
// the current partial month so a mid-month dip doesn't distort the score.
const healthLookbackMonths = 3

// healthNWTrendMonths is the net-worth-trend factor's trailing window.
const healthNWTrendMonths = 6

// healthFactorVar maps a healthscore factor key to its atom base name.
func healthFactorVar(key string) string {
	switch key {
	case "nw-trend":
		return "health_trend"
	default:
		return "health_" + key
	}
}

// HealthInputs derives the financial-health signals from the fundamental Data —
// the pure port of the /health screen's former inline assembly, reusing the same
// tested ledger/reports/budgeting primitives. Every factor carries an
// applicability flag so the model can drop what doesn't apply (e.g. no cards)
// and re-normalize, rather than penalizing a household for something it lacks.
func HealthInputs(d Data) healthscore.Inputs {
	if d.Ledger == nil {
		d.Ledger = d.budgetLedger()
	}
	bals, _ := ledger.Balances(d.Accounts, d.Transactions)
	return healthInputs(d, bals)
}

// healthInputs is HealthInputs over a precomputed balance map, so Vars() can
// share its single-pass Balances result instead of re-scanning the ledger.
func healthInputs(d Data, bals map[string]money.Money) healthscore.Inputs {
	rates := d.Rates
	if rates.Base == "" {
		rates.Base = "USD"
	}
	base := rates.Base
	in := healthscore.Inputs{Unavailable: map[string]error{}}
	fail := func(factor string, err error) {
		if in.Unavailable[factor] == nil {
			in.Unavailable[factor] = fmt.Errorf("health %s: %w", factor, err)
		}
	}
	curMonth := dateutil.MonthStart(d.Now)
	start := dateutil.AddMonths(curMonth, -healthLookbackMonths)
	flows, flowErr := reports.IncomeExpenseSeriesFromLedger(d.budgetLedger(), []time.Time{start, curMonth}, rates)
	monthlyIncome, avgMonthlySpend := int64(0), int64(0)
	if flowErr != nil {
		for _, key := range []string{"savings", "emergency", "debt"} {
			fail(key, flowErr)
		}
	} else if len(flows) == 1 {
		flow := flows[0]
		monthlyIncome = flow.Income / healthLookbackMonths
		avgMonthlySpend = flow.Expense / healthLookbackMonths
		if flow.Income > 0 {
			in.HasIncome = true
			net, err := money.Sub(flow.Income, flow.Expense)
			if err == nil {
				in.SavingsRatePct, err = healthPercent(net, flow.Income, money.RoundTowardZero)
			}
			if err != nil {
				fail("savings", err)
			}
		}
	}
	if avgMonthlySpend > 0 {
		liquid, err := ledger.LiquidFromBalances(d.Accounts, bals, rates)
		if err != nil {
			fail("emergency", err)
		} else {
			in.HasLiquidData = true
			in.EmergencyMonths = float64(liquid.Amount) / float64(avgMonthlySpend)
		}
	}
	if in.HasIncome {
		var minSum int64
		for _, a := range d.Accounts {
			if a.Archived || !a.IsLiability() {
				continue
			}
			in.HasLiabilities = true
			if a.MinPayment.Amount == 0 {
				continue
			}
			converted, err := currency.ConvertBetween(a.MinPayment.Amount, a.MinPayment.Currency, base, rates)
			if err == nil {
				converted, err = money.Abs(converted)
			}
			if err == nil {
				minSum, err = money.Add(minSum, converted)
			}
			if err != nil {
				fail("debt", fmt.Errorf("account %s minimum payment: %w", a.ID, err))
				break
			}
		}
		if in.Unavailable["debt"] == nil {
			if monthlyIncome <= 0 {
				fail("debt", fmt.Errorf("monthly income rounds to zero"))
			} else {
				var err error
				in.ObligationRatioPct, err = healthPercent(minSum, monthlyIncome, money.RoundTowardZero)
				if err != nil {
					fail("debt", err)
				}
			}
		}
	}
	if len(d.Budgets) > 0 {
		in.BudgetAdherenceWindow = healthscore.WindowCurrentPeriod
		measureAt := d.Now
		ms, me := budgeting.PeriodRange(domain.PeriodMonthly, d.Now, d.WeekStart)
		if age := periodage.Of(ms, me, d.Now); age.Early() {
			measureAt = age.Start.Add(-time.Nanosecond)
			in.BudgetAdherenceWindow = healthscore.WindowPriorPeriod
		}
		within := int64(0)
		for _, b := range d.Budgets {
			bs, be := budgeting.PeriodRange(b.Period, measureAt, d.WeekStart)
			status, err := budgeting.EvaluateRollup(b, d.budgetLedger(), bs, be, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(d.Categories, b.TrackedCategoryIDs()))
			if err != nil {
				fail("budget", fmt.Errorf("budget %s: %w", b.ID, err))
				break
			}
			if status.State != budgeting.StateOver {
				within++
			}
		}
		if in.Unavailable["budget"] == nil {
			var err error
			in.BudgetAdherencePct, err = healthPercent(within, int64(len(d.Budgets)), money.RoundTowardZero)
			if err != nil {
				fail("budget", err)
			} else {
				in.HasBudgets = true
			}
		}
	}
	var balSum, limitSum int64
	for _, a := range d.Accounts {
		if a.Archived || a.CreditLimit.Amount <= 0 {
			continue
		}
		bal, ok := bals[a.ID]
		if !ok {
			fail("utilization", fmt.Errorf("account %s balance unavailable", a.ID))
			break
		}
		owed, err := money.Abs(bal.Amount)
		if err != nil {
			fail("utilization", fmt.Errorf("account %s balance magnitude: %w", a.ID, err))
			break
		}
		ob, err := currency.ConvertBetween(owed, bal.Currency, base, rates)
		if err == nil {
			balSum, err = money.Add(balSum, ob)
		}
		if err == nil {
			var ol int64
			ol, err = currency.ConvertBetween(a.CreditLimit.Amount, a.CreditLimit.Currency, base, rates)
			if err == nil {
				limitSum, err = money.Add(limitSum, ol)
			}
		}
		if err != nil {
			fail("utilization", fmt.Errorf("account %s credit: %w", a.ID, err))
			break
		}
	}
	if in.Unavailable["utilization"] == nil && limitSum > 0 {
		var err error
		in.AggUtilizationPct, err = healthPercent(balSum, limitSum, money.RoundHalfAwayFromZero)
		if err != nil {
			fail("utilization", err)
		} else {
			in.HasCredit = true
		}
	}
	nwStart := dateutil.AddMonths(curMonth, -healthNWTrendMonths)
	series, err := ledger.NetWorthSeries(d.Accounts, d.Transactions, []time.Time{nwStart, d.Now}, rates)
	if err != nil {
		fail("nw-trend", err)
	} else if len(series) == 2 && series[0].Amount > 0 {
		in.NWTrendPct, err = healthTrendPercent(series[0].Amount, series[1].Amount)
		if err != nil {
			fail("nw-trend", err)
		} else {
			in.HasNWTrend = true
		}
	}
	return in
}

func healthPercent(numerator, denominator int64, rounding money.RoundingMode) (int, error) {
	wide, err := money.MulDiv(numerator, 100, denominator, rounding)
	if err != nil {
		return 0, err
	}
	value := int(wide)
	if int64(value) != wide {
		return 0, fmt.Errorf("health percentage: %w", money.ErrOverflow)
	}
	return value, nil
}

func healthTrendPercent(start, end int64) (float64, error) {
	if start <= 0 {
		return 0, fmt.Errorf("net-worth trend requires a positive baseline")
	}
	// Subtract in a wide rational: two valid int64 positions can have a change
	// outside int64, while their percentage remains a small, valid number.
	delta := new(big.Rat).Sub(new(big.Rat).SetInt64(end), new(big.Rat).SetInt64(start))
	delta.Mul(delta, big.NewRat(100, start))
	pct, _ := delta.Float64()
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return 0, fmt.Errorf("net-worth trend is not finite")
	}
	return pct, nil
}

// addHealthVars runs the health model over the shared HealthInputs derivation
// and exposes each factor's score + exact weight (zero when inapplicable or in
// the not-enough-data case), the deficit penalty, and the two raw values. The
// health_score molecule then reproduces healthscore.Evaluate's headline exactly
// (guarded by healthscore.TestWeightFormulaIdentity and this package's tests).
func addHealthVars(out map[string]float64, d Data, bals map[string]money.Money) {
	for _, name := range HealthVarNames {
		out[name] = 0
	}
	in := healthInputs(d, bals)
	r := healthscore.Evaluate(in)
	for _, f := range r.Factors {
		name := healthFactorVar(f.Key)
		out[name] = float64(f.Score)
		out[name+"_weight"] = f.Weight
	}
	if r.NegativeCashFlow {
		out["health_penalty"] = healthscore.NegativeCashFlowPenalty
	}
	out["health_emergency_months"] = in.EmergencyMonths
	out["health_obligation_pct"] = float64(in.ObligationRatioPct)
	for key := range in.Unavailable {
		name := healthFactorVar(key)
		delete(out, name)
		delete(out, name+"_weight")
		switch key {
		case "savings":
			delete(out, "health_penalty")
		case "emergency":
			delete(out, "health_emergency_months")
		case "debt":
			delete(out, "health_obligation_pct")
		}
	}
}
