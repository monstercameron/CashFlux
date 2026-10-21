// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/credithealth"
	"github.com/monstercameron/CashFlux/internal/creditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// buildUtilTrend acquires fallible sources; native credithealth owns selection,
// currencies, checked arithmetic and point availability.
func buildUtilTrend(app *appstate.App, accountID string) credithealth.UtilizationHistory {
	unavailable := func(err error) credithealth.UtilizationHistory { return credithealth.UtilizationHistory{Err: err} }
	if app == nil {
		return unavailable(fmt.Errorf("credit history: app is not ready"))
	}
	account, found, err := app.Store().GetAccount(accountID)
	if err != nil {
		return unavailable(fmt.Errorf("credit history: read account: %w", err))
	}
	if !found {
		return unavailable(fmt.Errorf("credit history: account %q is unavailable", accountID))
	}
	if account.CreditLimit.Amount == 0 {
		return credithealth.UtilizationHistory{MissingLimit: true}
	}
	snapshots, err := app.Store().ListBalanceSnapshots(accountID)
	if err != nil {
		return unavailable(fmt.Errorf("credit history: read snapshots: %w", err))
	}
	settings, err := app.Store().GetSettings()
	if err != nil {
		return unavailable(fmt.Errorf("credit history: read rates: %w", err))
	}
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return credithealth.BuildUtilizationHistory(account, snapshots, currency.Rates{Base: base, Rates: settings.FXRates})
}

// creditHistoryNode formats native facts. Unknown points retain their dated row
// and never receive a zero-width bar or a fabricated percentage.
func creditHistoryNode(trend credithealth.UtilizationHistory) ui.Node {
	if trend.MissingLimit {
		return Fragment()
	}
	if trend.Err == nil && !trend.Ready {
		return P(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin-top": "0.5rem"}), uistate.T("credit.trendNoHistory"))
	}
	rows := []any{css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Style(map[string]string{"margin-top": "0.5rem"}), Attr("data-testid", "credit-history"),
		P(css.Class("t-caption", tw.TextDim), uistate.T("credit.trendTitle")),
		P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "credit-history-basis"), uistate.T("credit.historyBasis"))}
	if trend.Err != nil {
		rows = append(rows, P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "credit-history-unavailable"), Attr("role", "status"), uistate.T("credit.historyUnavailable")))
	}
	if trend.AssumedAccountCurrency {
		rows = append(rows, P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.historyAssumedCurrency")))
	}
	if trend.Total > len(trend.Points) && len(trend.Points) > 0 {
		rows = append(rows, P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.historyLatest", len(trend.Points), trend.Total)))
	}
	rows = append(rows, MapKeyed(trend.Points, func(point credithealth.UtilizationHistoryPoint) any { return point.ID }, func(point credithealth.UtilizationHistoryPoint) ui.Node {
		date := Span(css.Class("t-caption", tw.TextFaint, "util-trend-date"), Style(map[string]string{"min-width": "4.5rem"}), point.AsOf.Format("Jan 2"))
		if point.Err != nil {
			return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("data-testid", "credit-history-point-"+point.ID), date, Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "credit-history-gap-"+point.ID), uistate.T("credit.historyPointUnavailable")))
		}
		return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Attr("data-testid", "credit-history-point-"+point.ID), date,
			Div(css.Class("util-trend-track"), Style(map[string]string{"flex": "1", "background": "var(--border)", "border-radius": "2px", "height": "6px", "overflow": "hidden"}),
				Div(css.Class(creditUtilBarTone(point.Band), "util-trend-bar"), Attr("data-testid", "credit-history-bar-"+point.ID), Style(map[string]string{"width": fmt.Sprintf("%d%%", point.FillPercent), "height": "100%", "border-radius": "2px"}))),
			Span(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"min-width": "2.5rem", "text-align": "right"}), fmt.Sprintf("%d%%", point.UtilPct)))
	}))
	return Div(rows...)
}

