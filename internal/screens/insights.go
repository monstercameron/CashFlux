//go:build js && wasm

package screens

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Insights is AI analysis (OpenAI, client-side, bring-your-own-key): an
// "Explain my month" narrative generated from the user's live figures.
func Insights() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	settings := app.Settings()
	key := settings.OpenAIKey
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := pr.BackendActive()
	model := settings.OpenAIModel
	if model == "" {
		model = "gpt-4o-mini"
	}
	// Reasoning models (o-series, gpt-5.x) reject a non-default temperature on
	// /chat/completions, so omit it (0 is dropped by omitempty) for them; other
	// models get a mild 0.4. This keeps the chat working whatever model is picked.
	chatTemp := 0.4
	if reasoningModel(model) {
		chatTemp = 0
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: settings.FXRates}

	accounts := app.Accounts()
	txns := app.Transactions()
	net, _, _, _ := ledger.NetWorth(accounts, txns, rates)
	mStart, mEnd := dateutil.MonthRange(time.Now())
	income, expense, _ := ledger.PeriodTotals(txns, mStart, mEnd, rates)
	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}
	// The only financial data sent to the model: aggregates, no PII (see ai.FinancialContext).
	aiCtx := ai.FinancialContext{NetWorth: fmtMoney(net), Income: fmtMoney(income), Spending: fmtMoney(expense), Accounts: active}

	// Starter questions for the Ask box (L8): tailored to this month's top spend
	// category so a blank box never stalls the user.
	topCatSpend := map[string]int64{}
	for _, t := range txns {
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
	starters := insights.SuggestedQuestions(insights.QuestionContext{TopCategory: topCat})

	nav := router.UseNavigate()
	// The no-key hint is a clear call to action that hops to Settings (where the AI
	// key lives), not a dead-end sentence (C59; same fix as C54). Built fresh per
	// use so the two placements get independent button nodes.
	keyHintNode := func() ui.Node {
		return Div(
			P(Class("muted"), uistate.T("insights.keyHint")),
			Button(Class("btn"), Type("button"), OnClick(func() { nav.Navigate(uistate.RoutePath("/settings")) }), uistate.T("nav.settings")),
		)
	}

	// The Insights screen is a chat with the CashFlux assistant (C82 wiring): a
	// conversation thread the user types into, answered from their own figures.
	turns := ui.UseState([]chatTurn{})
	input := ui.UseState("")
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
		ctx += " For any specific number (a category total, an account balance, affordability), CALL A TOOL — never guess or say you lack the data."
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: persona},
			{Role: ai.RoleSystem, Content: ctx},
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
		return ai.SendChatTools(key, ai.DefaultBaseURL, model, messages, chatTemp, tools, onResult, onErr)
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
		tools := buildChatTools(app, base, rates)
		specs := make([]ai.Tool, len(tools))
		handlers := make(map[string]func(json.RawMessage) string, len(tools))
		for i, t := range tools {
			specs[i] = t.spec
			handlers[t.spec.Function.Name] = t.run
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
					return
				}
				msgs = append(msgs, r.msg)
				for _, tc := range r.msg.ToolCalls {
					out := "tool unavailable"
					if h := handlers[tc.Function.Name]; h != nil {
						out = h(json.RawMessage(tc.Function.Arguments))
					}
					msgs = append(msgs, ai.ToolResultMessage(tc.ID, tc.Function.Name, out))
				}
			}
			loading.Set(false)
			errMsg.Set(uistate.T("insights.tooManySteps"))
		}()
	}

	// sendText posts a user turn, then runs the model on the new history.
	sendText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" || loading.Get() {
			return
		}
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		hist := append(append([]chatTurn{}, turns.Get()...), chatTurn{ID: id.New(), Role: "user", Text: text})
		turns.Set(hist)
		input.Set("")
		run(hist)
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
		_ = app.PutConversation(domain.Conversation{ID: cid, Title: conversationTitle(ts), Messages: msgs, CreatedAt: created, UpdatedAt: time.Now()})
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
		return nil
	}, "cf-insights-init")

	// Auto-scroll the thread to the bottom whenever a message is added or the
	// "thinking" indicator toggles, so a freshly spawned bubble stays in view.
	scrollSig := strconv.Itoa(len(turns.Get()))
	if loading.Get() {
		scrollSig += "|L"
	}
	ui.UseEffect(func() func() { scrollChatToEnd(); return nil }, scrollSig)

	onSubmit := ui.UseEvent(Prevent(func() { sendText(input.Get()) }))
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

	highlights := spendingHighlights(txns, app.Categories(), base, rates)

	// Pinned insights, newest first.
	pins := app.SavedInsights()
	sort.Slice(pins, func(i, j int) bool { return pins[i].CreatedAt.After(pins[j].CreatedAt) })
	pinnedCard := Fragment()
	if len(pins) > 0 {
		pinnedCard = Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.pinnedTitle")),
			Div(Class("rows"), MapKeyed(pins,
				func(p domain.SavedInsight) any { return p.ID },
				func(p domain.SavedInsight) ui.Node {
					return ui.CreateElement(PinnedInsightRow, pinnedInsightRowProps{Insight: p, OnDelete: deletePinned})
				},
			)),
		)
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

	// The conversation thread scrolls inside a bounded region so the composer below it
	// stays on screen no matter how long the conversation grows (the thread scrolls,
	// the input doesn't move). Auto-scroll keeps the newest message in view.
	thread := Div(Attr("id", "cf-chat-thread"), Class("flex flex-col gap-3 mb-3 overflow-y-auto max-h-[55vh] pr-1"),
		MapKeyed(convo,
			func(t chatTurn) any { return t.ID },
			func(t chatTurn) ui.Node {
				if t.Role == "user" {
					return ui.CreateElement(UserBubble, userBubbleProps{ID: t.ID, Text: t.Text, OnDelete: deleteTurn, OnRetry: retryFor(t.ID)})
				}
				return ui.CreateElement(AssistantBubble, asstBubbleProps{ID: t.ID, Text: t.Text, Usage: t.Usage, Model: model, OnPin: pinText, OnDelete: deleteTurn, OnRetry: retryFor(t.ID)})
			},
		),
		If(loading.Get(), Div(Class("flex justify-start"),
			Div(Class("max-w-[85%] rounded-2xl bg-black/[0.04] px-3.5 py-2 text-[13px] text-faint"), uistate.T("insights.thinking")),
		)),
	)

	// Composer: the input row, or the key call-to-action when no key is set.
	var composer ui.Node
	if key == "" && !useBackendAI {
		composer = keyHintNode()
	} else {
		composer = Form(Class("mt-1"), OnSubmit(onSubmit),
			Div(Class("flex gap-2 items-center"),
				Input(Class("field field-wide"), Type("text"), Attr("aria-label", uistate.T("insights.askPlaceholder")), Placeholder(uistate.T("insights.askPlaceholder")), Value(input.Get()), OnInput(onInput)),
				IfElse(loading.Get(),
					Button(Class("btn"), Type("button"), OnClick(cancelAI), uistate.T("insights.cancel")),
					Button(Class("btn btn-primary inline-flex items-center gap-1.5"), Type("submit"), uiw.Icon(icon.Sparkles, Class("w-4 h-4 shrink-0")), Span(uistate.T("insights.send"))),
				),
			),
		)
	}

	// Starter chips seed an empty thread; tapping one sends it (L8).
	chips := Fragment()
	if empty && (key != "" || useBackendAI) && len(starters) > 0 {
		chips = Div(Class("flex flex-wrap gap-2 mb-2"),
			MapKeyed(starters,
				func(q string) any { return q },
				func(q string) ui.Node {
					return ui.CreateElement(suggestChip, suggestChipProps{Q: q, OnPick: sendText})
				},
			),
		)
	}

	// Conversation switcher: a "New chat" button plus a pill per saved chat (the
	// switcher row is always present so its New-chat hook stays at a stable position).
	convs := app.Conversations()
	sort.Slice(convs, func(i, j int) bool { return convs[i].UpdatedAt.After(convs[j].UpdatedAt) })
	pill := "inline-flex items-center gap-1 rounded-full px-3 py-1 text-[12px] border border-black/10 hover:bg-black/[0.03]"
	switcher := Div(Class("flex flex-wrap gap-2 mb-3 items-center"),
		Button(Class(pill), Type("button"), OnClick(newChatEvt), uiw.Icon(icon.PlusCircle, Class("w-3.5 h-3.5")), Span(uistate.T("insights.newChat"))),
		Button(Class(pill), Type("button"), Title(uistate.T("insights.editPrompt")), OnClick(openPrompt), uiw.Icon(icon.Settings, Class("w-3.5 h-3.5")), Span(uistate.T("insights.editPrompt"))),
		MapKeyed(convs,
			func(c domain.Conversation) any { return c.ID },
			func(c domain.Conversation) ui.Node {
				return ui.CreateElement(ConversationPill, convPillProps{C: c, Active: c.ID == convID.Get(), OnPick: switchTo, OnDelete: deleteConv})
			},
		),
	)

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
		backendToggle = Div(Class("flex items-center gap-2 mb-2 text-[12px] text-faint"),
			Span(label),
			Button(Class("underline hover:opacity-100"), Type("button"), OnClick(toggleBackend), action),
		)
	}

	return Div(
		highlights,
		// Pinned insights sit ABOVE the chat as quick references, so the conversation
		// thread below has room to grow.
		pinnedCard,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.chatTitle")),
			switcher,
			backendToggle,
			If(empty, P(Class("muted"), uistate.T("insights.chatHint"))),
			If(!empty, thread),
			chips,
			composer,
			If(errMsg.Get() != "", P(Class("err"), Attr("role", "alert"), errMsg.Get())),
		),
		// The editable system-prompt overlay (persona only; live data + tools are always
		// injected automatically by buildMessages).
		If(promptOpen.Get(), uiw.FlipPanel(uiw.FlipPanelProps{
			Title:   uistate.T("insights.promptTitle"),
			Width:   "640px",
			Height:  "520px",
			OnSave:  savePrompt,
			OnClose: closePrompt,
			Back: Div(Class("flex flex-col gap-2"),
				P(Class("muted text-[13px]"), uistate.T("insights.promptHint")),
				Textarea(Class("field field-wide"), Attr("rows", "12"), Attr("aria-label", uistate.T("insights.promptTitle")), OnInput(onPromptInput), promptDraft.Get()),
				Button(Class("btn self-start"), Type("button"), OnClick(resetPrompt), uistate.T("insights.promptReset")),
			),
		})),
	)
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
	cls := "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[12px] border "
	if p.Active {
		cls += "bg-sky-500/15 border-sky-500/40"
	} else {
		cls += "border-black/10 hover:bg-black/[0.03]"
	}
	title := strings.TrimSpace(p.C.Title)
	if title == "" {
		title = "Untitled chat"
	}
	return Div(Class(cls),
		Button(Class("max-w-[160px] truncate text-left"), Type("button"), OnClick(pick), title),
		Button(Class("text-faint opacity-60 hover:opacity-100"), Type("button"), Title(uistate.T("insights.deleteChat")), Attr("aria-label", uistate.T("insights.deleteChat")), OnClick(del), uiw.Icon(icon.Close, Class("w-3 h-3"))),
	)
}

