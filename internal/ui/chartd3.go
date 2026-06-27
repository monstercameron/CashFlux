// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"encoding/json"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/chartspec"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ChartProps configures a Chart.
type ChartProps struct {
	Spec   chartspec.Spec
	Height string // CSS height of the chart box, default "160px"
	Class  string // extra classes on the container
	Label  string // accessible description (the chart is role="img")
	// CurrencySymbol is the base-currency symbol used to render any axis whose
	// chartspec format is "money" (e.g. "$"/"€"/"£"). Empty defaults to "$".
	CurrencySymbol string
}

// Chart renders a chartspec.Spec with D3. The Go side owns a managed container
// the framework creates but doesn't draw into; an effect keyed on the serialized
// spec hands the element and the spec JSON to the cashfluxRenderChart shim, which
// draws into it and is theme-aware. The shim clears and redraws on each call, and
// the effect clears the box on unmount so no stale SVG lingers — the ref/portal
// pattern for letting D3 mutate the DOM without fighting the vdom.
func Chart(props ChartProps) uic.Node { return uic.CreateElement(chartD3, props) }

func chartD3(props ChartProps) uic.Node {
	id := uic.UseId()
	h := props.Height
	if h == "" {
		h = "160px"
	}

	specJSON := ""
	if b, err := json.Marshal(props.Spec); err == nil {
		specJSON = string(b)
	}

	// Redraw whenever the serialized spec changes (and once on mount). getElementById
	// resolves the managed container by its stable UseId; cashfluxRenderChart parses
	// the JSON and draws. Cleanup clears the box on unmount or before a redraw.
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if doc.IsNull() || doc.IsUndefined() {
			return nil
		}
		el := doc.Call("getElementById", id)
		if el.IsNull() || el.IsUndefined() {
			return nil
		}
		if fn := js.Global().Get("cashfluxRenderChart"); fn.Type() == js.TypeFunction {
			fn.Invoke(el, specJSON, props.CurrencySymbol)
		}
		return func() {
			if !el.IsNull() && !el.IsUndefined() {
				if fn := js.Global().Get("cashfluxDisposeChart"); fn.Type() == js.TypeFunction {
					fn.Invoke(el)
				} else {
					el.Set("innerHTML", "")
				}
			}
		}
	}, specJSON+"|"+props.CurrencySymbol)

	cls := "cf-chart"
	if props.Class != "" {
		cls += " " + props.Class
	}
	// C315: role="img" must have a non-empty accessible name. Fall back to a generic
	// "Chart" when no Label is supplied so the element is never an anonymous image.
	label := props.Label
	if label == "" {
		label = "Chart"
	}
	return Div(
		Attr("id", id),
		ClassStr(cls),
		Attr("role", "img"),
		Attr("aria-label", label),
		Style(map[string]string{"width": "100%", "height": h}),
	)
}
