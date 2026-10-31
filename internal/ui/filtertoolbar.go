// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/debounce"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// Chip is one removable active-filter chip in a FilterToolbar. Key is its stable
// identity (passed back to OnRemoveChip and used as the list key); Label is the
// human text shown on the chip.
type Chip struct {
	Key   string
	Label string
	// Class is an optional extra class on the chip. The transactions ledger uses it for its lens
	// chips (the member perspective and a budget drill-down). Their ✕ clears the top bar, not the
	// page filters, so the chip must look different or the reader cannot predict what each ✕ removes.
	Class string
	// RemoveLabel overrides the toolbar's generic "Remove this filter: …" wording
	// for this chip's ✕. A chip that is not a filter must not describe itself as
	// one, or the control names the wrong thing at the moment of the click.
	RemoveLabel string
}

// FilterToolbarProps configures a FilterToolbar. The component is screen-agnostic:
// callers supply the search wiring, the panel's field controls, the active-filter
// chips, and the handlers. Filters apply live and the panel only closes, so there is
// no Save.
type FilterToolbarProps struct {
	Search      string       // current search text
	SearchLabel string       // aria-label + placeholder for the search box
	OnSearch    func(string) // search input handler
	// SearchPending is true while typed text has not been applied yet. The search is debounced, so for
	// a few hundred milliseconds the list underneath still shows the previous query's results. While
	// this is true the control shows a searching note and marks itself busy for assistive technology.
	SearchPending bool
	// ResultsSelector names the element holding the rows this search filters, such as the ledger's
	// table. While a typed query has not been applied, those rows answer a superseded query. The
	// toolbar marks them stale on the keystroke, rather than waiting for a render that arrives too
	// late to matter. An empty selector turns this off.
	ResultsSelector string
	// OnClearSearch clears the query at once, bypassing the debounce that typing uses. When it is nil
	// the toolbar calls OnSearch("") instead, which is also immediate.
	OnClearSearch func()
	// FilterSheet shows the panel as a sheet (the .filter-sheet styling) instead of an inline
	// disclosure. It is opt-in, so existing pages keep the inline layout.
	FilterSheet bool

	FiltersLabel string   // trigger button text, e.g. "Filters"
	FiltersTitle string   // popover title + trigger tooltip
	FilterFields uic.Node // panel body: the labelled field controls
	// ActiveAriaLabel builds the trigger's accessible name from the active-filter count. The visible
	// badge is aria-hidden, so without this a screen reader hears only "Filters" and never the count.
	// The caller supplies a translated string such as "Filters — 3 active". Falls back to FiltersLabel
	// when nil.
	ActiveAriaLabel func(n int) string

	Chips []Chip // active-filter chips (empty hides the row + badge)
	// PreChips are chips that narrow the view but are NOT this page's filters, such as the top bar's
	// member perspective. They render before the "Filtering by" lead-in, so the row does not describe
	// them as filters. The Filters badge does not count them, and the reset does not claim to remove
	// them. Their ✕ still comes back through OnRemoveChip, keyed so the caller can route it.
	PreChips     []Chip
	OnRemoveChip func(key string) // a chip's ✕
	OnClearAll   func()           // the "clear all" link
	// ClearAllLabel is the text for the clear-all link. An EMPTY label hides the link. Use that
	// when chips are showing but none of them is something this reset would remove. The
	// transactions ledger's lens chips are cleared from the top bar, so a reset beside them
	// would advertise "clear all 0 filters" and do nothing.
	ClearAllLabel string
	RemoveLabel   string // aria-label prefix for a chip's ✕

	// SearchAddon is an optional control placed between Search and Filters on the
	// primary row. Other callers retain the existing layout when it is nil.
	SearchAddon uic.Node
	// PrimaryActions are optional controls placed after Filters on the primary row.
	// Actions remain the secondary row, preserving the established layout by default.
	PrimaryActions []uic.Node
	Actions        []uic.Node // trailing toolbar buttons (e.g. Clear, Export CSV)
}

