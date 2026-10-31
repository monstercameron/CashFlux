// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/institution"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// This file is the institution directory: a small contact list answering "who do I
// bank with". Each institution is a real entity (domain.Institution), so multi-
// institution analysis does not depend on matching the free-text Account.Institution
// string. Account rows are colored by their institution, and the emergency pack
// lists the institutions' support contacts.

// sortedInstitutions returns the household's institutions sorted by display name, so
// the manager list and the edit-form picker present a stable, predictable order.
func sortedInstitutions(app *appstate.App) []domain.Institution {
	if app == nil {
		return nil
	}
	insts := app.Institutions()
	institution.SortByName(insts)
	return insts
}

// institutionChip renders a small colored tag with the institution's name, shown on
// an account row so its institution reads at a glance without opening the editor.
// Returns Fragment() when instID does not resolve.
func institutionChip(instByID map[string]domain.Institution, instID string) ui.Node {
	in, ok := instByID[instID]
	if !ok {
		return Fragment()
	}
	return Span(css.Class("inst-chip", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
		Attr("data-testid", "inst-chip-"+instID),
		AriaLabel(uistate.T("accounts.institutionChipAria", in.DisplayName(uistate.T("accounts.institutionUntitled")))),
		Span(css.Class("inst-dot"), Attr("style", "--inst-color: "+institution.SwatchColor(in))),
		Span(css.Class(tw.TextDim), in.DisplayName(uistate.T("accounts.institutionUntitled"))),
	)
}

// ── manager modal: list view ──────────────────────────────────────────────────────

// InstitutionsManagerFormProps configures the institution-directory flip modal.
type InstitutionsManagerFormProps struct {
	OnDone func()
}

// InstitutionsManagerForm is the institution-directory modal body: a list of every
// institution (name, color, account count) with an Edit button, plus "Add
// institution". Editing or adding swaps in institutionEditForm, keyed on the target
// id so it remounts fresh (its state seeds from the target). This component stays
// mounted while the modal is open, so editingID is the only state that carries
// across the swap.
func InstitutionsManagerForm(props InstitutionsManagerFormProps) ui.Node {
	editingID := ui.UseState("") // "" = list view; "new" or an institution id = edit sub-form
	startNew := ui.UseEvent(Prevent(func() { editingID.Set("new") }))
	finish := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	if id := editingID.Get(); id != "" {
		closeSub := func() { editingID.Set("") }
		return WithKey(
			ui.CreateElement(institutionEditForm, institutionEditFormProps{ID: id, OnDone: closeSub}),
			id,
		)
	}

	app := appstate.Default
	insts := sortedInstitutions(app)
	accountCounts := map[string]int{}
	if app != nil {
		for _, a := range app.Accounts() {
			if a.InstitutionID != "" {
				accountCounts[a.InstitutionID]++
			}
		}
	}

	startEdit := func(instID string) { editingID.Set(instID) }

	rows := MapKeyed(insts, func(in domain.Institution) any { return in.ID }, func(in domain.Institution) ui.Node {
		return ui.CreateElement(institutionListRow, institutionListRowProps{
			Inst: in, AccountCount: accountCounts[in.ID], OnEdit: startEdit,
		})
	})

	return Div(css.Class("inv-pool-modal"),
		Div(css.Class("acct-callout-gap"), uiw.Callout(uiw.CalloutProps{Body: Span(uistate.T("accounts.institutionsHint"))})),
		If(len(insts) == 0, uiw.Callout(uiw.CalloutProps{Body: Span(uistate.T("accounts.institutionsEmpty"))})),
		Div(css.Class("rows"), rows),
		Div(css.Class(tw.Mt3, tw.Flex, tw.Gap2),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "institution-add"), OnClick(startNew), uistate.T("accounts.addInstitution")),
			If(props.OnDone != nil, Button(css.Class("btn"), Type("button"), Attr("data-testid", "institution-done"), OnClick(finish), uistate.T("action.done"))),
		),
	)
}

