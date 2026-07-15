// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/rulesuggest"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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

	// In-context add (G18 §1): the header's "+ Add rule" jumps to the on-page
	// quick-add form (the page previously offered the same form TWICE — inline
	// AND in a modal — with nothing explaining the difference).
	addRule := ui.UseEvent(Prevent(func() { focusByID("rule-quick-match") }))

	errMsg := ui.UseState("")
	dragSrc := ui.UseState("") // id of the rule being dragged (precedence reorder, C64)
	notice := uistate.UseNotice()

	// SMART-T14: Smart+ rule suggestions — an opt-in AI scan of the transactions
	// no rule covers yet. Hooks declared unconditionally at stable positions.
	pr := uistate.UsePrefs().Get()
	backendAI := pr.Normalize().BackendActive()
	hasProvider := aiProviderConfigured(app, backendAI)
	aiConn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	smartOn := uistate.LoadSmartSettings().IsEnabled("SMART-T14")
	aiSugs := ui.UseState([]smartai.SuggestedRule(nil))
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	aiScanned := ui.UseState(false)

	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	acctName := func(id string) string {
		for _, a := range app.Accounts() {
			if a.ID == id {
				return a.Name
			}
		}
		return id
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	fmtAmt := func(minor int64) string { return fmtMoney(money.New(minor, base)) }
	// condLine renders a rule's structured conditions in plain English ("" when
	// it has none) — the row, the precedence chain, and notices all read it.
	condLine := func(r rules.Rule) string {
		if len(r.Conditions) == 0 {
			return ""
		}
		return ruleCondsEnglish(r.Conditions, acctName, fmtAmt)
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
	// Full transaction contexts (transfers excluded, mirroring the engine at
	// entry/import) so counts, coverage, and the authoring preview evaluate
	// structured conditions exactly like FirstMatchFull.
	ctxs := ruleTxnCtxs(app)

	// runSmartScan builds the SMART-T14 context at click time (uncovered
	// transactions + the household's category names) and places the AI call
	// under the product routing policy. Results parse against the REAL category
	// list, so the model can never invent a category.
	runSmartScan := func() {
		currentRules := app.Rules()
		var txnLines strings.Builder
		n := 0
		for _, t := range app.Transactions() {
			if t.IsTransfer() {
				continue
			}
			if rules.FirstMatchFull(currentRules, t.Payee, t.Desc, t.Amount.Amount, t.AccountID, rules.NewTxnDate(t.Date)) != nil {
				continue // a rule already handles it
			}
			line := strings.TrimSpace(t.Payee + " — " + t.Desc)
			cur := catName[t.CategoryID]
			if cur == "" {
				cur = "(uncategorized)"
			}
			txnLines.WriteString("- " + line + " | " + fmtMoney(t.Amount) + " | currently: " + cur + "\n")
			if n++; n >= 40 {
				break
			}
		}
		if n == 0 {
			aiSugs.Set(nil)
			aiScanned.Set(true)
			aiErr.Set("")
			return
		}
		catIDByName := make(map[string]string, len(cats))
		var catList strings.Builder
		for _, c := range cats {
			catIDByName[c.Name] = c.ID
			catList.WriteString(c.Name + "\n")
		}
		aiLoading.Set(true)
		aiErr.Set("")
		runSmartAI(aiConn, smartai.RuleSuggest(txnLines.String(), catList.String()),
			func(text string) {
				parsed := smartai.ParseRuleSuggestions(text, catIDByName)
				// Drop anything an existing rule's phrase already covers.
				var fresh []smartai.SuggestedRule
				for _, s := range parsed {
					if rules.FirstMatch(currentRules, s.Match) == nil {
						fresh = append(fresh, s)
					}
				}
				aiSugs.Set(fresh)
				aiScanned.Set(true)
				aiLoading.Set(false)
			},
			func(errText string) {
				aiErr.Set(errText)
				aiScanned.Set(true)
				aiLoading.Set(false)
			})
	}
	onSmartScan := ui.UseEvent(Prevent(runSmartScan))
	onSmartToggle := func(on bool) {
		uistate.SetSmartFeatureEnabled("SMART-T14", on)
		if !on {
			aiSugs.Set(nil)
			aiScanned.Set(false)
			aiErr.Set("")
		}
		bump()
	}

	// ruleDisplayLabel names a rule for notices: the match phrase, or its
	// conditions in plain English for a condition-bearing rule.
	ruleDisplayLabel := func(r rules.Rule) string {
		if l := condLine(r); l != "" {
			return l
		}
		return r.Match
	}

	// Apply-to-existing is an irreversible bulk overwrite — preview the blast
	// radius first (dry-run, conditions-aware) and confirm before writing.
	applyExisting := ui.UseEvent(Prevent(func() {
		currentRules := app.Rules()
		total, perRule := app.PreviewApplyRules()
		if total == 0 {
			notice.Set(notice.Get().With(uistate.T("rules.appliedNone"), false))
			return
		}
		var parts []string
		for _, r := range currentRules {
			if cnt, ok := perRule[r.ID]; ok {
				parts = append(parts, uistate.T("rules.appliedPerRule", ruleDisplayLabel(r), plural(cnt, "transaction")))
			}
		}
		confirmMsg := uistate.T("rules.applyConfirm", plural(total, "transaction"))
		if len(parts) > 0 {
			confirmMsg += " " + strings.Join(parts, ", ") + "."
		}
		uistate.ConfirmModal(confirmMsg, false, func(ok bool) {
			if !ok {
				return
			}
			n, _, err := app.ApplyRulesWithCounts()
			if err != nil {
				notice.Set(notice.Get().With(err.Error(), true))
				return
			}
			notice.Set(notice.Get().With(uistate.T("rules.applied", plural(n, "transaction")), false))
			bump()
		})
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
	// signal (L15), evaluated with FULL transaction context so condition-bearing
	// rules count honestly (a plain-text count read "0 caught" for a rule that
	// catches hundreds). maxMatch anchors the per-rule weight bars.
	covered := rules.CoveredFull(rs, ctxs)
	hasTxns := len(ctxs) > 0
	matchCounts := make(map[string]int, len(rs))
	maxMatch := 0
	for _, r := range rs {
		n := r.MatchCountFull(ctxs)
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
	// moveRule shifts a rule one step up or down the precedence chain — the
	// keyboard-reachable complement to drag-to-reorder (the grip is mouse-only).
	moveRule := func(id string, delta int) {
		idx := -1
		for i, r := range rs {
			if r.ID == id {
				idx = i
				break
			}
		}
		j := idx + delta
		if idx < 0 || j < 0 || j >= len(rs) {
			return
		}
		res := make([]string, len(rs))
		for i, r := range rs {
			res[i] = r.ID
		}
		res[idx], res[j] = res[j], res[idx]
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
					Rule: r, CategoryName: catName[r.SetCategoryID], CondLine: condLine(r),
					Warning: warnByID[r.ID], MatchCount: matchCounts[r.ID], MaxMatchCount: maxMatch, ShowMatchCount: hasTxns,
					OnDelete:    deleteRule,
					OnMove:      moveRule,
					OnDragStart: func() { dragSrc.Set(rid) },
					OnDrop:      func() { reorder(rid) },
				})
			},
		)),
	)

	acceptSuggestion := func(r rules.Rule) {
		r.ID = id.New()
		// Accepted suggestions append to the END of the chain (see RuleAddForm).
		r.Order = app.NextRuleOrder()
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
	if len(ctxs) > 0 {
		coveredPct = covered * 100 / len(ctxs)
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
		takeaway = uistate.T("rls.coverTake", fmt.Sprintf("%d", covered), len(ctxs))
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
		Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "rules-add"), Title(uistate.T("rules.add")), OnClick(addRule),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
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
					RuleAddForm(RuleAddFormProps{MatchInputID: "rule-quick-match", OnDone: func() { uistate.PostNotice(uistate.T("rules.added"), false) }}),
				),
				If(len(rs) > 1, P(css.Class("muted"), uistate.T("rules.dragHint"))),
				list,
			))),
		// TX1: merchant-name (payee alias) management — clean up processor-noise
		// payee names once; they display cleanly everywhere. Additive tile.
		PayeeAliasSection(),
		// Suggestions surface above the precedence chain (C38): users discover
		// AI-suggested rules before the power-user precedence view.
		If(len(suggestions) > 0, rptTile("rules-suggest", "1 / span 4", suggestCard)),
		// SMART-T14: the opt-in Smart+ AI scan, between the deterministic
		// suggestions and the precedence chain.
		If(hasTxns, rptTile("rules-smart", "1 / span 4", rulesSmartSection(rulesSmartSectionArgs{
			On:          smartOn,
			HasProvider: hasProvider,
			Loading:     aiLoading.Get(),
			Scanned:     aiScanned.Get(),
			Err:         aiErr.Get(),
			Suggestions: aiSugs.Get(),
			OnToggle:    onSmartToggle,
			OnScan:      onSmartScan,
			OnAdd: func(s smartai.SuggestedRule) {
				acceptSuggestion(rules.Rule{Match: s.Match, SetCategoryID: s.CategoryID})
				// Drop the accepted suggestion from the pending list.
				cur := aiSugs.Get()
				next := make([]smartai.SuggestedRule, 0, len(cur))
				for _, c := range cur {
					if c.Match != s.Match {
						next = append(next, c)
					}
				}
				aiSugs.Set(next)
			},
		}))),
		// Precedence chain: first match wins, top to bottom; shadowed rules flagged
		// (C70/C64). Rendered natively in the surface's own language (a numbered
		// spine) — the old Mermaid flowchart wore the library's stock lavender theme.
		If(len(rs) > 1, rptTile("rules-order", "1 / span 4",
			rptSection("sec-rules-order", uistate.T("rules.orderTitle"), nil, Fragment(
				P(css.Class("muted"), uistate.T("rules.orderHint")),
				rulesPrecedenceChain(rs, catName, warnByID, condLine),
			)))),
	)
}

