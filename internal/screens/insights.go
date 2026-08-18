// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/anomalyattrib"
	"github.com/monstercameron/CashFlux/internal/anomalyprobe"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/chatpolish"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/flagverdict"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/insights/localqa"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/toolcite"
	"github.com/monstercameron/CashFlux/internal/toolperm"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Insights is AI analysis (OpenAI, client-side, bring-your-own-key): an
// "Explain my month" narrative generated from the user's live figures.
func Insights() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	settings := app.Settings()
	key := settings.OpenAIKey
	// AG18: resolve the effective endpoint — a user-supplied OpenAI-compatible base
	// URL (Ollama/LM Studio/proxy) wins, else the OpenAI default.
	aiBaseURL := aiprovider.ResolveBaseURL(settings.OpenAIBaseURL, ai.DefaultBaseURL)
	// AG17: the active conversation's privacy tier (full | aggregates-only). Read at a
	// stable hook slot; it gates both the injected context and which tools are offered.
	privacyTier := uistate.UsePrivacyTier()
	tier := privacyTier.Get()
	pr := uistate.UsePrefs().Get().Normalize()
	// MIA-extend (#445-9): read the active scope at a stable hook slot so all
	// spend calcs below can be pre-filtered consistently.
	insightsScopeAtom := uistate.UseActiveScope()
	useBackendAI := pr.BackendActive()
	// Model + thinking level are adjustable inline from the assistant header (a quick
	// switch, no trip to Settings). Seed from the saved settings; picks persist back so
	// they stick and stay in sync with Settings. modelList is populated live from
	// OpenAI's /v1/models when a key is set (falls back to the built-in defaults).
	initModel := settings.OpenAIModel
	if initModel == "" {
		initModel = "gpt-5.4-mini"
	}
	modelSel := ui.UseState(initModel)
	effortSel := ui.UseState(settings.OpenAIReasoningEffort)
	modelList := ui.UseState([]string{})
	ui.UseEffect(func() func() {
		if k := strings.TrimSpace(settings.OpenAIKey); k != "" {
			ai.FetchModels(k, aiBaseURL, func(ids []string) { modelList.Set(ids) }, func(string) {})
		}
		return nil
	}, "")
	pickEffort := func(v string) {
		effortSel.Set(v)
		s := app.Settings()
		s.OpenAIReasoningEffort = v
		_ = app.PutSettings(s)
	}
	model := modelSel.Get()
	// Reasoning models (o-series, gpt-5.x) reject a non-default temperature on
	// /chat/completions, so omit it (0 is dropped by omitempty) for them; other
	// models get a mild 0.4. This keeps the chat working whatever model is picked.
	chatTemp := 0.4
	if reasoningModel(model) {
		chatTemp = 0
	}
	// Thinking level (reasoning effort) applies to any reasoning model (o-series /
	// gpt-5.x). The assistant sends it via the Responses API, which accepts effort with
	// function tools, so it works for all reasoning models. Medium is the default.
	thinkingApplies := reasoningModel(model)
	chatEffort := ""
	if thinkingApplies {
		if chatEffort = effortSel.Get(); chatEffort == "" {
			chatEffort = "medium"
		}
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: settings.FXRates}

	accounts := app.Accounts()
	txns := app.Transactions()
	// MIA-extend (#445-9): apply the active scope to transactions so all spend
	// calcs (income/expense, topCatSpend, highlights, merchants, chart, series)
	// reflect the user's chosen scope. Household NW stays unscoped — it is an
	// account-level aggregate and the scoped tile lives on the dashboard.
	insightsSc := insightsScopeAtom.Get()
	insightsInstOf := func(a domain.Account) string { return a.Institution }
	insightsIDs := scope.ResolveScope(accounts, insightsSc, insightsInstOf)
	scopedTxns := scope.ApplyScopeToTxns(txns, insightsIDs)
	net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
	mStart, mEnd := dateutil.MonthRange(time.Now())
	income, expense, _ := ledger.PeriodTotals(scopedTxns, mStart, mEnd, rates)
	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}
	// The only financial data sent to the model: aggregates, no PII (see ai.FinancialContext).
	aiCtx := ai.FinancialContext{NetWorth: fmtMoney(net), Income: fmtMoney(income), Spending: fmtMoney(expense), Accounts: active}

	// Starter questions for the Ask box (L8): tailored to the user's live data so
	// a blank box never stalls them — top spend category, a near-limit budget, and
	// a near-target goal (C59: fuller context means more useful starter questions).
	// Starter questions are derived from three full-dataset scans (top spend
	// category, nearest-limit budget, soonest goal). They're pure over the data +
	// scope + month, so memoize them: the chat page re-renders on every keystroke,
	// send, and effect, and recomputing these each time (a topCatSpend scan of every
	// transaction + budget evaluation + a goals pass) was pure waste.
	starters := ui.UseMemo(func() []string {
		topCatSpend := map[string]int64{}
		for _, t := range scopedTxns {
			if t.IsExpense() && dateutil.InRange(t.Date, mStart, mEnd) {
				if conv, err := rates.Convert(t.Amount.Abs(), base); err == nil {
					topCatSpend[t.CategoryID] += conv.Amount
				}
			}
		}
		topCat := ""
		var topAmt int64
		for _, c := range app.Categories() { // category order → deterministic on ties
			if topCatSpend[c.ID] > topAmt {
				topAmt, topCat = topCatSpend[c.ID], c.Name
			}
		}

		// Near-limit budget: the budget closest to (or over) its limit this month.
		nearLimitBudget := ""
		if statuses, err := budgeting.EvaluateAll(app.Budgets(), txns, mStart, mEnd, rates, budgeting.DefaultNearThreshold); err == nil {
			for _, s := range statuses {
				if s.State == budgeting.StateNear || s.State == budgeting.StateOver {
					nearLimitBudget = s.Budget.Name
					break // first near/over budget (EvaluateAll order matches Budgets order)
				}
			}
		}

		// Upcoming goal: the active goal with the nearest non-zero target date.
		upcomingGoal := ""
		now := time.Now()
		var soonest time.Time
		for _, g := range app.Goals() {
			if g.Archived || g.TargetDate.IsZero() || !g.TargetDate.After(now) {
				continue
			}
			if soonest.IsZero() || g.TargetDate.Before(soonest) {
				soonest = g.TargetDate
				upcomingGoal = g.Name
			}
		}

		// EC-16: the situational half — what is true about the data RIGHT NOW, so
		// the first chip names something real rather than asking a reasonable
		// question in general.
		overBudgets, uncategorized := 0, 0
		if statuses, err := budgeting.EvaluateAll(app.Budgets(), txns, mStart, mEnd, rates, budgeting.DefaultNearThreshold); err == nil {
			for _, st := range statuses {
				if st.State == budgeting.StateOver {
					overBudgets++
				}
			}
		}
		var largestPayee, largestAmount string
		var largestMinor int64
		for _, t := range scopedTxns {
			if !dateutil.InRange(t.Date, mStart, mEnd) {
				continue
			}
			if t.CategoryID == "" && t.IsExpense() && !t.IsTransfer() {
				uncategorized++
			}
			if !t.IsExpense() || t.IsTransfer() {
				continue
			}
			amount := t.Amount.Abs()
			if conv, err := rates.Convert(amount, base); err == nil {
				amount = conv
			}
			if amount.Amount > largestMinor {
				largestMinor, largestPayee = amount.Amount, txnDisplayPayee(t)
				largestAmount = insightsMoneyFmt(amount.Amount, base)
			}
		}
		return insights.SuggestedQuestions(insights.QuestionContext{
			TopCategory:          topCat,
			NearLimitBudget:      nearLimitBudget,
			UpcomingGoal:         upcomingGoal,
			OverBudgetCount:      overBudgets,
			UncategorizedCount:   uncategorized,
			LargestExpensePayee:  largestPayee,
			LargestExpenseAmount: largestAmount,
		})
	}, app.Rev(), fmt.Sprintf("%v", insightsSc), mStart.Unix())

	nav := router.UseNavigate()
	// The no-key hint is a clear call to action that hops to Settings (where the AI
	// key lives), not a dead-end sentence (C59; same fix as C54). Built fresh per
	// use so the two placements get independent button nodes.
	// C247: enrich the no-key gate with cost/where-to-get/privacy context so users
	// understand BYOK before navigating away to Settings.
	// Mid-conversation, the keyless fact is a slim one-line strip — not a 4-line
	// essay stacked under the composer competing with it. The full pitch (cost,
	// privacy, where-to-get) lives once in the empty-thread intro callout.
	keyHintNode := func() ui.Node {
		return Div(css.Class("asst-keystrip"), Attr("data-testid", "assistant-keynote"),
			Span(css.Class("asst-keystrip-dot"), Attr("aria-hidden", "true")),
			Span(css.Class(tw.Text12, tw.TextDim), uistate.T("insights.keyHint")),
			Button(css.Class("btn-link", tw.Text12), Type("button"), OnClick(func() { uistate.OpenGlobalSettingsAt("ai") }), uistate.T("nav.settings")),
		)
	}

	// The Insights screen is a chat with the CashFlux assistant (C82 wiring): a
	// conversation thread the user types into, answered from their own figures.
	turns := ui.UseState([]chatTurn{})
	// The composer is deliberately NOT bound to a state that changes per keystroke.
	//
	// `value` is a special property the reconciler always writes, and it diffs the
	// new prop against the PREVIOUS RENDER'S prop rather than against what the box
	// actually holds. Binding it to per-keystroke state therefore meant every render
	// wrote a possibly-stale string back over the box: typing 76 characters landed 13
	// of them, scrambled, with the caret jumping to the end. It also re-rendered this
	// whole screen — thread, rail, hero, meters — once per character, at ~33ms a key.
	//
	// So the box owns its own text, and `composerSeed` moves ONLY when the app has
	// something to say (a send clearing it, an Explain chip prefilling it, a new
	// chat). While somebody types, the prop is unchanged, the diff skips the write
	// entirely, and the DOM keeps exactly what they typed. Anything that needs the
	// current text reads the element, which is always right.
	composerSeed := ui.UseState("")
	// composerFilled tracks only whether the box is empty, because that is all the
	// render depends on (the starter chips hide once there is a question in
	// progress). It flips at most twice per message instead of once per character.
	composerFilled := ui.UseState(false)
	// composerText reads what the composer currently holds.
	composerText := func() string {
		el := js.Global().Get("document").Call("getElementById", "cf-chat-input")
		if !el.Truthy() {
			return ""
		}
		return el.Get("value").String()
	}
	// setComposer puts text in the box programmatically. It writes the DOM as well as
	// the seed because the two most common values — "" on send and "" already — are
	// equal, and an unchanged prop is skipped by the very diff that makes typing work.
	setComposer := func(v string) {
		composerSeed.Set(v)
		composerFilled.Set(v != "")
		if el := js.Global().Get("document").Call("getElementById", "cf-chat-input"); el.Truthy() {
			el.Set("value", v)
		}
	}
	// Shell-style input history: histIdx is the cycle position over prior user messages
	// with Up/Down (-1 = not cycling); histDraft preserves the in-progress draft.
	histIdx := ui.UseState(-1)
	histDraft := ui.UseState("")
	// (fillAsk retired — starter chips now SEND directly, QA CF-26; the Discuss
	// flow attaches context bubbles rather than filling the box.)
	// ctxAttach holds flagged-activity items attached to the composer as context
	// bubbles (the "Discuss" action). They ride along above the input — never dumped
	// into it — and fold into the next message the user sends. removeCtx drops one.
	ctxAttach := ui.UseState([]flagContext{})
	removeCtx := func(cid string) {
		cur := ctxAttach.Get()
		next := make([]flagContext, 0, len(cur))
		for _, c := range cur {
			if c.ID != cid {
				next = append(next, c)
			}
		}
		ctxAttach.Set(next)
	}
	// Conversation id whose AI title generation has been attempted (once per chat).
	namingDone := ui.UseState("")
	// Typing must not re-render the screen. The only thing a render depends on is
	// whether the box is empty, so this flips a boolean at that boundary and is inert
	// for every keystroke in between.
	onInput := ui.UseEvent(func(v string) {
		if filled := v != ""; filled != composerFilled.Get() {
			composerFilled.Set(filled)
		}
	})
	loading := ui.UseState(false)
	errMsg := ui.UseState("")
	rev := ui.UseState(0)
	bump := func() { rev.Set(rev.Get() + 1) }
	var noCancel func()
	cancelFn := ui.UseState(noCancel)
	cancelAI := ui.UseEvent(func() {
		if c := cancelFn.Get(); c != nil {
			c()
		}
		loading.Set(false)
	})
	// The conversation this thread belongs to ("" = a new, unsaved chat). convCreated
	// preserves the original timestamp across saves; inited guards the one-time load.
	convID := ui.UseState("")
	convCreated := ui.UseState(time.Time{})
	inited := ui.UseState(false)
	// Editable system prompt (persona/instructions) — the live data context is always
	// appended separately, so editing this never loses the user's figures/tools.
	promptOpen := ui.UseState(false)
	promptDraft := ui.UseState("")
	// A mutating tool awaiting the user's approval in the thread (nil = none pending).
	pendingApproval := ui.UseState((*approvalReq)(nil))
	// The rail's conversation search (G2-C7). Empty means "show them all".
	convSearch := ui.UseState("")
	// The answer currently being written, fragment by fragment (G2-C7). Empty
	// between turns.
	streaming := ui.UseState("")

	// C390: picking a model sticks to THIS conversation as well as to the household
	// default, so returning to a long analytical thread does not quietly continue it
	// on whatever model was last chosen in a different chat. Defined here rather
	// than beside the other pickers because it needs the conversation's identity.
	pickModel := func(v string) {
		modelSel.Set(v)
		s := app.Settings()
		s.OpenAIModel = v
		_ = app.PutSettings(s)
		uistate.SetAgentModel(convID.Get(), v)
	}
	// pickBudget caps what this conversation may spend in total. It is a per-chat
	// decision, not a global one: "don't let this exploration run away" is a
	// different thing from "cap my month".
	pickBudget := func(tokens int) { uistate.SetAgentBudget(convID.Get(), tokens) }

	// railReady defers the periphery rail's heavy detectors (spend-anomaly + the four
	// SMART anomaly detectors, each a full-transaction scan) to just after first paint.
	// On the initial mount it's false, so the chat renders immediately without those
	// scans on the critical path; the effect flips it true, and the rail fills in a
	// frame later. The hooks below stay unconditional (rule of hooks); only the work
	// inside them is gated.
	railReady := ui.UseState(false)
	ui.UseEffect(func() func() {
		if !railReady.Get() {
			// Fill the rail once the page has settled — after first paint AND the 160ms
			// route cross-fade (var(--wonder-dur)), so the deferred re-render doesn't
			// abort the entrance transition. A Go timer keeps the primary chat
			// interactive immediately; the secondary rail loads a beat later.
			time.AfterFunc(300*time.Millisecond, func() { railReady.Set(true) })
		}
		return nil
	}, "rail-defer-once")

	// pinText saves an answer to the pinned-insights list. (Saving an answer as a
	// To-do is no longer a UI button — it becomes an agent tool the model invokes
	// when the user asks, C82.)
	pinText := func(text string) bool {
		text = strings.TrimSpace(text)
		if text == "" {
			return false
		}
		if err := app.PutSavedInsight(domain.SavedInsight{ID: id.New(), Text: text, CreatedAt: time.Now()}); err != nil {
			errMsg.Set(err.Error())
			return false
		}
		bump()
		return true
	}
	deletePinned := func(pid string) {
		_ = app.DeleteSavedInsight(pid)
		bump()
	}

	// buildMessages assembles the OpenAI message list: the (optionally user-edited)
	// persona/instructions prompt, then a live data-context system message (aggregates
	// + the user's category names + a directive to call tools for any specific figure),
	// then the conversation so far.
	buildMessages := func(hist []chatTurn) []ai.Message {
		persona := strings.TrimSpace(uistate.LoadSystemPrompt())
		if persona == "" {
			persona = defaultChatSystemPrompt
		}
		ctx := "Live context — " + aiCtx.Line()
		if names := categoryNames(app.Categories()); names != "" {
			ctx += " The user's categories: " + names + "."
		}
		if cfSummary := customFieldsSummary(app.CustomFieldDefs()); cfSummary != "" {
			ctx += " The user's custom fields: " + cfSummary + "."
		}
		ctx += " For any specific number (a category total, an account balance, affordability), CALL A TOOL — never guess or say you lack the data."
		// AG17: in aggregates-only mode, tell the model the boundary explicitly (the
		// transaction/payee tools are also withheld from its tool list, so this is a
		// belt-and-braces statement, not the enforcement itself).
		if tier == aicontext.TierAggregatesOnly {
			ctx += " PRIVACY: this is an aggregates-only conversation. You can see totals and KPIs but NOT individual transactions or payees; do not ask for or claim per-merchant detail."
		}
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: persona},
			{Role: ai.RoleSystem, Content: ctx},
		}
		// AG19: inject the user's transparent, durable memory so standing preferences
		// ("paid biweekly", "don't suggest cutting eating out") ride every turn.
		if mem := uistate.LoadAgentMemory().Prompt(); mem != "" {
			msgs = append(msgs, ai.Message{Role: ai.RoleSystem, Content: mem})
		}
		for _, t := range hist {
			role := ai.RoleUser
			if t.Role == "assistant" {
				role = ai.RoleAssistant
			}
			msgs = append(msgs, ai.Message{Role: role, Content: t.Text})
		}
		return msgs
	}

	// sendTools dispatches one model turn. Both paths advertise the same tools: the
	// direct key goes straight to OpenAI, the shared server key goes through the
	// backend proxy, and neither is the weaker assistant (C551).
	sendTools := func(messages []ai.Message, tools []ai.Tool, onDelta func(string), onResult func(ai.Message, ai.Usage), onErr func(string)) func() {
		if useBackendAI {
			return ai.SendProxyChatToolsStreaming(pr.ServerURL, uistate.EffectiveServerToken(pr.ServerToken), model, messages, chatTemp, chatEffort, tools, onDelta, onResult, onErr)
		}
		// Route the tool loop through the Responses API: it's the only endpoint that
		// accepts reasoning.effort together with function tools for the reasoning models
		// (gpt-5.x / o-series), so the thinking level actually works instead of being
		// rejected by /chat/completions. Streaming rides the same endpoint, so both
		// paths show the answer being written (G2-C7).
		return ai.SendResponsesChatToolsStreaming(key, aiBaseURL, model, messages, chatTemp, chatEffort, tools, onDelta, onResult, onErr)
	}

	// run drives the bounded tool-calling loop: ask the model; if it requests tools,
	// execute them locally and feed the results back; repeat until it answers (or a
	// step cap is hit). It runs in a goroutine, blocking on a channel per turn (Go
	// wasm schedules cooperatively, so the fetch callback resumes it). Turns are set
	// deterministically to the sent history + reply (the stale-base fix), and a shared
	// done channel lets Cancel unblock the loop.
	run := func(hist []chatTurn) {
		errMsg.Set("")
		// C390: a spent budget stops the conversation BEFORE the call, not after.
		// A cap that only notices it was exceeded once the tokens are gone is a
		// receipt, not a budget. The message says how to carry on, because the cap
		// is the user's own and they are allowed to change their mind.
		if uistate.AgentBudgetExhausted(convID.Get()) {
			errMsg.Set(uistate.T("insights.budgetSpent", ai.FormatTokens(uistate.AgentBudget(convID.Get()))))
			return
		}
		loading.Set(true)
		allTools := buildChatTools(app, base, rates, tier)
		// AG17: under aggregates-only, withhold the transaction/payee-detail read tools
		// so the privacy promise holds for tool results too, not just the injected context.
		tools := allTools[:0:0]
		for _, t := range allTools {
			if aicontext.ToolAllowed(t.spec.Function.Name, tier) {
				tools = append(tools, t)
			}
		}
		specs := make([]ai.Tool, len(tools))
		byName := make(map[string]chatTool, len(tools))
		for i, t := range tools {
			specs[i] = t.spec
			byName[t.spec.Function.Name] = t
		}
		msgs := buildMessages(hist)
		done := make(chan struct{})
		doneClosed := false
		closeDone := func() {
			if !doneClosed {
				doneClosed = true
				close(done)
			}
		}
		cancelFn.Set(closeDone)
		var total ai.Usage

		// C387: every tool this answer runs is recorded as it runs, so the reply can
		// show what it was computed from. The tool's own result is kept verbatim —
		// that is the evidence; a citation that only names a source proves nothing.
		var sources []domain.ChatSource

		go func() {
			// Last line of defence. Everything below runs in a goroutine, and an
			// unrecovered panic in a Go/wasm goroutine ends the whole program — the
			// entire app, not the chat. A failure here becomes a message in the
			// thread instead.
			defer func() {
				if r := recover(); r != nil {
					loading.Set(false)
					streaming.Set("")
					errMsg.Set(uistate.T("insights.loopFailed"))
				}
			}()
			for step := 0; step < 6; step++ {
				ch := make(chan agentStep, 1)
				// Each fragment lands in the live-answer atom, which the thread
				// renders as a growing bubble. It is cleared when the turn resolves,
				// at which point the real turn takes over — one answer on screen at
				// any moment, never a draft sitting beside its finished self.
				streaming.Set("")
				fc := sendTools(msgs, specs,
					func(delta string) { streaming.Set(streaming.Get() + delta) },
					func(m ai.Message, u ai.Usage) { ch <- agentStep{msg: m, usage: u} },
					func(e string) { ch <- agentStep{err: e} })
				cancelFn.Set(func() { fc(); closeDone() })

				var r agentStep
				select {
				case r = <-ch:
				case <-done:
					loading.Set(false)
					streaming.Set("")
					return
				}
				if r.err != "" {
					loading.Set(false)
					streaming.Set("")
					errMsg.Set(r.err)
					return
				}
				total.PromptTokens += r.usage.PromptTokens
				total.CompletionTokens += r.usage.CompletionTokens
				total.TotalTokens += r.usage.TotalTokens

				if !ai.WantsTools(r.msg) {
					loading.Set(false)
					streaming.Set("")
					// C516: a completion can come back BILLED but with no content —
					// a reasoning-only turn, a length cap hit before any visible
					// text, or a filtered response. The turn was then created with
					// an empty Text, which rendered as a bubble containing nothing
					// but the token line: "Reply: 479 tokens out · 9,637 in · cost
					// unavailable" and no answer. That is indistinguishable from the
					// assistant being broken, and it is the exact report.
					//
					// An empty answer is a failure, so it is reported as one, with
					// the retry the user would otherwise have to discover by
					// reopening the thread.
					text := r.msg.Content
					if strings.TrimSpace(text) == "" {
						text = uistate.T("insights.emptyReply")
					}
					reply := chatTurn{ID: id.New(), Role: "assistant", Text: text, Usage: total, Sources: sources}
					turns.Set(append(append([]chatTurn{}, hist...), reply))
					// AG20: feed this turn's token spend into the per-conversation
					// receipt (cost estimated from the resolved model).
					cost, costOK := ai.EstimateCostUSD(model, total)
					uistate.AddAgentCost(convID.Get(), total.TotalTokens, cost, costOK)
					// EC-15: and into the household's running AI spend meter, so
					// "what is this costing me?" has an answer here rather than on
					// next month's provider bill.
					app.RecordAISpend("assistant", model, total)
					return
				}
				// This turn asked for tools, so whatever text it streamed was a
				// preamble to that decision, not an answer. Clearing it here rather
				// than at the top of the next step stops it hanging over the tool
				// run — and, for a mutating tool, over the approval card, where a
				// stale half-sentence reads as part of what is being approved.
				streaming.Set("")
				msgs = append(msgs, r.msg)
				for _, tc := range r.msg.ToolCalls {
					args := json.RawMessage(tc.Function.Arguments)
					out := "tool unavailable"
					if tool, ok := byName[tc.Function.Name]; ok {
						// Mutating tools pause for the user's approval in the thread.
						if !tool.mutates {
							out = runReadTool(tc.Function.Name, func() string { return tool.run(args) })
						} else {
							preview := tc.Function.Name
							if tool.preview != nil {
								preview = tool.preview(args)
							}
							resp := make(chan bool, 1)
							pendingApproval.Set(&approvalReq{
								tool:    tc.Function.Name,
								preview: preview,
								perm:    toolperm.For(tc.Function.Name, args),
								resp:    resp,
							})
							var approved bool
							select {
							case approved = <-resp:
							case <-done:
								pendingApproval.Set(nil)
								loading.Set(false)
								streaming.Set("")
								return
							}
							pendingApproval.Set(nil)
							if !approved {
								msgs = append(msgs, ai.ToolResultMessage(tc.ID, tc.Function.Name, "The user declined this change."))
								continue
							}
							// C389/AG20: an approved write runs under the assistant
							// actor and takes its own undo point, so Activity shows
							// who made the change and offers to take it back. Without
							// this, a change the assistant made was indistinguishable
							// in the log from one the household made by hand.
							out = runAssistantWrite(tc.Function.Name, func() string { return tool.run(args) })
							uistate.AddAgentActions(convID.Get(), []string{tc.Function.Name})
						}
					}
					cite := toolcite.For(tc.Function.Name, args)
					sources = append(sources, domain.ChatSource{
						Tool: cite.Tool, Label: cite.Label, Scope: cite.Scope, Evidence: out,
					})
					msgs = append(msgs, ai.ToolResultMessage(tc.ID, tc.Function.Name, out))
				}
			}
			loading.Set(false)
			streaming.Set("")
			errMsg.Set(uistate.T("insights.tooManySteps"))
		}()
	}

	// sendText posts a user turn, then either answers deterministically (for
	// recognised affordability questions) or runs the AI model on the new history.
	sendText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" || loading.Get() {
			return
		}

		// C245: clear any stale key-error before evaluating this new question so a
		// prior "no key" message never persists into the next submission.
		errMsg.Set("")

		// Affordability fast-path: answer from real numbers, no AI key needed.
		if q, ok := insights.ParseAffordQuery(text); ok {
			monthlyNet := income.Amount - expense.Amount // this month's net (minor units)
			ar := insights.AffordAnswer(*q, net.Amount, monthlyNet, 0)
			hist := append(append([]chatTurn{}, turns.Get()...),
				chatTurn{ID: id.New(), Role: "user", Text: text},
				chatTurn{ID: id.New(), Role: "afford", Text: affordCardText(ar, q, base)},
			)
			turns.Set(hist)
			setComposer("")
			ctxAttach.Set(nil)
			histIdx.Set(-1)
			return
		}

		// C244 / QA CF-21: try the deterministic local answerer FIRST — for
		// everyone, keyed or not. A simple aggregate ("how much did we spend on
		// Auto loans last month?") used to ship ~8.6k tokens of context to the
		// model for a figure the local data answers exactly, instantly, and free.
		// Only unmatched questions fall through to the model (or the key hint).
		if intent, matched := localqa.Match(text); matched {
			src := newInsightsQASource(app, base, rates)
			if answer, answered := localqa.Answer(intent, src, text, func(minor int64) string {
				return insightsMoneyFmt(minor, base)
			}); answered {
				hist := append(append([]chatTurn{}, turns.Get()...),
					chatTurn{ID: id.New(), Role: "user", Text: text},
					chatTurn{ID: id.New(), Role: "assistant", Text: answer},
				)
				turns.Set(hist)
				setComposer("")
				ctxAttach.Set(nil)
				histIdx.Set(-1)
				return
			}
		}
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		hist := append(append([]chatTurn{}, turns.Get()...), chatTurn{ID: id.New(), Role: "user", Text: text})
		turns.Set(hist)
		setComposer("")
		ctxAttach.Set(nil)
		histIdx.Set(-1)
		run(hist)
	}

	// withContext folds any attached flag-context bubbles into a short preamble ahead
	// of a message body (the bubbles never live in the editable input); with nothing
	// attached it returns the body unchanged.
	withContext := func(body string) string {
		atts := ctxAttach.Get()
		if len(atts) == 0 {
			return body
		}
		var b strings.Builder
		b.WriteString(uistate.T("assistant.contextPreamble"))
		for _, c := range atts {
			b.WriteString("\n• ")
			b.WriteString(c.Title)
			if c.Detail != "" {
				b.WriteString(": ")
				b.WriteString(c.Detail)
			}
		}
		// AG8: the gathered evidence follows the headline list, so the model reasons
		// over the app's own figures rather than looking them up again — or, worse,
		// answering from the headline alone.
		for _, c := range atts {
			if strings.TrimSpace(c.Brief) != "" {
				b.WriteString("\n\n")
				b.WriteString(c.Brief)
			}
		}
		b.WriteString("\n\n")
		b.WriteString(body)
		return b.String()
	}
	// submitChat sends the composer as one user turn, folding in any attached context.
	submitChat := func() {
		typed := strings.TrimSpace(composerText())
		if typed == "" && len(ctxAttach.Get()) == 0 {
			return
		}
		body := typed
		if body == "" {
			body = uistate.T("assistant.contextDefaultAsk")
		}
		sendText(withContext(body))
	}
	// sendRemediation starts a one-click fix for the attached flag: it sends the chosen
	// remediation instruction (with the flag folded in as context) so the agent proposes
	// the concrete change for the user to approve in-thread — it never mutates directly.
	sendRemediation := func(instruction string) {
		if loading.Get() {
			return
		}
		sendText(withContext(instruction))
	}

	// resendLast re-answers the latest user prompt: drop any trailing assistant
	// reply, then run again (the "redo" action).
	resendLast := func() {
		if loading.Get() {
			return
		}
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		cur := turns.Get()
		i := len(cur)
		for i > 0 && cur[i-1].Role == "assistant" {
			i--
		}
		if i == 0 {
			return
		}
		hist := append([]chatTurn{}, cur[:i]...)
		turns.Set(hist)
		run(hist)
	}

	// editAndResend rewords a past question and asks it again (G2-C7). Everything
	// after the edited turn is dropped, because it answered a question that is no
	// longer being asked — keeping those replies would leave a thread whose answers
	// do not follow from what is above them, and would feed that contradiction back
	// to the model as context.
	editAndResend := func(tid, text string) {
		if loading.Get() {
			return
		}
		cur := turns.Get()
		msgs := make([]domain.ChatMessage, len(cur))
		for i, t := range cur {
			msgs[i] = domain.ChatMessage{ID: t.ID, Role: t.Role, Text: t.Text}
		}
		kept, ok := chatpolish.TruncateForResend(msgs, tid, text)
		if !ok {
			return
		}
		hist := make([]chatTurn, 0, len(kept))
		for i, m := range kept {
			turn := chatTurn{ID: m.ID, Role: m.Role, Text: m.Text}
			// Everything before the edit keeps its own accounting and sources;
			// only the reworded turn starts clean.
			if i < len(kept)-1 && i < len(cur) {
				turn.Usage, turn.Sources, turn.Feedback = cur[i].Usage, cur[i].Sources, cur[i].Feedback
			}
			hist = append(hist, turn)
		}
		turns.Set(hist)
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		run(hist)
	}

	// deleteTurn unravels the thread from the deleted message onward: deleting a
	// message drops it and every later turn (a conversation is a chain, so removing a
	// middle turn would leave a dangling continuation). Uses an explicit Set over the
	// current value (not a functional Update) for the same stale-base reason as onResult.
	deleteTurn := func(tid string) {
		cur := turns.Get()
		idx := -1
		for i, t := range cur {
			if t.ID == tid {
				idx = i
				break
			}
		}
		if idx < 0 {
			return
		}
		turns.Set(append([]chatTurn{}, cur[:idx]...))
	}

	// persist upserts the current thread as a conversation, creating one (and a fresh
	// id + created stamp) on the first message. Title comes from the first user line.
	persist := func(ts []chatTurn) {
		cid := convID.Get()
		if cid == "" {
			if len(ts) == 0 {
				return
			}
			cid = id.New()
			convID.Set(cid)
		}
		created := convCreated.Get()
		if created.IsZero() {
			created = time.Now()
			convCreated.Set(created)
		}
		msgs := make([]domain.ChatMessage, len(ts))
		for i, t := range ts {
			msgs[i] = domain.ChatMessage{ID: t.ID, Role: t.Role, Text: t.Text,
				Tokens: t.Usage.TotalTokens, PromptTokens: t.Usage.PromptTokens, CompletionTokens: t.Usage.CompletionTokens,
				CreatedAt: time.Now(), Sources: t.Sources, Feedback: string(t.Feedback)}
		}
		// Keep an AI-generated name once set, rather than re-deriving from the first line.
		title, named := conversationTitle(ts), false
		for _, c := range app.Conversations() {
			if c.ID == cid && c.Named {
				title, named = c.Title, true
				break
			}
		}
		_ = app.PutConversation(domain.Conversation{
			ID: cid, Title: title, Named: named, Messages: msgs,
			CreatedAt: created, UpdatedAt: time.Now(),
			// C390: the conversation's own model and cap ride with it, so they
			// survive the reload — the moment they matter is coming back to a long
			// thread tomorrow, which is exactly when a session-only value is gone.
			Model:       uistate.AgentModel(cid),
			TokenBudget: uistate.AgentBudget(cid),
		})
		bump()
	}

	// rateTurn records or clears a verdict on one answer, then persists it with the
	// conversation so the mark survives a reload — a rating that vanishes on
	// refresh teaches people not to bother giving one.
	rateTurn := func(tid string, v chatpolish.Feedback) {
		cur := turns.Get()
		next := make([]chatTurn, len(cur))
		copy(next, cur)
		for i := range next {
			if next[i].ID == tid {
				next[i].Feedback = v
				break
			}
		}
		turns.Set(next)
		persist(next)
	}

	// switchTo loads a saved conversation into the live thread.
	switchTo := func(cid string) {
		for _, c := range app.Conversations() {
			if c.ID != cid {
				continue
			}
			ts := make([]chatTurn, len(c.Messages))
			for i, m := range c.Messages {
				ts[i] = chatTurn{ID: m.ID, Role: m.Role, Text: m.Text,
					Usage:    ai.Usage{PromptTokens: m.PromptTokens, CompletionTokens: m.CompletionTokens, TotalTokens: m.Tokens},
					Sources:  m.Sources,
					Feedback: chatpolish.Feedback(m.Feedback)}
			}
			turns.Set(ts)
			convID.Set(cid)
			// C390: resume this conversation on the model it was using, when it
			// chose one. A thread reopened on a different model gives different
			// answers to the same question, which reads as the assistant changing
			// its mind rather than as the setting changing underneath it.
			// The saved values are restored into session state first, so the
			// pickers and the send-time budget check read the same thing whether
			// the conversation was opened this session or last week.
			if c.Model != "" {
				uistate.SetAgentModel(cid, c.Model)
			}
			if c.TokenBudget > 0 {
				uistate.SetAgentBudget(cid, c.TokenBudget)
			}
			if m := uistate.AgentModel(cid); m != "" {
				modelSel.Set(m)
			}
			convCreated.Set(c.CreatedAt)
			setComposer("")
			errMsg.Set("")
			return
		}
	}

	// newChat clears the thread for a fresh (unsaved) conversation.
	newChat := func() {
		turns.Set(nil)
		convID.Set("")
		convCreated.Set(time.Time{})
		setComposer("")
		errMsg.Set("")
	}

	// renameConv gives a chat a name of the user's choosing. An empty name is not
	// rejected — it clears the custom name and lets the title be derived from the
	// first message again, which is a legitimate thing to want and must not leave a
	// chat called nothing.
	renameConv := func(cid, title string) {
		for _, c := range app.Conversations() {
			if c.ID != cid {
				continue
			}
			clean, ok := chatpolish.CleanTitle(title)
			if !ok {
				c.Named = false
				c.Title = conversationTitle(turnsOf(c))
			} else {
				c.Title, c.Named = clean, true
			}
			c.UpdatedAt = time.Now()
			_ = app.PutConversation(c)
			bump()
			return
		}
	}

	// exportConv downloads a chat as Markdown — for pasting into a note, sending to
	// an accountant, or keeping a record of what the assistant said before acting on
	// it. Markdown because the export is for a person, and because the answers are
	// already Markdown so they travel as themselves.
	exportConv := func(cid string) {
		for _, c := range app.Conversations() {
			if c.ID != cid {
				continue
			}
			downloadBytes(chatpolish.ExportFilename(c, time.Now()), "text/markdown;charset=utf-8",
				[]byte(chatpolish.ExportMarkdown(c)))
			return
		}
	}

	// deleteConv removes a saved conversation; if it's the open one, start fresh.
	deleteConv := func(cid string) {
		_ = app.DeleteConversation(cid)
		if convID.Get() == cid {
			newChat()
		}
		bump()
	}

	// Persist whenever the thread's shape changes (message added/removed/redone).
	cur := turns.Get()
	persistSig := convID.Get() + "|" + strconv.Itoa(len(cur))
	if n := len(cur); n > 0 {
		persistSig += "|" + cur[n-1].ID
	}
	ui.UseEffect(func() func() {
		if len(turns.Get()) > 0 || convID.Get() != "" {
			persist(turns.Get())
		}
		return nil
	}, persistSig)

	// On first mount, resume the most recently updated conversation (if any).
	ui.UseEffect(func() func() {
		if inited.Get() {
			return nil
		}
		inited.Set(true)
		cs := app.Conversations()
		newest := ""
		var newestAt time.Time
		for _, c := range cs {
			if newest == "" || c.UpdatedAt.After(newestAt) {
				newest, newestAt = c.ID, c.UpdatedAt
			}
		}
		if newest != "" {
			switchTo(newest)
		}
		// AG7: an "Explain" chip elsewhere in the app seeds a grounded question and
		// navigates here; consume it once and prefill the composer so the user lands
		// with the derivation ready to send.
		if seed, ok := uistate.ConsumeExplainSeed(); ok {
			setComposer(seed)
		}
		return nil
	}, "cf-insights-init")

	// Auto-scroll the canvas to the bottom whenever a message is added or the
	// "thinking" indicator toggles, so a freshly spawned bubble stays in view.
	// On an EMPTY thread we must NOT scroll to the end — the empty state leads
	// with the greeting hero (and, keyless, a demo transcript beneath it), so
	// scrolling to the bottom would land the user on the demo tail as if it were
	// a real conversation. Leave the canvas at the top so the greeting shows first.
	scrollSig := strconv.Itoa(len(turns.Get()))
	if loading.Get() {
		scrollSig += "|L"
	}
	ui.UseEffect(func() func() {
		if len(turns.Get()) > 0 {
			scrollChatToEnd()
		}
		return nil
	}, scrollSig)

	// Composer keyboard: Enter sends (Shift+Enter ignored), Up/Down cycle prior messages
	// (shell-style). A raw document keydown listener (so it gets NATIVE events — the
	// framework's OnKeyDown dispatched a synthetic keydown that crashed the app's global
	// shortcut listener). To avoid the vdom desync that broke later clicks, when it sets
	// the input it ALSO dispatches a native 'input' event so the framework's OnInput
	// syncs the bound state, keeping the DOM and vdom in agreement.
	doc := js.Global().Get("document")
	ui.UseEffect(func() func() {
		setVal := func(target js.Value, v string) {
			target.Set("value", v)
			ev := js.Global().Get("Event").New("input", map[string]any{"bubbles": true})
			target.Call("dispatchEvent", ev)
		}
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			ev := args[0]
			target := ev.Get("target")
			if !target.Truthy() || target.Get("id").String() != "cf-chat-input" {
				return nil
			}
			k := ev.Get("key").String()
			if k == "Enter" && !ev.Get("shiftKey").Bool() {
				ev.Call("preventDefault")
				submitChat()
				return nil
			}
			if k != "ArrowUp" && k != "ArrowDown" {
				// Editing leaves history mode — but only write the state when it is
				// actually changing. Setting it unconditionally re-rendered the whole
				// screen on every printable character, which was the second half of
				// what made typing here expensive.
				if (len(k) == 1 || k == "Backspace" || k == "Delete") && histIdx.Get() != -1 {
					histIdx.Set(-1)
				}
				return nil
			}
			msgs := make([]string, 0)
			for _, t := range turns.Get() {
				if t.Role == "user" {
					msgs = append(msgs, t.Text)
				}
			}
			if len(msgs) == 0 {
				return nil
			}
			ev.Call("preventDefault")
			idx := histIdx.Get()
			if k == "ArrowUp" {
				if idx == -1 {
					histDraft.Set(composerText())
					idx = len(msgs) - 1
				} else if idx > 0 {
					idx--
				}
				histIdx.Set(idx)
				setVal(target, msgs[idx])
			} else { // ArrowDown
				if idx == -1 {
					return nil
				}
				idx++
				if idx >= len(msgs) {
					histIdx.Set(-1)
					setVal(target, histDraft.Get())
				} else {
					histIdx.Set(idx)
					setVal(target, msgs[idx])
				}
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
		}
	}, "cf-chat-history")

	// Internal links inside an answer (e.g. "[Open it](/todo#id)") navigate in-app via
	// the router and scroll to the linked item, instead of doing a full page load. The
	// model may phrase the link as a relative ("/todo#id") OR an absolute same-origin URL
	// ("http://host/todo#id"), so we read the anchor's parsed origin/pathname/hash rather
	// than string-matching the raw href. Modifier- and middle-clicks keep their default
	// (open-in-new-tab) behavior. Registered in the capture phase so it wins over the
	// browser's default navigation regardless of any other listeners.
	ui.UseEffect(func() func() {
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			ev := args[0]
			if evTruthy(ev, "defaultPrevented") || evTruthy(ev, "metaKey") || evTruthy(ev, "ctrlKey") ||
				evTruthy(ev, "shiftKey") || evTruthy(ev, "altKey") {
				return nil
			}
			if b := ev.Get("button"); b.Type() == js.TypeNumber && b.Int() != 0 {
				return nil // not a left-click
			}
			a := ev.Get("target")
			for a.Truthy() && a.Get("tagName").String() != "A" {
				a = a.Get("parentElement")
			}
			if !a.Truthy() || !a.Call("closest", ".insights-answer").Truthy() {
				return nil
			}
			// Route in-app when the link is same-origin OR points at a known app route
			// (a deep link to one of our screens is meant for us even if the model
			// phrased it with a different host). Anything else keeps its default.
			loc := js.Global().Get("location")
			path := a.Get("pathname").String()
			if path == "" || !strings.HasPrefix(path, "/") {
				return nil
			}
			sameOrigin := a.Get("origin").String() == loc.Get("origin").String()
			if !sameOrigin && !isAppRoutePath(path) {
				return nil
			}
			ev.Call("preventDefault")
			frag := strings.TrimPrefix(a.Get("hash").String(), "#")
			router.Navigate(path)
			if frag != "" {
				scrollToID(frag)
			}
			return nil
		})
		doc.Call("addEventListener", "click", cb, true)
		return func() {
			doc.Call("removeEventListener", "click", cb, true)
			cb.Release()
		}
	}, "cf-chat-links")

	// Once a chat has a few exchanges (>=4 messages), generate a short AI title for it
	// (once) and update the switcher tab. Skips conversations already AI-named.
	namingSig := convID.Get() + "|" + strconv.Itoa(len(turns.Get()))
	ui.UseEffect(func() func() {
		ts := turns.Get()
		cid := convID.Get()
		if cid == "" || len(ts) < 4 || (key == "" && !useBackendAI) || namingDone.Get() == cid {
			return nil
		}
		for _, c := range app.Conversations() {
			if c.ID == cid && c.Named {
				namingDone.Set(cid)
				return nil
			}
		}
		namingDone.Set(cid) // attempt only once per chat
		var b strings.Builder
		for _, t := range ts {
			b.WriteString(t.Role + ": " + t.Text + "\n")
		}
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "Give a very short, 2-4 word title for this personal-finance chat. Reply with ONLY the title — no quotes, no punctuation, no preamble."},
			{Role: ai.RoleUser, Content: b.String()},
		}
		onName := func(content string, _ ai.Usage) {
			name := cleanChatTitle(content)
			if name == "" {
				return
			}
			for _, c := range app.Conversations() {
				if c.ID == cid {
					c.Title, c.Named = name, true
					_ = app.PutConversation(c)
					bump()
					return
				}
			}
		}
		noErr := func(string) {}
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, uistate.EffectiveServerToken(pr.ServerToken), model, messages, 0, onName, noErr)
		} else {
			ai.SendChat(key, aiBaseURL, model, messages, 0, onName, noErr)
		}
		return nil
	}, namingSig)

	onSubmit := ui.UseEvent(Prevent(func() { submitChat() }))
	newChatEvt := ui.UseEvent(Prevent(func() { newChat() }))
	// System-prompt editor handlers.
	onPromptInput := ui.UseEvent(func(v string) { promptDraft.Set(v) })
	openPrompt := ui.UseEvent(Prevent(func() {
		cur := strings.TrimSpace(uistate.LoadSystemPrompt())
		if cur == "" {
			cur = defaultChatSystemPrompt
		}
		promptDraft.Set(cur)
		promptOpen.Set(true)
	}))
	resetPrompt := ui.UseEvent(Prevent(func() { promptDraft.Set(defaultChatSystemPrompt) }))
	savePrompt := func() {
		d := strings.TrimSpace(promptDraft.Get())
		if d == "" || d == defaultChatSystemPrompt {
			uistate.PersistSystemPrompt("") // fall back to the default
		} else {
			uistate.PersistSystemPrompt(d)
		}
		promptOpen.Set(false)
	}
	closePrompt := func() { promptOpen.Set(false) }
	// Toggle the backend AI proxy on/off so the user can force the direct OpenAI
	// provider (or back to the proxy) without leaving the chat.
	prefsAtom := uistate.UsePrefs()
	toggleBackend := ui.UseEvent(Prevent(func() {
		p := prefsAtom.Get()
		p.BackendDisabled = !p.BackendDisabled
		prefsAtom.Set(p)
		uistate.PersistPrefs(p)
	}))

	// C228: wire the highlight-row drill-through using the same pattern as the
	// reports category drill (L58 FILTER_CARRY). UseTxFilter is called once at a
	// stable position; the callback is threaded down as a plain func.
	txFilterAtom := uistate.UseTxFilter()
	catsByName := categoryNameToIDMap(app.Categories())
	viewCategoryTransactions := func(catName string) {
		catID := catsByName[catName]
		f := uistate.TxFilter{Category: catID}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	// The agent-first surface keeps the CHAT as the page; highlights and anomaly
	// findings become the rail's "what I noticed" observations. (The merchants
	// table and trend chart live on the hub's Insights tab — they were duplicated
	// here and buried the conversation.)
	// Memoize the spend-anomaly detection (four monthly per-category series over every
	// transaction) on the data revision + scope + month, so the chat page doesn't re-run
	// it on each keystroke — only when the underlying data actually changes.
	spendingAnoms := ui.UseMemo(func() []insights.Anomaly {
		if !railReady.Get() {
			return nil // deferred to just after first paint (see railReady)
		}
		return detectSpendingAnomalies(scopedTxns, app.Categories(), rates)
	}, app.Rev(), fmt.Sprintf("%v", insightsSc), mStart.Unix(), railReady.Get())
	highlights := spendingHighlights(spendingAnoms, base, func(a insights.Anomaly) string {
		return anomalyAttributionText(a, catsByName[a.Category], scopedTxns, rates, base, time.Now())
	}, viewCategoryTransactions)

	// C252: bridge the four anomaly-type SMART detectors (duplicate, spike, missing
	// transaction, balance anomaly) into /insights unconditionally — no Smart gate.
	// pr is already declared above (UsePrefs hook at stable position).
	flagged := smartAnomalyHighlights(app, pr.WeekStartWeekday(), railReady.Get(),
		func(ins smart.Insight) {
			// Discuss ATTACHES the flag as a context bubble on the composer (not raw
			// text in the input). Dedupe by title so tapping twice doesn't stack it.
			detail := strings.TrimRight(strings.TrimSpace(ins.Detail), ".")
			cur := ctxAttach.Get()
			for _, c := range cur {
				if c.Title == ins.Title {
					focusByID("cf-chat-input")
					return
				}
			}
			// AG8: the app looks BEFORE the agent speaks. Attaching the headline
			// alone bought either a first reply asking what the user already knew,
			// or a confident answer built from the headline — both wasting the one
			// exchange they were willing to have. The bubble now carries the rows
			// behind the flag, what that merchant normally costs, and any recurring
			// schedule that explains it, so the agent's job is the verdict.
			ctxAttach.Set(append(append([]flagContext{}, cur...),
				flagContext{ID: id.New(), Title: ins.Title, Detail: detail, Kind: ins.Feature,
					Brief: investigateFlag(app, ins, base)}))
			focusByID("cf-chat-input")
		})

	// Pinned insights, newest first. The rail shows a SCANNABLE PREVIEW — the three
	// most recent, each clamped to a couple of lines — and cross-links to the
	// Insights tab where the full list lives beside the briefing (hub-review P2:
	// the rail was a wall of full-length AI paragraphs; the whole set belongs on
	// the roomier Insights tab, not stacked in a sidebar column).
	pins := app.SavedInsights()
	sort.Slice(pins, func(i, j int) bool { return pins[i].CreatedAt.After(pins[j].CreatedAt) })
	railPins := pins
	if len(railPins) > 3 {
		railPins = railPins[:3]
	}
	hubTab := uistate.UseAssistantTab()
	openInsightsTab := ui.UseEvent(Prevent(func() { hubTab.Set(uistate.AssistantTabInsights) }))
	// Bespoke aside group (NOT a card): a small serif label with an accent tick, a
	// "see all" link, and the clamped pin previews — margin notes, not tiles.
	pinnedCard := Fragment()
	if len(pins) > 0 {
		pinnedCard = collapsibleNote(collapsibleNoteProps{
			Label:  uistate.T("insights.pinnedTitle"),
			TestID: "assistant-note-pins",
			Count:  len(railPins),
			Link: Button(css.Class("ask-note-link"), Type("button"),
				Attr("data-testid", "assistant-see-insights"),
				OnClick(openInsightsTab), uistate.T("assistant.seeAllInsights")),
			Body: Fragment(MapKeyed(railPins,
				func(p domain.SavedInsight) any { return p.ID },
				func(p domain.SavedInsight) ui.Node {
					return ui.CreateElement(PinnedInsightRow, pinnedInsightRowProps{Insight: p, OnDelete: deletePinned})
				},
			)),
		})
	}

	convo := turns.Get()
	empty := len(convo) == 0

	// Retry is offered on the latest message (user or assistant) when idle, so a
	// failed turn with no reply can still be re-sent. resendLast re-answers the last
	// user prompt either way.
	lastID := ""
	if n := len(convo); n > 0 {
		lastID = convo[n-1].ID
	}
	retryFor := func(tid string) func() {
		if tid == lastID && !loading.Get() {
			return resendLast
		}
		return nil
	}

	// The conversation is a plain flex column; the SINGLE scroller is the canvas
	// (.chat-scroll) that wraps it, so the composer below stays put while history
	// scrolls. Auto-scroll keeps the newest message in view.
	thread := Div(Attr("id", "cf-chat-thread"), css.Class("chat-thread", tw.Flex, tw.FlexCol, tw.Gap4),
		MapKeyed(convo,
			func(t chatTurn) any { return t.ID },
			func(t chatTurn) ui.Node {
				if t.Role == "user" {
					return ui.CreateElement(UserBubble, userBubbleProps{ID: t.ID, Text: t.Text, OnDelete: deleteTurn, OnRetry: retryFor(t.ID), OnEdit: editAndResend})
				}
				if t.Role == "afford" {
					return ui.CreateElement(AffordResultBubble, affordResultBubbleProps{ID: t.ID, HTML: t.Text, OnDelete: deleteTurn})
				}
				return ui.CreateElement(AssistantBubble, asstBubbleProps{ID: t.ID, Text: t.Text, Usage: t.Usage, Model: model,
					Sources: t.Sources, Feedback: t.Feedback, OnPin: pinText, OnDelete: deleteTurn, OnRetry: retryFor(t.ID), OnFeedback: rateTurn})
			},
		),
		// While a turn is in flight the thread shows either the answer being
		// written or, before the first fragment lands, the thinking line. They are
		// mutually exclusive: showing both would put a spinner above text that has
		// visibly started arriving.
		If(loading.Get() && strings.TrimSpace(streaming.Get()) == "", Div(css.Class(tw.Flex, tw.JustifyStart),
			Div(css.Class("chat-row-agent"),
				Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
				Div(css.Class("insights-thinking chat-thinking", tw.Text13, tw.TextFaint), uistate.T("insights.thinking")),
			),
		)),
		If(loading.Get() && strings.TrimSpace(streaming.Get()) != "",
			ui.CreateElement(StreamingBubble, streamingBubbleProps{Text: streaming.Get()})),
	)

	// Composer: always show the Ask input (so the starter chips have a visible box to
	// fill and a new user sees what they'd ask, L8). With AI configured it pairs with
	// Send/Cancel; without a key it pairs with the add-your-key call-to-action so the
	// user is guided to set one up before sending. A plain Div (not a Form) so there's
	// no native submit that could reload the page; Enter is handled by the keydown listener.
	noAI := key == "" && !useBackendAI
	var trailing ui.Node
	switch {
	case noAI:
		// C246: show a Send button on the no-key path so mouse/touch users can
		// submit via click rather than only via Enter. Same aria-label as the
		// keyed Send button (insights.send) for consistent screen-reader semantics.
		trailing = Button(css.Class("chat-send"), Type("button"), Attr("data-testid", "assistant-send"), Attr("aria-label", uistate.T("insights.send")), Title(uistate.T("insights.send")), OnClick(onSubmit), uiw.Icon(icon.ArrowUp, css.Class(tw.W4, tw.H4)))
	case loading.Get():
		trailing = Button(css.Class("btn"), Type("button"), OnClick(cancelAI), uistate.T("insights.cancel"))
	default:
		// C249: give the send button an explicit accessible name and mark the leading
		// icon decorative so screen readers announce just "Send".
		trailing = Button(css.Class("chat-send"), Type("button"), Attr("data-testid", "assistant-send"), Attr("aria-label", uistate.T("insights.send")), Title(uistate.T("insights.send")), OnClick(onSubmit), uiw.Icon(icon.ArrowUp, css.Class(tw.W4, tw.H4)))
	}
	inputRow := Div(css.Class("asst-composer", tw.Mt1, tw.Flex, tw.Gap2, tw.ItemsCenter),
		// The placeholder tells the truth about the current mode (review: "tell me
		// what to do" overpromised agentic action a keyless session can't deliver).
		Input(Attr("id", "cf-chat-input"), css.Class("field field-wide"), Type("text"), Attr("aria-label", uistate.T("insights.askPlaceholder")),
			Placeholder(func() string {
				if noAI {
					return uistate.T("insights.askPlaceholderKeyless")
				}
				return uistate.T("insights.askPlaceholder")
			}()), OnInput(onInput), uiw.FieldValue(composerSeed.Get())),
		// Voice input: dictate a question via the browser's built-in speech engine
		// (no service, no key). Renders nothing where the API is unavailable. The
		// transcript arrives on a raw speech callback (outside the framework's event
		// loop), so — like the Enter/arrow composer keys above — it writes the DOM
		// input value and dispatches a native 'input' event so OnInput syncs the bound
		// state and the vdom stays in agreement (a plain state.Set here wouldn't re-render).
		ui.CreateElement(asstVoiceButton, asstVoiceButtonProps{OnResult: func(t string) {
			el := js.Global().Get("document").Call("getElementById", "cf-chat-input")
			if !el.Truthy() {
				return
			}
			el.Set("value", strings.TrimSpace(t))
			ev := js.Global().Get("Event").New("input", map[string]any{"bubbles": true})
			el.Call("dispatchEvent", ev)
			el.Call("focus")
		}}),
		trailing,
	)
	// C390: this conversation's cap and what it has spent against it. Reading the
	// tally revision keeps the readout live as turns land.
	_ = uistate.UseAgentActionsRevision().Get()
	_ = uistate.UseAgentTallyRevision().Get()
	budgetCap := uistate.AgentBudget(convID.Get())
	budgetUsed := uistate.AgentTokensUsed(convID.Get())
	budgetLeft, budgetCapped := uistate.AgentBudgetRemaining(convID.Get())

	// C250: which model is answering, and what this chat has cost so far, sit
	// directly under the box where the next question is typed. Both belong at the
	// point of asking rather than in a header the eye has already left: the model
	// changes the answer, and the running spend is the number that decides whether
	// to ask again.
	composerMeta := Fragment()
	if !noAI {
		sessionCost, sessionCostOK := uistate.AgentCostSoFar(convID.Get())
		composerMeta = ui.CreateElement(composerStatus, composerStatusProps{
			Model: model, Tokens: budgetUsed, CostUSD: sessionCost, HasCost: sessionCostOK,
			Capped: budgetCapped, Remaining: budgetLeft,
		})
	}
	composer := Fragment(inputRow, composerMeta)
	if noAI {
		// The full key explainer shows under the composer only mid-conversation;
		// on an empty thread the agent intro's callout is the single CTA.
		composer = Fragment(inputRow, If(len(turns.Get()) > 0, keyHintNode()))
	}

	// Attached flag-context bubbles ride ABOVE the composer: each shows it's context
	// (styled distinctly from an editable field) with a remove control, and folds into
	// the next send. Wrapped per-row in ctxBubble so the remove hook stays stable (L-gotcha).
	atts := ctxAttach.Get()
	ctxBubbles := Fragment()
	if len(atts) > 0 {
		ctxBubbles = Div(css.Class("asst-ctx-row"), Attr("data-testid", "assistant-ctx-row"),
			Span(css.Class("asst-ctx-lead", tw.TextFaint), uistate.T("assistant.contextLabel")),
			MapKeyed(atts,
				func(c flagContext) any { return c.ID },
				func(c flagContext) ui.Node {
					return ui.CreateElement(ctxBubble, ctxBubbleProps{ID: c.ID, Title: c.Title, Detail: c.Detail, OnRemove: removeCtx})
				},
			),
		)
	}
	// Remediation action chips for the most-recently-attached flag: one-click ways to
	// kick off a fix. Clicking sends the remediation (with the flag as context) so the
	// agent proposes the concrete change to approve — the chip starts it, doesn't do it.
	remedyChips := Fragment()
	if len(atts) > 0 {
		if rs := remediationsFor(atts[len(atts)-1].Kind); len(rs) > 0 {
			remedyChips = Div(css.Class("asst-remedy-row"), Attr("data-testid", "assistant-remedy-row"),
				MapKeyed(rs,
					func(r remediation) any { return r.Label },
					func(r remediation) ui.Node {
						instr := r.Instruction
						return remedyChip(remedyChipProps{Label: r.Label, OnPick: func() { sendRemediation(instr) }})
					},
				),
			)
		}
	}

	// Starter chips (L8, C231): shown on an EMPTY thread only (with an empty Ask
	// box). Replaying the same fixed chips after real exchanges read as a bot
	// ignoring the conversation — an agent's follow-ups should come from the
	// thread itself, and until they can, showing none is more honest.
	// Tapping a chip SENDS the question (QA CF-26).
	// QA CF-18: keyless, offer the FIXED questions the on-device answerer
	// (localqa) actually handles — the copy promises "a fixed set of questions
	// straight from your data", so every offered chip must work without a key
	// instead of alerting "Add your OpenAI key first". The localqa.Match filter
	// stays as a safety net so a rephrased chip can never regress into the alert.
	shownStarters := starters
	if noAI {
		fixed := []string{
			uistate.T("insights.keylessQ1"),
			uistate.T("insights.keylessQ2"),
			uistate.T("insights.keylessQ3"),
			uistate.T("insights.keylessQ4"),
		}
		shownStarters = make([]string, 0, len(fixed))
		for _, q := range fixed {
			if _, ok := localqa.Match(q); ok {
				shownStarters = append(shownStarters, q)
			}
		}
	}
	chips := Fragment()
	if len(shownStarters) > 0 && !composerFilled.Get() && empty {
		chips = Div(css.Class(tw.Mb2),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
				MapKeyed(shownStarters,
					func(q string) any { return q },
					func(q string) ui.Node {
						// QA CF-26: a tapped suggestion SENDS (the commercial-chat
						// convention) — filling the box and demanding a second Send read
						// as a broken chip. fillAsk stays for the Discuss flow, where
						// reviewing the context before sending is the point.
						return ui.CreateElement(suggestChip, suggestChipProps{Q: q, OnPick: func(q string) {
							if loading.Get() {
								return
							}
							sendText(q)
						}})
					},
				),
			),
		)
	}

	// Chat header controls (New chat, the Advanced expander with Edit prompt) —
	// the saved-conversation pills moved to the rail so the thread stays the page.
	convs := app.Conversations()
	sort.Slice(convs, func(i, j int) bool { return convs[i].UpdatedAt.After(convs[j].UpdatedAt) })
	// Standard header actions: New chat + Edit prompt as labeled .btn-tool buttons (the
	// app-wide toolbar-button standard). The old "Advanced" expander that only revealed
	// Edit prompt is gone — it was a click to hide a single option.
	// UX-09: the conversation leads the page. The model/thinking/privacy/share
	// controls collapse behind ONE "Chat settings" header button (drawer below the
	// title, default closed), New chat moves into the header, the privacy state
	// stays visible as a compact badge beside the composer, and on narrow widths
	// the aside becomes a slide-in drawer — so the thread + composer own the first
	// screen on every viewport.
	ctrlOpen := ui.UseState(false)
	toggleCtrls := ui.UseEvent(Prevent(func() { ctrlOpen.Set(!ctrlOpen.Get()) }))
	asideOpen := ui.UseState(false)
	toggleAside := ui.UseEvent(Prevent(func() { asideOpen.Set(!asideOpen.Get()) }))
	closeAside := ui.UseEvent(Prevent(func() { asideOpen.Set(false) }))
	togglePrivacy := func() {
		next := aicontext.TierAggregatesOnly
		if tier == aicontext.TierAggregatesOnly {
			next = aicontext.TierFull
		}
		privacyTier.Set(next)
		uistate.PersistDefaultPrivacyTier(next) // remember as the default for new chats (AG17)
	}
	togglePrivacyEvt := ui.UseEvent(Prevent(func() { togglePrivacy() }))
	tierLabel := uistate.T("insights.privacyFull")
	if tier == aicontext.TierAggregatesOnly {
		tierLabel = uistate.T("insights.privacyAggregates")
	}
	// Pre-send share estimate, hoisted so the drawer's full preview and the dock's
	// one-line scope note read from the same numbers (mirrors buildMessages).
	sharePersona := strings.TrimSpace(uistate.LoadSystemPrompt())
	if sharePersona == "" {
		sharePersona = defaultChatSystemPrompt
	}
	shareMem := uistate.LoadAgentMemory().Prompt()
	shareChars := len(sharePersona) + len(aiCtx.Line()) + len(categoryNames(app.Categories())) + len(customFieldsSummary(app.CustomFieldDefs())) + len(shareMem)
	for _, t := range turns.Get() {
		shareChars += len(t.Text)
	}
	shareEstTokens := shareChars / 4
	chatControls := Div(css.Class("ask-controls"), Attr("data-testid", "assistant-controls"),
		Attr("role", "group"), Attr("aria-label", uistate.T("assistant.controlsLabel")),
		modelPicker(modelPickerProps{Models: modelList.Get(), Current: model, OnPick: pickModel}),
		If(thinkingApplies, thinkPicker(thinkPickerProps{Effort: effortSel.Get(), OnPick: pickEffort})),
		budgetPicker(budgetPickerProps{
			Budget: budgetCap, Used: budgetUsed, Remaining: budgetLeft, Capped: budgetCapped, OnPick: pickBudget,
		}),
		privacyChip(privacyChipProps{Tier: tier, OnToggle: togglePrivacy}),
		// Pre-send data-sharing preview: what the NEXT message will carry, with a
		// rough token estimate, mirroring buildMessages' actual inputs.
		sharePreview(sharePreviewProps{
			Tier: tier, CtxLine: aiCtx.Line(),
			Categories: len(app.Categories()), CustomFields: len(app.CustomFieldDefs()),
			MemoryOn: shareMem != "", Turns: len(turns.Get()),
			EstTokens: shareEstTokens,
		}),
		Div(css.Class("ask-ctrl-actions"),
			// C251: editing the system prompt is meaningless without a model to send
			// it to. Showing the control to a keyless user offers a setting that
			// cannot have an effect, which reads as the feature being broken.
			If(!noAI, Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-edit-prompt"),
				Title(uistate.T("insights.editPrompt")), OnClick(openPrompt),
				uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("insights.editPrompt")))),
		),
	)
	// Bespoke aside group: the saved conversations as a quiet vertical index.
	// G2-C7: the rail searches titles AND message text together, because someone
	// hunting for "the chat about the car insurance" does not remember whether that
	// phrase was the name or something they typed inside it. Matches show the line
	// they matched on, so a result says why it is a result.
	shown := convs
	var matches []chatpolish.Match
	if q := strings.TrimSpace(convSearch.Get()); q != "" {
		matches = chatpolish.Search(convs, q)
		shown = make([]domain.Conversation, 0, len(matches))
		for _, m := range matches {
			shown = append(shown, m.Conversation)
		}
	}
	excerptFor := map[string]string{}
	for _, m := range matches {
		excerptFor[m.Conversation.ID] = m.Excerpt
	}

	railConvs := Fragment()
	if len(convs) > 0 {
		railConvs = collapsibleNote(collapsibleNoteProps{
			Label:  uistate.T("assistant.conversations"),
			TestID: "assistant-note-convs",
			Count:  len(convs),
			Body: Fragment(
				ui.CreateElement(convSearchBox, convSearchBoxProps{
					Query: convSearch.Get(), OnQuery: func(v string) { convSearch.Set(v) },
				}),
				Div(css.Class("asst-convs"), Attr("data-testid", "assistant-convs"),
					MapKeyed(shown,
						func(c domain.Conversation) any { return c.ID },
						func(c domain.Conversation) ui.Node {
							return ui.CreateElement(ConversationPill, convPillProps{
								C: c, Active: c.ID == convID.Get(), Excerpt: excerptFor[c.ID],
								OnPick: switchTo, OnDelete: deleteConv, OnRename: renameConv, OnExport: exportConv,
							})
						},
					),
				),
				// An empty result is stated rather than left as a blank space, which
				// is indistinguishable from a list that failed to load.
				If(len(shown) == 0, P(css.Class("ask-note-hint"), Attr("data-testid", "assistant-convs-empty"),
					uistate.T("insights.searchNone", strings.TrimSpace(convSearch.Get())))),
				If(len(shown) > 0, P(css.Class("ask-note-hint"), uistate.T("assistant.railHint"))),
			),
		})
	}

	// Backend/OpenAI mode toggle — only meaningful when a backend is configured;
	// otherwise the chat always uses the direct OpenAI provider.
	// The session's credential, not the static one: on a hosted instance
	// prefs.ServerToken is empty while a rotating session is live, and reading it
	// here hid the backend/OpenAI toggle from every hosted user.
	backendConfigured := strings.TrimSpace(pr.ServerURL) != "" && uistate.Session(pr.ServerToken).Present()
	backendToggle := Fragment()
	if backendConfigured {
		label := uistate.T("insights.usingOpenAI")
		action := uistate.T("insights.useBackend")
		if useBackendAI {
			label = uistate.T("insights.usingBackend")
			action = uistate.T("insights.useOpenAI")
		}
		backendToggle = Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb2, tw.Text12, tw.TextFaint),
			Span(label),
			Button(css.Class(tw.Underline, tw.HoverOpacity100), Type("button"), OnClick(toggleBackend), action),
		)
	}

	approvalPreview := ""
	approvalPerm := toolperm.Permission{}
	if pa := pendingApproval.Get(); pa != nil {
		approvalPreview = pa.preview
		approvalPerm = pa.perm
	}

	noData := len(accounts) == 0 && len(txns) == 0

	// Agent intro (empty thread): an agent-voiced welcome that leads with what it
	// can DO — read the real figures, make approval-gated changes, estimate with
	// math + web — so a first-time user meets an agent, not a search box.
	agentIntro := Div(css.Class("asst-intro"), Attr("data-testid", "assistant-intro"),
		Div(ClassStr("asst-intro-title "+tw.Fold(tw.FontDisplay)), uistate.T("assistant.introTitle")),
		P(css.Class("muted"), uistate.T("assistant.introBody")),
		Div(css.Class("asst-intro-cap"), Span(css.Class("rec-tag"), uistate.T("assistant.capAskTag")), Span(uistate.T("assistant.capAsk"))),
		Div(css.Class("asst-intro-cap"), Span(css.Class("rec-tag"), uistate.T("assistant.capDoTag")), Span(uistate.T("assistant.capDo"))),
		Div(css.Class("asst-intro-cap"), Span(css.Class("rec-tag"), uistate.T("assistant.capEstimateTag")), Span(uistate.T("assistant.capEstimate"))),
		// Keyless: the crucial fact (fixed question set now, full agent with a key)
		// lives HERE, where attention lands — not in footer microcopy.
		If(noAI, ui.CreateElement(KeyExplainer, keyExplainerProps{BaseURL: settings.OpenAIBaseURL})),
	)

	// MIA-extend (#445-9): when the user has an active scope show a compact
	// muted chip so they know the figures below are filtered. Because screens
	// cannot import app (import cycle), we build this inline using the already-
	// read insightsSc value and the existing nav hook. No extra On* hook needed
	// — OnClick closures over nav directly.
	scopeNotice := Fragment()
	if !insightsSc.IsAll() {
		scopeNotice = Div(css.Class("scope-notice", tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb2)),
			Span(css.Class("t-caption text-dim"), uistate.T("insights.scopeNotice")),
			Button(
				Type("button"),
				css.Class("btn-link t-caption text-dim"),
				Attr("data-testid", "insights-scope-change"),
				OnClick(func() { nav.Navigate(uistate.RoutePath("/reports")) }),
				uistate.T("insights.scopeChangeReports"),
			),
		)
	}

	// The AGENT CONSOLE — a canvas with real depth: a scrolling region whose
	// content is BOTTOM-ANCHORED (a short thread sits just above the composer, the
	// slack sits above it as natural scrollback — never a void between the last
	// reply and the input), a centered warm hero on an empty thread, and a docked
	// composer the content scrolls beneath. The rail keeps the agent's periphery.
	// All chat state/handlers are untouched.
	statusCls, statusKey := "chat-status-dot is-live", "assistant.statusLive"
	if noAI {
		statusCls, statusKey = "chat-status-dot is-local", "assistant.statusLocal"
	}
	// The empty-thread hero: greeting + capabilities + starter tiles (+ the keyless
	// demo transcript), grouped as one unit. The console is content-height, so a
	// short thread never strands a void — no bottom/top anchoring needed.
	// A compact recent-conversations list in the empty body so the sparse chat
	// space works and returning users can resume a chat without opening the side
	// rail (detail5). Top 3 from the already-sorted conversation state.
	recentBlock := Fragment()
	if len(convs) > 0 {
		recent := convs
		if len(recent) > 3 {
			recent = recent[:3]
		}
		recentBlock = Div(css.Class("asst-recent", tw.Mt1), Attr("data-testid", "assistant-recent-convos"),
			Span(css.Class("asst-recent-label", tw.Text12, tw.FontSemibold, tw.TextFaint), uistate.T("detail5.recentLabel")),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.Mt1),
				MapKeyed(recent,
					func(c domain.Conversation) any { return c.ID },
					func(c domain.Conversation) ui.Node {
						return ui.CreateElement(ConversationPill, convPillProps{C: c, Active: c.ID == convID.Get(), OnPick: switchTo, OnDelete: deleteConv})
					},
				),
			),
		)
	}
	heroBlock := Div(css.Class("asst-hero"),
		agentIntro,
		chips,
		recentBlock,
		// C248: static example Q→A pairs preview the assistant for keyless users.
		If(noAI, exampleConversationsNode()),
	)
	chatConsole := Div(css.Class("chat-console"), Attr("data-testid", "assistant-chat"),
		Div(css.Class("chat-scroll"), Attr("id", "cf-chat-scroll"),
			Div(css.Class("chat-measure"),
				backendToggle,
				If(empty, heroBlock),
				If(!empty, thread),
				// AG1: a multi-step plan the agent proposed renders here as a
				// reviewable changeset (per-item toggles + Apply all + undo-all).
				PendingChangesetHost(),
				// Approval card: a mutating tool is paused waiting for the user's yes/no.
				If(approvalPreview != "", ui.CreateElement(ApprovalCard, approvalCardProps{
					Preview:   approvalPreview,
					Perm:      approvalPerm,
					OnApprove: func() { respondApproval(pendingApproval.Get(), true) },
					OnDecline: func() { respondApproval(pendingApproval.Get(), false) },
				})),
				// AG20: the running per-conversation receipt (actions taken + spend).
				If(!empty, AgentSessionReceipt(convID.Get())),
				If(errMsg.Get() != "", P(css.Class("err"), Attr("role", "alert"), errMsg.Get())),
			),
		),
		Div(css.Class("chat-dock"),
			Div(css.Class("chat-measure"),
				ctxBubbles,
				remedyChips,
				composer,
				// UX-09: the privacy state stays visible as a compact badge beside
				// the composer (tap toggles the tier), and one short line states the
				// data scope the NEXT message will carry.
				Div(css.Class("chat-dock-meta"),
					Button(css.Class("chat-privacy-badge"), Type("button"), Attr("data-testid", "assistant-privacy-badge"),
						Attr("aria-label", uistate.T("insights.privacyAria", tierLabel)),
						Title(uistate.T("insights.privacyLabel")), OnClick(togglePrivacyEvt),
						uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W3, tw.H3)), Span(tierLabel)),
					Span(css.Class("chat-dock-scope"), Attr("data-testid", "assistant-scope-line"),
						uistate.T("insights.nextScopeContext", strings.ToLower(tierLabel), ai.FormatTokens(shareEstTokens))),
					P(css.Class("chat-dock-hint", tw.TextFaint), uistate.T("assistant.composerHint")),
				),
			),
		),
	)

	// The Ask surface — a BESPOKE deck built from scratch (no bento host, no Widget
	// tile, no card rail): a dominant conversation column with its own slim header
	// bar (live/on-device status + the serif agent name on the left, New chat /
	// Advanced as quiet ghost actions on the right) over the content-height canvas,
	// and a quiet "margin notes" aside — chrome-less typographic groups, not tiles —
	// for the agent's periphery and saved chats.
	askHead := Div(css.Class("ask-head"),
		Div(css.Class("ask-head-row"),
			Div(css.Class("ask-head-id"),
				Span(ClassStr(statusCls), Attr("aria-hidden", "true")),
				H2(css.Class("ask-title"), uistate.T("assistant.agentTitle")),
			),
			// UX-09 header actions: New chat is the everyday verb; Chat settings
			// opens the collapsed controls drawer; Notes & chats slides the aside
			// in on narrow widths (hidden on desktop, where the aside is visible).
			Div(css.Class("ask-head-actions"),
				Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-new-chat"), OnClick(newChatEvt),
					uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("insights.newChat"))),
				Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-settings-toggle"),
					Attr("aria-expanded", ariaBool(ctrlOpen.Get())), Attr("aria-controls", "ask-settings-drawer"),
					OnClick(toggleCtrls),
					uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("assistant.chatSettings"))),
				Button(css.Class("btn btn-tool ask-aside-toggle"), Type("button"), Attr("data-testid", "assistant-aside-toggle"),
					Attr("aria-expanded", ariaBool(asideOpen.Get())),
					OnClick(toggleAside),
					uiw.Icon(icon.MessageCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("assistant.notesDrawer"))),
			),
		),
		// The status/caption sits on its own subtitle line below the agent name, rather
		// than sharing the title's baseline at a clashing scale.
		Span(css.Class("ask-status"), uistate.T(statusKey)),
	)
	askMain := Div(css.Class("ask-main"),
		askHead,
		// The controls drawer: the full settings cell, revealed by the header's
		// Chat settings button (default closed — the conversation leads).
		If(ctrlOpen.Get(), Div(Attr("id", "ask-settings-drawer"), css.Class("ask-settings-drawer"), chatControls)),
		chatConsole,
	)

	return Div(
		// When there is no financial data yet, show a guided empty state so a first-time
		// user knows to add an account before asking questions. The chat section is still
		// rendered below it so all hooks stay stable.
		If(noData, uiw.Card(uiw.CardProps{
			Body: ui.CreateElement(EmptyStateCTA, emptyCTAProps{
				Message:   uistate.T("insights.noData"),
				CTALabel:  uistate.T("insights.addAccount"),
				AddTarget: "account",
				Icon:      icon.Insights,
			}),
		})),
		// MIA-extend (#445-9): compact scope notice — shown when a scope is active
		// so the user knows these figures are filtered. "Change scope in Reports →"
		// links directly to the ScopeSelector on /reports.
		scopeNotice,
		Div(css.Class("ask-deck"), Attr("data-testid", "assistant-layout"), Attr("id", "ask"),
			askMain,
			// The agent's periphery as quiet margin notes: anomaly findings,
			// spending highlights, pins, saved conversations. On narrow widths it
			// becomes a slide-in drawer opened from the header (UX-09).
			If(asideOpen.Get(), Div(css.Class("ask-aside-backdrop"), Attr("data-testid", "assistant-aside-backdrop"), OnClick(closeAside))),
			Div(ClassStr("ask-aside"+func() string {
				if asideOpen.Get() {
					return " is-open"
				}
				return ""
			}()), Attr("data-testid", "assistant-rail"),
				Button(css.Class("ask-aside-close"), Type("button"), Attr("data-testid", "assistant-aside-close"),
					Attr("aria-label", uistate.T("action.close")), OnClick(closeAside),
					uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
				flagged, highlights, pinnedCard, railConvs),
		),
		// The editable system-prompt overlay (persona only; live data + tools are always
		// injected automatically by buildMessages).
		If(promptOpen.Get(), uiw.FlipPanel(uiw.FlipPanelProps{
			Title:   uistate.T("insights.promptTitle"),
			Width:   uiw.FlipMediumW, // a prompt editor: a textarea + hint
			Height:  uiw.FlipMediumH,
			OnSave:  savePrompt,
			OnClose: closePrompt,
			Back: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
				P(css.Class("muted", tw.Text13), uistate.T("insights.promptHint")),
				Textarea(css.Class("field field-wide"), Attr("rows", "12"), Attr("aria-label", uistate.T("insights.promptTitle")), OnInput(onPromptInput), promptDraft.Get()),
				Button(css.Class("btn", tw.SelfStart), Type("button"), OnClick(resetPrompt), uistate.T("insights.promptReset")),
			),
		})),
	)
}

