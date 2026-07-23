// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
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
	app := props.App
	w := uistate.UsePeriod().Get()
	now := time.Now()
	start, end := w.Range()
	if !start.After(now) {
		// Not a future window: render a hidden placeholder rather than an empty
		// Fragment. An empty→content transition would late-mount and append the tile
		// to the end of the bento (losing its lead position); a keyed placeholder
		// holds the slot so it mutates in place when the user pages into the future.
		return Div(Attr("data-testid", "budget-future-ph"), Style(map[string]string{"display": "none"}))
	}

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
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

	var inflow, outflow int64
	for _, g := range ghosts {
		conv, err := currency.ConvertBetween(g.Amount, curOf[g.RecurringID], base, rates)
		if err != nil {
			continue
		}
		if conv >= 0 {
			inflow += conv
		} else {
			outflow += -conv
		}
	}
	net := inflow - outflow

	stats := Div(css.Class("bfut-stats"),
		bfutStat("budgets.future.in", money.New(inflow, base), "text-up"),
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

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title:  uistate.T("budgets.future.title", w.Label()),
		TestID: "budgets-future",
		Body: Fragment(
			P(css.Class("muted", tw.Text13), uistate.T("budgets.future.desc")),
			stats,
			listBody,
		),
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-future", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// bfutStat renders one in/out/net figure block.
func bfutStat(labelKey string, m money.Money, tone string) ui.Node {
	return Div(css.Class("bfut-stat"),
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
