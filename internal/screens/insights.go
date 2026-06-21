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

	// Explain and Q&A keep SEPARATE result slots so running one never wipes the
	// other (C59): you can hold the monthly narrative while also asking a question.
	explainRes := ui.UseState("")
	qaRes := ui.UseState("")
	explainUsage := ui.UseState(ai.Usage{})
	qaUsage := ui.UseState(ai.Usage{})
	// loading holds which action is in flight ("", "explain", or "qa") so only that
	// card shows the busy/cancel state.
	loading := ui.UseState("")
	errMsg := ui.UseState("")
	savedE := ui.UseState("")
	pinnedE := ui.UseState("")
	savedQ := ui.UseState("")
	pinnedQ := ui.UseState("")
	rev := ui.UseState(0)
	bump := func() { rev.Set(rev.Get() + 1) }
	var noCancel func()
	cancelFn := ui.UseState(noCancel)
	cancelAI := ui.UseEvent(func() {
		if c := cancelFn.Get(); c != nil {
			c()
		}
		loading.Set("")
	})
	question := ui.UseState("")
	onQuestion := ui.UseEvent(func(v string) { question.Set(v) })

	// saveTask writes an answer to the To-do list; the full answer always lives in
	// the notes, never the title (C27).
	saveTask := func(text, title string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		title = strings.TrimSpace(title)
		if title == "" {
			title = uistate.T("insights.aiTaskTitle")
		}
		if rs := []rune(title); len(rs) > 80 { // rune-safe truncation for the title
			title = strings.TrimSpace(string(rs[:80])) + "…"
		}
		t := domain.Task{
			ID: id.New(), Title: title, Notes: text,
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceAI,
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
		}
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

	// Per-slot save/pin handlers — each acts only on its own answer (C59).
	saveE := ui.UseEvent(Prevent(func() {
		if r := explainRes.Get(); strings.TrimSpace(r) != "" {
			saveTask(r, uistate.T("insights.aiTaskTitle"))
			savedE.Set(uistate.T("insights.savedToTodo"))
		}
	}))
	pinE := ui.UseEvent(Prevent(func() {
		if pinText(explainRes.Get()) {
			pinnedE.Set(uistate.T("insights.pinnedConfirm"))
		}
	}))
	saveQ := ui.UseEvent(Prevent(func() {
		if r := qaRes.Get(); strings.TrimSpace(r) != "" {
			saveTask(r, question.Get())
			savedQ.Set(uistate.T("insights.savedToTodo"))
		}
	}))
	pinQ := ui.UseEvent(Prevent(func() {
		if pinText(qaRes.Get()) {
			pinnedQ.Set(uistate.T("insights.pinnedConfirm"))
		}
	}))

	// send runs one chat call and routes the reply into the given slot, leaving the
	// other slot untouched.
	send := func(messages []ai.Message, temp float64, which string, setRes func(string), setUsage func(ai.Usage)) {
		loading.Set(which)
		errMsg.Set("")
		setRes("")
		setUsage(ai.Usage{})
		onResult := func(content string, u ai.Usage) { loading.Set(""); setRes(content); setUsage(u) }
		onErr := func(e string) { loading.Set(""); errMsg.Set(e) }
		if useBackendAI {
			cancelFn.Set(ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, temp, onResult, onErr))
		} else {
			cancelFn.Set(ai.SendChat(key, ai.DefaultBaseURL, model, messages, temp, onResult, onErr))
		}
	}

	explain := ui.UseEvent(func() {
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		savedE.Set("")
		pinnedE.Set("")
		prompt := aiCtx.Line() + " In 3-4 friendly sentences, explain how my month went and one thing I could do next."
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, encouraging personal-finance assistant. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: prompt},
		}
		send(messages, 0.5, "explain", explainRes.Set, explainUsage.Set)
	})

	ask := ui.UseEvent(Prevent(func() {
		q := strings.TrimSpace(question.Get())
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		if q == "" {
			errMsg.Set(uistate.T("insights.needQuestion"))
			return
		}
		savedQ.Set("")
		pinnedQ.Set("")
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant. Answer using the provided context; if it isn't enough, say what's missing. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: "Context — " + aiCtx.Line() + "\n\nQuestion: " + q},
		}
		send(messages, 0.4, "qa", qaRes.Set, qaUsage.Set)
	}))

	highlights := spendingHighlights(txns, app.Categories(), base, rates)

	// usageNote renders the token/cost line for one answer (when the model's pricing
	// is known) — so bring-your-own-key users see what each answer costs.
	usageNote := func(u ai.Usage) ui.Node {
		if u.TotalTokens == 0 {
			return Fragment()
		}
		note := uistate.T("insights.usageTokens", u.TotalTokens)
		if cost, ok := ai.EstimateCostUSD(model, u); ok {
			note = uistate.T("insights.usageCost", u.TotalTokens, ai.FormatCostUSD(cost))
		}
		return P(Class("text-faint text-[11px] mt-2"), note)
	}

	busy := loading.Get()
	var action ui.Node
	switch {
	case key == "" && !useBackendAI:
		action = keyHintNode()
	case busy == "explain":
		action = Div(Class("flex items-center gap-2"),
			Button(Class("btn btn-primary"), Type("button"), Attr("disabled", "disabled"), uistate.T("insights.thinking")),
			Button(Class("btn"), Type("button"), OnClick(cancelAI), uistate.T("insights.cancel")),
		)
	case busy == "qa":
		// A Q&A is running; keep Explain visible but disabled (one call at a time).
		action = Button(Class("btn btn-primary inline-flex items-center gap-1.5"), Type("button"), Attr("disabled", "disabled"), uiw.Icon(icon.Sparkles, Class("w-4 h-4 shrink-0")), Span(uistate.T("insights.explainTitle")))
	default:
		action = Button(Class("btn btn-primary inline-flex items-center gap-1.5"), Type("button"), OnClick(explain), uiw.Icon(icon.Sparkles, Class("w-4 h-4 shrink-0")), Span(uistate.T("insights.explainTitle")))
	}

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

	return Div(
		highlights,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.explainTitle")),
			P(Class("muted"), uistate.T("insights.explainHint")),
			action,
			If(errMsg.Get() != "", P(Class("err"), Attr("role", "alert"), errMsg.Get())),
		),
		// Explain's answer lives in its OWN card (C59) so a later Q&A won't wipe it.
		If(explainRes.Get() != "", Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.answerTitle")),
			Div(Class("md insights-answer"), Markdown(explainRes.Get(), MarkdownRenderOptions{LinkTarget: "_blank", LinkRel: "noopener noreferrer"})),
			Div(Class("flex flex-wrap gap-2 items-center"),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.saveTaskTitle")), OnClick(saveE), uistate.T("insights.saveTask")),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pinE), uistate.T("insights.pin")),
				If(savedE.Get() != "", Span(Class("muted"), savedE.Get())),
				If(pinnedE.Get() != "", Span(Class("muted"), pinnedE.Get())),
			),
			usageNote(explainUsage.Get()),
		)),
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.askTitle")),
			// Tappable starter questions so a blank box never stalls the user (L8).
			If(len(starters) > 0, Div(Class("flex flex-wrap gap-2 mb-2"),
				MapKeyed(starters,
					func(q string) any { return q },
					func(q string) ui.Node {
						return ui.CreateElement(suggestChip, suggestChipProps{Q: q, OnPick: func(s string) { question.Set(s) }})
					},
				),
			)),
			// The Q&A needs a key; show the box either way so the feature is visible,
			// with a disabled preview + key hint when no key is set (C9).
			If(key != "" || useBackendAI, Form(Class("form-grid"), OnSubmit(ask),
				Input(Class("field field-wide"), Type("text"), Placeholder(uistate.T("insights.askPlaceholder")), Value(question.Get()), OnInput(onQuestion)),
				Button(Class("btn btn-primary inline-flex items-center gap-1.5"), Type("submit"), uiw.Icon(icon.Sparkles, Class("w-4 h-4 shrink-0")), Span(uistate.T("insights.ask"))),
				If(busy == "qa", Button(Class("btn"), Type("button"), OnClick(cancelAI), uistate.T("insights.cancel"))),
			)),
			If(key == "" && !useBackendAI, Div(
				// Disabled preview still reflects a picked starter question, so the
				// chips work as a compose aid even before a key is added.
				Input(Class("field field-wide"), Type("text"), Attr("disabled", "disabled"), Attr("aria-label", uistate.T("insights.askPlaceholder")), Placeholder(uistate.T("insights.askPlaceholder")), Value(question.Get())),
				keyHintNode(),
			)),
		),
		// The Q&A answer has its own card too. The model emits Markdown (lists, bold,
		// headings), rendered as rich text (C59). The framework's Markdown is GFM-aware
		// and drops active URL schemes (javascript:/data:), so model-authored text
		// can't smuggle an executable href; links open safely in a new tab.
		If(qaRes.Get() != "", Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.answerTitle")),
			Div(Class("md insights-answer"), Markdown(qaRes.Get(), MarkdownRenderOptions{LinkTarget: "_blank", LinkRel: "noopener noreferrer"})),
			Div(Class("flex flex-wrap gap-2 items-center"),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.saveTaskTitle")), OnClick(saveQ), uistate.T("insights.saveTask")),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pinQ), uistate.T("insights.pin")),
				If(savedQ.Get() != "", Span(Class("muted"), savedQ.Get())),
				If(pinnedQ.Get() != "", Span(Class("muted"), pinnedQ.Get())),
			),
			usageNote(qaUsage.Get()),
		)),
		pinnedCard,
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
