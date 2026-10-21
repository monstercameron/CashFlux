// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/widgetrender"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// surfaceTilesProps configures the bento tile list shared by widgetized surfaces.
type surfaceTilesProps struct {
	// Class is the page-specific class appended to the common bento class.
	Class string
	// Specs are rendered in order and keyed by stable, unique, nonempty placement IDs.
	Specs []domain.WidgetSpec
	// Context contains the surface data shared by all tile renderers. The current
	// placement's Spec is copied into Context for each render.
	Context widgetrender.RenderCtx
	// Attrs are additional attributes for the bento root.
	Attrs []any
}

// surfaceTiles renders a keyed bento and isolates a failed or unavailable tile
// from its siblings through safeRenderSpec.
func surfaceTiles(props surfaceTilesProps) ui.Node {
	args := []any{css.Class("bento " + props.Class)}
	args = append(args, props.Attrs...)
	args = append(args, MapKeyed(props.Specs,
		func(spec domain.WidgetSpec) any { return spec.ID },
		func(spec domain.WidgetSpec) ui.Node {
			ctx := props.Context
			ctx.Spec = spec
			if node, ok := safeRenderSpec(spec, ctx); ok {
				return node
			}
			return Fragment()
		},
	))
	return Div(args...)
}
