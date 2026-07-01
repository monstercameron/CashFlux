// SPDX-License-Identifier: MIT

// Package coverformula evaluates budget-cover formulas in a specific budget's context.
// A cover's amount (destination context) and each source's weight (source context) can
// be a formula over the engine variable surface plus that budget's own values —
// spent / limit / remaining / overspend / percent and its cf_budget_<key> custom
// fields — so a recurring cover can, e.g., always cover the current `overspend` or
// weight sources by `cf_budget_priority`. Pure Go; the caller supplies the live data.
package coverformula

import (
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
	Txns           []domain.Transaction
	Rates          currency.Rates
	Now            time.Time
	WeekStart      time.Weekday
	PayCycleAnchor time.Time
	Defs           []customfields.Def
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
	bs, be := budgeting.PeriodRangeAnchored(b.Period, c.Now, c.WeekStart, c.PayCycleAnchor)
	st, err := budgeting.EvaluateRollup(b, c.Txns, bs, be, c.Rates, budgeting.DefaultNearThreshold, nil)
	if err != nil {
		return 0, err
	}
	dec := currency.Decimals(b.Limit.Currency)
	ctxVars := engineenv.BudgetVars(b, minorToMajor(st.Spent.Amount, dec), minorToMajor(b.Limit.Amount, dec), c.Defs)
	v, err := formula.Eval(expr, formula.Env{Vars: engineenv.Merge(c.Base, ctxVars)})
	if err != nil {
		return 0, err
	}
	return asNumber(v)
}

// AmountMinor evaluates a cover-amount formula in the destination budget's context and
// returns the result in MINOR units (rounded), clamped at 0. Convenience for the apply
// path, which needs cents.
func (c Context) AmountMinor(expr string, dest domain.Budget) (int64, error) {
	major, err := c.EvalInBudget(expr, dest)
	if err != nil {
		return 0, err
	}
	dec := currency.Decimals(dest.Limit.Currency)
	minor := int64(roundHalfAway(major * pow10(dec)))
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
	w := int(roundHalfAway(v))
	if w < 0 {
		w = 0
	}
	return w, nil
}

func minorToMajor(minor int64, dec int) float64 { return float64(minor) / pow10(dec) }

func pow10(n int) float64 {
	d := 1.0
	for i := 0; i < n; i++ {
		d *= 10
	}
	return d
}

func roundHalfAway(f float64) float64 {
	if f < 0 {
		return -roundHalfAway(-f)
	}
	return float64(int64(f + 0.5))
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
	return 0, nil
}
