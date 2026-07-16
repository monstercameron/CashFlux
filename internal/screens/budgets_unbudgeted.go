// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// unbudgetedStripProps drives the "Unbudgeted spending" strip on /budgets.
type unbudgetedStripProps struct {
	App     *appstate.App
	Base    string
	CatName map[string]string
}

// unbudgetedCat is one candidate: an expense category with spending this month
// and no budget tracking it.
type unbudgetedCat struct {
	ID, Name   string
	SpentMinor int64
}

// unbudgetedStrip surfaces contextual budget creation (G8): the top expense
// categories with real spending this month that no budget tracks, each one click
// from a pre-filled add form (category + a suggested limit). Renders nothing when
// every spending category is already budgeted — the strip is an invitation, not
// a fixture.
func unbudgetedStrip(props unbudgetedStripProps) ui.Node {
	app := props.App
	if app == nil {
		return Fragment()
	}
	now := time.Now()
	ms, me := dateutil.MonthRange(now)
	rates := currency.Rates{Base: props.Base, Rates: app.Settings().FXRates}

	// Every category any budget tracks (primary or extra) is off the table.
	tracked := map[string]bool{}
	for _, b := range app.Budgets() {
		for _, cid := range b.TrackedCategoryIDs() {
			tracked[cid] = true
		}
	}
	// This month's expense spend per untracked category (base minor units).
	spend := map[string]int64{}
	for _, t := range app.Transactions() {
		if !t.IsExpense() || t.CategoryID == "" || tracked[t.CategoryID] || !dateutil.InRange(t.Date, ms, me) {
			continue
		}
		if conv, err := rates.Convert(t.Amount.Abs(), props.Base); err == nil {
			spend[t.CategoryID] += conv.Amount
		}
	}
	var cands []unbudgetedCat
	for _, c := range app.Categories() {
		if amt := spend[c.ID]; amt > 0 {
			cands = append(cands, unbudgetedCat{ID: c.ID, Name: c.Name, SpentMinor: amt})
		}
	}
	if len(cands) == 0 {
		return Fragment()
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].SpentMinor > cands[j].SpentMinor })
	const maxChips = 4
	if len(cands) > maxChips {
		cands = cands[:maxChips]
	}

	chips := MapKeyed(cands, func(c unbudgetedCat) any { return c.ID }, func(c unbudgetedCat) ui.Node {
		// Suggest from 6-month history; fall back to this month's actual spend.
		sug, _ := budgeting.SuggestLimit(c.ID, app.Transactions(), now, 6, rates)
		if sug <= 0 {
			sug = c.SpentMinor
		}
		return ui.CreateElement(unbudgetedChip, unbudgetedChipProps{
			CatID: c.ID, CatName: c.Name,
			SpentStr:   fmtMoney(money.New(c.SpentMinor, props.Base)),
			LimitMajor: money.FormatMinor(sug, currency.Decimals(props.Base)),
		})
	})
	return Div(css.Class("budget-unbudgeted"), Attr("data-testid", "budgets-unbudgeted"),
		Div(css.Class("budget-unbudgeted-head"),
			Span(css.Class("budget-unbudgeted-title"), uistate.T("budgets.unbudgetedHead")),
			Span(css.Class(tw.TextFaint, tw.Text12), uistate.T("budgets.unbudgetedHint")),
		),
		Div(css.Class("budget-unbudgeted-chips"), chips),
	)
}

// unbudgetedChipProps drives one category chip in the unbudgeted strip.
type unbudgetedChipProps struct {
	CatID, CatName, SpentStr, LimitMajor string
}

// unbudgetedChip is one "<category> · $spent — Budget this" chip. Clicking it opens
// the add-budget modal pre-seeded with the category and a suggested limit. Its own
// component so the click hook sits at a stable call-site (no On* in the map loop).
func unbudgetedChip(props unbudgetedChipProps) ui.Node {
	open := ui.UseEvent(Prevent(func() {
		uistate.SetBudgetAddSeed(uistate.BudgetAddSeed{
			Name: props.CatName, CategoryID: props.CatID, LimitMajor: props.LimitMajor,
		})
		uistate.SetAddTarget("budget")
	}))
	return Button(css.Class("budget-unbudgeted-chip"), Type("button"),
		Attr("data-testid", "budget-this-"+props.CatID),
		Title(uistate.T("budgets.budgetThis")), OnClick(open),
		Span(css.Class("budget-unbudgeted-cat"), uistate.T("budgets.unbudgetedChip", props.CatName, props.SpentStr)),
		Span(css.Class("budget-unbudgeted-cta"),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			uistate.T("budgets.budgetThis")),
	)
}
