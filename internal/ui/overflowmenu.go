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

// OverflowMenuItem describes one entry in an OverflowMenu popover.
type OverflowMenuItem struct {
	// Label is the visible text of the entry. It is also the entry's accessible name.
	Label string
	// Icon is an optional leading glyph. Leave it empty to omit the icon.
	Icon icon.Name
	// OnSelect runs when the reader chooses the entry, by click, Enter or Space. It may be nil.
	// The menu closes when an entry is chosen, whether or not OnSelect is set. An entry with a
	// Submenu opens that submenu instead and does not run OnSelect.
	OnSelect func()
	// TestID is the data-testid of the entry, for tests. Empty adds none.
	TestID string
	// Hidden leaves the entry out of the menu. Prefer it to building the slice conditionally, so
	// the slice keeps one fixed order.
	Hidden bool
	// Section starts a labelled group at this entry. Repeat the same name on the entries that
	// follow to keep them in the group. The heading is drawn once, above the first entry of the
	// group. It is aria-hidden, so it is a visual grouping and not part of any entry's name.
	Section string
	// Danger paints the entry as destructive, or as one that changes reported figures, so Delete
	// and Exclude do not look like Rename. It only sets the look. The entry's own handler still
	// decides whether to ask for confirmation.
	Danger bool
	// Submenu lists actions nested under this entry. The entry stays a menuitem. Enter or the
	// Right arrow opens the child menu, and the Left arrow or Escape closes it.
	Submenu []OverflowMenuItem
}

// OverflowMenuProps configures an OverflowMenu trigger and popover.
type OverflowMenuProps struct {
	// Items are the entries shown in the popover.
	Items []OverflowMenuItem
	// TriggerLabel is the accessible name of the ⋯ trigger, used for its aria-label and title.
	// It defaults to "More actions" when empty.
	TriggerLabel string
	// TriggerTestID is the data-testid of the trigger. Empty adds none.
	TriggerTestID string
	// TriggerText, when set, labels the trigger: the ⋯ glyph followed by this text. Use it in a
	// toolbar where a bare glyph would be unclear.
	TriggerText string
	// TriggerClass adds classes to the trigger. The trigger is always the shared tool button, so
	// these add to its look rather than replace it.
	TriggerClass string
	// TriggerVariant tints the trigger as ToolButton does: "primary" for the page's main action.
	TriggerVariant string
	// TriggerIcon replaces the ⋯ glyph. Use a chevron when the trigger picks a variant of the
	// action beside it, as the second half of a split button does. A ⋯ there reads as one more
	// generic menu, and readers would miss the variants.
	TriggerIcon icon.Name
	// ExtraContent is rendered after the menu entries. Use it for an existing component that has
	// to live inside this menu. Setting it keeps the panel mounted while the menu is closed, and
	// the hidden-menu class keeps the panel off screen.
	ExtraContent uic.Node
}

// OverflowMenu renders the standard "⋯" overflow pattern: a trigger button that opens a
// `.add-menu` popover (role=menu) with one `.add-item` per entry. Screens use it instead of
// writing the add-wrap and add-menu markup by hand. Each instance is its own component, so its
// hooks sit at stable render positions and it is safe to embed in any row.
func OverflowMenu(props OverflowMenuProps) uic.Node {
	return uic.CreateElement(overflowMenu, props)
}

