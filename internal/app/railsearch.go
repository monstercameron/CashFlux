// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/favorites"
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
		// No generic "field" class: it carries the app's standard 44px control
		// height, which is right for a form and wrong for a filter sitting under a
		// 46px identity row — the two together made the top of the rail as tall as
		// the pinned list. This input is styled completely by .railsearch-input.
		Input(css.Class("railsearch-input"), Type("text"),
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

// ── the shortcut's view of the pinned list ───────────────────────────────────
//
// The keydown listener is registered once at boot, outside any component, so it
// cannot read the favorites atom through a hook. The Sidebar publishes the live
// list here instead and the listener reads it — one writer, one reader, no
// subscription machinery for a slice of at most ten strings.
//
// It is deliberately NOT read back into the UI: the atom stays the source of
// truth for rendering, and this is a projection of it. Two-way would give the
// same list two owners.
var pinnedForShortcuts []string

// pinnedMover reorders the pinned list. The Sidebar publishes it alongside the
// list itself, because it is the only place holding the list the UI actually
// RENDERS — favPaths, after favorites.Clean has dropped anything unreachable.
//
// The first attempt had the global handler compute indices from the published
// list and then hand them to a uistate helper that re-read the atom. The atom
// holds the raw stored order; the rail renders the cleaned one. When those two
// disagree — one dead pin is enough — the indices address different rows and the
// move lands somewhere nobody asked for. One list, one owner.
var pinnedMover func(from, to int) bool

func publishPinnedMover(f func(from, to int) bool) { pinnedMover = f }

func publishFavorites(list []string) {
	next := make([]string, len(list))
	copy(next, list)
	pinnedForShortcuts = next
}

// pinnedPathForDigit returns the route a pressed digit should open, or "" when
// that slot is empty.
func pinnedPathForDigit(d byte) string {
	i, ok := favorites.SlotForDigit(d)
	if !ok || i >= len(pinnedForShortcuts) {
		return ""
	}
	return pinnedForShortcuts[i]
}

// focusMenuFilter puts the cursor in the rail's filter box (Alt+M), expanding the
// rail first when it is collapsed to icons and there is no field to focus.
//
// The re-query after expanding is deliberate: the field does not exist in the DOM
// while the rail is collapsed, so it cannot be captured beforehand, and the
// expansion has to reach the DOM before anything can be focused. A rAF is enough
// — the atom flip re-renders synchronously and paint follows on the next frame.
func focusMenuFilter() {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	focus := func() bool {
		el := doc.Call("querySelector", "[data-testid=railsearch-input]")
		if !el.Truthy() {
			return false
		}
		el.Call("focus")
		el.Call("select")
		return true
	}
	if focus() {
		return
	}
	uistate.ToggleRailCollapsed()
	var cb js.Func
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		defer cb.Release()
		focus()
		return nil
	})
	js.Global().Call("requestAnimationFrame", cb)
}

// moveFocusedPin moves the pinned row that currently has focus by delta slots and
// returns whether anything moved.
//
// It reads the row's position from the LIST rather than from a stored index: the
// pinned order changes under the user's hands while they hold Alt and tap an
// arrow, and an index captured when the key was first pressed would walk the
// wrong row on the second press.
//
// Focus is restored afterwards because the row is re-rendered by the move; without
// it the second Alt+Arrow would have nothing focused to act on, and a keyboard
// reorder would be a one-press feature.
func moveFocusedPin(doc js.Value, delta int) bool {
	active := doc.Get("activeElement")
	if !active.Truthy() {
		return false
	}
	row := active.Call("closest", "[data-testid=rail-pinned] .nav-row a")
	if !row.Truthy() {
		row = active.Call("closest", "[data-testid=rail-pinned] .nav-row")
		if !row.Truthy() {
			return false
		}
		row = row.Call("querySelector", "a")
		if !row.Truthy() {
			return false
		}
	}
	// Call("getAttribute", …), not Get("getAttribute").Invoke(…): the latter pulls
	// the function off the node and calls it unbound, so it never sees the element.
	attr := row.Call("getAttribute", "data-path")
	if !attr.Truthy() {
		return false
	}
	path := attr.String()
	from := favorites.IndexOf(pinnedForShortcuts, path)
	if from < 0 {
		return false
	}
	to := from + delta
	if to < 0 || to >= len(pinnedForShortcuts) {
		return false
	}
	if pinnedMover == nil || !pinnedMover(from, to) {
		return false
	}
	// Put focus back on the row after the move, so a RUN of Alt+Arrow presses keeps
	// moving the same destination. Without this the first press works and every
	// press after it does nothing, because the re-render replaced the focused node
	// and focus fell back to the document — which is a one-press reorder, i.e. not
	// a usable one.
	//
	// It retries across frames rather than firing once: a single rAF lands before
	// the Go re-render has committed, and the row it is looking for does not exist
	// yet (measured — focus ended up on "Skip to content").
	refocusPinned(doc, path, 20)
	return true
}

// refocusPinned looks for a pinned row by route and focuses it, retrying for up
// to `tries` animation frames while the rail re-renders.
func refocusPinned(doc js.Value, path string, tries int) {
	sel := "[data-testid=rail-pinned] .nav-row a[data-path=\"" + path + "\"]"
	var cb js.Func
	left := tries
	cb = js.FuncOf(func(js.Value, []js.Value) any {
		// Re-assert focus every frame for the whole window rather than stopping at
		// the first success. Stopping was not enough: the row gets focused, a later
		// commit of the same re-render replaces the node, and focus falls back to
		// the document — so the FIRST Alt+Arrow worked and every one after it did
		// nothing, which is the freeze (Cam 2026-08-31). Focusing something that is
		// already focused is a no-op, so repeating is free.
		if el := doc.Call("querySelector", sel); el.Truthy() {
			if !doc.Get("activeElement").Equal(el) {
				el.Call("focus")
			}
		}
		left--
		if left <= 0 {
			cb.Release()
			return nil
		}
		js.Global().Call("requestAnimationFrame", cb)
		return nil
	})
	js.Global().Call("requestAnimationFrame", cb)
}
