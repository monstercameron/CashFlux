// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aicontext"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/insights/localqa"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	pickModel := func(v string) {
		modelSel.Set(v)
		s := app.Settings()
		s.OpenAIModel = v
		_ = app.PutSettings(s)
	}
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

		return insights.SuggestedQuestions(insights.QuestionContext{
			TopCategory:     topCat,
			NearLimitBudget: nearLimitBudget,
			UpcomingGoal:    upcomingGoal,
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
	input := ui.UseState("")
	// Shell-style input history: histIdx is the cycle position over prior user messages
	// with Up/Down (-1 = not cycling); histDraft preserves the in-progress draft.
	histIdx := ui.UseState(-1)
	histDraft := ui.UseState("")
	// fillAsk drops a question into the Ask box and focuses it (used by the starter chips
	// and the "Discuss" action on a flagged-activity row) so the user can review or edit
	// before sending.
	fillAsk := func(q string) { input.Set(q); focusByID("cf-chat-input") }
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
	onInput := ui.UseEvent(func(v string) { input.Set(v) })
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

	// sendTools dispatches one model turn: the direct OpenAI path advertises tools; the
	// backend proxy path doesn't support tools yet, so it falls back to a plain reply.
	sendTools := func(messages []ai.Message, tools []ai.Tool, onResult func(ai.Message, ai.Usage), onErr func(string)) func() {
		if useBackendAI {
			return ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, chatTemp,
				func(content string, u ai.Usage) { onResult(ai.Message{Role: ai.RoleAssistant, Content: content}, u) }, onErr)
		}
		// Route the tool loop through the Responses API: it's the only endpoint that
		// accepts reasoning.effort together with function tools for the reasoning models
		// (gpt-5.x / o-series), so the thinking level actually works instead of being
		// rejected by /chat/completions.
		return ai.SendResponsesChatTools(key, aiBaseURL, model, messages, chatTemp, chatEffort, tools, onResult, onErr)
	}

	// run drives the bounded tool-calling loop: ask the model; if it requests tools,
	// execute them locally and feed the results back; repeat until it answers (or a
	// step cap is hit). It runs in a goroutine, blocking on a channel per turn (Go
	// wasm schedules cooperatively, so the fetch callback resumes it). Turns are set
	// deterministically to the sent history + reply (the stale-base fix), and a shared
	// done channel lets Cancel unblock the loop.
	run := func(hist []chatTurn) {
		errMsg.Set("")
		loading.Set(true)
		allTools := buildChatTools(app, base, rates)
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

		go func() {
			for step := 0; step < 6; step++ {
				ch := make(chan agentStep, 1)
				fc := sendTools(msgs, specs,
					func(m ai.Message, u ai.Usage) { ch <- agentStep{msg: m, usage: u} },
					func(e string) { ch <- agentStep{err: e} })
				cancelFn.Set(func() { fc(); closeDone() })

				var r agentStep
				select {
				case r = <-ch:
				case <-done:
					loading.Set(false)
					return
				}
				if r.err != "" {
					loading.Set(false)
					errMsg.Set(r.err)
					return
				}
				total.PromptTokens += r.usage.PromptTokens
				total.CompletionTokens += r.usage.CompletionTokens
				total.TotalTokens += r.usage.TotalTokens

				if !ai.WantsTools(r.msg) {
					loading.Set(false)
					reply := chatTurn{ID: id.New(), Role: "assistant", Text: r.msg.Content, Usage: total}
					turns.Set(append(append([]chatTurn{}, hist...), reply))
					// AG20: feed this turn's token spend into the per-conversation
					// receipt (cost estimated from the resolved model).
					cost, costOK := ai.EstimateCostUSD(model, total)
					uistate.AddAgentCost(convID.Get(), total.TotalTokens, cost, costOK)
					return
				}
				msgs = append(msgs, r.msg)
				for _, tc := range r.msg.ToolCalls {
					args := json.RawMessage(tc.Function.Arguments)
					out := "tool unavailable"
					if tool, ok := byName[tc.Function.Name]; ok {
						// Mutating tools pause for the user's approval in the thread.
						if tool.mutates {
							preview := tc.Function.Name
							if tool.preview != nil {
								preview = tool.preview(args)
							}
							resp := make(chan bool, 1)
							pendingApproval.Set(&approvalReq{tool: tc.Function.Name, preview: preview, resp: resp})
							var approved bool
							select {
							case approved = <-resp:
							case <-done:
								pendingApproval.Set(nil)
								loading.Set(false)
								return
							}
							pendingApproval.Set(nil)
							if !approved {
								msgs = append(msgs, ai.ToolResultMessage(tc.ID, tc.Function.Name, "The user declined this change."))
								continue
							}
						}
						out = tool.run(args)
					}
					msgs = append(msgs, ai.ToolResultMessage(tc.ID, tc.Function.Name, out))
				}
			}
			loading.Set(false)
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
			input.Set("")
			ctxAttach.Set(nil)
			histIdx.Set(-1)
			return
		}

		if key == "" && !useBackendAI {
			// C244: try to answer deterministically via localqa before falling back to
			// the key-hint error. This makes the chat useful even with no OpenAI key.
			intent, matched := localqa.Match(text)
			if matched {
				src := newInsightsQASource(app, base, rates)
				if answer, answered := localqa.Answer(intent, src, text, func(minor int64) string {
					return insightsMoneyFmt(minor, base)
				}); answered {
					hist := append(append([]chatTurn{}, turns.Get()...),
						chatTurn{ID: id.New(), Role: "user", Text: text},
						chatTurn{ID: id.New(), Role: "assistant", Text: answer},
					)
					turns.Set(hist)
					input.Set("")
					ctxAttach.Set(nil)
					histIdx.Set(-1)
					return
				}
			}
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		hist := append(append([]chatTurn{}, turns.Get()...), chatTurn{ID: id.New(), Role: "user", Text: text})
		turns.Set(hist)
		input.Set("")
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
		b.WriteString("\n\n")
		b.WriteString(body)
		return b.String()
	}
	// submitChat sends the composer as one user turn, folding in any attached context.
	submitChat := func() {
		typed := strings.TrimSpace(input.Get())
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
			msgs[i] = domain.ChatMessage{ID: t.ID, Role: t.Role, Text: t.Text, Tokens: t.Usage.TotalTokens, CreatedAt: time.Now()}
		}
		// Keep an AI-generated name once set, rather than re-deriving from the first line.
		title, named := conversationTitle(ts), false
		for _, c := range app.Conversations() {
			if c.ID == cid && c.Named {
				title, named = c.Title, true
				break
			}
		}
		_ = app.PutConversation(domain.Conversation{ID: cid, Title: title, Named: named, Messages: msgs, CreatedAt: created, UpdatedAt: time.Now()})
		bump()
	}

	// switchTo loads a saved conversation into the live thread.
	switchTo := func(cid string) {
		for _, c := range app.Conversations() {
			if c.ID != cid {
				continue
			}
			ts := make([]chatTurn, len(c.Messages))
			for i, m := range c.Messages {
				ts[i] = chatTurn{ID: m.ID, Role: m.Role, Text: m.Text, Usage: ai.Usage{TotalTokens: m.Tokens}}
			}
			turns.Set(ts)
			convID.Set(cid)
			convCreated.Set(c.CreatedAt)
			input.Set("")
			errMsg.Set("")
			return
		}
	}

	// newChat clears the thread for a fresh (unsaved) conversation.
	newChat := func() {
		turns.Set(nil)
		convID.Set("")
		convCreated.Set(time.Time{})
		input.Set("")
		errMsg.Set("")
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
			input.Set(seed)
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
				if len(k) == 1 || k == "Backspace" || k == "Delete" {
					histIdx.Set(-1) // editing leaves history mode
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
					histDraft.Set(input.Get())
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
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0, onName, noErr)
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
	highlights := spendingHighlights(spendingAnoms, base, viewCategoryTransactions)

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
			ctxAttach.Set(append(append([]flagContext{}, cur...),
				flagContext{ID: id.New(), Title: ins.Title, Detail: detail, Kind: ins.Feature}))
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
	openInsightsTab := ui.UseEvent(Prevent(func() { hubTab.Set("insights") }))
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
					return ui.CreateElement(UserBubble, userBubbleProps{ID: t.ID, Text: t.Text, OnDelete: deleteTurn, OnRetry: retryFor(t.ID)})
				}
				if t.Role == "afford" {
					return ui.CreateElement(AffordResultBubble, affordResultBubbleProps{ID: t.ID, HTML: t.Text, OnDelete: deleteTurn})
				}
				return ui.CreateElement(AssistantBubble, asstBubbleProps{ID: t.ID, Text: t.Text, Usage: t.Usage, Model: model, OnPin: pinText, OnDelete: deleteTurn, OnRetry: retryFor(t.ID)})
			},
		),
		If(loading.Get(), Div(css.Class(tw.Flex, tw.JustifyStart),
			Div(css.Class("chat-row-agent"),
				Div(css.Class("chat-avatar"), Attr("aria-hidden", "true"), "✦"),
				Div(css.Class("insights-thinking chat-thinking", tw.Text13, tw.TextFaint), uistate.T("insights.thinking")),
			),
		)),
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
			}()),
			Value(input.Get()), OnInput(onInput)),
		trailing,
	)
	composer := inputRow
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
	// Tapping a chip FILLS the Ask box (doesn't send) so the user can review/edit first.
	chips := Fragment()
	if len(starters) > 0 && input.Get() == "" && empty {
		chips = Div(css.Class(tw.Mb2),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2),
				MapKeyed(starters,
					func(q string) any { return q },
					func(q string) ui.Node {
						return ui.CreateElement(suggestChip, suggestChipProps{Q: q, OnPick: fillAsk})
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
	chatControls := Div(css.Class("ask-head-actions", tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
		modelPicker(modelPickerProps{Models: modelList.Get(), Current: model, OnPick: pickModel}),
		If(thinkingApplies, thinkPicker(thinkPickerProps{Effort: effortSel.Get(), OnPick: pickEffort})),
		privacyChip(privacyChipProps{Tier: tier, OnToggle: func() {
			next := aicontext.TierAggregatesOnly
			if tier == aicontext.TierAggregatesOnly {
				next = aicontext.TierFull
			}
			privacyTier.Set(next)
			uistate.PersistDefaultPrivacyTier(next) // remember as the default for new chats (AG17)
		}}),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-new-chat"), OnClick(newChatEvt),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("insights.newChat"))),
		Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "assistant-edit-prompt"),
			Title(uistate.T("insights.editPrompt")), OnClick(openPrompt),
			uiw.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W35, tw.H35)), Span(uistate.T("insights.editPrompt"))),
	)
	// Bespoke aside group: the saved conversations as a quiet vertical index.
	railConvs := Fragment()
	if len(convs) > 0 {
		railConvs = collapsibleNote(collapsibleNoteProps{
			Label:  uistate.T("assistant.conversations"),
			TestID: "assistant-note-convs",
			Count:  len(convs),
			Body: Fragment(
				Div(css.Class("asst-convs"), Attr("data-testid", "assistant-convs"),
					MapKeyed(convs,
						func(c domain.Conversation) any { return c.ID },
						func(c domain.Conversation) ui.Node {
							return ui.CreateElement(ConversationPill, convPillProps{C: c, Active: c.ID == convID.Get(), OnPick: switchTo, OnDelete: deleteConv})
						},
					),
				),
				P(css.Class("ask-note-hint"), uistate.T("assistant.railHint")),
			),
		})
	}

	// Backend/OpenAI mode toggle — only meaningful when a backend is configured;
	// otherwise the chat always uses the direct OpenAI provider.
	backendConfigured := strings.TrimSpace(pr.ServerURL) != "" && strings.TrimSpace(pr.ServerToken) != ""
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
	if pa := pendingApproval.Get(); pa != nil {
		approvalPreview = pa.preview
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
		If(noAI, Div(css.Class("asst-key-callout"), Attr("data-testid", "assistant-key-callout"),
			Span(uistate.T("assistant.keyCallout")),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), OnClick(func() { uistate.OpenGlobalSettingsAt("ai") }), uistate.T("nav.settings")),
		)),
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
	heroBlock := Div(css.Class("asst-hero"),
		agentIntro,
		chips,
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
				P(css.Class("chat-dock-hint", tw.TextFaint), uistate.T("assistant.composerHint")),
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
		Div(css.Class("ask-head-id"),
			Span(ClassStr(statusCls), Attr("aria-hidden", "true")),
			H2(css.Class("ask-title"), uistate.T("assistant.agentTitle")),
			Span(css.Class("ask-status"), uistate.T(statusKey)),
		),
		chatControls,
	)
	askMain := Div(css.Class("ask-main"),
		askHead,
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
			// spending highlights, pins, saved conversations.
			Div(css.Class("ask-aside"), Attr("data-testid", "assistant-rail"), flagged, highlights, pinnedCard, railConvs),
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
}