func overflowMenu(props OverflowMenuProps) uic.Node {
	// The wrapper's id is unique to this instance. The document listeners use it to tell a press
	// inside this menu from a press inside another, since several menus can be on the page at once.
	id := uic.UseId()
	menu := UseMenu(id)

	triggerLabel := props.TriggerLabel
	if triggerLabel == "" {
		triggerLabel = "More actions"
	}
	toggleOpen := uic.UseEvent(Prevent(func() { menu.Toggle() }))
	// Enter and Space open the menu from the keyboard.
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
	// The trigger is the shared ToolButton, so it looks like the other tool buttons beside it.
	// It stays highlighted while the menu is open.
	trigger := ToolButton(ToolButtonProps{
		Label: props.TriggerText, Icon: trigIcon, Title: triggerLabel, AriaLabel: triggerLabel,
		Open: menu.Open, Class: props.TriggerClass, Variant: props.TriggerVariant, TestID: props.TriggerTestID,
	},
		Attr("aria-haspopup", "menu"), Attr("aria-expanded", boolAttr(menu.Open)),
		OnKeyDown(openFromKeyboard), OnClick(toggleOpen),
	)

	// The panel carries the menu's classes and role, and the entries follow it. A section heading
	// goes above the first entry of each run of entries that share a section. The heading is
	// aria-hidden with role presentation, so it is not part of the menu's accessible structure.
	menuArgs := []any{ClassStr(menu.PanelClass()), Attr("role", "menu")}
	if menu.Shown {
		lastSection := ""
		sectionID := ""
		sectionRun := 0
		for _, item := range props.Items {
			if item.Hidden {
				continue
			}
			if item.Section != lastSection {
				lastSection = item.Section
				sectionID = ""
				if item.Section != "" {
					// Each run of entries gets its own id, so two runs with one name never clash.
					sectionRun++
					sectionID = fmt.Sprintf("%s-section-%d", id, sectionRun)
					menuArgs = append(menuArgs,
						Div(css.Class("add-menu-section"), Attr("id", sectionID), Attr("role", "presentation"), Attr("aria-hidden", "true"), item.Section))
				}
			}
			menuArgs = append(menuArgs, uic.CreateElement(overflowMenuItemBtn, overflowMenuItemProps{
				Item:      item,
				CloseMenu: func() { menu.Set(false) },
				SectionID: sectionID,
			}))
		}
	}
	if props.ExtraContent != nil {
		menuArgs = append(menuArgs, props.ExtraContent)
	}

	// Outside presses close the menu through DismissPopover's document listener, not through a
	// full-screen layer. A full-screen layer would take the clicks meant for the entries when the
	// row sits inside a bento tile's stacking context.
	var panel uic.Node = Fragment()
	if menu.Shown || props.ExtraContent != nil {
		panel = Div(menuArgs...)
	}
	return Div(css.Class(menu.WrapClass("add-wrap")), Attr("id", id), trigger, panel)
}

// menuPlacementClass returns the flip classes an open menu takes: open-left and open-up. Each
// class has a leading space, since the caller appends the result to the panel's class list.
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

// anchorOverflowMenu keeps an open menu beside its trigger, and flips it to the side and direction
// that fit, within the nearest transaction table when there is one. It places the menu before
// paint when the menu opens, and again on every window resize or scroll while it is open.
func anchorOverflowMenu(isOpen bool, wrapID string, setPlacement func(string)) {
	uic.UseLayoutEffect(func() func() {
		if isOpen {
			placeOverflowMenu(wrapID, setPlacement)
		}
		return nil
	}, isOpen, wrapID)
	// The window listeners stay attached while the menu is closed, so they check isOpen themselves.
	reposition := func(uic.Event) {
		if isOpen {
			placeOverflowMenu(wrapID, setPlacement)
		}
	}
	uic.UseWindowEvent("resize", reposition, isOpen, wrapID)
	uic.UseWindowEvent("scroll", reposition, isOpen, wrapID)
}

// Pixel spacing for an open menu. menuMargin keeps the menu off the edge of the viewport, or of
// the transaction table around it. menuGap is the space between the trigger and the menu.
const (
	menuMargin = 8.0
	menuGap    = 6.0
)

// minCappedMenuHeight is the least space a menu is capped to. With less room than this, the menu
// keeps its full height rather than being squeezed into a sliver.
const minCappedMenuHeight = 80.0

