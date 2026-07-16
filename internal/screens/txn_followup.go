// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// followUpItem is one linked to-do, shown in a transaction's follow-up hover popover.
type followUpItem struct {
	ID    string
	Title string
	Done  bool
	Due   string    // pre-formatted due date; "" when none
	dueT  time.Time // raw due, for sorting (zero = no due date)
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
		// Drop a redundant "Follow up:" prefix (tasks created before that prefix was
		// removed): in this popover — already under a charge's follow-up chip — it just
		// wastes the limited width.
		title := strings.TrimSpace(strings.TrimPrefix(t.Title, "Follow up:"))
		if title == "" {
			title = t.Title
		}
		info.Items = append(info.Items, followUpItem{ID: t.ID, Title: title, Done: done, Due: due, dueT: t.Due})
		m[t.RelatedID] = info
	}
	// Order each list so the popover can show the most relevant follow-ups first: open
	// before done, then soonest-due first (no due date last).
	for k := range m {
		sort.SliceStable(m[k].Items, func(i, j int) bool {
			a, b := m[k].Items[i], m[k].Items[j]
			if a.Done != b.Done {
				return !a.Done // open first
			}
			if a.dueT.IsZero() != b.dueT.IsZero() {
				return !a.dueT.IsZero() // dated before undated
			}
			return a.dueT.Before(b.dueT) // soonest first
		})
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

// toggleFollowUpTask flips a linked to-do's done state in place (open ↔ done) and bumps
// the data revision so the transactions surface re-reads the counts. Mirrors the to-do
// list's own check-off: completing goes through CompleteTask (which spawns a recurring
// task's next occurrence atomically); re-opening is a plain PutTask.
func toggleFollowUpTask(taskID string, currentlyDone bool) {
	app := appstate.Default
	if app == nil {
		return
	}
	if currentlyDone {
		for _, t := range app.Tasks() {
			if t.ID == taskID {
				t.Status = domain.StatusOpen
				_ = app.PutTask(t)
				break
			}
		}
	} else {
		_ = app.CompleteTask(taskID, id.New(), time.Now())
	}
	uistate.BumpDataRevision()
}

// txnFollowUpItemProps configure one to-do row inside the follow-up popover.
type txnFollowUpItemProps struct {
	ID    string
	Title string
	Done  bool
	Due   string
}

// txnFollowUpItem is one to-do line in the popover with a check-off toggle, so a
// follow-up can be marked done / re-opened right there without leaving the page. Its own
// component so the toggle hook sits at a stable position (never inside the popover's map
// loop).
func txnFollowUpItem(props txnFollowUpItemProps) ui.Node {
	onToggle := ui.UseEvent(func(e ui.Event) {
		e.StopPropagation()
		toggleFollowUpTask(props.ID, props.Done)
	})
	cls := "txnfu-item"
	checkCls := "txnfu-item-check"
	label := uistate.T("transactions.followUpMarkDone")
	var checkGlyph ui.Node = Fragment()
	if props.Done {
		cls += " is-done"
		checkCls += " is-done"
		label = uistate.T("transactions.followUpMarkOpen")
		checkGlyph = uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W3, tw.H3))
	}
	return Div(ClassStr(cls),
		Button(ClassStr(checkCls), Type("button"),
			Attr("role", "checkbox"), Attr("aria-checked", ariaBool(props.Done)),
			Attr("aria-label", label+" — "+props.Title), Title(label),
			OnClick(onToggle), checkGlyph),
		Span(css.Class("txnfu-item-title"), props.Title),
		If(props.Due != "", Span(css.Class("txnfu-item-due"), props.Due)),
	)
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
		// Show at most the top 3 OPEN follow-ups (items are pre-sorted open-first by due),
		// so a charge with many linked to-dos never floods the glance. Done items and any
		// overflow live in the full To-do list behind the footer link.
		const maxShown = 3
		shown := 0
		for _, it := range props.Items {
			if it.Done || shown >= maxShown {
				continue
			}
			kids = append(kids, ui.CreateElement(txnFollowUpItem, txnFollowUpItemProps{
				ID: it.ID, Title: it.Title, Done: it.Done, Due: it.Due,
			}))
			shown++
		}
		switch {
		case props.Open == 0:
			kids = append(kids, Div(css.Class("txnfu-empty"), uistate.T("transactions.followUpsAllDone")))
		case props.Open > shown:
			kids = append(kids, Div(css.Class("txnfu-more"), uistate.T("transactions.followUpsMore", props.Open-shown)))
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
