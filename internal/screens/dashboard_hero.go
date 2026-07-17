// SPDX-License-Identifier: MIT

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
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartai"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// dashboardHero renders the EC4 home band that sits above the bento grid on
// the Dashboard. It delivers an immediate, glanceable summary of the
// household's financial position with no scrolling required.
//
// Two states:
//   - Empty dataset (no accounts, no transactions): a welcoming first-run hero
//     with the app value proposition, a "Load sample" primary CTA, and an
//     "Add your first account" secondary button. This is the commercial first
//     impression — calm, typographic, actionable.
//   - Non-empty dataset: a time-of-day greeting (Good morning/afternoon/evening
//     by local hour), the net-worth hero figure, a compact this-month stats row
//     (income / spending / net / savings rate from the memoized §1.6 selectors),
//     and two quick-action buttons (add transaction, add account).
//
// All text goes through uistate.T for i18n. Every button carries Type("button"),
// aria-label, and is keyboard-reachable (WCAG 2.1.1 / 4.1.2). No business
// logic — pure formatting delegated to the shared format helpers (fmtMoney,
// figTone) and pure-logic functions (ledger.SavingsRate).
func dashboardHero() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get() // re-render after import / load-sample / wipe

	accounts := app.Accounts()
	txns := app.Transactions()

	// Empty dataset → first-run welcome state, rendered by its own component so
	// its hooks occupy stable positions independent of the non-empty path.
	if len(accounts) == 0 && len(txns) == 0 {
		return ui.CreateElement(heroWelcome, struct{}{})
	}

	// Non-empty → compute headline figures from the memoized §1.6 selectors,
	// then delegate to heroSummary so its hooks are at stable positions.
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	nw := useNetWorth(app, accounts, txns, rates)

	w := uistate.UsePeriod().Get()
	start, end := w.Range()
	income, expense := usePeriodTotals(app, txns, start, end, rates, "")
	// The hero sparkline is a trailing 6-month net-worth series over every transaction
	// — heavy, and secondary to the net-worth figure itself. Defer it off the initial
	// mount so the headline number + KPIs paint immediately; the sparkline + delta chip
	// fill in after first paint (they already degrade gracefully when absent).
	sparkReady := useAfterSettle("hero")

	netMoney := money.New(income.Amount-expense.Amount, income.Currency)
	savingsPct := ledger.SavingsRate(income.Amount, expense.Amount)

	// Net display: "+" prefix for surplus, accounting parens for deficit, plain
	// zero for breakeven — mirrors the dashboard cash-flow sub-line style (L4).
	netDisplay := fmtMoney(netMoney)
	if netMoney.Amount > 0 {
		netDisplay = "+" + fmtMoney(netMoney)
	}

	// A trailing 6-month net-worth series powers the hero sparkline, and the change
	// since the start of the current month becomes the delta chip — both turn the
	// flat figure into a living, contextual headline. Both degrade gracefully: if
	// the series can't be computed the spark/chip simply don't render.
	var spark []float64
	var delta, deltaTone string
	if sparkReady {
		spark, delta, deltaTone = heroNetWorthTrend(accounts, txns, rates, nw.Net.Currency)
	}
	// Paged to another period, the "▲ $X this month" chip would sit beside the
	// selected month's income/spending as if it belonged to that month — net
	// worth is position-as-of-today, so the chip says that instead (parity-scan
	// dashboard period contract).
	if now := time.Now(); now.Before(start) || !now.Before(end) {
		delta, deltaTone = uistate.T("dashboard.netWorthAsOfToday"), "text-dim"
	}

	return ui.CreateElement(heroSummary, heroSummaryProps{
		NetWorth:     fmtMoney(nw.Net),
		NetWorthTone: figTone(nw.Net),
		Income:       fmtMoney(income),
		Spending:     fmtMoney(expense),
		Net:          netDisplay,
		NetTone:      figTone(netMoney),
		SavingsPct:   savingsPct,
		Spark:        spark,
		Delta:        delta,
		DeltaTone:    deltaTone,
	})
}