// placeOverflowMenu measures an open menu against its trigger and the viewport, or against the
// transaction table around it. It reports the flip classes that fit, and caps the menu's height
// when the menu does not fit in the space below or above the trigger.
func placeOverflowMenu(wrapID string, setPlacement func(string)) {
	wrap := domNode(wrapID)
	if !wrap.Truthy() {
		return
	}
	menu := domValue(wrap, "querySelector", ".add-menu")
	if !menu.Truthy() {
		return
	}
	style := menu.Get("style")
	// Clear the height cap from the last placement, so the full height is measured.
	domSet(style, "maxHeight", "")
	domSet(style, "overflowY", "")
	// The menu grows in over its fade, so its height reads low until the animation settles.
	// Suspend the animation to measure the height it settles at. The animation is restored after
	// the last measurement.
	domSet(style, "animation", "none")
	menuRect := domValue(menu, "getBoundingClientRect")
	window := domDocument().Get("defaultView")
	viewportWidth := window.Get("innerWidth").Float()
	viewportHeight := window.Get("innerHeight").Float()
	boundaryTop, boundaryBottom := menuMargin, viewportHeight-menuMargin
	if table := domValue(wrap, "closest", ".txn-table"); table.Truthy() {
		tableRect := domValue(table, "getBoundingClientRect")
		boundaryTop = max(boundaryTop, tableRect.Get("top").Float()+menuMargin)
		boundaryBottom = min(boundaryBottom, tableRect.Get("bottom").Float()-menuMargin)
	}
	// Decide the side from the trigger's position and the menu's width, not from where the menu
	// sits now. The flip classes from the last open are still applied, so measuring the current
	// position would give a different answer on each open.
	wrapRect := domValue(wrap, "getBoundingClientRect")
	openLeft := wrapRect.Get("left").Float()+menuRect.Get("width").Float() > viewportWidth-menuMargin
	fullHeight := max(menuRect.Get("height").Float(), menu.Get("scrollHeight").Float())
	domSet(style, "animation", "")
	openUp := wrapRect.Get("bottom").Float()+menuGap+fullHeight > boundaryBottom
	available := boundaryBottom - wrapRect.Get("bottom").Float() - menuGap
	if openUp {
		available = wrapRect.Get("top").Float() - boundaryTop - menuGap
	}
	// When the menu does not fit, cap it to the room available and let it scroll inside the cap.
	if fullHeight > available && available > minCappedMenuHeight {
		domSet(style, "maxHeight", fmt.Sprintf("%.0fpx", available))
		domSet(style, "overflowY", "auto")
	}
	setPlacement(menuPlacementClass(openLeft, openUp))
}

// overflowMenuItemProps carries an entry's data and its close callbacks into overflowMenuItemBtn.
// Each entry is its own component and owns its click and key handlers, so no On* option is called
// inside the item loop.
type overflowMenuItemProps struct {
	Item      OverflowMenuItem
	CloseMenu func()
	// CloseSubmenu is set only on entries nested inside a submenu. It closes that submenu and
	// returns focus to the parent entry.
	CloseSubmenu func()
	// SectionID is the id of the section heading this entry sits under. The entry names it as its
	// accessible description, so a screen reader hears which group the entry is in. Empty for an
	// entry with no section.
	SectionID string
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
	// ArrowRight and Enter open a submenu. ArrowLeft and Escape close an open submenu first. When
	// no submenu is open, a nested entry returns focus to its parent. Escape is not stopped on a
	// top-level entry, so it reaches the menu and closes it.
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
				focusMenuItem(domNode(itemID))
			}
		}
	})

	// The danger class is the app's shared destructive style, so every menu shows risk the same way.
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
		btnArgs = append(btnArgs, Attr("aria-haspopup", "menu"), Attr("aria-expanded", boolAttr(submenuOpen.Get())))
	}
	// The visible label is the accessible name. The section heading is visual only, so it is not
	// part of the name; the entry describes itself with the heading instead.
	if props.SectionID != "" {
		btnArgs = append(btnArgs, Attr("aria-describedby", props.SectionID))
	}
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
					focusMenuItem(domNode(itemID))
				},
			}))
		}
	}
	return Div(ClassStr("add-submenu-wrap"), Attr("id", itemID+"-wrap"), parent,
		If(submenuOpen.Get(), Div(children...)))
}

// focusOverflowSubmenu moves focus to the first entry of an opened submenu. The submenu is a
// sibling of the parent entry's button, so the lookup starts at the entry's wrapper.
func focusOverflowSubmenu(itemID string) {
	wrap := domNode(itemID + "-wrap")
	if !wrap.Truthy() {
		return
	}
	menu := domValue(wrap, "querySelector", ".add-submenu")
	if menu.Truthy() {
		focusMenuItem(domValue(menu, "querySelector", `[role="menuitem"]`))
	}
}

// focusMenuItem focuses a menu entry without scrolling the page to it.
func focusMenuItem(item interop.Value) {
	if !item.Truthy() {
		return
	}
	item.Call("focus", map[string]any{"preventScroll": true})
}
