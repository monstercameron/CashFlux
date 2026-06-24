// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// smartAIConn is the resolved transport for a smart AI call: either the hosted
// backend proxy or the user's own OpenAI key. The hub reads prefs/settings at a
// stable render position and passes this down, so the call helpers stay free of
// hooks.
type smartAIConn struct {
	Backend     bool
	ServerURL   string
	ServerToken string
	Key         string
}

// resolveAIConn builds the transport config from the app's settings + prefs.
func resolveAIConn(app *appstate.App, backendActive bool, serverURL, serverToken string) smartAIConn {
	return smartAIConn{
		Backend:     backendActive,
		ServerURL:   serverURL,
		ServerToken: serverToken,
		Key:         app.Settings().OpenAIKey,
	}
}

// sendSmartAIOn places one call on a specific model via the resolved transport.
func sendSmartAIOn(c smartAIConn, model string, req smartai.Request, onResult func(string), onError func(string)) {
	msgs := []ai.Message{{Role: "system", Content: req.System}, {Role: "user", Content: req.User}}
	onR := func(text string, _ ai.Usage) { onResult(text) }
	if c.Backend {
		ai.SendProxyChat(c.ServerURL, c.ServerToken, model, msgs, 0, onR, onError)
		return
	}
	ai.SendChat(c.Key, ai.DefaultBaseURL, model, msgs, 0, onR, onError)
}

// runSmartAI runs a request under the product's routing policy: the cheap, fast
// gpt-5.4-mini first, escalating ONCE to gpt-5.5 when the mini answer isn't good
// enough (smartai.Acceptable is false). The escalation buys capability without
// paying for full-effort reasoning (see aiprovider.SmartEscalationProfile).
func runSmartAI(c smartAIConn, req smartai.Request, onResult func(string), onError func(string)) {
	sendSmartAIOn(c, aiprovider.SmartModelID, req, func(text string) {
		if smartai.Acceptable(text) {
			onResult(text)
			return
		}
		// Not good enough — escalate to the stronger model.
		sendSmartAIOn(c, aiprovider.SmartEscalationModelID, req, onResult, onError)
	}, onError)
}

// accountContextString builds a compact, plain-text snapshot of the user's
// non-archived accounts and balances for grounding an AI answer. It is capped so
// a large household doesn't blow the prompt budget.
func accountContextString(app *appstate.App, rates currency.Rates) string {
	var b strings.Builder
	n := 0
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		bal, err := ledger.Balance(a, app.Transactions())
		if err != nil {
			continue
		}
		b.WriteString("- ")
		b.WriteString(a.Name)
		b.WriteString(" (")
		b.WriteString(string(a.Type))
		b.WriteString("): ")
		b.WriteString(fmtMoney(bal))
		b.WriteByte('\n')
		if n++; n >= 25 {
			break
		}
	}
	if n == 0 {
		return "(no accounts yet)"
	}
	return b.String()
}

// smartAskBarProps carries everything the A5 ask bar needs without hooks.
type smartAskBarProps struct {
	Conn smartAIConn
	Cost string // pre-formatted per-question cost estimate
}

// smartAskBar is the SMART-A5 natural-language account Q&A control: an input, an
// Ask button with a visible per-question cost, and the answer. Its own component
// so its state hooks sit at stable positions.
func smartAskBar(props smartAskBarProps) ui.Node {
	question := ui.UseState("")
	answer := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")

	onInput := ui.UseEvent(func(v string) { question.Set(v) })

	ask := ui.UseEvent(func() {
		q := strings.TrimSpace(question.Get())
		if q == "" || loading.Get() {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		base := app.Settings().BaseCurrency
		if base == "" {
			base = "USD"
		}
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		req := smartai.AccountQA(q, accountContextString(app, rates))

		loading.Set(true)
		errMsg.Set("")
		answer.Set("")
		runSmartAI(props.Conn, req,
			func(text string) { loading.Set(false); answer.Set(strings.TrimSpace(text)) },
			func(e string) { loading.Set(false); errMsg.Set(e) },
		)
	})

	// Result area: error, answer, or nothing.
	var result ui.Node = Fragment()
	if errMsg.Get() != "" {
		result = P(ClassStr(tw.Fold(tw.Text13, tw.TextDown, tw.Mt2)), errMsg.Get())
	} else if answer.Get() != "" {
		result = P(ClassStr(tw.Fold(tw.Text14, tw.Mt2)), Attr("data-testid", "smart-ai-answer"), answer.Get())
	}

	btnLabel := uistate.T("smart.ask")
	if loading.Get() {
		btnLabel = uistate.T("smart.asking")
	}

	return Div(ClassStr("smart-card "+tw.Fold(tw.Flex, tw.FlexCol, tw.Gap2, tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
		Attr("data-testid", "smart-ask-A5"),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2)),
			Span(ClassStr(tw.Fold(tw.FontSemibold, tw.Text14)), uistate.T("smart.askTitle")),
			Span(ClassStr(tw.Fold(tw.Text11, tw.TextFaint)), props.Cost),
		),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
			Input(Attr("id", "smart-ask-input"), css.Class("field field-wide"), Type("text"),
				Attr("data-testid", "smart-ask-input"),
				Attr("aria-label", uistate.T("smart.askPlaceholder")),
				Placeholder(uistate.T("smart.askPlaceholder")),
				Value(question.Get()), OnInput(onInput),
			),
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("data-testid", "smart-ask-btn"),
				OnClick(ask),
				btnLabel,
			),
		),
		result,
	)
}

