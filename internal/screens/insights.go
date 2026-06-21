//go:build js && wasm

package screens

import (
	"sort"
	"strings"
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

	// saveTask writes an answer to the To-do list; the full answer always lives in the
	// notes, never the title (C27). pinText saves it to the pinned-insights list.
	saveTask := func(text string) bool {
		text = strings.TrimSpace(text)
		if text == "" {
			return false
		}
		t := domain.Task{
			ID: id.New(), Title: uistate.T("insights.aiTaskTitle"), Notes: text,
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceAI,
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return false
		}
		return true
	}
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

	// buildMessages turns the conversation so far into an OpenAI message list, led by
	// the assistant's instructions and the bounded financial context (aggregates only;
	// richer detail comes from gated tools, C82 wiring).
	buildMessages := func(hist []chatTurn) []ai.Message {
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant for CashFlux. Answer from the provided context; if it isn't enough to answer precisely, say what's missing. Plain English, no jargon."},
			{Role: ai.RoleSystem, Content: "Context — " + aiCtx.Line()},
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

	// sendText posts a user turn and routes the assistant's reply into a new turn.
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
		errMsg.Set("")
		loading.Set(true)
		messages := buildMessages(hist)
		onResult := func(content string, u ai.Usage) {
			loading.Set(false)
			turns.Update(func(cur []chatTurn) []chatTurn {
				return append(cur, chatTurn{ID: id.New(), Role: "assistant", Text: content, Usage: u})
			})
		}
		onErr := func(e string) { loading.Set(false); errMsg.Set(e) }
		if useBackendAI {
			cancelFn.Set(ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0.4, onResult, onErr))
		} else {
			cancelFn.Set(ai.SendChat(key, ai.DefaultBaseURL, model, messages, 0.4, onResult, onErr))
		}
	}
	onSubmit := ui.UseEvent(Prevent(func() { sendText(input.Get()) }))

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

	// The conversation thread: user bubbles right, assistant bubbles (their own
	// component, owning the save/pin hooks per the no-hooks-in-loops rule) left.
	thread := Div(Class("flex flex-col gap-3 mb-3"),
		MapKeyed(convo,
			func(t chatTurn) any { return t.ID },
			func(t chatTurn) ui.Node {
				if t.Role == "user" {
					return Div(Class("flex justify-end"),
						Div(Class("max-w-[85%] rounded-2xl bg-sky-500/10 px-3.5 py-2 text-[14px] whitespace-pre-wrap"), t.Text),
					)
				}
				return ui.CreateElement(AssistantBubble, asstBubbleProps{Text: t.Text, Usage: t.Usage, Model: model, OnSave: saveTask, OnPin: pinText})
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

	return Div(
		highlights,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.chatTitle")),
			If(empty, P(Class("muted"), uistate.T("insights.chatHint"))),
			If(!empty, thread),
			chips,
			composer,
			If(errMsg.Get() != "", P(Class("err"), Attr("role", "alert"), errMsg.Get())),
		),
		pinnedCard,
	)
}

// chatTurn is one message in the Insights conversation.
type chatTurn struct {
	ID    string
	Role  string // "user" | "assistant"
	Text  string
	Usage ai.Usage
}

type asstBubbleProps struct {
	Text   string
	Usage  ai.Usage
	Model  string
	OnSave func(string) bool
	OnPin  func(string) bool
}

// AssistantBubble renders one assistant message as Markdown with per-message
// Save-as-task / Pin actions and a token/cost note. The model emits Markdown,
// rendered as rich text; the framework's Markdown is GFM-aware and drops active
// URL schemes (javascript:/data:), so model text can't smuggle an executable
// href; links open safely in a new tab. Its own component so the action hooks
// stay stable across the message list (no hooks in loops).
func AssistantBubble(p asstBubbleProps) ui.Node {
	saved := ui.UseState(false)
	pinned := ui.UseState(false)
	save := ui.UseEvent(Prevent(func() {
		if p.OnSave(p.Text) {
			saved.Set(true)
		}
	}))
	pin := ui.UseEvent(Prevent(func() {
		if p.OnPin(p.Text) {
			pinned.Set(true)
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
	return Div(Class("flex justify-start"),
		Div(Class("max-w-[85%] rounded-2xl bg-black/[0.04] px-3.5 py-2.5"),
			Div(Class("md insights-answer text-[14px]"), Markdown(p.Text, MarkdownRenderOptions{LinkTarget: "_blank", LinkRel: "noopener noreferrer"})),
			Div(Class("flex flex-wrap gap-2 items-center mt-2"),
				IfElse(saved.Get(),
					Span(Class("muted text-[12px]"), uistate.T("insights.savedToTodo")),
					Button(Class("btn"), Type("button"), Title(uistate.T("insights.saveTaskTitle")), OnClick(save), uistate.T("insights.saveTask")),
				),
				IfElse(pinned.Get(),
					Span(Class("muted text-[12px]"), uistate.T("insights.pinnedConfirm")),
					Button(Class("btn"), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pin), uistate.T("insights.pin")),
				),
			),
			note,
		),
	)
}

type pinnedInsightRowProps struct {
	Insight  domain.SavedInsight
	OnDelete func(string)
}

// PinnedInsightRow renders one pinned insight with its date and a remove button.
// Long insights are clamped to two lines with a Show more/less toggle so the list
// stays compact (C59). It owns its own click handlers (per the no-hooks-in-loops
// rule).
func PinnedInsightRow(props pinnedInsightRowProps) ui.Node {
	p := props.Insight
	expanded := ui.UseState(false)
	del := ui.UseEvent(Prevent(func() { props.OnDelete(p.ID) }))
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	long := len([]rune(p.Text)) > 140
	descClass := "row-desc"
	if long && !expanded.Get() {
		descClass += " line-clamp-2"
	}
	moreLabel := uistate.T("insights.showMore")
	if expanded.Get() {
		moreLabel = uistate.T("insights.showLess")
	}
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class(descClass), p.Text),
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
