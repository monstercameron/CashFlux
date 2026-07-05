// SPDX-License-Identifier: MIT

//go:build js && wasm

// entityvar is the reusable "variable name" library shared by every entity that can be
// referenced in the formula/widget engine (budgets, accounts, …). It bundles the whole
// strategy in one place so each screen stays a thin caller:
//
//   - useEntityVarField — a hook that owns the var-name state, the "touched" flag, and
//     the autosuggest-from-name behaviour (typing the entity's name fills a slug into the
//     variable field until the user edits it), plus the name/var-name input handlers.
//   - entityVarField — the renderer: the input, a live chip showing the exact variable
//     the entity generates (e.g. account_checking_balance), and a collision warning.
//   - entityVarCollision — a warning when the resolved slug clashes with a sibling's.
//
// A new entity type only needs to define an entityVarKind and pass its siblings in.
package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// entityVarKind bundles the per-entity specifics of the variable surface: the variable
// prefix ("budget"/"account"), the slugger (must match what engineenv resolves), the
// field shown in the preview chip, and the i18n keys for the placeholder / fields hint /
// collision message.
type entityVarKind struct {
	Prefix         string              // e.g. "account" → account_<slug>_*
	Slug           func(string) string // must equal engineenv's slugger for this entity
	ChipField      string              // the suffix shown in the chip, e.g. "balance"
	FieldsHintKey  string              // i18n key: "· also _cleared"
	PlaceholderKey string              // i18n key for the empty-field placeholder
	TakenKey       string              // i18n key for the collision message (one %s = sibling name)
}

var budgetVarKind = entityVarKind{
	Prefix: "budget", Slug: engineenv.BudgetVarSlug, ChipField: "remaining",
	FieldsHintKey: "budgets.varNameFields", PlaceholderKey: "budgets.varNamePlaceholder", TakenKey: "budgets.varNameTaken",
}

var accountVarKind = entityVarKind{
	Prefix: "account", Slug: engineenv.AccountVarSlug, ChipField: "balance",
	FieldsHintKey: "accounts.varNameFields", PlaceholderKey: "accounts.varNamePlaceholder", TakenKey: "accounts.varNameTaken",
}

// varEntity is a minimal view of a sibling entity for collision checks.
type varEntity struct {
	ID      string
	Name    string
	VarName string
}

func budgetVarEntities(bs []domain.Budget) []varEntity {
	out := make([]varEntity, len(bs))
	for i, b := range bs {
		out[i] = varEntity{ID: b.ID, Name: b.Name, VarName: b.VarName}
	}
	return out
}

func accountVarEntities(as []domain.Account) []varEntity {
	out := make([]varEntity, len(as))
	for i, a := range as {
		out[i] = varEntity{ID: a.ID, Name: a.Name, VarName: a.VarName}
	}
	return out
}

// entityVarState is what useEntityVarField hands back: the live variable-name value plus
// the input handlers to wire onto the name field and the var-name field, and a Reset for
// add forms to clear both the value and the touched flag after a successful add.
type entityVarState struct {
	VarName   ui.State[string]
	touched   ui.State[bool]
	OnName    ui.Handler // wire onto the entity's NAME input (sets name + autosuggests the slug)
	OnVarName ui.Handler // wire onto the var-name input (marks touched, records the value)
}

// Reset clears the var name and the touched flag (so autosuggest resumes) — used by add
// forms after a successful submit.
func (s entityVarState) Reset() {
	s.VarName.Set("")
	s.touched.Set(false)
}

// useEntityVarField wires up the shared var-name behaviour for one form. Call it once,
// unconditionally, in the component (it registers hooks at stable positions). nameS is the
// form's own name state; this returns an OnName handler that BOTH updates nameS and — until
// the user edits the var field — keeps the variable field auto-filled with the slug of the
// name. initialVar seeds the field (an existing entity's VarName); a non-empty seed counts
// as "touched" so a saved handle isn't clobbered by a later rename.
func useEntityVarField(kind entityVarKind, nameS ui.State[string], initialVar string) entityVarState {
	varS := ui.UseState(initialVar)
	touchedS := ui.UseState(initialVar != "")
	onName := ui.UseEvent(func(v string) {
		nameS.Set(v)
		if !touchedS.Get() {
			varS.Set(kind.Slug(v))
		}
	})
	onVarName := ui.UseEvent(func(v string) {
		touchedS.Set(true)
		varS.Set(v)
	})
	return entityVarState{VarName: varS, touched: touchedS, OnName: onName, OnVarName: onVarName}
}

// entityVarBase is the base handle an entity exposes ("<prefix>_<slug>"), from the explicit
// var name when set else the display name.
func entityVarBase(kind entityVarKind, varName, name string) string {
	src := varName
	if src == "" {
		src = name
	}
	slug := kind.Slug(src)
	if slug == "" {
		slug = "…"
	}
	return kind.Prefix + "_" + slug
}

// entityVarPlaceholder is the auto-derived slug for a name, shown as the field placeholder.
func entityVarPlaceholder(kind entityVarKind, name string) string {
	if s := kind.Slug(name); s != "" {
		return s
	}
	return uistate.T(kind.PlaceholderKey)
}

// entityVarCollision returns a warning when the resolved slug for (varName else name)
// clashes with a sibling's variable — so two entities can't silently produce the same
// handle. Empty when there's no clash.
func entityVarCollision(kind entityVarKind, siblings []varEntity, selfID, varName, name string) string {
	src := varName
	if src == "" {
		src = name
	}
	slug := kind.Slug(src)
	if slug == "" {
		return ""
	}
	for _, e := range siblings {
		if e.ID == selfID {
			continue
		}
		other := e.VarName
		if other == "" {
			other = e.Name
		}
		if kind.Slug(other) == slug {
			return uistate.T(kind.TakenKey, e.Name)
		}
	}
	return ""
}

// entityVarField renders the shared editor: the input, a live chip showing the exact
// variable the entity generates, the extra fields it exposes, and a collision warning.
// inputID / warnTestID differ between add and edit forms.
func entityVarField(kind entityVarKind, siblings []varEntity, selfID, inputID, warnTestID, varName, name string, onInput ui.Handler) ui.Node {
	base := entityVarBase(kind, varName, name)
	warn := entityVarCollision(kind, siblings, selfID, varName, name)
	return Div(css.Class("entity-var-block"),
		Input(css.Class("field"), Attr("id", inputID), Type("text"),
			Placeholder(entityVarPlaceholder(kind, name)), Value(varName), OnInput(onInput)),
		Div(css.Class("entity-var-preview"),
			Span(css.Class("entity-var-preview-lead"), uistate.T("budgets.varNameGenerates")),
			Span(ClassStr("entity-var-chip"), base+"_"+kind.ChipField),
			Span(css.Class("entity-var-preview-fields"), uistate.T(kind.FieldsHintKey)),
		),
		If(warn != "", Span(css.Class("cover-fx-err"), Attr("data-testid", warnTestID), warn)),
	)
}
