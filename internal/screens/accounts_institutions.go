// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// accounts_institutions.go is the AC10 institution directory: a lightweight CRM for
// "who do I bank with" that grounds the ★★ Multi-Institution Analytics feature with a
// real entity (domain.Institution) instead of matching on the free-text
// Account.Institution string. It also colors account rows by their institution and
// feeds the AC16 emergency pack's support-contact roll-up.

// sortedInstitutions returns the household's institutions sorted by display name, so
// the manager list and the edit-form picker present a stable, predictable order.
func sortedInstitutions(app *appstate.App) []domain.Institution {
	if app == nil {
		return nil
	}
	insts := app.Institutions()
	sort.SliceStable(insts, func(i, j int) bool {
		return strings.ToLower(insts[i].TrimmedName()) < strings.ToLower(insts[j].TrimmedName())
	})
	return insts
}

// institutionSwatchColor returns the color to render for an institution chip/stripe,
// falling back to a calm neutral accent when the institution has none set.
func institutionSwatchColor(in domain.Institution) string {
	if c := strings.TrimSpace(in.Color); c != "" {
		return c
	}
	return "#7c83ff"
}

// institutionChip renders a small colored tag with the institution's name — used on
// the account row (AC10) so an account's institution reads at a glance without
// opening the editor. Returns Fragment() when instID does not resolve.
func institutionChip(instByID map[string]domain.Institution, instID string) ui.Node {
	in, ok := instByID[instID]
	if !ok {
		return Fragment()
	}
	return Span(css.Class("inst-chip", tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
		Attr("data-testid", "inst-chip-"+instID),
		Attr("aria-label", uistate.T("accounts.institutionChipAria", in.TrimmedName())),
		Span(Style(map[string]string{
			"width": "8px", "height": "8px", "border-radius": "50%",
			"background": institutionSwatchColor(in), "display": "inline-block", "flex-shrink": "0",
		})),
		Span(css.Class(tw.TextDim), in.TrimmedName()),
	)
}

// ── manager modal: list view ──────────────────────────────────────────────────────

// InstitutionsManagerFormProps configures the institution-directory flip modal.
type InstitutionsManagerFormProps struct {
	OnDone func()
}

// InstitutionsManagerForm is the AC10 institution-directory modal body: a list of
// every institution (name, color, account count) with Edit/Delete, plus "Add
// institution". Editing/adding swaps in institutionEditForm, keyed on the target id
// so it remounts fresh (its useState seeds from the target) each time the selection
// changes — this component itself never unmounts while the modal is open, so its own
// editingID toggle state is the only thing that needs to persist across that swap.
func InstitutionsManagerForm(props InstitutionsManagerFormProps) ui.Node {
	editingID := ui.UseState("") // "" = list view; "new" or an institution id = edit sub-form

	done := props.OnDone
	if done == nil {
		done = func() {}
	}

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

	startNew := ui.UseEvent(Prevent(func() { editingID.Set("new") }))
	startEdit := func(instID string) { editingID.Set(instID) }

	rows := MapKeyed(insts, func(in domain.Institution) any { return in.ID }, func(in domain.Institution) ui.Node {
		return ui.CreateElement(institutionListRow, institutionListRowProps{
			Inst: in, AccountCount: accountCounts[in.ID], OnEdit: startEdit,
		})
	})

	return Div(css.Class("inv-pool-modal"),
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.75rem"}), uistate.T("accounts.institutionsHint")),
		If(len(insts) == 0, P(css.Class("empty"), uistate.T("accounts.institutionsEmpty"))),
		Div(css.Class("rows"), rows),
		Div(css.Class(tw.Mt3),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "institution-add"), OnClick(startNew), uistate.T("accounts.addInstitution")),
		),
	)
}

type institutionListRowProps struct {
	Inst         domain.Institution
	AccountCount int
	OnEdit       func(string) // plain func — never an On* hook — safe inside MapKeyed
}

