// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/calendargrid"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// CalendarProps configures the reusable month calendar. It is deterministic: the
// caller supplies both the month to show and the "today" date (the component never
// calls time.Now()). All the grid geometry comes from the pure internal/calendargrid
// package; this component is a thin render shell over it.
type CalendarProps struct {
	Month     time.Time    // any day in the month to show
	WeekStart time.Weekday // first column's weekday (Sunday- or Monday-start, etc.)
	Selected  time.Time    // highlighted day; zero = none
	Today     time.Time    // for the "today" ring; zero = none
	// DayContent renders optional badges/markers beneath a day's number (e.g. dots,
	// amounts). May be nil. Called with the cell's date and whether it is in-month.
	DayContent func(day time.Time, inMonth bool) uic.Node
	// OnDayClick fires with a cell's date when clicked. When nil, cells are rendered
	// as non-interactive (no button, no hover affordance).
	OnDayClick   func(day time.Time)
	OnPrevMonth  func() // page to the previous month; nil disables the prev button
	OnNextMonth  func() // page to the next month; nil disables the next button
	Label        string // aria caption for the whole calendar; default "Calendar"
	Compact      bool   // smaller cells (date-picker sizing)
	TestIDPrefix string // e.g. "todo-cal" -> per-cell "todo-cal-day-2006-01-02", plus "-prev"/"-next"/"-title"
}

// Calendar renders a month grid: a header (prev · month/year title · next), a
// weekday-label row, and the weeks-of-days grid. Each day is its own DayCell
// sub-component so its click handler is registered at a stable position (never
// inside the render loop — GWC hook rule).
func Calendar(props CalendarProps) uic.Node {
	label := props.Label
	if label == "" {
		label = "Calendar"
	}

	// Header nav handlers live at stable (non-loop) positions in this component.
	prev := uic.UseEvent(Prevent(func() {
		if props.OnPrevMonth != nil {
			props.OnPrevMonth()
		}
	}))
	next := uic.UseEvent(Prevent(func() {
		if props.OnNextMonth != nil {
			props.OnNextMonth()
		}
	}))

	prevArgs := []any{css.Class("uical-nav-btn"), Type("button"), Attr("aria-label", "Previous month")}
	if props.TestIDPrefix != "" {
		prevArgs = append(prevArgs, Attr("data-testid", props.TestIDPrefix+"-prev"))
	}
	if props.OnPrevMonth != nil {
		prevArgs = append(prevArgs, OnClick(prev))
	} else {
		prevArgs = append(prevArgs, Disabled(true))
	}
	prevArgs = append(prevArgs, Icon(icon.ChevronLeft))

	nextArgs := []any{css.Class("uical-nav-btn"), Type("button"), Attr("aria-label", "Next month")}
	if props.TestIDPrefix != "" {
		nextArgs = append(nextArgs, Attr("data-testid", props.TestIDPrefix+"-next"))
	}
	if props.OnNextMonth != nil {
		nextArgs = append(nextArgs, OnClick(next))
	} else {
		nextArgs = append(nextArgs, Disabled(true))
	}
	nextArgs = append(nextArgs, Icon(icon.ChevronRight))

	titleArgs := []any{css.Class("uical-title")}
	if props.TestIDPrefix != "" {
		titleArgs = append(titleArgs, Attr("data-testid", props.TestIDPrefix+"-title"))
	}
	titleArgs = append(titleArgs, props.Month.Format("January 2006"))

	head := Div(css.Class("uical-head"),
		Button(prevArgs...),
		Div(titleArgs...),
		Button(nextArgs...),
	)

	// Weekday labels in week-start order (short names).
	wdArgs := []any{css.Class("uical-weekdays")}
	for _, wd := range calendargrid.WeekdayOrder(props.WeekStart) {
		wdArgs = append(wdArgs, Div(css.Class("uical-weekday"), wd.String()[:3]))
	}

	// The grid: one row per week, each day its own component.
	weeks := calendargrid.Month(props.Month, props.WeekStart, props.Today)
	gridArgs := []any{css.Class("uical-grid")}
	for _, wk := range weeks {
		rowArgs := []any{css.Class("uical-week")}
		for _, d := range wk {
			selected := !props.Selected.IsZero() && sameYMD(d.Date, props.Selected)
			testID := ""
			if props.TestIDPrefix != "" {
				testID = props.TestIDPrefix + "-day-" + d.Date.Format("2006-01-02")
			}
			rowArgs = append(rowArgs, uic.CreateElement(calDayCell, calDayCellProps{
				Date:       d.Date,
				InMonth:    d.InMonth,
				IsToday:    d.IsToday,
				Selected:   selected,
				DayContent: props.DayContent,
				OnClick:    props.OnDayClick,
				TestID:     testID,
			}))
		}
		gridArgs = append(gridArgs, Div(rowArgs...))
	}

	rootCls := "uical"
	if props.Compact {
		rootCls += " is-compact"
	}
	return Div(ClassStr(rootCls), Attr("role", "group"), Attr("aria-label", label),
		head,
		Div(wdArgs...),
		Div(gridArgs...),
	)
}

// calDayCellProps configures one day cell. Callbacks are plain funcs so the cell
// owns its own click hook (safe inside the parent's render loop — GWC rule).
type calDayCellProps struct {
	Date       time.Time
	InMonth    bool
	IsToday    bool
	Selected   bool
	DayContent func(day time.Time, inMonth bool) uic.Node
	OnClick    func(day time.Time) // nil => the cell is non-interactive
	TestID     string
}

// calDayCell renders one day: the number, an optional content well beneath, and
// the out-of-month / today / selected style hooks. Interactive cells render as a
// <button>; otherwise a plain <div>.
func calDayCell(props calDayCellProps) uic.Node {
	// Registered unconditionally at the top so the hook position is stable across
	// renders even when this cell is non-interactive.
	click := uic.UseEvent(func() {
		if props.OnClick != nil {
			props.OnClick(props.Date)
		}
	})

	cls := "uical-cell"
	if !props.InMonth {
		cls += " is-out"
	}
	if props.IsToday {
		cls += " is-today"
	}
	if props.Selected {
		cls += " is-selected"
	}

	var content uic.Node = Fragment()
	if props.DayContent != nil {
		content = props.DayContent(props.Date, props.InMonth)
	}

	common := []any{ClassStr(cls), Attr("aria-label", props.Date.Format("Monday, January 2, 2006"))}
	if props.Selected {
		common = append(common, Attr("aria-current", "date"))
	}
	if props.TestID != "" {
		common = append(common, Attr("data-testid", props.TestID))
	}
	common = append(common,
		Span(css.Class("uical-daynum"), strconv.Itoa(props.Date.Day())),
		Div(css.Class("uical-daycontent"), content),
	)

	if props.OnClick != nil {
		return Button(append([]any{Type("button"), OnClick(click)}, common...)...)
	}
	return Div(common...)
}

// sameYMD reports whether a and b are the same calendar day.
func sameYMD(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
