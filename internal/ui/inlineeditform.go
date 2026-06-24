//go:build js && wasm

package ui

import (
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ---------------------------------------------------------------------------
// InlineEditForm
// ---------------------------------------------------------------------------

// InlineEditFormProps configures an InlineEditForm.
type InlineEditFormProps struct {
	// Fields are the form controls rendered inside the .form-grid.
	// Build each field with FormField(label, control) or pass raw nodes.
	Fields []uic.Node
	// OnSave is called when the form is submitted (Enter or Save button click).
	// May be nil; the submit event is still prevented from bubbling.
	OnSave func()
	// OnCancel is called when the Cancel button is activated.
	// May be nil; the cancel button still renders.
	OnCancel func()
	// SaveLabel is the text of the Save / submit button. Defaults to "Save".
	SaveLabel string
	// CancelLabel is the text of the Cancel button. Defaults to "Cancel".
	CancelLabel string
	// AriaLabel is passed as aria-label on the <form>. Useful when multiple
	// inline forms appear on the same page (screen-reader orientation).
	AriaLabel string
	// TestID is an optional data-testid on the wrapping .row-edit div.
	TestID string
	// ExtraContent is rendered below the Save/Cancel buttons. Use this for
	// status messages, delta displays, or contextual help nodes.
	ExtraContent uic.Node
}

// InlineEditForm renders the standard per-row edit chrome: a `.row-edit`
// wrapper containing a `.form-grid` <form> with Save and Cancel buttons.
// It unifies the `Div(.row-edit) + Form(.form-grid)` scaffold repeated on
// every CRUD screen (accounts, categories, members, budgets, documents, …)
// so callers supply only their fields and callbacks.
//
// InlineEditForm is its own component so its OnSubmit hook stays at a stable
// render position regardless of where the form appears.
func InlineEditForm(props InlineEditFormProps) uic.Node {
	return uic.CreateElement(inlineEditForm, props)
}

func inlineEditForm(props InlineEditFormProps) uic.Node {
	// On mount, focus the first editable field so the keyboard path starts inside
	// the edit chrome (§6.7). Inline edit is one-at-a-time per screen, so the first
	// `.row-edit` field on the page is the just-opened one.
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		el := doc.Call("querySelector", ".row-edit input:not([type=hidden]), .row-edit select, .row-edit textarea")
		if !el.IsNull() && !el.IsUndefined() {
			el.Call("focus")
		}
		return nil
	}, []any{})

	saveLabel := props.SaveLabel
	if saveLabel == "" {
		saveLabel = "Save"
	}
	cancelLabel := props.CancelLabel
	if cancelLabel == "" {
		cancelLabel = "Cancel"
	}
	onSave := props.OnSave
	onCancel := props.OnCancel

	formArgs := []any{css.Class("form-grid")}
	if props.AriaLabel != "" {
		formArgs = append(formArgs, Attr("aria-label", props.AriaLabel))
	}
	formArgs = append(formArgs, OnSubmit(Prevent(func() {
		if onSave != nil {
			onSave()
		}
	})))
	for _, f := range props.Fields {
		formArgs = append(formArgs, f)
	}
	formArgs = append(formArgs,
		Button(css.Class("btn btn-primary"), Type("submit"), saveLabel),
		Button(css.Class("btn"), Type("button"), OnClick(func() {
			if onCancel != nil {
				onCancel()
			}
		}), cancelLabel),
		// Discoverability hint for the keyboard path (§6.6).
		Span(css.Class("kbd-hint"), "Enter to save · Esc to cancel"),
	)
	if props.ExtraContent != nil {
		formArgs = append(formArgs, props.ExtraContent)
	}

	wrapArgs := []any{css.Class("row-edit")}
	if props.TestID != "" {
		wrapArgs = append(wrapArgs, Attr("data-testid", props.TestID))
	}
	wrapArgs = append(wrapArgs, Form(formArgs...))
	return Div(wrapArgs...)
}
