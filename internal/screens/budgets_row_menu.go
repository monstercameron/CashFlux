// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRowActionsProps carries the explicit inputs for budgetRowActions.
type budgetRowActionsProps struct {
	isOver          bool
	s               budgeting.Status
	openTopup       ui.Handler
	menuOpen        ui.State[bool]
	menuID          string
	title           string
	toggleMenu      ui.Handler
	closeMenu       ui.Handler
	props           budgetRowProps
	openCover       ui.Handler
	openEdit        ui.Handler
	openCategories  ui.Handler
	notesLabel      string
	openNotes       ui.Handler
	openFormulas    ui.Handler
	releaseUnused   ui.Handler
	toggleSnooze    ui.Handler
	snoozeLabel     string
	hasRecurring    bool
	removeRecurring ui.Handler
	del             ui.Handler
	coverBtn        ui.Node
	canDrill        bool
	drillTitle      string
	drillMenu       ui.Handler
}

// budgetRowActions builds the row actions section.
func budgetRowActions(p budgetRowActionsProps) (ui.Node, ui.Node) {
	// Top up is a visible card button (the frequent proactive action) on budgets that
	// aren't over.
	var topupBtn ui.Node = Fragment()
	topupLabel := uistate.T("budgets.actionRaiseLimit")
	if p.s.Budget.IsSaving() {
		topupLabel = uistate.T("bc4b2.increaseTarget")
	}
	if !p.isOver {
		topupBtn = Button(css.Class("btn"), Type("button"), Attr("data-testid", "budget-topup-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.topupTitle")), OnClick(p.openTopup), topupLabel)
	}
	coverLabel := uistate.T("bv7g.cover.action")

	// The row actions, rendered as the card's footer (pinned to the bottom by CSS) so the
	// card reads top-to-bottom: title → amount → bar → status → actions. The footer keeps
	// only the EVERYDAY money moves inline — Cover/Top-up and the Transactions drill —
	// and everything configurational (Edit budget, Edit tracking, Notes, Formulas) folds
	// into the ⋯ overflow with the destructive actions at its bottom (standing directive:
	// delete never sits exposed on the row). Seven visible buttons made every card a
	// toolbar; three keep it a card (design critique #8).
	menuHidden := ""
	if !p.menuOpen.Get() {
		menuHidden = " hidden-menu"
	}
	// The ⋯ overflow is shared by both densities. In COMPACT the row has no footer, so
	// the money moves (Cover/Top-up, Transactions) join the top of the menu; the
	// destructive group stays pinned at the bottom either way.
	kebabNode := Div(css.Class("add-wrap"), Attr("id", p.menuID),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-kebab-"+p.s.Budget.ID), Attr("title", uistate.T("budgets.moreActions")), Attr("aria-label", uistate.T("budgets.moreActions")+" — "+p.title), Attr("aria-haspopup", "menu"), Attr("aria-expanded", ariaBool(p.menuOpen.Get())), OnClick(p.toggleMenu), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
		Div(ClassStr("add-backdrop"+menuHidden), OnClick(p.closeMenu)),
		Div(ClassStr("add-menu"+menuHidden), Attr("role", "menu"),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-topup-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.topupTitle")), OnClick(p.openTopup), topupLabel),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-cover-btn-"+p.s.Budget.ID), Title(uistate.T("bv7g.cover.action")), OnClick(p.openCover), coverLabel),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-budget-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.editTitle")), OnClick(p.openEdit), uistate.T("budgets.editAction")),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "edit-budget-cats-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.catsTitle")), OnClick(p.openCategories), uistate.T("budgets.actionEditTracking")),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-notes-btn-"+p.s.Budget.ID), Attr("aria-label", p.notesLabel+" — "+p.title), Title(uistate.T("budgets.notesTitle")), OnClick(p.openNotes), p.notesLabel),
			Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "budget-formulas-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.formulasTitle")), OnClick(p.openFormulas), uistate.T("budgets.actionEditFormulasPlain")),
			If(!p.s.Budget.IsSaving() && p.s.Budget.Rollover && p.s.Remaining.Amount > 0, Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "budget-release-btn-"+p.s.Budget.ID), Title(uistate.T("budgets.releaseTitle")),
				OnClick(p.releaseUnused), uistate.T("budgets.releaseAction"))),
			// WF17: pause a funding target instead of deleting it to stop the
			// nagging. Deleting to silence something is how settings get lost, and
			// until now it was the only option.
			If(p.s.Budget.HasTarget(), Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "budget-snooze-btn-"+p.s.Budget.ID),
				OnClick(p.toggleSnooze), p.snoozeLabel)),
			If(p.hasRecurring, Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "remove-recurring-btn-"+p.s.Budget.ID), OnClick(p.removeRecurring), uistate.T("budgets.removeRecurring"))),
			Button(css.Class("add-item danger"), Type("button"), Attr("role", "menuitem"), Attr("data-testid", "delete-budget-btn-"+p.s.Budget.ID), Attr("aria-label", uistate.T("budgets.deleteTitle")), Title(uistate.T("budgets.deleteTitle")), OnClick(p.del), uistate.T("budgets.deleteAction")),
		),
	)
	actionsRow := Div(css.Class("budget-actions"),
		p.coverBtn,
		topupBtn,
		If(p.canDrill, Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "budget-view-txns-"+p.s.Budget.ID), Title(p.drillTitle), OnClick(p.drillMenu),
			uiw.Icon(icon.Receipt, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("nav.transactions")))),
		kebabNode,
	)

	return kebabNode, actionsRow
}