// customFieldsSummary builds a compact plain-English list of custom field definitions
// for use in the Insights context message, so the AI can answer questions that
// reference custom fields (e.g. "show spending by Property"). Each field is described
// as "<label> (<type>, on <entity>)"; multiple fields are comma-separated.
// Returns an empty string when no custom fields are defined.
func customFieldsSummary(defs []customfields.Def) string {
	if len(defs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(defs))
	for _, d := range defs {
		desc := d.Label + " (" + string(d.Type)
		if d.Type == customfields.TypeSelect && len(d.Options) > 0 {
			desc += ": " + strings.Join(d.Options, "/")
		}
		desc += ", on " + d.EntityType + ")"
		parts = append(parts, desc)
	}
	return strings.Join(parts, ", ")
}

// conversationTitle derives a chat's title from its first user message (truncated),
// falling back to a generic label for an empty thread.
func conversationTitle(ts []chatTurn) string {
	for _, t := range ts {
		if t.Role != "user" {
			continue
		}
		s := strings.TrimSpace(t.Text)
		if s == "" {
			continue
		}
		if r := []rune(s); len(r) > 40 {
			s = strings.TrimSpace(string(r[:40])) + "…"
		}
		return s
	}
	return "New chat"
}

// cleanChatTitle normalizes an AI-suggested chat title: first line, no surrounding
// quotes/punctuation, capped length.
func cleanChatTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "\"'`.*#"))
	if r := []rune(s); len(r) > 40 {
		s = strings.TrimSpace(string(r[:40]))
	}
	return s
}

