// SPDX-License-Identifier: MIT

//go:build js && wasm

// dataedit_forms.go holds the flip-modal editor bodies for the Data & People
// pages: members (edit + PIN), categories, rules, and artifact rename. Each
// form is rendered by its shell-root host (internal/app dataedithost.go)
// inside a NoFooter FlipPanel, owns all its state (the host mounts it fresh on
// each open, so useState initializers seed from the entity), does its own save
// through appstate, and calls OnDone to close the modal. Saves bump the shared
// data revision (so memoized rows refresh) and request an immediate persist.
package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/memberrole"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// dataEditSaved is the shared post-save path for the modal editors: refresh
// every subscribed surface and flush the dataset now (a deliberate save must
// not ride the 4s autosave ticker into a lost-on-reload window).
func dataEditSaved() {
	uistate.BumpDataRevision()
	uistate.RequestPersist()
}

// ── Member editor ─────────────────────────────────────────────────────────────

// MemberEditFormProps drives the member editor rendered inside the shell-root
// flip modal. Mode selects the full editor or the PIN form.
type MemberEditFormProps struct {
	MemberID string
	Mode     string // uistate.MemberEditMode*
	OnDone   func()
}

// MemberEditForm is the member flip-modal body: name, color, personal
// preferences, role, and member-scoped custom fields — or, in PIN mode, the
// set/change PIN form (same testids the inline form carried).
func MemberEditForm(props MemberEditFormProps) ui.Node {
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	var m domain.Member
	found := false
	if app != nil {
		for _, mm := range app.Members() {
			if mm.ID == props.MemberID {
				m, found = mm, true
				break
			}
		}
	}

	errS := ui.UseState("")
	color := m.Color
	if color == "" {
		color = "#7c83ff"
	}
	nameS := ui.UseState(m.Name)
	colorS := ui.UseState(color)
	dateStyleS := ui.UseState(m.Prefs.DateStyle)
	defAcctS := ui.UseState(m.Prefs.DefaultAccountID)
	roleS := ui.UseState(string(memberrole.Resolve(m)))
	customS := ui.UseState(customMapToStrings(m.Custom))
	pinS := ui.UseState("")
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	onPIN := ui.UseEvent(func(v string) { pinS.Set(v) })
	setCustom := func(key, value string) {
		cur := customS.Get()
		next := make(map[string]string, len(cur)+1)
		for k, v := range cur {
			next[k] = v
		}
		next[key] = value
		customS.Set(next)
	}

	var memberDefs []customfields.Def
	if app != nil {
		memberDefs = app.CustomFieldDefsFor("member")
	}

	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil || !found {
			done()
			return
		}
		if n := strings.TrimSpace(nameS.Get()); n != "" {
			m.Name = n
		}
		m.Color = strings.TrimSpace(colorS.Get())
		// Per-member preferences (§1.19): empty = inherit the household default.
		m.Prefs.DateStyle = strings.TrimSpace(dateStyleS.Get())
		m.Prefs.DefaultAccountID = strings.TrimSpace(defAcctS.Get())
		if r, err := memberrole.ParseRole(strings.TrimSpace(roleS.Get())); err == nil {
			m.Role = r
		}
		m.Custom = customValuesToMap(memberDefs, customS.Get())
		if err := app.PutMember(m); err != nil {
			errS.Set(err.Error())
			return
		}
		dataEditSaved()
		done()
	}))
	savePIN := ui.UseEvent(Prevent(func() {
		if app == nil || !found {
			done()
			return
		}
		if err := app.SetMemberPIN(m.ID, pinS.Get()); err != nil {
			errS.Set(uistate.T("profileSwitch.pinTooWeak"))
			return
		}
		dataEditSaved()
		done()
	}))
	cancel := ui.UseEvent(Prevent(func() { done() }))

	// Land the cursor in the first field when the modal opens (§6.7).
	ui.UseEffect(func() func() {
		if props.Mode == uistate.MemberEditModePIN {
			focusByID("member-pin-" + m.ID)
		} else {
			focusByID("member-edit-" + m.ID)
		}
		return nil
	}, true)

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	if props.Mode == uistate.MemberEditModePIN {
		pinLbl := uistate.T("profileSwitch.setPIN")
		if app.MemberHasPIN(m.ID) {
			pinLbl = uistate.T("profileSwitch.changePIN")
		}
		return Form(css.Class("form-grid"),
			Attr("data-testid", "member-pin-form-"+m.ID),
			OnSubmit(savePIN),
			uiw.FormField(uistate.T("profileSwitch.pinNew"),
				Input(css.Class("field"), Attr("id", "member-pin-"+m.ID), Type("password"),
					Attr("autocomplete", "off"),
					Attr("data-testid", "member-pin-input-"+m.ID),
					Value(pinS.Get()),
					OnInput(onPIN),
				),
			),
			If(errS.Get() != "", P(css.Class("notice-danger"), errS.Get())),
			Button(css.Class("btn btn-primary"), Type("submit"), pinLbl),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("profileSwitch.pinFormCancel")),
		)
	}

	// Custom-field inputs (member-scoped defs), keyed so each owns its hook.
	customInputs := MapKeyed(memberDefs,
		func(d customfields.Def) any { return d.Key },
		func(d customfields.Def) ui.Node {
			return labeledField(d.Label, ui.CreateElement(CustomFieldInput, customFieldInputProps{
				Def: d, Value: customS.Get()[d.Key], OnChange: setCustom,
			}))
		})
	return Form(css.Class("form-grid"), OnSubmit(saveEdit),
		labeledField(uistate.T("members.name"),
			Input(css.Class("field"), Attr("id", "member-edit-"+m.ID), Type("text"), Attr("aria-label", uistate.T("members.name")), Placeholder(uistate.T("members.name")), Value(nameS.Get()), OnInput(onName))),
		labeledField(uistate.T("members.color"),
			Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("members.color")), Attr("aria-label", uistate.T("members.color")), Value(colorS.Get()), OnInput(onColor))),
		// Per-member preferences (§1.19): an optional personal date style and a
		// default account that seeds this member's quick-add. "Inherit" = use the
		// household default.
		labeledField(uistate.T("members.prefDateStyle"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   memberDateStyleOptions(),
				Selected:  dateStyleS.Get(),
				OnChange:  func(v string) { dateStyleS.Set(v) },
				AriaLabel: uistate.T("members.prefDateStyle"),
			})),
		labeledField(uistate.T("members.prefDefaultAccount"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   memberDefaultAccountOptions(),
				Selected:  defAcctS.Get(),
				OnChange:  func(v string) { defAcctS.Set(v) },
				AriaLabel: uistate.T("members.prefDefaultAccount"),
			})),
		labeledField(uistate.T("members.roleLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   memberRoleOptions(),
				Selected:  roleS.Get(),
				OnChange:  func(v string) { roleS.Set(v) },
				AriaLabel: "Role",
				TestID:    "member-edit-role-" + m.ID,
			})),
		customInputs,
		If(errS.Get() != "", P(css.Class("notice-danger"), errS.Get())),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
	)
}

