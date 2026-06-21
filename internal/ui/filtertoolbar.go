//go:build js && wasm

package ui

import (
	"strconv"

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

	trigger := Button(ClassStr("btn filters-trigger"), Type("button"),
		Attr("aria-haspopup", "dialog"), Title(props.FiltersTitle),
		OnClick(func() { open.Set(true) }),
		props.FiltersLabel,
		If(n > 0, Span(ClassStr("filter-badge"), Attr("aria-hidden", "true"), Text(strconv.Itoa(n)))),
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
		Div(ClassStr("filter-toolbar"),
			Input(ClassStr("field filter-search"), Type("search"),
				Attr("aria-label", props.SearchLabel), Placeholder(props.SearchLabel),
				Value(props.Search), OnInput(onSearch)),
			trigger,
			props.Actions,
		),
		If(n > 0, Div(ClassStr("filter-chips"), chips,
			Button(ClassStr("btn-link chip-clear-all"), Type("button"),
				OnClick(func() {
					if props.OnClearAll != nil {
						props.OnClearAll()
					}
				}),
				props.ClearAllLabel),
		)),
		If(open.Get(), FlipPanel(FlipPanelProps{
			Title:     props.FiltersTitle,
			Back:      props.FilterFields,
			Height:    "440px",
			CloseOnly: true,
			OnClose:   func() { open.Set(false) },
		})),
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
	return Span(ClassStr("filter-chip"),
		Span(ClassStr("chip-text"), props.Label),
		Button(ClassStr("chip-x"), Type("button"), Attr("aria-label", props.RemoveLabel),
			OnClick(func() {
				if props.OnRemove != nil {
					props.OnRemove(props.Key)
				}
			}),
			"✕"),
	)
}
