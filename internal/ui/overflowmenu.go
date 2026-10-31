// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// ---------------------------------------------------------------------------
// OverflowMenu
// ---------------------------------------------------------------------------

// OverflowMenuItem describes a single entry in an OverflowMenu popover.
type OverflowMenuItem struct {
	// Label is the human-readable text for the menu item.
	Label string
	// Icon is an optional leading glyph. Pass icon.Name("") to omit.
	Icon icon.Name
	// OnSelect is called when the user activates this item (click or Enter/Space).
	// May be nil (item renders but does nothing). The menu closes automatically
	// after any non-nil OnSelect fires.
	OnSelect func()
	// TestID is an optional data-testid attribute for e2e selectors.
	TestID string
	// Hidden suppresses this item from the rendered list when true.
	// Preferred over conditional slice-building at the call site so the
	// item order in the slice always matches intent.
	Hidden bool
	// Section, when set, opens a new labelled group starting at this item. A long
	// menu that mixes routine organization with reporting changes and deletions
	// reads as one flat list where every entry looks equally safe (C572); a section
	// heading plus the separator it draws tells the user what KIND of action they
	// are about to take before they take it. Repeat the same string on consecutive
	// items to keep them in one group; the heading renders once, at the first.
	Section string
	// Danger paints the item as destructive or reporting-altering, so Delete and
	// Exclude never look like Rename. It is a visual tier, not a behaviour — the
	// item's own handler still owns any confirmation.
	Danger bool
	// Submenu contains actions nested under this item. The parent remains a
	// menuitem; Enter/Right opens its child menu and Left/Escape closes it.
	Submenu []OverflowMenuItem
}

// OverflowMenuProps configures an OverflowMenu trigger + popover.
type OverflowMenuProps struct {
	// Items is the list of menu actions shown in the popover.
	Items []OverflowMenuItem
	// TriggerLabel is the accessible name of the ⋯ trigger button
	// (aria-label + title). Defaults to "More actions" when empty.
	TriggerLabel string
	// TriggerTestID is an optional data-testid on the trigger button.
	TriggerTestID string
	// TriggerText, when non-empty, renders the trigger as a LABELED button (the ⋯
	// glyph followed by this text) instead of the default icon-only ⋯. Use it in a
	// labeled toolbar where a bare glyph would be ambiguous.
	TriggerText string
	// TriggerClass adds classes to the trigger for a page that needs one more. The trigger is
	// always the shared tool button, so this does not replace its look.
	TriggerClass string
	// TriggerIcon overrides the ⋯ glyph. The default says "more actions", which is
	// wrong for a menu that is not a grab-bag: a split button's second half means
	// "choose a variant of the action next to me" and has to look like a chevron, or
	// it reads as one more generic kebab and nobody finds the variants (C573).
	TriggerIcon icon.Name
	// ExtraContent appends an optional group after the menu actions. It is intended
	// for an existing screen component that needs to live inside this menu; nil keeps
	// the historical menu DOM unchanged.
	ExtraContent uic.Node
}

// OverflowMenu renders the standard CashFlux "⋯" overflow pattern: a trigger
// button that opens a `.add-menu` popover (role=menu) with one `.add-item` per
// entry. It unifies the hand-rolled `add-wrap`/`add-menu`/`add-backdrop` pattern
// duplicated across accounts, artifacts, and other screens.
//
// OverflowMenu is its own component so its open-state hooks and per-item click
// hooks remain at stable render positions — safe to embed inside any row.
func OverflowMenu(props OverflowMenuProps) uic.Node {
	return uic.CreateElement(overflowMenu, props)
}

