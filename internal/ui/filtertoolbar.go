// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// Chip is one removable active-filter chip in a FilterToolbar. Key is its stable
// identity (passed back to OnRemoveChip and used as the list key); Label is the
// human text shown on the chip.
type Chip struct {
	Key   string
	Label string
}

// FilterToolbarProps configures a FilterToolbar. The component is screen-agnostic:
// callers supply the search wiring, the popover field controls, the active-filter
// chips, and the handlers. Filters are expected to apply live (the popover is
// close-only), so there is no Save.
type FilterToolbarProps struct {
	Search      string       // current search text
	SearchLabel string       // aria-label + placeholder for the search box
	OnSearch    func(string) // search input handler

	FiltersLabel string   // trigger button text, e.g. "Filters"
	FiltersTitle string   // popover title + trigger tooltip
	FilterFields uic.Node // popover body: the labelled field controls
	// ActiveAriaLabel builds the trigger's accessible name from the active-filter
	// count (C57): the visible badge is aria-hidden, so without this a screen reader
	// hears only "Filters" and never the count. Caller supplies a translated string
	// (e.g. "Filters — 3 active"). Falls back to FiltersLabel when nil.
	ActiveAriaLabel func(n int) string

	Chips         []Chip           // active-filter chips (empty hides the row + badge)
	OnRemoveChip  func(key string) // a chip's ✕
	OnClearAll    func()           // the "clear all" link
	ClearAllLabel string           // text for the clear-all link
	RemoveLabel   string           // aria-label for a chip's ✕

	Actions []uic.Node // trailing toolbar buttons (e.g. Clear, Export CSV)
}

// FilterToolbar is a portable, reusable compact filter UI: an always-visible
// search box, a "Filters" popover trigger badged with the active-filter count,
// caller-supplied trailing actions, and a row of removable chips below. The
// popover (a FlipPanel) and its open/close state are owned internally, so callers
// only hold their own filter state. Mirrors the DataTable widget's role for the
// ledger table — both live here so any screen can reuse them.
func FilterToolbar(props FilterToolbarProps) uic.Node {
	return uic.CreateElement(filterToolbar, props)
}

func filterToolbar(props FilterToolbarProps) uic.Node {
	open := uic.UseState(false)
	onSearch := uic.UseEvent(props.OnSearch)
	n := len(props.Chips)

	// C56: press "f" to open the filter panel (ignored while typing in a field or
	// with a modifier held). Document listener added on mount, removed on unmount.
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		if !doc.Truthy() {
			return nil
		}
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			e := args[0]
			if e.Get("key").String() != "f" || e.Get("metaKey").Bool() || e.Get("ctrlKey").Bool() || e.Get("altKey").Bool() {
				return nil
			}
			if ae := doc.Get("activeElement"); ae.Truthy() {
				switch ae.Get("tagName").String() {
				case "INPUT", "TEXTAREA", "SELECT":
					return nil
				}
				if ae.Get("isContentEditable").Bool() {
					return nil
				}
			}
			e.Call("preventDefault")
			open.Set(true)
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
		}
	}, "filter-shortcut")

	// C57: accessible name conveys the active count (the badge is aria-hidden).
	ariaLabel := props.FiltersLabel
	if props.ActiveAriaLabel != nil {
		ariaLabel = props.ActiveAriaLabel(n)
	}
	trigger := Button(css.Class("btn filters-trigger"), Type("button"),
		Attr("aria-haspopup", "dialog"), Title(props.FiltersTitle+" (f)"), Attr("aria-label", ariaLabel),
		OnClick(func() { open.Set(true) }),
		props.FiltersLabel,
		If(n > 0, Span(css.Class("filter-badge"), Attr("aria-hidden", "true"), Text(strconv.Itoa(n)))),
	)

	chips := MapKeyed(props.Chips,
		func(c Chip) any { return c.Key },
		func(c Chip) uic.Node {
			return uic.CreateElement(filterChip, filterChipProps{
				Label: c.Label, Key: c.Key, RemoveLabel: props.RemoveLabel, OnRemove: props.OnRemoveChip,
			})
		},
	)

	return Div(
		Div(css.Class("filter-toolbar"),
			Input(css.Class("field filter-search"), Type("search"),
				Attr("aria-label", props.SearchLabel), Placeholder(props.SearchLabel),
				Value(props.Search), OnInput(onSearch)),
			trigger,
			props.Actions,
		),
		If(n > 0, Div(css.Class("filter-chips"), chips,
			Button(css.Class("btn-link chip-clear-all"), Type("button"),
				OnClick(func() {
					if props.OnClearAll != nil {
						props.OnClearAll()
					}
				}),
				props.ClearAllLabel),
		)),
		// C52: inline collapsible filter panel — no backdrop, no occlusion. The table
		// remains visible below while the user adjusts filters. The panel mounts/unmounts
		// with the open state (same as before for the FlipPanel) so it is keyed correctly.
		If(open.Get(), Div(css.Class("filter-inline-panel"),
			Attr("role", "region"), Attr("aria-label", props.FiltersTitle),
			Div(css.Class("filter-inline-header"),
				H3(css.Class("filter-inline-title"), props.FiltersTitle),
				Button(css.Class("set-close"), Type("button"), Attr("title", uistate.T("action.close")),
					OnClick(func() { open.Set(false) }), "✕"),
			),
			Div(css.Class("filter-inline-body"), props.FilterFields),
		)),
	)
}

// filterChipProps configures one chip. It is its own component so the remove
// button's OnClick hook sits at a stable render position (the chip list is
// variable-length — see the framework loop-hook gotcha).
type filterChipProps struct {
	Label       string
	Key         string
	RemoveLabel string
	OnRemove    func(string)
}

func filterChip(props filterChipProps) uic.Node {
	return Span(css.Class("filter-chip"),
		Span(css.Class("chip-text"), props.Label),
		Button(css.Class("chip-x"), Type("button"), Attr("aria-label", props.RemoveLabel),
			OnClick(func() {
				if props.OnRemove != nil {
					props.OnRemove(props.Key)
				}
			}),
			"✕"),
	)
}
