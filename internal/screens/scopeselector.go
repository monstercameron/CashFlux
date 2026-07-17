// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"slices"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// ── Sub-components ────────────────────────────────────────────────────────────
//
// Each interactive element in a variable-length list must be its own component
// so ui.UseEvent hooks are registered at a stable (non-loop) position in the
// hook chain. The parent ScopeSelector passes a plain func() as OnToggle; the
// sub-component wraps it with UseEvent.

// scopeChipProps carries the display + interaction data for one multi-select chip.
type scopeChipProps struct {
	Label    string
	Selected bool
	OnToggle func()
}

// scopeChip renders a toggleable chip button. UseEvent is its only hook (hook 1),
// registered unconditionally on every render.
func scopeChip(p scopeChipProps) ui.Node {
	click := ui.UseEvent(func() { p.OnToggle() })
	cls := "scope-chip"
	if p.Selected {
		cls += " scope-chip-on"
	}
	sel := "false"
	if p.Selected {
		sel = "true"
	}
	return Button(
		css.Class(cls),
		Type("button"),
		Attr("aria-pressed", sel),
		OnClick(click),
		p.Label,
	)
}

// scopeAcctRowProps carries the display + interaction data for one account row
// in the collapsible individual-account checklist.
type scopeAcctRowProps struct {
	AccountID string
	Name      string
	Checked   bool
	OnToggle  func()
}

// scopeAcctRow renders a labelled checkbox row for an individual account.
// UseEvent is its only hook (hook 1).
func scopeAcctRow(p scopeAcctRowProps) ui.Node {
	click := ui.UseEvent(func() { p.OnToggle() })
	return Label(css.Class("scope-acct-row"),
		Input(
			Type("checkbox"),
			Checked(p.Checked),
			Attr("aria-label", p.Name),
			OnClick(click),
		),
		" ",
		p.Name,
	)
}

// ── ScopeSelector ─────────────────────────────────────────────────────────────

