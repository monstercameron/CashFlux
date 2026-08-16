// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetFundedProps carries the funding read plus the currency to format it in.
type budgetFundedProps struct {
	Funding budgeting.FundingRead
	Base    string
	// Historical is true once the period has closed, which turns "hasn't arrived
	// yet" into "never arrived" — a different sentence and a different decision.
	Historical bool
}

// budgetFundedCallout is the C587 state the zero-based hero was missing: the
// difference between a plan that is fully ASSIGNED and one that is fully FUNDED.
//
// The page could report 100% of an expected $10,709.16 assigned while $6,961.00
// had actually arrived, and the $3,748.16 gap appeared only inside a separate
// month-close flow. Nothing on the budgets page itself said the plan was writing
// cheques against money that had not turned up.
//
// It renders NOTHING when the plan is funded — the common, healthy case — so
// this is a state, not a permanent fixture. When it does appear it offers the
// one action that resolves it: scale every assignment down to the money in hand,
// through the same Adjust-all form that previews the change budget by budget and
// can be undone. A one-click fix that wrote silently would be a worse answer
// than the silence it replaces.
func budgetFundedCallout(props budgetFundedProps) ui.Node {
	f := props.Funding
	if !f.Meaningful() || f.FullyFunded() {
		return Fragment()
	}
	reducePct := f.ReduceToFitPct()
	openAdjust := uistate.UseBudgetAdjustOpen()
	reconcile := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetAdjustSeed(strconv.FormatFloat(reducePct, 'f', -1, 64))
		openAdjust.Set(true)
	}))

	unfunded := fmtMoney(money.New(f.Unfunded(), props.Base))
	received := fmtMoney(money.New(f.Received, props.Base))
	assigned := fmtMoney(money.New(f.Assigned, props.Base))
	titleKey := "budgets.fundedTitle"
	bodyKey := "budgets.fundedBody"
	if props.Historical {
		titleKey, bodyKey = "budgets.fundedTitleHist", "budgets.fundedBodyHist"
	}

	return Div(css.Class("budget-funded"), Attr("data-testid", "budgets-funded-callout"), Attr("role", "status"),
		Div(css.Class("budget-funded-main"),
			Span(css.Class("budget-funded-title"), Attr("data-testid", "budgets-funded-title"),
				uistate.T(titleKey, unfunded)),
			Span(css.Class("budget-funded-body", tw.TextDim), uistate.T(bodyKey, assigned, received)),
		),
		// Offered only when a single bulk adjustment can actually express the cut;
		// otherwise the honest thing is to say the gap and let the user decide
		// which assignments to move.
		If(reducePct < 0, Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "budgets-funded-reconcile"),
			Title(uistate.T("budgets.fundedReconcileTitle")), OnClick(reconcile),
			uistate.T("budgets.fundedReconcile"))),
	)
}
