// SPDX-License-Identifier: MIT

//go:build js && wasm

package uistate

import (
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

// budgetFormulasAtomID keys the shared "show budget metrics" toggle for the
// widgetized /budgets surface.
const budgetFormulasAtomID = "budgets:showFormulas"

// budgetEditAtomID keys the shared budget-editor modal selector.
const budgetEditAtomID = "budgets:edit"

// Budget-editor modes (which form the shell-root flip modal shows).
const (
	BudgetEditModeEdit     = "edit"     // full edit form (name/limit/period/owner/rollover/method/custom)
	BudgetEditModeTopup    = "topup"    // raise this budget's limit (this month / permanent, optionally covered)
	BudgetEditModeCover    = "cover"    // move limit from another budget to clear an overspend
	BudgetEditModeNotes    = "notes"    // add / edit the budget's free-text note
	BudgetEditModeFormulas = "formulas" // read-only: the budget's engine variables + values (copyable)
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
	// C521: the tracked-categories view is a PAGE of the editor, so opening or
	// closing the editor always returns to its first page. Doing this here rather
	// than in the panel's close handler is deliberate — FlipPanel captures that
	// handler in a ref, so a guard written there can run against a stale closure
	// and the editor reopens on a sub-page nobody asked for.
	if budgetCatsEditCaptured {
		capturedBudgetCatsEdit.Set("")
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

// UseCoverAllOpen returns the shared atom controlling whether the "Cover overages"
// flip modal is open (SMART-B14). The over-banner's "Cover all" button sets it; the
// shell-root budgetsCoverAllModal renders the modal when true.
func UseCoverAllOpen() state.Atom[bool] { return state.UseAtom("budgets:coverAllOpen", false) }

// UseTrackUntrackedOpen controls the "Track untracked spending" sheet: the
// review-and-apply surface behind the unbudgeted strip's bulk entry point.
//
// The strip's per-category "Budget this" chips stay as they are — they are a fine
// one-off invitation. This sheet is the other half: every untracked category over
// a twelve-month window, each row independently included, priced, and aimed at a
// new budget or an existing one, applied in a single pass.
func UseTrackUntrackedOpen() state.Atom[bool] {
	return state.UseAtom("budgets:trackUntrackedOpen", false)
}

// UseTrackUntrackedRows holds the sheet's per-row edits between renders, encoded
// as a string because atoms hold values. Format is one row per "|" segment:
//
//	<categoryID>~<include 0|1>~<amount minor>~<budgetID or empty>~<raise 0|1>
//
// A category absent from the string simply uses its seeded defaults, so the atom
// starts empty and only carries what the user has actually touched.
func UseTrackUntrackedRows() state.Atom[string] {
	return state.UseAtom("budgets:trackUntrackedRows", "")
}

// UseBudgetBasisOpen returns the shared atom controlling whether the "Income to budget
// with" flip modal is open. The budget summary's income button sets it; the shell-root
// BudgetBasisHost renders the modal (the income-source picker + rules) when true. Lives
// at the shell root — like the other budget modals — so the fixed panel clears the tile
// transform that would otherwise clip it.
func UseBudgetBasisOpen() state.Atom[bool] { return state.UseAtom("budgets:basisOpen", false) }

// BudgetBasisDraft is the staged copy of the income-basis prefs the "Income to budget
// with" modal edits. Changes are held in the draft and only written to the household
// prefs on Save (CommitBudgetBasisDraft); Cancel discards them by simply not committing.
// This is why the modal edits a draft rather than prefs directly: config changes in this
// project are staged behind a Save/Cancel footer, never applied live.
type BudgetBasisDraft struct {
	Mode        string   // budgeting.IncomeMode* — how income is resolved
	PaycheckMin int64    // paycheck threshold (minor, base) for "paychecks" mode
	Fixed       int64    // fixed monthly figure (minor, base) for "fixed" mode
	Cats        []string // chosen income categories for "categories" mode
	AvgMonths   int      // average the basis over this many recent months (0/1 = last month)
	Rollover    bool     // roll last month's leftover into this month
}

// UseBudgetBasisDraft returns the atom holding the modal's staged income-basis edits.
func UseBudgetBasisDraft() state.Atom[BudgetBasisDraft] {
	return state.UseAtom("budgets:basisDraft", BudgetBasisDraft{})
}

// NewBudgetBasisDraft seeds a draft from the given prefs — called when the modal opens so
// it starts from the household's current basis.
func NewBudgetBasisDraft(p prefs.Prefs) BudgetBasisDraft {
	return BudgetBasisDraft{
		Mode:        p.BudgetIncomeMode,
		PaycheckMin: p.BudgetPaycheckMinMinor,
		Fixed:       p.MonthlyIncomeMinor,
		Cats:        append([]string(nil), p.BudgetIncomeCategoryIDs...),
		AvgMonths:   p.BudgetIncomeAvgMonths,
		Rollover:    p.BudgetRolloverLeftover,
	}
}

// CommitBudgetBasisDraft writes the staged edits into the household prefs and persists
// them (invoked from the modal's Save button).
func CommitBudgetBasisDraft(d BudgetBasisDraft) {
	p := CurrentPrefs()
	p.BudgetIncomeMode = d.Mode
	p.BudgetPaycheckMinMinor = d.PaycheckMin
	p.MonthlyIncomeMinor = d.Fixed
	p.BudgetIncomeCategoryIDs = append([]string(nil), d.Cats...)
	p.BudgetIncomeAvgMonths = d.AvgMonths
	p.BudgetRolloverLeftover = d.Rollover
	SetPrefs(p)
	// Flush the dataset now so the choice survives a refresh right after Save — SetPrefs
	// writes to the store, but the autosave only reaches IndexedDB on its ticker/pagehide,
	// and a fire-and-forget write can be lost if the reload beats it (C2).
	RequestPersist()
}

// Budget sort keys the budget list can be ordered by (the toolbar's Sort control sets
// the atom; the list reads it). "health" is the default (over → near → at-risk → on
// track), the others let the user surface what matters: what's over, closest to the
// edge, most underused, biggest, or by name.
const (
	BudgetSortHealth        = "health"    // over → near → at-risk → on-track (default)
	BudgetSortOverage       = "overage"   // most over budget first
	BudgetSortNearOverage   = "near"      // closest to the limit first (highest % used, under 100)
	BudgetSortUnderutilized = "underused" // most room left (lowest % used) first
	BudgetSortAmount        = "amount"    // largest limit first
	BudgetSortName          = "name"      // alphabetical
)

// UseBudgetSort returns the shared atom holding the budget-list sort key (default
// BudgetSortHealth). Ephemeral (resets on reload) — a transient view choice.
func UseBudgetSort() state.Atom[string] { return state.UseAtom("budgets:sort", BudgetSortHealth) }

// Budget-list densities: comfortable = the full card per budget; compact = one
// scannable line per budget, for households with enough categories that the cards
// stop fitting on a screen (the list stops scaling around ten cards).
const (
	BudgetDensityComfortable = "comfortable"
	BudgetDensityCompact     = "compact"
)

// budgetDensityStoreID keys the persisted density choice in localStorage.
const budgetDensityStoreID = "cashflux:budgets:density"

// UseBudgetDensity returns the shared atom holding the /budgets list density, seeded
// from localStorage — unlike sort, density is a lasting workspace preference (how you
// read the page), not a transient view tweak, so it survives reloads.
func UseBudgetDensity() state.Atom[string] {
	d := kvGet(budgetDensityStoreID)
	if d != BudgetDensityCompact {
		d = BudgetDensityComfortable
	}
	return state.UseAtom("budgets:density", d)
}

// PersistBudgetDensity saves the density choice so the next visit opens the same way.
// kvSet writes into the SQLite dataset, which only reaches IndexedDB on the autosave
// ticker/pagehide — RequestPersist flushes it now so a prompt reload keeps the choice.
func PersistBudgetDensity(d string) {
	kvSet(budgetDensityStoreID, d)
	RequestPersist()
}

// UseBudgetsLastMonth returns the shared atom for the budgets "Last month's spend"
// toggle: when true, each budget row OVERLAYS last period's actual spending in its
// categories plus how it compares to this month's budget — a planning reference — while
// the view stays on THIS month. (It used to re-window the whole page to last month.)
// Budgets-local (doesn't touch the global period), and resets naturally on reload.
func UseBudgetsLastMonth() state.Atom[bool] { return state.UseAtom("budgets:lastMonth", false) }

// UseInvestShowFormulas returns the shared atom for the /investments "Portfolio metrics"
// formula tile reveal — the investments analog of the other formula toggles.
func UseInvestShowFormulas() state.Atom[bool] { return state.UseAtom("invest:showFormulas", false) }

// UseInvestAddOpen returns the shared atom controlling whether the /investments "Add
// holding" form is revealed (toggled by the toolbar's Add button; read by the securities
// tile so the form and the button stay in sync across the widgetized surface).
func UseInvestAddOpen() state.Atom[bool] { return state.UseAtom("invest:addOpen", false) }

// UseLoanScheduleOpen returns the shared atom naming the loan whose full
// amortization schedule is expanded ("" = none) (FP-T2a).
//
// One at a time, and shared rather than per-card: a thirty-year mortgage is 360
// rows, and rendering several schedules at once — or re-rendering one that a
// background update silently reopened — costs more than the answer is worth.
func UseLoanScheduleOpen() state.Atom[string] { return state.UseAtom("loans:scheduleOpen", "") }

// UseHoldingLotsOpen returns the shared atom naming the holding whose purchase
// history (tax lots) is expanded, "" for none (FP-T1d).
func UseHoldingLotsOpen() state.Atom[string] { return state.UseAtom("invest:lotsOpen", "") }

// UseHoldingSellOpen returns the shared atom naming the holding being sold, ""
// for none (FP-T1d).
func UseHoldingSellOpen() state.Atom[string] { return state.UseAtom("invest:sellOpen", "") }

// UseHoldingIncomeOpen returns the shared atom naming the holding whose income
// recorder is open, "" for none (FP-T1f).
func UseHoldingIncomeOpen() state.Atom[string] { return state.UseAtom("invest:incomeOpen", "") }

// UseHoldingPriceEdit returns the shared atom naming the holding whose price is
// being edited ("" = none), plus the draft price and as-of date (FP-T2c).
//
// SHARED atoms rather than per-row component state, and that is the whole point:
// a row's own state is wiped whenever a background re-render remounts the list —
// a notification arriving, another surface bumping the data revision — and the
// editor was closing mid-typing for reasons the user could not see or connect to
// anything they did. State that must outlive a re-render does not belong to a
// row.
func UseHoldingPriceEdit() state.Atom[string] { return state.UseAtom("invest:priceEditID", "") }

// UseHoldingPriceDraft returns the draft price being typed, in major units.
func UseHoldingPriceDraft() state.Atom[string] { return state.UseAtom("invest:priceDraft", "") }

// UseHoldingPriceAsOf returns the draft as-of date, as YYYY-MM-DD.
func UseHoldingPriceAsOf() state.Atom[string] { return state.UseAtom("invest:priceAsOf", "") }

// UseInvestGrowthMonths returns the shared atom for the /investments growth-chart window in
// months (1, 6, or 12; default 12), toggled by the chart's segmented 1M/6M/1Y control.
func UseInvestGrowthMonths() state.Atom[int] { return state.UseAtom("invest:growthMonths", 12) }

// UseInvestPoolEditID returns the atom driving the create/edit-pool flip modal: "" = closed,
// "new" = create a pool, or a pool id = edit that pool. Read by InvestPoolEditHost.
func UseInvestPoolEditID() state.Atom[string] { return state.UseAtom("invest:poolEdit", "") }

// UseBudgetAdjustOpen returns the shared atom controlling whether the "Adjust all"
// form is open (C592). The budgets toolbar's Adjust-all button sets it; the
// shell-root AdjustAllHost renders the modal when true.
//
// It replaced a PromptModal + ConfirmModal pair: a percentage typed into an
// unlabelled dialog, then a sentence quoting it back. Neither could show the
// budgets in scope or the before/after totals, which is the whole point of a bulk
// edit, and a modal shell is the only place with room to.
func UseBudgetAdjustOpen() state.Atom[bool] { return state.UseAtom("budgets:adjustOpen", false) }

// budgetAdjustSeed pre-fills the "Adjust all" percentage when the form is opened
// from somewhere that already knows the answer — the C587 "bring the plan down
// to what has arrived" action, which computes the exact reduction and hands it
// over rather than asking the user to work it out.
//
// A plain variable, not an atom: it is written on the budgets page and read on
// the first render of a modal that has not mounted yet, where a captured atom
// reference would still be nil. Cleared as soon as the form reads it, so a later
// manual open starts blank.
var budgetAdjustSeed string

// budgetAdjustScopeSeed pre-selects how long the next "Adjust all" open lasts
// (C671). It travels with the percentage because the two together are what the
// originating action promised: the reconcile button says it lowers this period's
// plan, so the form it opens has to open on this period, not on a permanent
// rewrite the button never mentioned. Empty leaves the form's own default.
var budgetAdjustScopeSeed string

// SetBudgetAdjustSeed pre-fills the next "Adjust all" open with a percentage and
// the scope the caller is promising. Pass an empty scope to leave the default.
func SetBudgetAdjustSeed(pct, scope string) {
	budgetAdjustSeed, budgetAdjustScopeSeed = pct, scope
}

// TakeBudgetAdjustSeed returns the pending pre-fill (percentage, scope) and
// clears it, so a later manual open starts from the form's own defaults.
func TakeBudgetAdjustSeed() (pct, scope string) {
	pct, scope = budgetAdjustSeed, budgetAdjustScopeSeed
	budgetAdjustSeed, budgetAdjustScopeSeed = "", ""
	return pct, scope
}
