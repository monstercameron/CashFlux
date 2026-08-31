// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgets_goalglyph.go is the compact row's goal-funded marker.
//
// The full pill reads "part funded by a goal" and is 148px wide. In a narrow name
// cell it either pushed the budget's own name out of existence or ellipsized to a
// meaningless stub ("part funded by a go", and at the worst width simply "p"). A
// marker that cannot show its own words is better as a SYMBOL: below the row's
// two-column threshold this glyph replaces the pill, and the words come back on
// demand rather than being squeezed into a space that cannot hold them.
//
// The delay before the popover is deliberate. A marker that opens instantly on
// pointer-over fires every time the cursor crosses the column on its way to the
// kebab, so ten rows of glyphs flicker as you travel down the list. The pause is
// hover INTENT — and because a pause with no feedback reads as the app ignoring
// you, the glyph shows it is counting: a small spinner runs for the wait, so the
// popover arrives as the end of something you watched start.

// goalGlyphHoverMs is how long the pointer must rest before the popover opens.
// Long enough to filter a cursor passing through, short enough that a deliberate
// hover does not feel broken.
const goalGlyphHoverMs = 380

// goalGlyphGraceMs is how often, after the pointer leaves, the marker re-checks
// whether the pointer has landed on the popover itself before closing.
//
// WCAG 2.2 SC 1.4.13 (Content on Hover or Focus) requires this content be
// HOVERABLE: the pointer must be able to travel from the trigger onto the popover
// without it vanishing. It could not — mouseleave closed it on the way, so anyone
// using magnification, or simply reading the rollover arithmetic slowly, could
// never reach the text. Dismissible (Escape, which also returns focus to the
// trigger) and Persistent (nothing auto-hides it) already held; this is the third.
const goalGlyphGraceMs = 140

type budgetGoalGlyphProps struct {
	// BudgetID scopes the wrapper's DOM id and test ids to one row, and Kind
	// distinguishes the markers on the SAME row — a budget can be goal-funded and
	// rolling over at once, and two wrappers sharing a DOM id would give the
	// portal two anchors to choose between.
	BudgetID, Kind string
	// Icon is the symbol the words collapse into.
	Icon icon.Name
	// Title is the marker's short name; Text is the sentence the popover shows.
	Title, Text string
	// PillText is the worded shape's label, PillClass its tone hook. An EMPTY
	// PillText means the marker has no worded shape at all: some sentences never
	// fit a name cell at any width, and pretending otherwise only moves the
	// overflow around (see the goal marker's call site).
	PillText, PillClass string
}

// markerHover is the row head's ONE hover behaviour, shared by every marker.
//
// Before this there were three systems on a single row: the note used a native
// `title`, the rollover pill opened its explainer on CLICK only, and the glyph
// had hover-intent AND a native title at the same time — so hovering the first
// two icons on a row behaved differently from hovering the third, and the third
// raced its own browser tooltip (Cam, 2026-08-31). The note's own code had
// already recorded why native titles lose: they never open on keyboard focus and
// do not exist on touch, so their text is mouse-only.
//
// One rule now: hover (or focus) PREVIEWS through the shared popover after a
// short intent delay, and click does the marker's real work — which for a plain
// explainer is toggling that same popover, and for the note is opening its editor.
type markerHover struct {
	Open, Pending          bool
	Enter, Leave, Activate ui.Handler
	Class                  string
}

