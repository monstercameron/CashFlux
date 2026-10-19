// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRowDisclosureResult carries the explicit results for useBudgetRowDisclosure.
type budgetRowDisclosureResult struct {
	menuOpen      ui.State[bool]
	detailsOpen   ui.State[bool]
	toggleDetails ui.Handler
	menuID        string
	toggleMenu    ui.Handler
	closeMenu     ui.Handler
	prefs         prefs.Prefs
}

// useBudgetRowDisclosure registers the menu and details hooks unconditionally.
func useBudgetRowDisclosure() budgetRowDisclosureResult {
	// The everyday actions are labelled tool buttons surfaced inline (the full-width
	// card has the room); the DESTRUCTIVE actions (Remove recurring, Delete) live in a
	// "⋯" overflow menu — a standing directive: delete always stays in the kebab, never
	// as an always-visible row button. Escape + outside-pointerdown dismiss the menu;
	// AnchorPopover flips it near the viewport edge.
	menuOpen := ui.UseState(false)
	// C595: the card stacked every caption it had — tracking, tags, committed,
	// set-aside, target, coverage, owner, method, custom fields, pace, carry,
	// metrics — between the bar and the actions, so "how much is left?" competed
	// with a dozen explanations. The explanatory ones now live behind one
	// disclosure; anything that CHANGES the headline figure's meaning (an
	// effective cap, an envelope balance, a projected overspend) stays out.
	detailsOpen := ui.UseState(false)
	toggleDetails := ui.UseEvent(Prevent(func() { detailsOpen.Set(!detailsOpen.Get()) }))
	menuID := ui.UseId()
	toggleMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(!menuOpen.Get()) }))
	closeMenu := ui.UseEvent(Prevent(func() { menuOpen.Set(false) }))
	uiw.DismissPopover(menuOpen.Get(), menuID, func() { menuOpen.Set(false) })
	uiw.AnchorPopover(menuOpen.Get(), menuID)
	prefs := uistate.UsePrefs().Get()
	return budgetRowDisclosureResult{
		menuOpen: menuOpen, detailsOpen: detailsOpen, toggleDetails: toggleDetails, menuID: menuID, toggleMenu: toggleMenu,
		closeMenu: closeMenu, prefs: prefs,
	}
}

// budgetRowNavigationResult carries the explicit results for useBudgetRowNavigation.
type budgetRowNavigationResult struct {
	del            ui.Handler
	drillScope     budgeting.Scope
	drill          ui.Handler
	canDrill       bool
	openEdit       ui.Handler
	openCategories ui.Handler
	openTopup      ui.Handler
}

// useBudgetRowNavigation registers drill-through and editor navigation handlers.
func useBudgetRowNavigation(menuOpen ui.State[bool], props budgetRowProps, s budgeting.Status) budgetRowNavigationResult {
	del := ui.UseEvent(Prevent(func() { menuOpen.Set(false); props.OnDelete(s.Budget.ID) }))
	// C585: the drill-through carries the budget's WHOLE spend scope — tracked
	// categories, their descendants, and any tracked tags — the same set the bar
	// counts against. Passing only TrackedCategoryIDs is what opened an empty
	// ledger under a non-zero total for a parent-category budget whose spend all
	// sat in its sub-categories.
	drillScope := budgetDrillScope(s.Budget)
	drill := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(drillScope, props.PeriodFrom, props.PeriodTo)
		}
	}))
	// The drill affordances show whenever the budget counts anything at all (a
	// multi-category budget may have no single primary CategoryID; a tag budget
	// may count charges from any of them).
	canDrill := !drillScope.IsEmpty()
	// Edit and Top up open the shell-root flip modal (BudgetEditHost) rather than an
	// inline row form: a row sits under transformed bento/tile ancestors, which threw an
	// in-row modal off-centre. SetBudgetEdit updates the atom the host captured. Edit is
	// the lower-frequency action, so it lives in the ⋯ menu (and closes it); Top up is a
	// visible card button.
	openEdit := ui.UseEvent(Prevent(func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeEdit})
	}))
	openCategories := ui.UseEvent(Prevent(func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetCategoriesEdit(s.Budget.ID)
	}))
	openTopup := ui.UseEvent(Prevent(func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeTopup})
	}))
	return budgetRowNavigationResult{
		del: del, drillScope: drillScope, drill: drill, canDrill: canDrill, openEdit: openEdit, openCategories: openCategories,
		openTopup: openTopup,
	}
}