type institutionListRowProps struct {
	Inst         domain.Institution
	AccountCount int
	OnEdit       func(string) // plain func — never an On* hook — safe inside MapKeyed
}

// institutionListRow is one institution row in the directory list. It is its own
// component so the Edit click hook keeps a stable position in a variable-length list.
func institutionListRow(props institutionListRowProps) ui.Node {
	in := props.Inst
	edit := ui.UseEvent(Prevent(func() { props.OnEdit(in.ID) }))
	var meta []string
	if props.AccountCount > 0 {
		meta = append(meta, uistate.TN("accounts.institutionLinkedOne", "accounts.institutionLinkedMany", props.AccountCount))
	}
	if phone := strings.TrimSpace(in.SupportPhone); phone != "" {
		meta = append(meta, in.SupportPhone)
	}
	return Div(css.Class("acct-entity"), Attr("data-testid", "institution-row-"+in.ID),
		uiw.EntityRow(uiw.EntityRowProps{
			Leading: Span(css.Class("inst-dot inst-dot-lg"), Attr("style", "--inst-color: "+institution.SwatchColor(in))),
			Title:   in.DisplayName(uistate.T("accounts.institutionUntitled")),
			Meta:    meta,
			Actions: []ui.Node{uiw.ToolButton(uiw.ToolButtonProps{
				Icon: icon.Pencil, Title: uistate.T("accounts.editInstitution"),
				AriaLabel: uistate.T("accounts.editInstitutionAria", in.DisplayName(uistate.T("accounts.institutionUntitled"))),
				TestID:    "institution-edit-" + in.ID,
			}, OnClick(edit))},
		}),
	)
}

// ── manager modal: add/edit sub-form ──────────────────────────────────────────────

type institutionEditFormProps struct {
	ID     string // "new" to create, else the institution id to edit
	OnDone func()
}

