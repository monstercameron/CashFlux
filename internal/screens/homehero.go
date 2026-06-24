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

// HomeHero is the dashboard's top "home" band (EC4). It sits above the bento
// grid and delivers an immediate, glanceable summary of the household's
// financial position — no scrolling required.
//
// Two states:
//   - Empty dataset (no accounts, no transactions): a welcoming first-run hero
//     with the app value proposition, a "Load sample" primary CTA, and an
//     "Add your first account" secondary button. Commercial first impression.
//   - Non-empty dataset: a time-of-day greeting, the net-worth hero figure,
//     a compact this-month stats row (income / spending / net / savings rate),
//     and two quick-action buttons (add transaction, add account).
//
// All text goes through uistate.T; every button carries Type("button"),
// aria-label, and is keyboard-reachable (WCAG 2.1.1 / 4.1.2). No business
// logic — pure formatting delegated to the memoized §1.6 selectors and the
// shared format helpers (fmtMoney, figTone, ledger.SavingsRate).
func HomeHero() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get() // re-render after import / load-sample / wipe

	accounts := app.Accounts()
	txns := app.Transactions()

	// Empty dataset (no accounts, no transactions) → first-run welcome state.
	if len(accounts) == 0 && len(txns) == 0 {
		return ui.CreateElement(homeHeroEmpty, struct{}{})
	}

	// Non-empty → compute headline figures from the memoized selectors, then
	// delegate to homeHeroFull so its hooks occupy stable positions.
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

	// Net display: prefix "+" for positive, accounting-format for negative,
	// plain zero for zero — mirrors dashboard cash-flow style (L4).
	netDisplay := fmtMoney(netMoney)
	if netMoney.Amount > 0 {
		netDisplay = "+" + fmtMoney(netMoney)
	}

	return ui.CreateElement(homeHeroFull, homeHeroFullProps{
		NetWorth:     fmtMoney(nw.Net),
		NetWorthTone: figTone(nw.Net),
		Income:       fmtMoney(income),
		Spending:     fmtMoney(expense),
		Net:          netDisplay,
		NetTone:      figTone(netMoney),
		SavingsPct:   savingsPct,
	})
}

// homeHeroFullProps carries the pre-formatted display values for the non-empty
// hero so each component owns exactly the hooks it declares.
type homeHeroFullProps struct {
	NetWorth     string
	NetWorthTone string
	Income       string
	Spending     string
	Net          string // formatted cash-flow net (income − spending)
	NetTone      string // color token, e.g. "text-up"
	SavingsPct   int
}

