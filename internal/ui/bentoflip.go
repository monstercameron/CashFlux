// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"runtime/debug"
	"syscall/js"
)

// recoverBento turns a panic in a bento drag/FLIP callback into a logged console
// error (with stack) instead of a fatal wasm crash. These callbacks run as raw
// js.FuncOf / rAF / setTimeout handlers OUTSIDE the framework's render+recover
// context, so an unguarded panic there kills the whole Go program ("Go program has
// already exited") and freezes the app — exactly the reported drag-freeze symptom.
// label names the call site so the logged error pinpoints the culprit.
func recoverBento(label string) {
	if r := recover(); r != nil {
		if c := js.Global().Get("console"); c.Truthy() {
			c.Call("error", fmt.Sprintf("[bento] recovered panic in %s: %v\n%s", label, r, debug.Stack()))
		}
	}
}

// safeFunc wraps a coordinator event/timer callback with recoverBento so a panic
// logs its stack and is contained, instead of taking down the whole app.
func safeFunc(label string, fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		defer recoverBento(label)
		return fn(this, args)
	})
}

// This file is the Go port of the former web/flip.js helper: it owns the
// dashboard bento's drag-reorder coordinator and FLIP reflow animation entirely
// in wasm, so no hand-written helper JavaScript is needed.
//
// Three behaviors live here:
//   - FlipBento: a FLIP animation — remember each tile's screen position, and on
//     the next layout-changing render measure the new position, jump each tile
//     back to where it was (no transition), then transition the offset to zero so
//     it glides to its new slot. CSS-grid placement doesn't transition on its own.
//   - The drag-target coordinator (bentoDragStart/Target/End): a stable insertion
//     target snapshotted at drag start, with hysteresis, so the preview doesn't
//     oscillate when FLIP-animated tiles pass under the pointer (B2).
//   - A scroll lock: pins the scroll position during a drag so the native HTML5
//     drag auto-scroll doesn't fight the reflow.
//
// InitBentoCoordinator wires the autonomous document listeners (called once at
// boot). widget.go calls bentoDragStart/Target/End directly; the dashboard calls
// FlipBento from its layout-signature effect.

const bentoSelector = ".bento > .w[data-widget]"

type bentoZone struct {
	id                       string
	left, right, top, bottom float64
	cx, cy                   float64
}

type bentoDragState struct {
	active    bool
	sourceID  string
	zones     []bentoZone
	targetID  string
	scrollEl  js.Value
	scrollTop float64
	windowY   float64
}

var (
	bentoPrev      = map[string][2]float64{} // last-seen tile positions for FLIP
	bentoDrag      bentoDragState
	bentoArmed     bool       // scroll lock armed (pointerdown) before a drag begins
	bentoArmScroll js.Value   // the scroller whose position is pinned
	bentoArmTop    float64    // pinned scrollTop
	bentoArmWinY   float64    // pinned window scrollY
	scrollLockOn   bool       // the scroll-lock rAF loop is running
	scrollLockFn   js.Func    // reused rAF callback (allocated once → no per-frame leak)
	flipRAFFn      js.Func    // reused rAF callback for the FLIP settle frame
	flipMovers     []js.Value // tiles to settle on the next FLIP frame
	retargeting    bool       // guards against re-entrant synthetic dragover/drop
	dimFn          js.Func    // reused rAF callback that dims the dragged tile a frame after dragstart
)

// endDragFully ends an in-flight or abandoned drag from a context where the native
// dragend may never arrive (a pointerup, Escape): it tears down the coordinator state,
// un-dims the tile, and releases the scroll lock. Cheap to call spuriously — it no-ops
// when nothing is active or armed.
func endDragFully() {
	if bentoDrag.active || bentoArmed {
		bentoDragEnd()
	}
}

func doc() js.Value { return js.Global().Get("document") }

func nowMs() float64 { return js.Global().Get("performance").Call("now").Float() }

// lastDragActivityMs is the timestamp of the last drag progress (dragstart or a
// dragover that moved the target). The scroll-lock watchdog uses it to force-end a
// drag that got stuck — e.g. a native drag the browser aborted without firing dragend
// (seen on Chrome 150 if the source element is mutated mid-drag) — so the page can't be
// left permanently locked (tile dimmed, cursor stuck "grabbing", scroll snapping back).
var lastDragActivityMs float64

func reduceMotion() bool {
	mm := js.Global().Get("matchMedia")
	if mm.Type() != js.TypeFunction {
		return false
	}
	return js.Global().Call("matchMedia", "(prefers-reduced-motion: reduce)").Get("matches").Bool()
}

