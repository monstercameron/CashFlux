// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/txncalendarview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestCSCALWeekHeaderKeepsColumnOrderAndNames(t *testing.T) {
	f := render.New(t)
	headers := calendarLabels(uistate.LoadPrefs()).Headers(time.Monday)
	f.Render(ui.CreateElement(txnCalWeekHeader, headers))
	var columns []*render.QueryNode
	for _, node := range f.AllByTag("div") {
		if node.Attr("role") == "columnheader" {
			columns = append(columns, node)
		}
	}
	if len(columns) != 7 {
		t.Fatalf("weekday columns=%d", len(columns))
	}
	for i, node := range columns {
		if node.Attr("class") != "cal-head" || node.Attr("aria-label") != headers[i].Aria || node.Text() != headers[i].Text {
			t.Errorf("column %d text=%q aria=%q class=%q", i, node.Text(), node.Attr("aria-label"), node.Attr("class"))
		}
	}
	if len(f.AllByTag("button")) != 0 {
		t.Fatal("static header acquired a control")
	}
	var total bool
	for _, node := range f.AllByTag("div") {
		if node.Attr("class") == "cal-head" && node.Text() == uistate.T("transactions.calWeekTotal") {
			total = true
		}
	}
	if !total {
		t.Fatal("weekly total heading missing")
	}
}

func TestCSCALGhostMarkerKeepsInlineMarkupAndCopy(t *testing.T) {
	f := render.New(t)
	summary := txncalendarview.GhostSummary{Class: "txn-cal-ghost txn-cal-ghost-missed", Status: "$220.00 not posted", Label: "Electric bill"}
	f.Render(ui.CreateElement(txnCalGhostMarker, summary))
	spans := f.AllByTag("span")
	if len(spans) != 3 {
		t.Fatalf("marker spans=%d want 3", len(spans))
	}
	for i, want := range []string{summary.Class, "txn-cal-ghost-status", "txn-cal-ghost-label"} {
		if spans[i].Attr("class") != want {
			t.Errorf("span %d class=%q want %q", i, spans[i].Attr("class"), want)
		}
		if spans[i].Attr("aria-label") != "" || spans[i].Attr("title") != "" {
			t.Fatal("marker duplicates accessible day description")
		}
	}
	if spans[1].Text() != summary.Status || spans[2].Text() != summary.Label {
		t.Fatalf("marker content=%q", spans[0].Text())
	}
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-cal-ghost") {
			t.Fatal("scheduled marker acquired block markup")
		}
	}
	if len(f.AllByTag("button")) != 0 {
		t.Fatal("scheduled marker acquired block or interactive markup")
	}
}

func TestCSCALWeekRowKeepsDayEventsStableAcrossChangingRows(t *testing.T) {
	f := render.New(t)
	picked := ""
	var resize func(int)
	harness := func() ui.Node {
		count := ui.UseState(2)
		resize = count.Set
		days := []txncalendarview.Day{
			{DateKey: "2026-09-30", Day: 30, MonthName: "September 2026"},
			{DateKey: "2026-10-01", Day: 1, InMonth: true, Today: true, Count: 6},
			{DateKey: "2026-10-02", Day: 2, InMonth: true},
		}
		return ui.CreateElement(txnCalWeekRow, txnCalWeekRowProps{
			Week:   txncalendarview.Week{Days: days[:count.Get()], Total: txncalendarview.WeekInput{Base: "USD", WeekOf: "Sep 27", SummaryID: "stable-week"}},
			OnPick: func(key string) { picked = key },
		})
	}
	f.Render(ui.CreateElement(harness))
	for _, count := range []int{2, 3, 1, 3} {
		resize(count)
		f.Stabilize()
		buttons := f.AllByTag("button")
		if len(buttons) != count {
			t.Fatalf("days=%d want %d", len(buttons), count)
		}
		picked = ""
		if buttons[0].Property("onclick") != nil {
			t.Fatal("padding day acquired a click handler")
		}
		if !strings.Contains(buttons[0].Attr("class"), " out") {
			t.Fatal("padding day lost its out-of-month class")
		}
		for i := 1; i < count; i++ {
			buttons[i].Click()
			f.Stabilize()
			if picked != buttons[i].Attr("data-date") {
				t.Fatalf("pick=%q want %q", picked, buttons[i].Attr("data-date"))
			}
		}
		total := r2aByTestID(f, "div", "txn-cal-week-total")
		if total == nil || total.Attr("aria-describedby") != "stable-week" {
			t.Fatal("weekly summary lost its description identity")
		}
	}
}

func TestCSCALCalendarLayoutKeepsCaptionNoticesAndLegend(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		month                      txncalendarview.Month
		warning, errorNotice, note bool
	}{
		{name: "normal", month: txncalendarview.Month{Caption: "October 2026", ShowScheduled: true}, note: true},
		{name: "partial FX", month: txncalendarview.Month{Caption: "October 2026", RateIssues: 2}, warning: true},
		{name: "projection failure", month: txncalendarview.Month{Caption: "October 2026", Err: errors.New("overflow")}, errorNotice: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(ui.CreateElement(txnCalCalendar, txnCalCalendarProps{Month: tc.month, Headers: calendarLabels(uistate.LoadPrefs()).Headers(time.Sunday)}))
			caption := r2aByTestID(f, "span", "txn-cal-month")
			if caption == nil || caption.Text() != tc.month.Caption || caption.Attr("class") != "txn-cal-month" {
				t.Fatal("caption changed")
			}
			for _, notice := range []struct {
				id   string
				want bool
			}{{"txn-cal-fx-warning", tc.warning}, {"txn-cal-fx-error", tc.errorNotice}} {
				node := r2aByTestID(f, "div", notice.id)
				if (node != nil) != notice.want {
					t.Errorf("notice %s=%v want %v", notice.id, node, notice.want)
				}
				if node != nil && node.Attr("role") != "status" {
					t.Error("notice lost status semantics")
				}
			}
			legend, note := false, false
			for _, node := range f.AllByTag("div") {
				if node.Attr("class") == "txn-cal-legend" {
					legend = true
				}
				if node.Text() == uistate.T("transactions.calDueFilterNote") {
					note = true
				}
			}
			if legend == tc.errorNotice || note != tc.note {
				t.Errorf("legend=%v note=%v", legend, note)
			}
		})
	}
}

func TestCSCALCalendarComposesWeekChildrenWithoutExtraMarkup(t *testing.T) {
	l := calendarLabels(uistate.LoadPrefs())
	month := l.Build(txncalendarview.Input{Now: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC), Rates: currency.Rates{Base: "USD"}})
	f := render.New(t)
	f.Render(ui.CreateElement(txnCalCalendar, txnCalCalendarProps{Month: month, Headers: l.Headers(time.Sunday)}))
	if len(f.AllByTag("button")) != 35 {
		t.Fatalf("calendar days=%d want 35", len(f.AllByTag("button")))
	}
	rows, head, totals := 0, 0, 0
	for _, node := range f.AllByTag("div") {
		switch node.Attr("class") {
		case "txn-cal-week-row":
			rows++
		case "txn-cal-week-head":
			head++
		}
		if node.Attr("data-testid") == "txn-cal-week-total" {
			totals++
		}
	}
	if rows != 5 || head != 1 || totals != 5 {
		t.Fatalf("grid rows=%d headers=%d totals=%d", rows, head, totals)
	}
	for _, button := range f.AllByTag("button") {
		if strings.TrimSpace(button.Attr("aria-label")) == "" {
			t.Fatal("day lost its accessible name")
		}
	}
}
