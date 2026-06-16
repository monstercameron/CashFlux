//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/ledger"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Insights is AI analysis (OpenAI, client-side, bring-your-own-key): an
// "Explain my month" narrative generated from the user's live figures.
func Insights() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), "App state is not ready yet."))
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

	result := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")

	explain := ui.UseEvent(func() {
		if key == "" {
			errMsg.Set("Add your OpenAI key in Settings first.")
			return
		}
		loading.Set(true)
		errMsg.Set("")
		result.Set("")
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

	var action ui.Node
	if key == "" {
		action = P(Class("muted"), "Add your OpenAI key in Settings to enable AI insights. Your key stays on this device and is only sent to OpenAI when you ask.")
	} else {
		label := "Explain my month"
		if loading.Get() {
			label = "Thinking…"
		}
		action = Button(Class("btn btn-primary"), Type("button"), OnClick(explain), label)
	}

	return Div(
		Section(Class("card"),
			H2(Class("card-title"), "Explain my month"),
			P(Class("muted"), "A friendly summary of your month, generated from your live figures."),
			action,
			If(errMsg.Get() != "", P(Class("err"), errMsg.Get())),
		),
		If(result.Get() != "", Section(Class("card"),
			H2(Class("card-title"), "Your month"),
			P(result.Get()),
		)),
	)
}