// creditBandTone maps a credithealth.Band to a semantic tone class.
//
// Only Excellent and Good are green (C354). A 55 used to be Good, so a score
// dragged down 38 points by utilization wore the same green ring as a healthy
// one — and the /health page's own 73 wore that ring too, which made the colour
// mean nothing.
func creditBandTone(b credithealth.Band) string {
	switch b {
	case credithealth.BandExcellent, credithealth.BandGood:
		return "text-up"
	case credithealth.BandFair, credithealth.BandNeedsWork:
		return "text-warn"
	default:
		return "text-down"
	}
}

// creditUtilBandTone maps a per-card utilization band to a semantic tone class.
func creditUtilBandTone(b credithealth.UtilBand) string {
	switch b {
	case credithealth.BandUtilBest:
		return "text-up"
	case credithealth.BandUtilGood:
		return "text-up"
	case credithealth.BandUtilFair:
		return "text-warn"
	case credithealth.BandUtilPoor, credithealth.BandUtilWorst:
		return "text-down"
	default:
		return "text-dim"
	}
}

// creditUtilBarTone maps a per-card utilization band to a progress-bar tone.
func creditUtilBarTone(b credithealth.UtilBand) string {
	switch b {
	case credithealth.BandUtilBest, credithealth.BandUtilGood:
		return "bg-up"
	case credithealth.BandUtilFair:
		return "bg-warn"
	default:
		return "bg-down"
	}
}

// creditHue maps a 0–100 proxy score to a continuous red→green hue (HSL),
// mirroring the approach in healthHue.
func creditHue(score int) int {
	// Anchored to the band thresholds (C354) rather than to a flat ×1.3 ramp,
	// which tinted a 55 nearly green while its band said Fair. 0 → red (0°),
	// 40 → amber (45°), 60 → yellow-green (75°), 100 → green (130°).
	switch {
	case score <= 0:
		return 0
	case score < 40:
		return score * 45 / 40
	case score < 60:
		return 45 + (score-40)*30/20
	case score >= 100:
		return 130
	default:
		return 75 + (score-60)*55/40
	}
}

// creditScoreRing renders known scores only. Unknown arithmetic/source facts
// receive an explicit status instead of a zero-valued gauge or severity band.
func creditScoreRing(r creditview.Model, size int) ui.Node {
	if !r.ProxyKnown {
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1), Attr("data-testid", "credit-score-unavailable"), Attr("role", "status"),
			P(css.Class("t-body", tw.TextDim), uistate.T("credit.modelUnavailable")))
	}
	color := fmt.Sprintf("hsl(%d, 64%%, 52%%)", creditHue(r.ProxyScore))
	center := Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(creditBandTone(r.Band))),
		Style(map[string]string{"font-size": fmt.Sprintf("%dpx", size/3)}), Attr("data-testid", "credit-score"), fmt.Sprintf("%d", r.ProxyScore))
	return scoreRingNode(float64(r.ProxyScore), color, size, uistate.T("credit.ringLabel", r.ProxyScore, string(r.Band)), center,
		Div(css.Class("t-caption", tw.TextFaint), uistate.T("credit.outOf100")))
}

func creditAggregateLabel(r creditview.Model) string {
	if r.UtilErr != nil {
		return uistate.T("credit.modelUnavailable")
	}
	if r.UtilPct < 0 {
		return uistate.T("credit.noLimit")
	}
	return fmt.Sprintf("%d%%", r.UtilPct)
}

func creditProvenance(r creditview.Model) ui.Node {
	base := r.Base
	if base == "" {
		base = uistate.T("credit.modelUnavailable")
	}
	return P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "credit-provenance"), uistate.T("credit.modelBasis", base, r.AsOf.Format("Jan 2, 2006")))
}

func creditSourceFailure() ui.Node {
	return uiw.Card(uiw.CardProps{TestID: "credit-source-unavailable", Body: Fragment(
		P(css.Class("t-body", tw.TextDim), Attr("role", "status"), uistate.T("credit.modelSourceUnavailable")),
		P(css.Class("t-caption", tw.TextFaint), credithealth.Disclaimer))})
}