// FilterToolbar is a reusable compact filter bar: an always-visible search box, a "Filters" trigger
// that shows the active-filter count, caller-supplied trailing actions, and a row of removable chips
// below. The filter panel and its open state are owned internally, so callers keep only their own
// filter state. Filters apply live, which is why the panel has no Save button. The DataTable widget
// is the matching piece for the ledger table; both live here so any screen can reuse them.
func FilterToolbar(props FilterToolbarProps) uic.Node {
	return uic.CreateElement(filterToolbar, props)
}

// filterSearchDebounce is how long a typed search waits for the reader to pause.
const filterSearchDebounce = 250 * time.Millisecond

// markSearchPending reveals the searching note, marks the search box busy and dims the results, all
// within the keystroke. It writes the DOM directly. Raising the SearchPending prop would cost
// a full re-render, which is slower than the debounce the note describes.
func markSearchPending(searchID, pendingID, resultsSelector string) {
	doc := domDocument()
	if !doc.Truthy() {
		return
	}
	if el := domNode(searchID); el.Truthy() {
		domValue(el, "setAttribute", "aria-busy", "true")
	}
	if el := domNode(pendingID); el.Truthy() {
		domValue(el.Get("classList"), "remove", "is-idle")
	}
	setSearchStale(resultsSelector, true)
}

// clearSearchPending undoes markSearchPending. A render cannot do this itself: its last output
// already says idle, so it sees no change to write.
func clearSearchPending(searchID, pendingID, resultsSelector string) {
	doc := domDocument()
	if !doc.Truthy() {
		return
	}
	if el := domNode(searchID); el.Truthy() {
		domValue(el, "setAttribute", "aria-busy", "false")
	}
	if el := domNode(pendingID); el.Truthy() {
		domValue(el.Get("classList"), "add", "is-idle")
	}
	setSearchStale(resultsSelector, false)
}

// setSearchStale dims the results and makes them inert while a typed query has not been applied. It
// uses a class of this component's own rather than aria-busy, which the table sets for its own
// reasons. Clearing a shared attribute here could cancel the protection around a sort that is still
// running.
func setSearchStale(selector string, stale bool) {
	if selector == "" {
		return
	}
	doc := domDocument()
	if !doc.Truthy() {
		return
	}
	nodes := domValue(doc, "querySelectorAll", selector)
	for i := range nodes.Get("length").Int() {
		classes := domValue(nodes, "item", i).Get("classList")
		if stale {
			domValue(classes, "add", "is-search-stale")
		} else {
			domValue(classes, "remove", "is-search-stale")
		}
	}
}