// useBudgetRowFundingActions registers target snoozing and release-unused handlers.
func useBudgetRowFundingActions(s budgeting.Status, props budgetRowProps, menuOpen ui.State[bool]) (string, ui.Handler, ui.Handler) {
	// Release unused funds: the mirror of Top up — a NEGATIVE one-time period
	// boost lowering THIS period's cap to what's spent, so the leftover returns
	// to the plan (To Assign under zero-based budgeting) instead of implying it
	// may still be spent. Future periods are untouched (boosts are per-period).
	// WF17: pause or resume the funding target. One action that reads what it
	// does rather than a toggle labelled the same in both states — a control whose
	// label does not change is one nobody trusts they have pressed.
	snoozeLabel := uistate.T("budgets.pauseBudgetForMonth")
	if s.Budget.TargetSnoozed(time.Now()) {
		snoozeLabel = uistate.T("budgets.resumeTarget")
	}
	toggleSnooze := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		b := s.Budget
		now := time.Now()
		if b.TargetSnoozed(now) {
			b.TargetSnoozedUntil = time.Time{}
			if err := app.PutBudget(b); err != nil {
				uistate.PostNotice(budgetErrorText(err), true)
				return
			}
			uistate.PostNotice(uistate.T("budgets.resumedNotice", budgetTitle(s.Budget.Name, props.Category)), false)
		} else {
			// A month, not a date picker. The common case is "not this month", and
			// asking somebody to choose a date for something they want to stop
			// thinking about is the friction that sends them back to deleting it.
			until := now.AddDate(0, 1, 0)
			b.TargetSnoozedUntil = until
			if err := app.PutBudget(b); err != nil {
				uistate.PostNotice(budgetErrorText(err), true)
				return
			}
			uistate.PostNotice(uistate.T("budgets.snoozedNotice",
				budgetTitle(s.Budget.Name, props.Category),
				uistate.LoadPrefs().FormatDate(until)), false)
		}
		uistate.BumpDataRevision()
	}))

	releaseUnused := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		app := appstate.Default
		if app == nil || s.Remaining.Amount <= 0 {
			return
		}
		remaining := s.Remaining
		// C597: the same impact vocabulary every other funds-moving action uses.
		msg := uistate.T("budgets.releaseConfirm", fmtMoney(remaining), s.Budget.Name) + " " +
			budgetRowReleaseImpact()
		uistate.ConfirmModal(msg, false, func(ok bool) {
			if !ok {
				return
			}
			for _, b := range app.Budgets() {
				if b.ID != s.Budget.ID {
					continue
				}
				prefs := uistate.LoadPrefs()
				start := budgetPeriodKeyStartWithWeekStart(b.Period, props.Anchor, budgetPayCycleAnchor(prefs.PayCycleAnchor), prefs.WeekStartWeekday())
				updated, err := b.WithPeriodBoostChecked(start, -remaining.Amount)
				if err == nil {
					err = app.PutBudgetAt(updated, props.Anchor)
				}
				if err != nil {
					uistate.PostNotice(budgetErrorText(err), true)
					return
				}
				uistate.BumpDataRevision()
				uistate.PostUndoable(uistate.T("budgets.releasedToast", fmtMoney(remaining), b.Name))
				return
			}
		})
	}))

	return snoozeLabel, toggleSnooze, releaseUnused
}

