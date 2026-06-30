// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"math"
	"runtime/debug"
	"strconv"
	"syscall/js"

	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// VirtualSpec configures DataTable's windowed (virtualized) body. Instead of
// materializing every row, the table renders only the rows in (and near) the
// viewport, with two spacer rows whose heights stand in for the off-screen rows —
// so the scroll bar, scroll height, and row positions are all correct while the DOM
// holds only a few dozen <tr>s. This keeps a list of thousands of rows smooth.
//
// The window is recomputed from the scroll container's position on every scroll /
// resize; the per-index RowAt is called only for the visible slice.
type VirtualSpec struct {
	Count     int                 // total number of rows
	RowHeight int                 // fixed row height in px (rows must be uniform)
	Overscan  int                 // extra rows rendered beyond the viewport each side (default 8)
	Scroller  string              // CSS selector of the scroll container (default "main.cf-scroll")
	ColSpan   int                 // columns the spacer rows span (defaults to the table's column count)
	RowAt     func(i int) ui.Node // builds the <tr> for row i — called only for the window
	KeyAt     func(i int) any     // stable key for row i (for keyed reconciliation)
}

// vWindow is the currently-rendered row window: first visible row index + how many
// rows fit the viewport. Stored in state and updated from the scroll handler.
type vWindow struct {
	first int
	count int
}

// recoverVirtual contains a panic in a raw scroll/resize callback (which runs
// outside the framework's render+recover) so it logs instead of killing the app.
func recoverVirtual() {
	if r := recover(); r != nil {
		if c := js.Global().Get("console"); c.Truthy() {
			c.Call("error", fmt.Sprintf("[dt-virtual] recovered panic: %v\n%s", r, debug.Stack()))
		}
	}
}

// deferMacrotask runs fn on the next macrotask (setTimeout 0), so a state change made
// just before the call paints first — e.g. showing the sort spinner before the heavier
// re-sort runs. The js.Func is released after it fires; a panic is contained so a raw
// timer callback can't take down the wasm app.
func deferMacrotask(fn func()) {
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		defer cb.Release()
		defer func() {
			if r := recover(); r != nil {
				if c := js.Global().Get("console"); c.Truthy() {
					c.Call("error", fmt.Sprintf("[ui] deferred task panic: %v", r))
				}
			}
		}()
		fn()
		return nil
	})
	js.Global().Call("setTimeout", cb, 0)
}

// dtVirtualBody is DataTable's windowed <tbody>. It owns the scroll listener +
// window state (its own component, so the scroll-driven re-renders stay local to the
// table body rather than re-rendering the whole surface). It measures its own tbody's
// position relative to the scroll container, so the window tracks the real scroll
// offset even as tiles above the table change height.
func dtVirtualBody(vs VirtualSpec) ui.Node {
	rowH := vs.RowHeight
	if rowH <= 0 {
		rowH = 35
	}
	overscan := vs.Overscan
	if overscan <= 0 {
		overscan = 8
	}
	scrollerSel := vs.Scroller
	if scrollerSel == "" {
		scrollerSel = "main.cf-scroll"
	}

	win := ui.UseState(vWindow{first: 0, count: 40})

	// Attach scroll (on the container) + resize (on the window) listeners; recompute
	// which rows are near the viewport and store the window. fastEqual skips the state
	// set when the window is unchanged, so a scroll only re-renders the body when the
	// row window actually shifts (≈ every rowH px). Cleanup releases the js.Funcs.
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		win2 := js.Global()
		if !doc.Truthy() {
			return nil
		}
		compute := func() {
			defer recoverVirtual()
			scroller := doc.Call("querySelector", scrollerSel)
			tbody := doc.Call("querySelector", "tbody.dt-vbody")
			if !scroller.Truthy() || !tbody.Truthy() {
				return
			}
			sTop := scroller.Call("getBoundingClientRect").Get("top").Float()
			tTop := tbody.Call("getBoundingClientRect").Get("top").Float()
			// How far the list's top has scrolled above the container's top edge. tbody.top
			// is row 0's virtual position (the top spacer starts there), so this tracks the
			// real scroll offset into the list and self-corrects against spacer math.
			pixelsIn := sTop - tTop
			if pixelsIn < 0 {
				pixelsIn = 0
			}
			viewH := scroller.Get("clientHeight").Float()
			first := int(pixelsIn) / rowH
			count := int(math.Ceil(viewH/float64(rowH))) + 1
			win.Set(vWindow{first: first, count: count})
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any { compute(); return nil })
		scroller := doc.Call("querySelector", scrollerSel)
		if scroller.Truthy() {
			scroller.Call("addEventListener", "scroll", cb)
		}
		win2.Call("addEventListener", "resize", cb)
		compute() // initial window on mount
		return func() {
			if scroller.Truthy() {
				scroller.Call("removeEventListener", "scroll", cb)
			}
			win2.Call("removeEventListener", "resize", cb)
			cb.Release()
		}
	}, scrollerSel)

	w := win.Get()
	total := vs.Count
	if total < 0 {
		total = 0
	}
	start := w.first - overscan
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := w.first + w.count + overscan
	if end > total {
		end = total
	}
	if end < start {
		end = start
	}

	topH := start * rowH
	botH := (total - end) * rowH
	if botH < 0 {
		botH = 0
	}

	idxs := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		idxs = append(idxs, i)
	}
	windowRows := MapKeyed(idxs,
		func(i int) any { return vs.KeyAt(i) },
		func(i int) ui.Node { return vs.RowAt(i) },
	)

	return Tbody(
		css.Class("dt-vbody"),
		dtSpacerRow(topH, vs.ColSpan, "dt-vspacer-top"),
		windowRows,
		dtSpacerRow(botH, vs.ColSpan, "dt-vspacer-bot"),
	)
}

// dtSpacerRow is a zero-content row whose single colspanning cell stands in for the
// off-screen rows above/below the window, holding the table's scroll height. It is
// aria-hidden so assistive tech ignores the filler.
func dtSpacerRow(heightPx, colSpan int, cls string) ui.Node {
	if colSpan <= 0 {
		colSpan = 1
	}
	return Tr(ClassStr(cls), Attr("aria-hidden", "true"),
		Td(Attr("colspan", strconv.Itoa(colSpan)),
			Style(map[string]string{"height": strconv.Itoa(heightPx) + "px", "padding": "0", "border": "0"})))
}