type convPillProps struct {
	C        domain.Conversation
	Active   bool
	OnPick   func(string)
	OnDelete func(string)
	// OnRename gives the chat a name of the user's choosing; an empty name means
	// "go back to deriving it from the first message" (G2-C7). Nil hides the control.
	OnRename func(id, title string)
	// OnExport downloads the chat as Markdown. Nil hides the control.
	OnExport func(id string)
	// Excerpt is the line this chat matched a search on, shown under the title so
	// a result says WHY it is a result. Empty when not searching.
	Excerpt string
}

// turnsOf adapts a saved conversation's messages to the turn shape
// conversationTitle expects, so a cleared name can be re-derived from the first
// message exactly as it was derived originally.
func turnsOf(c domain.Conversation) []chatTurn {
	out := make([]chatTurn, len(c.Messages))
	for i, m := range c.Messages {
		out[i] = chatTurn{ID: m.ID, Role: m.Role, Text: m.Text}
	}
	return out
}

type convSearchBoxProps struct {
	Query   string
	OnQuery func(string)
}

// convSearchBox is the rail's search field. Its own component so the input hook
// sits at a stable position above the conversation list.
func convSearchBox(p convSearchBoxProps) ui.Node {
	onInput := ui.UseEvent(func(e ui.Event) { p.OnQuery(e.GetValue()) })
	clear := ui.UseEvent(Prevent(func() { p.OnQuery("") }))
	return Div(css.Class("conv-search"),
		Input(css.Class("field"), Type("search"), Attr("data-testid", "assistant-conv-search"),
			Attr("aria-label", uistate.T("insights.searchAria")),
			Placeholder(uistate.T("insights.searchPlaceholder")), OnInput(onInput), uiw.FieldValue(p.Query)),
		If(strings.TrimSpace(p.Query) != "", Button(css.Class("conv-search-clear"), Type("button"),
			Attr("data-testid", "assistant-conv-search-clear"),
			Attr("aria-label", uistate.T("insights.searchClear")), Title(uistate.T("insights.searchClear")),
			OnClick(clear), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3)))),
	)
}

