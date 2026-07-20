// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/debtcoach"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// --- shared coaching model ------------------------------------------------------

// baseDebts returns the FX-converted base-currency debts (for the whole-plan
// figures) alongside the base currency code — the same AggregateDebts the strategy
// panel and ladder use, so every derived plan agrees.
func baseDebts(app *appstate.App, base string) []payoff.Debt {
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	debts, _ := payoff.AggregateDebts(app.Accounts(), app.Transactions(), base, rates)
	return debts
}

// coachAlerts builds the debtcoach snapshot from the shared debt view and evaluates
// the rules. Per-account figures stay in each debt's own currency (the per-debt rules
// are currency-consistent); the portfolio aggregates come pre-converted from the
// engine surface via computeDebtView.
func coachAlerts(app *appstate.App) []debtcoach.Alert {
	v := computeDebtView(app)
	lines := make([]debtcoach.DebtLine, 0, len(v.Liabs))
	for _, ac := range v.Liabs {
		owed := v.OwedByID[ac.ID]
		lines = append(lines, debtcoach.DebtLine{
			Name:       ac.Name,
			Balance:    owed.Amount,
			AprPercent: ac.InterestRateAPR,
			MinPayment: ac.MinPayment.Amount,
			Limit:      ac.CreditLimit.Amount,
			Revolving:  ac.CreditLimit.Amount > 0,
		})
	}
	debts := baseDebts(app, v.Base)
	var monthlyInterest int64
	for _, d := range debts {
		monthlyInterest += int64(math.Round(float64(d.Balance) * d.AprPercent / 1200.0))
	}
	minMonths, minOK := 0, false
	if plan, ok := payoff.BuildPlan(debts, 0, strategyFromConfig(v.Cfg)); ok {
		minMonths, minOK = plan.Months, true
	}
	return debtcoach.Evaluate(debtcoach.Input{
		Debts:                lines,
		Assets:               majorMoney(v.Vars["assets"], v.Base).Amount,
		Liabilities:          v.TotalOwed.Amount,
		MinPaymentsTotal:     majorMoney(v.Vars["min_payments_total"], v.Base).Amount,
		MonthlyInterestTotal: monthlyInterest,
		CreditUtilPct:        v.Vars["credit_utilization"],
		MinOnlyMonths:        minMonths,
		MinOnlyOK:            minOK,
		WarnUtilPct:          v.Cfg.WarnUtilizationPct,
		HighUtilPct:          v.Cfg.HighUtilizationPct,
	})
}

// --- debt-alerts: the "Watch-outs" tile -----------------------------------------

// debtAlertsWidget renders the debtcoach alerts as a ranked, glanceable list: a
// severity-banded rail, a plain-English headline, why it matters, and a link to the
// place you'd act on it. When nothing fires it shows a calm all-clear instead of a
// blank tile — the "you're on top of this" read is itself worth showing.
func debtAlertsWidget(props debtPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	alerts := coachAlerts(app)

	var body ui.Node
	if len(alerts) == 0 {
		body = Div(css.Class("debt-allclear"), Attr("data-testid", "debt-allclear"),
			Span(css.Class("debt-allclear-icon"), Attr("aria-hidden", "true"),
				uiw.Icon(icon.CheckCircle, css.Class(tw.W5, tw.H5))),
			Div(
				Div(css.Class("debt-allclear-title", tw.FontDisplay), uistate.T("debt.alerts.allClearTitle")),
				P(css.Class("debt-allclear-text", tw.TextDim), uistate.T("debt.alerts.allClearBody")),
			),
		)
	} else {
		body = Div(css.Class("debt-alerts"), Attr("role", "list"),
			MapKeyed(alerts,
				func(a debtcoach.Alert) any { return a.Kind },
				func(a debtcoach.Alert) ui.Node { return debtAlertRow(a, base) },
			),
		)
	}

	sec := debtSection("sec-watchouts", uistate.T("debt.alerts.title"), nil, body)
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-alerts", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: sec,
	})
}

// debtAlertRow renders one watch-out. No hooks (the action is a plain anchor), so it
// is safe to map over a variable-length alert list.
func debtAlertRow(a debtcoach.Alert, base string) ui.Node {
	title, text := alertCopy(a, base)
	sev := a.Severity.String()
	var glyph = icon.AlertCircle
	switch a.Severity {
	case debtcoach.Critical:
		glyph = icon.AlertTriangle
	case debtcoach.Info:
		glyph = icon.Sparkles
	}
	return Div(ClassStr("debt-alert debt-alert-"+sev), Attr("role", "listitem"),
		Attr("data-testid", "debt-alert-"+a.Kind),
		Div(css.Class("debt-alert-rail"), Attr("aria-hidden", "true")),
		Span(css.Class("debt-alert-icon"), Attr("aria-hidden", "true"),
			uiw.Icon(glyph, css.Class(tw.W4, tw.H4))),
		Div(css.Class("debt-alert-body"),
			Div(css.Class("debt-alert-title"), title),
			P(css.Class("debt-alert-text", tw.TextDim), text),
			alertAction(a),
		),
	)
}

