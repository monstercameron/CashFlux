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
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/ledger"
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
	rates := d.Rates
	if rates.Base == "" {
		rates.Base = "USD"
	}
	base := rates.Base

	var in healthscore.Inputs

	// Trailing three full months (exclude the current partial month).
	curMonth := dateutil.MonthStart(d.Now)
	start := dateutil.AddMonths(curMonth, -healthLookbackMonths)
	flow, err := reports.IncomeVsExpense(d.Transactions, start, curMonth, rates)
	hasFlow := err == nil && (flow.Income > 0 || flow.Expense > 0)

	monthlyIncome := int64(0)
	avgMonthlySpend := int64(0)
	if hasFlow {
		monthlyIncome = flow.Income / healthLookbackMonths
		avgMonthlySpend = flow.Expense / healthLookbackMonths
		if flow.Income > 0 {
			in.HasIncome = true
			in.SavingsRatePct = ledger.SavingsRate(flow.Income, flow.Expense)
		}
	}

	// Emergency fund: liquid cash ÷ average monthly spending.
	if avgMonthlySpend > 0 {
		if liquid, lerr := ledger.LiquidBalance(d.Accounts, d.Transactions, rates); lerr == nil {
			in.HasLiquidData = true
			in.EmergencyMonths = float64(liquid.Amount) / float64(avgMonthlySpend)
		}
	}

	// Debt payments vs income: Σ liability minimum payments ÷ monthly income.
	// Applicable whenever there's income; zero debt scores 100 (in the model).
	if in.HasIncome {
		var minSum int64
		anyLiab := false
		for _, a := range d.Accounts {
			if a.Archived || !a.IsLiability() {
				continue
			}
			anyLiab = true
			conv, cerr := currency.ConvertBetween(a.MinPayment.Amount, a.MinPayment.Currency, base, rates)
			if cerr != nil {
				conv = a.MinPayment.Amount
			}
			minSum += conv
		}
		in.HasLiabilities = anyLiab
		if monthlyIncome > 0 {
			in.ObligationRatioPct = int(minSum * 100 / monthlyIncome)
		}
	}

	// Budget adherence: share of budgets within their limit this period. Mirrors
	// the dashboard's budget evaluation (rollup over sub-categories).
	if len(d.Budgets) > 0 {
		total, within := 0, 0
		for _, b := range d.Budgets {
			bs, be := budgeting.PeriodRange(b.Period, d.Now, d.WeekStart)
			st, berr := budgeting.EvaluateRollup(b, d.Transactions, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(d.Categories, b.TrackedCategoryIDs()))
			if berr != nil {
				continue
			}
			total++
			if st.State != budgeting.StateOver {
				within++
			}
		}
		if total > 0 {
			in.HasBudgets = true
			in.BudgetAdherencePct = within * 100 / total
		}
	}

	// Aggregate credit utilization: Σ card balances ÷ Σ card limits.
	var balSum, limitSum int64
	for _, a := range d.Accounts {
		if a.Archived || a.CreditLimit.Amount <= 0 {
			continue
		}
		bal, berr := ledger.Balance(a, d.Transactions)
		if berr != nil {
			continue
		}
		owed := bal.Amount
		if owed < 0 {
			owed = -owed
		}
		ob, cerr := currency.ConvertBetween(owed, bal.Currency, base, rates)
		if cerr != nil {
			ob = owed
		}
		ol, cerr := currency.ConvertBetween(a.CreditLimit.Amount, a.CreditLimit.Currency, base, rates)
		if cerr != nil {
			ol = a.CreditLimit.Amount
		}
		balSum += ob
		limitSum += ol
	}
	if limitSum > 0 {
		in.HasCredit = true
		if pct, ok := ledger.Utilization(balSum, limitSum); ok {
			in.AggUtilizationPct = pct
		}
	}

	// Net-worth trend: the trailing six-month change as a percentage. A meaningful
	// percentage needs a positive starting net worth; otherwise the factor is
	// inapplicable (the model excludes it and re-normalizes the remaining weights).
	nwStart := dateutil.AddMonths(curMonth, -healthNWTrendMonths)
	if series, nwErr := ledger.NetWorthSeries(d.Accounts, d.Transactions, []time.Time{nwStart, d.Now}, rates); nwErr == nil && len(series) == 2 && series[0].Amount > 0 {
		in.HasNWTrend = true
		in.NWTrendPct = float64(series[1].Amount-series[0].Amount) / float64(series[0].Amount) * 100
	}

	return in
}

// addHealthVars runs the health model over the shared HealthInputs derivation
// and exposes each factor's score + exact weight (zero when inapplicable or in
// the not-enough-data case), the deficit penalty, and the two raw values. The
// health_score molecule then reproduces healthscore.Evaluate's headline exactly
// (guarded by healthscore.TestWeightFormulaIdentity and this package's tests).
func addHealthVars(out map[string]float64, d Data) {
	for _, name := range HealthVarNames {
		out[name] = 0
	}
	in := HealthInputs(d)
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
}
