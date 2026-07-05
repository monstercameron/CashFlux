// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

type moreMenuProps struct {
	OnDashboard bool
}

// MoreMenu is the top bar's overflow ("⋯") menu. On narrow screens — where the
// responsive CSS hides the inline secondary toggles (Customize, Theme, Help,
// Music) so the bar stays one row — this menu surfaces those same actions as
// LABELED rows, so nothing becomes unreachable and the once-cryptic icons gain
// names. Its items re-trigger the same actions as the inline buttons (which stay
// mounted, just visually hidden), so no stateful effect — e.g. MuzakToggle's
// audio player — is duplicated here. The popover/backdrop are always rendered and
// shown/hidden with a class so the On* hooks keep stable positions.
func MoreMenu(props moreMenuProps) uic.Node {
	open := uic.UseState(false)
	menuID := uic.UseId()
	nav := router.UseNavigate()
	pAtom := uistate.UsePrefs()
	settings := uistate.UseSettings()
	closeMenu := func() { open.Set(false) }

	// Escape / outside-click dismissal, matching the +Add menu.
	ui.DismissPopover(open.Get(), menuID, closeMenu)

	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}

	// item builds one labeled menu row. Called a fixed number of times at stable
	// positions (If only gates RENDERING — the node, and thus its OnClick hook, is
	// always built), so the hook sequence is constant.
	item := func(label string, ic icon.Name, onSelect func()) uic.Node {
		return Button(css.Class("add-item", tw.Flex, tw.ItemsCenter, tw.Gap25), Type("button"), Attr("role", "menuitem"),
			OnClick(func() { closeMenu(); onSelect() }),
			ui.Icon(ic, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(label),
		)
	}

	// Theme cycle mirrors ThemeToggle (Dark → Light → System → Dark).
	p := pAtom.Get()
	cycleTheme := func() {
		next := prefs.ThemeDark
		switch p.Theme {
		case prefs.ThemeDark:
			next = prefs.ThemeLight
		case prefs.ThemeLight:
			next = prefs.ThemeSystem
		}
		np := pAtom.Get()
		np.Theme = next
		uistate.ApplyPrefs(np)
		uistate.PersistPrefs(np)
		pAtom.Set(np)
		uistate.ApplyTheme(uistate.LoadTheme())
	}
	themeLabel := uistate.T("topbar.theme") + " · " + uistate.T("settings.theme"+themeWord(p.Theme))

	return Div(css.Class("add-wrap topbar-more", tw.Flex, tw.ItemsCenter), Attr("id", menuID),
		Button(css.Class("icon-btn more-btn", tw.W7, tw.H7, tw.TextDim, tw.HoverTextFg), Type("button"),
			Attr("title", uistate.T("topbar.more")), Attr("aria-label", uistate.T("topbar.more")),
			Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
			Attr("data-testid", "topbar-more"),
			OnClick(func() { open.Set(!open.Get()) }),
			ui.Icon(icon.MoreH, css.Class(tw.W5, tw.H5)),
		),
		Div(ClassStr("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(ClassStr("add-menu open-left"+hidden), Attr("role", "menu"),
			// Settings leads the menu — the global panel's single entry point now
			// that the rail's household card no longer opens it.
			Button(css.Class("add-item", tw.Flex, tw.ItemsCenter, tw.Gap25), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "topbar-settings"),
				OnClick(func() { closeMenu(); settings.Set(uistate.Global()) }),
				ui.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("topbar.settings")),
			),
			If(props.OnDashboard, item(uistate.T("dashboard.customize"), icon.Customize, func() { nav.Navigate(uistate.RoutePath("/widget-manager")) })),
			item(themeLabel, icon.Appearance, cycleTheme),
			item(uistate.T("nav.help"), icon.HelpCircle, func() { nav.Navigate(uistate.RoutePath("/help")) }),
		),
	)
}