func scrollHost() js.Value {
	d := doc()
	if !d.Truthy() {
		return js.Null()
	}
	if el := d.Call("querySelector", "main.cf-scroll"); el.Truthy() {
		return el
	}
	if el := d.Get("scrollingElement"); el.Truthy() {
		return el
	}
	return d.Get("documentElement")
}

func bentoCenterRect(r js.Value) (float64, float64) {
	return r.Get("left").Float() + r.Get("width").Float()/2, r.Get("top").Float() + r.Get("height").Float()/2
}

// restoreDragScroll re-pins the scroller and window to their locked positions,
// undoing any native drag auto-scroll that happened this frame.
func restoreDragScroll() {
	el, top, winY := js.Null(), 0.0, 0.0
	switch {
	case bentoDrag.active:
		el, top, winY = bentoDrag.scrollEl, bentoDrag.scrollTop, bentoDrag.windowY
	case bentoArmed:
		el, top, winY = bentoArmScroll, bentoArmTop, bentoArmWinY
	default:
		return
	}
	if el.Truthy() && el.Get("scrollTop").Float() != top {
		el.Set("scrollTop", top)
	}
	if js.Global().Get("scrollY").Float() != winY {
		js.Global().Call("scrollTo", js.Global().Get("scrollX"), winY)
	}
}

func startScrollLock() {
	if scrollLockOn {
		return
	}
	scrollLockOn = true
	js.Global().Call("requestAnimationFrame", scrollLockFn)
}

func armScrollLock(sourceID string) {
	sc := scrollHost()
	bentoArmed = true
	bentoArmScroll = sc
	if sc.Truthy() {
		bentoArmTop = sc.Get("scrollTop").Float()
	} else {
		bentoArmTop = 0
	}
	bentoArmWinY = js.Global().Get("scrollY").Float()
	if d := doc(); d.Truthy() {
		d.Get("documentElement").Call("setAttribute", "data-bento-dragging", orTrue(sourceID))
	}
	startScrollLock()
}

func clearScrollLock() {
	bentoArmed = false
	if !bentoDrag.active {
		if d := doc(); d.Truthy() {
			d.Get("documentElement").Call("removeAttribute", "data-bento-dragging")
		}
	}
}

func orTrue(s string) string {
	if s == "" {
		return "true"
	}
	return s
}

// bentoDragStart snapshots the pre-drag tile geometry into stable zones so the
// insertion target stays tied to pointer travel, not to animated siblings.
func bentoDragStart(sourceID string) {
	defer recoverBento("bentoDragStart")
	if sourceID == "" {
		return
	}
	d := doc()
	if !d.Truthy() {
		return
	}
	nodes := d.Call("querySelectorAll", bentoSelector)
	n := nodes.Get("length").Int()
	zones := make([]bentoZone, 0, n)
	for i := 0; i < n; i++ {
		el := nodes.Index(i)
		id := el.Call("getAttribute", "data-widget").String()
		if id == "" || id == sourceID {
			continue
		}
		r := el.Call("getBoundingClientRect")
		cx, cy := bentoCenterRect(r)
		zones = append(zones, bentoZone{
			id: id, left: r.Get("left").Float(), right: r.Get("right").Float(),
			top: r.Get("top").Float(), bottom: r.Get("bottom").Float(), cx: cx, cy: cy,
		})
	}
	sc := bentoArmScroll
	if !sc.Truthy() {
		sc = scrollHost()
	}
	top := bentoArmTop
	winY := bentoArmWinY
	if !bentoArmed {
		if sc.Truthy() {
			top = sc.Get("scrollTop").Float()
		}
		winY = js.Global().Get("scrollY").Float()
	}
	bentoDrag = bentoDragState{active: true, sourceID: sourceID, zones: zones, scrollEl: sc, scrollTop: top, windowY: winY}
	lastDragActivityMs = nowMs()            // arm the stuck-drag watchdog
	lastDropSource, lastDropTarget = "", "" // fresh drag: forget the prior outcome
	d.Get("documentElement").Call("setAttribute", "data-bento-dragging", orTrue(sourceID))
	// Dim on the NEXT frame, not synchronously here: .w.drag sets pointer-events:none,
	// and mutating the dragged element that way DURING the dragstart event deadlocks the
	// browser's native drag (the drag never progresses). Deferring a frame lets the drag
	// establish first — this matches the old behavior, where the dim came from a
	// re-render scheduled after dragstart, not inline.
	if dimFn.Truthy() {
		js.Global().Call("requestAnimationFrame", dimFn)
	}
	startScrollLock()
}