// heroNetWorthTrend builds the hero's trailing 6-month net-worth series (for the
// sparkline) and the month-over-start delta (for the chip). Returns an empty
// series and delta on any computation error so the hero renders cleanly without
// them.
func heroNetWorthTrend(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, base string) (spark []float64, delta, deltaTone string) {
	const months = 6
	now := time.Now()
	cur := dateutil.MonthStart(now)
	cutoffs := make([]time.Time, 0, months+1)
	for i := months - 1; i >= 0; i-- {
		cutoffs = append(cutoffs, dateutil.AddMonths(cur, -i))
	}
	cutoffs = append(cutoffs, now) // current point
	series, err := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	if err != nil || len(series) < 2 {
		return nil, "", ""
	}
	spark = make([]float64, len(series))
	for i, m := range series {
		spark[i] = float64(m.Amount)
	}
	// Delta = change since the start of the current month (the last two points).
	d := series[len(series)-1].Amount - series[len(series)-2].Amount
	arrow, tone := "▲", "text-up"
	mag := d
	switch {
	case d < 0:
		arrow, tone, mag = "▼", "text-down", -d
	case d == 0:
		arrow, tone = "■", "text-dim"
	}
	delta = arrow + " " + fmtMoney(money.New(mag, base)) + " " + uistate.T("home.thisMonth")
	return spark, delta, tone
}

// ---------------------------------------------------------------------------
// Non-empty summary hero
// ---------------------------------------------------------------------------

// heroSummaryProps carries the pre-formatted display values for the non-empty
// hero so the parent can call memoized selectors before rendering the component.
type heroSummaryProps struct {
	NetWorth     string
	NetWorthTone string // color token, e.g. "text-up" or "text-down"
	Income       string
	Spending     string
	Net          string // formatted cash-flow net (income − spending)
	NetTone      string // color token for the net figure
	SavingsPct   int
	Spark        []float64 // trailing 6-month net-worth series for the hero sparkline (nil → no chart)
	Delta        string    // formatted "▲ $X this month" net-worth change ("" → no chip)
	DeltaTone    string    // color token for the delta chip
}

// heroSummary renders the non-empty hero: a time-of-day greeting, the
// net-worth hero figure, a this-month stats row, and quick-action buttons.
func heroSummary(props heroSummaryProps) ui.Node {
	// Greeting keyed on local wall-clock hour — no UTC offset needed here because
	// Go's time.Now() returns wall clock in the process's local timezone.
	greeting := uistate.T("home.greetingEvening")
	h := time.Now().Hour()
	switch {
	case h < 12:
		greeting = uistate.T("home.greetingMorning")
	case h < 17:
		greeting = uistate.T("home.greetingAfternoon")
	}

	// Quick-action handlers — declared unconditionally for stable hook ordering
	// (On*-hooks-in-loops rule: these hooks must be at fixed positions per render).
	openQuickAdd := ui.UseEvent(func() { uistate.SetQuickAdd(true) })
	openAddAccount := ui.UseEvent(func() { uistate.SetAddTarget("account") })

	savingsStr := fmt.Sprintf("%d%%", props.SavingsPct)
	savingsTone := "text-up"
	switch {
	case props.SavingsPct < 0:
		savingsTone = "text-down"
	case props.SavingsPct < 10:
		savingsTone = "text-warn"
	}

	// Optional delta chip ("▲ $X this month") and net-worth sparkline.
	var delta ui.Node = Fragment()
	if props.Delta != "" {
		delta = Span(ClassStr("home-hero-delta "+tw.ColorClass(props.DeltaTone)), props.Delta)
	}
	var spark ui.Node = Fragment()
	if len(props.Spark) >= 2 {
		spark = Div(css.Class("home-hero-spark"),
			uiw.AreaChart(uiw.AreaChartProps{
				Values: props.Spark, GradientID: "hero-spark", Width: 260, Height: 72,
				Label: uistate.T("home.netWorth"),
			}),
		)
	}

	return Div(css.Class("home-hero"),
		// Greeting + a quiet date line for context — H2 because the topbar shell
		// provides the implicit page landmark (WCAG 2.4.6, no level-skip).
		Div(css.Class("home-hero-top"),
			H2(ClassStr("home-hero-greeting "+tw.Fold(tw.FontDisplay, tw.Text28, tw.LeadingTight, tw.TrackingTight)),
				greeting,
			),
			P(css.Class("home-hero-date"), time.Now().Format("Monday, January 2")),
		),

		// Headline row: the net-worth figure + change chip on the left, a living
		// 6-month sparkline filling the right.
		Div(css.Class("home-hero-main"),
			Div(css.Class("home-hero-nw-block"),
				Span(css.Class("home-hero-nw-label"),
					uistate.T("home.netWorth"),
					// Optional smart explainer (gated by the global density dial).
					smartTooltipFor(uistate.LoadSmartSettings(), "networth", uistate.T("home.netWorth"), uistate.T("smart.tipNetWorth")),
				),
				Div(ClassStr("home-hero-nw-fig fig "+
					tw.Fold(tw.FontDisplay)+" "+
					tw.ColorClass(props.NetWorthTone)),
					Attr("data-countup", ""),
					props.NetWorth,
				),
				delta,
			),
			spark,
		),

		// This-month compact stat row (income / spending / net / savings rate).
		Div(css.Class("home-hero-stats"),
			heroStat(uistate.T("home.income"), props.Income, "text-up"),
			heroStat(uistate.T("home.spending"), props.Spending, "text-down"),
			heroStat(uistate.T("home.net"), props.Net, props.NetTone),
			heroStat(uistate.T("home.savingsRate"), savingsStr, savingsTone),
		),

		// Quick actions — the two most common entry points, surfaced without hunting.
		// Standard glyph buttons (leading icon + label, like the page toolbars).
		Div(css.Class("home-hero-actions"),
			Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddTxnAria")),
				Attr("title", uistate.T("home.quickAddTxnAria")),
				Attr("data-testid", "hero-add-txn"),
				OnClick(openQuickAdd),
				uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("home.quickAddTxn")),
			),
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddAccountAria")),
				Attr("title", uistate.T("home.quickAddAccountAria")),
				Attr("data-testid", "hero-add-account"),
				OnClick(openAddAccount),
				uiw.Icon(icon.CreditCard, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("home.quickAddAccount")),
			),
			// Presentation modes: one click swaps the widget set for a moment
			// (daily check-in, payday, month end, debt, goals). The user can
			// still drag/resize afterwards; Settings → Reset restores default.
			ui.CreateElement(dashPresetPicker, struct{}{}),
		),

		// Quote of the day (opt-in AI Smart+ feature) — a calm footer ribbon.
		ui.CreateElement(heroQuote, struct{}{}),
	)
}

