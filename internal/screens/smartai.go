// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/aiprovider"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// smartAIConn is the resolved transport for a smart AI call: either the hosted
// backend proxy or the user's own OpenAI key. The hub reads prefs/settings at a
// stable render position and passes this down, so the call helpers stay
// hook-free.
type smartAIConn struct {
	Backend     bool
	ServerURL   string
	ServerToken string
	Key         string
}

// resolveAIConn builds the transport config from the app's settings + prefs.
func resolveAIConn(app *appstate.App, backendActive bool, serverURL, serverToken string) smartAIConn {
	return smartAIConn{Backend: backendActive, ServerURL: serverURL, ServerToken: serverToken, Key: app.Settings().OpenAIKey}
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

// runSmartAI runs a request under the product routing policy: gpt-5.4-mini first,
// escalating ONCE to gpt-5.5 when the mini answer isn't good enough.
func runSmartAI(c smartAIConn, req smartai.Request, onResult func(string), onError func(string)) {
	sendSmartAIOn(c, aiprovider.SmartModelID, req, func(text string) {
		if smartai.Acceptable(text) {
			onResult(text)
			return
		}
		sendSmartAIOn(c, aiprovider.SmartEscalationModelID, req, onResult, onError)
	}, onError)
}

// ratesOf builds the household rate table from the app's settings (no hooks).
func ratesOf(app *appstate.App) currency.Rates {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: app.Settings().FXRates}
}

// --- context builders (all hook-free; safe to call at click time) ---------

// accountContextString is a compact snapshot of non-archived accounts + balances.
func accountContextString(app *appstate.App) string {
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
		b.WriteString("- " + a.Name + " (" + string(a.Type) + "): " + fmtMoney(bal) + "\n")
		if n++; n >= 25 {
			break
		}
	}
	if n == 0 {
		return "(no accounts yet)"
	}
	return b.String()
}

// financialContextString summarizes net worth and this-month flows.
func financialContextString(app *appstate.App) string {
	rates := ratesOf(app)
	accounts, txns := app.Accounts(), app.Transactions()
	net, assets, liab, _ := ledger.NetWorth(accounts, txns, rates)
	var b strings.Builder
	b.WriteString("Net worth: " + fmtMoney(net) + " (assets " + fmtMoney(assets) + ", debts " + fmtMoney(liab) + ")\n")
	start, end := dateutil.MonthRange(time.Now())
	if income, expense, err := ledger.PeriodTotals(txns, start, end, rates); err == nil {
		b.WriteString("This month — income " + fmtMoney(income) + ", spending " + fmtMoney(expense) + "\n")
	}
	return b.String()
}

// subsContextString lists the detected subscriptions for the overlap check.
func subsContextString(app *appstate.App) string {
	subs, err := subscriptions.Detect(app.Transactions(), ratesOf(app), 3)
	if err != nil || len(subs) == 0 {
		return "(no subscriptions detected)"
	}
	var b strings.Builder
	for i, s := range subs {
		if i >= 30 {
			break
		}
		b.WriteString("- " + s.Name + ": " + fmtMoney(money.New(s.Amount, s.Currency)) + "/" + string(s.Cadence) + "\n")
	}
	return b.String()
}

// categoriesContextString lists the user's category names for auto-categorization.
func categoriesContextString(app *appstate.App) string {
	var b strings.Builder
	for _, c := range app.Categories() {
		b.WriteString("- " + c.Name + "\n")
	}
	if b.Len() == 0 {
		return "(no categories yet)"
	}
	return b.String()
}

// txnContextString lists recent transactions for the tax-relevant scan.
func txnContextString(app *appstate.App) string {
	txns := app.Transactions()
	var b strings.Builder
	n := 0
	for i := len(txns) - 1; i >= 0 && n < 40; i-- {
		t := txns[i]
		if t.IsTransfer() {
			continue
		}
		b.WriteString("- " + t.Date.Format("Jan 2") + " " + txnLabelOf(t) + ": " + fmtMoney(t.Amount) + "\n")
		n++
	}
	if n == 0 {
		return "(no transactions yet)"
	}
	return b.String()
}

// goalsContextString lists the user's goals with progress for prioritization.
func goalsContextString(app *appstate.App) string {
	var b strings.Builder
	for _, g := range app.Goals() {
		if g.Archived {
			continue
		}
		line := "- " + g.Name + ": " + fmtMoney(g.CurrentAmount) + " of " + fmtMoney(g.TargetAmount)
		if !g.TargetDate.IsZero() {
			line += " by " + g.TargetDate.Format("Jan 2006")
		}
		b.WriteString(line + "\n")
	}
	if b.Len() == 0 {
		return "(no goals yet)"
	}
	return b.String()
}

