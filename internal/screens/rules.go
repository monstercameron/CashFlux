// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/rulesuggest"
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
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	rev := state.UseAtom("rev:rules", 0)
	bump := func() { rev.Set(rev.Get() + 1) }
	// Re-render after modal saves (RuleEditHost bumps the shared data revision).
	_ = uistate.UseDataRevision().Get()

	// In-context add (G18 §1): an "+ Add rule" button in the "Your rules" header.
	addRule := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("rule") }))

	errMsg := ui.UseState("")
	dragSrc := ui.UseState("") // id of the rule being dragged (precedence reorder, C64)
	notice := uistate.UseNotice()

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}

	deleteRule := func(ruleID string) {
		// C110: confirm before deleting — rule deletion was immediate with no undo.
		uistate.ConfirmModal(uistate.T("rules.deleteConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteRule(ruleID); err != nil {
				errMsg.Set(err.Error())
				return
			}
			errMsg.Set("")
			bump()
		})
	}
	// Text each rule is matched against (payee + description), mirroring the engine
	// at entry/import. Computed once and reused for the per-rule counts below.
	txns := app.Transactions()
	texts := make([]string, len(txns))
	for i, t := range txns {
		texts[i] = t.Payee + " " + t.Desc
	}

	applyExisting := ui.UseEvent(Prevent(func() {
		// Capture the rule list at event time so match phrases are current.
		currentRules := app.Rules()
		n, perRule, err := app.ApplyRulesWithCounts()
		if err != nil {
			notice.Set(notice.Get().With(err.Error(), true))
			return
		}
		if n == 0 {
			notice.Set(notice.Get().With(uistate.T("rules.appliedNone"), false))
		} else {
			msg := uistate.T("rules.applied", plural(n, "transaction"))
			// Append per-rule breakdown when at least one rule fired.
			if len(perRule) > 0 {
				var parts []string
				for _, r := range currentRules {
					if cnt, ok := perRule[r.ID]; ok {
						parts = append(parts, uistate.T("rules.appliedPerRule", r.Match, plural(cnt, "transaction")))
					}
				}
				if len(parts) > 0 {
					msg += " — " + strings.Join(parts, ", ") + "."
				}
			}
			notice.Set(notice.Get().With(msg, false))
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
	// signal (L15), reusing the texts computed above. maxMatch anchors the per-rule
	// weight bars (each rule's catch vs the heaviest rule's).
	covered := rules.Covered(rs, texts)
	hasTxns := len(texts) > 0
	matchCounts := make(map[string]int, len(rs))
	maxMatch := 0
	for _, r := range rs {
		n := r.MatchCount(texts)
		matchCounts[r.ID] = n
		if n > maxMatch {
			maxMatch = n
		}
	}

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
		hhRowsList(MapKeyed(rs,
			func(r rules.Rule) any { return r.ID },
			func(r rules.Rule) ui.Node {
				rid := r.ID
				return ui.CreateElement(RuleRow, ruleRowProps{
					Rule: r, CategoryName: catName[r.SetCategoryID],
					Warning: warnByID[r.ID], MatchCount: matchCounts[r.ID], MaxMatchCount: maxMatch, ShowMatchCount: hasTxns,
					OnDelete:    deleteRule,
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
	// Collapsed to 5 by default; "Show all" toggle reveals the rest.
	const suggestCap = 5
	showAllSuggestions := ui.UseState(false)
	toggleShowAll := ui.UseEvent(Prevent(func() { showAllSuggestions.Set(!showAllSuggestions.Get()) }))
	suggestions := rulesuggest.Suggest(app.Transactions(), rs, 3)
	suggestCard := Fragment()
	if len(suggestions) > 0 {
		visible := suggestions
		if !showAllSuggestions.Get() && len(suggestions) > suggestCap {
			visible = suggestions[:suggestCap]
		}
		var toggleBtn ui.Node = Fragment()
		if len(suggestions) > suggestCap {
			label := uistate.T("rules.suggestShowAll", len(suggestions))
			if showAllSuggestions.Get() {
				label = uistate.T("rules.suggestShowFewer")
			}
			toggleBtn = Button(css.Class("btn"), Type("button"), OnClick(toggleShowAll), label)
		}
		suggestCard = rptSection("sec-rules-suggested", uistate.T("rules.suggestedTitleCount", len(suggestions)), nil,
			Fragment(
				P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), uistate.T("rules.suggestedHint")),
				hhRowsList(MapKeyed(visible,
					func(s rulesuggest.Suggestion) any { return s.Rule.Match },
					func(s rulesuggest.Suggestion) ui.Node {
						return ui.CreateElement(SuggestionRow, suggestionRowProps{
							Suggestion: s, CategoryName: catName[s.Rule.SetCategoryID], OnAdd: acceptSuggestion,
						})
					},
				)),
				toggleBtn,
			),
		)
	}

	// ── Hero: the coverage figure, the health chips, and the takeaway. ──────────
	shadowedCount := len(warnByID)
	coveredPct := 0
	if len(texts) > 0 {
		coveredPct = covered * 100 / len(texts)
	}
	countLine := uistate.T("rules.countWord", len(rs))
	if len(rs) == 1 {
		countLine = uistate.T("rules.countWordOne")
	}
	eyebrow := countLine + " · " + uistate.T("rules.firstWins")
	chips := []ui.Node{
		rptChip(uistate.T("rules.chipRules"), fmt.Sprintf("%d", len(rs)), ""),
	}
	if hasTxns {
		chips = append(chips, rptChip(uistate.T("rules.chipCovered"), plural(covered, "transaction"), rptToneCls("pos")))
	}
	if shadowedCount > 0 {
		chips = append(chips, rptChip(uistate.T("rules.chipShadowed"), fmt.Sprintf("%d", shadowedCount), rptToneCls("neg")))
	}
	if len(suggestions) > 0 {
		chips = append(chips, rptChip(uistate.T("rules.chipSuggested"), fmt.Sprintf("%d", len(suggestions)), ""))
	}

	takeaway := uistate.T("rls.noneTake")
	if len(rs) > 0 && hasTxns {
		takeaway = uistate.T("rls.coverTake", fmt.Sprintf("%d", covered), len(texts))
		if shadowedCount == 1 {
			takeaway += " " + uistate.T("rls.shadowClauseOne")
		} else if shadowedCount > 1 {
			takeaway += " " + uistate.T("rls.shadowClauseN", shadowedCount)
		}
		if len(suggestions) > 0 {
			takeaway += " " + uistate.T("rls.suggestClause", len(suggestions))
		}
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-rules-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), eyebrow),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("rules.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), fmt.Sprintf("%d%%", coveredPct)),
			),
		),
		Div(css.Class("debt-chips"), chips),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "rules-takeaway"), takeaway),
	)
	heroTile := rptTile("rules-hero", "1 / span 4", rptSection("", uistate.T("rules.heroTitle"), nil, heroBody))

	// Section header actions: apply-to-existing + the add-rule modal button.
	headerActions := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		If(len(rs) > 0, Button(css.Class("btn"), Type("button"), Title(uistate.T("rules.applyExistingTitle")), OnClick(applyExisting), uistate.T("rules.applyExisting"))),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "rules-add"), Title(uistate.T("rules.add")), OnClick(addRule),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("rules.addRule"))),
	)

	// Lead with the user's own rules (G18 §1): Your rules → Suggestions
	// (discovery aid) → Rule order (power-user precedence view).
	return Div(css.Class("bento bento-rules"),
		heroTile,
		If(errMsg.Get() != "", rptTile("rules-err", "1 / span 4", P(css.Class("notice-danger"), errMsg.Get()))),
		rptTile("rules-list", "1 / span 4",
			rptSection("sec-rules-list", uistate.T("rules.listTitle"), headerActions, Fragment(
				// GI1: an on-page add-rule row at the top of "Your rules" — match +
				// category + tags + Add, inline — so a rule can be created without
				// opening the modal. Reuses the same RuleAddForm (with its live
				// match-count preview + validation) the modal uses.
				Div(css.Class(tw.Mb2),
					H3(css.Class("set-label"), uistate.T("rules.quickAddTitle")),
					RuleAddForm(RuleAddFormProps{OnDone: func() { uistate.PostNotice(uistate.T("rules.added"), false) }}),
				),
				If(len(rs) > 1, P(css.Class("muted"), uistate.T("rules.dragHint"))),
				list,
			))),
		// Suggestions surface above the precedence chain (C38): users discover
		// AI-suggested rules before the power-user precedence view.
		If(len(suggestions) > 0, rptTile("rules-suggest", "1 / span 4", suggestCard)),
		// Precedence chain: first match wins, top to bottom; shadowed rules flagged
		// (C70/C64). Rendered natively in the surface's own language (a numbered
		// spine) — the old Mermaid flowchart wore the library's stock lavender theme.
		If(len(rs) > 1, rptTile("rules-order", "1 / span 4",
			rptSection("sec-rules-order", uistate.T("rules.orderTitle"), nil, Fragment(
				P(css.Class("muted"), uistate.T("rules.orderHint")),
				rulesPrecedenceChain(rs, catName, warnByID),
			)))),
	)
}