// ConversationPill is one chat in the switcher: tap the title to open it, the × to
// delete it. Its own component so the pick/delete hooks stay stable across the list.
func ConversationPill(p convPillProps) ui.Node {
	pick := ui.UseEvent(Prevent(func() { p.OnPick(p.C.ID) }))
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.C.ID) }))
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
	return Div(ClassStr(cls),
		Button(css.Class(tw.MaxW160, tw.Truncate, tw.TextLeft), Type("button"), OnClick(pick), title),
		Button(css.Class(tw.TextFaint, tw.Opacity60, tw.HoverOpacity100), Type("button"), Title(uistate.T("insights.deleteChat")), Attr("aria-label", uistate.T("insights.deleteChat")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W3, tw.H3))),
	)
}

// chatTurn is one message in the Insights conversation.
type chatTurn struct {
	ID    string
	Role  string // "user" | "assistant"
	Text  string
	Usage ai.Usage
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
}

// UserBubble renders one user message with its actions (Retry on the latest, Delete)
// in a row UNDER the bubble. Its own component so the action hooks stay stable across
// the list (no hooks in loops).
func UserBubble(p userBubbleProps) ui.Node {
	del := ui.UseEvent(Prevent(func() { p.OnDelete(p.ID) }))
	retryEvt := ui.UseEvent(Prevent(func() {
		if p.OnRetry != nil {
			p.OnRetry()
		}
	}))
	actBtn := tw.Fold(tw.TextFaint, tw.Opacity70, tw.HoverOpacity100, tw.InlineFlex, tw.ItemsCenter)
	return Div(css.Class("group", tw.Flex, tw.FlexCol, tw.ItemsEnd),
		Div(css.Class("asst-msg-user", tw.MaxW85, tw.Text14, tw.WhitespacePreWrap), p.Text),
		Div(css.Class(tw.Flex, tw.Gap3, tw.ItemsCenter, tw.Mt1, tw.Px1, tw.Opacity0, tw.GroupHoverOpacity100, tw.GroupFocusWithinOpacity100, tw.MotionSafeTransitionOpacity),
			If(p.OnRetry != nil, Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, css.Class(tw.W4, tw.H4)))),
			Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
}