func overflowMenu(props OverflowMenuProps) uic.Node {
	// Stable per-instance id on the wrapper so the document-level dismissal listeners can tell
	// clicks inside THIS menu from clicks in another instance (many can be on a page at once).
	id := uic.UseId()
	menu := UseMenu(id)

	triggerLabel := props.TriggerLabel
	if triggerLabel == "" {
		triggerLabel = "More actions"
	}
	toggleOpen := uic.UseEvent(Prevent(func() { menu.Toggle() }))
	openFromKeyboard := uic.UseEvent(func(e uic.KeyboardEvent) {
		if e.GetKey() != "Enter" && e.GetKey() != " " {
			return
		}
		e.PreventDefault()
		menu.Set(true)
	})

	trigIcon := props.TriggerIcon
	if trigIcon == "" {
		trigIcon = icon.MoreH
	}
	// The trigger is the shared ToolButton, so a More menu looks like the Filters button beside it.
	// An open menu keeps it highlighted.
	trigger := ToolButton(ToolButtonProps{
		Label: props.TriggerText, Icon: trigIcon, Title: triggerLabel, AriaLabel: triggerLabel,
		Open: menu.Open, Class: props.TriggerClass, TestID: props.TriggerTestID,
	},
		Attr("aria-haspopup", "menu"), Attr("aria-expanded", boolAttr(menu.Open)),
		OnKeyDown(openFromKeyboard), OnClick(toggleOpen),
	)

	// Menu children: role and class, then the actions. Sections (C572): a heading is emitted the
	// first time a section name appears. It carries role="presentation" so it stays out of the
	// menu's owned children, and each item carries the same fact in its own accessible name, so
	// the grouping survives for anyone who never sees the heading.
	menuArgs := []any{ClassStr(menu.PanelClass()), Attr("role", "menu")}
	if menu.Shown {
		lastSection := ""
		for _, item := range props.Items {
			if item.Hidden {
				continue
			}
			if item.Section != lastSection {
				lastSection = item.Section
				if item.Section != "" {
					menuArgs = append(menuArgs,
						Div(css.Class("add-menu-section"), Attr("role", "presentation"), Attr("aria-hidden", "true"), item.Section))
				}
			}
			menuArgs = append(menuArgs, uic.CreateElement(overflowMenuItemBtn, overflowMenuItemProps{
				Item:      item,
				CloseMenu: func() { menu.Set(false) },
			}))
		}
	}
	if props.ExtraContent != nil {
		menuArgs = append(menuArgs, props.ExtraContent)
	}

	// No backdrop click-catcher: the outside-press listener in dismissOverflowMenu closes the
	// menu. A fixed, viewport-covering backdrop wins the hit-test over the menu items when the
	// row sits inside a bento tile's stacking context, so clicks never reach the items.
	var panel uic.Node = Fragment()
	if menu.Shown || props.ExtraContent != nil {
		panel = Div(menuArgs...)
	}
	return Div(css.Class(menu.WrapClass("add-wrap")), Attr("id", id), trigger, panel)
}

// menuPlacementClass names the flip classes an open menu takes.
func menuPlacementClass(openLeft, openUp bool) string {
	class := ""
	if openLeft {
		class += " open-left"
	}
	if openUp {
		class += " open-up"
	}
	return class
}

// anchorOverflowMenu keeps an open menu attached to its trigger and flips it to the side and
// direction that fit, within its nearest transaction table when possible. Placement is decided
// before paint (UseLayoutEffect), and again whenever the window resizes or scrolls while open.
func anchorOverflowMenu(isOpen bool, wrapID string, setPlacement func(string)) {
	uic.UseLayoutEffect(func() func() {
		if isOpen {
			placeOverflowMenu(wrapID, setPlacement)
		}
		return nil
	}, isOpen, wrapID)
	reposition := func(uic.Event) {
		if isOpen {
			placeOverflowMenu(wrapID, setPlacement)
		}
	}
	uic.UseWindowEvent("resize", reposition, isOpen, wrapID)
	uic.UseWindowEvent("scroll", reposition, isOpen, wrapID)
}

// placeOverflowMenu measures an open menu against its trigger and the viewport, or the
// transaction table around it, and reports the flip classes that fit.
func placeOverflowMenu(wrapID string, setPlacement func(string)) {
	const margin = 8.0
	wrap := domNode(wrapID)
	if !wrap.Truthy() {
		return
	}
	menu := domValue(wrap, "querySelector", ".add-menu")
	if !menu.Truthy() {
		return
	}
	style := menu.Get("style")
	domSet(style, "maxHeight", "")
	domSet(style, "overflowY", "")
	// The menu grows in over its fade, so its height reads low until the animation settles.
	// Suspend the animation to measure the height it settles at, then hand it back.
	domSet(style, "animation", "none")
	rect := domValue(menu, "getBoundingClientRect")
	window := domDocument().Get("defaultView")
	vw := window.Get("innerWidth").Float()
	vh := window.Get("innerHeight").Float()
	boundaryTop, boundaryBottom := margin, vh-margin
	if table := domValue(wrap, "closest", ".txn-table"); table.Truthy() {
		tableRect := domValue(table, "getBoundingClientRect")
		boundaryTop = max(boundaryTop, tableRect.Get("top").Float()+margin)
		boundaryBottom = min(boundaryBottom, tableRect.Get("bottom").Float()-margin)
	}
	// Decide the side from where the menu sits by default, not where it is now. The flip
	// classes are already applied, so measuring the current rect would flip the answer on
	// every open.
	wrapRect := domValue(wrap, "getBoundingClientRect")
	openLeft := wrapRect.Get("left").Float()+rect.Get("width").Float() > vw-margin
	fullHeight := max(rect.Get("height").Float(), menu.Get("scrollHeight").Float())
	domSet(style, "animation", "")
	openUp := wrapRect.Get("bottom").Float()+6+fullHeight > boundaryBottom
	avail := boundaryBottom - wrapRect.Get("bottom").Float() - 6
	if openUp {
		avail = wrapRect.Get("top").Float() - boundaryTop - 6
	}
	if fullHeight > avail && avail > 80 {
		domSet(style, "maxHeight", fmt.Sprintf("%.0fpx", avail))
		domSet(style, "overflowY", "auto")
	}
	setPlacement(menuPlacementClass(openLeft, openUp))
}

