// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
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
	// ScalableCount is how many budgets the reconcile action would actually
	// change: the ones visible under this member perspective with a limit to
	// scale, which is exactly the set the Adjust-all form previews. It is stated
	// on the button's summary so the size of the change is known before the form
	// opens, not after (C671).
	ScalableCount int
	// PeriodLabel names the window being viewed ("Jul 2026"). The reconcile action
	// reaches "this period", and on a closed period the summary has to say WHICH,
	// because altering a month that is over is a different act from correcting the
	// one you are in.
	PeriodLabel string
}

// fundedReconcileLabelKey, fundedReconcileTitleKey and fundedReconcileSummaryKey
// pick the reconcile action's copy for a live period or a closed one.
//
// All three branch, deliberately. The first version of this fix put the
// closed-period warning only in the caption below the button, which leaves the
// prominent control saying "this period" over a month that ended — the same shape
// as the defect being fixed, one layer down. The label names the month, the
// tooltip says it has ended, and the caption carries the size.
func fundedReconcileLabelKey(historical bool) string {
	if historical {
		return "budgets.fundedReconcilePast"
	}
	return "budgets.fundedReconcileScoped"
}

func fundedReconcileTitleKey(historical bool) string {
	if historical {
		return "budgets.fundedReconcileTitlePast"
	}
	return "budgets.fundedReconcileTitleScoped"
}

func fundedReconcileSummaryKey(historical bool) string {
	if historical {
		return "budgets.fundedReconcileSummaryPast"
	}
	return "budgets.fundedReconcileSummary"
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
	// Nothing outstanding means nothing is waiting to arrive, so what is "unfunded"
	// is purely over-assignment — a number the hero already states. Left in, this
	// card repeated that figure under a different claim ("planned against income
	// you haven't received") that is false once every expected dollar has landed:
	// identical digits, two meanings, one of them wrong (Cam, 2026-08-31).
	if f.Shortfall() == 0 {
		return Fragment()
	}
	reducePct := f.ReduceToFitPct()
	openAdjust := uistate.UseBudgetAdjustOpen()
	// C671: the action opens a form that used to be pre-filled for a PERMANENT
	// rewrite of every budget, and said so only after it was open. Reconciling one
	// underfunded month is a current-period act, so that is what this hands over —
	// and the label, tooltip and summary below all say so before it is pressed.
	// The form still offers the permanent choice; it is no longer the silent one.
	reconcile := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetAdjustSeed(strconv.FormatFloat(reducePct, 'f', -1, 64), string(budgeting.AdjustThisPeriod))
		openAdjust.Set(true)
	}))

	// Every reconcile string names the period, so a missing label falls back to a
	// phrase rather than leaving a possessive dangling off nothing.
	periodLabel := props.PeriodLabel
	if periodLabel == "" {
		periodLabel = uistate.T("budgets.fundedReconcileThisPeriod")
	}
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
		If(reducePct < 0, Fragment(
			// The CONTROL carries the disclosure, not only the caption under it. A
			// reader who acts on the prominent thing and skips the small grey line
			// beneath it is the reader this ticket is about, so on a closed period
			// the button and its tooltip name the month themselves.
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "budgets-funded-reconcile"),
				Title(uistate.T(fundedReconcileTitleKey(props.Historical), periodLabel)), OnClick(reconcile),
				uistate.T(fundedReconcileLabelKey(props.Historical), periodLabel)),
			// C671: the size and the reach, stated before the click rather than
			// discovered inside the form it opens. The count is the budgets this
			// perspective can actually scale — the same set the form previews — so
			// the two figures here are the two figures there.
			//
			// A closed period gets its own sentence. "This period" is honest but
			// incomplete when the period has already ended: the change would alter
			// what the app reports about a month that is over, which is a different
			// act from correcting the one you are living in, and naming the month is
			// the only way the reader can tell which they are about to do.
			Span(css.Class("budget-funded-body", tw.TextDim), Attr("data-testid", "budgets-funded-reconcile-summary"),
				uistate.T(fundedReconcileSummaryKey(props.Historical), plural(props.ScalableCount, "budget"),
					formatAdjustPct(math.Abs(reducePct)), periodLabel)),
		)),
	)
}