// useMarkerHover wires the delay, the portal and the dismiss for one marker.
// onActivate is the click action; nil means "click toggles the explainer".
func useMarkerHover(wrapID, title, text string, onActivate func()) markerHover {
	open := ui.UseState(false)
	pending := ui.UseState(false)
	closing := ui.UseState(false)

	uiw.DismissPopover(open.Get(), wrapID, func() { open.Set(false) })
	uiw.SmartTipPortal(open.Get(), wrapID, title, text)

	// The wait runs as an effect keyed on `pending`, so leaving tears the timer down
	// through the cleanup rather than racing it: a pointer crossing the column on
	// its way to the kebab cancels before anything opens.
	ui.UseEffect(func() func() {
		if !pending.Get() {
			return nil
		}
		cancelled := false
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			if cancelled {
				return nil
			}
			pending.Set(false)
			open.Set(true)
			return nil
		})
		handle := js.Global().Call("setTimeout", cb, goalGlyphHoverMs)
		return func() {
			cancelled = true
			js.Global().Call("clearTimeout", handle)
			cb.Release()
		}
	}, pending.Get())

	// The grace window. It POLLS :hover rather than listening on the popover,
	// because the popover is portalled to <body> by SmartTipPortal and is created
	// and destroyed outside this component — there is no stable node here to bind
	// to, and a listener attached at open time would be attached to a node a later
	// re-render replaced. Asking the document who is hovered right now cannot go
	// stale that way.
	ui.UseEffect(func() func() {
		if !closing.Get() {
			return nil
		}
		doc := js.Global().Get("document")
		stopped := false
		hovered := func(el js.Value) bool {
			return el.Truthy() && el.Call("matches", ":hover").Bool()
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			if stopped {
				return nil
			}
			if hovered(doc.Call("getElementById", wrapID)) ||
				hovered(doc.Call("querySelector", "[data-testid=smart-tip-pop]")) {
				return nil // the pointer is still on the marker or its popover
			}
			closing.Set(false)
			open.Set(false)
			return nil
		})
		handle := js.Global().Call("setInterval", cb, goalGlyphGraceMs)
		return func() {
			stopped = true
			js.Global().Call("clearInterval", handle)
			cb.Release()
		}
	}, closing.Get())

	// Keyboard preview. The framework's OnFocus/OnBlur props are registered but do
	// not fire — MEASURED on a freshly mounted marker: focus lands on the button,
	// `pending` stays false and nothing opens (2026-08-31). So the pair is bound
	// natively instead.
	//
	// It binds on the DOCUMENT and resolves the wrapper at EVENT time, not on the
	// wrapper at mount time. The first attempt did the latter and silently did
	// nothing: on first render the effect runs before the wrapper carries its id,
	// getElementById returns null, and because wrapID never changes the effect had
	// no reason to run again — so the listener was never attached and focus behaved
	// exactly as it had before the fix. Resolving late has no such window.
	//
	// focusin/focusout are used rather than focus/blur because they BUBBLE, and
	// because scoping to the wrapper is what stops a focus move between the pill
	// and the glyph from reading as a blur: relatedTarget is still inside, so the
	// popover does not flicker shut within one marker.
	ui.UseEffect(func() func() {
		doc := js.Global().Get("document")
		inside := func(v js.Value) bool {
			if !v.Truthy() {
				return false
			}
			el := doc.Call("getElementById", wrapID)
			return el.Truthy() && el.Call("contains", v).Bool()
		}
		onIn := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 || !inside(args[0].Get("target")) {
				return nil
			}
			if !open.Get() {
				pending.Set(true)
			}
			return nil
		})
		onOut := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 || !inside(args[0].Get("target")) {
				return nil
			}
			if inside(args[0].Get("relatedTarget")) {
				return nil
			}
			pending.Set(false)
			open.Set(false)
			return nil
		})
		doc.Call("addEventListener", "focusin", onIn)
		doc.Call("addEventListener", "focusout", onOut)
		return func() {
			doc.Call("removeEventListener", "focusin", onIn)
			doc.Call("removeEventListener", "focusout", onOut)
			onIn.Release()
			onOut.Release()
		}
	}, wrapID)

	h := markerHover{Open: open.Get(), Pending: pending.Get()}
	h.Enter = ui.UseEvent(func() {
		closing.Set(false)
		if !open.Get() {
			pending.Set(true)
		}
	})
	// Leaving ARMS a close rather than performing one, so the pointer has time to
	// cross the gap onto the popover.
	h.Leave = ui.UseEvent(func() {
		pending.Set(false)
		if open.Get() {
			closing.Set(true)
		}
	})
	// Click is the touch and keyboard path — neither has a hover — so a marker must
	// never be reachable by pointer alone.
	h.Activate = ui.UseEvent(Prevent(func() {
		pending.Set(false)
		closing.Set(false)
		if onActivate != nil {
			// The explainer would sit over whatever the action opens.
			open.Set(false)
			onActivate()
			return
		}
		open.Set(!open.Get())
	}))
	if h.Pending {
		h.Class = " is-waiting"
	}
	if h.Open {
		h.Class += " is-open"
	}
	return h
}