// CurrentDragSource returns the id of the tile currently being dragged ("" when
// none). The per-tile drop handler reads it to know what to reorder — without
// subscribing to a state atom, so a drag triggers no re-render until the drop.
func CurrentDragSource() string { return bentoDrag.sourceID }

// lastDropSource / lastDropTarget capture the source + insertion target of the most
// recent drag at the moment it ended — stashed BEFORE bentoDragEnd resets the live
// state, because the coordinator's capture-phase drop/dragend listeners run before the
// per-tile bubble-phase OnDragEnd that performs the reorder. The reorder reads these.
var (
	lastDropSource string
	lastDropTarget string
)

// LastDropSource / LastDropTarget expose the stashed src/target of the just-ended drag.
func LastDropSource() string { return lastDropSource }
func LastDropTarget() string { return lastDropTarget }

// stashDropOutcome records the current drag's source + target so the per-tile
// OnDragEnd can reorder after the coordinator has torn the drag down. Only records
// while a drag is active and a target exists, so a trailing dragend (after a drop
// already ended the drag) can't clobber a good target with an empty one.
func stashDropOutcome() {
	if bentoDrag.active && bentoDrag.targetID != "" {
		lastDropSource = bentoDrag.sourceID
		lastDropTarget = bentoDrag.targetID
	}
}

// dimSourceTile / undimSourceTiles toggle the dragged tile's dimmed look (.w.drag)
// directly in the DOM. Doing this here (instead of via a state atom the widgets read)
// is the whole point: a state change would re-render the data-heavy dashboard on a
// large ledger and freeze the drag. The class is purely visual; CSS owns the styling.
func dimSourceTile(id string) {
	if t := tileByID(id); t.Truthy() {
		t.Get("classList").Call("add", "drag")
	}
}

func undimSourceTiles() {
	d := doc()
	if !d.Truthy() {
		return
	}
	dimmed := d.Call("querySelectorAll", ".bento > .w.drag")
	for i := 0; i < dimmed.Get("length").Int(); i++ {
		dimmed.Index(i).Get("classList").Call("remove", "drag")
	}
}

func bentoDragEnd() {
	defer recoverBento("bentoDragEnd")
	restoreDragScroll()
	undimSourceTiles()
	bentoDrag = bentoDragState{}
	clearScrollLock()
}

func zoneContains(z bentoZone, x, y, pad float64) bool {
	return x >= z.left-pad && x <= z.right+pad && y >= z.top-pad && y <= z.bottom+pad
}

// bentoDragTarget returns the stable insertion target for the pointer at (x,y).
// Hysteresis keeps the current target while the pointer stays near its zone, so a
// FLIP-animated tile that slides under the pointer can't steal the target.
func bentoDragTarget(x, y float64) (result string) {
	defer recoverBento("bentoDragTarget")
	if !bentoDrag.active || len(bentoDrag.zones) == 0 {
		return ""
	}
	lastDragActivityMs = nowMs() // drag is progressing — keep the watchdog quiet
	restoreDragScroll()
	if isNaN(x) || isNaN(y) {
		return bentoDrag.targetID
	}
	if bentoDrag.targetID != "" {
		for _, z := range bentoDrag.zones {
			if z.id == bentoDrag.targetID && zoneContains(z, x, y, 18) {
				return bentoDrag.targetID
			}
		}
	}
	bestID := ""
	bestScore := 1e18
	for _, z := range bentoDrag.zones {
		dx, dy := x-z.cx, y-z.cy
		score := dx*dx + dy*dy
		if zoneContains(z, x, y, 0) {
			score -= 1000000
		}
		if score < bestScore {
			bestScore = score
			bestID = z.id
		}
	}
	bentoDrag.targetID = bestID
	return bestID
}

func isNaN(f float64) bool { return f != f }