type creditCardProps struct {
	Card creditview.Card
	App  *appstate.App
}

// creditCard owns the interactive row, keyed by stable account identity. All
// amounts, ratios, bands, target payments and widths are prepared native facts.
func creditCard(p creditCardProps) ui.Node {
	c := p.Card
	label := uistate.T("credit.modelUnavailable")
	if c.NoLimit {
		label = uistate.T("credit.noLimit")
	} else if c.UtilKnown {
		label = fmt.Sprintf("%d%%", c.UtilPct)
	}
	var bar ui.Node = Fragment()
	var bandLabel ui.Node = Fragment()
	if c.UtilKnown && !c.NoLimit {
		bar = uiw.ProgressBar(uiw.ProgressBarProps{Percent: c.Meter, Tone: creditUtilBarTone(c.Band)})
		bandLabel = Span(ClassStr("t-caption "+tw.ColorClass(creditUtilBandTone(c.Band))), string(c.Band))
	}
	owed := uistate.T("credit.modelUnavailable")
	if c.OwedErr == nil {
		owed = fmtMoney(c.Owed)
	}
	limit := fmtMoney(c.Limit)
	if !currency.Valid(c.Limit.Currency) {
		limit = uistate.T("credit.modelUnavailable")
	}
	var nudge ui.Node = Fragment()
	if c.HasPayment {
		nudge = P(css.Class("t-caption", tw.TextDim, tw.Mt1), Attr("data-testid", "credit-card-payment-"+c.AccountID),
			uistate.T("credit.payToReach30Prefix"), Span(css.Class("t-caption", tw.FontMedium), fmtMoney(c.Target30)), uistate.T("credit.payToReach30Suffix"))
	}
	save := func(id, accountCurrency string, limit money.Money) error {
		return saveCreditLimit(p.App, id, accountCurrency, limit)
	}
	return Div(css.Class("credit-card-item", tw.Flex, tw.FlexCol, tw.Gap1), Attr("data-testid", "credit-card-"+c.AccountID),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween), Span(css.Class("t-body", tw.FontMedium), c.Name),
			Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2), Span(css.Class("t-body", tw.TextDim), Attr("data-testid", "credit-card-util-"+c.AccountID), label), bandLabel)),
		bar, P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "credit-card-balances-"+c.AccountID), uistate.T("credit.balanceOfLimit", owed, limit)), nudge,
		creditHistoryNode(c.History),
		// Keep the editor's draft identity when conditional payment/history
		// siblings appear or disappear before it in the flattened child list.
		WithKey(Details(css.Class("hlt-curve"), Summary(Attr("aria-label", uistate.T("debt.creditLimitAria", c.Name)), uistate.T("credit.editLimit")),
			ui.CreateElement(creditLimitEditor, creditLimitEditorProps{AccountID: c.AccountID, AccountCurrency: c.Balance.Currency, Currency: c.EditorCurrency, LimitMinor: c.Limit.Amount, OnSave: save})), "credit-limit-"+c.AccountID))
}

func creditCards(r creditview.Model, app *appstate.App) ui.Node {
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4, tw.Mt3), MapKeyed(r.Cards, func(c creditview.Card) any { return c.AccountID }, func(c creditview.Card) ui.Node {
		return ui.CreateElement(creditCard, creditCardProps{Card: c, App: app})
	}))
}

type creditLimitEditorProps struct {
	AccountID, AccountCurrency, Currency string
	LimitMinor                           int64
	OnSave                               func(string, string, money.Money) error
}

