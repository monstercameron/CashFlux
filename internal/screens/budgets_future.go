// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgetplan"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetFutureMaxRows caps the projected-occurrence list so a long future window
// (e.g. a whole year) stays glanceable; the remainder is summarized as "+ N more".
const budgetFutureMaxRows = 8

// budgetFutureWidget answers coworker feedback #6 — "when navigating future
// timelines, show future-related things, not empty values." When the global
// period control is paged to a window that BEGINS in the future, the budgets
// page would otherwise show empty actuals (no transactions exist yet). This
// self-gating tile fills that gap: it projects the household's recurring bills
// and income into the viewed window and shows what's set to land — money in,
// money out, and the individual occurrences by date. It renders nothing for the
// current or a past period, so it never clutters the normal view.
func budgetFutureWidget(props budgetSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	w := uistate.UsePeriod().Get()
	activeMember := uistate.UseActiveMember().Get()
	prefs := uistate.UsePrefs().Get()
	open := ui.UseState(false)
	toggleOpen := ui.UseEvent(Prevent(func() { open.Set(!open.Get()) }))
	return renderBudgetFutureWidget(props, w, time.Now().UTC(), activeMember, prefs, open.Get(), toggleOpen)
}

func renderBudgetFutureWidget(props budgetSummaryProps, w period.Window, now time.Time, activeMember string, prefs prefs.Prefs, open bool, toggleOpen ui.Handler) ui.Node {
	toggleOnClick := OnClick(toggleOpen)
	app := props.App
	start, end := w.Range()
	if !isFuturePeriod(w, now) {
		// Not a future window: render a hidden placeholder rather than an empty
		// Fragment. An empty→content transition would late-mount and append the tile
		// to the end of the bento (losing its lead position); a keyed placeholder
		// holds the slot so it mutates in place when the user pages into the future.
		return Div(Attr("data-testid", "budget-future-ph"), Style(map[string]string{"display": "none"}))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = budgetBaseCurrency(base)
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Active recurrings only, with a currency lookup for base conversion (Ghost
	// carries a signed minor amount but not its currency).
	active := make([]domain.Recurring, 0)
	curOf := make(map[string]string)
	for _, r := range app.Recurring() {
		if r.Active() {
			active = append(active, r)
			curOf[r.ID] = r.Amount.Currency
		}
	}
	ghosts := txncalendar.Ghosts(active, start, end)
	sort.SliceStable(ghosts, func(i, j int) bool { return ghosts[i].Date.Before(ghosts[j].Date) })

	// Keep the future income figure on the same basis as the budget hero. Scheduled
	// deposits remain visible as individual recurring rows, while this total is the
	// plan's expected income, not a second estimate derived from the recurrence list.
	view := computeBudgetView(app, activeMember, w, prefs, false)
	expectedWindowIncome, err := budgetplan.ScaleMonthlyIncome(money.New(view.BannerIncome, base), start, end)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	expectedIncome := expectedWindowIncome.Amount
	var outflow int64
	for _, g := range ghosts {
		conv, err := currency.ConvertBetween(g.Amount, curOf[g.RecurringID], base, rates)
		if err != nil {
			continue
		}
		if conv < 0 {
			positive, err := money.Neg(conv)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
			outflow, err = money.Add(outflow, positive)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
		}
	}
	var receivedIncome int64
	if lines, err := reports.IncomeByCategory(app.Transactions(), start, end, rates); err == nil {
		for _, line := range lines {
			receivedIncome, err = money.Add(receivedIncome, line.Amount)
			if err != nil {
				return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
			}
		}
	}
	planned, err := money.Add(view.TotalLimit, view.SavingsAssigned)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}
	net, err := money.Sub(expectedIncome, outflow)
	if err != nil {
		return Div(css.Class("budget-future-error"), Attr("role", "alert"), uistate.T("budgets.annualGridError"))
	}

	stats := Div(css.Class("bfut-stats"),
		bfutStat("budgets.future.expectedIncome", money.New(expectedIncome, base), "text-up"),
		bfutStat("budgets.future.out", money.New(outflow, base), "text-down"),
		bfutStat("budgets.future.net", money.New(net, base), netTone(net)),
	)

	var listBody ui.Node
	if len(ghosts) == 0 {
		listBody = P(css.Class("empty t-body", tw.TextDim), Attr("data-testid", "budgets-future-empty"),
			uistate.T("budgets.future.none"))
	} else {
		rows := make([]ui.Node, 0, budgetFutureMaxRows+1)
		for i, g := range ghosts {
			if i >= budgetFutureMaxRows {
				rows = append(rows, P(css.Class("brc-more t-caption", tw.TextFaint),
					uistate.T("budgets.future.more", len(ghosts)-budgetFutureMaxRows)))
				break
			}
			rows = append(rows, budgetFutureRow(g, curOf[g.RecurringID]))
		}
		listBody = Div(css.Class("brc-rows"), rows)
	}

	billCount := 0
	for _, g := range ghosts {
		if g.Amount < 0 {
			billCount++
		}
	}
	foldLabel := uistate.TN("budgets.future.foldHintOne", "budgets.future.foldHintMany",
		billCount, fmtMoney(money.New(outflow, base)))
	buttonLabel := uistate.T("budgets.future.showAria")
	if open {
		buttonLabel = uistate.T("budgets.future.hideAria")
	}
	fold := Button(css.Class("budget-fold-toggle"), Type("button"),
		Attr("data-testid", "budgets-future-toggle"), Attr("aria-expanded", ariaBool(open)),
		Attr("aria-label", buttonLabel), toggleOnClick,
		Span(css.Class("budget-fold-toggle-label"), uistate.T("budgets.future.title", w.Label())+" · "),
		Span(css.Class("budget-fold-toggle-hint"), foldLabel),
	)

	var body ui.Node = Fragment()
	if open {
		body = Fragment(
			P(css.Class("budget-future-plan", tw.Text13), Attr("data-testid", "budgets-future-planning"), uistate.T("budgets.future.planning",
				fmtMoney(money.New(planned, base)), fmtMoney(money.New(expectedIncome, base)))),
			P(css.Class("budget-future-received", tw.Text12, tw.TextDim), Attr("data-testid", "budgets-future-received"), uistate.T("budgets.future.received",
				fmtMoney(money.New(receivedIncome, base)))),
			P(css.Class("muted", tw.Text13), uistate.T("budgets.future.desc")),
			stats,
			// EC-11: whether the month ahead is actually set up, as sentences
			// naming what is missing. A score would tell the reader something is
			// wrong and nothing about what.
			readinessNotes(app, now, rates, base),
			listBody,
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-future", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Div(css.Class("brc budget-fold"), Attr("data-testid", "budgets-future"), fold, body),
	})
}

// isFuturePeriod reports whether the viewed window starts after today. Budgets uses
// this same decision to replace present-tense funding alarms with planning language.
func isFuturePeriod(w period.Window, now time.Time) bool {
	start, _ := w.Range()
	return start.After(now)
}

// bfutStat renders one in/out/net figure block.
func bfutStat(labelKey string, m money.Money, tone string) ui.Node {
	return Div(css.Class("bfut-stat"),
		Attr("data-testid", "budgets-future-stat-"+strings.TrimPrefix(labelKey, "budgets.future.")),
		Span(css.Class("bfut-stat-label", tw.TextDim), uistate.T(labelKey)),
		Span(ClassStr("bfut-stat-val fig "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(tone)), fmtMoney(m)),
	)
}

// netTone picks the semantic colour for a net figure.
func netTone(net int64) string {
	if net < 0 {
		return "text-down"
	}
	return "text-up"
}

// budgetFutureRow renders one projected occurrence: its due date, the recurring's
// label, and its signed amount. Display-only, safe inside the row loop.
func budgetFutureRow(g txncalendar.Ghost, cur string) ui.Node {
	amt := money.New(g.Amount, cur)
	tone := "text-up"
	if g.Amount < 0 {
		tone = "text-down"
	}
	return Div(css.Class("brc-row"), Attr("data-testid", "budgets-future-row"),
		Span(css.Class("brc-cadence"), uistate.LoadPrefs().FormatDate(g.Date)),
		Div(css.Class("brc-body"),
			Span(css.Class("brc-label", tw.Fold(tw.FontDisplay)), g.Label),
		),
		Div(css.Class("brc-amtcol"),
			Span(ClassStr("brc-amt fig "+tw.ColorClass(tone)), fmtMoney(amt)),
		),
	)
}
