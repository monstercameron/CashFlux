//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// WidgetBuilder is the (placeholder) widget-creation screen: a future surface for
// composing a dashboard widget from a data source, transform, and visualization.
// Blank for now — routing + rail entry only.
func WidgetBuilder() ui.Node {
	return Section(Class("card"),
		H3(Class("card-title"), uistate.T("widgetBuilder.title")),
		P(Class("empty"), uistate.T("widgetBuilder.empty")),
	)
}

// WidgetManager is the (placeholder) widget-management screen: a future surface
// for browsing, editing, enabling, and deleting saved widgets. Blank for now —
// routing + rail entry only.
func WidgetManager() ui.Node {
	return Section(Class("card"),
		H3(Class("card-title"), uistate.T("widgetManager.title")),
		P(Class("empty"), uistate.T("widgetManager.empty")),
	)
}
