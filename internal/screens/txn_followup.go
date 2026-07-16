// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// followUpItem is one linked to-do, shown in a transaction's follow-up hover popover.
type followUpItem struct {
	Title string
	Done  bool
	Due   string // pre-formatted due date; "" when none
}

// followUpInfo is a transaction's follow-up tally plus the items behind it.
type followUpInfo struct {
	Open, Total int
	Items       []followUpItem
}

// followUpInfoByTxn groups follow-up tasks by their linked transaction id (open/total +
// a display list), built once from the task list so each row reads it in O(1).
func followUpInfoByTxn(tasks []domain.Task, formatDue func(time.Time) string) map[string]followUpInfo {
	m := make(map[string]followUpInfo)
	for _, t := range tasks {
		if t.RelatedType != domain.RelatedTransaction || t.RelatedID == "" {
			continue
		}
		info := m[t.RelatedID]
		info.Total++
		done := t.Status == domain.StatusDone
		if !done {
			info.Open++
		}
		due := ""
		if !t.Due.IsZero() {
			due = formatDue(t.Due)
		}
		info.Items = append(info.Items, followUpItem{Title: t.Title, Done: done, Due: due})
		m[t.RelatedID] = info
	}
	return m
}

// followUpChipText renders the chip figure "open/total" (e.g. "1/2").
func followUpChipText(open, total int) string { return strconv.Itoa(open) + "/" + strconv.Itoa(total) }

// followUpChipMod tones the chip: accented while any follow-up is open, muted once done.
func followUpChipMod(open int) string {
	if open > 0 {
		return " has-open"
	}
	return " all-done"
}

// txnFollowUpChipProps configure one row's follow-up chip + hover popover.
type txnFollowUpChipProps struct {
	TxnID  string
	Open   int
	Total  int
	Items  []followUpItem
	OnOpen func() // click → the To-do list filtered to transaction-linked tasks
}

// txnFollowUpChip is the per-row follow-up affordance: a small "open/total" chip that,
// after ~500ms of hover, reveals an anchored popover listing the linked to-dos for a
// quick glance without leaving the page — and, on click, jumps to the filtered To-do
// list. Its own component so its state/hover hooks sit at stable positions, never inside
// the row's map loop.
func txnFollowUpChip(props txnFollowUpChipProps) ui.Node {
	open := ui.UseState(false)
	hovering := ui.UseRef(false)
	wrapID := "txnfu-" + props.TxnID

	uiw.DismissPopover(open.Get(), wrapID, func() { open.Set(false) })
	uiw.AnchorPopover(open.Get(), wrapID)

	// `hovering` tracks the pointer being over the chip OR the popover (one combined hover
	// region), so the same enter/leave handlers wire to both — that's what lets the mouse
	// cross the small gap from the chip into the popover without it despawning.
	//
	// enter: reveal only after 500ms of CONTINUOUS hover (a pointer merely passing over the
	// row never flashes it) — and cancel any pending close.
	enter := func() {
		hovering.Set(true)
		if open.Get() {
			return // already open; the flag cancels the grace-period close
		}
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			if hovering.Get() {
				open.Set(true)
			}
			cb.Release()
			return nil
		})
		js.Global().Call("setTimeout", cb, 500)
	}
	// leave: don't despawn instantly — wait a short grace period so the pointer can bridge
	// the chip→popover gap. The callback re-reads the live flag, so re-entering (chip or
	// popover) within the window keeps it open.
	leave := func() {
		hovering.Set(false)
		var cb js.Func
		cb = js.FuncOf(func(js.Value, []js.Value) any {
			if !hovering.Get() {
				open.Set(false)
			}
			cb.Release()
			return nil
		})
		js.Global().Call("setTimeout", cb, 240)
	}
	onEnter := ui.UseEvent(func(e ui.Event) { enter() })
	onLeave := ui.UseEvent(func(e ui.Event) { leave() })
	// Click still navigates (StopPropagation so the chip doesn't also open the row's edit
	// modal). The popover is glance-only; the chip is the way to open the full list.
	onClick := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		if props.OnOpen != nil {
			props.OnOpen()
		}
	})

	var pop ui.Node = Fragment()
	if open.Get() {
		kids := []any{ClassStr("add-menu txnfu-pop"), Attr("role", "dialog"),
			Attr("data-testid", "txn-followup-pop-"+props.TxnID),
			// Hovering the popover keeps it open (cancels the grace-period close), so the
			// pointer can move off the chip and read/interact with the list.
			OnMouseEnter(onEnter), OnMouseLeave(onLeave),
			Div(css.Class("txnfu-pop-head"), uistate.T("transactions.followUpsPopHead", props.Open, props.Total)),
		}
		for _, it := range props.Items {
			cls := "txnfu-item"
			ic := icon.CheckCircle
			if it.Done {
				cls += " is-done"
				ic = icon.Check
			}
			kids = append(kids, Div(ClassStr(cls),
				uiw.Icon(ic, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
				Span(css.Class("txnfu-item-title"), it.Title),
				If(it.Due != "", Span(css.Class("txnfu-item-due"), it.Due)),
			))
		}
		kids = append(kids, Button(css.Class("txnfu-pop-foot"), Type("button"),
			Attr("data-testid", "txn-followup-pop-link-"+props.TxnID), OnClick(onClick),
			uistate.T("transactions.followUpsPopLink")))
		pop = Div(kids...)
	}

	return Span(ClassStr("txn-followup-wrap add-wrap"), Attr("id", wrapID),
		OnMouseEnter(onEnter), OnMouseLeave(onLeave),
		Button(ClassStr("txn-followup-chip"+followUpChipMod(props.Open)), Type("button"),
			Attr("data-testid", "txn-followup-chip-"+props.TxnID),
			Attr("aria-haspopup", "dialog"), Attr("aria-expanded", ariaBool(open.Get())),
			// No native `title` here — it would fight the hover popover with a second,
			// redundant tooltip. aria-label carries the summary for screen readers.
			Attr("aria-label", uistate.T("transactions.followUpsAria", props.Open)),
			OnClick(onClick),
			uiw.Icon(icon.CheckCircle, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
			Span(followUpChipText(props.Open, props.Total))),
		pop,
	)
}