// ConversationPill is one chat in the switcher: tap the title to open it, the pencil
// to name it, the arrow to export it, the × to delete it. Its own component so every
// hook stays at a stable position across the list.
func ConversationPill(p convPillProps) ui.Node {
	renaming := ui.UseState(false)
	draft := ui.UseState("")
	pick := ui.UseEvent(Prevent(func() { p.OnPick(p.C.ID) }))
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.C.ID) }))
	startRename := ui.UseEvent(Prevent(func() {
		draft.Set(p.C.Title)
		renaming.Set(true)
	}))
	onDraft := ui.UseEvent(func(e ui.Event) { draft.Set(e.GetValue()) })
	commitRename := ui.UseEvent(Prevent(func() {
		renaming.Set(false)
		if p.OnRename != nil {
			p.OnRename(p.C.ID, draft.Get())
		}
	}))
	// Enter commits and Escape backs out: a rename box you can only leave with the
	// mouse is a rename box people abandon half-typed.
	onKey := ui.UseEvent(func(e ui.Event) {
		switch e.GetKey() {
		case "Enter":
			renaming.Set(false)
			if p.OnRename != nil {
				p.OnRename(p.C.ID, draft.Get())
			}
		case "Escape":
			renaming.Set(false)
		}
	})
	exportEvt := ui.UseEvent(Prevent(func() {
		if p.OnExport != nil {
			p.OnExport(p.C.ID)
		}
	}))

	cls := "conv-pill " + tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tw.RoundedFull, tw.Px3, tw.Py1, tw.Text12, tw.Border) + " "
	if p.Active {
		cls += tw.Fold(tw.BgSky15, tw.BorderSky40)
	} else {
		cls += tw.Fold(tw.BorderBlack10, tw.HoverBgBlack03)
	}
	title := strings.TrimSpace(p.C.Title)
	if title == "" {
		title = "Untitled chat"
	}
	if renaming.Get() {
		return Div(css.Class("conv-rename"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "conv-rename-input"),
				Attr("aria-label", uistate.T("insights.renameAria")), OnInput(onDraft), OnKeyDown(onKey), OnBlur(commitRename), uiw.FieldValue(draft.Get())),
		)
	}
	pill := Div(ClassStr(cls),
		Button(css.Class(tw.MaxW160, tw.Truncate, tw.TextLeft), Type("button"), OnClick(pick), title),
		If(p.OnRename != nil, Button(css.Class(tw.TextFaint, tw.Opacity60, tw.HoverOpacity100), Type("button"),
			Attr("data-testid", "conv-rename"), Title(uistate.T("insights.renameChat")),
			Attr("aria-label", uistate.T("insights.renameChatFor", title)), OnClick(startRename),
			uiw.Icon(icon.Pencil, css.Class(tw.W3, tw.H3)))),
		If(p.OnExport != nil, Button(css.Class(tw.TextFaint, tw.Opacity60, tw.HoverOpacity100), Type("button"),
			Attr("data-testid", "conv-export"), Title(uistate.T("insights.exportChat")),
			Attr("aria-label", uistate.T("insights.exportChatFor", title)), OnClick(exportEvt),
			uiw.Icon(icon.Upload, css.Class(tw.W3, tw.H3)))),
		Button(css.Class(tw.TextFaint, tw.Opacity60, tw.HoverOpacity100), Type("button"), Title(uistate.T("insights.deleteChat")), Attr("aria-label", uistate.T("insights.deleteChat")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3))),
	)
	if strings.TrimSpace(p.Excerpt) == "" {
		return pill
	}
	return Div(css.Class("conv-hit"), pill,
		P(css.Class("conv-hit-excerpt"), Attr("data-testid", "conv-excerpt"), p.Excerpt))
}

