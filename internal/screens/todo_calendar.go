// SPDX-License-Identifier: MIT

//go:build js && wasm

// To-do calendar (schedule) view — one of the three To-do surface views (list / board
// / calendar). It renders the standardized, reusable uiw.Calendar primitive with each
// day carrying the tasks due that day as small clickable chips; clicking a chip opens
// the task, and a hover-revealed "+" on a day starts a new task already scheduled for
// that day (one click to schedule). It is a pure projection of the (already filtered)
// task list.
//
// DayContent is called inside the calendar's positional day cells, so it must register
// NO hooks itself — every interactive element it emits (a task chip, the add button) is
// its own child component via ui.CreateElement, keeping the day cell's hook list stable
// as the month pages.
package screens

import (
	"sort"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// maxTodoCalChips caps how many task chips a day cell shows before collapsing the rest
// into a "+N" overflow chip, so a busy day never blows out the row height.
const maxTodoCalChips = 3

type todoCalendarProps struct {
	Tasks      []domain.Task
	Month      time.Time    // any day within the month to display
	Today      time.Time    // for the calendar's "today" ring
	WeekStart  time.Weekday // from prefs
	OnPrev     func()
	OnNext     func()
	OnOpenTask func(id string)
	OnAddOnDay func(day time.Time)
}

// todoCalDayChipProps configures a single day's task chip (its own component so the
// click handler is never registered inside the day-bucket loop — framework rule).
type todoCalDayChipProps struct {
	Task   domain.Task
	OnOpen func(id string)
}

// TodoCalDayChip renders one task as a compact chip inside a calendar day cell: a
// priority dot + the (truncated) title, opening the task on click.
func TodoCalDayChip(props todoCalDayChipProps) ui.Node {
	t := props.Task
	open := ui.UseEvent(Prevent(func() {
		if props.OnOpen != nil {
			props.OnOpen(t.ID)
		}
	}))
	cls := "tcal-chip p-" + string(t.Priority)
	if t.Status == domain.StatusDone {
		cls += " is-done"
	}
	return Button(ClassStr(cls), Type("button"),
		Attr("data-testid", "todo-cal-chip-"+t.ID), Title(t.Title), OnClick(open),
		Span(css.Class("tcal-chip-dot"), Attr("aria-hidden", "true")),
		Span(css.Class("tcal-chip-title"), t.Title),
	)
}

// todoCalAddBtnProps configures a day's "schedule a task here" affordance.
type todoCalAddBtnProps struct {
	Day   time.Time
	Label string
	OnAdd func(day time.Time)
}

// TodoCalAddBtn is the hover-revealed "+" that starts a new task due on this day. Its
// own component so its click hook lives outside the calendar's positional day loop.
func TodoCalAddBtn(props todoCalAddBtnProps) ui.Node {
	add := ui.UseEvent(Prevent(func() {
		if props.OnAdd != nil {
			props.OnAdd(props.Day)
		}
	}))
	return Button(css.Class("tcal-add"), Type("button"),
		Attr("data-testid", "todo-cal-add-"+props.Day.Format("2006-01-02")),
		Attr("aria-label", props.Label), Title(props.Label), OnClick(add),
		uiw.Icon(icon.Plus, css.Class(tw.W3, tw.H3)))
}

// tasksByDueDay buckets tasks with a due date by their local calendar date (ISO key).
// Tasks are ordered within a day by priority (high first) then title, so the chips a
// day shows first are the ones that matter most.
func tasksByDueDay(tasks []domain.Task) map[string][]domain.Task {
	m := make(map[string][]domain.Task)
	for _, t := range tasks {
		if t.Due.IsZero() {
			continue
		}
		key := dateutil.FormatDate(t.Due)
		m[key] = append(m[key], t)
	}
	for k := range m {
		day := m[k]
		sort.SliceStable(day, func(i, j int) bool {
			pi, pj := priorityRank(day[i].Priority), priorityRank(day[j].Priority)
			if pi != pj {
				return pi < pj
			}
			return day[i].Title < day[j].Title
		})
		m[k] = day
	}
	return m
}

// priorityRank orders priorities high(0) → medium(1) → low(2) for within-day sorting.
func priorityRank(p domain.TaskPriority) int {
	switch p {
	case domain.PriorityHigh:
		return 0
	case domain.PriorityLow:
		return 2
	default:
		return 1
	}
}

// todoCalendarView renders the calendar schedule view over the reusable uiw.Calendar.
func todoCalendarView(props todoCalendarProps) ui.Node {
	buckets := tasksByDueDay(props.Tasks)

	// DayContent is hook-free: it only composes child components + static nodes.
	dayContent := func(day time.Time, inMonth bool) ui.Node {
		if !inMonth {
			return Fragment()
		}
		kids := make([]ui.Node, 0, maxTodoCalChips+2)
		if due := buckets[dateutil.FormatDate(day)]; len(due) > 0 {
			shown := due
			overflow := 0
			if len(shown) > maxTodoCalChips {
				overflow = len(shown) - maxTodoCalChips
				shown = shown[:maxTodoCalChips]
			}
			chips := make([]ui.Node, 0, len(shown)+1)
			for _, t := range shown {
				chips = append(chips, ui.CreateElement(TodoCalDayChip, todoCalDayChipProps{Task: t, OnOpen: props.OnOpenTask}))
			}
			if overflow > 0 {
				chips = append(chips, Span(css.Class("tcal-more"), Attr("aria-hidden", "true"), "+"+strconv.Itoa(overflow)))
			}
			kids = append(kids, Div(css.Class("tcal-daytasks"), chips))
		}
		kids = append(kids, ui.CreateElement(TodoCalAddBtn, todoCalAddBtnProps{
			Day:   day,
			Label: uistate.T("todo.calendarAddOnDay", day.Format("Mon Jan 2")),
			OnAdd: props.OnAddOnDay,
		}))
		return Div(css.Class("tcal-daycell"), kids)
	}

	return Div(css.Class("tcal"),
		uiw.Calendar(uiw.CalendarProps{
			Month:        props.Month,
			WeekStart:    props.WeekStart,
			Today:        props.Today,
			DayContent:   dayContent,
			OnPrevMonth:  props.OnPrev,
			OnNextMonth:  props.OnNext,
			Label:        uistate.T("todo.calendarLabel"),
			TestIDPrefix: "todo-cal",
		}),
	)
}