// rulesPrecedenceChain renders the first-match-wins order as a numbered spine:
// each link shows its precedence number, the match phrase, and the category it
// files into; shadowed rules dim with their warning inline. A native replacement
// for the stock-themed Mermaid flowchart (aria-label preserved).
func rulesPrecedenceChain(rs []rules.Rule, catName map[string]string, warnByID map[string]string, condLine func(rules.Rule) string) ui.Node {
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
		identity := uistate.T("rules.matchLabel", r.Match)
		if l := condLine(r); l != "" {
			identity = uistate.T("rules.condLabel", l)
		}
		items = append(items, Div(ClassStr(cls), Attr("role", "listitem"),
			Span(css.Class("rule-chain-n", tw.FontDisplay), fmt.Sprintf("%d", i+1)),
			Div(css.Class("rule-chain-body"),
				Span(css.Class("rule-chain-match"), identity),
				Span(css.Class("rule-chain-cat"), "→ "+cat),
				If(warnByID[r.ID] != "", Span(css.Class("rule-chain-warn", tw.TextWarn), warnByID[r.ID])),
			),
		))
	}
	return Div(items...)
}

// validateRuleInput returns the i18n key of the first problem with a rule's
// match/category, or "" when the rule is saveable. A match phrase is required
// UNLESS structured conditions are set (conditions override the phrase at
// evaluation time, so a pure-conditions rule is legitimate — C105). Keeps the
// raw appstate error out of the UI by checking the same invariants client-side.
// ruleBillAccountOptions builds the account picker options for a rule's "link as bill
// payment" action: a leading "no bill account" option plus every non-archived account.
func ruleBillAccountOptions(app *appstate.App) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("rules.billNone")}}
	if app == nil {
		return opts
	}
	for _, a := range app.Accounts() {
		if !a.Archived {
			opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
		}
	}
	return opts
}

