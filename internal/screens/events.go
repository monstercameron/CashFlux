// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — events.go holds the /events route: the management surface for
// first-class spending events (TX10). It lists each event with its date range,
// total, and transaction count, and offers add / inline-edit / delete. Creating an
// event auto-associates the transactions in its range (with a confirmation count);
// "View transactions" drops the event's date range onto the ledger filter.
package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/events"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

const eventDateLayout = "2006-01-02"

// parseEventDate parses a yyyy-mm-dd input into a UTC calendar date; a blank
// string yields the zero time (an open-ended end).
func parseEventDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(eventDateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// formatEventRange renders an event's date range for a row ("Jun 1 – Jun 12,
// 2026", or "Jun 1, 2026 – open-ended").
func formatEventRange(e domain.Event) string {
	start := e.Start.Format("Jan 2, 2006")
	if e.End.IsZero() {
		return start + " – " + uistate.T("events.openEnded")
	}
	return start + " – " + e.End.Format("Jan 2, 2006")
}

// Events is the /events route — the spending-events management surface (TX10).
func Events() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	txns := app.Transactions()

	// --- add-form state ---
	adding := ui.UseState(false)
	nameS := ui.UseState("")
	startS := ui.UseState(time.Now().Format(eventDateLayout))
	endS := ui.UseState("")
	noteS := ui.UseState("")
	errS := ui.UseState("")
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onStart := ui.UseEvent(func(v string) { startS.Set(v) })
	onEnd := ui.UseEvent(func(v string) { endS.Set(v) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })

	resetForm := func() {
		nameS.Set("")
		startS.Set(time.Now().Format(eventDateLayout))
		endS.Set("")
		noteS.Set("")
		errS.Set("")
	}
	openAdd := ui.UseEvent(Prevent(func() { resetForm(); adding.Set(true) }))
	cancelAdd := ui.UseEvent(Prevent(func() { adding.Set(false); errS.Set("") }))
	saveAdd := ui.UseEvent(Prevent(func() {
		start, ok := parseEventDate(startS.Get())
		if !ok || start.IsZero() {
			errS.Set(uistate.T("events.start"))
			return
		}
		end, ok := parseEventDate(endS.Get())
		if !ok {
			errS.Set(uistate.T("events.end"))
			return
		}
		saved, err := app.PutEvent(domain.Event{
			Name:  nameS.Get(),
			Start: start,
			End:   end,
			Note:  strings.TrimSpace(noteS.Get()),
		})
		if err != nil {
			errS.Set(err.Error())
			return
		}
		n, err := app.AutoAssociateEvent(saved.ID)
		if err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		if n > 0 {
			uistate.PostNotice(uistate.T("events.tagged", strconv.Itoa(n)), false)
		} else {
			uistate.PostNotice(uistate.T("events.taggedNone"), false)
		}
		adding.Set(false)
		resetForm()
	}))

	// --- row callbacks ---
	saveEvent := func(updated domain.Event) {
		if _, err := app.PutEvent(updated); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("events.saved"), false)
	}
	deleteEvent := func(e domain.Event) {
		uistate.ConfirmModal(uistate.T("events.deleteConfirm", e.Name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteEvent(e.ID); err != nil {
				uistate.PostNotice(err.Error(), true)
				return
			}
			uistate.RequestPersist()
			uistate.BumpDataRevision()
		})
	}
	viewEvent := func(e domain.Event) {
		f := txFilter.Get()
		f.From = e.Start.Format(eventDateLayout)
		if e.End.IsZero() {
			f.To = ""
		} else {
			f.To = e.End.Format(eventDateLayout)
		}
		f = f.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	evs := app.Events()

	var addForm ui.Node = Fragment()
	if adding.Get() {
		addForm = Div(css.Class("card", tw.P3, tw.Mb3), Attr("data-testid", "event-add-form"),
			Div(css.Class(tw.Grid, tw.Gap2),
				Label(css.Class("fctrl-label"), uistate.T("events.name")),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("events.namePh")),
					Attr("data-testid", "event-name"), Value(nameS.Get()), OnInput(onName)),
				Div(css.Class(tw.Flex, tw.Gap2),
					Div(css.Class(tw.Flex1),
						Label(css.Class("fctrl-label"), uistate.T("events.start")),
						Input(css.Class("field"), Type("date"), Attr("data-testid", "event-start"),
							Value(startS.Get()), OnInput(onStart))),
					Div(css.Class(tw.Flex1),
						Label(css.Class("fctrl-label"), uistate.T("events.end")),
						Input(css.Class("field"), Type("date"), Attr("data-testid", "event-end"),
							Value(endS.Get()), OnInput(onEnd))),
				),
				Span(css.Class("text-dim", tw.Text13), uistate.T("events.endHint")),
				Label(css.Class("fctrl-label"), uistate.T("events.note")),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("events.notePh")),
					Value(noteS.Get()), OnInput(onNote)),
				If(errS.Get() != "", Div(css.Class("form-error"), Attr("role", "alert"), errS.Get())),
				Div(css.Class(tw.Flex, tw.Gap2),
					Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "event-save"),
						OnClick(saveAdd), uistate.T("events.save")),
					Button(css.Class("btn btn-ghost"), Type("button"), OnClick(cancelAdd), uistate.T("events.cancel")),
				),
			),
		)
	}

	var list ui.Node
	if len(evs) == 0 {
		list = Div(css.Class("empty"), Attr("data-testid", "events-empty"), uistate.T("events.empty"))
	} else {
		list = Div(css.Class(tw.Grid, tw.Gap2), Attr("data-testid", "events-list"),
			MapKeyed(evs,
				func(e domain.Event) any { return e.ID },
				func(e domain.Event) ui.Node {
					members := app.EventMembers(e.ID)
					// Show the SIGNED net (spending negative, income positive) rather
					// than spend-only: an event whose range caught income too would
					// otherwise read a misleading $0.00. Pure-spending trips still show
					// their real cost (a negative figure), and the amount tone follows.
					net, _ := events.Totals(members, txns)
					return ui.CreateElement(eventRow, eventRowProps{
						Event:    e,
						Count:    len(members),
						SpendStr: fmtMoney(money.New(net, base)),
						OnSave:   saveEvent,
						OnDelete: deleteEvent,
						OnView:   viewEvent,
					})
				},
			),
		)
	}

	return Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb3),
			H2(css.Class(tw.TextLg, tw.FontSemibold), uistate.T("events.title")),
			Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "events-add"), Title(uistate.T("events.addTitle")), OnClick(openAdd),
				uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("events.add"))),
		),
		addForm,
		list,
	)
}

