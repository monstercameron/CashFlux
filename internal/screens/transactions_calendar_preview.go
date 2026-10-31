// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// The hover preview on a calendar day. Pointing at a day waits txnCalPreviewDelayMs before its
// popover opens, so a pointer passing over the grid opens nothing. The popover then shows a
// loader over the day's rows, and the stylesheet fades the loader out. The preview lives in the
// calendar, outside the week grid, so its portal never takes a grid cell.
const (
	txnCalPreviewIdle = iota
	txnCalPreviewWaiting
	txnCalPreviewOpen
)

const (
	txnCalPreviewDelayMs = 250
	txnCalPreviewMaxRows = 6
)

// txnCalPreview is the day whose preview is open, or waiting to open, and its phase.
type txnCalPreview struct {
	Key   string
	Phase int
}

// txnCalDayItem is one transaction listed in a day's preview.
type txnCalDayItem struct {
	Label  string
	Amount money.Money
}

// txnCalHover is the channel a day cell uses to report the pointer entering and leaving it.
type txnCalHover struct {
	Enter func(dateKey string)
	Leave func()
}

// txnCalDayItems groups the shown transactions by calendar day, so a hovered day lists its own.
func txnCalDayItems(shown []domain.Transaction) map[string][]txnCalDayItem {
	out := make(map[string][]txnCalDayItem)
	for _, t := range shown {
		label := t.Payee
		if label == "" {
			label = t.Desc
		}
		key := txncalendar.DayKey(t.Date)
		out[key] = append(out[key], txnCalDayItem{Label: label, Amount: t.Amount})
	}
	return out
}

// txnCalPreviewOverlay is the day's popover, anchored to the day's cell. It is closed until the
// preview's popover opens.
func txnCalPreviewOverlay(preview txnCalPreview, items map[string][]txnCalDayItem, leave func()) ui.Node {
	return uiw.Overlay(uiw.OverlayProps{
		Open: preview.Key != "" && preview.Phase == txnCalPreviewOpen, Kind: uiw.OverlayPopover,
		Title: txnCalPreviewTitle(preview.Key), Label: uistate.T("transactions.calPreviewLabel", preview.Key), AnchorID: "txn-cal-day-" + preview.Key,
		OnClose: leave, TestID: "txn-cal-preview",
		Body: txnCalPreviewBody(items[preview.Key]),
	})
}

// txnCalPreviewBody shows the day's rows beneath a loader. The loader and the rows are both in
// the popover when it opens, so the rows need no second state change to appear.
func txnCalPreviewBody(items []txnCalDayItem) ui.Node {
	loader := Div(css.Class("txn-cal-preview-loading"), Attr("role", "status"),
		Span(css.Class("dt-spin dt-spin-sm"), Attr("aria-hidden", "true")),
		uistate.T("transactions.calPreviewLoading"))
	var content ui.Node
	if len(items) == 0 {
		content = P(css.Class("muted"), uistate.T("transactions.calPreviewEmpty"))
	} else {
		args := []any{css.Class("txn-cal-preview-list"), Attr("data-testid", "txn-cal-preview-list")}
		for i, item := range items {
			if i == txnCalPreviewMaxRows {
				args = append(args, P(css.Class("muted"), uistate.T("transactions.calPreviewMore", len(items)-i)))
				break
			}
			args = append(args, Div(css.Class("txn-cal-preview-row"),
				Span(css.Class("txn-cal-preview-label"), item.Label),
				Span(css.Class("txn-cal-preview-amount"), fmtMoney(item.Amount))))
		}
		content = Div(args...)
	}
	return Div(css.Class("txn-cal-preview-body"), loader, Div(css.Class("txn-cal-preview-content"), content))
}

// txnCalPreviewTitle names the previewed day in the user's date style, so the card's heading
// reads as a date rather than a storage key.
func txnCalPreviewTitle(dateKey string) string {
	d, err := dateutil.ParseDate(dateKey)
	if err != nil {
		return dateKey
	}
	return calendarLabels(uistate.LoadPrefs()).Prefs.FormatDate(d)
}