// institutionEditForm is the add/edit fields for one institution: name, color,
// support phone/URL, and a free-text note. Deleting removes the institution and
// reassigns (clears InstitutionID on) every account that referenced it — the
// appstate layer already handles the reassign-on-delete.
func institutionEditForm(props institutionEditFormProps) ui.Node {
	app := appstate.Default
	isNew := props.ID == "" || props.ID == "new"
	var existing domain.Institution
	// found is false when the institution being edited no longer exists, for example because it was
	// deleted in another tab. Saving or deleting then reports that instead of creating or deleting
	// something else.
	found := isNew
	if !isNew && app != nil {
		for _, in := range app.Institutions() {
			if in.ID == props.ID {
				existing = in
				found = true
				break
			}
		}
	}

	nameS := ui.UseState(existing.Name)
	colorS := ui.UseState(existing.Color)
	phoneS := ui.UseState(existing.SupportPhone)
	urlS := ui.UseState(existing.SupportURL)
	noteS := ui.UseState(existing.Note)
	errS := ui.UseState("")

	onName := func(v string) { nameS.Set(v) }
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	onPhone := ui.UseEvent(func(v string) { phoneS.Set(v) })
	onURL := ui.UseEvent(func(v string) { urlS.Set(v) })
	onNote := func(v string) { noteS.Set(v) }

	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		if !found {
			errS.Set(uistate.T("accounts.institutionGone"))
			return
		}
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			errS.Set(uistate.T("accounts.institutionNameRequired"))
			return
		}
		color, err := institution.NormalizeColor(colorS.Get())
		if err != nil {
			slog.Warn("institution colour rejected", "institution", existing.ID, "error", err)
			errS.Set(uistate.T("accounts.institutionColorInvalid"))
			return
		}
		in := existing
		in.Name = name
		in.Color = color
		in.SupportPhone = strings.TrimSpace(phoneS.Get())
		in.SupportURL = strings.TrimSpace(urlS.Get())
		in.Note = strings.TrimSpace(noteS.Get())
		if in.ID == "" {
			in.ID = id.New()
		}
		if err := app.PutInstitution(in); err != nil {
			slog.Error("save institution", "institution", in.ID, "error", err)
			errS.Set(acctWriteErrorText(err))
			return
		}
		dataEditSaved()
		uistate.PostNotice(uistate.T("accounts.institutionSaved"), false)
		done()
	}))

	del := ui.UseEvent(Prevent(func() {
		if app == nil || isNew {
			return
		}
		if !found {
			errS.Set(uistate.T("accounts.institutionGone"))
			return
		}
		name := existing.DisplayName(uistate.T("accounts.institutionUntitled"))
		uistate.ConfirmModal(uistate.T("accounts.institutionDeleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteInstitution(existing.ID); err != nil {
				postAcctWriteError("delete institution", err)
				return
			}
			dataEditSaved()
			uistate.PostNotice(uistate.T("accounts.institutionDeleted", name), false)
			done()
		})
	}))

	cancel := ui.UseEvent(Prevent(func() { done() }))

	title := uistate.T("accounts.addInstitution")
	if !isNew {
		title = uistate.T("accounts.editInstitution")
	}

	deleteBtn := If(!isNew, Button(css.Class("btn btn-sm danger"), Type("button"), Attr("data-testid", "institution-delete"),
		OnClick(del), uistate.T("accounts.deleteInstitution")))

	swatch := colorS.Get()
	if swatch == "" {
		swatch = institution.DefaultSwatchColor
	}

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "institution-edit-form"), OnSubmit(save),
		Div(css.Class("modal-scroll"),
			H4(css.Class("set-label"), title),
			uiw.FormField(uistate.T("accounts.institutionNameLabel"),
				uiw.TextInput(uiw.TextFieldProps{
					Value: nameS.Get(), Placeholder: uistate.T("accounts.institutionNamePh"), OnInput: onName,
					Extra: []any{Attr("id", "inst-name"), Attr("autofocus", ""), Attr("data-testid", "institution-name")},
				})),
			uiw.FormField(uistate.T("accounts.institutionColorLabel"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(css.Class("color-input"), Type("color"), Attr("data-testid", "institution-color"),
						AriaLabel(uistate.T("accounts.institutionColorLabel")), OnInput(onColor), uiw.FieldValue(swatch)))),
			uiw.FormField(uistate.T("accounts.institutionPhoneLabel"),
				Input(css.Class("field"), Type("tel"), Attr("data-testid", "institution-phone"),
					Placeholder(uistate.T("accounts.institutionPhonePh")), OnInput(onPhone), uiw.FieldValue(phoneS.Get()))),
			uiw.FormField(uistate.T("accounts.institutionURLLabel"),
				Input(css.Class("field"), Type("url"), Attr("data-testid", "institution-url"),
					Placeholder(uistate.T("accounts.institutionURLPh")), OnInput(onURL), uiw.FieldValue(urlS.Get()))),
			uiw.FormField(uistate.T("accounts.institutionNoteLabel"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: noteS.Get(), Placeholder: uistate.T("accounts.institutionNotePh"),
					AriaLabel: uistate.T("accounts.institutionNoteLabel"), OnInput: onNote})),
			If(errS.Get() != "", uiw.Callout(uiw.CalloutProps{Tone: uiw.CalloutDanger, Live: uiw.CalloutLiveAlert, Body: Span(errS.Get())})),
		),
		Div(css.Class("modal-foot"),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			deleteBtn,
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "institution-save"), uistate.T("action.save")),
		),
	)
}

// institutionPickerOptions builds the edit-form's institution picker options: a
// "No institution" placeholder followed by every institution sorted by name.
func institutionPickerOptions(app *appstate.App) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("accounts.institutionNone")}}
	for _, in := range sortedInstitutions(app) {
		opts = append(opts, uiw.SelectOption{Value: in.ID, Label: in.DisplayName(uistate.T("accounts.institutionUntitled"))})
	}
	return opts
}
