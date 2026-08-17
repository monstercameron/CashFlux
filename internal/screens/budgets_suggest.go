// SPDX-License-Identifier: MIT

//go:build js && wasm

// A suggested limit for ONE budget (SM-12).
//
// The auto-budget flow already proposes limits for every category at once, behind
// a slider and a review step. That is the right shape for setting up a budget set
// and the wrong shape for the far commoner moment: looking at one budget that is
// obviously mis-sized and wanting it to match reality. This is that moment — one
// line, one figure, one click.
//
// The arithmetic is budgeting.SuggestLimit, the same trailing-average the
// auto-budget flow uses, so accepting the hint and running auto-budget can never
// propose two different numbers for the same category.
package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// suggestLimitCode gates SM-12. B12 is "Healthy budget average" — a Free feature,
// so the hint runs with no provider and defaults on.
const suggestLimitCode = "SMART-B12"

// suggestLimitMonths is how much history the average is taken over. Three months
// is long enough to absorb one odd month and short enough to still describe how
// the household lives now.
const suggestLimitMonths = 3

// suggestLimitDriftBP is how far the current limit must sit from the suggestion
// before the hint appears, in basis points. A budget already within 10% of its own
// trailing average does not need advice; showing it anyway is how a helpful line
// becomes wallpaper that gets ignored when it finally matters.
const suggestLimitDriftBP = 1000

// budgetSuggestProps carries the budget the hint is about.
type budgetSuggestProps struct {
	Budget domain.Budget
}

// budgetSuggestHint renders "your 3-month average here is $X — use it". Its own
// component: it renders inside the budget list and owns a click hook.
func budgetSuggestHint(props budgetSuggestProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	settings := uistate.LoadSmartSettings()
	b := props.Budget

	suggested, ok := budgetSuggestedLimit(app, b)

	apply := func() {
		a := appstate.Default
		if a == nil || !ok {
			return
		}
		// Re-read the budget at click time: the list behind this hint may have been
		// re-rendered from newer data since it was drawn.
		var cur domain.Budget
		found := false
		for _, x := range a.Budgets() {
			if x.ID == b.ID {
				cur, found = x, true
				break
			}
		}
		if !found {
			return
		}
		auditview.CaptureNow()
		cur.Limit = money.New(suggested, cur.Limit.Currency)
		if err := a.PutBudget(cur); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostUndoable(uistate.T("sm12.appliedUndo", cur.Name, fmtMoney(cur.Limit)))
	}

	if !ok {
		return Fragment()
	}
	amount := fmtMoney(money.New(suggested, b.Limit.Currency))
	return smartRowHintFor(settings, suggestLimitCode, smart.AffordanceFieldAssist, smartRowHintProps{
		Key:         "sm12:" + b.ID,
		TestID:      "sm12-" + b.ID,
		Text:        uistate.T("sm12.hint", suggestLimitMonths, amount),
		ActionLabel: uistate.T("sm12.action", amount),
		OnAction:    apply,
		Dismissible: true,
	})
}

// budgetSuggestedLimit returns the trailing-average limit for a budget, or
// ok=false when there is nothing worth suggesting.
//
// It declines in three cases, each deliberate: a budget with no history to learn
// from (the average would be a guess dressed as a measurement), a budget whose
// limit already sits within suggestLimitDriftBP of the average (correct advice
// nobody needs is how advice stops being read), and a saving-direction budget —
// a savings TARGET is a decision about what you intend to put away, and "your
// three-month average" is an argument for aiming lower, which is the opposite of
// what a savings goal is for.
func budgetSuggestedLimit(app *appstate.App, b domain.Budget) (int64, bool) {
	if app == nil || b.Direction.IsSaving() {
		return 0, false
	}
	// Only a monthly budget can be compared with a monthly average without
	// converting the period, and a conversion here would silently propose a weekly
	// limit built from a monthly figure.
	if b.Period != domain.PeriodMonthly {
		return 0, false
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// The budget's WHOLE scope, matching what its meter counts: a parent-category
	// budget must average its sub-categories too, or the suggestion would sit below
	// a spend figure it cannot explain (the C586 mismatch, in the other direction).
	covers := categorytree.DescendantsOfAll(app.Categories(), b.TrackedCategoryIDs())
	if len(covers) == 0 {
		return 0, false
	}
	suggested, err := budgeting.SuggestLimitIn(covers, app.Transactions(), time.Now(), suggestLimitMonths, rates)
	if err != nil || suggested <= 0 {
		return 0, false
	}
	limit := b.Limit.Amount
	if limit > 0 {
		drift := suggested - limit
		if drift < 0 {
			drift = -drift
		}
		if drift*10000 < limit*suggestLimitDriftBP {
			return 0, false
		}
	}
	return suggested, true
}