// ScopeSelector renders the multi-dimension filter panel for /reports (#444).
// It exposes chips for Institutions, Owners (household members + Shared), and
// Account Types, a collapsible individual-account checklist, and a saved-views
// control. Every state change calls uistate.SetReportScope preserving dimensions
// that were not touched.
//
// Hook order (stable, unconditional):
//  1. uistate.UseReportScope()
//  2. ui.UseState — showSave
//  3. ui.UseState — saveName
//  4. ui.UseState — showAccts
//  5. ui.UseState — selectedSV
//  6. ui.UseEvent — clearAll
//  7. ui.UseEvent — openSave
//  8. ui.UseEvent — cancelSave
//  9. ui.UseEvent — toggleAccts
func ScopeSelector() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}

	scopeAtom := uistate.UseReportScope() // hook 1
	sc := scopeAtom.Get()

	showSave := ui.UseState(false)  // hook 2
	saveName := ui.UseState("")     // hook 3
	showAccts := ui.UseState(false) // hook 4
	selectedSV := ui.UseState("")   // hook 5: selected saved-view ID in the dropdown

	clearAll := ui.UseEvent(func() { // hook 6
		uistate.SetReportScope(scope.ReportScope{})
	})
	openSave := ui.UseEvent(func() { // hook 7
		showSave.Set(true)
		saveName.Set("")
	})
	cancelSave := ui.UseEvent(func() { // hook 8
		showSave.Set(false)
		saveName.Set("")
	})
	toggleAccts := ui.UseEvent(func() { // hook 9
		showAccts.Set(!showAccts.Get())
	})

	// ── Data ─────────────────────────────────────────────────────────────────
	accounts := app.Accounts()
	members := app.Members()
	savedViews := app.SavedViews()

	// Sorted unique institution names from the live account list.
	insts := domain.UniqueInstitutions(accounts)

	// ── Mutation helpers (non-hook closures, safe to define freely) ───────────

	// toggleInstitution adds or removes inst from the scope.Institutions dimension.
	toggleInstitution := func(inst string) {
		cur := scopeAtom.Get()
		cur.Institutions = toggleStringSlice(cur.Institutions, inst)
		uistate.SetReportScope(cur)
	}

	// toggleOwner adds or removes ownerID from the scope.Owners dimension.
	toggleOwner := func(ownerID string) {
		cur := scopeAtom.Get()
		cur.Owners = toggleStringSlice(cur.Owners, ownerID)
		uistate.SetReportScope(cur)
	}

	// toggleType adds or removes t from the scope.Types dimension.
	toggleType := func(t domain.AccountType) {
		cur := scopeAtom.Get()
		cur.Types = toggleTypeSlice(cur.Types, t)
		uistate.SetReportScope(cur)
	}

	// toggleAccountID adds or removes acctID from the scope.AccountIDs dimension.
	toggleAccountID := func(acctID string) {
		cur := scopeAtom.Get()
		cur.AccountIDs = toggleStringSlice(cur.AccountIDs, acctID)
		uistate.SetReportScope(cur)
	}

	// ── Saved-view handlers (non-hook) ────────────────────────────────────────

	// svChange applies the saved view whose ID is selected in the dropdown.
	svChange := OnChange(func(svID string) {
		selectedSV.Set(svID)
		if svID == "" {
			return
		}
		for _, sv := range savedViews {
			if sv.ID == svID {
				uistate.SetReportScope(sv.Scope)
				return
			}
		}
	})

	// saveConfirm persists the current scope as a named saved view.
	saveConfirm := OnClick(func() {
		name := strings.TrimSpace(saveName.Get())
		if name == "" {
			return
		}
		sv := scope.SavedView{ID: id.New(), Name: name, Scope: scopeAtom.Get()}
		_ = app.PutSavedView(sv)
		showSave.Set(false)
		saveName.Set("")
	})

	// deleteView removes the currently selected saved view.
	deleteView := OnClick(func() {
		sel := selectedSV.Get()
		if sel == "" {
			return
		}
		_ = app.DeleteSavedView(sel)
		selectedSV.Set("")
	})

	// ── Institution chips ─────────────────────────────────────────────────────
	var instChips []ui.Node
	for _, inst := range insts {
		inst := inst // capture loop variable
		instChips = append(instChips, ui.CreateElement(scopeChip, scopeChipProps{
			Label:    inst,
			Selected: slices.Contains(sc.Institutions, inst),
			OnToggle: func() { toggleInstitution(inst) },
		}))
	}

	// ── Owner chips: members + "Shared" ──────────────────────────────────────
	var ownerChips []ui.Node
	// "Shared" chip for household-level accounts (GroupOwnerID).
	ownerChips = append(ownerChips, ui.CreateElement(scopeChip, scopeChipProps{
		Label:    uistate.T("scope.shared"),
		Selected: slices.Contains(sc.Owners, domain.GroupOwnerID),
		OnToggle: func() { toggleOwner(domain.GroupOwnerID) },
	}))
	for _, m := range members {
		m := m
		ownerChips = append(ownerChips, ui.CreateElement(scopeChip, scopeChipProps{
			Label:    m.Name,
			Selected: slices.Contains(sc.Owners, m.ID),
			OnToggle: func() { toggleOwner(m.ID) },
		}))
	}

	// ── Type chips ────────────────────────────────────────────────────────────
	var typeChips []ui.Node
	for _, t := range domain.AllAccountTypes {
		t := t
		typeChips = append(typeChips, ui.CreateElement(scopeChip, scopeChipProps{
			Label:    selectorTypeLabel(t),
			Selected: slices.Contains(sc.Types, t),
			OnToggle: func() { toggleType(t) },
		}))
	}

	// ── Individual account rows ───────────────────────────────────────────────
	var acctRows []ui.Node
	for _, a := range accounts {
		if a.Archived {
			continue
		}
		a := a
		acctRows = append(acctRows, ui.CreateElement(scopeAcctRow, scopeAcctRowProps{
			AccountID: a.ID,
			Name:      a.Name,
			Checked:   slices.Contains(sc.AccountIDs, a.ID),
			OnToggle:  func() { toggleAccountID(a.ID) },
		}))
	}

	// ── Saved-view option elements (plain HTML, no hooks) ─────────────────────
	var svOpts []ui.Node
	svOpts = append(svOpts, Option(Value(""), uistate.T("scope.savedViews.select")))
	for _, sv := range savedViews {
		svOpts = append(svOpts, Option(Value(sv.ID), SelectedIf(sv.ID == selectedSV.Get()), sv.Name))
	}

	// ── Layout ────────────────────────────────────────────────────────────────
	// The selector uses its own CSS classes (scope-*) so it never touches the
	// screenlint-tracked "rows" / "card" class baselines.

	clearBtn := If(!sc.IsAll(), Button(
		css.Class("scope-chip", "scope-chip-clear"),
		Type("button"),
		Attr("data-testid", "scope-selector-clear"),
		OnClick(clearAll),
		uistate.T("scope.viewAll"),
	))

	// "Save current as…" name-entry form (shown only when openSave clicked).
	saveForm := If(showSave.Get(),
		Span(css.Class("scope-save-form"),
			Input(
				css.Class("field"),
				Type("text"),
				Attr("placeholder", uistate.T("scope.savedViews.namePlaceholder")),
				Attr("aria-label", uistate.T("scope.savedViews.namePlaceholder")),
				OnInput(func(v string) { saveName.Set(v) }),
			),
			Button(css.Class("btn", "btn-sm"), Type("button"), saveConfirm,
				uistate.T("scope.savedViews.confirm")),
			Button(css.Class("btn", "btn-sm"), Type("button"), OnClick(cancelSave),
				uistate.T("scope.savedViews.cancel")),
		),
	)

	// Saved-views row: dropdown + save-as button + delete button.
	savedViewsRow := If(len(savedViews) > 0 || !sc.IsAll(),
		Div(css.Class("scope-row"),
			Span(css.Class("scope-label"), uistate.T("scope.savedViews")),
			Select(css.Class("field", "scope-sv-select"),
				Attr("aria-label", uistate.T("scope.savedViews")),
				svChange,
				svOpts,
			),
			If(!showSave.Get(),
				Button(css.Class("btn", "btn-sm"), Type("button"), OnClick(openSave),
					uistate.T("scope.savedViews.save")),
			),
			saveForm,
			If(selectedSV.Get() != "",
				Button(css.Class("btn", "btn-sm"), Type("button"),
					Attr("aria-label", uistate.T("scope.savedViews.delete")),
					deleteView,
					uistate.T("scope.savedViews.delete")),
			),
		),
	)

	// Individual accounts collapsible section.
	acctSection := Fragment()
	if len(acctRows) > 0 {
		acctToggleLabel := uistate.T("scope.showAccounts")
		acctSection = Div(css.Class("scope-row"),
			Button(css.Class("scope-chip"), Type("button"),
				Attr("aria-expanded", boolStr(showAccts.Get())),
				OnClick(toggleAccts),
				acctToggleLabel,
			),
			If(showAccts.Get(),
				Div(css.Class("scope-accts"), acctRows),
			),
		)
	}

	return Div(
		css.Class("scope-selector"),
		Attr("data-testid", "scope-selector"),
		// Institutions row (hidden when no institutions exist yet).
		If(len(instChips) > 0, Div(css.Class("scope-row"),
			Span(css.Class("scope-label"), uistate.T("scope.institutions")),
			Div(css.Class("scope-chips"), instChips),
		)),
		// Owners row.
		Div(css.Class("scope-row"),
			Span(css.Class("scope-label"), uistate.T("scope.owners")),
			Div(css.Class("scope-chips"), ownerChips),
		),
		// Types row.
		Div(css.Class("scope-row"),
			Span(css.Class("scope-label"), uistate.T("scope.types")),
			Div(css.Class("scope-chips"), typeChips),
		),
		// Individual accounts (collapsible).
		acctSection,
		// Saved views row.
		savedViewsRow,
		// Clear-all button (visible only when scope is non-empty).
		If(!sc.IsAll(), Div(css.Class("scope-row"), clearBtn)),
	)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// selectorTypeLabel converts a snake_case domain.AccountType to a Title Case
// human-readable label (e.g. "credit_card" → "Credit Card").
func selectorTypeLabel(t domain.AccountType) string {
	words := strings.Split(string(t), "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// toggleStringSlice returns a new slice with s toggled: removed if present,
// appended if absent. Returns nil (not []) when the result is empty so
// ReportScope.IsAll() works correctly.
func toggleStringSlice(ss []string, s string) []string {
	for i, v := range ss {
		if v == s {
			out := make([]string, 0, len(ss)-1)
			out = append(out, ss[:i]...)
			out = append(out, ss[i+1:]...)
			if len(out) == 0 {
				return nil
			}
			return out
		}
	}
	return append(ss, s)
}

// toggleTypeSlice returns a new slice with t toggled: removed if present,
// appended if absent. Returns nil when the result is empty.
func toggleTypeSlice(ts []domain.AccountType, t domain.AccountType) []domain.AccountType {
	for i, v := range ts {
		if v == t {
			out := make([]domain.AccountType, 0, len(ts)-1)
			out = append(out, ts[:i]...)
			out = append(out, ts[i+1:]...)
			if len(out) == 0 {
				return nil
			}
			return out
		}
	}
	return append(ts, t)
}
