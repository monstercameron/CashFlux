//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
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

	result := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")
	saved := ui.UseState("")
	question := ui.UseState("")
	onQuestion := ui.UseEvent(func(v string) { question.Set(v) })

	saveAsTask := ui.UseEvent(Prevent(func() {
		r := strings.TrimSpace(result.Get())
		if r == "" {
			return
		}
		title := r
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

	explain := ui.UseEvent(func() {
		if key == "" {
			errMsg.Set(uistate.T("insights.needKey"))
			return
		}
		loading.Set(true)
		errMsg.Set("")
		result.Set("")
		saved.Set("")
		prompt := fmt.Sprintf(
			"My figures this month — net worth: %s, income: %s, spending: %s. In 3-4 friendly sentences, explain how my month went and one thing I could do next.",
			fmtMoney(net), fmtMoney(income), fmtMoney(expense),
		)
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, encouraging personal-finance assistant. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: prompt},
		}
		ai.SendChat(key, ai.DefaultBaseURL, model, messages, 0.5,
			func(content string) { loading.Set(false); result.Set(content) },
			func(e string) { loading.Set(false); errMsg.Set(e) },
		)
	})

	ask := ui.UseEvent(Prevent(func() {
		q := strings.TrimSpace(question.Get())
		if key == "" {
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
		ctx := fmt.Sprintf("Context — net worth: %s, this month's income: %s, spending: %s, across %d active accounts.",
			fmtMoney(net), fmtMoney(income), fmtMoney(expense), active)
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a concise, friendly personal-finance assistant. Answer using the provided context; if it isn't enough, say what's missing. Plain English, no jargon."},
			{Role: ai.RoleUser, Content: ctx + "\n\nQuestion: " + q},
		}
		ai.SendChat(key, ai.DefaultBaseURL, model, messages, 0.4,
			func(content string) { loading.Set(false); result.Set(content) },
			func(e string) { loading.Set(false); errMsg.Set(e) },
		)
	}))

	highlights := spendingHighlights(txns, app.Categories(), base, rates)

	var action ui.Node
	if key == "" {
		action = P(Class("muted"), uistate.T("insights.keyHint"))
	} else {
		label := uistate.T("insights.explainTitle")
		if loading.Get() {
			label = uistate.T("insights.thinking")
		}
		action = Button(Class("btn btn-primary"), Type("button"), OnClick(explain), label)
	}

	return Div(
		highlights,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.explainTitle")),
			P(Class("muted"), uistate.T("insights.explainHint")),
			action,
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		),
		If(key != "", Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.askTitle")),
			Form(Class("form-grid"), OnSubmit(ask),
				Input(Class("field field-wide"), Type("text"), Placeholder(uistate.T("insights.askPlaceholder")), Value(question.Get()), OnInput(onQuestion)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("insights.ask")),
			),
		)),
		If(result.Get() != "", Section(Class("card"),
			H2(Class("card-title"), uistate.T("insights.answerTitle")),
			P(result.Get()),
			Button(Class("btn"), Type("button"), Title(uistate.T("insights.saveTaskTitle")), OnClick(saveAsTask), uistate.T("insights.saveTask")),
			If(saved.Get() != "", Span(Class("muted"), Style(map[string]string{"margin-left": "0.5rem"}), saved.Get())),
		)),
	)
}

// spendingHighlights renders an offline "what changed" card: it detects
// categories whose spend this month deviates materially from their recent
// average and explains each in plain English. It needs no AI key. Returns an
// empty node when there's nothing notable, so the card simply doesn't appear.
// The card is non-interactive, so its rows are safe to render in a loop.
func spendingHighlights(txns []domain.Transaction, categories []domain.Category, base string, rates currency.Rates) ui.Node {
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
		return Fragment()
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

	anomalies := insights.Detect(series, insights.DefaultOptions())
	if len(anomalies) == 0 {
		return Fragment()
	}

	rows := make([]ui.Node, 0, len(anomalies))
	for _, a := range anomalies {
		pct := a.PctChange
		if pct < 0 {
			pct = -pct
		}
		current := fmtMoney(money.New(a.Current, base))
		baseline := fmtMoney(money.New(a.Baseline, base))
		tone, key := "text-up", "insights.highlightDown"
		if a.Direction == insights.Up {
			tone, key = "text-down", "insights.highlightUp"
		}
		rows = append(rows, P(Class("insight-row"),
			Span(Class("insight-dot "+tone), If(a.Direction == insights.Up, Text("↑")), If(a.Direction == insights.Down, Text("↓"))),
			Span(uistate.T(key, a.Category, pct, current, baseline)),
		))
	}

	return Section(Class("card"),
		H2(Class("card-title"), uistate.T("insights.highlightsTitle")),
		P(Class("muted"), uistate.T("insights.highlightsHint")),
		Div(Class("insight-list"), rows),
	)
}