// ---------------------------------------------------------------------------
// First-run welcome hero
// ---------------------------------------------------------------------------

// heroWelcome renders the first-run welcome state for an empty dataset. Its
// own component keeps its hooks at positions independent of the non-empty
// heroSummary (the On*-hooks-in-loops rule: different component = different
// hook sequence, which is fine; same-component conditional hooks are not).
func heroWelcome(_ struct{}) ui.Node {
	rev := uistate.UseDataRevision()
	// Load sample: calls app.LoadSample(), bumps the data revision so the
	// dashboard re-reads the now-populated store, and marks the sample banner
	// active. Mirrors the same action in accounts.go and the Settings panel.
	onLoadSample := ui.UseEvent(func() {
		app := appstate.Default
		if app == nil {
			return
		}
		if err := app.LoadSample(); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		rev.Set(rev.Get() + 1)
		uistate.SetSampleActive(true)
		// Flush immediately so a reload within the autosave tick can't lose the
		// freshly loaded sample (C2).
		uistate.RequestPersist()
	})
	onAddAccount := ui.UseEvent(func() { uistate.SetAddTarget("account") })

	return Div(css.Class("home-hero home-hero--welcome"),
		// Accessible band heading.
		H2(ClassStr("home-hero-welcome-title "+
			tw.Fold(tw.FontDisplay, tw.Text28, tw.LeadingTight, tw.TrackingTight)),
			uistate.T("home.welcomeTitle"),
		),
		// One-line value prop — the commercial first impression.
		P(css.Class("home-hero-welcome-body t-body", tw.TextDim, tw.Mt2),
			uistate.T("home.welcomeBody"),
		),
		Div(css.Class("home-hero-actions", tw.Mt3),
			// Primary CTA: load sample data — fastest path to a populated dashboard.
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("aria-label", uistate.T("home.loadSampleAria")),
				Attr("title", uistate.T("home.loadSampleAria")),
				Attr("data-testid", "hero-load-sample"),
				OnClick(onLoadSample),
				uistate.T("home.loadSample"),
			),
			// Secondary: start entering real data immediately.
			Button(css.Class("btn"), Type("button"),
				Attr("aria-label", uistate.T("home.addFirstAria")),
				Attr("title", uistate.T("home.addFirstAria")),
				Attr("data-testid", "hero-add-first-account"),
				OnClick(onAddAccount),
				uistate.T("home.addFirst"),
			),
		),
	)
}

// ---------------------------------------------------------------------------
// Shared helper
// ---------------------------------------------------------------------------