// rulesPrecedenceChain renders the first-match-wins order as a numbered spine:
// each link shows its precedence number, the match phrase, and the category it
// files into; shadowed rules dim with their warning inline. A native replacement
// for the stock-themed Mermaid flowchart (aria-label preserved).
func rulesPrecedenceChain(rs []rules.Rule, catName map[string]string, warnByID map[string]string) ui.Node {
	items := []any{css.Class("rule-chain"), Attr("role", "list"), Attr("aria-label", uistate.T("rules.precedenceLabel"))}
	for i, r := range rs {
		cat := catName[r.SetCategoryID]
		if cat == "" {
			cat = uistate.T("rules.unknownCategory")
		}
		cls := "rule-chain-item"
		if warnByID[r.ID] != "" {
			cls += " rule-chain-shadowed"
		}
		items = append(items, Div(ClassStr(cls), Attr("role", "listitem"),
			Span(css.Class("rule-chain-n", tw.FontDisplay), fmt.Sprintf("%d", i+1)),
			Div(css.Class("rule-chain-body"),
				Span(css.Class("rule-chain-match"), uistate.T("rules.matchLabel", r.Match)),
				Span(css.Class("rule-chain-cat"), "→ "+cat),
				If(warnByID[r.ID] != "", Span(css.Class("rule-chain-warn", tw.TextWarn), warnByID[r.ID])),
			),
		))
	}
	return Div(items...)
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
		Button(css.Class("btn"), Type("button"), Title(uistate.T("rules.acceptTitle")), OnClick(add), uistate.T("rules.accept")),
	)
}

