//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// CustomPage renders a user-authored page resolved by its slug. The page's title
// is shown by the shell's top bar; this body renders the page's bento grid of
// custom widgets. In Phase A the grid is empty — a page can be created, named,
// ordered, and navigated to, with an empty-state prompt — and widget rendering
// lands in Phase B. A slug with no matching page (e.g. one that was deleted)
// shows a friendly not-found message rather than a blank screen.
func CustomPage(slug string) ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}
	page, ok := pages.BySlug(app.CustomPages(), slug)
	if !ok {
		return Section(Class("card"), P(Class("empty"), uistate.T("pages.notFound")))
	}
	if len(page.Widgets) == 0 {
		return Section(Class("card"), P(Class("empty"), uistate.T("pages.empty")))
	}
	// Phase B will render page.Layout + page.Widgets through the widget registry.
	return Section(Class("card"), P(Class("empty"), uistate.T("pages.empty")))
}
