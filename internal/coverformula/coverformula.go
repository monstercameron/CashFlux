// SPDX-License-Identifier: MIT

// Package coverformula evaluates budget-cover formulas in a specific budget's context.
// A cover's amount (destination context) and each source's weight (source context) can
// be a formula over the engine variable surface plus that budget's own values —
// spent / limit / remaining / overspend / percent and its cf_budget_<key> custom
// fields — so a recurring cover can, e.g., always cover the current `overspend` or
// weight sources by `cf_budget_priority`. Pure Go; the caller supplies the live data.
package coverformula

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
)

// Context bundles the live data needed to evaluate a cover formula. Base is the global
// engine surface (engineenv.Vars); each evaluation layers a budget's own context on
// top of it (via engineenv.BudgetVars + Merge).
type Context struct {
	Base           map[string]float64
	Txns           budgeting.NettedLedger
	Rates          currency.Rates
	Now            time.Time
	WeekStart      time.Weekday
	PayCycleAnchor time.Time
	Defs           []customfields.Def
	Categories     []domain.Category
	Recurring      []domain.Recurring
	PoolRollover   bool
}

// ViewInput returns the shared budget evaluator inputs for formulas and covers.
func (c Context) ViewInput() budgeting.ViewInput {
	return budgeting.ViewInput{Ledger: &c.Txns, Transactions: c.Txns,
		Categories: c.Categories, Recurring: c.Recurring, Rates: c.Rates, Base: c.Rates.Base,
		PoolRollover: c.PoolRollover,
		Period:       budgeting.ViewPeriod{Anchor: c.Now, Now: c.Now, WeekStart: c.WeekStart, PayCycleAnchor: c.PayCycleAnchor}}
}

// EvalInBudget evaluates expr in b's context and returns the numeric result in MAJOR
// units (the same units the engine surface uses — dollars, not cents). An empty expr
// returns (0, nil). Errors from the budget evaluation or the formula are returned so
// the caller can surface them.
func (c Context) EvalInBudget(expr string, b domain.Budget) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, nil
	}
	v := budgeting.EvaluateViewBudget(b, c.ViewInput())
	if !v.Valid {
		return 0, v.Error
	}
	st := v.Status
	dec := currency.Decimals(st.Limit.Currency)
	ctxVars := engineenv.BudgetVars(b, minorToMajor(st.Spent.Amount, dec), minorToMajor(st.Limit.Amount, dec), c.Defs)
	value, err := formula.Eval(expr, formula.Env{Vars: engineenv.Merge(c.Base, ctxVars)})
	if err != nil {
		return 0, err
	}
	return asNumber(value)
}

// AmountMinor evaluates a cover-amount formula in the destination budget's context and
// returns the result in MINOR units (rounded), clamped at 0. Convenience for the apply
// path, which needs cents.
func (c Context) AmountMinor(expr string, dest domain.Budget) (int64, error) {
	major, err := c.EvalInBudget(expr, dest)
	if err != nil {
		return 0, err
	}
	dec := currency.Decimals(dest.WithBaseCurrency(c.Rates.Base).Limit.Currency)
	scaled := math.Round(major * pow10(dec))
	if math.IsNaN(scaled) || math.IsInf(scaled, 0) || scaled >= float64(math.MaxInt64) || scaled < float64(math.MinInt64) {
		return 0, fmt.Errorf("cover formula amount is outside the supported range")
	}
	minor := int64(scaled)
	if minor < 0 {
		minor = 0
	}
	return minor, nil
}

// Weight evaluates a source's weight formula in that source's context and returns a
// non-negative integer weight (rounded).
func (c Context) Weight(expr string, src domain.Budget) (int, error) {
	v, err := c.EvalInBudget(expr, src)
	if err != nil {
		return 0, err
	}
	rounded := math.Round(v)
	if math.IsNaN(rounded) || math.IsInf(rounded, 0) || rounded >= float64(int(^uint(0)>>1)) {
		return 0, fmt.Errorf("cover formula weight is outside the supported range")
	}
	if rounded <= 0 {
		return 0, nil
	}
	return int(rounded), nil
}

func minorToMajor(minor int64, dec int) float64 { return float64(minor) / pow10(dec) }

func pow10(n int) float64 {
	d := 1.0
	for i := 0; i < n; i++ {
		d *= 10
	}
	return d
}

// asNumber coerces a formula Value (number or bool) to float64.
func asNumber(v formula.Value) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("cover formula must return a number or a yes/no value")
}