// heroStat renders one labelled figure in the this-month stats row.
// tone is a color-token string accepted by tw.ColorClass (e.g. "text-up",
// "text-down", "text-warn", "text-dim", or "" for the default foreground).
func heroStat(label, value, tone string) ui.Node {
	valueCls := "home-hero-stat-value fig " + tw.Fold(tw.FontDisplay) + " " + tw.ColorClass(tone)
	return Div(css.Class("home-hero-stat"),
		Span(css.Class("home-hero-stat-label t-caption", tw.TextFaint), label),
		Span(ClassStr(valueCls), Attr("data-countup", ""), value),
	)
}

// ---------------------------------------------------------------------------
// Quote of the day (SMART-QUOTE) — an opt-in AI Smart+ feature
// ---------------------------------------------------------------------------

// quoteCode is the SMART series feature code for the daily quote.
const quoteCode = "SMART-QUOTE"

// quoteThemes are the selectable styles. The value is sent verbatim to the model
// as the requested theme, and shown as the option label.
var quoteThemes = []string{"Stoic", "Mindful", "Playful", "Poetic", "Practical", "Bold", "Witty"}

// cleanQuote strips wrapping whitespace and any surrounding quotation marks the
// model may add, so the displayed line is just the quote.
func cleanQuote(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'“”‘’")
	return strings.TrimSpace(s)
}

func quoteSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// splitQuote separates a "<quote> — <author>" line into its text and attribution.
// It splits on the LAST dash separator so a dash inside the quote doesn't steal
// the author. Returns an empty author when no separator is present.
func splitQuote(s string) (quote, author string) {
	for _, sep := range []string{" — ", " – ", " -- ", " — ", "—", "–"} {
		if i := strings.LastIndex(s, sep); i > 0 {
			return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(sep):])
		}
	}
	return strings.TrimSpace(s), ""
}