func creditLimitEditor(p creditLimitEditorProps) ui.Node {
	init := ""
	if p.LimitMinor > 0 {
		init = money.FormatMinor(p.LimitMinor, currency.Decimals(p.Currency))
	}
	val := ui.UseState(init)
	origin := ui.UseState([2]string{p.AccountCurrency, p.Currency})
	saved := ui.UseState(false)
	failed := ui.UseState(false)
	onInput := ui.UseEvent(func(v string) { val.Set(v); saved.Set(false); failed.Set(false) })
	commit := func() {
		identity := origin.Get()
		amount, err := money.ParseMinor(strings.TrimSpace(val.Get()), currency.Decimals(identity[1]))
		if err == nil && amount >= 0 && p.OnSave != nil && identity == [2]string{p.AccountCurrency, p.Currency} {
			err = p.OnSave(p.AccountID, identity[0], money.New(amount, identity[1]))
		} else {
			err = fmt.Errorf("invalid credit limit")
		}
		saved.Set(err == nil)
		failed.Set(err != nil)
	}
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt1),
		Span(css.Class("t-caption", tw.TextFaint), uistate.T("credit.limitLabel")),
		Span(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "credit-limit-currency-"+p.AccountID), origin.Get()[1]),
		Input(css.Class("field"), Type("number"), Attr("min", "0"), Step("any"), Attr("data-testid", "credit-limit-edit-"+p.AccountID),
			Attr("aria-label", uistate.T("credit.limitEditAria", p.AccountID)), Placeholder(uistate.T("credit.limitPlaceholder")), OnInput(onInput), OnBlur(func() { commit() }), uiw.FieldValue(val.Get())),
		If(saved.Get(), Span(css.Class("t-caption", tw.TextUp), Attr("role", "status"), uistate.T("credit.limitSaved"))),
		If(failed.Get(), Span(css.Class("t-caption", tw.TextDim), Attr("role", "status"), Attr("data-testid", "credit-limit-error-"+p.AccountID), uistate.T("credit.modelLimitSaveFailed"))))
}

// CreditHealthPanelProps selects the embedded hand-off or complete breakdown.
type CreditHealthPanelProps struct{ Summary bool }

// CreditHealthPanel shares the same native preparation as /credit and AI.
// A summary returns before constructing cards, editors, advice or AI children.
func CreditHealthPanel(p CreditHealthPanelProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	r := prepareCredit(app, time.Now(), !p.Summary, !p.Summary)
	if r.Fatal {
		return creditSourceFailure()
	}
	if r.Empty {
		return uiw.Card(uiw.CardProps{Body: Fragment(P(css.Class("empty"), uistate.T("credit.emptyBody")), P(css.Class("t-caption", tw.TextFaint), r.Disclaimer))})
	}
	hero := uiw.Card(uiw.CardProps{Body: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5), creditScoreRing(r, 150),
		Div(css.Class(tw.Flex1), If(r.ProxyKnown, Div(css.Class("t-figure-lg"), string(r.Band))),
			P(css.Class("t-caption", tw.TextFaint), Attr("data-testid", "credit-estimate-note"), uistate.T("detail6.creditScoreLabel")),
			P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "credit-aggregate-util"), uistate.T("credit.aggUtil", creditAggregateLabel(r)))))})
	if p.Summary {
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap3), hero, creditProvenance(r),
			P(css.Class("t-caption"), Attr("data-testid", "credit-handoff"), A(Href(uistate.RoutePath("/credit")), uistate.T("credit.seeFullPanel"))), P(css.Class("t-caption", tw.TextFaint), r.Disclaimer))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5), hero, creditProvenance(r),
		uiw.Card(uiw.CardProps{Title: uistate.T("credit.demeritsTitle"), TestID: "credit-demerits", Body: creditIssues(r, false)}),
		uiw.Card(uiw.CardProps{Title: uistate.T("credit.improveTitle"), TestID: "credit-advice", Body: creditIssues(r, true)}),
		ui.CreateElement(creditAINode), uiw.Card(uiw.CardProps{Title: uistate.T("credit.utilTitle"), Body: creditCards(r, app)}),
		creditMissingNote(r), P(css.Class("t-caption", tw.TextFaint), r.Disclaimer))
}

