// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"regexp"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/formula"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// stuTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement.
func stuTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// molNameRe validates a new compound variable's name: lowercase letters,
// digits, underscores, starting with a letter — the shape every other engine
// variable uses, so it drops into formulas without quoting.
var molNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// StudioFormulas is the Studio "Formulas" tab (also the /customize route body):
// a bento surface pairing the formula workbench (the searchable FormulaBuilder
// with the saved-formulas list) with the COMPOUND-VARIABLE editor — the
// molecules (net_worth, health_score, credit_proxy, …) shown with their live
// values and exact formula definitions, editable in place. Editing one
// reshapes it everywhere: the engine recomputes every surface from the
// persisted definition (see appstate.Molecules' layering).
func StudioFormulas() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	rev := ui.UseState(0)
	_ = rev.Get()
	bump := func() { rev.Set(rev.Get() + 1) }

	vars := liveEngineVars(app)
	evalDraft := func(expr string) (string, error) {
		v, err := formula.Eval(expr, formula.Env{Vars: vars})
		if err != nil {
			return "", err
		}
		return formatFormulaValue(v), nil
	}

	// New compound-variable form state (hooks at stable positions).
	newName := ui.UseState("")
	newFormula := ui.UseState("")
	newMsg := ui.UseState("")
	onNewName := ui.UseEvent(func(v string) { newName.Set(v) })
	onNewFormula := ui.UseEvent(func(v string) { newFormula.Set(v) })
	createMol := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(newName.Get())
		f := strings.TrimSpace(newFormula.Get())
		if !molNameRe.MatchString(name) {
			newMsg.Set(uistate.T("studio.molNameHint"))
			return
		}
		if _, exists := vars[name]; exists {
			newMsg.Set(uistate.T("studio.molNameTaken", name))
			return
		}
		if err := app.PutMolecule(domain.Molecule{Name: name, Formula: f}); err != nil {
			newMsg.Set(err.Error())
			return
		}
		newName.Set("")
		newFormula.Set("")
		newMsg.Set(uistate.T("studio.molSaved"))
		uistate.BumpDataRevision()
		bump()
	}))

	// Built-in defaults by name, to tag overridden molecules and offer reset.
	defaults := map[string]string{}
	for _, m := range engineenv.DefaultMolecules() {
		defaults[m.Name] = m.Formula
	}

	var molRows []ui.Node
	for _, m := range app.Molecules() {
		def, isBuiltIn := defaults[m.Name]
		kind := "custom"
		if isBuiltIn {
			kind = "builtin"
			if m.Formula != def {
				kind = "overridden"
			}
		}
		molRows = append(molRows, ui.CreateElement(studioMoleculeRow, studioMoleculeRowProps{
			M: m, Value: vars[m.Name], Kind: kind, Eval: evalDraft,
			OnSave: func(mol domain.Molecule) error {
				if err := app.PutMolecule(mol); err != nil {
					return err
				}
				uistate.BumpDataRevision()
				bump()
				return nil
			},
			OnRemove: func(name string) {
				if _, err := app.DeleteMolecule(name); err != nil {
					return
				}
				uistate.BumpDataRevision()
				bump()
			},
		}))
	}

	newForm := Form(css.Class("stu-mol-edit"), OnSubmit(createMol), Attr("data-testid", "mol-new-form"),
		Div(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("studio.molNewTitle")),
		Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "mol-new-name"),
				Attr("aria-label", uistate.T("studio.molNewTitle")),
				Placeholder(uistate.T("studio.molNewName")),
				Value(newName.Get()), OnInput(onNewName)),
			Input(css.Class("field", tw.Flex1), Type("text"), Attr("data-testid", "mol-new-formula"),
				Attr("aria-label", uistate.T("studio.molNewFormula")),
				Placeholder(uistate.T("studio.molNewFormula")),
				Value(newFormula.Get()), OnInput(onNewFormula)),
			Button(css.Class("btn", "btn-primary"), Type("submit"), Attr("data-testid", "mol-new-create"),
				uistate.T("studio.molCreate")),
		),
		P(css.Class("t-caption", tw.TextFaint), uistate.T("studio.molNameHint")),
		If(newMsg.Get() != "", P(css.Class("t-caption"), Attr("role", "status"), newMsg.Get())),
	)

	molTile := stuTile("stu-molecules", "1 / span 4",
		hltSection("sec-stu-molecules", uistate.T("studio.molTitle"), nil, Fragment(
			P(css.Class("muted"), uistate.T("studio.molHint")),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), molRows),
			newForm,
		)))

	workbench := stuTile("stu-workbench", "1 / span 4",
		FormulaBuilder(FormulaBuilderProps{ShowSaved: true}))

	// The studio masthead every sibling tab opens with (eyebrow + serif title
	// + lede), so the hub reads as one surface.
	masthead := Div(css.Class("wman-head"),
		Span(css.Class("studio-eyebrow"), uistate.T("wman.eyebrow")),
		H2(css.Class("studio-design-title"), uistate.T("stuh.formulasTitle")),
		P(css.Class("studio-design-sub"), uistate.T("stuh.formulasLede")),
	)

	return Div(css.Class("stu-deck"),
		masthead,
		Div(css.Class("bento bento-studio"), workbench, molTile),
	)
}