func filterToolbar(props FilterToolbarProps) uic.Node {
	open := uic.UseState(false)
	uid := uic.UseId()
	// Escape closes the filter panel, from the trigger or from inside the panel. The hook is
	// registered unconditionally, never inside the If that renders the panel, so its position
	// stays stable.
	onPanelKey := uic.UseEvent(func(e uic.KeyboardEvent) {
		if e.GetKey() == "Escape" && open.Get() {
			e.PreventDefault()
			focusFilterTrigger(uid)
			open.Set(false)
		}
	})
	closePanel := func() {
		focusFilterTrigger(uid)
		open.Set(false)
	}
	panelMounted := UsePanelFade(open.Get())
	// The keystroke writes directly to these two nodes, before the app re-renders.
	searchID, pendingID := "fctrl-search-"+uid, "fctrl-pending-"+uid
	searchKey := "filter-search:" + uid
	// A typed query applies after filterSearchDebounce of quiet, so a word typed at speed is one
	// search rather than one per letter. The note and the stale rows still answer the keystroke
	// at once.
	onSearch := uic.UseEvent(func(v string) {
		markSearchPending(searchID, pendingID, props.ResultsSelector)
		debounce.Call(searchKey, filterSearchDebounce, func() {
			if props.OnSearch != nil {
				props.OnSearch(v)
			}
		})
	})
	// On teardown, cancel any pending query so it cannot fire into an unmounted toolbar.
	uic.UseEffect(func() func() {
		return func() { debounce.Flush(searchKey) }
	}, searchKey)
	// clearSearchNow clears the query at once. It drops any pending debounced search first, so
	// the old query cannot land after the clear.
	clearSearchNow := func() {
		debounce.Flush(searchKey)
		if props.OnClearSearch != nil {
			props.OnClearSearch()
			return
		}
		if props.OnSearch != nil {
			props.OnSearch("")
		}
	}
	// onClear is registered here, not inside the If that renders the clear button, so its hook
	// keeps a stable position.
	onClear := uic.UseEvent(clearSearchNow)
	// Escape clears the query, the keyboard equivalent of the clear button. It skips the debounce,
	// so a pending typed query cannot apply after the clear.
	onSearchKey := uic.UseEvent(func(e uic.KeyboardEvent) {
		if e.GetKey() == "Escape" {
			e.PreventDefault()
			clearSearchNow()
		}
	})
	// Clear the stale marking on the render where the query has actually been applied. The marking was
	// made outside the vdom, so nothing else would remove it. The effect runs on every render, so a
	// results table is never left inert.
	uic.UseLayoutEffect(func() func() {
		if !props.SearchPending {
			clearSearchPending(searchID, pendingID, props.ResultsSelector)
		}
		return nil
	})
	n := len(props.Chips)

	// Pressing "f" opens the filter panel. It is ignored while focus is in a text field or a modifier
	// key is held. The listener is active while the toolbar is mounted.
	uic.UseDocumentEvent("keydown", func(e uic.Event) {
		if e.GetKey() != "f" || e.GetMetaKey() || e.GetCtrlKey() || e.GetAltKey() {
			return
		}
		if active := domDocument().Get("activeElement"); active.Truthy() {
			switch active.Get("tagName").String() {
			case "INPUT", "TEXTAREA", "SELECT":
				return
			}
			if active.Get("isContentEditable").Bool() {
				return
			}
		}
		e.PreventDefault()
		open.Set(true)
	})

	// The accessible name carries the active count, because the badge is aria-hidden.
	ariaLabel := props.FiltersLabel
	if props.ActiveAriaLabel != nil {
		ariaLabel = props.ActiveAriaLabel(n)
	}
	// The Filters button is a ToolButton: a filter glyph, the label, and the active count as an
	// accent badge. It stays highlighted while the panel is open.
	badge := ""
	if n > 0 {
		badge = strconv.Itoa(n)
	}
	// aria-busy mirrors SearchPending. The note's class carries the same state.
	searchBusy := boolAttr(props.SearchPending)
	pendingCls := "fctrl-pending is-idle"
	if props.SearchPending {
		pendingCls = "fctrl-pending"
	}
	// The panel is an inline disclosure, not a modal, so the table stays visible beneath it.
	// aria-expanded and aria-controls describe that. aria-haspopup="dialog" would promise a modal.
	trigger := ToolButton(ToolButtonProps{
		Label: props.FiltersLabel, Icon: icon.Filter, Badge: badge, Active: n > 0, Open: open.Get(),
		Title: ariaLabel, AriaLabel: ariaLabel,
	},
		Attr("id", "filter-trigger-"+uid), Attr("aria-expanded", boolAttr(open.Get())),
		Attr("aria-controls", "filter-inline-panel"),
		OnKeyDown(onPanelKey), OnClick(func() { open.Set(!open.Get()) }),
	)

	renderChip := func(c Chip) uic.Node {
		remove := props.RemoveLabel
		if c.RemoveLabel != "" {
			remove = c.RemoveLabel
		}
		return uic.CreateElement(filterChip, filterChipProps{
			Label: c.Label, Key: c.Key, Class: c.Class,
			RemoveLabel: remove, Named: c.RemoveLabel == "",
			OnRemove: props.OnRemoveChip,
		})
	}
	chips := MapKeyed(props.Chips, func(c Chip) any { return c.Key }, renderChip)
	preChips := MapKeyed(props.PreChips, func(c Chip) any { return c.Key }, renderChip)

	searchCls := "fctrl fctrl-search"
	if props.Search != "" {
		searchCls += " is-active"
	}
	primaryActions := make([]any, 0, len(props.PrimaryActions))
	for _, action := range props.PrimaryActions {
		primaryActions = append(primaryActions, action)
	}
	panelTitle := props.FiltersTitle
	if props.FilterSheet {
		panelTitle = props.FiltersLabel
	}
	return Div(
		// Two rows. Row 1 holds the search field, the Filters trigger and the primary actions.
		// Row 2 holds the remaining actions, so the buttons never wrap into a ragged line.
		Div(css.Class("filter-toolbar"),
			Div(css.Class("filter-toolbar-primary"),
				// The search box uses the shared .fctrl control style: a magnifier, a borderless input,
				// and a clear button that shows only while a query is set.
				Label(css.Class(searchCls),
					Icon(icon.Search, css.Class(tw.ShrinkO, tw.W35, tw.H35)),
					// FieldValue, not Value: props.Search trails the typed text by several keystrokes,
					// because the search is debounced. FieldValue carries the value as an attribute,
					// so the reconciler never writes that trailing value back over the box. A
					// write-back used to delete characters mid-word.
					Input(css.Class("fctrl-input"), Type("text"), FieldValue(props.Search),
						Attr("id", searchID), Attr("data-testid", "filter-toolbar-search"), Attr("aria-controls", pendingID),
						Attr("role", "searchbox"), Attr("aria-label", props.SearchLabel), Placeholder(props.SearchLabel),
						// While the debounce is still pending the rows below are the previous query's.
						// aria-busy tells assistive technology the region is mid-update, and the visible
						// note beside it tells everyone else, so nobody acts on a result the query has
						// already excluded.
						Attr("aria-busy", searchBusy),
						OnInput(onSearch), OnKeyDown(onSearchKey)),
					// The note is always in the DOM and hidden by a class, rather than rendered only
					// while SearchPending is true. That flag is app state, so raising it costs a full
					// re-render of a heavy screen. The note then arrived after the debounce it describes
					// had nearly elapsed, which made it useless as feedback.
					//
					// With the element already present, the input handler reveals it on the keystroke
					// (see markSearchPending), and the render that follows agrees. The class belongs to
					// the vdom, so when the query commits the ordinary render hides the note again.
					Span(ClassStr(pendingCls), Attr("id", pendingID),
						Attr("data-testid", "filter-search-pending"),
						Attr("role", "status"), Attr("aria-live", "polite"),
						uistate.T("filters.searchPending")),
					// The name says what the control clears. A bare "Clear" would not say how much of the
					// view it removes, since this page also has a filter reset and a panel close.
					If(props.Search != "", Button(css.Class("fctrl-clear"), Type("button"),
						Attr("data-testid", "filter-clear-search"),
						Attr("aria-label", uistate.T("action.clearSearch")),
						Attr("title", uistate.T("action.clearSearch")), OnClick(onClear),
						Icon(icon.Close, css.Class(tw.W3, tw.H3)))),
				),
				If(props.SearchAddon != nil, props.SearchAddon),
				trigger,
				If(len(primaryActions) > 0, Fragment(primaryActions...)),
			),
			If(len(props.Actions) > 0, Div(css.Class("filter-toolbar-actions"), props.Actions)),
		),
		// The chip row is the view's summary, so it reads as a sentence: "Filtering by: Groceries,
		// July 2026". The lead-in also gives the reset link something to belong to.
		//
		// PreChips sit before the lead-in, because they are not filters. Putting "Viewing as Priya"
		// under "Filtering by" would merge two kinds of state that this row exists to keep apart. When
		// only PreChips are present, the lead-in and the reset are both absent, so nothing on the row
		// claims to be a filter.
		// IfElse keeps one node in this slot whether or not chips exist. When the chips came and
		// went through a bare If, the inline panel below was remounted, and a multi-select sheet
		// inside it closed on its first pick.
		IfElse(n > 0 || len(props.PreChips) > 0, Div(css.Class("filter-chips"), Attr("data-testid", "filter-summary"),
			preChips,
			If(n > 0, Span(css.Class("filter-chips-lead"), Text(uistate.T("filters.summaryLead")))),
			chips,
			If(props.ClearAllLabel != "", Button(css.Class("btn-link chip-clear-all"), Type("button"),
				Attr("data-testid", "filter-clear-all"),
				Attr("aria-label", props.ClearAllLabel),
				OnClick(func() {
					if props.OnClearAll != nil {
						props.OnClearAll()
					}
				}),
				props.ClearAllLabel)),
		), Span(Attr("hidden", ""))),
		// The inline panel has no backdrop, so the table stays visible below while filters change.
		// It stays mounted through its fade-out (UsePanelFade).
		If(open.Get() || panelMounted, Div(ClassStr(filterPanelClass(props.FilterSheet, !open.Get())),
			Attr("id", "filter-inline-panel"),
			Attr("role", "region"), Attr("aria-label", panelTitle),
			// Escape from anywhere inside the panel, not just from the trigger.
			OnKeyDown(onPanelKey),
			Div(css.Class("filter-inline-header"),
				H3(css.Class("filter-inline-title", tw.Flex, tw.ItemsCenter, tw.Gap2),
					Icon(icon.Filter, css.Class(tw.W4, tw.H4)), Span(panelTitle)),
				// This ✕ closes the panel and changes no filter. It sits among controls that do
				// remove filters, so its label says what it does.
				Button(css.Class("set-close"), Type("button"),
					Attr("data-testid", "filter-panel-close"),
					Attr("aria-label", uistate.T("filters.closePanel")),
					Attr("title", uistate.T("filters.closePanel")),
					OnClick(closePanel), Icon(icon.Close, css.Class(tw.W4, tw.H4))),
			),
			Div(css.Class("filter-inline-body"), Attr("data-scroll-owner", "sheet"), props.FilterFields),
			If(props.FilterSheet, Div(css.Class("filter-sheet-footer"),
				Button(css.Class("btn-link filter-sheet-clear"), Type("button"),
					Attr("data-testid", "filter-sheet-clear-all"),
					OnClick(func() {
						if props.OnClearAll != nil {
							props.OnClearAll()
						}
					}),
					uistate.T("transactions.clearAllFilters")),
				Button(css.Class("btn btn-primary filter-sheet-done"), Type("button"),
					Attr("data-testid", "filter-sheet-done"),
					OnClick(closePanel),
					uistate.T("action.done")),
			)),
		)),
	)
}