func creditMissingNote(r creditview.Model) ui.Node {
	if r.CardsMissingLimit == 0 {
		return Fragment()
	}
	return P(css.Class("muted", tw.Mt2), uistate.T("credit.missingLimitNote", r.CardsMissingLimit))
}

func creditDemeritText(d credithealth.CheckedDemerit) string {
	switch d.Kind {
	case credithealth.DemeritUtilization:
		return uistate.T("credit.demeritUtil", d.Pct)
	case credithealth.DemeritCardUtil:
		return uistate.T("credit.demeritCardUtil", d.Name, d.Pct)
	case credithealth.DemeritOnTime:
		return uistate.T("credit.demeritOnTime")
	case credithealth.DemeritAge:
		return uistate.T("credit.demeritAge")
	case credithealth.DemeritOverLimit:
		return uistate.T("credit.demeritOverLimit", d.Name, d.Pct)
	case credithealth.DemeritMissingLimit:
		return uistate.T("credit.demeritMissingLimit", d.Pct)
	}
	return ""
}

func creditAdviceText(a credithealth.CheckedAdvice) string {
	switch a.Kind {
	case credithealth.AdviceLowerUtilization:
		return uistate.T("credit.adviceLowerUtil", fmtMoney(a.Pay), a.Name, a.TargetPct)
	case credithealth.AdviceOnTime:
		return uistate.T("credit.adviceOnTime")
	case credithealth.AdviceAddLimit:
		return uistate.T("credit.adviceAddLimit", a.Name)
	}
	return ""
}

// creditContextString includes the same checked facts as visual hosts. Unknown
// values are identified explicitly; suggestions retain their native currencies.
func creditContextString(app *appstate.App) string {
	r := prepareCredit(app, time.Now(), true, false)
	var b strings.Builder
	fmt.Fprintf(&b, "Scope: %s; base currency: %s; as of: %s; current saved FX rates.\n", r.Scope, r.Base, r.AsOf.Format(time.RFC3339))
	b.WriteString(r.Disclaimer + "\n")
	if r.Fatal {
		b.WriteString("Credit data unavailable.\n")
		return b.String()
	}
	if r.ProxyKnown {
		fmt.Fprintf(&b, "Overall score: %d/100 (%s)\n", r.ProxyScore, r.Band)
	} else {
		b.WriteString("Overall score: unavailable.\n")
	}
	fmt.Fprintf(&b, "Overall utilization: %s\n", creditAggregateLabel(r))
	if r.OnTime.Err != nil {
		b.WriteString("On-time factor: unavailable.\n")
	} else if !r.OnTime.NoData {
		fmt.Fprintf(&b, "On-time factor: %d/100\n", r.OnTime.Score)
	}
	if !r.Age.NoData {
		fmt.Fprintf(&b, "Age factor: %d/100\n", r.Age.Score)
	}
	for _, c := range r.Cards {
		if c.UtilKnown && !c.NoLimit {
			fmt.Fprintf(&b, "- %s [%s]: %d%% utilization\n", c.Name, c.AccountID, c.UtilPct)
		} else if c.NoLimit {
			fmt.Fprintf(&b, "- %s [%s]: no limit set\n", c.Name, c.AccountID)
		} else {
			fmt.Fprintf(&b, "- %s [%s]: utilization unavailable\n", c.Name, c.AccountID)
		}
	}
	for _, d := range r.Demerits {
		fmt.Fprintf(&b, "- %s\n", creditDemeritText(d))
	}
	for _, a := range r.Advice {
		fmt.Fprintf(&b, "- %s", creditAdviceText(a))
		if a.ProjectedKnown {
			fmt.Fprintf(&b, " (projected score %d)", a.ProjectedScore)
		} else if a.Kind == credithealth.AdviceLowerUtilization {
			b.WriteString(" (score impact unavailable)")
		}
		b.WriteByte('\n')
	}
	if r.Incomplete {
		b.WriteString("Some credit facts are unavailable; absent issues are not confirmation that every factor is healthy.\n")
	}
	return b.String()
}

