// SPDX-License-Identifier: MIT

package idlecash

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

// LedgerInputs are the household records the idle-cash figure is derived from.
type LedgerInputs struct {
	Accounts []domain.Account
	// Balances holds each account's booked balance. An account missing from it makes
	// the liquid total unavailable.
	Balances  map[string]money.Money
	Recurring []domain.Recurring
	Goals     []domain.Goal
	// Rates converts every figure to Rates.Base.
	Rates currency.Rates
	// Now fixes the month whose bills and goal set-asides are committed.
	Now time.Time
	// BenchmarkAPRPercent is the user-entered annual yield, as in Inputs.
	BenchmarkAPRPercent float64
}

// FromLedger derives the idle-cash breakdown from household records. Liquid cash is
// the spendable balance across cash-type accounts. Committed is the bills due before
// the end of this month plus the goal set-asides prorated for it, the same basis the
// safe-to-spend figure uses, so the two reconcile. All amounts are in Rates.Base
// minor units. An error means a figure could not be computed exactly, and no
// breakdown is returned.
func FromLedger(in LedgerInputs) (Result, error) {
	liquid, err := ledger.LiquidFromBalances(in.Accounts, in.Balances, in.Rates)
	if err != nil {
		return Result{}, fmt.Errorf("idlecash: liquid cash: %w", err)
	}
	_, monthEnd := dateutil.MonthRange(in.Now)
	bills, err := safespend.BillsDueBeforeChecked(in.Accounts, in.Recurring, in.Now, monthEnd, in.Rates)
	if err != nil {
		return Result{}, fmt.Errorf("idlecash: bills due this month: %w", err)
	}
	goalNeeds, err := safespend.GoalContributionsProratedChecked(in.Goals, in.Now, in.Rates)
	if err != nil {
		return Result{}, fmt.Errorf("idlecash: goal set-asides: %w", err)
	}
	committed, err := money.Add(bills, goalNeeds)
	if err != nil {
		return Result{}, fmt.Errorf("idlecash: committed total: %w", err)
	}
	return Evaluate(Inputs{
		LiquidMinor:         liquid.Amount,
		CommittedMinor:      committed,
		BenchmarkAPRPercent: in.BenchmarkAPRPercent,
	}), nil
}
