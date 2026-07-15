// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// QuickAddBudgetImpact renders the entry-time budget-impact caption (TX17): a
// single t-caption line answering "what does this entry do to my money?" while
// Quick-Add is being filled.
//
//   - categoryID / amountMinor are the in-progress entry. amountMinor is the
//     unsigned magnitude the user typed (Quick-Add applies the sign on save).
//   - When categoryID has a budget THIS period, the line leads with what the
//     entry leaves in that budget ("leaves $142 in Dining this month"). The tone
//     shifts to a warning when the entry would push the budget over its limit.
//   - It always shows safe-to-spend, read from the same live engine surface the
//     dashboard uses (safe_to_spend molecule).
//   - It renders nothing until a category is chosen and a non-zero amount typed —
//     the two inputs the caption needs. The budget clause is omitted when the
//     category has no budget (safe-to-spend still shows).
//
// The caption is advisory only; the actual over-budget "cover" moment (TX14) is
// posted by the save path, not here.
func QuickAddBudgetImpact(app *appstate.App, categoryID string, amountMinor int64) uic.Node {
	if app == nil || categoryID == "" || amountMinor <= 0 {
		return Fragment()
	}

	now := time.Now()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	cats := app.Categories()
	weekStart := uistate.LoadPrefs().WeekStartWeekday()

	// Budget clause: find the budget that tracks this category (directly or via a
	// parent-category rollup) for its current period, and compute what the pending
	// entry would leave.
	var (
		haveBudget bool
		budgetName string
		leaveMinor int64
		leaveCur   string
		over       bool
	)
	for _, b := range app.Budgets() {
		covers := categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())
		if !b.TracksCategory(categoryID) && !covers[categoryID] {
			continue
		}
		bs, be := budgeting.PeriodRange(b.Period, now, weekStart)
		st, err := budgeting.EvaluateRollup(b, app.Transactions(), bs, be, rates, budgeting.DefaultNearThreshold, covers)
		if err != nil {
			continue
		}
		haveBudget = true
		budgetName = b.Name
		leaveCur = st.Remaining.Currency
		if leaveCur == "" {
			leaveCur = base
		}
		// The entry is charged in the budget's currency at parity here (Quick-Add
		// records in the account currency; the caption is a live estimate, so a
		// same-currency subtraction is the honest, cheap read).
		leaveMinor = st.Remaining.Amount - amountMinor
		over = leaveMinor < 0
		break
	}

	// Safe-to-spend from the live engine surface (major-unit float → minor units),
	// formatted like every other money figure (symbol + thousands grouping).
	vars := liveEngineVars(app)
	stsMinor := int64(vars["safe_to_spend"]*100 + 0.5)
	if vars["safe_to_spend"] < 0 {
		stsMinor = int64(vars["safe_to_spend"]*100 - 0.5)
	}
	stsText := fmtMoney(money.New(stsMinor, base))

	var line string
	warn := false
	if haveBudget {
		if over {
			line = uistate.T("txImpact.overBudget", budgetName, fmtMoney(money.New(absCents(leaveMinor), leaveCur)), stsText)
			warn = true
		} else {
			line = uistate.T("txImpact.leaves", fmtMoney(money.New(leaveMinor, leaveCur)), budgetName, stsText)
		}
	} else {
		line = uistate.T("txImpact.safeOnly", stsText)
	}

	classes := []any{css.Class("t-caption", tw.TextDim)}
	if warn {
		classes = []any{css.Class("t-caption", "neg")}
	}
	return P(append(classes,
		Attr("role", "status"),
		Attr("data-testid", "qa-budget-impact"),
		Style(map[string]string{"margin-top": "0.25rem"}),
		line)...)
}

// absCents returns the absolute value of a minor-unit amount for display.
func absCents(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
