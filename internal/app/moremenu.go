// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	uic "github.com/monstercameron/GoWebComponents/v4/ui"
)

type moreMenuProps struct {
	OnDashboard bool
	// ActivePath is the logical route path, threaded so the relocated Smart-insights
	// peek (which is per-page) knows which route it is rendering for.
	ActivePath string
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
		// Re-base a saved custom theme that disagrees with the new mode — the
		// theme's luminance paints the shell, so without this the cycle is a
		// silent no-op whenever a preset/custom theme is saved. Same rule as
		// the Appearance segmented and ThemeToggle.
		uistate.SyncThemeToMode(np)
		uistate.ApplyTheme(uistate.LoadTheme())
	}
	themeLabel := uistate.T("topbar.theme") + " · " + uistate.T("settings.theme"+themeWord(p.Theme))

	lockEnabled := loadAppLock().Enabled

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
			// DP-header refinement (2026-07-19): a quiet cluster of the ambient controls
			// relocated out of the crowded top bar — the activity/history "Updated …"
			// stamp and the Smart-insights peek. They render as their real components
			// (own fibers → stable hook order; each keeps its exact data-testid +
			// accessible label), and stay mounted whenever this popover is in the DOM so
			// the Smart engine pass keeps running. Each self-hides when it has nothing
			// to show. (The music toggle moved BACK inline to tb-actions — un/mute is a
			// one-click reflex action; see TopBar.)
			// A caption names the cluster (UI/UX task #25): inside a menu of labeled
			// rows, the bare stamp + insights chip read as unlabeled icon soup.
			Div(css.Class("tb-more-quick-wrap"),
				Span(css.Class("tb-more-quick-label", "t-caption"), Attr("aria-hidden", "true"),
					uistate.T("topbar.quickClusterLabel")),
				Div(css.Class("tb-more-quick", tw.Flex, tw.ItemsCenter, tw.FlexWrap, tw.Gap25),
					uic.CreateElement(UpdatedStamp),
					screens.SmartPeekForPath(props.ActivePath),
				),
			),
			// Settings leads the menu — the global panel's single entry point now
			// that the rail's household card no longer opens it.
			Button(css.Class("add-item", tw.Flex, tw.ItemsCenter, tw.Gap25), Type("button"), Attr("role", "menuitem"),
				Attr("data-testid", "topbar-settings"),
				OnClick(func() { closeMenu(); uistate.OpenGlobalSettings() }),
				ui.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("topbar.settings")),
			),
			If(props.OnDashboard, item(uistate.T("dashboard.customize"), icon.Customize, func() { nav.Navigate(uistate.RoutePath("/widget-manager")) })),
			item(themeLabel, icon.Appearance, cycleTheme),
			item(uistate.T("nav.help"), icon.HelpCircle, func() { nav.Navigate(uistate.RoutePath("/help")) }),
			If(lockEnabled, item(uistate.T("applock.cmdLock"), icon.Lock, func() { showAppLockGate() })),
		),
	)
}
