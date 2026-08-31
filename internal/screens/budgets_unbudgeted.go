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
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// unbudgetedStripProps drives the "Unbudgeted spending" strip on /budgets.
type unbudgetedStripProps struct {
	App     *appstate.App
	Base    string
	CatName map[string]string
	// From/To are the VIEWED period's half-open range, and Label names it (C607).
	//
	// The strip used to read time.Now() and say "this month" whatever period the
	// page was showing: on a closed Jul 2026 view in August it listed August's
	// spending under a label claiming otherwise. A supporting module that quietly
	// changes scope while the page's own figures do not is worse than one that
	// simply has no data for the period.
	From, To time.Time
	Label    string
	// Historical is true when the viewed window has already ended, which changes
	// the tense of everything here from an invitation to a record.
	Historical bool
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
	// C607: the VIEWED period, not today's month. A zero range means the caller
	// did not say, and falling back to this month is then honest rather than a
	// silent mismatch — but every caller passes one.
	ms, me := props.From, props.To
	if ms.IsZero() || me.IsZero() {
		ms, me = dateutil.MonthRange(now)
	}
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
	// The cap is fine — this is a teaser, not an inventory — but it used to be
	// SILENT: a household with nine untracked categories was shown four and never
	// told the other five existed, so "tidy these up" looked finished when it was
	// not (Cam, 2026-08-31). The count rides on the bulk link below.
	const maxChips = 4
	totalCands := len(cands)
	if len(cands) > maxChips {
		cands = cands[:maxChips]
	}

	chips := MapKeyed(cands, func(c unbudgetedCat) any { return c.ID }, func(c unbudgetedCat) ui.Node {
		// Suggest from 6-month history ending at the VIEWED period, falling back to
		// that period's actual spend — so the suggested limit describes the same
		// window the chip's figure does (C607).
		sug, _ := budgeting.SuggestLimit(c.ID, app.Transactions(), ms, 6, rates)
		if sug <= 0 {
			sug = c.SpentMinor
		}
		return ui.CreateElement(unbudgetedChip, unbudgetedChipProps{
			CatID: c.ID, CatName: c.Name,
			SpentStr:   fmtMoney(money.New(c.SpentMinor, props.Base)),
			LimitMajor: money.FormatMinor(sug, currency.Decimals(props.Base)),
		})
	})
	// C607: the hint NAMES the period it describes. "this month" was true only by
	// coincidence, and false in exactly the case a user is most likely to
	// misread — a closed period they have deliberately paged back to.
	hintKey := "budgets.unbudgetedHintPeriod"
	if props.Historical {
		hintKey = "budgets.unbudgetedHintPeriodPast"
	}
	label := props.Label
	if label == "" {
		label = uistate.T("budgets.unbudgetedThisPeriod")
	}
	return Div(css.Class("budget-unbudgeted"), Attr("data-testid", "budgets-unbudgeted"),
		Div(css.Class("budget-unbudgeted-head"),
			Span(css.Class("budget-unbudgeted-title"), uistate.T("budgets.unbudgetedHead")),
			Span(css.Class(tw.TextFaint, tw.Text12), Attr("data-testid", "budgets-unbudgeted-hint"),
				uistate.T(hintKey, label)),
			// The chips are capped at four and scoped to the viewed period, and
			// neither limit is visible from here. This is the way to the rest: every
			// untracked category over a year, each one handled on its own terms.
			ui.CreateElement(trackAllButton, trackAllButtonProps{Total: totalCands}),
		),
		Div(css.Class("budget-unbudgeted-chips"), chips),
	)
}

type trackAllButtonProps struct{ Total int }

// trackAllButton opens the bulk sheet. Its own component so the click hook sits at
// a stable call-site, matching every other control on this strip.
func trackAllButton(props trackAllButtonProps) ui.Node {
	open := uistate.UseTrackUntrackedOpen()
	onOpen := ui.UseEvent(Prevent(func() { open.Set(true) }))
	// The label carries the number the chips cannot: how many untracked categories
	// there actually are, over a full year rather than just this period.
	label := uistate.T("budgets.trackAll")
	if props.Total > 0 {
		label = uistate.T("budgets.trackAllCount", props.Total)
	}
	return Button(css.Class("budget-unbudgeted-all"), Type("button"),
		Attr("data-testid", "budgets-track-all"), OnClick(onOpen), label)
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
