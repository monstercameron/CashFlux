// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

// addMenuWidthPx is the menu's footprint (min-width 210 + padding/border + a small
// safety margin). If the +Add button has less than this much room to the viewport's
// right edge, the menu opens leftward instead so it never overflows off-screen.
const addMenuWidthPx = 224.0

// addMenuShouldOpenLeft reports whether the +Add popover should open leftward
// (toward the rail) because there isn't enough room on the right of the button.
// The button reflows between the left and right of the topbar across widths, so
// this is measured live at open-time rather than guessed from a breakpoint. It is
// a no-op safe default (false → open right) when the DOM isn't reachable.
func addMenuShouldOpenLeft() bool {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return false
	}
	btn := doc.Call("querySelector", ".add-btn")
	if !btn.Truthy() {
		return false
	}
	rect := btn.Call("getBoundingClientRect")
	right := rect.Get("right").Float()
	vw := js.Global().Get("innerWidth").Float()
	return (vw - right) < addMenuWidthPx
}

// AddMenu is the top-bar "+ Add" control: a button that opens a small popover of
// add actions so data entry isn't trapped on each entity's screen (C23). "New
// transaction" opens the inline quick-add panel; the other entities route to
// their screen, where the add form lives. The popover and its click-catching
// backdrop are always rendered and shown/hidden with a CSS class, so the On*
// hooks stay at stable positions (the framework's hooks-in-loops rule).
func AddMenu() uic.Node {
	open := uic.UseState(false)
	openLeft := uic.UseState(false)
	menuID := uic.UseId()
	quickAdd := uistate.UseQuickAdd()
	transferOpen := uistate.UseAcctTransferOpen()
	nav := router.UseNavigate()
	closeMenu := func() { open.Set(false) }
	// Toggle the popover; when opening, pick the side with room so the menu never
	// overflows the viewport (button-on-the-right) nor hides behind the rail
	// (button-on-the-left). Measured live because the button reflows across widths.
	toggleMenu := func() {
		if !open.Get() {
			openLeft.Set(addMenuShouldOpenLeft())
		}
		open.Set(!open.Get())
	}

	// Full WAI-ARIA menu-button keyboard + dismissal behaviour (Escape closes +
	// refocuses the trigger, outside-pointerdown closes, ArrowUp/Down/Home/End rove
	// focus among the [role=menuitem] entries) via the shared helper. The
	// `.add-backdrop` element can't be relied on for outside-clicks — it's fixed
	// inside the topbar's sticky (z-index:5) stacking context, so it doesn't paint
	// over the page content; the helper uses a stacking-immune document listener.
	ui.DismissPopover(open.Get(), menuID, closeMenu)

	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}
	// Direction class for the menu only (not the full-screen backdrop).
	menuDir := ""
	if openLeft.Get() {
		menuDir = " open-left"
	}
	// item builds one menu row. Called a fixed number of times at stable
	// positions (not a variable-length loop), so the OnClick hooks are stable.
	item := func(labelKey string, ic icon.Name, onSelect func()) uic.Node {
		return Button(css.Class("add-item", tw.Flex, tw.ItemsCenter, tw.Gap25), Type("button"), Attr("role", "menuitem"),
			OnClick(func() {
				closeMenu()
				onSelect()
			}),
			ui.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T(labelKey)),
		)
	}
	// aria-expanded reflects the popover state for assistive tech (this is a menu
	// button, now icon-only — so it needs an explicit accessible name + hover title).
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	return Div(css.Class("add-wrap", tw.Flex, tw.ItemsCenter), Attr("id", menuID),
		// C44: the primary "+" is now a one-click "Add transaction" (the overwhelmingly
		// most common action), instead of merely opening a menu that then needs a
		// second click. A small caret beside it opens the full add-anything menu.
		Button(css.Class("add-btn"),
			Attr("title", uistate.T("addmenu.transaction")),
			Attr("aria-label", uistate.T("addmenu.transaction")),
			Attr("data-testid", "add-transaction-btn"),
			OnClick(func() { closeMenu(); quickAdd.Set(true) }),
			ui.Icon(icon.Plus, css.Class(tw.W18px, tw.H18px)),
		),
		Button(css.Class("add-caret"),
			Attr("title", uistate.T("topbar.add")),
			Attr("aria-label", uistate.T("addmenu.more")),
			Attr("aria-haspopup", "menu"),
			Attr("aria-expanded", expanded),
			Attr("data-testid", "add-menu-caret"),
			OnClick(toggleMenu),
			ui.Icon(icon.ChevronDown, css.Class(tw.W4, tw.H4)),
		),
		Div(ClassStr("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(ClassStr("add-menu"+hidden+menuDir), Attr("role", "menu"),
			item("addmenu.transaction", icon.Transactions, func() { quickAdd.Set(true) }),
			// The transfer workflow, reachable from the global add path — not only
			// from Accounts (2026-07-18 assessment: ledger-entry mental model).
			item("addmenu.transfer", icon.Repeat, func() { transferOpen.Set(true) }),
			item("addmenu.account", icon.Accounts, func() { uistate.SetAddTarget("account") }),
			item("addmenu.budget", icon.Budgets, func() { uistate.SetAddTarget("budget") }),
			item("addmenu.goal", icon.Goals, func() { uistate.SetAddTarget("goal") }),
			item("addmenu.task", icon.Todo, func() { uistate.SetAddTarget("task") }),
			item("addmenu.category", icon.Tag, func() { uistate.SetAddTarget("category") }),
			item("addmenu.member", icon.Users, func() { uistate.SetAddTarget("member") }),
			item("addmenu.rule", icon.Filter, func() { uistate.SetAddTarget("rule") }),
			item("addmenu.document", icon.ScanLine, func() { nav.Navigate(uistate.RoutePath("/documents")) }),
		),
	)
}