// txnLabelOf is the display label for a transaction (payee, else description).
func txnLabelOf(t domain.Transaction) string {
	if s := strings.TrimSpace(t.Payee); s != "" {
		return s
	}
	return strings.TrimSpace(t.Desc)
}

// --- generic AI feature spec + controls -----------------------------------

// aiFeatureSpec describes how to render and build a request for one AI feature.
// Input features show an entry field; the rest show a single action button. The
// build closure runs at click time with the live app, so it always sees current
// data and never calls a hook.
type aiFeatureSpec struct {
	input       bool
	title       string
	placeholder string
	btnLabel    string
	build       func(app *appstate.App, question string) smartai.Request
}

// aiSpec returns the render spec for an implemented AI feature.
func aiSpec(code string) (aiFeatureSpec, bool) {
	switch code {
	case "SMART-A5":
		return aiFeatureSpec{input: true, title: uistate.T("smart.askTitle"), placeholder: uistate.T("smart.askPlaceholder"),
			build: func(app *appstate.App, q string) smartai.Request {
				return smartai.AccountQA(q, accountContextString(app))
			}}, true
	case "SMART-A10":
		return aiFeatureSpec{title: uistate.T("smart.healthTitle"), btnLabel: uistate.T("smart.healthBtn"),
			build: func(app *appstate.App, _ string) smartai.Request {
				return smartai.AccountHealth(accountContextString(app))
			}}, true
	case "SMART-G4":
		return aiFeatureSpec{input: true, title: uistate.T("smart.goalTitle"), placeholder: uistate.T("smart.goalPlaceholder"),
			build: func(app *appstate.App, q string) smartai.Request {
				return smartai.GoalDraft(q, financialContextString(app))
			}}, true
	case "SMART-P2":
		return aiFeatureSpec{input: true, title: uistate.T("smart.scenarioTitle"), placeholder: uistate.T("smart.scenarioPlaceholder"),
			build: func(app *appstate.App, q string) smartai.Request {
				return smartai.ScenarioDraft(q, financialContextString(app))
			}}, true
	case "SMART-P3":
		return aiFeatureSpec{title: uistate.T("smart.outlookTitle"), btnLabel: uistate.T("smart.outlookBtn"),
			build: func(app *appstate.App, _ string) smartai.Request { return smartai.Outlook(financialContextString(app)) }}, true
	case "SMART-AL4":
		return aiFeatureSpec{input: true, title: uistate.T("smart.allocTitle"), placeholder: uistate.T("smart.allocPlaceholder"),
			build: func(app *appstate.App, q string) smartai.Request {
				return smartai.AllocationIntent(q, financialContextString(app))
			}}, true
	case "SMART-SU2":
		return aiFeatureSpec{title: uistate.T("smart.overlapTitle"), btnLabel: uistate.T("smart.overlapBtn"),
			build: func(app *appstate.App, _ string) smartai.Request {
				return smartai.OverlapDetect(subsContextString(app))
			}}, true
	case "SMART-D4":
		return aiFeatureSpec{input: true, title: uistate.T("smart.todoTitle"), placeholder: uistate.T("smart.todoPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.TodoParse(q) }}, true
	case "SMART-A3":
		return aiFeatureSpec{input: true, title: uistate.T("smart.cleanupTitle"), placeholder: uistate.T("smart.cleanupPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.AccountCleanup(q) }}, true
	case "SMART-T1":
		return aiFeatureSpec{input: true, title: uistate.T("smart.categorizeTitle"), placeholder: uistate.T("smart.categorizePlaceholder"),
			build: func(app *appstate.App, q string) smartai.Request {
				return smartai.Categorize(q, categoriesContextString(app))
			}}, true
	case "SMART-T3":
		return aiFeatureSpec{input: true, title: uistate.T("smart.searchTitle"), placeholder: uistate.T("smart.searchPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.SearchParse(q) }}, true
	case "SMART-T5":
		return aiFeatureSpec{input: true, title: uistate.T("smart.merchantTitle"), placeholder: uistate.T("smart.merchantPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.MerchantCleanup(q) }}, true
	case "SMART-T12":
		return aiFeatureSpec{title: uistate.T("smart.taxTitle"), btnLabel: uistate.T("smart.taxBtn"),
			build: func(app *appstate.App, _ string) smartai.Request { return smartai.TaxRelevant(txnContextString(app)) }}, true
	case "SMART-G9":
		return aiFeatureSpec{title: uistate.T("smart.priorityTitle"), btnLabel: uistate.T("smart.priorityBtn"),
			build: func(app *appstate.App, _ string) smartai.Request {
				return smartai.GoalPriority(goalsContextString(app))
			}}, true
	case "SMART-SU10":
		return aiFeatureSpec{input: true, title: uistate.T("smart.benchmarkTitle"), placeholder: uistate.T("smart.benchmarkPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.SubBenchmark(q) }}, true
	case "SMART-SU13":
		return aiFeatureSpec{title: uistate.T("smart.bundleTitle"), btnLabel: uistate.T("smart.bundleBtn"),
			build: func(app *appstate.App, _ string) smartai.Request { return smartai.BundleFinder(subsContextString(app)) }}, true
	case "SMART-T10":
		return aiFeatureSpec{input: true, title: uistate.T("smart.importTitle"), placeholder: uistate.T("smart.importPlaceholder"),
			build: func(_ *appstate.App, q string) smartai.Request { return smartai.ImportMapping(q) }}, true
	}
	return aiFeatureSpec{}, false
}

// smartAIControlProps carries one feature's spec + transport + cost label.
type smartAIControlProps struct {
	Code string
	Spec aiFeatureSpec
	Conn smartAIConn
	Cost string
}

// smartAIControl renders one AI feature: an input bar or an action button, plus
// the answer area. Its own component so the state hooks sit at stable positions.
func smartAIControl(props smartAIControlProps) ui.Node {
	question := ui.UseState("")
	answer := ui.UseState("")
	loading := ui.UseState(false)
	errMsg := ui.UseState("")

	onInput := ui.UseEvent(func(v string) { question.Set(v) })

	run := ui.UseEvent(func() {
		if loading.Get() {
			return
		}
		q := strings.TrimSpace(question.Get())
		if props.Spec.input && q == "" {
			return
		}
		app := appstate.Default
		if app == nil {
			return
		}
		req := props.Spec.build(app, q)
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
		result = P(ClassStr(tw.Fold(tw.Text14, tw.Mt2)), Attr("data-testid", "smart-ai-answer-"+props.Code), answer.Get())
	}

	btnLabel := props.Spec.btnLabel
	if props.Spec.input {
		btnLabel = uistate.T("smart.ask")
	}
	if loading.Get() {
		btnLabel = uistate.T("smart.asking")
	}

	var control ui.Node
	if props.Spec.input {
		control = Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap2)),
			Input(css.Class("field field-wide"), Type("text"),
				Attr("data-testid", "smart-ai-input-"+props.Code),
				Attr("aria-label", props.Spec.placeholder), Placeholder(props.Spec.placeholder),
				Value(question.Get()), OnInput(onInput),
			),
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("data-testid", "smart-ai-btn-"+props.Code), OnClick(run), btnLabel,
			),
		)
	} else {
		control = Button(ClassStr("btn btn-primary "+tw.Fold(tw.SelfStart)), Type("button"),
			Attr("data-testid", "smart-ai-btn-"+props.Code), OnClick(run), btnLabel,
		)
	}

	return Div(ClassStr("smart-card "+tw.Fold(tw.Flex, tw.FlexCol, tw.Gap2, tw.Border, tw.BorderLine, tw.RoundedXl, tw.Px3, tw.Py2)),
		Attr("data-testid", "smart-ai-feature-"+props.Code),
		Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2)),
			Span(ClassStr(tw.Fold(tw.FontSemibold, tw.Text14)), props.Spec.title),
			Span(ClassStr(tw.Fold(tw.Text11, tw.TextFaint)), props.Cost),
		),
		control,
		result,
	)
}

// smartAISection renders the enabled, implemented AI features. It is shown only
// when an inference provider is configured (the AI cost is real, so the gate is
// honest); otherwise it shows a "configure a provider" hint, never dead controls.
func smartAISection(settings smart.Settings, conn smartAIConn, hasProvider bool) ui.Node {
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
		return uiw.Card(uiw.CardProps{Title: uistate.T("smart.aiTitle"), TestID: "smart-ai",
			Body: P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)), uistate.T("smart.aiNeedsProvider"))})
	}
	return uiw.Card(uiw.CardProps{Title: uistate.T("smart.aiTitle"), TestID: "smart-ai",
		Body: Div(ClassStr(tw.Fold(tw.Flex, tw.FlexCol, tw.Gap3)),
			MapKeyed(enabled,
				func(f smart.Feature) any { return f.Code },
				func(f smart.Feature) ui.Node {
					spec, ok := aiSpec(f.Code)
					if !ok {
						return Fragment()
					}
					cost := uistate.T("smart.aiCostPrefix") + " " + smart.FormatCents(f.EstimateCost(false).Cents) + uistate.T("smart.perUse")
					return ui.CreateElement(smartAIControl, smartAIControlProps{Code: f.Code, Spec: spec, Conn: conn, Cost: cost})
				},
			),
		),
	})
}
