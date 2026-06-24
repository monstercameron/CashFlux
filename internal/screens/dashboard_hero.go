// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
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

	netMoney := money.New(income.Amount-expense.Amount, income.Currency)
	savingsPct := ledger.SavingsRate(income.Amount, expense.Amount)

	// Net display: "+" prefix for surplus, accounting parens for deficit, plain
	// zero for breakeven — mirrors the dashboard cash-flow sub-line style (L4).
	netDisplay := fmtMoney(netMoney)
	if netMoney.Amount > 0 {
		netDisplay = "+" + fmtMoney(netMoney)
	}

	return ui.CreateElement(heroSummary, heroSummaryProps{
		NetWorth:     fmtMoney(nw.Net),
		NetWorthTone: figTone(nw.Net),
		Income:       fmtMoney(income),
		Spending:     fmtMoney(expense),
		Net:          netDisplay,
		NetTone:      figTone(netMoney),
		SavingsPct:   savingsPct,
	})
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

	return Div(css.Class("home-hero"),
		// Band heading — H2 because the topbar shell provides the implicit page
		// landmark; this is the next heading level (WCAG 2.4.6, no level-skip).
		H2(ClassStr("home-hero-greeting "+tw.Fold(tw.FontDisplay, tw.Text28, tw.LeadingTight, tw.TrackingTight)),
			greeting,
		),

		// Net-worth hero figure — the single most important number on this page.
		Div(css.Class("home-hero-nw"),
			Span(css.Class("home-hero-nw-label t-caption", tw.TextFaint),
				uistate.T("home.netWorth"),
			),
			Div(ClassStr("home-hero-nw-fig fig t-figure-lg "+
				tw.Fold(tw.FontDisplay)+" "+
				tw.ColorClass(props.NetWorthTone)),
				Attr("data-countup", ""),
				props.NetWorth,
			),
		),

		// This-month compact stat row (income / spending / net / savings rate).
		Div(css.Class("home-hero-stats"),
			heroStat(uistate.T("home.income"), props.Income, "text-up"),
			heroStat(uistate.T("home.spending"), props.Spending, "text-down"),
			heroStat(uistate.T("home.net"), props.Net, props.NetTone),
			heroStat(uistate.T("home.savingsRate"), savingsStr, savingsTone),
		),

		// Quick actions — the two most common entry points, surfaced without hunting.
		Div(css.Class("home-hero-actions"),
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddTxnAria")),
				Attr("title", uistate.T("home.quickAddTxnAria")),
				Attr("data-testid", "hero-add-txn"),
				OnClick(openQuickAdd),
				uistate.T("home.quickAddTxn"),
			),
			Button(css.Class("btn"), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddAccountAria")),
				Attr("title", uistate.T("home.quickAddAccountAria")),
				Attr("data-testid", "hero-add-account"),
				OnClick(openAddAccount),
				uistate.T("home.quickAddAccount"),
			),
		),
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