// budgetRowDetailActionsResult carries the explicit results for useBudgetRowDetailActions.
type budgetRowDetailActionsResult struct {
	openCover       ui.Handler
	openNotesFn     func()
	openNotes       ui.Handler
	openFormulas    ui.Handler
	drillMenu       ui.Handler
	openBudgetTodos func()
	openTodos       ui.Handler
	todosOpen       ui.State[bool]
	toggleTodos     ui.Handler
	removeRecurring ui.Handler
}

// useBudgetRowDetailActions registers note, formula, follow-up, and recurring actions.
func useBudgetRowDetailActions(s budgeting.Status, menuOpen ui.State[bool], props budgetRowProps, drillScope budgeting.Scope) budgetRowDetailActionsResult {
	// "Cover…" opens the shell-root flip modal (BudgetEditHost cover mode), which picks
	// a source budget + amount and moves the limit — no longer an inline row form.
	openCover := ui.UseEvent(Prevent(func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeCover})
	}))
	// Notes + Formulas open their own modes of the same shell-root editor modal.
	// (They also close the kebab, which is where they now live.)
	// The body is shared: the kebab item needs a Handler, the note marker needs a
	// plain func it can register as its own click (its component owns that hook).
	openNotesFn := func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeNotes})
	}
	openNotes := ui.UseEvent(Prevent(openNotesFn))
	openFormulas := ui.UseEvent(Prevent(func() {
		closeBudgetMenuAndRestoreOpener(menuOpen, s.Budget.ID)
		uistate.SetBudgetEdit(uistate.BudgetEdit{ID: s.Budget.ID, Mode: uistate.BudgetEditModeFormulas})
	}))
	// Transactions drill navigates to the filtered ledger.
	drillMenu := ui.UseEvent(Prevent(func() {
		if props.OnDrill != nil {
			props.OnDrill(drillScope, props.PeriodFrom, props.PeriodTo)
		}
	}))
	// Jump to the To-do list PRE-FILTERED to this budget's follow-ups (link=budget +
	// the specific budget id), used by both a to-do row's title and the "+N more" link.
	openBudgetTodos := func() {
		uistate.SetTodoFilterLink(uistate.TodoLinkBudget)
		uistate.SetTodoFilterLinkID(s.Budget.ID)
		if props.OnViewTodos != nil {
			props.OnViewTodos()
		}
	}
	openTodos := ui.UseEvent(Prevent(func() { openBudgetTodos() }))
	// UX-05: the linked follow-ups list starts collapsed — the header row (with its
	// open/total count) is always visible; the items reveal on click.
	todosOpen := ui.UseState(false)
	toggleTodos := ui.UseEvent(Prevent(func() { todosOpen.Set(!todosOpen.Get()) }))
	removeRecurring := ui.UseEvent(Prevent(func() {
		menuOpen.Set(false)
		if props.OnRemoveRecurring != nil {
			props.OnRemoveRecurring(s.Budget.ID)
		}
	}))
	return budgetRowDetailActionsResult{
		openCover: openCover, openNotesFn: openNotesFn, openNotes: openNotes, openFormulas: openFormulas, drillMenu: drillMenu,
		openBudgetTodos: openBudgetTodos, openTodos: openTodos, todosOpen: todosOpen, toggleTodos: toggleTodos,
		removeRecurring: removeRecurring,
	}
}

// closeBudgetMenuAndRestoreOpener closes an open row menu and focuses its stable
// trigger before a dialog mounts. The dialog then captures the correct opener.
func closeBudgetMenuAndRestoreOpener(menuOpen ui.State[bool], budgetID string) {
	wasOpen := menuOpen.Get()
	menuOpen.Set(false)
	if !wasOpen {
		return
	}
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	trigger := doc.Call("querySelector", `[data-testid="budget-kebab-`+budgetID+`"]`)
	if trigger.Truthy() {
		trigger.Call("focus")
	}
}