// fmtMinorAmount formats minor-unit integer cents into a comma-grouped decimal
// display string without a symbol (the symbol is prepended by callers) — the
// same grouping every other money figure gets via money.FormatAccounting, so
// /credit, /investments, /loans, and /duplicates figures no longer render
// "$33720.00" beside a "$4,590.56" from the shared formatter (C337).
func fmtMinorAmount(minor int64, decimals int) string {
	if decimals == 0 {
		return money.Group(fmt.Sprintf("%d", minor))
	}
	div := int64(1)
	for i := 0; i < decimals; i++ {
		div *= 10
	}
	whole := minor / div
	frac := minor % div
	if frac < 0 {
		frac = -frac
	}
	return money.Group(fmt.Sprintf("%d.%0*d", whole, decimals, frac))
}

// creditAINode renders the opt-in Smart+ AI credit analysis (SMART-A11): a deeper,
// personalized read of the demerits + advice. It appears only when the feature is enabled
// and an inference provider is configured; enabled-without-provider shows a quiet hint, and
// off shows nothing (opt-in, never a dead control). UsePrefs is called unconditionally so
// hook order is stable if the feature is toggled.
func creditAINode() ui.Node {
	pr := uistate.UsePrefs().Get()
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	const code = "SMART-A11"
	settings := uistate.LoadSmartSettings()
	if !settings.IsEnabled(code) || settings.IsMuted(code) {
		return Fragment()
	}
	backendAI := uistate.BackendConnectionActive(pr)
	if !aiProviderConfigured(app, backendAI) {
		return uiw.Card(uiw.CardProps{Title: uistate.T("credit.aiTitle"), TestID: "credit-ai",
			Body: P(css.Class("t-caption", tw.TextDim), uistate.T("smart.aiNeedsProvider"))})
	}
	f, ok := smart.ByCode(code)
	if !ok {
		return Fragment()
	}
	conn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
	return uiw.Card(uiw.CardProps{Title: uistate.T("credit.aiTitle"), TestID: "credit-ai",
		Body: smartAIFeatureNode(f, conn)})
}

type creditFactorTileProps struct {
	Key, Title, VarName string
	Factor              creditview.Factor
}

func creditFactorTile(p creditFactorTileProps) ui.Node {
	f := p.Factor
	if f.Err != nil {
		return hltSection("sec-cf-"+p.Key, p.Title, nil, P(css.Class("empty"), Attr("data-testid", "cf-unavailable-"+p.Key), uistate.T("credit.modelUnavailable")))
	}
	if f.NoData {
		return hltSection("sec-cf-"+p.Key, p.Title, nil, Fragment(P(css.Class("empty"), Attr("data-testid", "cf-na-"+p.Key), uistate.T("credit.na."+p.Key)), P(css.Class("muted"), uistate.T("credit.f."+p.Key+".why"))))
	}
	status := Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cf-unmet-"+p.Key), uistate.T("credit.f."+p.Key+".room"))
	if f.OnTarget {
		status = Span(css.Class("t-caption", tw.TextUp), Attr("data-testid", "cf-met-"+p.Key), uistate.T("credit.f."+p.Key+".good"))
	}
	return hltSection("sec-cf-"+p.Key, p.Title, nil, Fragment(
		Div(css.Class("hlt-factor-head"), Span(Span(ClassStr("hlt-factor-value "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(healthTextTone(healthBandForScore(f.Score)))), fmt.Sprintf("%d", f.Score)), Span(css.Class("t-caption", tw.TextFaint), " "+uistate.T("credit.outOf100"))), status),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: f.Meter, Tone: healthBarTone(f.Score)}), P(css.Class("muted", tw.Mt2), uistate.T("credit.f."+p.Key+".why")),
		Details(css.Class("hlt-curve"), Summary(uistate.T("health.curveSummary")), P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.f."+p.Key+".curve")), P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", f.Score, f.WeightPercent)),
			Div(css.Class("hlt-varchip"), Attr("data-testid", "cf-var-"+p.Key), Title(uistate.T("health.varChipTitle")), Code(p.VarName), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", f.Score))))))
}

