// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import "github.com/monstercameron/GoWebComponents/v4/state"

// budgetFormulasAtomID keys the shared "show budget metrics" toggle for the
// widgetized /budgets surface.
const budgetFormulasAtomID = "budgets:showFormulas"

// budgetEditAtomID keys the shared budget-editor modal selector.
const budgetEditAtomID = "budgets:edit"

// Budget-editor modes (which form the shell-root flip modal shows).
const (
	BudgetEditModeEdit  = "edit"  // full edit form (name/limit/period/owner/rollover/method/custom)
	BudgetEditModeTopup = "topup" // raise this budget's limit by an entered amount
	BudgetEditModeCover = "cover" // move limit from another budget to clear an overspend
)

// BudgetEdit selects the budget + editor a modal should show. A zero value (empty ID)
// means no modal is open. Mode is one of the BudgetEditMode* constants.
type BudgetEdit struct {
	ID   string
	Mode string
}

// capturedBudgetEdit holds the atom reference captured during a render so that
// SetBudgetEdit/CloseBudgetEdit can update it from a click handler without calling
// state.UseAtom outside a render (which panics). Mirrors the account-editor atom.
var (
	capturedBudgetEdit state.Atom[BudgetEdit]
	budgetEditCaptured bool
)

// UseBudgetEdit returns the shared atom selecting which budget editor modal is open.
// The budget row's Edit / Top up buttons set it; the shell-mounted BudgetEditHost
// reads it and renders the matching form inside a flip modal — rather than an inline
// row form, which sat under transformed bento/tile ancestors and rendered off-centre.
// Calling it (in a render) also captures the atom for SetBudgetEdit/CloseBudgetEdit.
func UseBudgetEdit() state.Atom[BudgetEdit] {
	a := state.UseAtom(budgetEditAtomID, BudgetEdit{})
	capturedBudgetEdit = a
	budgetEditCaptured = true
	return a
}

// SetBudgetEdit opens the budget editor modal for the given budget + mode. Safe to
// call from a click handler (uses the captured atom, not UseAtom).
func SetBudgetEdit(e BudgetEdit) {
	if budgetEditCaptured {
		capturedBudgetEdit.Set(e)
	}
}

// CloseBudgetEdit clears the budget-editor atom (dismisses the modal).
func CloseBudgetEdit() { SetBudgetEdit(BudgetEdit{}) }

// capturedBudgetCatsEdit mirrors the budget-editor captured-atom pattern for the
// "tracked categories" modal, so a row ⋯ menu can open it from a click handler.
var (
	capturedBudgetCatsEdit state.Atom[string]
	budgetCatsEditCaptured bool
)

// UseBudgetCategoriesEdit returns the atom holding the id of the budget whose
// "tracked categories" flip modal is open ("" = closed). The row ⋯ menu sets it; the
// shell-root BudgetCategoriesHost renders the modal.
func UseBudgetCategoriesEdit() state.Atom[string] {
	a := state.UseAtom("budgets:catsEdit", "")
	capturedBudgetCatsEdit = a
	budgetCatsEditCaptured = true
	return a
}

// SetBudgetCategoriesEdit opens (budgetID) or closes ("") the tracked-categories modal.
// Safe from a click handler (uses the captured atom, not UseAtom).
func SetBudgetCategoriesEdit(budgetID string) {
	if budgetCatsEditCaptured {
		capturedBudgetCatsEdit.Set(budgetID)
	}
}

// UseBudgetsShowFormulas returns the shared atom selecting whether the "Budget
// metrics" formula tile is revealed on /budgets. The toolbar's Formulas toggle sets
// it; the surface host appends the formula tile when it is on. Opt-in so the default
// page stays focused on the budgets themselves, while power users can compute over
// their budget aggregates and number-typed budget custom fields (which surface as
// cf_budget_* variables in the engine).
func UseBudgetsShowFormulas() state.Atom[bool] { return state.UseAtom(budgetFormulasAtomID, false) }

// UseGoalsShowFormulas returns the shared atom selecting whether the "Goal metrics"
// formula tile is revealed on /goals — the goals analog of UseBudgetsShowFormulas.
func UseGoalsShowFormulas() state.Atom[bool] { return state.UseAtom("goals:showFormulas", false) }

// UseDebtShowFormulas returns the shared atom selecting whether the "Debt metrics" formula
// tile is revealed on /debt — the debt analog of UseBudgetsShowFormulas. Opt-in so the
// default page stays focused on the debts themselves, while power users can compute over
// the debt_* engine variables (owed / APR / utilization / min payment) and the debt
// aggregate atoms/molecules (credit_utilization, min_payments_total, …).
func UseDebtShowFormulas() state.Atom[bool] { return state.UseAtom("debt:showFormulas", false) }

// UseBudgetAutoOpen returns the shared atom controlling whether the "Auto budget" review
// flip modal is open. The budgets toolbar's Auto-budget button sets it; the shell-root
// AutoBudgetHost renders the modal when true.
func UseBudgetAutoOpen() state.Atom[bool] { return state.UseAtom("budgets:autoOpen", false) }

// UseBudgetsLastMonth returns the shared atom for the budgets page's one-click "Last
// month" toggle: when true, every budget tile evaluates the PREVIOUS period instead of
// the current one, so the user can see last month's picture at a glance. Budgets-local
// (doesn't touch the global period), and resets naturally on reload.
func UseBudgetsLastMonth() state.Atom[bool] { return state.UseAtom("budgets:lastMonth", false) }

// UseInvestShowFormulas returns the shared atom for the /investments "Portfolio metrics"
// formula tile reveal — the investments analog of the other formula toggles.
func UseInvestShowFormulas() state.Atom[bool] { return state.UseAtom("invest:showFormulas", false) }

// UseInvestAddOpen returns the shared atom controlling whether the /investments "Add
// holding" form is revealed (toggled by the toolbar's Add button; read by the securities
// tile so the form and the button stay in sync across the widgetized surface).
func UseInvestAddOpen() state.Atom[bool] { return state.UseAtom("invest:addOpen", false) }

// UseInvestGrowthMonths returns the shared atom for the /investments growth-chart window in
// months (1, 6, or 12; default 12), toggled by the chart's segmented 1M/6M/1Y control.
func UseInvestGrowthMonths() state.Atom[int] { return state.UseAtom("invest:growthMonths", 12) }

// UseInvestPoolEditID returns the atom driving the create/edit-pool flip modal: "" = closed,
// "new" = create a pool, or a pool id = edit that pool. Read by InvestPoolEditHost.
func UseInvestPoolEditID() state.Atom[string] { return state.UseAtom("invest:poolEdit", "") }
