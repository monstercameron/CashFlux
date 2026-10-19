// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
	// TriggerClass overrides the trigger button's class (default "btn"). Pass e.g.
	// "btn btn-tool" so the overflow trigger matches a labeled toolbar's buttons.
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
	open := uic.UseState(false)
	// Stable per-instance id on the wrapper so the document-level dismissal
	// listeners can tell clicks inside THIS menu from clicks in another instance
	// (many overflow menus can be on a page at once).
	id := uic.UseId()

	triggerLabel := props.TriggerLabel
	if triggerLabel == "" {
		triggerLabel = "More actions"
	}
	toggleOpen := uic.UseEvent(Prevent(func() { open.Set(!open.Get()) }))

	// Keyboard + outside-click dismissal (WAI-ARIA menu button). Registered only
	// while open, torn down on close/unmount (mirrors addmenu.go):
	//   • Escape closes and returns focus to the ⋯ trigger;
	//   • pointerdown outside this instance's wrapper closes it — robust regardless
	//     of the `.add-backdrop` stacking (the fixed backdrop doesn't paint over
	//     page content, so it can't be relied on for outside-clicks).
	dismissOverflowMenu(open.Get(), id, func() { open.Set(false) })
	// Keep the popover inside the viewport (the ⋯ trigger usually sits near a row's
	// right edge): flip it left/up when its natural below-right position would overflow.
	anchorOverflowMenu(open.Get(), id)

	// Use the `hidden-menu` class: it's the one the stylesheet actually hides
	// (display:none on `.add-menu`). The bare `hidden` class is unstyled.
	// aria-expanded reflects the popover state for assistive tech.
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}

	trigClass := props.TriggerClass
	if trigClass == "" {
		trigClass = "btn"
	}
	trigIcon := props.TriggerIcon
	if trigIcon == "" {
		trigIcon = icon.MoreH
	}
	triggerArgs := []any{
		css.Class(trigClass),
		Type("button"),
		Attr("title", triggerLabel),
		Attr("aria-label", triggerLabel),
		Attr("aria-haspopup", "menu"),
		Attr("aria-expanded", expanded),
		OnClick(toggleOpen),
		Icon(trigIcon, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
	}
	// Labeled variant: append the visible text after the glyph (glyph + label), so the
	// overflow reads as "⋯ More" rather than a bare, ambiguous dots button.
	if props.TriggerText != "" {
		triggerArgs = append(triggerArgs, Span(props.TriggerText))
	}
	if props.TriggerTestID != "" {
		triggerArgs = append(triggerArgs, Attr("data-testid", props.TriggerTestID))
	}

	menuClass := "add-menu"
	if !open.Get() {
		menuClass += " hidden-menu"
	}
	menuArgs := []any{ClassStr(menuClass), Attr("role", "menu")}
	// Sections (C572): a heading is emitted the first time a section name appears.
	// It carries role="presentation" so it stays out of the menu's owned children —
	// a role="menu" may only own menuitems (and groups), and a stray labelled div
	// would make the menu structurally invalid to a screen reader.
	//
	// The heading is therefore a SIGHTED affordance, and each item carries the same
	// fact in its own accessible name ("Delete — Remove") so the grouping survives
	// for anyone who never sees the heading. That keeps one flat list of menuitems,
	// which is both valid ARIA and what the existing keyboard handling expects.
	// Items with no Section stay unlabelled at the top, which is what an unsectioned
	// menu (every other caller) renders today.
	if open.Get() {
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
				CloseMenu: func() { open.Set(false) },
			}))
		}
	}
	if props.ExtraContent != nil {
		menuArgs = append(menuArgs, props.ExtraContent)
	}

	// No `.add-backdrop` click-catcher: DismissPopover (above) already closes the menu
	// on any outside pointerdown via a document listener. A fixed backdrop is not only
	// redundant, it's harmful here — as a transparent, viewport-covering element it wins
	// the hit-test over the menu items themselves when the row sits inside a bento tile's
	// stacking context, so clicks on the items land on the backdrop and never fire.
	wrapClass := "add-wrap"
	if open.Get() {
		// The transaction-table styles use this state to tint the owning row while
		// its menu is open. It is harmless for menus outside a table.
		wrapClass += " overflow-menu-open"
	}
	var menu uic.Node = Fragment()
	if open.Get() || props.ExtraContent != nil {
		menu = Div(menuArgs...)
	}
	return Div(css.Class(wrapClass), Attr("id", id), Button(triggerArgs...), menu)
}