type ruleRowProps struct {
	Rule           rules.Rule
	CategoryName   string
	Warning        string // non-empty when this rule never fires (shadowed)
	MatchCount     int    // how many existing transactions this rule's phrase hits
	MaxMatchCount  int    // the heaviest rule's count — anchors the weight bar
	ShowMatchCount bool   // whether to show the count (there are transactions to count)
	OnDelete       func(string)
	OnDragStart    func()
	OnDrop         func()
}

// RuleRow is a per-rule weighted ledger row. Edit opens the shell-root flip
// modal (RuleEditHost), which — unlike the old inline form — preserves the
// rule's precedence Order and structured Conditions on save.
func RuleRow(props ruleRowProps) ui.Node {
	r := props.Rule
	del := ui.UseEvent(Prevent(func() { props.OnDelete(r.ID) }))
	startEdit := ui.UseEvent(Prevent(func() { uistate.SetRuleEdit(r.ID) }))

	target := props.CategoryName
	if target == "" {
		target = uistate.T("rules.unknownCategory")
	}
	meta := uistate.T("rules.appliesTo", target)
	if len(r.SetTags) > 0 {
		meta += " · " + strings.Join(r.SetTags, ", ")
	}
	// C102: surface the rename action in the read-only row so users can see it fires.
	if r.RenameDesc != "" {
		meta += " · " + uistate.T("rules.renameDescMeta", r.RenameDesc)
	}

	// The rule's weight: how many transactions its phrase catches, as a figure
	// column + a share bar against the heaviest rule.
	var bar ui.Node = Fragment()
	if props.ShowMatchCount && props.MaxMatchCount > 0 && props.MatchCount > 0 {
		pct := props.MatchCount * 100 / props.MaxMatchCount
		bar = Div(css.Class("share-bar", "share-bar-thin"),
			Div(css.Class("share-bar-fill"), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
	}
	var figure ui.Node = Fragment()
	if props.ShowMatchCount {
		figure = Div(css.Class("rule-figure"),
			Span(css.Class("rule-figure-n"), plural(props.MatchCount, "transaction")),
			Span(css.Class("rule-figure-sub"), uistate.T("rules.caughtSub")),
		)
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
			Span(css.Class("row-desc"), Span(css.Class("rule-match"), uistate.T("rules.matchLabel", r.Match))),
			Span(css.Class("row-meta"), meta),
			If(props.Warning != "", Span(css.Class("row-meta", tw.TextWarn), props.Warning)),
			bar,
		),
		figure,
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("rules.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "rule-menu-" + r.ID,
			AriaLabel:    uistate.T("rules.menuAria"),
			ToggleTestID: "rule-menu-btn-" + r.ID,
			Items: []ui.Node{
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "rule-delete-"+r.ID), Attr("aria-label", uistate.T("rules.deleteTitle")),
					Title(uistate.T("rules.deleteTitle")), OnClick(del), uistate.T("rules.deleteTitle")),
			},
		}),
	)
}