func validateRuleInput(match string, hasConditions, hasAction bool) string {
	if strings.TrimSpace(match) == "" && !hasConditions {
		return "rules.matchRequired"
	}
	if !hasAction {
		return "rules.actionRequired"
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
	Rule         rules.Rule
	CategoryName string
	// CondLine is the rule's structured conditions in plain English ("" when the
	// rule matches by phrase) — shown as the row's identity so a condition rule
	// isn't a black box behind an ignored match phrase.
	CondLine       string
	Warning        string // non-empty when this rule never fires (shadowed)
	MatchCount     int    // how many existing transactions this rule's phrase hits
	MaxMatchCount  int    // the heaviest rule's count — anchors the weight bar
	ShowMatchCount bool   // whether to show the count (there are transactions to count)
	OnDelete       func(string)
	OnMove         func(id string, delta int) // keyboard-reachable precedence nudge (in the row menu)
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
	moveUp := ui.UseEvent(Prevent(func() {
		if props.OnMove != nil {
			props.OnMove(r.ID, -1)
		}
	}))
	moveDown := ui.UseEvent(Prevent(func() {
		if props.OnMove != nil {
			props.OnMove(r.ID, 1)
		}
	}))

	target := props.CategoryName
	if target == "" {
		target = uistate.T("rules.unknownCategory")
	}
	// A condition rule's identity IS its conditions (the engine ignores the
	// phrase when conditions are set) — say so instead of showing a dead phrase.
	identity := uistate.T("rules.matchLabel", r.Match)
	if props.CondLine != "" {
		identity = uistate.T("rules.condLabel", props.CondLine)
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
			Span(css.Class("row-desc"), Span(css.Class("rule-match"), identity)),
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
				// Keyboard-reachable precedence nudges (the drag grip is mouse-only).
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "rule-moveup-"+r.ID), OnClick(moveUp), uistate.T("rules.moveUp")),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "rule-movedown-"+r.ID), OnClick(moveDown), uistate.T("rules.moveDown")),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "rule-delete-"+r.ID), Attr("aria-label", uistate.T("rules.deleteTitle")),
					Title(uistate.T("rules.deleteTitle")), OnClick(del), uistate.T("rules.deleteTitle")),
			},
		}),
	)
}