// dismissOverflowMenu installs the menu-button keyboard and outside-pointer
// listeners when the host DOM supports them. The component testkit intentionally
// has a small DOM surface, so missing browser APIs must not panic a wasm effect.
func dismissOverflowMenu(isOpen bool, wrapID string, onClose func()) {
	openKey := "closed"
	if isOpen {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		if !isOpen || wrapID == "" || onClose == nil {
			return nil
		}
		doc := js.Global().Get("document")
		if !doc.Truthy() || doc.Get("addEventListener").Type() != js.TypeFunction || doc.Get("removeEventListener").Type() != js.TypeFunction {
			return nil
		}
		var typeahead string
		lastType := time.Time{}
		keyCB := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 {
				return nil
			}
			e := args[0]
			key := e.Get("key").String()
			if key == "Escape" {
				onClose()
				if doc.Get("getElementById").Type() == js.TypeFunction {
					w := doc.Call("getElementById", wrapID)
					if w.Truthy() && w.Get("querySelector").Type() == js.TypeFunction {
						if trigger := w.Call("querySelector", "button"); trigger.Truthy() && trigger.Get("focus").Type() == js.TypeFunction {
							trigger.Call("focus")
						}
					}
				}
				return nil
			}
			if doc.Get("getElementById").Type() != js.TypeFunction {
				return nil
			}
			w := doc.Call("getElementById", wrapID)
			active := doc.Get("activeElement")
			if !w.Truthy() || !active.Truthy() || w.Get("contains").Type() != js.TypeFunction || !w.Call("contains", active).Bool() || w.Get("querySelectorAll").Type() != js.TypeFunction {
				return nil
			}
			items := w.Call("querySelectorAll", `[role="menuitem"]`)
			n := items.Get("length").Int()
			if n == 0 {
				return nil
			}
			cur := -1
			for i := range n {
				if items.Index(i).Equal(active) {
					cur = i
					break
				}
			}
			next := -1
			switch key {
			case "ArrowDown":
				next = (cur + 1) % n
			case "ArrowUp":
				next = (cur - 1 + n) % n
			case "Home":
				next = 0
			case "End":
				next = n - 1
			default:
				if len(key) != 1 || e.Get("ctrlKey").Bool() || e.Get("altKey").Bool() || e.Get("metaKey").Bool() {
					return nil
				}
				now := time.Now()
				if now.Sub(lastType) > 700*time.Millisecond {
					typeahead = ""
				}
				lastType = now
				typeahead += strings.ToLower(key)
				start := (cur + 1) % n
				for offset := range n {
					idx := (start + offset) % n
					label := strings.ToLower(strings.TrimSpace(items.Index(idx).Get("textContent").String()))
					if strings.HasPrefix(label, typeahead) {
						next = idx
						break
					}
				}
				if next < 0 {
					return nil
				}
			}
			e.Call("preventDefault")
			items.Index(next).Call("focus")
			return nil
		})
		downCB := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) == 0 || doc.Get("getElementById").Type() != js.TypeFunction {
				return nil
			}
			w := doc.Call("getElementById", wrapID)
			target := args[0].Get("target")
			if w.Truthy() && target.Truthy() && w.Get("contains").Type() == js.TypeFunction && !w.Call("contains", target).Bool() {
				onClose()
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", keyCB)
		doc.Call("addEventListener", "pointerdown", downCB)
		return func() {
			doc.Call("removeEventListener", "keydown", keyCB)
			doc.Call("removeEventListener", "pointerdown", downCB)
			keyCB.Release()
			downCB.Release()
		}
	}, openKey)
}