// eventRowProps configures one event row on the /events surface.
type eventRowProps struct {
	Event    domain.Event
	Count    int
	SpendStr string
	OnSave   func(domain.Event)
	OnDelete func(domain.Event)
	OnView   func(domain.Event)
}

// eventRow renders one event: its name, date range, total and count, plus
// inline-edit / delete / view-transactions actions. It owns its handler hooks
// (per the GWC rule: On* handlers live in a per-row component, never in a loop)
// and an internal edit-mode state seeded from the event.
func eventRow(props eventRowProps) ui.Node {
	e := props.Event
	editing := ui.UseState(false)
	nameS := ui.UseState(e.Name)
	startS := ui.UseState(e.Start.Format(eventDateLayout))
	endInit := ""
	if !e.End.IsZero() {
		endInit = e.End.Format(eventDateLayout)
	}
	endS := ui.UseState(endInit)
	noteS := ui.UseState(e.Note)
	errS := ui.UseState("")
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })
	onStart := ui.UseEvent(func(v string) { startS.Set(v) })
	onEnd := ui.UseEvent(func(v string) { endS.Set(v) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })

	startEdit := ui.UseEvent(Prevent(func() { editing.Set(true) }))
	cancelEdit := ui.UseEvent(Prevent(func() {
		nameS.Set(e.Name)
		startS.Set(e.Start.Format(eventDateLayout))
		endS.Set(endInit)
		noteS.Set(e.Note)
		errS.Set("")
		editing.Set(false)
	}))
	saveEdit := ui.UseEvent(Prevent(func() {
		start, ok := parseEventDate(startS.Get())
		if !ok || start.IsZero() {
			errS.Set(uistate.T("events.start"))
			return
		}
		end, ok := parseEventDate(endS.Get())
		if !ok {
			errS.Set(uistate.T("events.end"))
			return
		}
		updated := e
		updated.Name = nameS.Get()
		updated.Start = start
		updated.End = end
		updated.Note = strings.TrimSpace(noteS.Get())
		props.OnSave(updated)
		editing.Set(false)
	}))
	del := ui.UseEvent(Prevent(func() { props.OnDelete(e) }))
	view := ui.UseEvent(Prevent(func() { props.OnView(e) }))

	if editing.Get() {
		return Div(css.Class("card", tw.P3), Attr("data-testid", "event-row-"+e.ID),
			Div(css.Class(tw.Grid, tw.Gap2),
				Input(css.Class("field"), Type("text"), Value(nameS.Get()), OnInput(onName)),
				Div(css.Class(tw.Flex, tw.Gap2),
					Input(css.Class("field", tw.Flex1), Type("date"), Value(startS.Get()), OnInput(onStart)),
					Input(css.Class("field", tw.Flex1), Type("date"), Value(endS.Get()), OnInput(onEnd)),
				),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("events.notePh")), Value(noteS.Get()), OnInput(onNote)),
				If(errS.Get() != "", Div(css.Class("form-error"), Attr("role", "alert"), errS.Get())),
				Div(css.Class(tw.Flex, tw.Gap2),
					Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "event-row-save-"+e.ID),
						OnClick(saveEdit), uistate.T("events.save")),
					Button(css.Class("btn btn-ghost"), Type("button"), OnClick(cancelEdit), uistate.T("events.cancel")),
				),
			),
		)
	}

	return Div(css.Class("card", tw.P3, tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2),
		Attr("data-testid", "event-row-"+e.ID),
		Div(css.Class(tw.Flex1),
			Div(css.Class(tw.FontSemibold), e.Name),
			Div(css.Class("text-dim", tw.Text13), formatEventRange(e)),
			If(e.Note != "", Div(css.Class("text-dim", tw.Text13), e.Note)),
			Div(css.Class("text-dim", tw.Text13), uistate.T("events.rowMeta", props.Count, props.SpendStr)),
		),
		Div(css.Class(tw.InlineFlex, tw.ItemsCenter, tw.Gap15),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"), Attr("data-testid", "event-view-"+e.ID),
				Title(uistate.T("events.viewTitle")), OnClick(view), uistate.T("events.view")),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"), Attr("data-testid", "event-edit-"+e.ID),
				OnClick(startEdit), uistate.T("events.edit")),
			Button(css.Class("btn btn-ghost btn-sm"), Type("button"), Attr("data-testid", "event-delete-"+e.ID),
				OnClick(del), uistate.T("events.delete")),
		),
	)
}