// smartAISection renders the interactive AI features the user has enabled and
// that have a shipped UI. It is only shown when an inference provider is
// configured (the AI cost is real, so the gate is honest); otherwise it shows a
// short "configure a provider" hint instead of dead controls.
func smartAISection(settings smart.Settings, conn smartAIConn, hasProvider bool) ui.Node {
	// Which implemented AI features are enabled?
	var enabled []smart.Feature
	for _, f := range smart.Catalog() {
		if f.Tier == smart.TierAI && smartai.Implemented(f.Code) && settings.IsEnabled(f.Code) {
			enabled = append(enabled, f)
		}
	}
	if len(enabled) == 0 {
		return Fragment()
	}
	if !hasProvider {
		return uiw.Card(uiw.CardProps{
			Title:  uistate.T("smart.aiTitle"),
			TestID: "smart-ai",
			Body:   P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)), uistate.T("smart.aiNeedsProvider")),
		})
	}
	return uiw.Card(uiw.CardProps{
		Title:  uistate.T("smart.aiTitle"),
		TestID: "smart-ai",
		Body: Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol, tw.Gap3)),
			MapKeyed(enabled,
				func(f smart.Feature) any { return f.Code },
				func(f smart.Feature) ui.Node { return smartAIControl(f, conn) },
			),
		),
	})
}

// smartAIControl renders the interactive control for one implemented AI feature.
func smartAIControl(f smart.Feature, conn smartAIConn) ui.Node {
	cost := uistate.T("smart.aiCostPrefix") + " " + smart.FormatCents(f.EstimateCost(false).Cents) + uistate.T("smart.perUse")
	switch f.Code {
	case "SMART-A5":
		return ui.CreateElement(smartAskBar, smartAskBarProps{Conn: conn, Cost: cost})
	case "SMART-P3":
		return ui.CreateElement(smartOutlookCard, smartOutlookProps{Conn: conn, Cost: cost})
	default:
		return Fragment()
	}
}

// outlookContextString builds a compact snapshot of the household's position for
// the P3 narration: net worth, this-month income/expense/net, and liquid cash.
func outlookContextString(app *appstate.App, rates currency.Rates) string {
	accounts := app.Accounts()
	txns := app.Transactions()
	net, assets, liab, _ := ledger.NetWorth(accounts, txns, rates)
	var b strings.Builder
	b.WriteString("Net worth: " + fmtMoney(net) + " (assets " + fmtMoney(assets) + ", debts " + fmtMoney(liab) + ")\n")
	w := uistate.UsePeriod().Get()
	start, end := w.Range()
	income, expense, err := ledger.PeriodTotals(txns, start, end, rates)
	if err == nil {
		b.WriteString("This period — income " + fmtMoney(income) + ", spending " + fmtMoney(expense) + "\n")
	}
	return b.String()
}

// smartOutlookProps carries the P3 narration control's config.
type smartOutlookProps struct {
	Conn smartAIConn
	Cost string
}

// smartOutlookCard is the SMART-P3 control: a "Summarize my outlook" button that
// narrates the live figures into one plain-English paragraph. Its own component
// for stable state hooks.
func smartOutlookCard(props smartOutlookProps) ui.Node {
	answer := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")

	run := ui.UseEvent(func() {
		if loading.Get() {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		base := app.Settings().BaseCurrency
		if base == "" {
			base = "USD"
		}
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		req := smartai.Outlook(outlookContextString(app, rates))
		loading.Set(true)
		errMsg.Set("")
		answer.Set("")
		runSmartAI(props.Conn, req,
			func(text string) { loading.Set(false); answer.Set(strings.TrimSpace(text)) },
			func(e string) { loading.Set(false); errMsg.Set(e) },
		)
	})

	var result ui.Node = Fragment()
	if errMsg.Get() != "" {
		result = P(ClassStr(tw.Fold(tw.Text13, tw.TextDown, tw.Mt2)), errMsg.Get())
	} else if answer.Get() != "" {
		result = P(ClassStr(tw.Fold(tw.Text14, tw.Mt2)), Attr("data-testid", "smart-outlook-answer"), answer.Get())
	}
	btnLabel := uistate.T("smart.outlookBtn")
	if loading.Get() {
		btnLabel = uistate.T("smart.asking")
	}

	return Div(ClassStr("smart-card "+tw.Fold(tw.Flex, tw.FlexCol, tw.Gap2, tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
		Attr("data-testid", "smart-outlook-P3"),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2)),
			Span(ClassStr(tw.Fold(tw.FontSemibold, tw.Text14)), uistate.T("smart.outlookTitle")),
			Span(ClassStr(tw.Fold(tw.Text11, tw.TextFaint)), props.Cost),
		),
		Button(ClassStr("btn btn-primary "+tw.Fold(tw.SelfStart)), Type("button"),
			Attr("data-testid", "smart-outlook-btn"), OnClick(run), btnLabel,
		),
		result,
	)
}
