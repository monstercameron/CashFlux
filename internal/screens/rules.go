//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/rulesuggest"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:rules", 0)
	bump := func() { rev.Set(rev.Get() + 1) }

	match := ui.UseState("")
	categoryID := ui.UseState("")
	tags := ui.UseState("")
	errMsg := ui.UseState("")
	notice := uistate.UseNotice()

	onMatch := ui.UseEvent(func(v string) { match.Set(v) })
	onCategory := ui.UseEvent(func(e ui.Event) { categoryID.Set(e.GetValue()) })
	onTags := ui.UseEvent(func(v string) { tags.Set(v) })

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}

	add := ui.UseEvent(Prevent(func() {
		if errKey := validateRuleInput(match.Get(), categoryID.Get()); errKey != "" {
			errMsg.Set(uistate.T(errKey))
			return
		}
		r := rules.Rule{
			ID:            id.New(),
			Match:         strings.TrimSpace(match.Get()),
			SetCategoryID: categoryID.Get(),
			SetTags:       textutil.CommaFields(tags.Get()),
		}
		if err := app.PutRule(r); err != nil {
			errMsg.Set(err.Error())
			return
		}
		match.Set("")
		categoryID.Set("")
		tags.Set("")
		errMsg.Set("")
		bump()
	}))

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
	// at entry/import. Computed once and reused for the per-rule counts below and
	// the live authoring preview.
	txns := app.Transactions()
	texts := make([]string, len(txns))
	for i, t := range txns {
		texts[i] = t.Payee + " " + t.Desc
	}
	// Live match-count preview while authoring: how many existing transactions the
	// phrase being typed would hit, so the user can trust a rule before saving (C64).
	liveMatch := strings.TrimSpace(match.Get())
	liveCount := 0
	if liveMatch != "" {
		liveCount = rules.Rule{Match: liveMatch}.MatchCount(texts)
	}

	form := Section(Class("card"),
		H2(Class("card-title"), uistate.T("rules.add")),
		P(Class("muted"), uistate.T("rules.hint")),
		Form(Class("form-grid"), OnSubmit(add),
			Input(append([]any{Class("field"), Attr("id", "rule-add"), Type("text"), Attr("aria-required", "true"), Placeholder(uistate.T("rules.matchPlaceholder")), Value(match.Get()), OnInput(onMatch)}, errAttrs("rule-err", errMsg.Get())...)...),
			Select(Class("field"), OnChange(onCategory), categoryOptions(cats, categoryID.Get())),
			Input(Class("field"), Type("text"), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tags.Get()), OnInput(onTags)),
			Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.add")),
		),
		If(liveMatch != "" && len(texts) > 0, P(Class("muted"), Attr("role", "status"), uistate.T("rules.matchCountMeta", plural(liveCount, "transaction")))),
		errText("rule-err", errMsg.Get()),
	)

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

	list := IfElse(len(rs) == 0,
		ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("rules.empty"), CTALabel: uistate.T("rules.addFirst"), FocusID: "rule-add"}),
		Div(Class("rows"), MapKeyed(rs,
			func(r rules.Rule) any { return r.ID },
			func(r rules.Rule) ui.Node {
				return ui.CreateElement(RuleRow, ruleRowProps{
					Rule: r, Categories: cats, CategoryName: catName[r.SetCategoryID],
					Warning: warnByID[r.ID], MatchCount: r.MatchCount(texts), ShowMatchCount: hasTxns,
					OnDelete: deleteRule, OnSave: saveRule,
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
		suggestCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("rules.suggestedTitle")),
			P(Class("muted"), uistate.T("rules.suggestedHint")),
			Div(Class("rows"), MapKeyed(suggestions,
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
		form,
		suggestCard,
		Section(Class("card"),
			Div(Class("flex items-center justify-between"),
				H2(Class("card-title"), uistate.T("rules.listTitle")),
				If(len(rs) > 0, Button(Class("btn"), Type("button"), Title(uistate.T("rules.applyExistingTitle")), OnClick(applyExisting), uistate.T("rules.applyExisting"))),
			),
			If(len(rs) > 0 && hasTxns, P(Class("muted"), uistate.T("rules.coverage", covered, len(texts)))),
			list,
		),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), uistate.T("rules.suggestionDesc", s.Rule.Match, cat)),
			Span(Class("row-meta"), uistate.T("rules.suggestionMeta", s.Total)),
		),
		Button(Class("btn btn-primary"), Type("button"), Title(uistate.T("rules.acceptTitle")), OnClick(add), uistate.T("rules.accept")),
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
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", "rule-edit-"+r.ID), Type("text"), Placeholder(uistate.T("rules.matchPlaceholder")), Value(matchS.Get()), OnInput(onMatch)),
				Select(Class("field"), OnChange(onCat), categoryOptions(props.Categories, catS.Get())),
				Input(Class("field"), Type("text"), Placeholder(uistate.T("rules.tagsPlaceholder")), Value(tagsS.Get()), OnInput(onTags)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), uistate.T("rules.matchLabel", r.Match)),
			Span(Class("row-meta"), meta),
			If(props.ShowMatchCount, Span(Class("row-meta"), uistate.T("rules.matchCountMeta", plural(props.MatchCount, "transaction")))),
			If(props.Warning != "", Span(Class("row-meta text-warn"), props.Warning)),
		),
		Button(Class("btn inline-flex items-center gap-1.5"), Type("button"), Title(uistate.T("rules.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, Class("w-4 h-4 shrink-0")), Span(uistate.T("action.edit"))),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("rules.deleteTitle")), Title(uistate.T("rules.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
	)
}