// overflowMenuItemProps carries the item data plus the close callback into the
// per-item sub-component. The sub-component owns its click hook so we never
// call On* inside the item loop (the On*-in-loop rule).
type overflowMenuItemProps struct {
	Item         OverflowMenuItem
	CloseMenu    func()
	CloseSubmenu func()
}

func overflowMenuItemBtn(props overflowMenuItemProps) uic.Node {
	item := props.Item
	closeMenu := props.CloseMenu
	onSelect := item.OnSelect
	submenuOpen := uic.UseState(false)
	itemID := uic.UseId()
	// Focus the first entry of a freshly opened submenu once it is in the DOM, before paint.
	uic.UseLayoutEffect(func() func() {
		if submenuOpen.Get() {
			focusOverflowSubmenu(itemID)
		}
		return nil
	}, submenuOpen.Get())
	onKey := uic.UseEvent(func(e uic.KeyboardEvent) {
		switch e.GetKey() {
		case "ArrowRight", "Enter":
			if len(item.Submenu) > 0 {
				e.PreventDefault()
				submenuOpen.Set(true)
			}
		case "ArrowLeft", "Escape":
			if submenuOpen.Get() {
				e.PreventDefault()
				e.StopPropagation()
				submenuOpen.Set(false)
			} else if props.CloseSubmenu != nil {
				e.PreventDefault()
				e.StopPropagation()
				props.CloseSubmenu()
				focusMenuItem(docMenuItem(itemID))
			}
		}
	})

	// `.danger` is the app's existing destructive overflow-item class (the accounts
	// kebab's Delete already uses it) — reused rather than forked, so every menu in
	// the app paints risk the same way.
	cls := "add-item"
	if item.Danger {
		cls += " danger"
	}
	btnArgs := []any{
		ClassStr(cls),
		Type("button"),
		Attr("id", itemID),
		Attr("role", "menuitem"),
		OnKeyDown(onKey),
		OnClick(func() {
			if len(item.Submenu) > 0 {
				submenuOpen.Set(!submenuOpen.Get())
				return
			}
			if closeMenu != nil {
				closeMenu()
			}
			if onSelect != nil {
				onSelect()
			}
		}),
	}
	if len(item.Submenu) > 0 {
		expanded := "false"
		if submenuOpen.Get() {
			expanded = "true"
		}
		btnArgs = append(btnArgs, Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded))
	}
	// The visible item label is also its accessible name. Section headings remain a
	// visual grouping and never leak into the action's name.
	if item.TestID != "" {
		btnArgs = append(btnArgs, Attr("data-testid", item.TestID))
	}
	if item.Icon != "" {
		btnArgs = append(btnArgs, Icon(item.Icon, css.Class(tw.W4, tw.H4)))
	}
	btnArgs = append(btnArgs, item.Label)
	parent := Button(btnArgs...)
	children := []any{ClassStr("add-submenu"), Attr("role", "menu")}
	if submenuOpen.Get() {
		for _, child := range item.Submenu {
			children = append(children, uic.CreateElement(overflowMenuItemBtn, overflowMenuItemProps{
				Item: child, CloseMenu: closeMenu, CloseSubmenu: func() {
					submenuOpen.Set(false)
					focusMenuItem(docMenuItem(itemID))
				},
			}))
		}
	}
	return Div(ClassStr("add-submenu-wrap"), Attr("id", itemID+"-wrap"), parent,
		If(submenuOpen.Get(), Div(children...)))
}

// focusOverflowSubmenu moves focus into an opened submenu. The submenu is a
// sibling of its parent button, so the lookup must start at the item's wrapper.
func focusOverflowSubmenu(itemID string) {
	wrap := docMenuItem(itemID + "-wrap")
	if !wrap.Truthy() {
		return
	}
	menu := domValue(wrap, "querySelector", ".add-submenu")
	if menu.Truthy() {
		focusMenuItem(domValue(menu, "querySelector", `[role="menuitem"]`))
	}
}

// docMenuItem returns the element with the given id, or the zero Value when it is not on the page.
func docMenuItem(id string) interop.Value {
	return domNode(id)
}

// focusMenuItem focuses a menu entry without scrolling the page to it.
func focusMenuItem(item interop.Value) {
	if !item.Truthy() {
		return
	}
	item.Call("focus", map[string]any{"preventScroll": true})
}
