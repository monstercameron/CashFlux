// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recap"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// recapStat is one cell of the recap banner: a leading glyph + label, a headline
// value, and a sub-line, with an optional tone ("good"/"bad"/"") on the value.
func recapStat(testID string, glyph icon.Name, label, value, sub, tone string) ui.Node {
	valCls := "cf-recap-val"
	if tone != "" {
		valCls += " " + tone
	}
	return Div(css.Class("cf-recap-stat"), Attr("data-testid", testID),
		Div(css.Class("cf-recap-lbl"),
			uiw.Icon(glyph, css.Class(tw.W35, tw.H35, tw.ShrinkO)),
			Span(label),
		),
		Span(css.Class(valCls), value),
		If(sub != "", Span(css.Class("cf-recap-sub"), sub)),
	)
}

// monthlyRecapWidget renders the Monthly Recap dashboard banner (CG-S1): a
// glanceable "month in review". It intentionally avoids repeating the hero's
// figures — the lead cell is the month-over-month spend CHANGE (which the hero
// lacks), followed by the "where did it go" story (top category, biggest
// expense, biggest change) and a no-spend-days streak. Read-only, so its cells
// inline safely.
func monthlyRecapWidget(c widgetrender.RenderCtx) ui.Node {
	const widgetID = "monthly-recap"
	// The recap follows the dashboard's SELECTED period, not the wall clock: a
	// past month recaps as its own completed month, the current month recaps
	// live up to today (parity-scan defect: paging the period left the recap on
	// "July 1–17"). A future month has no activity and shows the empty state.
	now := time.Now()
	anchor := now
	if now.Before(c.Start) || !now.Before(c.End) {
		anchor = c.End.Add(-time.Second) // last instant of the viewed period
	}
	rec, err := recap.Compute(anchor, c.ScopedTxns, c.ScopedAccounts, c.Rates)
	if err != nil || !rec.HasData {
		return uiw.Widget(uiw.WidgetProps{
			ID: widgetID, Title: uistate.T("dashboard.monthlyRecap"),
			Draggable: !c.Preview, Resizable: !c.Preview, Preview: c.Preview,
			Body: P(css.Class("cf-recap-empty t-body", tw.TextDim), Attr("data-testid", "recap-empty"),
				uistate.T("dashboard.recapEmpty")),
		})
	}

	base := rec.Base
	m := func(v int64) string { return fmtMoney(money.New(v, base)) }
	vsLast := uistate.T("dashboard.recapVsLast")

	// Header: an explicit day range ("July 1–15") — one phrase, no redundant
	// "as of" stamp. A completed month shows just its name.
	heading := anchor.Month().String()
	if !rec.Complete {
		heading += " 1–" + strconv.Itoa(anchor.Day())
	}

	// Stat 1 — the lead: spend CHANGE vs the same span last month (the hero shows
	// the raw spend total, so we lead with what it can't: the direction). The raw
	// amount is relegated to the sub-line as context.
	var stat1 ui.Node
	switch {
	case rec.SpendDeltaKnown && rec.SpendDeltaPct < 0:
		stat1 = recapStat("recap-spent", icon.TrendingDown, uistate.T("dashboard.recapVsLastLabel"),
			"↓"+strconv.FormatInt(-rec.SpendDeltaPct, 10)+"%",
			m(rec.Expense)+" · "+uistate.T("dashboard.recapWas", m(rec.PrevExpense)), "good")
	case rec.SpendDeltaKnown && rec.SpendDeltaPct > 0:
		stat1 = recapStat("recap-spent", icon.TrendingUp, uistate.T("dashboard.recapVsLastLabel"),
			"↑"+strconv.FormatInt(rec.SpendDeltaPct, 10)+"%",
			m(rec.Expense)+" · "+uistate.T("dashboard.recapWas", m(rec.PrevExpense)), "bad")
	default:
		// No comparable prior spend — just show the total, honestly labelled.
		stat1 = recapStat("recap-spent", icon.ArrowUpCircle, uistate.T("dashboard.recapSpent"),
			m(rec.Expense), uistate.T("dashboard.recapSpendNew"), "")
	}

	// Stat 2 — Top category.
	topStat := recapStat("recap-top", icon.Tag, uistate.T("dashboard.recapTopCategory"),
		categoryDisplayName(c, rec.TopCategoryID), m(rec.TopCategoryAmount), "")

	stats := []any{css.Class("cf-recap-stats"), stat1, topStat}

	// Stat 3 — Biggest single expense. Suppressed when it's the same money as the
	// top category (same amount + same category) so two identical figures don't sit
	// side by side looking like a bug.
	sameAsTop := rec.BiggestExpenseAmount == rec.TopCategoryAmount &&
		rec.BiggestExpenseCategoryID == rec.TopCategoryID
	if rec.BiggestExpenseKnown && !sameAsTop {
		stats = append(stats, recapStat("recap-biggest", icon.Receipt,
			uistate.T("dashboard.recapBiggestExpenseLabel"), rec.BiggestExpenseDesc, m(rec.BiggestExpenseAmount), ""))
	}

	// Stat 4 — Biggest category change vs last month (down = good).
	if rec.MoverHasData && rec.MoverDelta != 0 {
		glyph, tone, arrow := icon.TrendingUp, "bad", "↑"
		d := rec.MoverDelta
		if d < 0 {
			glyph, tone, arrow, d = icon.TrendingDown, "good", "↓", -d
		}
		stats = append(stats, recapStat("recap-mover", glyph, uistate.T("dashboard.recapBiggestChange"),
			categoryDisplayName(c, rec.MoverID), arrow+" "+m(d)+" "+vsLast, tone))
	}

	// Stat 5 — no-spend-days streak, folded into the grid (not a footer orphan).
	if rec.NoSpendDays > 0 {
		sub := uistate.T("dashboard.recapNoSpendSub")
		if rec.Complete {
			sub = uistate.T("dashboard.recapNoSpendSubDone")
		}
		stats = append(stats, recapStat("recap-nospend", icon.Calendar, uistate.T("dashboard.recapNoSpendLabel"),
			strconv.Itoa(rec.NoSpendDays), sub, ""))
	}

	body := Div(css.Class("cf-recap"), Attr("data-testid", "monthly-recap"),
		Div(css.Class("cf-recap-head"),
			uiw.Icon(icon.Calendar, css.Class(tw.W4, tw.H4, tw.ShrinkO)),
			Span(css.Class("cf-recap-title"), heading),
		),
		Div(stats...),
	)

	return uiw.Widget(uiw.WidgetProps{
		ID: widgetID, Title: uistate.T("dashboard.monthlyRecap"),
		Draggable: !c.Preview, Resizable: !c.Preview, Preview: c.Preview,
		Body: body,
	})
}

// categoryDisplayName resolves a category id to its name, labelling the empty id
// (uncategorized spend) with a friendly fallback.
func categoryDisplayName(c widgetrender.RenderCtx, id string) string {
	if id == "" {
		return uistate.T("dashboard.recapUncategorized")
	}
	for _, cat := range c.App.Categories() {
		if cat.ID == id {
			return cat.Name
		}
	}
	return uistate.T("dashboard.recapUncategorized")
}