// ruleCondsEnglish renders structured rule conditions as one plain-English
// clause ("amount over $100 and payee contains \u201cuber\u201d-style text) so
// condition rules are never a black box behind an ignored match phrase (C105).
func ruleCondsEnglish(conds []rules.RuleCondition, acctName func(string) string, fmtAmt func(int64) string) string {
	parts := make([]string, 0, len(conds))
	for _, c := range conds {
		parts = append(parts, ruleCondEnglish(c, acctName, fmtAmt))
	}
	return strings.Join(parts, uistate.T("rls.cond.joiner"))
}

// ruleCondEnglish renders one structured condition in plain English. Unknown
// field/op combinations fall back to a literal "field op value" so nothing is
// silently hidden.
func ruleCondEnglish(c rules.RuleCondition, acctName func(string) string, fmtAmt func(int64) string) string {
	v := strings.TrimSpace(c.Value)
	switch c.Field {
	case rules.ConditionFieldPayee, rules.ConditionFieldDescription:
		word := uistate.T("rls.cond.fieldPayee")
		if c.Field == rules.ConditionFieldDescription {
			word = uistate.T("rls.cond.fieldDesc")
		}
		if c.Op == rules.ConditionOpEquals {
			return uistate.T("rls.cond.textEquals", word, v)
		}
		return uistate.T("rls.cond.textContains", word, v)
	case rules.ConditionFieldAmount:
		amt := v
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			amt = fmtAmt(n)
		}
		switch c.Op {
		case rules.ConditionOpGt:
			return uistate.T("rls.cond.amtGt", amt)
		case rules.ConditionOpGte:
			return uistate.T("rls.cond.amtGte", amt)
		case rules.ConditionOpLt:
			return uistate.T("rls.cond.amtLt", amt)
		case rules.ConditionOpLte:
			return uistate.T("rls.cond.amtLte", amt)
		case rules.ConditionOpEq:
			return uistate.T("rls.cond.amtEq", amt)
		case rules.ConditionOpNeq:
			return uistate.T("rls.cond.amtNeq", amt)
		}
	case rules.ConditionFieldAccount:
		if c.Op == rules.ConditionOpIsNot {
			return uistate.T("rls.cond.acctIsNot", acctName(v))
		}
		return uistate.T("rls.cond.acctIs", acctName(v))
	case rules.ConditionFieldDate:
		switch c.Op {
		case rules.ConditionOpInMonth:
			return uistate.T("rls.cond.dateInMonth", v)
		case rules.ConditionOpOn:
			return uistate.T("rls.cond.dateOn", v)
		case rules.ConditionOpBefore:
			return uistate.T("rls.cond.dateBefore", v)
		case rules.ConditionOpAfter:
			return uistate.T("rls.cond.dateAfter", v)
		}
	}
	return string(c.Field) + " " + string(c.Op) + " " + v
}

