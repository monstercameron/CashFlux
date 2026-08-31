// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/navsearch"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/screens"
	ui "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// railsearch.go is the rail's destination filter: the box itself, and the walk
// that collects everything the box is allowed to find.
//
// The rail holds around thirty destinations across four groups, and the tools,
// system and my-pages sections all default to COLLAPSED — so reaching one meant
// remembering which section it lived in and opening that section first. Typing is
// faster than remembering.
//
// While a query is present the rail renders a FLAT ranked list instead of its
// sections. That is deliberate: section headers, drag handles, the Alt+N digits
// and the sliding active indicator all describe the menu's structure, and a
// filtered result is not that structure — the indicator in particular would point
// at whatever position the active item used to occupy, which during a search is
// usually an item that is no longer on screen.

// railSearchable is the flat, localized list of everything the filter can find,
// in menu order. Ranking is navsearch's job; this decides only what is in scope.
//
// Hidden modules are excluded, because a destination the household has switched
// off should not come back through a side door. Custom pages ARE included, and
// read here rather than inside CustomPagesNav: they carry names the household
// chose, which makes them the most likely thing to be searched for and the least
// likely to be remembered by section.
func railSearchable(hiddenPrimary, hiddenTools, hiddenSystem []railItem) []navsearch.Item {
	primary := uistate.T("nav.primaryLabel")
	items := make([]navsearch.Item, 0, 32)
	for _, it := range hiddenPrimary {
		items = append(items, navsearch.Item{Label: uistate.T(it.Key), Path: it.Path, Section: primary})
	}
	// Tools keep the registry's display order so a section-name query returns its
	// contents in the order the rail would have shown them.
	bySub := map[string][]railItem{}
	for _, it := range hiddenTools {
		bySub[it.SubGroup] = append(bySub[it.SubGroup], it)
	}
	for _, sg := range screens.ToolsSubGroups {
		for _, it := range bySub[sg] {
			items = append(items, navsearch.Item{
				Label: uistate.T(it.Key), Path: it.Path, Section: toolSubGroupLabel(sg),
			})
		}
	}
	system := uistate.T("rail.system")
	for _, it := range hiddenSystem {
		items = append(items, navsearch.Item{Label: uistate.T(it.Key), Path: it.Path, Section: system})
	}
	if app := appstate.Default; app != nil {
		myPages := uistate.T("rail.myPages")
		for _, p := range pages.Ordered(pages.Visible(app.CustomPages())) {
			items = append(items, navsearch.Item{
				Label: p.Name, Path: uistate.RoutePath("/p/" + p.Slug), Section: myPages,
			})
		}
	}
	return items
}

// railIconFor returns the rail icon for a route. Custom pages have no railMeta
// entry — their icon is the same generic page mark the My-pages section uses —
// and any route added without design metadata falls back the same way rather than
// rendering iconless and half a row narrower than its neighbours.
func railIconFor(path string) icon.Name {
	if m, ok := railMeta[path]; ok {
		return m.Icon
	}
	return icon.FileText
}

type railSearchBoxProps struct {
	Query   string
	Matches int
	// Handlers, not funcs: uic.UseEvent returns a Handler that re-points its
	// closure each render, which is what keeps the captured match list fresh.
	OnInput uic.Handler
	OnKey   uic.Handler
	OnClear uic.Handler
}

// railSearchBox is its own component so its handler hooks sit at a stable position
// regardless of how the rail's contents change around it.
func railSearchBox(props railSearchBoxProps) uic.Node {
	searching := strings.TrimSpace(props.Query) != ""
	var clear uic.Node = Fragment()
	if searching {
		clear = Button(css.Class("railsearch-clear"), Type("button"),
			Attr("data-testid", "railsearch-clear"),
			Attr("aria-label", uistate.T("railsearch.clear")),
			OnClick(props.OnClear),
			ui.Icon(icon.Close, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
		)
	}
	// The count is a live region rather than visible text: sighted users can see
	// the list resize, and a running number beside the box would be noise. A screen
	// reader gets nothing from a list that silently changes length, so it is
	// announced there and only there.
	var count uic.Node = Fragment()
	if searching {
		count = Div(css.Class("sr-only"), Attr("role", "status"), Attr("aria-live", "polite"),
			Attr("data-testid", "railsearch-count"),
			uistate.TN("railsearch.countOne", "railsearch.countMany", props.Matches))
	}
	return Div(css.Class("railsearch"),
		ui.Icon(icon.Search, css.Class("railsearch-icon", tw.ShrinkO, tw.W4, tw.H4)),
		Input(css.Class("field railsearch-input"), Type("text"),
			Attr("data-testid", "railsearch-input"),
			Attr("aria-label", uistate.T("railsearch.label")),
			// A filter is not a combobox: it rewrites a list of links in place
			// rather than proposing values for the field, so it carries no
			// aria-autocomplete or aria-activedescendant. autocomplete/spellcheck
			// are off because destination names are not prose.
			Attr("autocomplete", "off"), Attr("spellcheck", "false"),
			Placeholder(uistate.T("railsearch.placeholder")),
			OnInput(props.OnInput), OnKeyDown(props.OnKey),
			ui.FieldValue(props.Query),
		),
		clear,
		count,
	)
}