// FlipBento animates every bento tile from its previous screen position to its
// new one. Call it after each layout-changing render. Honors reduced motion (then
// it only records positions).
func FlipBento() {
	defer recoverBento("FlipBento")
	d := doc()
	if !d.Truthy() {
		return
	}
	restoreDragScroll()
	nodes := d.Call("querySelectorAll", bentoSelector)
	n := nodes.Get("length").Int()
	reduce := reduceMotion()
	next := make(map[string][2]float64, n)
	flipMovers = flipMovers[:0]
	dragging := ""
	if bentoDrag.active {
		dragging = bentoDrag.sourceID
	}
	for i := 0; i < n; i++ {
		el := nodes.Index(i)
		id := el.Call("getAttribute", "data-widget").String()
		if id == "" {
			continue
		}
		r := el.Call("getBoundingClientRect")
		left, top := r.Get("left").Float(), r.Get("top").Float()
		next[id] = [2]float64{left, top}
		style := el.Get("style")
		if dragging != "" && id == dragging {
			style.Set("transition", "")
			style.Set("transform", "")
			continue
		}
		old, ok := bentoPrev[id]
		if !ok || reduce {
			continue
		}
		dx, dy := old[0]-left, old[1]-top
		if dx == 0 && dy == 0 {
			continue
		}
		style.Set("transition", "none")
		style.Set("transform", fmt.Sprintf("translate(%gpx,%gpx)", dx, dy))
		flipMovers = append(flipMovers, el)
	}
	bentoPrev = next
	if len(flipMovers) > 0 {
		d.Get("body").Call("getBoundingClientRect") // force reflow so offsets paint first
		js.Global().Call("requestAnimationFrame", flipRAFFn)
	}
}

// tileByID returns the bento tile element with the given data-widget id, or a null
// Value. It uses CSS.escape on the id so an id with a quote, colon, or other CSS-
// special character can't make querySelector throw (which, in a raw js callback,
// would panic and take down the app). The former web/flip.js escaped quotes for the
// same reason; CSS.escape is stricter and covers every special character.
func tileByID(id string) js.Value {
	d := doc()
	if !d.Truthy() || id == "" {
		return js.Null()
	}
	esc := id
	if cssNS := js.Global().Get("CSS"); cssNS.Truthy() && cssNS.Get("escape").Type() == js.TypeFunction {
		esc = cssNS.Call("escape", id).String()
	}
	return d.Call("querySelector", bentoSelector+`[data-widget="`+esc+`"]`)
}

func tileFromEvent(e js.Value) js.Value {
	t := e.Get("target")
	if !t.Truthy() || t.Get("closest").Type() != js.TypeFunction {
		return js.Null()
	}
	return t.Call("closest", bentoSelector)
}