// chatTurn is one message in the Insights conversation.
type chatTurn struct {
	ID    string
	Role  string // "user" | "assistant"
	Text  string
	Usage ai.Usage
	// Sources are the tool runs this answer was computed from (C387). Empty on a
	// user turn, on an answer the model gave without consulting anything, and on
	// the deterministic local answers, which cite themselves in their own text.
	Sources []domain.ChatSource
	// Feedback is the reader's verdict on this answer (G2-C7). It stays on the
	// device: it marks which answers were worth keeping, and goes nowhere.
	Feedback chatpolish.Feedback
}

// asstVoiceButtonProps carries the callback the mic fills the composer with.
type asstVoiceButtonProps struct {
	OnResult func(string)
}

// asstVoiceButton is the composer's dictation control: a mic that uses the browser's
// built-in Web Speech API (SpeechRecognition) to transcribe a spoken question into
// the input — entirely on the device's own speech engine, no CashFlux service and no
// API key. It renders NOTHING on browsers without the API (Firefox, some mobile), so
// it's a progressive enhancement that never shows a dead control. Its own component so
// the recognition hooks sit at stable positions.
func asstVoiceButton(props asstVoiceButtonProps) ui.Node {
	supported := ui.UseState(false)
	listening := ui.UseState(false)
	ui.UseEffect(func() func() {
		g := js.Global()
		if g.Get("SpeechRecognition").Truthy() || g.Get("webkitSpeechRecognition").Truthy() {
			supported.Set(true)
		}
		return nil
	}, "asst-voice-support")

	start := ui.UseEvent(Prevent(func() {
		if listening.Get() {
			return
		}
		// Some browsers throw from recognition.start() (permission denied, already
		// started, or an unimplemented stub) — a JS throw would otherwise panic the
		// wasm app. Recover so a mic hiccup is a no-op, never a crash.
		defer func() {
			if r := recover(); r != nil {
				listening.Set(false)
			}
		}()
		g := js.Global()
		ctor := g.Get("SpeechRecognition")
		if !ctor.Truthy() {
			ctor = g.Get("webkitSpeechRecognition")
		}
		if !ctor.Truthy() {
			return
		}
		rec := ctor.New()
		rec.Set("lang", "en-US")
		rec.Set("interimResults", false)
		rec.Set("maxAlternatives", 1)
		var onResult, onEnd, onErr js.Func
		release := func() { onResult.Release(); onEnd.Release(); onErr.Release() }
		onResult = js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 {
				res := args[0].Get("results")
				if res.Truthy() && res.Length() > 0 {
					first := res.Index(0)
					if first.Length() > 0 {
						if txt := first.Index(0).Get("transcript"); txt.Truthy() && props.OnResult != nil {
							props.OnResult(txt.String())
						}
					}
				}
			}
			return nil
		})
		onEnd = js.FuncOf(func(_ js.Value, _ []js.Value) any { listening.Set(false); release(); return nil })
		onErr = js.FuncOf(func(_ js.Value, _ []js.Value) any { listening.Set(false); release(); return nil })
		rec.Set("onresult", onResult)
		rec.Set("onend", onEnd)
		rec.Set("onerror", onErr)
		rec.Call("start")
		listening.Set(true)
	}))

	if !supported.Get() {
		return Fragment()
	}
	cls := "icon-btn asst-mic"
	label := uistate.T("assistant.voiceStart")
	if listening.Get() {
		cls += " is-listening"
		label = uistate.T("assistant.voiceListening")
	}
	return Button(ClassStr(cls), Type("button"), Attr("data-testid", "asst-voice-btn"),
		Attr("aria-label", label), Attr("aria-pressed", ariaBool(listening.Get())),
		Title(label), OnClick(start),
		uiw.Icon(icon.Mic, css.Class(tw.W5, tw.H5)))
}

// flagContext is a flagged-activity item attached to the composer as a context
// bubble (the "Discuss" action). It rides above the input and is folded into the
// prompt at send time — never typed into the input field itself. Kind is the SMART
// detector's feature code (e.g. "SMART-T2"), which drives the remediation chips.
type flagContext struct {
	ID     string
	Title  string
	Detail string
	Kind   string
	// Brief is the evidence the app gathered about this flag before anyone asked
	// (AG8): the rows behind it, what the merchant normally costs, any recurring
	// schedule that explains it. It rides with the message but is never shown in
	// the bubble — the bubble says WHICH flag is attached, and this says what is
	// known about it.
	Brief string
}

// investigateFlag gathers the evidence behind a flagged finding so the assistant
// can open with a verdict instead of a question (AG8).
//
// It returns "" when there is nothing to gather — a finding the probe cannot
// anchor to a merchant or an account. An empty brief is deliberate: filling one in
// by guessing at relevance would make the model's verdict worse than no brief at
// all.
func investigateFlag(app *appstate.App, ins smart.Insight, base string) string {
	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
	}
	txns := app.Transactions()
	payees := make([]string, 0, len(txns))
	seen := map[string]bool{}
	for _, t := range txns {
		name := strings.TrimSpace(t.Payee)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		payees = append(payees, name)
	}
	finding := anomalyprobe.Finding{
		Feature: ins.Feature,
		Title:   ins.Title,
		Detail:  ins.Detail,
		Payee:   anomalyprobe.PayeeIn(ins.Title+" "+ins.Detail, payees),
	}
	if ins.HasAmount {
		finding.AmountMinor = ins.Amount.Amount
	}
	ev := anomalyprobe.Gather(finding, txns, app.Recurring(), func(id string) string { return catName[id] }, time.Now())
	if len(ev.Related) == 0 && ev.MerchantHistory == nil && ev.RecurringMatch == "" {
		return ""
	}
	return ev.Brief(finding,
		func(minor int64) string { return insightsMoneyFmt(minor, base) },
		func(t time.Time) string { return uistate.LoadPrefs().FormatDate(t) })
}

// remediation is a one-click fix offered as an action chip for a flagged activity.
// Label is the chip text; Instruction is the message sent to the agent to start it.
type remediation struct {
	Label       string
	Instruction string
}

// remediationsFor returns the remediation action chips for a flagged-activity kind,
// keyed by the SMART detector's feature code. Mutating fixes are phrased to route
// through the agent's in-thread approval — the chip starts the fix, it never acts
// silently. Returns nil for kinds without a canned remediation set.
func remediationsFor(feature string) []remediation {
	switch feature {
	case "SMART-T2": // duplicate transaction
		return []remediation{
			{uistate.T("remedy.dupRemove"), uistate.T("remedy.dupRemoveMsg")},
			{uistate.T("remedy.dupMerge"), uistate.T("remedy.dupMergeMsg")},
			{uistate.T("remedy.dupKeep"), uistate.T("remedy.dupKeepMsg")},
			{uistate.T("remedy.dupReverse"), uistate.T("remedy.dupReverseMsg")},
		}
	case "SMART-T7": // missing / expected transaction
		return []remediation{
			{uistate.T("remedy.missAdd"), uistate.T("remedy.missAddMsg")},
			{uistate.T("remedy.missPaused"), uistate.T("remedy.missPausedMsg")},
			{uistate.T("remedy.missLater"), uistate.T("remedy.missLaterMsg")},
		}
	case "SMART-T6": // spending spike
		return []remediation{
			{uistate.T("remedy.spikeExplain"), uistate.T("remedy.spikeExplainMsg")},
			{uistate.T("remedy.spikeExpected"), uistate.T("remedy.spikeExpectedMsg")},
			{uistate.T("remedy.spikeGuard"), uistate.T("remedy.spikeGuardMsg")},
		}
	case "SMART-A1": // balance anomaly
		return []remediation{
			{uistate.T("remedy.balReconcile"), uistate.T("remedy.balReconcileMsg")},
			{uistate.T("remedy.balUpdate"), uistate.T("remedy.balUpdateMsg")},
			{uistate.T("remedy.balExplain"), uistate.T("remedy.balExplainMsg")},
		}
	}
	return nil
}

type remedyChipProps struct {
	Label  string
	OnPick func()
}

// remedyChip is one clickable remediation action. Own component so its click hook
// stays at a stable position across the (variable-length) chip list.
func remedyChip(p remedyChipProps) ui.Node { return ui.CreateElement(remedyChipComp, p) }

func remedyChipComp(p remedyChipProps) ui.Node {
	onClick := ui.UseEvent(func() {
		if p.OnPick != nil {
			p.OnPick()
		}
	})
	return Button(css.Class("asst-remedy"), Type("button"), Attr("data-testid", "assistant-remedy-chip"), OnClick(onClick),
		uiw.Icon(icon.Sparkles, css.Class("asst-remedy-icon", tw.ShrinkO, tw.W3, tw.H3)),
		Span(p.Label),
	)
}

type ctxBubbleProps struct {
	ID       string
	Title    string
	Detail   string
	OnRemove func(string)
}

// ctxBubble renders one attached flag-context as a removable chip above the
// composer. Its own component so the remove-click hook sits at a stable position
// across the (variable-length) attachment list (framework loop-hook gotcha).
func ctxBubble(p ctxBubbleProps) ui.Node {
	return ui.CreateElement(ctxBubbleComp, p)
}

func ctxBubbleComp(p ctxBubbleProps) ui.Node {
	onRemove := ui.UseEvent(func() {
		if p.OnRemove != nil {
			p.OnRemove(p.ID)
		}
	})
	tip := strings.TrimSpace(p.Title)
	if p.Detail != "" {
		tip += " — " + p.Detail
	}
	return Div(css.Class("asst-ctx"), Attr("data-testid", "assistant-ctx-bubble"), Title(tip),
		uiw.Icon(icon.Paperclip, css.Class("asst-ctx-icon", tw.ShrinkO, tw.W3, tw.H3)),
		Span(css.Class("asst-ctx-label"), p.Title),
		Button(css.Class("asst-ctx-x"), Type("button"), Attr("data-testid", "assistant-ctx-remove"),
			Attr("aria-label", uistate.T("assistant.ctxRemove")), Title(uistate.T("assistant.ctxRemove")), OnClick(onRemove),
			uiw.Icon(icon.Close, css.Class(tw.ShrinkO, tw.W3, tw.H3))),
	)
}

type userBubbleProps struct {
	ID       string
	Text     string
	OnDelete func(string)
	OnRetry  func() // non-nil only on the latest message
	// OnEdit resends this question with new wording, dropping everything that
	// came after it (G2-C7). Nil where editing does not apply.
	OnEdit func(id, text string)
}

// UserBubble renders one user message with its actions (Edit, Retry on the latest,
// Delete) in a row UNDER the bubble. Its own component so the action hooks stay
// stable across the list (no hooks in loops).
//
// Editing turns the bubble into a textarea in place. That is deliberate: the
// alternative — copying the old question into the composer — leaves the original
// sitting above as a question that was never really asked, and the thread stops
// being a record of the conversation that happened.
func UserBubble(p userBubbleProps) ui.Node {
	editing := ui.UseState(false)
	draft := ui.UseState("")
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.ID) }))
	retryEvt := ui.UseEvent(Prevent(func() {
		if p.OnRetry != nil {
			p.OnRetry()
		}
	}))
	startEdit := ui.UseEvent(Prevent(func() {
		draft.Set(p.Text)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	onDraft := ui.UseEvent(func(e ui.Event) { draft.Set(e.GetValue()) })
	// Escape backs out, because a textarea that can only be left by clicking a
	// button traps anyone working from the keyboard.
	onKey := ui.UseEvent(func(e ui.Event) {
		if e.GetKey() == "Escape" {
			editing.Set(false)
		}
	})
	submitEdit := ui.UseEvent(Prevent(func() {
		text := strings.TrimSpace(draft.Get())
		if text == "" || p.OnEdit == nil {
			return
		}
		editing.Set(false)
		p.OnEdit(p.ID, text)
	}))

	if editing.Get() {
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.ItemsEnd),
			Div(css.Class("asst-msg-edit"), Attr("data-testid", "assistant-edit-box"),
				Textarea(css.Class("field"), Attr("data-testid", "assistant-edit-input"),
					Attr("aria-label", uistate.T("insights.editAria")), Attr("rows", "3"), OnInput(onDraft), OnKeyDown(onKey), uiw.FieldValue(draft.Get())),
				Div(css.Class("asst-msg-edit-actions"),
					Button(css.Class("btn btn-primary btn-sm"), Type("button"),
						Attr("data-testid", "assistant-edit-send"), OnClick(submitEdit), uistate.T("insights.editSend")),
					Button(css.Class("btn btn-sm"), Type("button"),
						Attr("data-testid", "assistant-edit-cancel"), OnClick(cancelEdit), uistate.T("action.cancel")),
				),
				P(css.Class("asst-msg-edit-note"), uistate.T("insights.editNote")),
			),
		)
	}

	actBtn := tw.Fold(tw.TextFaint, tw.Opacity70, tw.HoverOpacity100, tw.InlineFlex, tw.ItemsCenter)
	return Div(css.Class("group", tw.Flex, tw.FlexCol, tw.ItemsEnd),
		Div(css.Class("asst-msg-user", tw.MaxW85, tw.Text14, tw.WhitespacePreWrap), p.Text),
		Div(css.Class(tw.Flex, tw.Gap3, tw.ItemsCenter, tw.Mt1, tw.Px1, tw.Opacity0, tw.GroupHoverOpacity100, tw.GroupFocusWithinOpacity100, tw.MotionSafeTransitionOpacity),
			If(p.OnEdit != nil, Button(ClassStr(actBtn), Type("button"), Attr("data-testid", "assistant-edit-msg"),
				Title(uistate.T("insights.editMsg")), Attr("aria-label", uistate.T("insights.editMsg")), OnClick(startEdit),
				uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4)))),
			If(p.OnRetry != nil, Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, css.Class(tw.W4, tw.H4)))),
			Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
}

type composerStatusProps struct {
	Model  string
	Tokens int
	// CostUSD is the conversation's accumulated cost from the shared tally, and
	// HasCost is false when the model's pricing is unknown. Passed in rather than
	// derived here so this line and the receipt can never disagree.
	CostUSD   float64
	HasCost   bool
	Capped    bool
	Remaining int
}

// composerStatus is the line under the composer naming the model that will answer
// and what this conversation has spent (C250).
//
// The cost is stated as an estimate in as many words, because it is one: it is
// derived from a pricing table that drifts upstream. A figure presented as exact
// when it is not teaches people to distrust the accurate parts of the app too.
func composerStatus(p composerStatusProps) ui.Node {
	if strings.TrimSpace(p.Model) == "" {
		return Fragment()
	}
	spend := Fragment()
	if p.Tokens > 0 {
		text := uistate.T("assistant.spentTokens", ai.FormatTokens(p.Tokens))
		// The cost comes from the same tally the receipt below uses, which is fed
		// each turn's real input/output split. Deriving it here from the token
		// TOTAL is what this line did first, and it priced every output token at
		// the input rate — several times cheap on a reasoning model, and a second
		// figure on screen disagreeing with the receipt's.
		if cost, ok := p.CostUSD, p.HasCost; ok && cost > 0 {
			text = uistate.T("assistant.spentTokensCost", ai.FormatTokens(p.Tokens), ai.FormatCostUSD(cost))
		}
		spend = Span(css.Class("asst-status-spend"), Attr("data-testid", "assistant-session-spend"), text)
	}
	return Div(css.Class("asst-status"), Attr("data-testid", "assistant-composer-status"),
		Span(css.Class("asst-status-model"), Attr("data-testid", "assistant-active-model"),
			Title(uistate.T("assistant.activeModelTitle")), p.Model),
		spend,
	)
}

type streamingBubbleProps struct{ Text string }

// StreamingBubble renders the answer as it is being written (G2-C7).
//
// It shows PLAIN TEXT, not rendered Markdown, and that is deliberate. A partial
// Markdown document is frequently invalid — a half-written table, an unclosed code
// fence, a heading whose line has not ended — and re-rendering it on every fragment
// makes the answer visibly thrash between layouts as it arrives. Plain text grows
// steadily and is replaced by the properly rendered answer the moment the turn
// completes. It is aria-live=polite so a screen reader hears the finished answer
// once, rather than being interrupted by every fragment.
func StreamingBubble(p streamingBubbleProps) ui.Node {
	ui.UseEffect(func() func() { scrollChatToEnd(); return nil }, p.Text)
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.ItemsStart),
		Div(css.Class("chat-row-agent"),
			Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
			Div(css.Class("insights-answer asst-streaming", tw.Text14),
				Attr("data-testid", "assistant-streaming"),
				Attr("aria-live", "polite"), Attr("aria-busy", "true"),
				p.Text,
				Span(css.Class("asst-caret"), Attr("aria-hidden", "true")),
			),
		),
	)
}

type asstBubbleProps struct {
	ID       string
	Text     string
	Usage    ai.Usage
	Model    string
	Sources  []domain.ChatSource // the tool runs this answer was computed from (C387)
	Feedback chatpolish.Feedback // the reader's verdict on this answer (G2-C7)
	OnPin    func(string) bool
	OnDelete func(string)
	OnRetry  func() // non-nil only on the latest assistant turn
	// OnFeedback records or clears a verdict on this answer. Nil where rating
	// does not apply.
	OnFeedback func(id string, v chatpolish.Feedback)
}

// asstActionRowClass returns the class list for an answer's action row. The row is
// normally revealed on hover — actions on every message would shout over the
// conversation — but once a verdict has been given the row stays visible, because
// a rating you cannot see is a rating you cannot check or change.
func asstActionRowClass(f chatpolish.Feedback) string {
	base := tw.Fold(tw.Flex, tw.FlexWrap, tw.Gap3, tw.ItemsCenter, tw.Mt1, tw.Px1)
	if f != chatpolish.FeedbackNone {
		return base
	}
	return base + " " + tw.Fold(tw.Opacity0, tw.GroupHoverOpacity100, tw.GroupFocusWithinOpacity100, tw.MotionSafeTransitionOpacity)
}

// feedbackBtnClass marks the chosen verdict so the state is visible without
// relying on colour alone (the aria-pressed attribute carries it for AT).
func feedbackBtnClass(base string, current, self chatpolish.Feedback) string {
	if current == self {
		return base + " asst-rated"
	}
	return base
}

