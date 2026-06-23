//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/mermaid"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/rulesuggest"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Rules manages auto-categorization rules: a match phrase assigns a category
// (and optional tags) to transactions whose payee/description contains it. Add,
// list, inline-edit, and delete; the first matching rule wins at entry/import.
func Rules() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:rules", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	errMsg := ui.UseState("")
	dragSrc := ui.UseState("") // id of the rule being dragged (precedence reorder, C64)
	notice := uistate.UseNotice()

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}

	deleteRule := func(ruleID string) {
		if err := app.DeleteRule(ruleID); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}
	saveRule := func(ruleID, m, cat, tagStr string) {
		if errKey := validateRuleInput(m, cat); errKey != "" {
			errMsg.Set(uistate.T(errKey))
			return
		}
		r := rules.Rule{ID: ruleID, Match: strings.TrimSpace(m), SetCategoryID: cat, SetTags: textutil.CommaFields(tagStr)}
		if err := app.PutRule(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	// Text each rule is matched against (payee + description), mirroring the engine
	// at entry/import. Computed once and reused for the per-rule counts below.
	txns := app.Transactions()
	texts := make([]string, len(txns))
	for i, t := range txns {
		texts[i] = t.Payee + " " + t.Desc
	}

	applyExisting := ui.UseEvent(Prevent(func() {
		n, err := app.ApplyRules()
		if err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		if n == 0 {
			notice.Set(notice.Get().With(uistate.T("rules.appliedNone"), false))
		} else {
			notice.Set(notice.Get().With(uistate.T("rules.applied", plural(n, "transaction")), false))
		}
		bump()
	}))

	rs := app.Rules()
	// Flag rules that never fire because an earlier rule already matches them.
	warnByID := map[string]string{}
	for _, c := range rules.Conflicts(rs) {
		if c.ShadowedBy >= 0 {
			warnByID[rs[c.Index].ID] = uistate.T("rules.shadowed", rs[c.ShadowedBy].Match)
		} else {
			warnByID[rs[c.Index].ID] = uistate.T("rules.noMatch")
		}
	}
	// Per-rule match counts + overall coverage — the "before you Apply to existing"
	// signal (L15), reusing the texts computed above.
	covered := rules.Covered(rs, texts)
	hasTxns := len(texts) > 0

	// Drag-to-reorder precedence (C64): drop the dragged rule in front of the target,
	// renumber Order via appstate, and refresh. First matching rule wins, so order =
	// precedence.
	reorder := func(targetID string) {
		src := dragSrc.Get()
		dragSrc.Set("")
		if src == "" || src == targetID {
			return
		}
		res := make([]string, 0, len(rs))
		for _, r := range rs {
			if r.ID == src {
				continue
			}
			if r.ID == targetID {
				res = append(res, src)
			}
			res = append(res, r.ID)
		}
		if err := app.ReorderRules(res); err == nil {
			bump()
		}
	}

	list := IfElse(len(rs) == 0,
		ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("rules.empty"), CTALabel: uistate.T("rules.addFirst"), AddTarget: "rule"}),
		Div(css.Class("rows"), MapKeyed(rs,
			func(r rules.Rule) any { return r.ID },
			func(r rules.Rule) ui.Node {
				rid := r.ID
				return ui.CreateElement(RuleRow, ruleRowProps{
					Rule: r, Categories: cats, CategoryName: catName[r.SetCategoryID],
					Warning: warnByID[r.ID], MatchCount: r.MatchCount(texts), ShowMatchCount: hasTxns,
					OnDelete: deleteRule, OnSave: saveRule,
					OnDragStart: func() { dragSrc.Set(rid) },
					OnDrop:      func() { reorder(rid) },
				})
			},
		)),
	)

	acceptSuggestion := func(r rules.Rule) {
		r.ID = id.New()
		if err := app.PutRule(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		errMsg.Set("")
		bump()
	}

	// Suggested rules from past categorizations (excluding ones a rule already covers).
	suggestions := rulesuggest.Suggest(app.Transactions(), rs, 3)
	suggestCard := Fragment()
	if len(suggestions) > 0 {
		suggestCard = Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("rules.suggestedTitle")),
			P(css.Class("muted"), uistate.T("rules.suggestedHint")),
			Div(css.Class("rows"), MapKeyed(suggestions,
				func(s rulesuggest.Suggestion) any { return s.Rule.Match },
				func(s rulesuggest.Suggestion) ui.Node {
					return ui.CreateElement(SuggestionRow, suggestionRowProps{
						Suggestion: s, CategoryName: catName[s.Rule.SetCategoryID], OnAdd: acceptSuggestion,
					})
				},
			)),
		)
	}

	return Div(
		suggestCard,
		Section(css.Class("card"),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween),
				H2(css.Class("card-title"), uistate.T("rules.listTitle")),
				If(len(rs) > 0, Button(css.Class("btn"), Type("button"), Title(uistate.T("rules.applyExistingTitle")), OnClick(applyExisting), uistate.T("rules.applyExisting"))),
			),
			If(len(rs) > 0 && hasTxns, P(css.Class("muted"), uistate.T("rules.coverage", covered, len(texts)))),
			list,
		),
		// Precedence chain: first match wins, top to bottom; shadowed rules flagged (C70/C64).
		If(len(rs) > 1, Section(css.Class("card"),
			H2(css.Class("card-title"), "Rule order"),
			P(css.Class("muted"), "First match wins, top to bottom."),
			uiw.Mermaid(uiw.MermaidProps{
				Source: mermaid.FromRules(rs, func(id string) string { return catName[id] }),
				Label:  "Rule precedence chain",
			}),
		)),
	)
}