// alertCopy maps an alert kind to its headline and explanation, formatting the
// numeric fields the copy needs. All wording lives in the i18n catalog.
func alertCopy(a debtcoach.Alert, base string) (title, text string) {
	amt := func() string { return fmtMoney(money.New(a.Amount, base)) }
	more := ""
	if a.Count > 1 {
		more = " " + uistate.T("debt.alerts.more", a.Count-1)
	}
	switch a.Kind {
	case "over-limit":
		return uistate.T("debt.alert.overLimit.title"),
			uistate.T("debt.alert.overLimit.body", a.Subject, a.Pct) + more
	case "min-underwater":
		return uistate.T("debt.alert.underwater.title"),
			uistate.T("debt.alert.underwater.body", a.Subject) + more
	case "utilization-high":
		return uistate.T("debt.alert.utilHigh.title"),
			uistate.T("debt.alert.utilHigh.body", fmt.Sprintf("%.0f", a.Pct))
	case "utilization-warn":
		return uistate.T("debt.alert.utilWarn.title"),
			uistate.T("debt.alert.utilWarn.body", fmt.Sprintf("%.0f", a.Pct))
	case "debt-over-assets":
		return uistate.T("debt.alert.overAssets.title"),
			uistate.T("debt.alert.overAssets.body", fmt.Sprintf("%.0f", a.Pct))
	case "high-apr":
		return uistate.T("debt.alert.highApr.title"),
			uistate.T("debt.alert.highApr.body", a.Subject, a.Pct) + more
	case "interest-heavy":
		return uistate.T("debt.alert.interestHeavy.title"),
			uistate.T("debt.alert.interestHeavy.body", fmt.Sprintf("%.0f", a.Pct), amt())
	case "slow-payoff":
		return uistate.T("debt.alert.slow.title"),
			uistate.T("debt.alert.slow.body", fmtMonthsHuman(a.Months))
	}
	return a.Kind, ""
}

// alertAction returns the "go fix it" link for an alert, or an empty node when the
// fix is right here on the page (e.g. the slow-payoff nudge points at the tuner).
func alertAction(a debtcoach.Alert) ui.Node {
	switch a.Kind {
	case "over-limit", "utilization-high", "utilization-warn":
		return debtOwnerLink("/accounts", uistate.T("debt.linkCards"))
	case "min-underwater", "interest-heavy":
		return debtOwnerLink("/allocate", uistate.T("debt.linkAllocate"))
	case "debt-over-assets":
		return debtOwnerLink("/networth", uistate.T("debt.linkNetWorth"))
	}
	return Fragment()
}

// --- debt-tuner: the interactive strategy tuner ---------------------------------