// ── Category editor ───────────────────────────────────────────────────────────

// CategoryEditFormProps drives the category editor flip-modal body.
type CategoryEditFormProps struct {
	CategoryID string
	OnDone     func()
}

// CategoryEditForm edits a category's name, kind, parent, color, and
// deductible flag. Changing the kind resets the parent (a category can only
// nest under its own kind).
func CategoryEditForm(props CategoryEditFormProps) ui.Node {
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	var c domain.Category
	found := false
	var cats []domain.Category
	if app != nil {
		cats = app.Categories()
		for _, cc := range cats {
			if cc.ID == props.CategoryID {
				c, found = cc, true
				break
			}
		}
	}

	errS := ui.UseState("")
	nameS := ui.UseState(c.Name)
	kindS := ui.UseState(string(c.Kind))
	parentS := ui.UseState(c.ParentID)
	colorS := ui.UseState(catColor(c.Color))
	deductibleS := ui.UseState(c.Deductible)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onColor := ui.UseEvent(func(v string) { colorS.Set(v) })
	onDeductible := ui.UseEvent(func(e ui.Event) { deductibleS.Set(e.IsChecked()) })
	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil || !found {
			done()
			return
		}
		if n := strings.TrimSpace(nameS.Get()); n != "" {
			c.Name = n
		}
		if k := domain.CategoryKind(kindS.Get()); k.Valid() {
			c.Kind = k
		}
		c.ParentID = parentS.Get()
		c.Color = colorS.Get()
		c.Deductible = deductibleS.Get()
		if err := app.PutCategory(c); err != nil {
			errS.Set(err.Error())
			return
		}
		dataEditSaved()
		done()
	}))
	cancel := ui.UseEvent(Prevent(func() { done() }))
	ui.UseEffect(func() func() {
		focusByID("cat-edit-" + c.ID)
		return nil
	}, true)

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	// Parent options: same-kind categories except this one (prevents self-parenting).
	var sameKind []domain.Category
	for _, cc := range cats {
		if string(cc.Kind) == kindS.Get() && cc.ID != c.ID {
			sameKind = append(sameKind, cc)
		}
	}
	parentOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("categories.noParent")}}
	for _, f := range categorytree.Flatten(sameKind) {
		parentOpts = append(parentOpts, uiw.SelectOption{Value: f.Category.ID, Label: uiw.IndentLabel(f.Depth) + f.Category.Name})
	}
	kindOpts := []uiw.SelectOption{
		{Value: string(domain.KindExpense), Label: uistate.T("category.expense")},
		{Value: string(domain.KindIncome), Label: uistate.T("category.income")},
	}
	return Form(css.Class("form-grid"), OnSubmit(saveEdit),
		// Visible label for the name field (C63 labelling gap: placeholder-only
		// is insufficient for screen readers and sighted users who clear the field).
		labeledField(uistate.T("common.name"),
			Input(css.Class("field"), Attr("id", "cat-edit-"+c.ID), Type("text"), Placeholder(uistate.T("common.name")), Value(nameS.Get()), OnInput(onName))),
		labeledField("Category type",
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   kindOpts,
				Selected:  kindS.Get(),
				OnChange:  func(v string) { kindS.Set(v); parentS.Set("") },
				AriaLabel: "Category type",
			})),
		labeledField("Parent category",
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   parentOpts,
				Selected:  parentS.Get(),
				OnChange:  func(v string) { parentS.Set(v) },
				AriaLabel: "Parent category",
			})),
		labeledField(uistate.T("categories.color"),
			Input(css.Class("color-input"), Type("color"), Attr("title", uistate.T("categories.color")), Attr("aria-label", uistate.T("categories.color")), Value(colorS.Get()), OnInput(onColor))),
		Label(css.Class("checkbox-label"), Attr("title", uistate.T("categories.deductibleTitle")),
			Input(Type("checkbox"), Attr("id", "cat-edit-deductible-"+c.ID), Attr("aria-label", uistate.T("categories.deductible")), Attr("data-testid", "cat-deductible-"+c.ID), CheckedIf(deductibleS.Get()), OnChange(onDeductible)),
			Text(" "+uistate.T("categories.deductible")),
		),
		If(errS.Get() != "", P(css.Class("notice-danger"), errS.Get())),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
	)
}