// AssistantBubble renders one assistant message as Markdown (via the vendored
// marked + DOMPurify, set as sanitized innerHTML by the effect below) with Copy,
// Pin, Retry (latest only), and Delete actions plus a token/cost note. Its own
// component so the action + effect hooks stay stable across the list (no hooks in
// loops).
func AssistantBubble(p asstBubbleProps) ui.Node {
	pinned := ui.UseState(false)
	copied := ui.UseState(false)
	mdID := "cf-md-" + p.ID
	// Fill the answer body after EVERY render, not only when the text changes.
	//
	// The answer is Markdown written imperatively into a node the vdom owns, so the
	// vdom — which believes that node has no children — strips it on any re-render of
	// this bubble. The fill therefore has to be re-applied every time, and the old
	// signature (text, plus the pin/copy toggles) only covered the re-renders this
	// component causes ITSELF. Everything else left the bubble blank: a rating, a
	// citation arriving, the session tally ticking, the spend meter writing settings —
	// each re-rendered the bubble without changing the text, so the effect never
	// re-fired and the answer appeared for a moment and then vanished.
	//
	// No deps means "run on every render" (the runtime treats an empty dep list as
	// always-changed), and UseLayoutEffect runs synchronously after the DOM mutation
	// and before paint, so the refill lands in the same frame that cleared it and
	// there is no flicker. renderMarkdown skips the parse when the node already holds
	// this text, so the common case costs one attribute read.
	ui.UseLayoutEffect(func() func() { renderMarkdown(mdID, p.Text); return nil })
	pin := ui.UseEvent(Prevent(func() {
		if p.OnPin(p.Text) {
			pinned.Set(true)
		}
	}))
	copyEvt := ui.UseEvent(Prevent(func() {
		copyText(p.Text)
		copied.Set(true)
	}))
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.ID) }))
	retryEvt := ui.UseEvent(Prevent(func() {
		if p.OnRetry != nil {
			p.OnRetry()
		}
	}))
	rateUp := ui.UseEvent(Prevent(func() {
		if p.OnFeedback != nil {
			p.OnFeedback(p.ID, chatpolish.ToggleFeedback(p.Feedback, chatpolish.FeedbackUp))
		}
	}))
	rateDown := ui.UseEvent(Prevent(func() {
		if p.OnFeedback != nil {
			p.OnFeedback(p.ID, chatpolish.ToggleFeedback(p.Feedback, chatpolish.FeedbackDown))
		}
	}))
	var note ui.Node = Fragment()
	if p.Usage.TotalTokens > 0 {
		// QPASS-D honesty: the old line labelled the WHOLE request ("This reply:
		// 8,578 tokens") as the reply and formatted a $0.00 cost from a rehydrated
		// turn whose input/output split had been zeroed. Split the display — reply
		// (output) tokens as the headline, context (input) as secondary — and cost
		// the turn from the split, never from a zeroed one.
		var txt string
		if p.Usage.PromptTokens > 0 || p.Usage.CompletionTokens > 0 {
			// Full split available: cost is trustworthy (or "cost unavailable"
			// when the model has no known pricing) — never a bogus $0.00.
			costText := uistate.T("insights.costUnavailable")
			if cost, ok := ai.EstimateCostUSD(p.Model, p.Usage); ok {
				costText = ai.FormatCostUSD(cost)
			}
			txt = uistate.T("insights.replyUsageSplit",
				ai.FormatTokens(p.Usage.CompletionTokens),
				ai.FormatTokens(p.Usage.PromptTokens),
				costText)
		} else {
			// Legacy turn saved before the split was recorded: only the total
			// survived, so an accurate input/output cost can't be recomputed —
			// say the honest total rather than invent a $0.00.
			txt = uistate.T("insights.replyUsageTotalNA", ai.FormatTokens(p.Usage.TotalTokens))
		}
		note = P(css.Class(tw.TextFaint, tw.Text11, tw.Mt2), txt)
	}
	actBtn := tw.Fold(tw.TextFaint, tw.Opacity70, tw.HoverOpacity100, tw.InlineFlex, tw.ItemsCenter)
	return Div(css.Class("group", tw.Flex, tw.FlexCol, tw.ItemsStart),
		Div(css.Class("chat-row-agent"),
			Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
			// marked fills this element via the effect above.
			Div(Attr("id", mdID), css.Class("md insights-answer chat-agent-body", tw.Text14)),
		),
		// Actions sit UNDER the bubble, revealed when the bubble is hovered/focused.
		Div(ClassStr(asstActionRowClass(p.Feedback)),
			IfElse(copied.Get(),
				Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("insights.copied")),
				Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.copy")), Attr("aria-label", uistate.T("insights.copy")), OnClick(copyEvt), uiw.Icon(icon.Copy, css.Class(tw.W4, tw.H4))),
			),
			IfElse(pinned.Get(),
				Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("insights.pinnedConfirm")),
				Button(ClassStr(actBtn+" "+tw.Fold(tw.Gap1, tw.Text12)), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pin), uistate.T("insights.pin")),
			),
			If(p.OnRetry != nil, Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, css.Class(tw.W4, tw.H4)))),
			// The verdict buttons stay visible once one is given: a rating that
			// disappears when the pointer leaves cannot be seen, changed, or
			// trusted to have registered.
			If(p.OnFeedback != nil, Button(ClassStr(feedbackBtnClass(actBtn, p.Feedback, chatpolish.FeedbackUp)), Type("button"),
				Attr("data-testid", "assistant-rate-up"), Attr("aria-pressed", ariaBool(p.Feedback == chatpolish.FeedbackUp)),
				Title(uistate.T("insights.rateUp")), Attr("aria-label", uistate.T("insights.rateUp")), OnClick(rateUp),
				uiw.Icon(icon.ThumbsUp, css.Class(tw.W4, tw.H4)))),
			If(p.OnFeedback != nil, Button(ClassStr(feedbackBtnClass(actBtn, p.Feedback, chatpolish.FeedbackDown)), Type("button"),
				Attr("data-testid", "assistant-rate-down"), Attr("aria-pressed", ariaBool(p.Feedback == chatpolish.FeedbackDown)),
				Title(uistate.T("insights.rateDown")), Attr("aria-label", uistate.T("insights.rateDown")), OnClick(rateDown),
				uiw.Icon(icon.ThumbsDown, css.Class(tw.W4, tw.H4)))),
			Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		citationPanel(p.Text, p.Sources),
		note,
	)
}

// citationPanel renders "How I got this" under an answer: every tool the assistant
// ran, and the result each one handed back (C387).
//
// It appears only when the answer makes a numeric claim AND something was actually
// consulted. An answer with no figures has nothing to check, and an answer the model
// gave from context alone would produce an empty panel that implies sourcing it
// doesn't have. It is collapsed by default — the point is that the evidence is
// THERE, not that it is in the way — and it is plain <details>, so it works before
// any script runs and keyboard-opens for free.
func citationPanel(reply string, sources []domain.ChatSource) ui.Node {
	if len(sources) == 0 || !toolcite.Numeric(reply) {
		return Fragment()
	}
	rows := make([]any, 0, len(sources))
	for _, s := range sources {
		src := toolcite.Source{Tool: s.Tool, Label: s.Label, Scope: s.Scope}
		body := Fragment()
		if strings.TrimSpace(s.Evidence) != "" {
			body = Pre(css.Class("asst-cite-evidence"), strings.TrimSpace(s.Evidence))
		}
		rows = append(rows, Li(css.Class("asst-cite-item"),
			P(css.Class("asst-cite-title"), src.Title()),
			body,
		))
	}
	list := append([]any{css.Class("asst-cite-list")}, rows...)
	return Details(css.Class("asst-cite"), Attr("data-testid", "assistant-citations"),
		Summary(css.Class("asst-cite-summary"), uistate.T("insights.citeSummary", len(sources))),
		Ul(list...),
	)
}

// renderMarkdown sets the element's sanitized, Markdown-rendered HTML using the
// vendored marked + DOMPurify globals; falls back to the raw text when absent.
func renderMarkdown(elemID, mdText string) {
	renderMarkdownAttempt(elemID, mdText, 3)
}

// renderMarkdownAttempt is renderMarkdown with a bounded retry: the effect that
// calls this can fire before the chat list's DOM append lands (several
// AssistantBubble instances mounting together on first paint, e.g. a fresh
// /assistant load with existing history), so getElementById can miss on the
// first try. Without a retry the bubble stays permanently empty — the answer
// exists in the turn's Text, it just never reaches the DOM — because the
// effect's dependency (the text signature) never changes again to re-fire it.
// Retrying across a couple of animation frames costs nothing once the element
// is already there (first attempt always wins in the common case) and self-
// heals the rare late-mount race.
func renderMarkdownAttempt(elemID, mdText string, retriesLeft int) {
	doc := js.Global().Get("document")
	el := doc.Call("getElementById", elemID)
	if !el.Truthy() {
		if retriesLeft > 0 {
			var cb js.Func
			cb = js.FuncOf(func(this js.Value, args []js.Value) any {
				cb.Release()
				renderMarkdownAttempt(elemID, mdText, retriesLeft-1)
				return nil
			})
			js.Global().Call("requestAnimationFrame", cb)
		}
		return
	}
	// The caller re-fills on every render, so skip the Markdown parse and the
	// sanitize when this node already holds this exact text.
	//
	// Both halves of the check are load-bearing. The fingerprint alone would be
	// wrong because a re-render strips the node's CHILDREN while leaving its
	// attributes behind — the stamp would still claim the answer is on screen
	// after the answer had been wiped off it. The emptiness check alone would be
	// wrong because it would re-parse an unchanged answer on every render.
	stamp := markdownStamp(mdText)
	dataset := el.Get("dataset")
	if dataset.Truthy() && dataset.Get("cfMd").String() == stamp && el.Get("innerHTML").String() != "" {
		return
	}
	html := mdText
	if m := js.Global().Get("marked"); m.Truthy() {
		html = m.Call("parse", mdText).String()
	}
	if dp := js.Global().Get("DOMPurify"); dp.Truthy() {
		html = dp.Call("sanitize", html).String()
	}
	el.Set("innerHTML", html)
	if dataset.Truthy() {
		dataset.Set("cfMd", stamp)
	}
}

// markdownStamp fingerprints an answer for the re-fill guard. It is a hash rather
// than the text itself because the stamp lives in a DOM attribute, and answers run
// to kilobytes — storing one verbatim on every bubble would double the thread's
// weight in the DOM to save a string compare.
func markdownStamp(mdText string) string {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	hash := uint64(offset64)
	for i := 0; i < len(mdText); i++ {
		hash ^= uint64(mdText[i])
		hash *= prime64
	}
	return strconv.FormatUint(hash, 16)
}

// reasoningModel reports whether a model id is an OpenAI reasoning model (o-series
// or gpt-5.x), which reject a custom temperature on /chat/completions.
func reasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") || strings.HasPrefix(m, "gpt-5")
}

// assistantLegacyModelBits marks catalog ids that are NOT sensible chat picks:
// embeddings, audio, images, moderation, research/legacy families, and aliases.
var assistantLegacyModelBits = []string{
	"embedding", "whisper", "tts", "audio", "realtime", "moderation",
	"dall-e", "image", "davinci", "babbage", "curie", "instruct",
	"search", "transcribe", "computer-use", "codex", "chatgpt-", "gpt-3.5",
}

// assistantDatedModel matches dated snapshot ids (gpt-4o-2024-08-06, gpt-4-0613).
var assistantDatedModel = regexp.MustCompile(`\d{4}-\d{2}-\d{2}$|-\d{4}$`)

// assistantRecommendedModel reports whether a catalog id belongs in the short
// recommended list: a current, undated chat model.
func assistantRecommendedModel(m string) bool {
	lm := strings.ToLower(strings.TrimSpace(m))
	for _, bit := range assistantLegacyModelBits {
		if strings.Contains(lm, bit) {
			return false
		}
	}
	return !assistantDatedModel.MatchString(lm)
}

// assistantModelFamilyRank orders recommended models newest-family-first so the
// short list leads with the sensible defaults (mini variants right after their
// full-size family — the cost-conscious pick stays visible).
func assistantModelFamilyRank(m string) int {
	lm := strings.ToLower(m)
	switch {
	case strings.HasPrefix(lm, "gpt-5"):
		return 0
	case strings.HasPrefix(lm, "o4"):
		return 1
	case strings.HasPrefix(lm, "o3"):
		return 2
	case strings.HasPrefix(lm, "gpt-4.1"):
		return 3
	case strings.HasPrefix(lm, "gpt-4o"):
		return 4
	default:
		return 5
	}
}

// assistantRecommendedCap is the most models the short list shows; the tail is
// reachable behind the picker's "Show all models" toggle.
const assistantRecommendedCap = 6

// assistantModelSplit partitions the /models catalog for the picker (QA CF-20:
// after key setup the raw list buried three sensible choices under dozens of
// dated, legacy, and research ids): a short, family-ranked recommended set of
// current chat models (capped — 2026-07-18 assessment: even the undated set
// was "too broad for normal users"), everything else behind the Advanced
// toggle. The current selection always stays visible.
func assistantModelSplit(models []string, cur string) (rec, rest []string) {
	ids := models
	if len(ids) == 0 {
		ids = []string{"gpt-5.4-mini", "gpt-5.5", "o4-mini"}
	}
	cur = strings.TrimSpace(cur)
	seenCur := false
	for _, m := range ids {
		if m == cur {
			seenCur = true
		}
		if assistantRecommendedModel(m) {
			rec = append(rec, m)
		} else {
			rest = append(rest, m)
		}
	}
	sort.SliceStable(rec, func(i, j int) bool {
		return assistantModelFamilyRank(rec[i]) < assistantModelFamilyRank(rec[j])
	})
	if len(rec) > assistantRecommendedCap {
		rest = append(rec[assistantRecommendedCap:], rest...)
		rec = rec[:assistantRecommendedCap]
	}
	if cur != "" && !seenCur {
		rec = append([]string{cur}, rec...)
	}
	// The active model always sits in the visible list, wherever it came from.
	if cur != "" {
		for i, m := range rest {
			if m == cur {
				rest = append(rest[:i:i], rest[i+1:]...)
				rec = append([]string{cur}, rec...)
				break
			}
		}
	}
	if len(rec) == 0 {
		rec, rest = rest, nil
	}
	return rec, rest
}

type modelPickerProps struct {
	Models  []string
	Current string
	OnPick  func(string)
}

// modelPicker is the inline model switcher in the assistant header. Its own component
// so the select's change hook sits at a stable position (the option list is variable).
func modelPicker(p modelPickerProps) ui.Node { return ui.CreateElement(modelPickerComp, p) }

func modelPickerComp(p modelPickerProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) {
		if p.OnPick != nil {
			p.OnPick(e.GetValue())
		}
	})
	// Progressive disclosure (2026-07-18 assessment: even the split list was
	// "too broad for normal users"): the select holds ONLY the short ranked
	// recommended set until "All models" is switched on, which appends the
	// long tail (dated snapshots, legacy/research families) behind a separator.
	showAll := ui.UseState(false)
	onToggleAll := ui.UseEvent(func() { showAll.Set(!showAll.Get()) })
	// The shared control-pill (.todo-ctrl + .todo-select) used on To-do/Goals/Budgets/
	// Accounts/Transactions: an uppercase label + a borderless in-pill select, so the
	// model switcher reads identically to every other page's controls.
	rec, rest := assistantModelSplit(p.Models, p.Current)
	showRest := showAll.Get() && len(rest) > 0
	return Fragment(
		Label(css.Class("todo-ctrl"), Title(uistate.T("assistant.modelPick")),
			Span(css.Class("todo-ctrl-label"), uistate.T("assistant.modelLabel")),
			Select(css.Class("todo-select"), Attr("aria-label", uistate.T("assistant.modelPick")), Attr("data-testid", "assistant-model"), OnChange(onChange),
				MapKeyed(rec,
					func(m string) any { return m },
					func(m string) ui.Node { return Option(Value(m), SelectedIf(m == p.Current), m) },
				),
				If(showRest, Option(Value(""), Attr("disabled", "disabled"), "── "+uistate.T("assistant.allModels")+" ──")),
				If(showRest, Fragment(MapKeyed(rest,
					func(m string) any { return "rest:" + m },
					func(m string) ui.Node { return Option(Value(m), SelectedIf(m == p.Current), m) },
				))),
			),
		),
		If(len(rest) > 0, Button(css.Class("btn-link"), Type("button"),
			Attr("data-testid", "assistant-model-showall"),
			Attr("aria-pressed", ariaBool(showAll.Get())),
			Attr("title", uistate.T("assistant.allModelsHint")),
			OnClick(onToggleAll),
			IfElse(showAll.Get(), Text(uistate.T("assistant.fewerModels")), Text(uistate.T("assistant.moreModels"))))),
	)
}

type budgetPickerProps struct {
	Budget    int
	Used      int
	Remaining int
	Capped    bool
	OnPick    func(int)
}

// budgetPicker is the per-conversation spending cap and its remaining readout
// (C390). Its own component so the select's change hook sits at a stable position.
func budgetPicker(p budgetPickerProps) ui.Node { return ui.CreateElement(budgetPickerComp, p) }

// budgetOptions are the caps offered. They are round numbers rather than dollar
// amounts because the cap is enforced in tokens, and a dollar figure would be an
// estimate presented as a limit — the one place an estimate does real harm.
var budgetOptions = []int{0, 25000, 50000, 100000, 250000}

func budgetPickerComp(p budgetPickerProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) {
		n := 0
		if v := strings.TrimSpace(e.GetValue()); v != "" {
			n, _ = strconv.Atoi(v)
		}
		p.OnPick(n)
	})
	// The readout answers the question the cap creates ("how much is left?"). An
	// uncapped chat still shows what it has spent, because that is the number
	// somebody looks at just before deciding to set a cap.
	readout := uistate.T("assistant.budgetUsed", ai.FormatTokens(p.Used))
	cls := "asst-budget-readout"
	if p.Capped {
		readout = uistate.T("assistant.budgetLeft", ai.FormatTokens(p.Remaining))
		if p.Remaining == 0 {
			cls += " is-spent"
		}
	}
	return Label(css.Class("todo-ctrl"), Title(uistate.T("assistant.budgetPick")),
		Span(css.Class("todo-ctrl-label"), uistate.T("assistant.budgetLabel")),
		Select(css.Class("todo-select"), Attr("aria-label", uistate.T("assistant.budgetPick")),
			Attr("data-testid", "assistant-budget"), OnChange(onChange),
			MapKeyed(budgetOptions,
				func(n int) any { return n },
				func(n int) ui.Node {
					return Option(Value(strconv.Itoa(n)), SelectedIf(n == p.Budget), budgetOptionLabel(n))
				},
			),
		),
		Span(ClassStr(cls), Attr("data-testid", "assistant-budget-readout"), readout),
	)
}

// budgetOptionLabel renders a cap as a short token figure, or the no-cap option.
func budgetOptionLabel(n int) string {
	if n == 0 {
		return uistate.T("assistant.budgetNone")
	}
	return uistate.T("assistant.budgetOption", ai.FormatTokens(n))
}

type privacyChipProps struct {
	Tier     aicontext.ConversationTier
	OnToggle func()
}

// privacyChip is the visible per-conversation privacy control (AG17): a chip that
// states the active tier ("Full detail" / "Aggregates only") and toggles it on
// click. Its own component so the click hook sits at a stable position. role=status
// so assistive tech announces the active tier, and the title explains what each
// tier shares so the control is self-documenting.
func privacyChip(p privacyChipProps) ui.Node { return ui.CreateElement(privacyChipComp, p) }

func privacyChipComp(p privacyChipProps) ui.Node {
	onClick := ui.UseEvent(func() {
		if p.OnToggle != nil {
			p.OnToggle()
		}
	})
	agg := p.Tier == aicontext.TierAggregatesOnly
	label := uistate.T("insights.privacyFull")
	title := uistate.T("insights.privacyFullHint")
	if agg {
		label = uistate.T("insights.privacyAggregates")
		title = uistate.T("insights.privacyAggregatesHint")
	}
	cls := "asst-privacy-btn"
	if agg {
		cls += " is-aggregates"
	}
	// Same control-pill (.todo-ctrl) as the model/thinking selects: an uppercase label
	// with the value — here a toggle button (lock glyph + active tier) — sitting
	// borderless inside the pill, so privacy reads as one more standard control.
	return Div(css.Class("todo-ctrl"),
		Span(css.Class("todo-ctrl-label"), uistate.T("insights.privacyLabel")),
		Button(css.Class(cls), Type("button"), Attr("data-testid", "assistant-privacy-chip"),
			Attr("role", "status"), Attr("aria-live", "polite"),
			Attr("aria-label", uistate.T("insights.privacyAria", label)), Title(title), OnClick(onClick),
			uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(label),
		),
	)
}

type thinkPickerProps struct {
	Effort string
	OnPick func(string)
}

// thinkPicker is the inline thinking-level (reasoning_effort) switcher, shown only for
// reasoning models. Its own component so its change hook stays isolated — the parent
// mounts/unmounts it as the model changes without disturbing its own hook order.
func thinkPicker(p thinkPickerProps) ui.Node { return ui.CreateElement(thinkPickerComp, p) }

func thinkPickerComp(p thinkPickerProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) {
		if p.OnPick != nil {
			p.OnPick(e.GetValue())
		}
	})
	return Label(css.Class("todo-ctrl"), Title(uistate.T("assistant.thinkPick")),
		Span(css.Class("todo-ctrl-label"), uistate.T("assistant.thinkLabel")),
		Select(css.Class("todo-select"), Attr("aria-label", uistate.T("assistant.thinkPick")), Attr("data-testid", "assistant-think"), OnChange(onChange),
			Option(Value("low"), SelectedIf(p.Effort == "low"), uistate.T("assistant.thinkLow")),
			Option(Value("medium"), SelectedIf(p.Effort == "medium" || p.Effort == ""), uistate.T("assistant.thinkMedium")),
			Option(Value("high"), SelectedIf(p.Effort == "high"), uistate.T("assistant.thinkHigh")),
		),
	)
}

// scrollChatToEnd scrolls the bounded canvas (#cf-chat-scroll — the single
// scroller wrapping the thread) to its bottom (only the container, never the
// page), so the latest message stays in view. The scroll is deferred via
// setTimeout so it runs AFTER the bubbles' Markdown innerHTML has been filled
// (each bubble renders in its own effect, growing scrollHeight) — otherwise an
// on-load resume would scroll a still-empty container and land at the top.
func scrollChatToEnd() {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		el := js.Global().Get("document").Call("getElementById", "cf-chat-scroll")
		if el.Truthy() {
			el.Set("scrollTop", el.Get("scrollHeight"))
		}
		return nil
	})
	js.Global().Call("setTimeout", cb, 80)
}

// isAppRoutePath reports whether a raw URL pathname (possibly host-prefixed by the
// route base) resolves to one of the app's registered screens, so a chat deep link
// is recognized as in-app even when phrased with an unexpected host. Custom pages
// (/p/:slug) and the root count as app routes.
func isAppRoutePath(rawPath string) bool {
	lp := uistate.LogicalPath(rawPath)
	if lp == "" || lp == "/" || strings.HasPrefix(lp, "/p/") {
		return true
	}
	seg := lp
	if rest := strings.TrimPrefix(lp, "/"); rest != "" {
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			seg = "/" + rest[:i]
		}
	}
	for _, r := range All() {
		if r.Path == lp || r.Path == seg {
			return true
		}
	}
	return false
}

// evTruthy safely reads a boolean-ish property off a JS event, returning false when
// the property is undefined (synthetic events may omit modifier-key fields) so a
// missing field never panics like Value.Bool on undefined would.
func evTruthy(ev js.Value, prop string) bool {
	v := ev.Get(prop)
	return v.Type() == js.TypeBoolean && v.Bool()
}

// scrollToID scrolls to (and briefly highlights) the element with the given id after
// a short delay — used to jump to a chat-linked item once its screen has rendered.
func scrollToID(id string) {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		el := js.Global().Get("document").Call("getElementById", id)
		if !el.Truthy() {
			return nil
		}
		el.Call("scrollIntoView", js.ValueOf(map[string]any{"behavior": "smooth", "block": "center"}))
		if cl := el.Get("classList"); cl.Truthy() {
			cl.Call("add", "cf-jump-flash")
		}
		return nil
	})
	js.Global().Call("setTimeout", cb, 400)
}

// copyText writes text to the system clipboard (best-effort, no-op if unavailable).
func copyText(text string) {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return
	}
	if cb := nav.Get("clipboard"); cb.Truthy() {
		cb.Call("writeText", text)
	}
}