// rulesSmartSectionArgs bundles the SMART-T14 section's render inputs — the
// caller (Rules) owns all the state and hooks.
type rulesSmartSectionArgs struct {
	On          bool
	HasProvider bool
	Loading     bool
	Scanned     bool
	Err         string
	Suggestions []smartai.SuggestedRule
	OnToggle    func(bool)
	OnScan      ui.Handler
	OnAdd       func(smartai.SuggestedRule)
}

// rulesSmartSection renders the opt-in Smart+ AI rule-suggestion panel: the
// toggle, the scan/rescan button with its loading state, and parsed
// suggestions as add-able rows. Off by default — the scan sends transaction
// details to the configured AI provider only when the user opts in AND clicks.
func rulesSmartSection(a rulesSmartSectionArgs) ui.Node {
	body := []any{
		P(css.Class("muted"), uistate.T("rules.smartHint")),
		Div(Attr("data-testid", "rules-smart-toggle"), uiw.ToggleRow(uiw.ToggleRowProps{Label: uistate.T("rules.smartToggle"), On: a.On, OnChange: a.OnToggle})),
	}
	switch {
	case a.On && !a.HasProvider:
		body = append(body, P(css.Class("muted"), uistate.T("smart.aiNeedsProvider")))
	case a.On:
		scanLabel := uistate.T("rules.smartScan")
		if a.Scanned || len(a.Suggestions) > 0 {
			scanLabel = uistate.T("rules.smartRescan")
		}
		controls := []any{css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap)}
		if a.Loading {
			controls = append(controls,
				Button(css.Class("btn"), Type("button"), Attr("disabled", "disabled"), Attr("data-testid", "rules-smart-scan"), scanLabel),
				Span(css.Class("muted"), Attr("role", "status"), uistate.T("rules.smartScanning")))
		} else {
			controls = append(controls,
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "rules-smart-scan"), OnClick(a.OnScan), scanLabel))
		}
		body = append(body, Div(controls...))
		if a.Err != "" {
			body = append(body, P(css.Class("notice-danger"), Attr("data-testid", "rules-smart-err"), a.Err))
		}
		if a.Scanned && !a.Loading && a.Err == "" && len(a.Suggestions) == 0 {
			body = append(body, P(css.Class("muted"), Attr("data-testid", "rules-smart-empty"), uistate.T("rules.smartEmpty")))
		}
		if len(a.Suggestions) > 0 {
			body = append(body, hhRowsList(MapKeyed(a.Suggestions,
				func(s smartai.SuggestedRule) any { return s.Match },
				func(s smartai.SuggestedRule) ui.Node {
					return ui.CreateElement(smartSuggestionRow, smartSuggestionRowProps{Suggestion: s, OnAdd: a.OnAdd})
				})))
		}
	}
	return rptSection("sec-rules-smart", uistate.T("rules.smartTitle"), nil, Fragment(body...))
}

type smartSuggestionRowProps struct {
	Suggestion smartai.SuggestedRule
	OnAdd      func(smartai.SuggestedRule)
}

// smartSuggestionRow renders one AI-suggested rule with its Add button. Its own
// component so the click hook stays out of the variable-length list.
func smartSuggestionRow(props smartSuggestionRowProps) ui.Node {
	s := props.Suggestion
	add := ui.UseEvent(Prevent(func() { props.OnAdd(s) }))
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), uistate.T("rules.suggestionDesc", s.Match, s.CategoryName)),
			Span(css.Class("row-meta"), uistate.T("rules.smartMeta")),
		),
		Button(css.Class("btn"), Type("button"), Title(uistate.T("rules.acceptTitle")),
			Attr("data-testid", "rules-smart-add"), OnClick(add), uistate.T("rules.accept")),
	)
}