type asstBubbleProps struct {
	ID       string
	Text     string
	Usage    ai.Usage
	Model    string
	OnPin    func(string) bool
	OnDelete func(string)
	OnRetry  func() // non-nil only on the latest assistant turn
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
	// Render the Markdown after mount and whenever the text changes (streaming-ready).
	// The signature also folds in the local action toggles so the effect re-fills the
	// innerHTML after a self re-render (pin/copy) that the vdom would otherwise clear.
	sig := p.Text
	if pinned.Get() {
		sig += "|p"
	}
	if copied.Get() {
		sig += "|c"
	}
	ui.UseEffect(func() func() { renderMarkdown(mdID, p.Text); return nil }, sig)
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
	var note ui.Node = Fragment()
	if p.Usage.TotalTokens > 0 {
		txt := uistate.T("insights.usageTokens", p.Usage.TotalTokens)
		if cost, ok := ai.EstimateCostUSD(p.Model, p.Usage); ok {
			txt = uistate.T("insights.usageCost", p.Usage.TotalTokens, ai.FormatCostUSD(cost))
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
		Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap3, tw.ItemsCenter, tw.Mt1, tw.Px1, tw.Opacity0, tw.GroupHoverOpacity100, tw.GroupFocusWithinOpacity100, tw.MotionSafeTransitionOpacity),
			IfElse(copied.Get(),
				Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("insights.copied")),
				Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.copy")), Attr("aria-label", uistate.T("insights.copy")), OnClick(copyEvt), uiw.Icon(icon.Copy, css.Class(tw.W4, tw.H4))),
			),
			IfElse(pinned.Get(),
				Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("insights.pinnedConfirm")),
				Button(ClassStr(actBtn+" "+tw.Fold(tw.Gap1, tw.Text12)), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pin), uistate.T("insights.pin")),
			),
			If(p.OnRetry != nil, Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, css.Class(tw.W4, tw.H4)))),
			Button(ClassStr(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
		note,
	)
}