type pinnedInsightRowProps struct {
	Insight  domain.SavedInsight
	OnDelete func(string)
}

// PinnedInsightRow renders one pinned insight as Markdown (via marked) with its
// date and a remove button. Long insights are clamped to three lines with a Show
// more/less toggle so the list stays compact. It owns its own hooks (per the
// no-hooks-in-loops rule).
func PinnedInsightRow(props pinnedInsightRowProps) ui.Node {
	p := props.Insight
	expanded := ui.UseState(false)
	del := ui.UseEvent(Prevent(func() { props.OnDelete(p.ID) }))
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	mdID := "cf-pin-" + p.ID
	// Re-fill on every render: the vdom strips this imperatively written innerHTML
	// whenever it re-renders the row, and the old signature only changed when the
	// row expanded — so a re-render from anywhere else (a pin added elsewhere in
	// the list, the surrounding screen updating) blanked the insight permanently.
	// See AssistantBubble for the full reasoning.
	ui.UseLayoutEffect(func() func() { renderMarkdown(mdID, p.Text); return nil })

	long := len([]rune(p.Text)) > 140 || strings.Contains(p.Text, "\n")
	descClass := "insights-answer " + tw.Fold(tw.Text135)
	if long && !expanded.Get() {
		descClass += " line-clamp-3"
	}
	moreLabel := uistate.T("insights.showMore")
	if expanded.Get() {
		moreLabel = uistate.T("insights.showLess")
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Div(Attr("id", mdID), ClassStr(descClass)),
			If(long, Button(css.Class("btn-link", tw.Text11, tw.Mt1, tw.SelfStart), Type("button"), OnClick(toggle), moreLabel)),
			// C235: attribute pinned insights as AI-generated and show a prefs-formatted save date.
			Span(css.Class("row-meta"), uistate.T("insights.pinnedAttribution", uistate.LoadPrefs().FormatDate(p.CreatedAt))),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("insights.unpinTitle")), Title(uistate.T("insights.unpinTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// respondApproval sends the user's yes/no to a pending mutating tool (no-op if none).
func respondApproval(pa *approvalReq, ok bool) {
	if pa != nil {
		pa.resp <- ok
	}
}

type approvalCardProps struct {
	Preview   string
	Perm      toolperm.Permission
	OnApprove func()
	OnDecline func()
}

// ApprovalCard asks the user to approve or decline a pending mutating tool. Its own
// component so its action hooks re-attach cleanly each time it mounts.
//
// The card leads with the tool's own sentence — that is what the assistant is
// proposing, in its words — and puts the structured reading underneath: what it
// will change, what it needs to look at to do so, and whether it can be undone
// (C388). The order matters: the change comes first because that is what is being
// consented to; the reads come second because they are the part people forget to
// ask about; reversibility comes last because it decides how carefully to read the
// first two.
func ApprovalCard(p approvalCardProps) ui.Node {
	approve := ui.UseEvent(Prevent(func() { p.OnApprove() }))
	decline := ui.UseEvent(Prevent(func() { p.OnDecline() }))
	return Div(css.Class("asst-approve"), Attr("data-testid", "assistant-approval"),
		Attr("role", "group"), Attr("aria-label", uistate.T("insights.approveTitle")),
		P(css.Class("asst-approve-title"), uistate.T("insights.approveTitle")),
		P(css.Class("asst-approve-preview"), p.Preview),
		approvalEffects(p.Perm),
		Div(css.Class("asst-approve-actions"),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "assistant-approve"), OnClick(approve), uistate.T("insights.approve")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "assistant-decline"), OnClick(decline), uistate.T("insights.decline")),
		),
	)
}

// approvalEffects renders the structured half of the approval card. It renders
// nothing when the permission is empty, so a caller with no structured reading
// (an older code path, a tool with no arguments yet) still shows a usable card.
func approvalEffects(perm toolperm.Permission) ui.Node {
	if len(perm.Writes) == 0 && len(perm.Reads) == 0 {
		return Fragment()
	}
	rows := make([]any, 0, len(perm.Writes)+len(perm.Reads)+1)
	for _, w := range perm.Writes {
		rows = append(rows, Li(css.Class("asst-approve-effect", "is-write"),
			Span(css.Class("asst-approve-dot"), Attr("aria-hidden", "true")), w.Line()))
	}
	for _, r := range perm.Reads {
		rows = append(rows, Li(css.Class("asst-approve-effect", "is-read"),
			Span(css.Class("asst-approve-dot"), Attr("aria-hidden", "true")), r.Line()))
	}
	undoKey := "insights.approveIrreversible"
	undoCls := "asst-approve-undo is-permanent"
	if perm.Reversible {
		undoKey = "insights.approveReversible"
		undoCls = "asst-approve-undo"
	}
	body := append([]any{css.Class("asst-approve-effects"), Attr("data-testid", "assistant-approval-effects")}, rows...)
	return Fragment(
		Ul(body...),
		P(ClassStr(undoCls), Attr("data-testid", "assistant-approval-undo"), uistate.T(undoKey)),
	)
}

// confidenceChip marks a finding that is an inference rather than a restatement of
// recorded data (C391).
//
// It renders NOTHING on a finding that is simply arithmetic over the ledger. That
// is the whole design: a tier chip on every row would be wallpaper, and wallpaper
// is not read. The chip appears exactly where it changes what to do next — "these
// two look like duplicates" deserves a second look in a way that "this budget is
// over" does not — so its presence carries the meaning before its text is read.
func confidenceChip(in smart.Insight) ui.Node {
	c := in.ResolvedConfidence()
	if !c.Hedged() {
		return Fragment()
	}
	cls := "insight-conf"
	if c == smart.ConfidencePossible {
		cls += " is-possible"
	}
	return Span(ClassStr(cls), Attr("data-testid", "flag-confidence"),
		Title(uistate.T("assistant.confidenceHint")), c.Label())
}

type suggestChipProps struct {
	Q      string
	OnPick func(string)
}

// suggestChip renders one tappable starter question that fills the Ask box. Its own
// component so the click handler's hook stays stable across the chip list.
func suggestChip(props suggestChipProps) ui.Node {
	q, onPick := props.Q, props.OnPick
	return Button(css.Class("btn chip-suggest"), Type("button"), OnClick(func() { onPick(q) }), q)
}

// exampleConversationsNode renders 2–3 static, clearly-labelled example Q→A pairs
// so keyless users can preview the AI assistant's value before adding a key (C248).
// The examples are purely illustrative — no inputs, no handlers — so they are safe
// to render in a plain loop (no OnClick/UseEvent inside).
func exampleConversationsNode() ui.Node {
	type examplePair struct{ q, a string }
	pairs := []examplePair{
		{uistate.T("insights.exampleQ1"), uistate.T("insights.exampleA1")},
		{uistate.T("insights.exampleQ2"), uistate.T("insights.exampleA2")},
		{uistate.T("insights.exampleQ3"), uistate.T("insights.exampleA3")},
	}
	rows := make([]any, 0, len(pairs)*2)
	for _, p := range pairs {
		rows = append(rows,
			// User bubble: right-aligned via MlAuto, sky tint — mirrors the real chat.
			Div(css.Class(tw.Flex, tw.JustifyStart, tw.Mb2),
				Div(css.Class("asst-msg-user", tw.MaxW85, tw.Text13, tw.WhitespacePreWrap, tw.MlAuto), p.q),
			),
			// Assistant bubble: left-aligned, neutral tint — mirrors the real chat.
			Div(css.Class(tw.Flex, tw.JustifyStart, tw.Mb2),
				Div(css.Class("chat-row-agent", tw.MaxW85),
					Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
					Div(css.Class("chat-agent-body", tw.Text13), p.a),
				),
			),
		)
	}
	// The demo transcript must be visually DISTINCT from a live thread (dashed
	// container, dimmed bubbles) — reusing the real bubble style verbatim made a
	// keyless first-run read scripted answers as their own figures.
	return Div(css.Class("asst-examples", tw.Mt3, tw.Mb2), Attr("data-testid", "assistant-examples"),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb2),
			Span(css.Class(tw.Text12, tw.FontSemibold, tw.TextFaint), uistate.T("insights.examplesLabel")),
			Span(css.Class(tw.Text11, tw.TextFaint), "·"),
			Span(css.Class(tw.Text12, tw.TextFaint), uistate.T("insights.examplesHint")),
		),
		Div(rows...),
		// (The add-a-key CTA lives once, in the agent intro — repeating it here
		// made the keyless screen pitch the key three separate times.)
	)
}

// smartAnomalyInsightRowProps carries one detector finding to its per-row
// component. The route is the page the action navigates to; OnClick holds the
// handler so On* never lives inside a loop.
type smartAnomalyInsightRowProps struct {
	Insight   smart.Insight
	Route     string
	OnClick   func() // navigate to the finding's source (transactions / accounts)
	OnDiscuss func() // drop the finding's context into the chat for a discussion
}

// SmartAnomalyInsightRow renders one flagged-activity row with a click-through
// to the relevant page. It is its own component so OnClick registers at a
// stable hook position across the list (no On* in loops).
func SmartAnomalyInsightRow(p smartAnomalyInsightRowProps) ui.Node {
	navigate := ui.UseEvent(func() { p.OnClick() })
	discuss := ui.UseEvent(func() {
		if p.OnDiscuss != nil {
			p.OnDiscuss()
		}
	})
	iconName := icon.AlertTriangle
	if p.Insight.Severity == smart.SeverityInfo {
		iconName = icon.AlertCircle
	}
	// A row is no longer a single click-through button — it carries two explicit
	// actions: "Source" navigates to the finding's transaction/account, and "Discuss"
	// drops its context into the chat so the user can talk it through with the agent.
	return Div(css.Class("insight-row insight-row-flagged"),
		Span(ClassStr("insight-dot text-down"), uiw.Icon(iconName, css.Class(tw.W4, tw.H4))),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.MinW0, tw.WFull),
			// Full text rides the title attribute: the CSS ellipsis can land right
			// after an amount's period ("about $20.…" reads as four dots) and there
			// is no room to widen the rail — hover/long-press reveals the whole
			// finding instead (UI/UX tasks #37/#38).
			Span(css.Class(tw.Text14, tw.FontMedium, tw.Truncate), Title(p.Insight.Title), p.Insight.Title),
			Span(css.Class("muted", tw.Text13, tw.Truncate), Title(p.Insight.Detail), p.Insight.Detail),
			confidenceChip(p.Insight),
			Div(css.Class("insight-row-actions"),
				// QA CF-22: every row's actions shared one accessible name ("Go to the
				// source of this flag" ×15) — the name now carries the finding's title
				// so AT users can tell the targets apart.
				Button(css.Class("insight-row-btn"), Type("button"),
					Attr("data-testid", "flag-source"), Attr("aria-label", uistate.T("assistant.flagSourceAriaFor", p.Insight.Title)),
					Title(uistate.T("assistant.flagSourceAriaFor", p.Insight.Title)), OnClick(navigate),
					uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
					Span(uistate.T("assistant.flagSource")),
				),
				// "Discuss" only where there's a chat to drop the context into (the Ask
				// tab); the Insights data panel reuses this row without a chat, so it
				// passes no OnDiscuss and the button is omitted.
				If(p.OnDiscuss != nil, Button(css.Class("insight-row-btn"), Type("button"),
					Attr("data-testid", "flag-discuss"), Attr("aria-label", uistate.T("assistant.flagDiscussAriaFor", p.Insight.Title)),
					Title(uistate.T("assistant.flagDiscussAriaFor", p.Insight.Title)), OnClick(discuss),
					uiw.Icon(icon.MessageCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
					Span(uistate.T("assistant.flagDiscuss")),
				)),
			),
		),
	)
}

// flaggedGroupMin is the run length at which same-kind findings fold into one
// collapsible summary row instead of listing individually. A run of two is
// clearer shown in full; three or more is the "wall of nearly identical rows"
// the grouping exists to tame (detail5).
const flaggedGroupMin = 3

// insightRun is a maximal run of consecutive same-feature findings. Runs shorter
// than flaggedGroupMin render as individual rows; longer runs fold into one
// SmartAnomalyGroupRow.
type insightRun struct {
	Feature string
	Items   []smart.Insight
}

// groupInsightRuns folds a findings list into consecutive same-feature runs,
// preserving order. It is a pure presentation-layer fold — it never re-runs or
// re-ranks the detectors, only groups what they already produced (the smartengine
// stays untouched).
func groupInsightRuns(ins []smart.Insight) []insightRun {
	var runs []insightRun
	for _, in := range ins {
		if n := len(runs); n > 0 && runs[n-1].Feature == in.Feature {
			runs[n-1].Items = append(runs[n-1].Items, in)
			continue
		}
		runs = append(runs, insightRun{Feature: in.Feature, Items: []smart.Insight{in}})
	}
	return runs
}

// flaggedGroupHeading is the plain-English summary line for a folded run: the
// kind of finding plus how many there are.
func flaggedGroupHeading(feature string, n int) string {
	switch feature {
	case "SMART-T7": // missing / expected transaction
		return uistate.T("detail5.groupExpected", n)
	case "SMART-T2": // duplicate transaction
		return uistate.T("detail5.groupDuplicate", n)
	case "SMART-T6": // spending spike
		return uistate.T("detail5.groupSpike", n)
	case "SMART-A1": // balance anomaly
		return uistate.T("detail5.groupBalance", n)
	}
	return uistate.T("detail5.groupGeneric", n)
}

// smartAnomalyGroupRowProps carries a folded run of same-kind findings to its
// component. OnReview (optional) is the group's primary action — for expected
// payments it navigates to the recurring/bills surface. OnNavigate opens one
// item's source page (route passed by the child row).
type smartAnomalyGroupRowProps struct {
	Run         insightRun
	Heading     string
	ReviewLabel string // "" = no primary action
	ReviewAria  string
	OnReview    func()       // nil unless ReviewLabel is set
	OnNavigate  func(string) // per-item source navigation
}

// SmartAnomalyGroupRow renders a collapsed summary of a same-kind run — one
// headline with the count, an expand toggle, and (optionally) a primary action —
// that expands to the individual findings. Its own component so the open-state
// hook sits at a stable position across the list (no On* in loops).
func SmartAnomalyGroupRow(p smartAnomalyGroupRowProps) ui.Node {
	open := ui.UseState(false)
	toggle := ui.UseEvent(func() { open.Set(!open.Get()) })
	review := ui.UseEvent(func() {
		if p.OnReview != nil {
			p.OnReview()
		}
	})
	n := len(p.Run.Items)
	chev := icon.ChevronRight
	toggleLabel, toggleAria := uistate.T("detail5.groupExpand"), uistate.T("detail5.groupExpandAria", n)
	if open.Get() {
		chev = icon.ChevronDown
		toggleLabel, toggleAria = uistate.T("detail5.groupCollapse"), uistate.T("detail5.groupCollapseAria", n)
	}

	actions := []any{css.Class("insight-row-actions"),
		Button(css.Class("insight-row-btn"), Type("button"),
			Attr("data-testid", "flag-group-toggle"),
			Attr("aria-expanded", ariaBool(open.Get())), Attr("aria-label", toggleAria),
			Title(toggleAria), OnClick(toggle),
			uiw.Icon(chev, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(toggleLabel),
		),
	}
	if p.ReviewLabel != "" {
		actions = append(actions, Button(css.Class("insight-row-btn"), Type("button"),
			Attr("data-testid", "flag-group-review"), Attr("aria-label", p.ReviewAria),
			Title(p.ReviewAria), OnClick(review),
			uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(p.ReviewLabel),
		))
	}

	var body ui.Node = Fragment()
	if open.Get() {
		rows := make([]ui.Node, 0, n)
		for _, ins := range p.Run.Items {
			capturedIns := ins
			route := "/transactions"
			if capturedIns.Page == smart.PageAccounts {
				route = "/accounts"
			}
			capturedRoute := route
			rows = append(rows, ui.CreateElement(SmartAnomalyInsightRow, smartAnomalyInsightRowProps{
				Insight: capturedIns,
				Route:   capturedRoute,
				OnClick: func() { p.OnNavigate(capturedRoute) },
			}))
		}
		body = Div(css.Class("insight-list"), Style(map[string]string{"margin-left": "1.5rem"}), rows)
	}

	return Div(css.Class(tw.FlexCol), Attr("data-testid", "flag-group"), Attr("data-feature", p.Run.Feature),
		Div(css.Class("insight-row insight-row-flagged"),
			Span(ClassStr("insight-dot text-down"), uiw.Icon(icon.AlertTriangle, css.Class(tw.W4, tw.H4))),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.MinW0, tw.WFull),
				Span(css.Class(tw.Text14, tw.FontMedium, tw.Truncate), Title(p.Heading), p.Heading),
				Div(actions...),
			),
		),
		body,
	)
}

// insightSourceCue is the small trailing chevron that marks a deterministic
// insight row as drilling to its source transactions — the same evidence
// affordance the flagged-activity rows carry, rendered consistently on the
// category-shift and top-merchant rows (detail5 audit-and-fill). It is a
// decorative marker (aria-hidden); the row's own aria-label already names the
// drill target, and the hint rides the title for sighted users.
func insightSourceCue() ui.Node {
	return Span(css.Class("insight-src-cue", tw.ShrinkO, tw.TextFaint), Attr("aria-hidden", "true"),
		Title(uistate.T("detail5.sourceHint")),
		uiw.Icon(icon.ChevronRight, css.Class(tw.W35, tw.H35)))
}

// collapsibleNoteProps configures a collapsible aside "margin note" section: a label
// (with an optional count badge + trailing link) that toggles a body. It starts
// COLLAPSED so the assistant rail is compact by default and the user expands only what
// they want to see.
type collapsibleNoteProps struct {
	Label  string
	TestID string
	Count  int
	Link   ui.Node
	Body   ui.Node
}

// collapsibleNote renders one collapsible aside section. It's its own component so the
// toggle's UseState survives the aside's frequent re-renders (the aside re-runs on every
// chat keystroke) rather than resetting the way native <details> would.
func collapsibleNote(props collapsibleNoteProps) ui.Node {
	return ui.CreateElement(collapsibleNoteComp, props)
}

func collapsibleNoteComp(p collapsibleNoteProps) ui.Node {
	open := ui.UseState(false) // start collapsed
	toggle := ui.UseEvent(func() { open.Set(!open.Get()) })
	chev := icon.ChevronRight
	if open.Get() {
		chev = icon.ChevronDown
	}
	btn := []any{
		css.Class("ask-note-toggle"), Type("button"),
		Attr("aria-expanded", fmt.Sprintf("%v", open.Get())), OnClick(toggle),
		uiw.Icon(chev, css.Class("ask-note-chev", tw.W3, tw.H3)),
		Span(css.Class("ask-note-label"), p.Label),
	}
	if p.Count > 0 {
		btn = append(btn, Span(css.Class("ask-note-count"), fmt.Sprintf("%d", p.Count)))
	}
	if p.TestID != "" {
		btn = append(btn, Attr("data-testid", p.TestID))
	}
	head := []any{css.Class("ask-note-head"), Button(btn...)}
	if p.Link != nil && open.Get() {
		head = append(head, p.Link)
	}
	var body ui.Node = Fragment()
	if open.Get() {
		body = Div(css.Class("ask-note-body"), p.Body)
	}
	return Div(css.Class("ask-note"), Div(head...), body)
}

// smartAnomalyHighlights runs the four anomaly-type SMART detectors (SMART-A1
// balance anomaly, SMART-T2 duplicates, SMART-T6 spending spikes, SMART-T7
// missing transaction) unconditionally — no Smart opt-in gate — and renders
// their findings as a "Flagged activity" card on /insights. Returns an empty
// node when the detectors find nothing.
func smartAnomalyHighlights(app *appstate.App, weekStart time.Weekday, ready bool, onDiscuss func(smart.Insight)) ui.Node {
	nav := router.UseNavigate()
	// Run with all Free features enabled so the four anomaly detectors always
	// fire regardless of the user's per-feature SMART opt-in state. Memoized on the
	// data revision + week start: the detectors scan every transaction, and this card
	// re-renders on every chat keystroke — recomputing per character was pure waste.
	// The result is read-only (iterated to build rows below). `ready` is false on the
	// caller's first paint so the scan is deferred off the initial mount.
	flagged := ui.UseMemo(func() []smart.Insight {
		if !ready {
			return nil
		}
		return runAnomalyDetectors(app, weekStart)
	}, app.Rev(), int(weekStart), ready)
	if len(flagged) == 0 {
		return Fragment()
	}

	rows := make([]ui.Node, 0, len(flagged))
	for _, ins := range flagged {
		route := "/transactions"
		if ins.Page == smart.PageAccounts {
			route = "/accounts"
		}
		capturedIns := ins
		capturedRoute := route
		rows = append(rows, ui.CreateElement(SmartAnomalyInsightRow, smartAnomalyInsightRowProps{
			Insight: capturedIns,
			Route:   capturedRoute,
			OnClick: func() { nav.Navigate(uistate.RoutePath(capturedRoute)) },
			OnDiscuss: func() {
				if onDiscuss != nil {
					onDiscuss(capturedIns)
				}
			},
		}))
	}

	return collapsibleNote(collapsibleNoteProps{
		Label:  uistate.T("insights.flaggedTitle"),
		TestID: "assistant-note-flagged",
		Count:  len(flagged),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("insights.flaggedHint")),
			Div(css.Class("insight-list"), rows),
		),
	})
}

