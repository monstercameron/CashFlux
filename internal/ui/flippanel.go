//go:build js && wasm

package ui

import (
	"syscall/js"

	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// FlipPanelProps configures a FlipPanel overlay.
type FlipPanelProps struct {
	Title   string   // settings title (centered in the header)
	Back    uic.Node // settings body content (scrolls inside the back face)
	Width   string   // panel width, default "384px" (use "760px" for global settings)
	Height  string   // panel height, default "470px" (use "560px" for global settings)
	OnSave  func()   // invoked on Save (then the panel closes)
	OnClose func()   // invoked on Cancel/close (and after Save)
}

// FlipPanel is the candidate-C settings overlay shared by both per-widget and
// global settings: a dimmed/blurred backdrop centering a card that lifts and
// flips (3D rotateY) from a neutral front to a settings back face with a
// centered title, a close button, a scrollable body, and a dark Save/Cancel
// footer. Generic and props-driven — callers supply the title, the body, and the
// size. The open animation runs once on mount.
func FlipPanel(props FlipPanelProps) uic.Node {
	return uic.CreateElement(flipPanel, props)
}

func flipPanel(props FlipPanelProps) uic.Node {
	// shown drives both the backdrop fade-in and the card flip; it flips to true
	// once just after mount so the CSS transition animates.
	shown := uic.UseState(false)
	uic.UseEffect(func() func() {
		if !shown.Get() {
			shown.Set(true)
		}
		return nil
	}, true)

	// Modal keyboard behavior: Esc closes; Tab is trapped inside the dialog;
	// focus moves into the dialog on open and is restored to the trigger on close.
	// The listener is added on mount and removed on unmount, which (since the panel
	// mounts fresh each open and unmounts on close) matches the dialog's lifetime.
	onCloseRef := props.OnClose
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		// Remember what had focus so we can restore it when the dialog closes.
		prevFocus := doc.Get("activeElement")

		// focusables lists the dialog's tabbable elements in DOM order.
		focusables := func() []js.Value {
			wrap := doc.Call("querySelector", ".flip-wrap")
			if wrap.IsNull() || wrap.IsUndefined() {
				return nil
			}
			list := wrap.Call("querySelectorAll", "a[href], button, input, select, textarea, [tabindex]")
			out := make([]js.Value, 0, list.Get("length").Int())
			for i := 0; i < list.Get("length").Int(); i++ {
				el := list.Index(i)
				if el.Call("getAttribute", "tabindex").String() == "-1" {
					continue
				}
				if d := el.Get("disabled"); !d.IsUndefined() && d.Bool() {
					continue
				}
				out = append(out, el)
			}
			return out
		}

		// Move focus into the dialog (its first focusable) so keyboard/SR users
		// start inside the modal rather than behind it.
		if fs := focusables(); len(fs) > 0 {
			fs[0].Call("focus")
		}

		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			e := args[0]
			switch e.Get("key").String() {
			case "Escape":
				if onCloseRef != nil {
					onCloseRef()
				}
			case "Tab":
				fs := focusables()
				if len(fs) == 0 {
					return nil
				}
				first, last := fs[0], fs[len(fs)-1]
				active := doc.Get("activeElement")
				if e.Get("shiftKey").Bool() {
					if active.Equal(first) {
						e.Call("preventDefault")
						last.Call("focus")
					}
				} else if active.Equal(last) {
					e.Call("preventDefault")
					first.Call("focus")
				}
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
			if !prevFocus.IsNull() && !prevFocus.IsUndefined() {
				prevFocus.Call("focus")
			}
		}
	}, true)

	width, height := props.Width, props.Height
	if width == "" {
		width = "384px"
	}
	if height == "" {
		height = "470px"
	}

	backdropCls, innerCls := "flip-backdrop", "flip-inner"
	if shown.Get() {
		backdropCls += " show"
		innerCls += " flipped"
	}

	onClose, onSave := props.OnClose, props.OnSave

	return Div(Class(backdropCls),
		Div(Class("flip-wrap"), Style(map[string]string{"width": width, "height": height}),
			Attr("role", "dialog"), Attr("aria-modal", "true"), Attr("aria-label", props.Title),
			Div(Class(innerCls),
				// Front face — a neutral card briefly seen during the flip.
				Div(Class("flip-face"),
					Div(Class("wh"), Span(Class("grip"), "⠿"), H3(props.Title)),
				),
				// Back face — the settings panel.
				Div(Class("flip-face flip-back"),
					Div(Class("set-h"),
						Span(Style(map[string]string{"width": "1.5rem"})), // balance the close button so the title centers
						H3(props.Title),
						Button(Class("set-close"), Type("button"), Attr("title", "Close"),
							OnClick(func() {
								if onClose != nil {
									onClose()
								}
							}),
							"✕",
						),
					),
					Div(Class("set-body"), props.Back),
					Div(Class("set-foot"),
						Button(Class("set-btn cancel"), Type("button"),
							OnClick(func() {
								if onClose != nil {
									onClose()
								}
							}),
							"Cancel",
						),
						Button(Class("set-btn save"), Type("button"),
							OnClick(func() {
								if onSave != nil {
									onSave()
								}
								if onClose != nil {
									onClose()
								}
							}),
							"Save",
						),
					),
				),
			),
		),
	)
}