// homeHeroFull renders the non-empty hero: a time-of-day greeting, the
// net-worth hero figure, a this-month stats row, and quick-action buttons.
func homeHeroFull(props homeHeroFullProps) ui.Node {
	// Time-of-day greeting keyed on local wall-clock hour.
	greeting := uistate.T("home.greetingEvening")
	h := time.Now().Hour()
	switch {
	case h < 12:
		greeting = uistate.T("home.greetingMorning")
	case h < 17:
		greeting = uistate.T("home.greetingAfternoon")
	}

	// Quick-action hooks — unconditional declarations for stable hook ordering.
	quickAddTxn := ui.UseEvent(func() { uistate.SetQuickAdd(true) })
	quickAddAcct := ui.UseEvent(func() { uistate.SetAddTarget("account") })

	savingsStr := fmt.Sprintf("%d%%", props.SavingsPct)
	savingsTone := "text-up"
	switch {
	case props.SavingsPct < 0:
		savingsTone = "text-down"
	case props.SavingsPct < 10:
		savingsTone = "text-warn"
	}

	return Div(css.Class("home-hero"),
		// Band heading — accessible landmark; the widget's H2 is one level below
		// the topbar shell's implicit page title (WCAG 2.4.6, no level-skip).
		H2(ClassStr("home-hero-greeting "+tw.Fold(tw.FontDisplay, tw.Text28, tw.LeadingTight, tw.TrackingTight)),
			greeting,
		),

		// Net-worth hero figure — the single most important number on the page.
		Div(css.Class("home-hero-nw"),
			Span(css.Class("home-hero-nw-label t-caption", tw.TextFaint), uistate.T("home.netWorth")),
			Div(ClassStr("home-hero-nw-fig fig t-figure-lg "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(props.NetWorthTone)),
				Attr("data-countup", ""),
				props.NetWorth,
			),
		),

		// This-month compact stat row (income / spending / net / savings rate).
		Div(css.Class("home-hero-stats"),
			heroStatBlock(uistate.T("home.income"), props.Income, "text-up"),
			heroStatBlock(uistate.T("home.spending"), props.Spending, "text-down"),
			heroStatBlock(uistate.T("home.net"), props.Net, props.NetTone),
			heroStatBlock(uistate.T("home.savingsRate"), savingsStr, savingsTone),
		),

		// Quick actions — low-friction entry points for the two most common tasks.
		Div(css.Class("home-hero-actions"),
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddTxnAria")),
				Attr("title", uistate.T("home.quickAddTxnAria")),
				Attr("data-testid", "hero-add-txn"),
				OnClick(quickAddTxn),
				uistate.T("home.quickAddTxn"),
			),
			Button(css.Class("btn"), Type("button"),
				Attr("aria-label", uistate.T("home.quickAddAccountAria")),
				Attr("title", uistate.T("home.quickAddAccountAria")),
				Attr("data-testid", "hero-add-account"),
				OnClick(quickAddAcct),
				uistate.T("home.quickAddAccount"),
			),
		),
	)
}

// homeHeroEmpty renders the first-run welcome state for an empty dataset.
// Its own component keeps its hooks at positions independent of the
// non-empty variant's hooks (the On*-hooks-in-loops rule applies here too).
func homeHeroEmpty(_ struct{}) ui.Node {
	rev := uistate.UseDataRevision()
	loadSample := ui.UseEvent(func() {
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
	addAccount := ui.UseEvent(func() { uistate.SetAddTarget("account") })

	return Div(css.Class("home-hero home-hero--welcome"),
		// Accessible band heading for screen readers and sighted users.
		H2(ClassStr("home-hero-welcome-title "+tw.Fold(tw.FontDisplay, tw.Text28, tw.LeadingTight, tw.TrackingTight)),
			uistate.T("home.welcomeTitle"),
		),
		P(css.Class("home-hero-welcome-body t-body", tw.TextDim, tw.Mt2),
			uistate.T("home.welcomeBody"),
		),
		Div(css.Class("home-hero-actions", tw.Mt3),
			// Primary: load sample data — the fastest path to seeing the app.
			Button(css.Class("btn btn-primary"), Type("button"),
				Attr("aria-label", uistate.T("home.loadSampleAria")),
				Attr("title", uistate.T("home.loadSampleAria")),
				Attr("data-testid", "hero-load-sample"),
				OnClick(loadSample),
				uistate.T("home.loadSample"),
			),
			// Secondary: start with real data right away.
			Button(css.Class("btn"), Type("button"),
				Attr("aria-label", uistate.T("home.addFirstAria")),
				Attr("title", uistate.T("home.addFirstAria")),
				Attr("data-testid", "hero-add-first-account"),
				OnClick(addAccount),
				uistate.T("home.addFirst"),
			),
		),
	)
}

// heroStatBlock renders one labelled figure in the this-month stats row.
// tone is a color-token string (e.g. "text-up", "text-down", "text-warn", "").
func heroStatBlock(label, value, tone string) ui.Node {
	valueCls := "home-hero-stat-value fig " + tw.Fold(tw.FontDisplay) + " " + tw.ColorClass(tone)
	return Div(css.Class("home-hero-stat"),
		Span(css.Class("home-hero-stat-label t-caption", tw.TextFaint), label),
		Span(ClassStr(valueCls), Attr("data-countup", ""), value),
	)
}