// heroQuote renders the dashboard "Quote of the day". It is an opt-in AI feature
// (Smart+): when enabled and an OpenAI key (or backend) is configured it generates
// one short money-mindset quote per day in the chosen theme via the user's own
// key, caches it in the SMART Results store (so it shows between renders without
// re-spending), and refreshes when the day rolls over or the theme changes. All
// hooks are called unconditionally (stable positions) before any state-based
// branch in the returned markup.
func heroQuote(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	prefs := uistate.UsePrefs().Get().Normalize()
	tick := ui.UseState(0)
	_ = tick.Get() // re-render after async generation / setting changes
	loading := ui.UseState(false)
	errored := ui.UseState(false)

	settings := uistate.LoadSmartSettings()
	enabled := app != nil && settings.IsEnabled(quoteCode)
	theme := settings.QuoteThemeOr()
	useContext := settings.QuoteUseContext
	now := time.Now()
	cached := settings.ResultFor(quoteCode)
	last := settings.LastRunAt(quoteCode)
	fresh := cached != "" && quoteSameDay(last, now)

	aiKey, model := "", "gpt-5.4-mini"
	if app != nil {
		aiKey = app.Settings().OpenAIKey
		if m := app.Settings().OpenAIModel; m != "" {
			model = m
		}
	}
	useBackend := prefs.BackendActive()
	hasProvider := aiKey != "" || useBackend

	// Handlers (declared unconditionally for stable hook order).
	enable := ui.UseEvent(func() { uistate.SetSmartFeatureEnabled(quoteCode, true); tick.Set(tick.Get() + 1) })
	onTheme := ui.UseEvent(func(e ui.Event) {
		uistate.SetSmartQuoteTheme(e.GetValue())
		loading.Set(false)
		tick.Set(tick.Get() + 1)
	})
	onNew := ui.UseEvent(func() {
		uistate.SetSmartQuoteTheme(theme)
		loading.Set(false)
		errored.Set(false)
		tick.Set(tick.Get() + 1)
	})
	onContext := ui.UseEvent(func() {
		uistate.SetSmartQuoteContext(!useContext)
		loading.Set(false)
		errored.Set(false)
		tick.Set(tick.Get() + 1)
	})

	// Personalization snapshot (only built — and only sent — when the user opts in).
	context := ""
	if useContext && app != nil {
		context = financialContextString(app)
	}

	// Auto-generate once per (theme, context, day): the key embeds LastRun + today
	// so a "new quote" (which clears LastRun), a theme/context change, and a day
	// rollover all re-fire it, while a fresh same-day cache does not.
	autoKey := fmt.Sprintf("quote|%v|%s|%v|%v|%d|%s", enabled, theme, useContext, hasProvider, last.Unix(), now.Format("2006-01-02"))
	ui.UseEffect(func() func() {
		// errored gates re-entry (QA task #43): a failed generation used to
		// re-fire this effect endlessly — MarkSmartRun changes autoKey on every
		// attempt — hammering the API. Now a failure rests on the calm retry
		// line until the user asks again (↻ / theme change clear errored).
		if !enabled || app == nil || !hasProvider || fresh || loading.Get() || errored.Get() {
			return nil
		}
		loading.Set(true)
		errored.Set(false)
		uistate.MarkSmartRun(quoteCode, now) // stamp before the call to guard re-entry
		req := smartai.QuoteOfDay(theme, context)
		messages := []ai.Message{
			{Role: ai.RoleSystem, Content: req.System},
			{Role: ai.RoleUser, Content: req.User},
		}
		onOK := func(c string, _ ai.Usage) {
			q := cleanQuote(c)
			loading.Set(false)
			if q == "" {
				errored.Set(true)
				tick.Set(tick.Get() + 1)
				return
			}
			uistate.SetSmartResult(quoteCode, q, time.Now())
			tick.Set(tick.Get() + 1)
		}
		onErr := func(_ string) { loading.Set(false); errored.Set(true); tick.Set(tick.Get() + 1) }
		// QA task #43 root cause: 0.85 was sent unconditionally, but the DEFAULT
		// model (gpt-5.4-mini) is a reasoning model that REJECTS a custom
		// temperature on /chat/completions — every quote request 400'd, so the
		// feature never worked out of the box. Mirror the chat assistant's
		// reasoningModel gate: 0 omits the field (temperature,omitempty).
		temp := 0.85
		if reasoningModel(model) {
			temp = 0
		}
		if useBackend {
			ai.SendProxyChat(prefs.ServerURL, prefs.ServerToken, model, messages, temp, onOK, onErr)
		} else {
			ai.SendChat(aiKey, ai.DefaultBaseURL, model, messages, temp, onOK, onErr)
		}
		return nil
	}, autoKey)

	if app == nil {
		return Fragment()
	}

	// Disabled → a quiet, discoverable opt-in.
	if !enabled {
		return Div(css.Class("home-hero-quote home-hero-quote--off"),
			Button(css.Class("hero-quote-enable"), Type("button"), OnClick(enable),
				Attr("data-testid", "hero-quote-enable"),
				Span(css.Class("hero-quote-mark"), "✦"),
				uistate.T("home.quoteEnable"),
			),
		)
	}

	// Body: the cited quote, a loading line, a "needs key" hint, or — on a failed
	// generation — a calm retry line (rather than a stuck "Composing…").
	var body ui.Node
	switch {
	case fresh:
		qt, author := splitQuote(cached)
		body = Span(css.Class("hero-quote-text"), Attr("data-testid", "hero-quote-text"),
			qt,
			If(author != "", Span(css.Class("hero-quote-cite"), " — "+author)),
		)
	case !hasProvider:
		body = Span(css.Class("hero-quote-hint"), uistate.T("home.quoteNeedKey"))
	case loading.Get():
		body = Span(css.Class("hero-quote-text hero-quote-loading"), uistate.T("home.quoteLoading"))
	case errored.Get():
		body = Span(css.Class("hero-quote-hint"), uistate.T("home.quoteError"))
	default:
		body = Span(css.Class("hero-quote-text hero-quote-loading"), uistate.T("home.quoteLoading"))
	}

	themeSel := []any{css.Class("hero-quote-theme"),
		Attr("aria-label", uistate.T("home.quoteThemeLabel")), Attr("title", uistate.T("home.quoteThemeLabel")),
		OnChange(onTheme)}
	for _, th := range quoteThemes {
		themeSel = append(themeSel, Option(Value(th), SelectedIf(th == theme), th))
	}

	// Personalize toggle: a pill that includes the user's financial context to
	// steer which quote is chosen. aria-pressed reflects state.
	ctxCls := "hero-quote-ctx"
	if useContext {
		ctxCls += " is-on"
	}

	return Div(css.Class("home-hero-quote"),
		Span(css.Class("hero-quote-mark"), "✦"),
		body,
		Div(css.Class("hero-quote-controls"),
			Button(ClassStr(ctxCls), Type("button"), OnClick(onContext),
				Attr("aria-pressed", fmt.Sprintf("%v", useContext)),
				Attr("data-testid", "hero-quote-context"),
				Attr("title", uistate.T("home.quoteContextTitle")),
				uistate.T("home.quoteContext")),
			If(hasProvider, Button(css.Class("hero-quote-refresh"), Type("button"), OnClick(onNew),
				Attr("aria-label", uistate.T("home.quoteNew")), Attr("title", uistate.T("home.quoteNew")), "↻")),
			Select(themeSel...),
		),
	)
}