// budgetGoalGlyph renders ONE marker in both of its shapes — the worded pill and
// the glyph — driven by a SINGLE hover system, with CSS choosing which shape is
// visible at the current width.
//
// They were two components: a pill rendered by the row and a glyph rendered
// beside it, and for rollover the badge wrapped the glyph while ALSO running its
// own hover. That is two popover systems for one piece of information — they can
// open together, they position against different anchors, and they have to be
// kept in agreement by hand (Cam, 2026-08-31). One component, one open state, one
// portal: the shapes cannot disagree because there is only one of everything.
func budgetGoalGlyph(props budgetGoalGlyphProps) ui.Node {
	wrapID := "marker-" + props.Kind + "-" + props.BudgetID
	h := useMarkerHover(wrapID, props.Title, props.Text, nil)

	// Pointer preview + click. Focus is NOT wired here: OnFocus/OnBlur do not fire
	// in this framework, so the keyboard preview is bound natively on the wrapper
	// by useMarkerHover. Keeping dead props here would have said otherwise.
	hover := []any{
		OnMouseEnter(h.Enter), OnMouseLeave(h.Leave), OnClick(h.Activate),
		Attr("aria-expanded", ariaBool(h.Open)),
		Attr("aria-describedby", wrapID+"-desc"),
	}
	// The pill carries its own words, so its accessible name is its text. The glyph
	// has none, so it borrows the sentence — nothing is lost by dropping to a symbol.
	// Neither takes a `title`: a native tooltip racing the popover on the same
	// element was the duplication this alignment removed.
	var pill ui.Node = Fragment()
	if props.PillText != "" {
		pillArgs := append([]any{
			ClassStr("budget-marker-pill " + props.PillClass + h.Class), Type("button"),
			Attr("data-testid", "budget-marker-pill-"+props.Kind+"-"+props.BudgetID),
		}, hover...)
		pill = Button(append(pillArgs, Span(props.PillText))...)
	}

	glyphArgs := append([]any{
		ClassStr("budget-goalglyph" + h.Class), Type("button"),
		Attr("data-testid", "budget-glyph-"+props.Kind+"-"+props.BudgetID),
		// The NAME is the marker's short title, not the sentence: the sentence is on
		// the described-by node above, and a label that repeats it makes a screen
		// reader read the whole explanation twice before saying what the control is.
		Attr("aria-label", props.Title),
	}, hover...)
	glyph := Button(append(glyphArgs,
		uiw.Icon(props.Icon, css.Class("budget-goalglyph-icon", tw.ShrinkO, tw.W4, tw.H4)),
		// The spinner is a sibling rather than a swap so the icon never jumps:
		// it rides on top for the wait and is removed when the popover opens.
		If(h.Pending, Span(css.Class("budget-goalglyph-spin"), Attr("aria-hidden", "true"),
			Attr("data-testid", "budget-glyph-spin-"+props.Kind+"-"+props.BudgetID))),
	)...)

	// A marker with no worded shape has nothing to fall back to, so its glyph must
	// show at EVERY width — otherwise the row simply stops saying the budget is
	// goal-funded once the viewport is wide enough for a pill that does not exist.
	wrapCls := "budget-marker-wrap add-wrap "
	if props.PillText == "" {
		wrapCls += "is-glyphonly "
	}
	// The pill's accessible name is its own short label ("Rolls over"), so without
	// this the explanatory sentence existed ONLY inside the popover — reachable by
	// pointer and by Enter, but never announced when the control is simply read.
	// A described-by span states it unconditionally; the glyph already carries the
	// sentence as its label, and pointing both at one node keeps them from drifting.
	descID := wrapID + "-desc"
	return Span(ClassStr(wrapCls+tw.Fold(tw.InlineFlex, tw.ItemsCenter)),
		Attr("id", wrapID),
		Span(css.Class("sr-only"), Attr("id", descID), props.Text),
		pill, glyph)
}

type budgetNotesMarkerProps struct {
	BudgetID, Label, Text, RowTitle string
	OnOpen                          func()
}

// budgetNotesMarker is the note indicator, on the SAME hover system as every other
// marker: the note's words preview in the shared popover, and a click still opens
// the editor. It keeps its own text span, which the head hides once the row drops
// to symbols.
func budgetNotesMarker(props budgetNotesMarkerProps) ui.Node {
	wrapID := "markernotes-" + props.BudgetID
	h := useMarkerHover(wrapID, props.Label, props.Text, props.OnOpen)

	return Span(ClassStr("budget-crow-notes-wrap add-wrap "+tw.Fold(tw.InlineFlex, tw.ItemsCenter)),
		Attr("id", wrapID),
		Button(ClassStr("budget-crow-notes"+h.Class), Type("button"),
			Attr("data-testid", "budget-notes-"+props.BudgetID),
			Attr("aria-label", props.Label+" — "+props.RowTitle),
			OnMouseEnter(h.Enter), OnMouseLeave(h.Leave), OnClick(h.Activate),
			uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(css.Class("budget-crow-notes-text"), props.Text),
			If(h.Pending, Span(css.Class("budget-goalglyph-spin"), Attr("aria-hidden", "true"),
				Attr("data-testid", "budget-glyph-spin-notes-"+props.BudgetID))),
		),
	)
}

// goalGlyphTitle is the marker's short name, used as the popover's heading.
func goalGlyphTitle() string { return uistate.T("budgets.goalFundedGlyphTitle") }