// insightsHighlightRowProps carries the display data and drill callback for one
// spending-highlight row. OnDrill is called with the anomaly's category name
// when the user clicks the row, so the parent can resolve it to an ID and
// navigate to /transactions filtered to that category (C228).
type insightsHighlightRowProps struct {
	Anomaly insights.Anomaly
	Base    string
	// Attribution is what explains the overspend, or the finding that nothing
	// single-handedly does. Empty when there is nothing honest to say.
	Attribution string
	// Verdict is the judgement already recorded against this flag, if any.
	Verdict flagverdict.Verdict
	// OnJudge records a verdict (empty clears it). Nil hides the control, which
	// is what surfaces that cannot persist a judgement should do rather than
	// offering one that goes nowhere.
	OnJudge func(v flagverdict.Verdict)
	OnDrill func(catName string)
}

// insightsHighlightRow renders a single clickable spending-highlight row. It is
// a standalone component so its OnClick hook is registered at a stable render
// position — not inside the variable-length anomaly loop in spendingHighlights
// (CRITICAL: never call On* helpers inside a variable-length loop).
func insightsHighlightRow(props insightsHighlightRowProps) ui.Node {
	a := props.Anomaly
	drill := ui.UseEvent(func() { props.OnDrill(a.Category) })
	// The judgement control is a SIBLING of the drill button, never inside it: a
	// button nested in a button is invalid, and a click on the inner one would
	// also drill through to the ledger — the reader would answer a question and
	// be thrown onto another page for their trouble.
	judge := ui.UseEvent(func(v string) {
		if props.OnJudge != nil {
			props.OnJudge(flagverdict.Verdict(v))
		}
	})
	row := Button(
		css.Class("insight-row insight-row--clickable"),
		Type("button"),
		Attr("aria-label", uistate.T("insights.highlightDrillAria", a.Category)),
		OnClick(drill),
		Span(ClassStr("insight-dot "+highlightTone(a)), uiw.Icon(highlightArrow(a), css.Class(tw.W4, tw.H4))),
		Span(css.Class(tw.Flex1, tw.TextLeft),
			Span(css.Class(tw.Block), highlightText(a, props.Base)),
			If(props.Attribution != "",
				Span(css.Class("muted"), css.Class(tw.Block), css.Class(tw.TextXs),
					Attr("data-testid", "insight-attribution"), props.Attribution)),
			If(verdictFollowUp(props.Verdict) != "",
				Span(css.Class(tw.Block), css.Class(tw.TextXs),
					Attr("data-testid", "insight-verdict-follow"), verdictFollowUp(props.Verdict))),
		),
		insightSourceCue(),
	)
	if props.OnJudge == nil {
		return row
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
		row,
		verdictSelect(a.Category, props.Verdict, judge),
	)
}

// verdictSelect offers the five judgements for a flag. The blank first option is
// the honest default: no verdict has been recorded, which is different from any
// of the five, and it doubles as the way back — re-selecting it clears a
// judgement, so a flag silenced by mistake returns.
func verdictSelect(category string, current flagverdict.Verdict, onChange ui.Handler) ui.Node {
	args := []any{
		css.Class("field", tw.TextXs),
		Attr("data-testid", "insight-verdict"),
		Attr("aria-label", uistate.T("insights.verdictAria", category)),
		OnChange(onChange),
		Option(Value(""), SelectedIf(current == ""), uistate.T("insights.verdictNone")),
	}
	for _, v := range flagverdict.Verdicts() {
		args = append(args, Option(Value(string(v)), SelectedIf(current == v), verdictLabel(v)))
	}
	return Select(args...)
}

// verdictLabel is the plain-English name of a verdict.
func verdictLabel(v flagverdict.Verdict) string {
	switch v {
	case flagverdict.OneTime:
		return uistate.T("insights.verdictOneTime")
	case flagverdict.Expected:
		return uistate.T("insights.verdictExpected")
	case flagverdict.WrongCategory:
		return uistate.T("insights.verdictWrongCategory")
	case flagverdict.NewNormal:
		return uistate.T("insights.verdictNewNormal")
	case flagverdict.Investigate:
		return uistate.T("insights.verdictInvestigate")
	}
	return string(v)
}

// judgedFlagsNoteProps carries the hidden-flag count and the way back.
type judgedFlagsNoteProps struct {
	Count    int
	Shown    bool
	OnToggle func()
}

// judgedFlagsNote says how many flags a recorded judgement is hiding, and offers
// them back. It is its own component so its click hook sits at a stable render
// position.
//
// It exists because a suppression the user cannot see is indistinguishable from
// a detector that stopped working. Saying "2 flags you judged are hidden" costs
// one muted line and keeps the silence accountable.
func judgedFlagsNote(props judgedFlagsNoteProps) ui.Node {
	toggle := ui.UseEvent(Prevent(func() { props.OnToggle() }))
	label := "insights.verdictHiddenShow"
	if props.Shown {
		label = "insights.verdictHiddenHide"
	}
	return P(css.Class("muted"), css.Class(tw.TextXs), Attr("data-testid", "insight-judged-note"),
		uistate.TN("insights.verdictHiddenOne", "insights.verdictHiddenMany", props.Count),
		" ",
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "insight-judged-toggle"),
			OnClick(toggle), uistate.T(label)),
	)
}

// flagKey identifies one category's flag in one month. The month is part of it
// because a one-off is a statement about THAT month: next month's flag on the
// same category is new information, and must not inherit last month's answer.
func flagKey(category string, now time.Time) string {
	return category + "|" + now.Format("2006-01")
}

// verdictFollowUp is the line a non-hiding verdict leaves on the row. "Wrong
// category" and "Investigate" deliberately do not silence the flag, so without
// this the reader answers the question and sees nothing happen — which reads as
// the app ignoring them.
func verdictFollowUp(v flagverdict.Verdict) string {
	switch flagverdict.Effect(v).Follow {
	case flagverdict.FollowRecategorize:
		return uistate.T("insights.verdictFollowRecategorize")
	case flagverdict.FollowTrack:
		return uistate.T("insights.verdictFollowTrack")
	}
	return ""
}

// spendingHighlights renders an offline "what changed" card: it detects
// categories whose spend this month deviates materially from their recent
// average and explains each in plain English. It needs no AI key. Returns an
// empty node when there's nothing notable, so the card simply doesn't appear.
// Each row is wrapped in its own component so the OnClick hook stays at a
// stable render position (C228 drill-through).
//
// Anomalies are computed by the caller (and memoized there) rather than here: the
// detection builds four monthly per-category spend series over every transaction,
// and this card re-renders on every chat keystroke — so recomputing it inline was
// per-character waste. This function is now a pure renderer of pre-computed data.
func spendingHighlights(anomalies []insights.Anomaly, base string, attrib func(insights.Anomaly) string, onDrill func(catName string)) ui.Node {
	if len(anomalies) == 0 {
		return Fragment()
	}

	rows := MapKeyed(anomalies,
		func(a insights.Anomaly) any { return a.Category },
		func(a insights.Anomaly) ui.Node {
			line := ""
			if attrib != nil {
				line = attrib(a)
			}
			return ui.CreateElement(insightsHighlightRow, insightsHighlightRowProps{
				Anomaly:     a,
				Base:        base,
				Attribution: line,
				OnDrill:     onDrill,
			})
		},
	)

	return collapsibleNote(collapsibleNoteProps{
		Label:  uistate.T("insights.highlightsTitle"),
		TestID: "assistant-note-highlights",
		Count:  len(anomalies),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("insights.highlightsHint")),
			Div(css.Class("insight-list"), rows),
		),
	})
}

// anomalyPurchases collects the current month's spending behind one flagged
// category, using EXACTLY the filters the detection itself used
// (ledger.CategorySpendSeries): expenses that count in reports, converted to the
// base currency, taken as magnitudes and grouped by the transaction's own
// category. Any looser filter here would produce a culprit list that contradicts
// the total it claims to explain — an excluded reimbursement named as the cause
// of an overspend it was deliberately kept out of.
//
// Larger marks a purchase materially bigger than what that payee usually costs,
// judged against the same three baseline months the anomaly was measured over. A
// payee never seen before is not "larger" — it has no usual to be larger than.
func anomalyPurchases(catID string, txns []domain.Transaction, rates currency.Rates, now time.Time) []anomalyattrib.Purchase {
	curStart, _ := dateutil.MonthRange(now)
	baseStart := dateutil.AddMonths(curStart, -3)
	curEnd := dateutil.AddMonths(curStart, 1)

	var cur []anomalyattrib.Purchase
	priorSum := map[string]int64{}
	priorN := map[string]int{}
	for _, t := range txns {
		if t.CategoryID != catID || !t.IsExpense() || !t.CountsInReports() {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			continue
		}
		minor := conv.Abs().Amount
		switch {
		case dateutil.InRange(t.Date, curStart, curEnd):
			payee := strings.TrimSpace(t.Payee)
			if payee == "" {
				payee = strings.TrimSpace(t.Desc)
			}
			if payee == "" {
				payee = uistate.T("insights.attribUnnamedPayee")
			}
			cur = append(cur, anomalyattrib.Purchase{ID: t.ID, Payee: payee, Minor: minor})
		case dateutil.InRange(t.Date, baseStart, curStart):
			key := strings.ToLower(strings.TrimSpace(t.Payee))
			priorSum[key] += minor
			priorN[key]++
		}
	}
	for i, p := range cur {
		key := strings.ToLower(p.Payee)
		if n := priorN[key]; n > 0 {
			usual := priorSum[key] / int64(n)
			// Half again as much as usual, so ordinary variation in a repeat
			// purchase is not reported as remarkable.
			cur[i].Larger = usual > 0 && p.Minor*2 > usual*3
		}
	}
	return cur
}

// anomalyAttributionText answers the question a spending flag leaves hanging:
// what did I actually buy. Returns "" when there is nothing honest to add.
//
// The negative answer is carried too, and is the more useful half. If the
// overspend is spread across every purchase in the category, saying so stops the
// reader hunting for a culprit that does not exist — the habit moved, and no
// single receipt is at fault (WF-SM1).
func anomalyAttributionText(a insights.Anomaly, catID string, txns []domain.Transaction, rates currency.Rates, base string, now time.Time) string {
	if a.Direction != insights.Up || a.Delta <= 0 {
		return "" // nothing was overspent, so nothing needs explaining
	}
	ps := anomalyPurchases(catID, txns, rates, now)
	att := anomalyattrib.Explain(a.Delta, ps)
	if !att.Known {
		return ""
	}
	if !att.Concentrated {
		return uistate.TN("insights.attribDiffuseOne", "insights.attribDiffuseMany", len(ps))
	}
	names := make([]string, 0, len(att.Culprits))
	for _, c := range att.Culprits {
		names = append(names, c.Payee+" "+fmtMoney(money.New(c.Minor, base)))
	}
	joined := strings.Join(names, ", ")
	var line string
	if att.Everything {
		// Not "explains 100%", which reads as a suspiciously round number and
		// invites the reader to check the arithmetic. They account for the whole
		// of it, so say that.
		line = uistate.TN("insights.attribAllOne", "insights.attribAllMany", len(att.Culprits), joined)
	} else {
		line = uistate.TN("insights.attribOne", "insights.attribMany",
			len(att.Culprits), int64(att.ExplainedPct+0.5), joined)
	}
	switch {
	case att.UnusuallyLarge == 0:
	case len(att.Culprits) == 1:
		line += " " + uistate.T("insights.attribLargerThatOne")
	default:
		line += " " + uistate.TN("insights.attribLargerOne", "insights.attribLargerMany", att.UnusuallyLarge)
	}
	return line
}

// spendAnomaliesCache backs detectSpendingAnomaliesMemo (single dashboard surface;
// wasm is single-threaded, so no lock).
var spendAnomaliesCache = map[string][]insights.Anomaly{}

// detectSpendingAnomaliesMemo wraps detectSpendingAnomalies with a revision-keyed
// cache. The detection builds four monthly per-category spend series over every
// transaction — heavy — and the dashboard calls it more than once per render (the
// top-highlight widget and the attention widget), re-running on every dashboard
// re-render. scopeKey distinguishes callers that pass different transaction sets so
// they never share an entry. Returns a fresh copy, so a caller that takes
// &result[0] can't mutate the cached slice. The month is part of the key (the
// series are month-relative); any data edit bumps rev and invalidates.
func detectSpendingAnomaliesMemo(rev uint64, scopeKey string, txns []domain.Transaction, categories []domain.Category, rates currency.Rates) []insights.Anomaly {
	key := strconv.FormatUint(rev, 10) + "|" + scopeKey + "|" + time.Now().Format("2006-01")
	v, ok := spendAnomaliesCache[key]
	if !ok {
		if len(spendAnomaliesCache) > 6 {
			spendAnomaliesCache = map[string][]insights.Anomaly{}
		}
		v = detectSpendingAnomalies(txns, categories, rates)
		spendAnomaliesCache[key] = v
	}
	if len(v) == 0 {
		return nil
	}
	out := make([]insights.Anomaly, len(v))
	copy(out, v)
	return out
}

// detectSpendingAnomalies builds the last four monthly per-category spend series
// and returns the detected anomalies (most significant first). Shared by the
// Insights highlights card and the dashboard top-highlight widget. Returns nil
// when there's nothing to detect.
func detectSpendingAnomalies(txns []domain.Transaction, categories []domain.Category, rates currency.Rates) []insights.Anomaly {
	curStart, _ := dateutil.MonthRange(time.Now())
	// Four monthly periods (three baseline + the current month) → five boundaries.
	bounds := []time.Time{
		dateutil.AddMonths(curStart, -3),
		dateutil.AddMonths(curStart, -2),
		dateutil.AddMonths(curStart, -1),
		curStart,
		dateutil.AddMonths(curStart, 1),
	}
	spendByCat, err := ledger.CategorySpendSeries(txns, bounds, rates)
	if err != nil || len(spendByCat) == 0 {
		return nil
	}
	names := make(map[string]string, len(categories))
	for _, c := range categories {
		names[c.ID] = c.Name
	}
	series := make([]insights.CategorySeries, 0, len(spendByCat))
	for catID, spend := range spendByCat {
		name := names[catID]
		if name == "" {
			name = uistate.T("insights.uncategorized")
		}
		series = append(series, insights.CategorySeries{Category: name, Spend: spend})
	}
	// C232: while the current month is only partly elapsed, suppress "decrease"
	// anomalies — a category not yet spent on this month would otherwise read as a
	// false "down 100%". Increases still surface (an overspend is real as it lands).
	// Threshold: treat the month as "complete enough" to trust a decrease at 90%+.
	opts := insights.DefaultOptions()
	now := time.Now()
	_, monthEnd := dateutil.MonthRange(now)
	monthDays := monthEnd.Sub(curStart).Hours() / 24
	if monthDays > 0 {
		elapsed := now.Sub(curStart).Hours() / 24
		if elapsed/monthDays < 0.9 {
			opts.SuppressDecrease = true
		}
	}
	return insights.Detect(series, opts)
}

// categoryNameToIDMap builds a reverse map from category name → category ID
// used by the drill-through callback (C228) to look up the ID from the
// anomaly's Category field (which is the display name).
func categoryNameToIDMap(categories []domain.Category) map[string]string {
	m := make(map[string]string, len(categories))
	for _, c := range categories {
		m[c.Name] = c.ID
	}
	return m
}

// highlightText is the plain-English sentence for one spending anomaly.
func highlightText(a insights.Anomaly, base string) string {
	current := fmtMoney(money.New(a.Current, base))
	baseline := fmtMoney(money.New(a.Baseline, base))
	// C232: the current period is the in-progress month, so a category with nothing
	// spent yet reads as a misleading "down 100%". State it plainly instead.
	if a.Current == 0 && a.Direction == insights.Down {
		return uistate.T("insights.highlightNone", a.Category, baseline)
	}
	pct := a.PctChange
	if pct < 0 {
		pct = -pct
	}
	// C233: include the explicit dollar change, not just the percentage.
	delta := a.Delta
	if delta < 0 {
		delta = -delta
	}
	deltaStr := fmtMoney(money.New(delta, base))
	key := "insights.highlightDown"
	if a.Direction == insights.Up {
		key = "insights.highlightUp"
	}
	return uistate.T(key, a.Category, pct, deltaStr, current, baseline)
}

// highlightTone is the green/red text class for an anomaly's direction (up in
// spending is red, down is green).
func highlightTone(a insights.Anomaly) string {
	if a.Direction == insights.Up {
		return "text-down"
	}
	return "text-up"
}

// highlightArrow is the arrow-up/arrow-down glyph for an anomaly's direction; it
// inherits the row's tone color via currentColor (C46).
func highlightArrow(a insights.Anomaly) icon.Name {
	if a.Direction == insights.Up {
		return icon.ArrowUp
	}
	return icon.ArrowDown
}

// merchantSpend holds one payee's aggregated expense total for the top-merchants
// card (C229).
type merchantSpend struct {
	Name  string
	Total int64 // minor units, base currency
	Count int
}

// insightsMerchantRowProps carries the display data and drill callback for one
// top-merchants row. OnDrill is called with the merchant name when the user
// clicks the row, navigating to /transactions filtered to that payee (C229).
type insightsMerchantRowProps struct {
	Merchant merchantSpend
	Base     string
	Rank     int
	OnDrill  func(name string)
}

// insightsMerchantRow renders a single clickable top-merchant row. It is its
// own component so its OnClick hook registers at a stable render position — never
// inside the variable-length merchant loop (CRITICAL: never call On* in loops).
func insightsMerchantRow(props insightsMerchantRowProps) ui.Node {
	m := props.Merchant
	drill := ui.UseEvent(func() { props.OnDrill(m.Name) })
	amtStr := fmtMoney(money.New(m.Total, props.Base))
	// QA CF-30: "(1 txns)" — pluralize via the shared helper.
	txLabel := "(" + plural(m.Count, "txn") + ")"
	ariaLabel := uistate.T("insights.merchantDrillAria", m.Name)
	return Button(
		css.Class("insight-row insight-row--clickable"),
		Type("button"),
		Attr("aria-label", ariaLabel),
		OnClick(drill),
		Span(css.Class("insight-rank"), strconv.Itoa(props.Rank)),
		Span(css.Class("insight-merchant-name", tw.Flex1, tw.TextLeft, tw.Truncate), m.Name),
		Span(css.Class("insight-merchant-amount", tw.TextRight),
			Span(css.Class(tw.FontMedium), amtStr),
			Span(css.Class("muted", tw.Text12, tw.Ml1), txLabel),
		),
		insightSourceCue(),
	)
}

// affordCardText builds the inner HTML for a grounded affordability answer card.
// The markup is later set via innerHTML; the outer element carries the
// data-cf="afford-result" selector so e2e tests can assert on it.
func affordCardText(ar insights.AffordResult, q *insights.AffordQuery, base string) string {
	amtStr := fmtMoney(money.New(q.Amount, base))
	projStr := fmtMoney(money.New(ar.Projected, base))
	availStr := fmtMoney(money.New(ar.Available, base))

	var headline, surplusLine string
	if ar.CanAfford {
		headline = uistate.T("insights.affordYes", amtStr)
		surplusStr := fmtMoney(money.New(ar.Surplus, base))
		surplusLine = uistate.T("insights.affordSurplus", surplusStr)
	} else {
		shortfall := ar.Surplus
		if shortfall < 0 {
			shortfall = -shortfall
		}
		shortfallStr := fmtMoney(money.New(shortfall, base))
		headline = uistate.T("insights.affordNo", shortfallStr)
		surplusLine = uistate.T("insights.affordShortfall", shortfallStr)
	}
	projLine := uistate.T("insights.affordProjected", availStr+" (balance "+projStr+")")
	assumptLabel := uistate.T("insights.affordAssumptions")

	var b strings.Builder
	b.WriteString(headline + "\n" + projLine + "\n" + surplusLine + "\n\n" + assumptLabel + "\n")
	for _, a := range ar.Assumptions {
		b.WriteString("- " + a + "\n")
	}
	return b.String()
}

type affordResultBubbleProps struct {
	ID       string
	HTML     string // plain text (Markdown) content for the card
	OnDelete func(string)
}

// AffordResultBubble renders a deterministic affordability answer card in the
// chat thread. It uses the same Markdown renderer as AssistantBubble but carries
// the data-cf="afford-result" attribute for e2e targeting. Its own component so
// the delete hook stays stable across the list (no On* in loops).
func AffordResultBubble(p affordResultBubbleProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.ID) }))
	mdID := "cf-afford-" + p.ID
	// Re-filled every render for the same reason as AssistantBubble: keying on the
	// text meant any re-render that did not change the text left the card blank.
	ui.UseLayoutEffect(func() func() { renderMarkdown(mdID, p.HTML); return nil })
	actBtn := tw.Fold(tw.TextFaint, tw.Opacity70, tw.HoverOpacity100, tw.InlineFlex, tw.ItemsCenter)
	return Div(Attr("data-cf", "afford-result"), css.Class("group", tw.Flex, tw.FlexCol, tw.ItemsStart),
		Div(css.Class(tw.MaxW85, tw.Rounded2xl, tw.Px35, tw.Py25, tw.Border, "border-sky-200 bg-sky-50"),
			Div(Attr("id", mdID), css.Class("md insights-answer", tw.Text14)),
		),
		Div(css.Class(tw.Flex, tw.Gap3, tw.ItemsCenter, tw.Mt1, tw.Px1, tw.Opacity0, tw.GroupHoverOpacity100, tw.GroupFocusWithinOpacity100, tw.MotionSafeTransitionOpacity),
			Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
}