func creditIssues(r creditview.Model, up bool) ui.Node {
	items := []any{css.Class("credit-list")}
	if up {
		for _, a := range r.Advice {
			var chip ui.Node = Fragment()
			if a.ImpactKnown && a.ImpactPts > 0 {
				if a.ProjectedKnown {
					chip = Span(css.Class("credit-pts credit-pts-up"), uistate.T("credit.ptsUpTo", a.ImpactPts, a.ProjectedScore))
				} else {
					chip = Span(css.Class("credit-pts credit-pts-up"), uistate.T("credit.ptsUp", a.ImpactPts))
				}
			}
			items = append(items, Div(css.Class("credit-item"), Span(css.Class("credit-item-icon is-up"), Attr("aria-hidden", "true"), uiw.Icon(icon.ArrowUpCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4))), Span(css.Class("credit-item-text"), creditAdviceText(a)), chip))
		}
	} else {
		for _, d := range r.Demerits {
			var chip ui.Node = Fragment()
			if d.PointsLost > 0 {
				chip = Span(css.Class("credit-pts credit-pts-down"), uistate.T("credit.ptsDown", d.PointsLost))
			}
			items = append(items, Div(css.Class("credit-item"), Span(css.Class("credit-item-icon is-down"), Attr("aria-hidden", "true"), uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4))), Span(css.Class("credit-item-text"), creditDemeritText(d)), chip))
		}
	}
	if r.Incomplete {
		items = append(items, P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "credit-details-incomplete"), uistate.T("credit.modelPartial")))
	} else if len(items) == 1 {
		key := "credit.demeritsEmpty"
		if up {
			key = "credit.adviceEmpty"
		}
		items = append(items, P(css.Class("empty"), uistate.T(key)))
	}
	return Div(items...)
}

func creditUtilization(r creditview.Model, app *appstate.App) ui.Node {
	status := Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cf-unmet-util"), uistate.T("health.target", uistate.T("credit.utilTarget")))
	if r.UtilOnTarget {
		status = Span(css.Class("t-caption", tw.TextUp), Attr("data-testid", "cf-met-util"), uistate.T("health.onTarget", uistate.T("credit.utilTarget")))
	}
	var meter, curve ui.Node = Fragment(), Fragment()
	if r.UtilErr == nil && r.UtilPct >= 0 {
		meter = Div(css.Class("crd-meter-wrap"), uiw.ProgressBar(uiw.ProgressBarProps{Percent: r.UtilMeter, Tone: creditUtilBarTone(r.UtilBand)}), Div(css.Class("crd-target-tick"), Attr("title", uistate.T("credit.utilTarget")), Attr("aria-hidden", "true")))
		curve = Details(css.Class("hlt-curve"), Summary(uistate.T("health.curveSummary")), P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.f.util.curve")), P(css.Class("t-caption", tw.TextFaint), uistate.T("health.scoreDetail", r.UtilScore, r.UtilWeightPercent)),
			Div(css.Class("hlt-varchip"), Attr("data-testid", "cf-var-util"), Title(uistate.T("health.varChipTitle")), Code("credit_util_score"), Span(css.Class(tw.TextDim), fmt.Sprintf(" · %d", r.UtilScore))))
	}
	return hltSection("sec-cf-util", uistate.T("credit.utilTitle"), nil, Fragment(Div(css.Class("hlt-factor-head"), Span(css.Class("hlt-factor-value", tw.FontDisplay), Attr("data-testid", "credit-aggregate-util"), creditAggregateLabel(r)), status), meter,
		P(css.Class("muted", tw.Mt2), uistate.T("credit.f.util.why")), curve, creditCards(r, app), creditMissingNote(r)))
}