type studioMoleculeRowProps struct {
	M        domain.Molecule
	Value    float64
	Kind     string // "builtin" | "overridden" | "custom"
	Eval     func(string) (string, error)
	OnSave   func(domain.Molecule) error
	OnRemove func(string) // reset (overridden built-in) or delete (custom)
}

// studioMoleculeRow renders one compound variable: its name, provenance tag,
// live value, doc, and formula — with an in-place editor (live preview, save,
// and reset-to-default / delete). Its own component so its hooks sit at stable
// positions (rows render in a variable-length loop).
func studioMoleculeRow(p studioMoleculeRowProps) ui.Node {
	editing := ui.UseState(false)
	draft := ui.UseState("")
	msg := ui.UseState("")
	onDraft := ui.UseEvent(func(v string) { draft.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		draft.Set(p.M.Formula)
		msg.Set("")
		editing.Set(true)
	}))
	cancel := ui.UseEvent(Prevent(func() { editing.Set(false); msg.Set("") }))
	save := ui.UseEvent(Prevent(func() {
		m := p.M
		m.Formula = strings.TrimSpace(draft.Get())
		if err := p.OnSave(m); err != nil {
			msg.Set(err.Error())
			return
		}
		editing.Set(false)
		msg.Set("")
	}))
	remove := ui.UseEvent(Prevent(func() {
		p.OnRemove(p.M.Name)
		editing.Set(false)
	}))

	tagKey, tagCls := "studio.molBuiltIn", "stu-mol-tag"
	switch p.Kind {
	case "overridden":
		tagKey, tagCls = "studio.molOverridden", "stu-mol-tag is-custom"
	case "custom":
		tagKey, tagCls = "studio.molCustom", "stu-mol-tag is-custom"
	}

	head := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap),
		Code(css.Class("fb-chip-label"), p.M.Name),
		Span(ClassStr(tagCls), uistate.T(tagKey)),
		Span(css.Class("t-caption", tw.TextDim), "= "+groupThousands(p.Value)),
		Span(css.Class(tw.Flex1)),
		IfElse(editing.Get(),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "mol-cancel-"+p.M.Name), OnClick(cancel), uistate.T("studio.molCancel")),
			Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "mol-edit-"+p.M.Name), OnClick(startEdit), uistate.T("studio.molEdit")),
		),
	)

	var body ui.Node
	if editing.Get() {
		// Guardrail at the point of action: this is a LIVE definition — the app
		// recomputes every page and widget that reads it the moment it saves.
		warning := P(ClassStr("t-caption "+tw.ColorClass("text-warn")), Attr("data-testid", "mol-warning-"+p.M.Name),
			uistate.T("studio.molLiveWarning", p.M.Name))
		// Live preview of the draft against the current figures.
		var preview ui.Node
		if v, err := p.Eval(strings.TrimSpace(draft.Get())); err != nil {
			preview = P(ClassStr("t-caption "+tw.ColorClass("text-down")), Attr("role", "status"),
				uistate.T("studio.molPreviewErr", err.Error()))
		} else {
			preview = P(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "mol-preview-"+p.M.Name),
				uistate.T("studio.molPreview", v))
		}
		var resetBtn ui.Node = Fragment()
		if p.Kind == "overridden" {
			resetBtn = Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "mol-reset-"+p.M.Name),
				OnClick(remove), uistate.T("studio.molReset"))
		} else if p.Kind == "custom" {
			resetBtn = Button(css.Class("btn", "btn-sm", "btn-del"), Type("button"), Attr("data-testid", "mol-delete-"+p.M.Name),
				OnClick(remove), uistate.T("studio.molDelete"))
		}
		body = Div(css.Class("stu-mol-edit"),
			warning,
			Textarea(css.Class("field"), Attr("data-testid", "mol-draft-"+p.M.Name),
				Attr("aria-label", uistate.T("studio.molFormulaLabel", p.M.Name)),
				OnInput(onDraft), draft.Get()),
			preview,
			Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap),
				Button(css.Class("btn", "btn-primary", "btn-sm"), Type("button"), Attr("data-testid", "mol-save-"+p.M.Name),
					OnClick(save), uistate.T("studio.molSave")),
				resetBtn,
			),
			If(msg.Get() != "", P(ClassStr("t-caption "+tw.ColorClass("text-down")), Attr("role", "alert"), msg.Get())),
		)
	} else {
		body = Code(css.Class("stu-mol-formula"), p.M.Formula)
	}

	return Div(css.Class("row", tw.Flex, tw.FlexCol, tw.Gap1), Attr("data-testid", "mol-row-"+p.M.Name),
		head,
		If(p.M.Doc != "", P(css.Class("t-caption", tw.TextDim), p.M.Doc)),
		body,
	)
}
