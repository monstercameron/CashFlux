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
	useBackendAI := strings.TrimSpace(pr.ServerURL) != "" && strings.TrimSpace(pr.ServerToken) != ""
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

	result := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")
	saved := ui.UseState("")
	usage := ui.UseState(ai.Usage{})
	pinned := ui.UseState("")
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
	question := ui.UseState("")
	onQuestion := ui.UseEvent(func(v string) { question.Set(v) })

	saveAsTask := ui.UseEvent(Prevent(func() {
		r := strings.TrimSpace(result.Get())
		if r == "" {
			return
		}
		// Title from the question when this was a Q&A; otherwise a short generic
		// label. The full answer always lives in the notes, not the title (C27).
		title := strings.TrimSpace(question.Get())
		if title == "" {
			title = uistate.T("insights.aiTaskTitle")
		}
		if rs := []rune(title); len(rs) > 80 { // rune-safe truncation for the title
			title = strings.TrimSpace(string(rs[:80])) + "…"
		}
		t := domain.Task{
			ID: id.New(), Title: title, Notes: r,
			Status: domain.StatusOpen, Priority: domain.PriorityMedium, Source: domain.SourceAI,
		}
		if err := app.PutTask(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		saved.Set(uistate.T("insights.savedToTodo"))
	}))

	pinInsight := ui.UseEvent(Prevent(func() {
		r := strings.TrimSpace(result.Get())
		if r == "" {
			return
		}
		if err := app.PutSavedInsight(domain.SavedInsight{ID: id.New(), Text: r, CreatedAt: time.Now()}); err != nil {
			errMsg.Set(err.Error())
			return
		}
		pinned.Set(uistate.T("insights.pinnedConfirm"))
		bump()
	}))
	deletePinned := func(pid string) {
		_ = app.DeleteSavedInsight(pid)
		bump()
	}

	explain := ui.UseEvent(func() {
		if key == "" && !useBackendAI {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		loading.Set(true)
		errMsg.Set("")
		result.Set("")
		saved.Set("")
		pinned.Set("")
		usage.Set(ai.Usage{})
		prompt := aiCtx.Line() + " In 3-4 friendly sentences, explain how my month went and one thing I could do next."
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, encouraging personal-finance assistant. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: prompt},
		}
		if useBackendAI {
			cancelFn.Set(ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0.5,
				func(content string, u ai.Usage) { loading.Set(false); result.Set(content); usage.Set(u) },
				func(e string) { loading.Set(false); errMsg.Set(e) },
			))
		} else {
			cancelFn.Set(ai.SendChat(key, ai.DefaultBaseURL, model, messages, 0.5,
				func(content string, u ai.Usage) { loading.Set(false); result.Set(content); usage.Set(u) },
				func(e string) { loading.Set(false); errMsg.Set(e) },
			))
		}
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
		loading.Set(true)
		errMsg.Set("")
		result.Set("")
		saved.Set("")
		pinned.Set("")
		usage.Set(ai.Usage{})
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant. Answer using the provided context; if it isn't enough, say what's missing. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: "Context — " + aiCtx.Line() + "\n\nQuestion: " + q},
		}
		if useBackendAI {
			cancelFn.Set(ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, messages, 0.4,
				func(content string, u ai.Usage) { loading.Set(false); result.Set(content); usage.Set(u) },
				func(e string) { loading.Set(false); errMsg.Set(e) },
			))
		} else {
			cancelFn.Set(ai.SendChat(key, ai.DefaultBaseURL, model, messages, 0.4,
				func(content string, u ai.Usage) { loading.Set(false); result.Set(content); usage.Set(u) },
				func(e string) { loading.Set(false); errMsg.Set(e) },
			))
		}
	}))

	highlights := spendingHighlights(txns, app.Categories(), base, rates)

	// Show how many tokens the last answer used and (when the model's pricing is
	// known) its approximate cost — so bring-your-own-key users see what they spend.
	usageNote := ""
	if u := usage.Get(); u.TotalTokens > 0 {
		if cost, ok := ai.EstimateCostUSD(model, u); ok {
			usageNote = uistate.T("insights.usageCost", u.TotalTokens, ai.FormatCostUSD(cost))
		} else {
			usageNote = uistate.T("insights.usageTokens", u.TotalTokens)
		}
	}

	var action ui.Node
	switch {
	case key == "" && !useBackendAI:
		action = P(Class("muted"), uistate.T("insights.keyHint"))
	case loading.Get():
		action = Div(Class("flex items-center gap-2"),
			Button(Class("btn btn-primary"), Type("button"), Attr("disabled", "disabled"), uistate.T("insights.thinking")),
			Button(Class("btn"), Type("button"), OnClick(cancelAI), uistate.T("insights.cancel")),
		)
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
			)),
			If(key == "" && !useBackendAI, Div(
				// Disabled preview still reflects a picked starter question, so the
				// chips work as a compose aid even before a key is added.
				Input(Class("field field-wide"), Type("text"), Attr("disabled", "disabled"), Attr("aria-label", uistate.T("insights.askPlaceholder")), Placeholder(uistate.T("insights.askPlaceholder")), Value(question.Get())),
				P(Class("muted"), uistate.T("insights.keyHint")),
			)),
		),
		If(result.Get() != "", Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.answerTitle")),
			P(result.Get()),
			Div(Class("flex flex-wrap gap-2 items-center"),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.saveTaskTitle")), OnClick(saveAsTask), uistate.T("insights.saveTask")),
				Button(Class("btn"), Type("button"), Title(uistate.T("insights.pinTitle")), OnClick(pinInsight), uistate.T("insights.pin")),
				If(saved.Get() != "", Span(Class("muted"), saved.Get())),
				If(pinned.Get() != "", Span(Class("muted"), pinned.Get())),
			),
			If(usageNote != "", P(Class("text-faint text-[11px] mt-2"), usageNote)),
		)),
		pinnedCard,
	)
}

type pinnedInsightRowProps struct {
	Insight  domain.SavedInsight
	OnDelete func(string)
}

// PinnedInsightRow renders one pinned insight with its date and a remove button.
// It owns its own click handler (per the no-hooks-in-loops rule).
func PinnedInsightRow(props pinnedInsightRowProps) ui.Node {
	p := props.Insight
	del := ui.UseEvent(Prevent(func() { props.OnDelete(p.ID) }))
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), p.Text),
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
