// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetEditNotesSaveProps carries the explicit inputs for useBudgetEditNotesSave.
type budgetEditNotesSaveProps struct {
	app    *appstate.App
	props  BudgetEditFormProps
	errS   ui.State[string]
	done   func()
	notesS ui.State[string]
}

// useBudgetEditNotesSave registers note persistence and destructive-removal confirmation.
func useBudgetEditNotesSave(p budgetEditNotesSaveProps) (ui.Handler, ui.Handler) {
	// writeNote is the single path that changes a budget's note, so that clearing
	// the field and pressing Save cannot behave differently from pressing Remove.
	// An empty result IS a removal and says so; announcing "Note saved" for a
	// deletion would be the interface making a false statement about what it did.
	writeNote := func(next string) {
		for _, bb := range p.app.Budgets() {
			if bb.ID != p.props.BudgetID {
				continue
			}
			had := strings.TrimSpace(bb.Notes) != ""
			bb.Notes = next
			if err := p.app.PutBudget(bb); err != nil {
				p.errS.Set(budgetErrorText(err))
				return
			}
			// Writing through the store updates memory; RequestPersist is what puts
			// it in the dataset. Without it the note survives only until a reload.
			uistate.RequestPersist()
			uistate.BumpDataRevision()
			// Say so at the moment of the click. The row does change, but the user
			// is looking at the modal, and a modal that just closes is what made a
			// working save read as a broken one. The budget is named because notes
			// are per-budget and several get written in a row.
			// Report what actually changed. Opening the editor on a budget with no
			// note and closing it with Save leaves an empty note empty — announcing
			// "Note removed" there would claim a deletion that never happened, which
			// is the same class of untruth as the "Note saved" this replaced.
			switch title := budgetTitle(bb.Name, budgetCategoryName(p.app, bb.CategoryID)); {
			case next == "" && had:
				uistate.PostNotice(uistate.T("budgets.notesRemoved", title), false)
			case next != "":
				uistate.PostNotice(uistate.T("budgets.notesSaved", title), false)
			}
			p.done()
			return
		}
		// A save that matched nothing used to close the editor as though it had
		// worked: the note simply never appeared on the card, with no error to
		// explain it. Silence is the worst outcome for a write — say so and stay
		// open so the text is not lost.
		p.errS.Set(uistate.T("budgets.notesSaveFailed"))
	}

	// confirmRemoval asks before discarding text that cannot be recovered. Read
	// fresh from the store rather than from the budget captured at render, so the
	// question is asked about what is actually there.
	confirmRemoval := func(then func()) {
		title := ""
		for _, bb := range p.app.Budgets() {
			if bb.ID == p.props.BudgetID {
				title = budgetTitle(bb.Name, budgetCategoryName(p.app, bb.CategoryID))
			}
		}
		uistate.ConfirmModal(uistate.T("budgets.notesRemoveConfirm", title), true, func(ok bool) {
			if ok {
				then()
			}
		})
	}

	// noteExists reports whether there is currently a note to lose.
	noteExists := func() bool {
		for _, bb := range p.app.Budgets() {
			if bb.ID == p.props.BudgetID {
				return strings.TrimSpace(bb.Notes) != ""
			}
		}
		return false
	}

	saveNotes := ui.UseEvent(Prevent(func() {
		if p.app == nil {
			p.done()
			return
		}
		next := strings.TrimSpace(p.notesS.Get())
		// Emptying the box and pressing Save destroys the note exactly as the
		// Remove button does — and it is the quicker, more natural gesture for
		// anyone already editing the text. Guarding only the button would leave
		// the easier path as the unguarded one.
		if next == "" && noteExists() {
			confirmRemoval(func() { writeNote("") })
			return
		}
		writeNote(next)
	}))

	// removeNote clears the note after confirming. Emptying the field and pressing
	// Save does the same thing, but that is a destructive gesture with no name —
	// this is the named one.
	removeNote := ui.UseEvent(Prevent(func() {
		if p.app == nil {
			p.done()
			return
		}
		confirmRemoval(func() { writeNote("") })
	}))

	return saveNotes, removeNote
}

// budgetEditNotesProps carries the explicit inputs for budgetEditNotes.
type budgetEditNotesProps struct {
	b          domain.Budget
	removeNote ui.Handler
	saveNotes  ui.Handler
	app        *appstate.App
	notesS     ui.State[string]
	onNotes    func(v string)
	errLine    ui.Node
	cancel     ui.Handler
}

// budgetEditNotes builds the edit notes section.
func budgetEditNotes(p budgetEditNotesProps) ui.Node {
	// The visible "Notes" label is gone: the modal title and the hint under it
	// already say what this field is, so a third label repeated the same word a
	// third time. The accessible name is carried on the field itself, so nothing
	// is lost for a screen reader.
	//
	// Removing a note is an explicit control, offered only when there IS one,
	// and separated from Cancel/Save so it cannot be hit while reaching for the
	// primary action. It confirms first, because the text is not recoverable.
	hasNote := strings.TrimSpace(p.b.Notes) != ""
	var removeBtn ui.Node = Fragment()
	if hasNote {
		// btn-link WITHOUT btn: `.btn` is registered after `.btn-link` in the
		// generated sheet and overrides every property they share (border,
		// padding, background, min-height), so "btn btn-link" rendered a
		// full-weight button that merely happened to be red — the loudest
		// thing in the footer, for the action that should be the quietest.
		// The danger tone is a decision; that weight was a cascade accident.
		removeBtn = Button(css.Class("btn-link budget-notes-remove"), Type("button"),
			Attr("data-testid", "budget-notes-remove"), OnClick(p.removeNote),
			uistate.T("budgets.notesRemove"))
	}
	return Form(css.Class("acct-edit-form", "budget-notes-form"), OnSubmit(p.saveNotes),
		Div(css.Class("modal-scroll", "budget-notes-scroll"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0"}),
				uistate.T("budgets.notesHint", budgetTitle(p.b.Name, budgetCategoryName(p.app, p.b.CategoryID)))),
			uiw.TextAreaInput(uiw.TextFieldProps{Value: p.notesS.Get(), Placeholder: uistate.T("budgets.notesPlaceholder"),
				AriaLabel: uistate.T("budgets.notesLabel"), OnInput: p.onNotes}),
			p.errLine,
		),
		Div(css.Class("modal-foot", "budget-notes-foot"),
			removeBtn,
			Button(css.Class("btn"), Type("button"), OnClick(p.cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "budget-notes-save"),
				OnClick(p.saveNotes), uistate.T("action.save")),
		),
	)
}