// ── Rule editor ───────────────────────────────────────────────────────────────

// RuleEditFormProps drives the rule editor flip-modal body.
type RuleEditFormProps struct {
	RuleID string
	OnDone func()
}

// RuleEditForm edits a rule's match phrase, category, tags, and rename action.
// It mutates the EXISTING rule, so precedence Order and structured Conditions
// survive the edit — the old inline form rebuilt the rule from scratch and
// silently dropped both.
func RuleEditForm(props RuleEditFormProps) ui.Node {
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	var r rules.Rule
	found := false
	var cats []domain.Category
	if app != nil {
		cats = app.Categories()
		for _, rr := range app.Rules() {
			if rr.ID == props.RuleID {
				r, found = rr, true
				break
			}
		}
	}

	errS := ui.UseState("")
	matchS := ui.UseState(r.Match)
	catS := ui.UseState(r.SetCategoryID)
	tagsS := ui.UseState(strings.Join(r.SetTags, ", "))
	renameDescS := ui.UseState(r.RenameDesc)
	onMatch := ui.UseEvent(func(v string) { matchS.Set(v) })
	onTags := ui.UseEvent(func(v string) { tagsS.Set(v) })
	onRenameDesc := ui.UseEvent(func(v string) { renameDescS.Set(v) })
	saveEdit := ui.UseEvent(Prevent(func() {
		if app == nil || !found {
			done()
			return
		}
		if errKey := validateRuleInput(matchS.Get(), catS.Get()); errKey != "" {
			errS.Set(uistate.T(errKey))
			return
		}
		r.Match = strings.TrimSpace(matchS.Get())
		r.SetCategoryID = catS.Get()
		r.SetTags = textutil.CommaFields(tagsS.Get())
		r.RenameDesc = strings.TrimSpace(renameDescS.Get())
		if err := app.PutRule(r); err != nil {
			errS.Set(err.Error())
			return
		}
		dataEditSaved()
		done()
	}))
	cancel := ui.UseEvent(Prevent(func() { done() }))
	ui.UseEffect(func() func() {
		focusByID("rule-edit-" + r.ID)
		return nil
	}, true)

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	return Form(css.Class("form-grid"), OnSubmit(saveEdit),
		labeledField(uistate.T("rules.matchFieldLabel"),
			Input(css.Class("field"), Attr("id", "rule-edit-"+r.ID), Type("text"), Attr("aria-label", uistate.T("rules.matchFieldLabel")), Placeholder(uistate.T("rules.matchPlaceholder")), Value(matchS.Get()), OnInput(onMatch))),
		labeledField(uistate.T("rules.categoryFieldLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   categorySelectOptions(cats, catS.Get()),
				Selected:  catS.Get(),
				OnChange:  func(v string) { catS.Set(v) },
				AriaLabel: uistate.T("rules.categoryFieldLabel"),
			})),
		labeledField(uistate.T("rules.tagsFieldLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("rules.tagsFieldLabel")), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tagsS.Get()), OnInput(onTags))),
		// C102: rename description action — when filled, matching transactions have their
		// description rewritten to this value (e.g. clean up garbled bank feed text).
		labeledField(uistate.T("rules.renameDescFieldLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("rules.renameDescFieldLabel")), Placeholder(uistate.T("rules.renameDescPlaceholder")), Value(renameDescS.Get()), OnInput(onRenameDesc))),
		If(len(r.Conditions) > 0, P(css.Class("muted"), uistate.T("rules.editKeepsConditions", len(r.Conditions)))),
		If(errS.Get() != "", P(css.Class("notice-danger"), errS.Get())),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
	)
}

// ── Artifact rename ───────────────────────────────────────────────────────────

// ArtifactRenameFormProps drives the artifact rename flip-modal body.
type ArtifactRenameFormProps struct {
	ArtifactID string
	OnDone     func()
}

// ArtifactRenameForm renames one vault file.
func ArtifactRenameForm(props ArtifactRenameFormProps) ui.Node {
	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	var a domain.Artifact
	found := false
	if app != nil {
		for _, aa := range app.Artifacts() {
			if aa.ID == props.ArtifactID {
				a, found = aa, true
				break
			}
		}
	}

	errS := ui.UseState("")
	nameS := ui.UseState(a.Name)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	saveRename := ui.UseEvent(Prevent(func() {
		if app == nil || !found {
			done()
			return
		}
		n := strings.TrimSpace(nameS.Get())
		if n == "" {
			done()
			return
		}
		a.Name = n
		if err := app.PutArtifact(a); err != nil {
			errS.Set(err.Error())
			return
		}
		dataEditSaved()
		done()
	}))
	cancel := ui.UseEvent(Prevent(func() { done() }))
	ui.UseEffect(func() func() {
		focusByID("artifact-rename-" + a.ID)
		return nil
	}, true)

	if !found {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	return Form(css.Class("form-grid"), OnSubmit(saveRename),
		labeledField(uistate.T("artifacts.renameLabel"),
			Input(css.Class("field"), Attr("id", "artifact-rename-"+a.ID), Attr("aria-label", uistate.T("artifacts.renameLabel")),
				Value(nameS.Get()), OnInput(onName), Attr("data-testid", "artifact-rename-input"))),
		If(errS.Get() != "", P(css.Class("notice-danger"), errS.Get())),
		Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
		Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
	)
}