// debtTunerWidget is the page's control surface: pick a payoff method and set an
// extra monthly payment, and the whole page — the ladder order, the summary's
// debt-free date, the burn-down — recomputes to match, because those all read the
// same persisted DebtConfig this tile writes. A live readout shows the resulting
// plan and what the extra buys versus paying only the minimums.
func debtTunerWidget(props debtPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	dec := currency.Decimals(base)

	// $25 in this currency's minor units — the stepper increment.
	step := int64(25)
	for i := 0; i < dec; i++ {
		step *= 10
	}

	cfg := uistate.DebtConfigGet()

	// Handlers sit at fixed positions (the control set is fixed, never a loop), each
	// re-reading the config so concurrent edits don't clobber. Every write persists
	// (SetDebtConfig flushes) and bumps the data revision so the ladder/summary redraw.
	commit := func(mut func(*uistate.DebtConfig)) {
		c := uistate.DebtConfigGet()
		mut(&c)
		if c.DefaultExtraMinor < 0 {
			c.DefaultExtraMinor = 0
		}
		uistate.SetDebtConfig(c)
		uistate.BumpDataRevision()
	}
	setSnow := ui.UseEvent(Prevent(func() { commit(func(c *uistate.DebtConfig) { c.DefaultStrategy = "snowball" }) }))
	setAval := ui.UseEvent(Prevent(func() { commit(func(c *uistate.DebtConfig) { c.DefaultStrategy = "avalanche" }) }))
	incExtra := ui.UseEvent(Prevent(func() { commit(func(c *uistate.DebtConfig) { c.DefaultExtraMinor += step }) }))
	decExtra := ui.UseEvent(Prevent(func() { commit(func(c *uistate.DebtConfig) { c.DefaultExtraMinor -= step }) }))
	clearExtra := ui.UseEvent(Prevent(func() { commit(func(c *uistate.DebtConfig) { c.DefaultExtraMinor = 0 }) }))
	suggestExtra := ui.UseEvent(Prevent(func() {
		s := payoff.SuggestedExtra(baseDebts(app, base))
		commit(func(c *uistate.DebtConfig) { c.DefaultExtraMinor = s })
	}))
	onExactExtra := ui.UseEvent(func(v string) {
		minor, err := money.ParseMinor(strings.TrimSpace(v), dec)
		if err != nil {
			return
		}
		commit(func(c *uistate.DebtConfig) { c.DefaultExtraMinor = minor })
	})

	isSnow := cfg.DefaultStrategy == "snowball"
	segSnow := "seg-btn"
	segAval := "seg-btn"
	if isSnow {
		segSnow += " is-active"
	} else {
		segAval += " is-active"
	}

	// Segmented method picker with a one-line "what this means" under each choice.
	methodPick := Div(css.Class("debt-tuner-block"),
		Span(css.Class("debt-tuner-label", tw.TextDim), uistate.T("debt.tuner.methodLabel")),
		Div(css.Class("seg"), Attr("role", "group"), Attr("aria-label", uistate.T("debt.tuner.methodLabel")),
			Button(ClassStr(segSnow), Type("button"), Attr("aria-pressed", ariaBool(isSnow)),
				Attr("data-testid", "debt-tuner-snowball"), OnClick(setSnow),
				Span(css.Class("seg-btn-title"), uistate.T("planning.snowball")),
				Span(css.Class("seg-btn-sub"), uistate.T("debt.tuner.snowballSub"))),
			Button(ClassStr(segAval), Type("button"), Attr("aria-pressed", ariaBool(!isSnow)),
				Attr("data-testid", "debt-tuner-avalanche"), OnClick(setAval),
				Span(css.Class("seg-btn-title"), uistate.T("planning.avalanche")),
				Span(css.Class("seg-btn-sub"), uistate.T("debt.tuner.avalancheSub"))),
		),
	)

	// Extra-payment stepper: a big tappable −/value/+ with an exact field and quick
	// "suggested"/"clear" chips beside it.
	extraStr := money.FormatMinor(cfg.DefaultExtraMinor, dec)
	extraPick := Div(css.Class("debt-tuner-block"),
		Span(css.Class("debt-tuner-label", tw.TextDim), uistate.T("debt.tuner.extraLabel", base)),
		Div(css.Class("debt-stepper"),
			Button(css.Class("debt-step-btn"), Type("button"), Attr("data-testid", "debt-extra-dec"),
				Attr("aria-label", uistate.T("debt.tuner.decrease")), OnClick(decExtra), "−"),
			Input(css.Class("field debt-step-input"), Type("number"), Attr("min", "0"), Step("1"),
				Attr("data-testid", "debt-extra-input"), Attr("aria-label", uistate.T("debt.tuner.extraLabel", base)),
				Value(extraStr), OnChange(onExactExtra)),
			Button(css.Class("debt-step-btn"), Type("button"), Attr("data-testid", "debt-extra-inc"),
				Attr("aria-label", uistate.T("debt.tuner.increase")), OnClick(incExtra), "+"),
		),
		Div(css.Class("debt-tuner-chips"),
			Button(css.Class("chip-btn"), Type("button"), Attr("data-testid", "debt-extra-suggest"),
				Title(uistate.T("planning.fillSensibleTitle")), OnClick(suggestExtra), uistate.T("debt.tuner.suggest")),
			If(cfg.DefaultExtraMinor > 0,
				Button(css.Class("chip-btn"), Type("button"), Attr("data-testid", "debt-extra-clear"),
					OnClick(clearExtra), uistate.T("debt.tuner.clear"))),
		),
	)

	body := debtSection("sec-tuner", uistate.T("debt.tuner.title"),
		debtOwnerLink("/allocate", uistate.T("debt.linkAllocate")),
		Fragment(
			P(css.Class("muted"), uistate.T("debt.tuner.hint")),
			Div(css.Class("debt-tuner-grid"), methodPick, extraPick),
			debtTunerReadout(app, base, cfg),
		))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-tuner", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// debtTunerReadout renders the live outcome of the tuned plan: the debt-free date,
// months, and total interest, plus what the extra payment buys versus minimums only.
func debtTunerReadout(app *appstate.App, base string, cfg uistate.DebtConfig) ui.Node {
	debts := baseDebts(app, base)
	plan, ok := payoff.BuildPlan(debts, cfg.DefaultExtraMinor, strategyFromConfig(cfg))
	if !ok || len(debts) == 0 {
		return Fragment()
	}
	freeDate := payoff.DebtFreeMonth(time.Now(), plan.Months).Format("Jan 2006")

	stats := Div(css.Class("debt-tuner-stats"),
		tunerStat(uistate.T("debt.debtFreeLabel"), freeDate),
		tunerStat(uistate.T("debt.tuner.timeToFree"), fmtMonthsHuman(plan.Months)),
		tunerStat(uistate.T("debt.totalInterestLabel"), fmtMoney(money.New(plan.TotalInterest, base))),
	)

	// The payoff of the extra: only meaningful once there IS an extra and minimums
	// alone also clear the debt (so there's a baseline to beat).
	var impact ui.Node = Fragment()
	if cfg.DefaultExtraMinor > 0 {
		if minPlan, okMin := payoff.BuildPlan(debts, 0, strategyFromConfig(cfg)); okMin {
			monthsSaved := minPlan.Months - plan.Months
			interestSaved := minPlan.TotalInterest - plan.TotalInterest
			if monthsSaved > 0 || interestSaved > 0 {
				impact = P(css.Class("debt-tuner-impact"), Attr("data-testid", "debt-tuner-impact"),
					uistate.T("debt.tuner.impact", fmtMonthsHuman(monthsSaved), fmtMoney(money.New(interestSaved, base))))
			}
		}
	} else {
		impact = P(css.Class("debt-tuner-impact muted"), uistate.T("debt.tuner.addExtraHint"))
	}
	return Fragment(stats, impact)
}

// tunerStat is one label/value pair in the readout row.
func tunerStat(label, value string) ui.Node {
	return Div(css.Class("debt-stat"),
		Div(css.Class("debt-stat-label", tw.TextDim), label),
		Div(css.Class("debt-stat-value", tw.FontDisplay), value),
	)
}

// --- debt-learn: the teaching accordion -----------------------------------------

// debtLearnWidget is the "understand debt" panel: native disclosure cards that
// explain the strategies, the minimum-payment trap, utilization, the order to
// tackle debt, and when consolidating helps. Native <details> so it's keyboard-
// and screen-reader-friendly with no extra hook state. The first card is open.
func debtLearnWidget(props debtPanelProps) ui.Node {
	learn := func(id, q, aKey string, open bool) ui.Node {
		args := []any{css.Class("debt-learn-item"), Attr("data-testid", "debt-learn-"+id)}
		if open {
			args = append(args, Open(true))
		}
		args = append(args,
			Summary(css.Class("debt-learn-q"), Span(q)),
			P(css.Class("debt-learn-a", tw.TextDim), uistate.T(aKey)),
		)
		return Details(args...)
	}
	body := debtSection("sec-learn", uistate.T("debt.learn.title"), nil, Fragment(
		P(css.Class("muted"), uistate.T("debt.learn.hint")),
		Div(css.Class("debt-learn"),
			learn("methods", uistate.T("debt.learn.methodsQ"), "debt.learn.methodsA", true),
			learn("trap", uistate.T("debt.learn.trapQ"), "debt.learn.trapA", false),
			learn("utilization", uistate.T("debt.learn.utilQ"), "debt.learn.utilA", false),
			learn("order", uistate.T("debt.learn.orderQ"), "debt.learn.orderA", false),
			learn("consolidate", uistate.T("debt.learn.consolidateQ"), "debt.learn.consolidateA", false),
		),
	))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-learn", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- shared helpers -------------------------------------------------------------

// fmtMonthsHuman renders a month count compactly (e.g. "3 yr 4 mo", "8 mo"), routed
// through the i18n catalog so no copy is hardcoded in view code.
func fmtMonthsHuman(months int) string {
	if months <= 0 {
		return uistate.T("debt.dur.now")
	}
	y, m := months/12, months%12
	switch {
	case y > 0 && m > 0:
		return uistate.T("debt.dur.yearsMonths", y, m)
	case y > 0:
		return uistate.T("debt.dur.years", y)
	default:
		return uistate.T("debt.dur.months", m)
	}
}
