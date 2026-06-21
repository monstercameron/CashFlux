//go:build js && wasm

package ui

import (
	"syscall/js"

	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// MermaidProps configures a Mermaid diagram.
type MermaidProps struct {
	Source string // Mermaid diagram source (from internal/mermaid generators)
	Class  string // extra classes on the container
	Label  string // accessible description (the diagram is role="img")
}

// Mermaid renders Mermaid source to inline SVG. Like Chart, the Go side owns a
// managed container the framework creates but doesn't draw into; an effect keyed on
// the source hands the element and the source to the cashfluxRenderMermaid shim
// (strict, no-CDN, vendored mermaid.min.js), which renders into it. Cleanup clears
// the box on unmount/redraw so no stale SVG lingers — the ref/portal pattern that
// lets the renderer mutate the DOM without fighting the vdom (C70).
func Mermaid(props MermaidProps) uic.Node { return uic.CreateElement(mermaidView, props) }

func mermaidView(props MermaidProps) uic.Node {
	id := uic.UseId()
	src := props.Source

	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		el := doc.Call("getElementById", id)
		if el.IsNull() || el.IsUndefined() {
			return nil
		}
		if fn := js.Global().Get("cashfluxRenderMermaid"); fn.Type() == js.TypeFunction {
			fn.Invoke(el, src)
		}
		return func() {
			if !el.IsNull() && !el.IsUndefined() {
				if fn := js.Global().Get("cashfluxDisposeMermaid"); fn.Type() == js.TypeFunction {
					fn.Invoke(el)
				} else {
					el.Set("innerHTML", "")
				}
			}
		}
	}, src)

	cls := "cf-mermaid"
	if props.Class != "" {
		cls += " " + props.Class
	}
	return Div(
		Attr("id", id),
		ClassStr(cls),
		Attr("role", "img"),
		Attr("aria-label", props.Label),
	)
}