// institutionListRow is one institution row in the directory list. Its own component
// so the Edit click hook stays stable across a variable-length institution list
// (CLAUDE.md §gotchas — never an On* hook inside a loop).
func institutionListRow(props institutionListRowProps) ui.Node {
	in := props.Inst
	edit := ui.UseEvent(Prevent(func() { props.OnEdit(in.ID) }))
	return Div(css.Class("row"), Attr("data-testid", "institution-row-"+in.ID),
		Span(Style(map[string]string{
			"width": "10px", "height": "10px", "border-radius": "50%",
			"background": institutionSwatchColor(in), "flex-shrink": "0",
		})),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), in.TrimmedName()),
			If(props.AccountCount > 0, Span(css.Class("row-meta"), uistate.T("accounts.institutionAccountCount", plural(props.AccountCount, "account")))),
			If(strings.TrimSpace(in.SupportPhone) != "", Span(css.Class("row-meta"), in.SupportPhone)),
		),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "institution-edit-"+in.ID),
			Attr("aria-label", uistate.T("accounts.editInstitution")), Title(uistate.T("accounts.editInstitution")), OnClick(edit),
			uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4))),
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
	if !isNew && app != nil {
		for _, in := range app.Institutions() {
			if in.ID == props.ID {
				existing = in
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

	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	onPhone := ui.UseEvent(func(v string) { phoneS.Set(v) })
	onURL := ui.UseEvent(func(v string) { urlS.Set(v) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })

	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	save := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		name := strings.TrimSpace(nameS.Get())
		if name == "" {
			errS.Set(uistate.T("accounts.institutionNameRequired"))
			return
		}
		in := existing
		in.Name = name
		in.Color = strings.TrimSpace(colorS.Get())
		in.SupportPhone = strings.TrimSpace(phoneS.Get())
		in.SupportURL = strings.TrimSpace(urlS.Get())
		in.Note = strings.TrimSpace(noteS.Get())
		if in.ID == "" {
			in.ID = id.New()
		}
		if err := app.PutInstitution(in); err != nil {
			errS.Set(err.Error())
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
		name := existing.TrimmedName()
		uistate.ConfirmModal(uistate.T("accounts.institutionDeleteConfirm", name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteInstitution(existing.ID); err != nil {
				uistate.PostNotice(err.Error(), true)
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

	var deleteBtn ui.Node = Fragment()
	if !isNew {
		deleteBtn = Button(css.Class("btn btn-sm danger"), Type("button"), Attr("data-testid", "institution-delete"),
			OnClick(del), uistate.T("accounts.deleteInstitution"))
	}

	swatch := colorS.Get()
	if swatch == "" {
		swatch = "#7c83ff"
	}

	return Form(css.Class("acct-edit-form"), Attr("data-testid", "institution-edit-form"), OnSubmit(save),
		Div(css.Class("modal-scroll"),
			H4(css.Class("set-label"), title),
			labeledField(uistate.T("accounts.institutionNameLabel"),
				Input(css.Class("field"), Attr("id", "inst-name"), Attr("autofocus", ""), Attr("data-testid", "institution-name"),
					Type("text"), Placeholder(uistate.T("accounts.institutionNamePh")), Value(nameS.Get()), OnInput(onName))),
			labeledField(uistate.T("accounts.institutionColorLabel"),
				Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
					Input(css.Class("color-input"), Type("color"), Attr("data-testid", "institution-color"),
						Attr("aria-label", uistate.T("accounts.institutionColorLabel")), Value(swatch), OnInput(onColor)))),
			labeledField(uistate.T("accounts.institutionPhoneLabel"),
				Input(css.Class("field"), Type("tel"), Attr("data-testid", "institution-phone"),
					Placeholder(uistate.T("accounts.institutionPhonePh")), Value(phoneS.Get()), OnInput(onPhone))),
			labeledField(uistate.T("accounts.institutionURLLabel"),
				Input(css.Class("field"), Type("url"), Attr("data-testid", "institution-url"),
					Placeholder(uistate.T("accounts.institutionURLPh")), Value(urlS.Get()), OnInput(onURL))),
			labeledField(uistate.T("accounts.institutionNoteLabel"),
				uiw.TextAreaInput(uiw.TextFieldProps{Value: noteS.Get(), Placeholder: uistate.T("accounts.institutionNotePh"),
					AriaLabel: uistate.T("accounts.institutionNoteLabel"), OnInput: onNote})),
			If(errS.Get() != "", P(css.Class("err"), Attr("role", "alert"), errS.Get())),
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
		opts = append(opts, uiw.SelectOption{Value: in.ID, Label: in.TrimmedName()})
	}
	return opts
}
