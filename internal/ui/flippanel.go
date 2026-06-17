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

	// Esc closes the dialog (a standard modal affordance). The listener is added
	// on mount and removed on unmount — which, since the panel mounts fresh each
	// open and unmounts on close, matches the dialog's lifetime exactly.
	onCloseRef := props.OnClose
	uic.UseEffect(func() func() {
		if onCloseRef == nil {
			return nil
		}
		doc := js.Global().Get("document")
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Get("key").String() == "Escape" {
				onCloseRef()
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
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