// validateRuleInput returns the i18n key of the first problem with a rule's
// match/category, or "" when both are present. Keeps the raw appstate error out
// of the UI by checking the same invariants client-side first.
func validateRuleInput(match, categoryID string) string {
	if strings.TrimSpace(match) == "" {
		return "rules.matchRequired"
	}
	if categoryID == "" {
		return "rules.categoryRequired"
	}
	return ""
}

// categoryOptions builds <option>s for a category picker (a leading "choose"
// placeholder, then every category by name), marking selected as current.
func categoryOptions(cats []domain.Category, selected string) []ui.Node {
	opts := []ui.Node{Option(Value(""), SelectedIf(selected == ""), uistate.T("rules.chooseCategory"))}
	for _, c := range cats {
		opts = append(opts, Option(Value(c.ID), SelectedIf(selected == c.ID), c.Name))
	}
	return opts
}

type suggestionRowProps struct {
	Suggestion   rulesuggest.Suggestion
	CategoryName string
	OnAdd        func(rules.Rule)
}

// SuggestionRow renders one suggested rule with its supporting evidence and an
// Add button that accepts it. It owns its own click handler (per the no-hooks-in-
// loops rule).
func SuggestionRow(props suggestionRowProps) ui.Node {
	s := props.Suggestion
	add := ui.UseEvent(Prevent(func() { props.OnAdd(s.Rule) }))
	cat := props.CategoryName
	if cat == "" {
		cat = uistate.T("rules.unknownCategory")
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), uistate.T("rules.suggestionDesc", s.Rule.Match, cat)),
			Span(css.Class("row-meta"), uistate.T("rules.suggestionMeta", s.Total)),
		),
		Button(css.Class("btn btn-primary"), Type("button"), Title(uistate.T("rules.acceptTitle")), OnClick(add), uistate.T("rules.accept")),
	)
}

type ruleRowProps struct {
	Rule           rules.Rule
	Categories     []domain.Category
	CategoryName   string
	Warning        string // non-empty when this rule never fires (shadowed)
	MatchCount     int    // how many existing transactions this rule's phrase hits
	ShowMatchCount bool   // whether to show the count (there are transactions to count)
	OnDelete       func(string)
	OnSave         func(id, match, category, tags string)
	OnDragStart    func()
	OnDrop         func()
}

// RuleRow is a per-rule row, editable inline (match + category + tags). All hooks
// are declared unconditionally so the edit toggle never reorders them.
func RuleRow(props ruleRowProps) ui.Node {
	r := props.Rule
	del := ui.UseEvent(Prevent(func() { props.OnDelete(r.ID) }))
	editing := ui.UseState(false)
	matchS := ui.UseState(r.Match)
	catS := ui.UseState(r.SetCategoryID)
	tagsS := ui.UseState(strings.Join(r.SetTags, ", "))
	onMatch := ui.UseEvent(func(v string) { matchS.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catS.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tagsS.Set(v) })
	startEdit := ui.UseEvent(Prevent(func() {
		matchS.Set(r.Match)
		catS.Set(r.SetCategoryID)
		tagsS.Set(strings.Join(r.SetTags, ", "))
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(r.ID, matchS.Get(), catS.Get(), tagsS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("rule-edit-" + r.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				Input(css.Class("field"), Attr("id", "rule-edit-"+r.ID), Type("text"), Attr("aria-label", uistate.T("rules.matchFieldLabel")), Placeholder(uistate.T("rules.matchPlaceholder")), Value(matchS.Get()), OnInput(onMatch)),
				Select(css.Class("field"), Attr("aria-label", uistate.T("rules.categoryFieldLabel")), OnChange(onCat), categoryOptions(props.Categories, catS.Get())),
				Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("rules.tagsFieldLabel")), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tagsS.Get()), OnInput(onTags)),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	target := props.CategoryName
	if target == "" {
		target = uistate.T("rules.unknownCategory")
	}
	meta := uistate.T("rules.appliesTo", target)
	if len(r.SetTags) > 0 {
		meta += " · " + strings.Join(r.SetTags, ", ")
	}
	return Div(css.Class("row"), Attr("draggable", "true"),
		OnDragStart(func() {
			if props.OnDragStart != nil {
				props.OnDragStart()
			}
		}),
		OnDragOver(Prevent(func() {})), // allow drop
		OnDrop(Prevent(func() {
			if props.OnDrop != nil {
				props.OnDrop()
			}
		})),
		Span(css.Class("rule-grip"), Attr("aria-hidden", "true"), Title(uistate.T("rules.dragTitle")), uiw.Icon(icon.MoreH, css.Class(tw.W4, tw.H4))),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), uistate.T("rules.matchLabel", r.Match)),
			Span(css.Class("row-meta"), meta),
			If(props.ShowMatchCount, Span(css.Class("row-meta"), uistate.T("rules.matchCountMeta", plural(props.MatchCount, "transaction")))),
			If(props.Warning != "", Span(css.Class("row-meta", tw.TextWarn), props.Warning)),
		),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("rules.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("rules.deleteTitle")), Title(uistate.T("rules.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}