// InitBentoCoordinator registers the autonomous document listeners (scroll-lock
// arming + drag retargeting) and the reusable rAF callbacks. Call once at boot.
func InitBentoCoordinator() {
	d := doc()
	if !d.Truthy() {
		return
	}

	scrollLockFn = safeFunc("scrollLock", func(js.Value, []js.Value) any {
		if !bentoDrag.active && !bentoArmed {
			scrollLockOn = false
			return nil
		}
		// Watchdog: if a drag has been active with no progress for 1.5s, the native drag
		// was almost certainly aborted without a dragend — force-end it so the scroll lock
		// and dimmed tile can't stay stuck forever.
		if bentoDrag.active && nowMs()-lastDragActivityMs > 1500 {
			endDragFully()
			scrollLockOn = false
			return nil
		}
		restoreDragScroll()
		js.Global().Call("requestAnimationFrame", scrollLockFn)
		return nil
	})
	flipRAFFn = safeFunc("flipRAF", func(js.Value, []js.Value) any {
		for _, el := range flipMovers {
			st := el.Get("style")
			st.Set("transition", "transform .16s cubic-bezier(.2,.8,.2,1)")
			st.Set("transform", "")
		}
		flipMovers = flipMovers[:0]
		return nil
	})
	dimFn = safeFunc("dim", func(js.Value, []js.Value) any {
		if bentoDrag.active {
			dimSourceTile(bentoDrag.sourceID)
		}
		return nil
	})

	// Arm the scroll lock as soon as a tile is pressed (drag starts only after some
	// movement, so arming on pointerdown captures the pre-drag scroll position).
	armOnPress := safeFunc("armOnPress", func(_ js.Value, args []js.Value) any {
		e := args[0]
		tile := tileFromEvent(e)
		if !tile.Truthy() {
			return nil
		}
		if e.Get("button").Truthy() && e.Get("button").Int() != 0 {
			return nil
		}
		// Don't arm for interactive controls inside the tile (buttons, selects, …).
		if t := e.Get("target"); t.Truthy() && t.Get("closest").Type() == js.TypeFunction {
			if t.Call("closest", "button,a,input,select,textarea").Truthy() {
				return nil
			}
		}
		armScrollLock(tile.Call("getAttribute", "data-widget").String())
		return nil
	})
	d.Call("addEventListener", "pointerdown", armOnPress, true)
	d.Call("addEventListener", "mousedown", armOnPress, true)

	d.Call("addEventListener", "dragstart", safeFunc("dragstart", func(_ js.Value, args []js.Value) any {
		tile := tileFromEvent(args[0])
		if tile.Truthy() {
			bentoDragStart(tile.Call("getAttribute", "data-widget").String())
		}
		return nil
	}), true)

	// Retarget native dragover to the STABLE tile when the browser's hit-test (over
	// an animated tile) differs from the snapshotted insertion target.
	d.Call("addEventListener", "dragover", safeFunc("dragover", func(_ js.Value, args []js.Value) any {
		e := args[0]
		restoreDragScroll()
		if retargeting || !bentoDrag.active {
			return nil
		}
		stableID := bentoDragTarget(e.Get("clientX").Float(), e.Get("clientY").Float())
		if stableID == "" {
			return nil
		}
		tile := tileFromEvent(e)
		curID := ""
		if tile.Truthy() {
			curID = tile.Call("getAttribute", "data-widget").String()
		}
		if curID == "" || curID == stableID {
			return nil
		}
		stableTile := tileByID(stableID)
		if !stableTile.Truthy() {
			return nil
		}
		e.Call("preventDefault")
		e.Call("stopImmediatePropagation")
		retargeting = true
		stableTile.Call("dispatchEvent", newDragEvent("dragover", e))
		retargeting = false
		return nil
	}), true)

	d.Call("addEventListener", "scroll", safeFunc("scroll", func(js.Value, []js.Value) any {
		restoreDragScroll()
		return nil
	}), true)

	// Retarget drop to the stable tile too, then end the drag.
	d.Call("addEventListener", "drop", safeFunc("drop", func(_ js.Value, args []js.Value) any {
		e := args[0]
		if !retargeting && bentoDrag.active {
			bentoDragTarget(e.Get("clientX").Float(), e.Get("clientY").Float()) // refresh target from drop coords
			stashDropOutcome()                                                  // record src+target before teardown
			stableID := bentoDragTarget(e.Get("clientX").Float(), e.Get("clientY").Float())
			tile := tileFromEvent(e)
			curID := ""
			if tile.Truthy() {
				curID = tile.Call("getAttribute", "data-widget").String()
			}
			if stableID != "" && curID != "" && stableID != curID {
				stableTile := tileByID(stableID)
				if stableTile.Truthy() {
					e.Call("preventDefault")
					e.Call("stopImmediatePropagation")
					retargeting = true
					stableTile.Call("dispatchEvent", newDragEvent("drop", e))
					retargeting = false
					bentoDragEnd()
					return nil
				}
			}
		}
		bentoDragEnd()
		return nil
	}), true)

	// dragend always fires at the end of a drag (on the source tile). The outcome was
	// already stashed; the per-tile OnDragEnd performs the single reorder. Here we just
	// tear the drag down (un-dim + release scroll lock).
	d.Call("addEventListener", "dragend", safeFunc("dragend", func(js.Value, []js.Value) any {
		stashDropOutcome() // capture outcome for a drop outside any tile
		bentoDragEnd()
		return nil
	}), true)

	// pointerup / mouseup / pointercancel: release the scroll-lock arm, exactly as the
	// original flip.js did. We deliberately do NOT clear the drag atoms here: a native
	// drag begins by firing pointercancel (the pointer gesture converts to a drag), so
	// clearing the source/preview atoms on these events would wipe the just-started drag
	// — leaving the tile un-dimmed and the reorder dead. The dim atoms are cleared only
	// on the true end events (dragend / drop) below.
	clear := safeFunc("pointerEnd", func(js.Value, []js.Value) any { clearScrollLock(); return nil })
	d.Call("addEventListener", "mouseup", clear, true)
	d.Call("addEventListener", "pointerup", clear, true)
	d.Call("addEventListener", "pointercancel", clear, true)

	// Escape explicitly cancels a stuck or in-flight drag (no pointer motion involved,
	// so this can't fight a legitimate drag).
	d.Call("addEventListener", "keydown", safeFunc("keydown", func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Get("key").String() == "Escape" && (bentoDrag.active || bentoArmed) {
			endDragFully()
		}
		return nil
	}), true)
}

// newDragEvent builds a synthetic DragEvent mirroring src's coordinates and
// dataTransfer, for re-dispatch to the stable tile.
func newDragEvent(kind string, src js.Value) js.Value {
	dt := src.Get("dataTransfer")
	if !dt.Truthy() {
		dt = js.Global().Get("DataTransfer").New()
	}
	init := js.Global().Get("Object").New()
	init.Set("bubbles", true)
	init.Set("cancelable", true)
	init.Set("clientX", src.Get("clientX"))
	init.Set("clientY", src.Get("clientY"))
	init.Set("dataTransfer", dt)
	return js.Global().Get("DragEvent").New(kind, init)
}