// renderMarkdown sets the element's sanitized, Markdown-rendered HTML using the
// vendored marked + DOMPurify globals; falls back to the raw text when absent.
func renderMarkdown(elemID, mdText string) {
	doc := js.Global().Get("document")
	el := doc.Call("getElementById", elemID)
	if !el.Truthy() {
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
}

// reasoningModel reports whether a model id is an OpenAI reasoning model (o-series
// or gpt-5.x), which reject a custom temperature on /chat/completions.
func reasoningModel(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4") || strings.HasPrefix(m, "gpt-5")
}

// assistantModelIDs returns the ids for the header model picker: the live list from
// OpenAI when available, else the built-in defaults, always including the current
// selection so a custom/older model stays visible even if it's not in the list.
func assistantModelIDs(models []string, cur string) []string {
	ids := models
	if len(ids) == 0 {
		ids = []string{"gpt-5.4-mini", "gpt-5.5", "o4-mini"}
	}
	cur = strings.TrimSpace(cur)
	if cur != "" {
		for _, m := range ids {
			if m == cur {
				return ids
			}
		}
		ids = append([]string{cur}, ids...)
	}
	return ids
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
	return Label(css.Class("ask-quickctl"), Title(uistate.T("assistant.modelPick")),
		Span(css.Class("ask-quickctl-lbl"), uistate.T("assistant.modelLabel")),
		Select(css.Class("ask-quickctl-sel"), Attr("aria-label", uistate.T("assistant.modelPick")), Attr("data-testid", "assistant-model"), OnChange(onChange),
			MapKeyed(assistantModelIDs(p.Models, p.Current),
				func(m string) any { return m },
				func(m string) ui.Node { return Option(Value(m), SelectedIf(m == p.Current), m) },
			),
		),
	)
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
	cls := "ask-quickctl asst-privacy-chip"
	if agg {
		cls += " is-aggregates"
	}
	return Button(css.Class(cls), Type("button"), Attr("data-testid", "assistant-privacy-chip"),
		Attr("role", "status"), Attr("aria-live", "polite"),
		Attr("aria-label", uistate.T("insights.privacyAria", label)), Title(title), OnClick(onClick),
		uiw.Icon(icon.Lock, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(css.Class("ask-quickctl-lbl"), uistate.T("insights.privacyLabel")),
		Span(label),
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
	return Label(css.Class("ask-quickctl"), Title(uistate.T("assistant.thinkPick")),
		Span(css.Class("ask-quickctl-lbl"), uistate.T("assistant.thinkLabel")),
		Select(css.Class("ask-quickctl-sel"), Attr("aria-label", uistate.T("assistant.thinkPick")), Attr("data-testid", "assistant-think"), OnChange(onChange),
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
	// Render the Markdown after mount / when expanded toggles (the vdom would clear
	// the innerHTML on a self re-render otherwise).
	sig := mdID
	if expanded.Get() {
		sig += "|x"
	}
	ui.UseEffect(func() func() { renderMarkdown(mdID, p.Text); return nil }, sig)

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
	OnApprove func()
	OnDecline func()
}

// ApprovalCard asks the user to approve or decline a pending mutating tool. Its own
// component so its action hooks re-attach cleanly each time it mounts.
func ApprovalCard(p approvalCardProps) ui.Node {
	approve := ui.UseEvent(Prevent(func() { p.OnApprove() }))
	decline := ui.UseEvent(Prevent(func() { p.OnDecline() }))
	return Div(css.Class(tw.RoundedXl, tw.Border, tw.BorderAmber50, tw.BgAmber10, tw.Px35, tw.Py25, tw.Mb2, tw.Text13),
		P(css.Class(tw.FontSemibold), uistate.T("insights.approveTitle")),
		P(css.Class(tw.Mt1), p.Preview),
		Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
			Button(css.Class("btn btn-primary"), Type("button"), OnClick(approve), uistate.T("insights.approve")),
			Button(css.Class("btn"), Type("button"), OnClick(decline), uistate.T("insights.decline")),
		),
	)
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
			Span(css.Class(tw.Text14, tw.FontMedium, tw.Truncate), p.Insight.Title),
			Span(css.Class("muted", tw.Text13, tw.Truncate), p.Insight.Detail),
			Div(css.Class("insight-row-actions"),
				Button(css.Class("insight-row-btn"), Type("button"),
					Attr("data-testid", "flag-source"), Attr("aria-label", uistate.T("assistant.flagSourceAria")),
					Title(uistate.T("assistant.flagSourceAria")), OnClick(navigate),
					uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
					Span(uistate.T("assistant.flagSource")),
				),
				// "Discuss" only where there's a chat to drop the context into (the Ask
				// tab); the Insights data panel reuses this row without a chat, so it
				// passes no OnDiscuss and the button is omitted.
				If(p.OnDiscuss != nil, Button(css.Class("insight-row-btn"), Type("button"),
					Attr("data-testid", "flag-discuss"), Attr("aria-label", uistate.T("assistant.flagDiscussAria")),
					Title(uistate.T("assistant.flagDiscussAria")), OnClick(discuss),
					uiw.Icon(icon.MessageCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
					Span(uistate.T("assistant.flagDiscuss")),
				)),
			),
		),
	)
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
	OnDrill func(catName string)
}

// insightsHighlightRow renders a single clickable spending-highlight row. It is
// a standalone component so its OnClick hook is registered at a stable render
// position — not inside the variable-length anomaly loop in spendingHighlights
// (CRITICAL: never call On* helpers inside a variable-length loop).
func insightsHighlightRow(props insightsHighlightRowProps) ui.Node {
	a := props.Anomaly
	drill := ui.UseEvent(func() { props.OnDrill(a.Category) })
	return Button(
		css.Class("insight-row insight-row--clickable"),
		Type("button"),
		Attr("aria-label", uistate.T("insights.highlightDrillAria", a.Category)),
		OnClick(drill),
		Span(ClassStr("insight-dot "+highlightTone(a)), uiw.Icon(highlightArrow(a), css.Class(tw.W4, tw.H4))),
		Span(highlightText(a, props.Base)),
	)
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
func spendingHighlights(anomalies []insights.Anomaly, base string, onDrill func(catName string)) ui.Node {
	if len(anomalies) == 0 {
		return Fragment()
	}

	rows := MapKeyed(anomalies,
		func(a insights.Anomaly) any { return a.Category },
		func(a insights.Anomaly) ui.Node {
			return ui.CreateElement(insightsHighlightRow, insightsHighlightRowProps{
				Anomaly: a,
				Base:    base,
				OnDrill: onDrill,
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
	txLabel := uistate.T("insights.merchantTxCount", m.Count)
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
	ui.UseEffect(func() func() { renderMarkdown(mdID, p.HTML); return nil }, p.HTML)
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