// filterPanelClass returns the classes of the filter panel: the fade style, the sheet variant when
// sheet is set, and is-leaving while the panel fades out.
func filterPanelClass(sheet, leaving bool) string {
	class := "filter-inline-panel fade-panel"
	if sheet {
		class += " filter-sheet"
	}
	if leaving {
		class += " is-leaving"
	}
	return class
}

// filterChipProps configures one chip. It is its own component so the remove
// button's OnClick hook sits at a stable render position. The chip list is variable-length, so a
// hook registered directly in the loop would not keep its position.
type filterChipProps struct {
	Label       string
	Key         string
	Class       string
	RemoveLabel string
	// Named asks the chip to append its own label to RemoveLabel, turning the
	// toolbar's generic "Remove this filter" into "Remove this filter: Groceries"
	// so a screen-reader user can tell one ✕ from the next. A chip that supplied
	// its own complete RemoveLabel sets this false and is left as written.
	Named    bool
	OnRemove func(string)
}

// filterChip renders one removable chip.
func filterChip(props filterChipProps) uic.Node {
	cls := "filter-chip"
	if props.Class != "" {
		cls += " " + props.Class
	}
	// The ✕'s accessible name names the chip it removes, so a screen-reader user can tell
	// several ✕s apart.
	removeLabel := props.RemoveLabel
	if props.Named && removeLabel != "" && props.Label != "" {
		removeLabel = removeLabel + ": " + props.Label
	}
	return Span(ClassStr(cls),
		Span(css.Class("chip-text"), props.Label),
		Button(css.Class("chip-x"), Type("button"), Attr("aria-label", removeLabel),
			OnClick(func() {
				if props.OnRemove != nil {
					props.OnRemove(props.Key)
				}
			}),
			Icon(icon.Close, css.Class(tw.W3, tw.H3))),
	)
}

// focusFilterTrigger returns keyboard focus to the Filters button of the toolbar with the given uid.
func focusFilterTrigger(uid string) {
	if trigger := domNode("filter-trigger-" + uid); trigger.Truthy() {
		domValue(trigger, "focus")
	}
}