type creditMetricsProps struct {
	Shown  bool
	Toggle func()
}

func creditMetricsButton(p creditMetricsProps) ui.Node {
	cls, label := "strip-toggle", uistate.T("credit.metricsShow")
	if p.Shown {
		cls += " is-on"
		label = uistate.T("credit.metricsHide")
	}
	return Button(ClassStr(cls), Type("button"), Attr("aria-pressed", ariaBool(p.Shown)), Attr("data-testid", "credit-toggle-formulas"), Title(uistate.T("credit.metricsTitle")), OnClick(Prevent(p.Toggle)), Text(label))
}

// CreditScreen renders prepared financial facts; the controller owns reads and
// native creditview/credithealth own calculations and independent availability.
func CreditScreen() ui.Node {
	_ = uistate.UseDataRevision().Get()
	showFormulas := ui.UseState(false)
	app := appstate.Default
	r := prepareCredit(app, time.Now(), true, true)
	if r.Fatal {
		return creditSourceFailure()
	}
	if r.Empty {
		return Fragment(ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("credit.emptyBody"), CTALabel: uistate.T("accounts.addFirst"), AddTarget: "account"}), P(css.Class("t-caption", tw.TextFaint), r.Disclaimer))
	}
	scoreFormula := ""
	for _, m := range app.Molecules() {
		if m.Name == "credit_proxy" {
			scoreFormula = m.Formula
			break
		}
	}
	button := ui.CreateElement(creditMetricsButton, creditMetricsProps{Shown: showFormulas.Get(), Toggle: func() { showFormulas.Set(!showFormulas.Get()) }})
	hero := hltTile("crd-hero", "1 / span 4", hltSection("sec-credit-hero", uistate.T("credit.pageTitle"), button, Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5, tw.FlexWrap), creditScoreRing(r, 150), Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.JustifyCenter), If(r.ProxyKnown, Div(css.Class("t-figure-lg", tw.FontDisplay), string(r.Band))),
			Div(css.Class("crd-disclaimer", tw.Mt2), Attr("data-testid", "credit-disclaimer"), uiw.Icon(icon.HelpCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(r.Disclaimer)))), creditProvenance(r),
		If(scoreFormula != "", Details(css.Class("hlt-formula"), Attr("data-testid", "credit-formula"), Summary(css.Class("t-caption", tw.TextDim), uistate.T("health.formulaTitle")), Code(css.Class("hlt-formula-code"), "credit_proxy = "+scoreFormula), P(css.Class("t-caption", tw.TextFaint), uistate.T("credit.formulaNote")))))))
	tiles := []ui.Node{hero, hltTile("crd-util", "1 / span 4", creditUtilization(r, app)),
		hltTile("crd-ontime", "span 2", ui.CreateElement(creditFactorTile, creditFactorTileProps{Key: "ontime", Title: uistate.T("credit.ontimeTitle"), Factor: r.OnTime, VarName: "credit_ontime_score"})),
		hltTile("crd-age", "span 2", ui.CreateElement(creditFactorTile, creditFactorTileProps{Key: "age", Title: uistate.T("credit.ageTitle"), Factor: r.Age, VarName: "credit_age_score"})),
		hltTile("crd-down", "span 2", hltSection("sec-credit-down", uistate.T("credit.demeritsTitle"), nil, creditIssues(r, false))),
		hltTile("crd-up", "span 2", hltSection("sec-credit-up", uistate.T("credit.improveTitle"), nil, creditIssues(r, true))),
		Div(Style(map[string]string{"grid-column": "1 / span 4"}), ui.CreateElement(creditAINode))}
	if showFormulas.Get() {
		tiles = append(tiles, hltTile("crd-formula", "1 / span 4", Fragment(P(css.Class("t-caption", tw.TextDim), uistate.T("credit.formulaHint")), ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("credit.metricsShow"), Initial: "credit_proxy", ShowSaved: true}))))
	}
	return Div(css.Class("bento bento-credit"), tiles)
}
