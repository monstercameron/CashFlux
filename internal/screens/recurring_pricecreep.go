// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// priceCreepNotices surfaces detected price creep (XC5) at the top of /recurring:
// one quiet line per crept bill with a "Review" action that opens the accept-flow
// card in place. This is the tool's home surface; the SMART flag's action lands
// here too.
func priceCreepNotices() ui.Node {
	app := appstate.Default
	uistate.UseDataRevision() // re-detect when data changes
	openID := ui.UseState("")
	expanded := ui.UseState(false)
	onExpand := ui.UseEvent(func() { expanded.Set(true) })

	if app == nil {
		return Fragment()
	}
	creeps := smartengine.DetectCreep(buildSmartInput(app, uistate.LoadPrefs().WeekStartWeekday()))
	if len(creeps) == 0 {
		return Fragment()
	}
	// The detection is currency-blind (minor units); read each bill's currency
	// off its recurring for display and the accept flow.
	curOf := map[string]string{}
	for _, r := range app.Recurring() {
		curOf[r.ID] = r.Amount.Currency
	}

	// The open card replaces the notice list while a review is in progress.
	if id := openID.Get(); id != "" {
		for _, c := range creeps {
			if c.RecurringID == id {
				return ui.CreateElement(PriceAcceptCard, PriceAcceptProps{
					RecurringID: c.RecurringID, Label: c.Label,
					OldMinor: c.ExpectedMinor, NewMinor: c.NewMinor,
					Currency: curOf[c.RecurringID],
					OnDone:   func() { openID.Set("") },
				})
			}
		}
		// The creep resolved while the card was open (accepted elsewhere) — close.
		openID.Set("")
	}

	// More than two increases collapse to one quiet summary line — a wall of
	// stacked warnings reads as nagging, and one line carries the same message.
	if len(creeps) > 2 && !expanded.Get() {
		return Div(css.Class("notice-warn", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb2),
			Attr("role", "status"), Attr("data-testid", "pricecreep-notices"),
			Span(css.Class("t-body"), uistate.T("pricecreep.summaryLine", len(creeps))),
			Span(css.Class(tw.Flex1)),
			Button(css.Class("btn btn-sm"), Type("button"),
				Attr("data-testid", "pricecreep-expand"), OnClick(onExpand),
				uistate.T("pricecreep.showAll")),
		)
	}

	rows := make([]ui.Node, 0, len(creeps))
	for _, c := range creeps {
		c := c
		rows = append(rows, ui.CreateElement(priceCreepNoticeRow, priceCreepNoticeRowProps{
			Creep: c, Currency: curOf[c.RecurringID],
			OnOpen: func(id string) { openID.Set(id) },
		}))
	}
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2),
		Attr("data-testid", "pricecreep-notices"), rows)
}

type priceCreepNoticeRowProps struct {
	Creep    smartengine.Creep
	Currency string
	OnOpen   func(recurringID string)
}

// priceCreepNoticeRow is one crept bill's notice line — its own component so the
// Review button's hook sits at a stable position (rows render in a loop).
func priceCreepNoticeRow(p priceCreepNoticeRowProps) ui.Node {
	onReview := ui.UseEvent(func() { p.OnOpen(p.Creep.RecurringID) })
	return Div(css.Class("notice-warn", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("role", "status"), Attr("data-testid", "pricecreep-notice-"+p.Creep.RecurringID),
		Span(css.Class("t-body"), uistate.T("pricecreep.noticeLine", p.Creep.Label,
			fmtMoney(money.New(p.Creep.ExpectedMinor, p.Currency)),
			fmtMoney(money.New(p.Creep.NewMinor, p.Currency)))),
		Span(css.Class(tw.Flex1)),
		Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "pricecreep-review-"+p.Creep.RecurringID),
			OnClick(onReview), uistate.T("pricecreep.review")),
	)
}