// anchorOverflowMenu keeps an open menu attached to its trigger and flips it
// within its nearest transaction table when possible. Browser APIs are checked
// before use so a partial test DOM simply skips visual placement.
func anchorOverflowMenu(isOpen bool, wrapID string) {
	openKey := "closed"
	if isOpen {
		openKey = "open"
	}
	uic.UseEffect(func() func() {
		if !isOpen || wrapID == "" {
			return nil
		}
		win := js.Global()
		doc := win.Get("document")
		if !doc.Truthy() || doc.Get("getElementById").Type() != js.TypeFunction || win.Get("requestAnimationFrame").Type() != js.TypeFunction {
			return nil
		}
		const margin = 8.0
		reposition := js.FuncOf(func(_ js.Value, _ []js.Value) any {
			w := doc.Call("getElementById", wrapID)
			if !w.Truthy() || w.Get("querySelector").Type() != js.TypeFunction || w.Get("getBoundingClientRect").Type() != js.TypeFunction {
				return nil
			}
			menu := w.Call("querySelector", ".add-menu")
			if !menu.Truthy() || menu.Get("getBoundingClientRect").Type() != js.TypeFunction {
				return nil
			}
			cl := menu.Get("classList")
			if !cl.Truthy() || cl.Get("remove").Type() != js.TypeFunction || cl.Get("add").Type() != js.TypeFunction {
				return nil
			}
			cl.Call("remove", "open-left")
			cl.Call("remove", "open-up")
			st := menu.Get("style")
			st.Set("maxHeight", "")
			st.Set("overflowY", "")
			r := menu.Call("getBoundingClientRect")
			vw := win.Get("innerWidth").Float()
			vh := win.Get("innerHeight").Float()
			boundaryTop, boundaryBottom := margin, vh-margin
			if w.Get("closest").Type() == js.TypeFunction {
				if table := w.Call("closest", ".txn-table"); table.Truthy() && table.Get("getBoundingClientRect").Type() == js.TypeFunction {
					tableRect := table.Call("getBoundingClientRect")
					boundaryTop = max(boundaryTop, tableRect.Get("top").Float()+margin)
					boundaryBottom = min(boundaryBottom, tableRect.Get("bottom").Float()-margin)
				}
			}
			if r.Get("right").Float() > vw-margin {
				cl.Call("add", "open-left")
			}
			if r.Get("bottom").Float() > boundaryBottom {
				cl.Call("add", "open-up")
			}
			wr := w.Call("getBoundingClientRect")
			avail := boundaryBottom - wr.Get("bottom").Float() - 6
			if cl.Get("contains").Type() == js.TypeFunction && cl.Call("contains", "open-up").Bool() {
				avail = wr.Get("top").Float() - boundaryTop - 6
			}
			if r.Get("height").Float() > avail && avail > 80 {
				st.Set("maxHeight", fmt.Sprintf("%.0fpx", avail))
				st.Set("overflowY", "auto")
			}
			return nil
		})
		raf := win.Call("requestAnimationFrame", reposition)
		if win.Get("addEventListener").Type() != js.TypeFunction || win.Get("removeEventListener").Type() != js.TypeFunction {
			return func() {
				if win.Get("cancelAnimationFrame").Type() == js.TypeFunction {
					win.Call("cancelAnimationFrame", raf)
				}
				reposition.Release()
			}
		}
		resizeCB := js.FuncOf(func(_ js.Value, _ []js.Value) any { reposition.Invoke(); return nil })
		win.Call("addEventListener", "resize", resizeCB)
		win.Call("addEventListener", "scroll", resizeCB, map[string]any{"capture": true, "passive": true})
		return func() {
			if win.Get("cancelAnimationFrame").Type() == js.TypeFunction {
				win.Call("cancelAnimationFrame", raf)
			}
			win.Call("removeEventListener", "resize", resizeCB)
			win.Call("removeEventListener", "scroll", resizeCB, map[string]any{"capture": true})
			reposition.Release()
			resizeCB.Release()
		}
	}, openKey)
}

// overflowMenuItemProps carries the item data plus the close callback into the
// per-item sub-component. The sub-component owns its click hook so we never
// call On* inside the item loop (the On*-in-loop rule).
type overflowMenuItemProps struct {
	Item      OverflowMenuItem
	CloseMenu func()
}

func overflowMenuItemBtn(props overflowMenuItemProps) uic.Node {
	item := props.Item
	closeMenu := props.CloseMenu
	onSelect := item.OnSelect

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
		Attr("role", "menuitem"),
		OnClick(func() {
			if closeMenu != nil {
				closeMenu()
			}
			if onSelect != nil {
				onSelect()
			}
		}),
	}
	// The section heading is aria-hidden (it cannot be an owned child of role=menu),
	// so each item restates its section in its own accessible name. Without this the
	// grouping would be a purely visual cue and a screen-reader user would hear ten
	// equally-weighted entries — the flat list C572 set out to fix.
	if item.Section != "" {
		btnArgs = append(btnArgs, Attr("aria-label", item.Label+" — "+item.Section))
	}
	if item.TestID != "" {
		btnArgs = append(btnArgs, Attr("data-testid", item.TestID))
	}
	if item.Icon != "" {
		btnArgs = append(btnArgs, Icon(item.Icon, css.Class(tw.W4, tw.H4)))
	}
	btnArgs = append(btnArgs, item.Label)
	return Button(btnArgs...)
}