// chatTurn is one message in the Insights conversation.
type chatTurn struct {
	ID    string
	Role  string // "user" | "assistant"
	Text  string
	Usage ai.Usage
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
	actBtn := "text-faint opacity-70 hover:opacity-100 inline-flex items-center"
	return Div(Class("flex flex-col items-end group"),
		Div(Class("max-w-[85%] rounded-2xl bg-sky-500/10 px-3.5 py-2 text-[14px] whitespace-pre-wrap"), p.Text),
		Div(Class("flex gap-3 items-center mt-1 px-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 motion-safe:transition-opacity"),
			If(p.OnRetry != nil, Button(Class(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, Class("w-4 h-4")))),
			Button(Class(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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
		note = P(Class("text-faint text-[11px] mt-2"), txt)
	}
	actBtn := "text-faint opacity-70 hover:opacity-100 inline-flex items-center"
	return Div(Class("flex flex-col items-start group"),
		Div(Class("max-w-[85%] rounded-2xl bg-black/[0.04] px-3.5 py-2.5"),
			// marked fills this element via the effect above.
			Div(Attr("id", mdID), Class("md insights-answer text-[14px]")),
		),
		// Actions sit UNDER the bubble, revealed when the bubble is hovered/focused.
		Div(Class("flex flex-wrap gap-3 items-center mt-1 px-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 motion-safe:transition-opacity"),
			IfElse(copied.Get(),
				Span(Class("text-faint text-[12px]"), uistate.T("insights.copied")),
				Button(Class(actBtn), Type("button"), Title(uistate.T("insights.copy")), Attr("aria-label", uistate.T("insights.copy")), OnClick(copyEvt), uiw.Icon(icon.Copy, Class("w-4 h-4"))),
			),
			IfElse(pinned.Get(),
				Span(Class("text-faint text-[12px]"), uistate.T("insights.pinnedConfirm")),
				Button(Class(actBtn+" gap-1 text-[12px]"), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pin), uistate.T("insights.pin")),
			),
			If(p.OnRetry != nil, Button(Class(actBtn), Type("button"), Title(uistate.T("insights.retry")), Attr("aria-label", uistate.T("insights.retry")), OnClick(retryEvt), uiw.Icon(icon.Refresh, Class("w-4 h-4")))),
			Button(Class(actBtn), Type("button"), Title(uistate.T("insights.deleteMsg")), Attr("aria-label", uistate.T("insights.deleteMsg")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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

// scrollChatToEnd scrolls the bounded thread container to its bottom (only the
// container, never the page), so the latest message stays in view. The scroll is
// deferred via setTimeout so it runs AFTER the bubbles' Markdown innerHTML has been
// filled (each bubble renders in its own effect, growing scrollHeight) — otherwise
// an on-load resume would scroll a still-empty container and land at the top.
func scrollChatToEnd() {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		cb.Release()
		el := js.Global().Get("document").Call("getElementById", "cf-chat-thread")
		if el.Truthy() {
			el.Set("scrollTop", el.Get("scrollHeight"))
		}
		return nil
	})
	js.Global().Call("setTimeout", cb, 80)
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
	descClass := "insights-answer text-[13.5px]"
	if long && !expanded.Get() {
		descClass += " line-clamp-3"
	}
	moreLabel := uistate.T("insights.showMore")
	if expanded.Get() {
		moreLabel = uistate.T("insights.showLess")
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Div(Attr("id", mdID), Class(descClass)),
			If(long, Button(Class("btn-link text-[11px] mt-1 self-start"), Type("button"), OnClick(toggle), moreLabel)),
			Span(Class("row-meta"), p.CreatedAt.Format("Jan 2, 2006")),
		),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("insights.unpinTitle")), Title(uistate.T("insights.unpinTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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
	return Button(Class("btn chip-suggest"), Type("button"), OnClick(func() { onPick(q) }), q)
}

// spendingHighlights renders an offline "what changed" card: it detects
// categories whose spend this month deviates materially from their recent
// average and explains each in plain English. It needs no AI key. Returns an
// empty node when there's nothing notable, so the card simply doesn't appear.
// The card is non-interactive, so its rows are safe to render in a loop.
func spendingHighlights(txns []domain.Transaction, categories []domain.Category, base string, rates currency.Rates) ui.Node {
	anomalies := detectSpendingAnomalies(txns, categories, rates)
	if len(anomalies) == 0 {
		return Fragment()
	}

	rows := make([]ui.Node, 0, len(anomalies))
	for _, a := range anomalies {
		rows = append(rows, P(Class("insight-row"),
			Span(Class("insight-dot "+highlightTone(a)), uiw.Icon(highlightArrow(a), Class("w-4 h-4"))),
			Span(highlightText(a, base)),
		))
	}

	return Section(Class("card"),
		H2(Class("card-title"), uistate.T("insights.highlightsTitle")),
		P(Class("muted"), uistate.T("insights.highlightsHint")),
		Div(Class("insight-list"), rows),
	)
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
	return insights.Detect(series, insights.DefaultOptions())
}

// highlightText is the plain-English sentence for one spending anomaly.
func highlightText(a insights.Anomaly, base string) string {
	pct := a.PctChange
	if pct < 0 {
		pct = -pct
	}
	current := fmtMoney(money.New(a.Current, base))
	baseline := fmtMoney(money.New(a.Baseline, base))
	key := "insights.highlightDown"
	if a.Direction == insights.Up {
		key = "insights.highlightUp"
	}
	return uistate.T(key, a.Category, pct, current, baseline)
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
