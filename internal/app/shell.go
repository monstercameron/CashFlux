// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/navorder"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// ShellProps configures the chrome around a screen.
type ShellProps struct {
	Title    string
	Subtitle string
	// ActivePath is the LOGICAL route path of the screen this Shell renders (e.g.
	// "/accounts"), supplied by the route factory. It is threaded to the rail and
	// breadcrumb so the active highlight moves on navigation: the chrome cannot
	// read it from router.InspectCurrentRoute() at render time because Sidebar and
	// TopBar are memoized (no prop change) and would not re-render on a route
	// change, freezing the highlight (regression covered by e2e/navigation.test.mjs).
	ActivePath string
	View       func() uic.Node
}

// sidebarProps carries the active route path so the rail re-renders and the
// highlight follows each navigation.
type sidebarProps struct {
	ActivePath string
}

// mobileTabItemProps configures one item in the mobile bottom tab bar.
type mobileTabItemProps struct {
	Label  string
	Path   string
	Icon   icon.Name
	Active bool
}

// mobileTabItem is a single tappable entry in the mobile bottom tab bar.
// Its own component so its click-handler hook stays at a stable position
// regardless of how many items are in the bar (the On*-hooks-in-loops rule).
func mobileTabItem(props mobileTabItemProps) uic.Node {
	nav := router.UseNavigate()
	path := props.Path
	cls := "mobile-tab-item"
	if props.Active {
		cls += " active"
	}
	args := []any{
		ClassStr(cls),
		Title(props.Label),
		Attr("aria-label", props.Label),
		OnClick(Prevent(func() {
			if path != "" {
				nav.Navigate(uistate.RoutePath(path))
			}
		})),
	}
	if path != "" {
		args = append(args, Attr("href", uistate.RoutePath(path)))
	}
	if props.Active {
		args = append(args, Attr("aria-current", "page"))
	}
	args = append(args,
		ui.Icon(props.Icon, css.Class(tw.W5, tw.H5)),
		Span(css.Class("mobile-tab-label"), props.Label),
	)
	return A(args...)
}

// mobileTabBarProps carries the active route for the bar.
type mobileTabBarProps struct {
	ActivePath string
}

// MobileTabBar renders a fixed bottom tab bar for phones. The CSS agent
// controls visibility: it is shown only under a phone-width breakpoint and
// hidden on desktop. It surfaces the four primary destinations plus a quick
// +Add shortcut as tappable anchors with icon + label. The desktop left rail
// is left entirely intact — this is purely additive (L11).
func MobileTabBar(props mobileTabBarProps) uic.Node {
	cur := props.ActivePath
	// Five fixed primary slots — enough for one-thumb reach on a 390px viewport.
	// The +Add slot opens the Quick-Add overlay rather than navigating.
	quickAdd := uistate.UseQuickAdd()
	openAdd := uic.UseEvent(func() { quickAdd.Set(true) })
	return Nav(css.Class("mobile-tabbar"), Attr("aria-label", uistate.T("nav.mobileTabLabel")),
		uic.CreateElement(mobileTabItem, mobileTabItemProps{
			Label:  uistate.T("nav.dashboard"),
			Path:   "/",
			Icon:   icon.Dashboard,
			Active: cur == "/",
		}),
		uic.CreateElement(mobileTabItem, mobileTabItemProps{
			Label:  uistate.T("nav.transactions"),
			Path:   "/transactions",
			Icon:   icon.Transactions,
			Active: cur == "/transactions",
		}),
		uic.CreateElement(mobileTabItem, mobileTabItemProps{
			Label:  uistate.T("nav.accounts"),
			Path:   "/accounts",
			Icon:   icon.Accounts,
			Active: cur == "/accounts",
		}),
		uic.CreateElement(mobileTabItem, mobileTabItemProps{
			Label:  uistate.T("nav.budgets"),
			Path:   "/budgets",
			Icon:   icon.Budgets,
			Active: cur == "/budgets",
		}),
		// +Add slot: opens the Quick-Add overlay (same as the top-bar Add button).
		// It is a button — not an anchor — because it has no route destination.
		Button(css.Class("mobile-tab-item mobile-tab-add"), Type("button"),
			Attr("aria-label", uistate.T("action.quickAdd")),
			Title(uistate.T("action.quickAdd")),
			OnClick(openAdd),
			ui.Icon(icon.PlusCircle, css.Class(tw.W5, tw.H5)),
			Span(css.Class("mobile-tab-label"), uistate.T("action.add")),
		),
	)
}

// Shell renders the candidate-C application chrome: a fixed left rail and an
// independently scrolling main pane with a sticky top bar, wrapping the active
// screen's content. (Ported from design/candidate-c.html.)
func Shell(props ShellProps) uic.Node {
	// On each route change: set the document title to the active screen (always,
	// including first load, so tabs/history/screen readers name the page), then
	// move focus into <main> — but not on the first render, so a keyboard user's
	// initial Tab still reaches the skip link. This keeps SPA navigation from
	// leaving focus stranded on the previous screen.
	firstRender := uic.UseRef(true)
	// Subscribe to the shared data-revision atom so a whole-dataset replacement that
	// happens outside any screen — undo/redo, post-decrypt hydration, import — re-
	// renders the active screen even when that screen doesn't read the revision
	// itself. (Also captures the atom so uistate.BumpDataRevision can post from a
	// global callback.)
	_ = uistate.UseDataRevision().Get()
	docTitle := props.Title + " · " + uistate.T("app.name")
	uic.UseEffect(func() func() {
		setDocumentTitle(docTitle)
		if firstRender.Get() {
			firstRender.Set(false)
			return nil
		}
		focusMain()
		triggerPageEnter()
		return nil
	}, props.ActivePath)

	// The skip link must include the current path: a document <base href> is set so
	// deep-link refreshes resolve assets, but that also makes a bare "#main" resolve
	// against the base (navigating to the root). Anchoring it to the live path keeps
	// "skip to content" an in-page jump on every route.
	return Div(css.Class("cf-shell", tw.Flex, tw.HScreen, tw.OverflowHidden, tw.BgBase, tw.TextFg, tw.FontSans),
		A(css.Class("skip-link"), Attr("href", uistate.RoutePath(props.ActivePath)+"#main"), uistate.T("a11y.skipToContent")),
		uic.CreateElement(Sidebar, sidebarProps{ActivePath: props.ActivePath}),
		Main(css.Class("cf-scroll", tw.Flex1, tw.MinW0, tw.OverflowYAuto), Attr("id", "main"), Attr("tabindex", "-1"),
			uic.CreateElement(TopBar, topBarProps{Title: props.Title, ActivePath: props.ActivePath}),
			uic.CreateElement(SampleDataBanner),
			uic.CreateElement(SubscriptionBanner),
			// C281: "Viewing as <member>" scope banner — shown whenever the top-bar
			// member switcher has a member selected. Renders nothing for the default
			// everyone/"" view. Placed after the other global status banners so the
			// stacking order reads: sample → subscription → member scope.
			uic.CreateElement(ScopeBanner),
			// Each screen renders as its OWN component (CreateElement → its own fiber,
			// so its hooks never share the Shell's), keyed by the active route path.
			// The key is what makes navigating BETWEEN two pages of the same component
			// type work: every "/p/:slug" View closure is created at one source line, so
			// they share a function code-pointer and the reconciler would treat them as
			// the same element and skip the re-render (custom→custom showed the previous
			// page's body). A per-path key gives each route a distinct identity, so the
			// reconciler unmounts the old page and mounts the new one on every navigation
			// (regression covered by e2e/loopstory_90_custompage_nav.mjs).
			Div(css.Class(tw.P10px), Attr("id", "cf-page-view"),
				// Intersperse the SMART layer: a glanceable, opt-in insight strip
				// above each relevant page's content (additive — nothing renders
				// until the user enables features that produce insights here).
				screens.SmartStripForPath(props.ActivePath),
				WithKey(uic.CreateElement(props.View), props.ActivePath)),
		),
		// Mobile bottom tab bar (L11): shown only at phone widths (CSS agent controls
		// the breakpoint). The desktop left rail is unchanged — this is additive.
		uic.CreateElement(MobileTabBar, mobileTabBarProps{ActivePath: props.ActivePath}),
		uic.CreateElement(SettingsHost),
		uic.CreateElement(QuickAddHost),
		uic.CreateElement(AddHost),
		uic.CreateElement(TxnEditHost),
		uic.CreateElement(AccountEditHost),
		uic.CreateElement(DialogHost),
		// C274: profile-switch modal — "Who's using CashFlux?" device user-switching.
		uic.CreateElement(ProfileSwitchHost),
		// C309 (#464): sync-conflict resolve modal — "Keep my changes" (force-push)
		// or "Use server version" (pull + discard stash). Opened by the amber chip.
		uic.CreateElement(SyncConflictHost),
		uic.CreateElement(UpgradeSheet),
		uic.CreateElement(Toast),
		// Headless SMART proactive digest driver: fires on cadence when opted in,
		// posting a brief insight summary to the notification feed. Mounted once
		// here (not in a loop) so its hook depth is always constant.
		uic.CreateElement(screens.SmartDigestDriver),
	)
}

// railItem is one primary navigation entry: an i18n label key, route, and icon.
type railItem struct {
	Key      string // i18n key, resolved via uistate.T at render
	Path     string
	Icon     icon.Name
	SubGroup string // Tools sub-section (C67); "" for Primary/System
}

// railMeta maps a route path to its rail presentation: the i18n label key and the
// icon. This is the design layer — kept out of the screens registry, which stays
// presentation-free — while the registry's Group field decides *membership*. A
// route with no entry here still appears (B7), falling back to its registry label
// and a neutral icon rather than being dropped.
var railMeta = map[string]struct {
	Key  string
	Icon icon.Name
}{
	"/":               {"nav.dashboard", icon.Dashboard},
	"/accounts":       {"nav.accounts", icon.Accounts},
	"/transactions":   {"nav.transactions", icon.Transactions},
	"/budgets":        {"nav.budgets", icon.Budgets},
	"/goals":          {"nav.goals", icon.Goals},
	"/todo":           {"nav.todo", icon.Todo},
	"/planning":       {"nav.planning", icon.Planning},
	"/debt":           {"nav.debt", icon.Planning},
	"/allocate":       {"nav.allocate", icon.Allocate},
	"/reports":        {"nav.reports", icon.Reports},
	"/networth":       {"nav.netWorth", icon.TrendingUp},
	"/recurring":      {"nav.recurring", icon.Bills},
	"/subscriptions":  {"nav.subscriptions", icon.Subscriptions},
	"/bills":          {"nav.bills", icon.Bills},
	"/split":          {"nav.split", icon.Split},
	"/insights":       {"nav.insights", icon.Insights},
	"/documents":      {"nav.documents", icon.Page},
	"/customize":      {"nav.customize", icon.Customize},
	"/fields":         {"nav.fields", icon.Tag},
	"/artifacts":      {"nav.artifacts", icon.Page},
	"/workflows":      {"nav.workflows", icon.Customize},
	"/widget-builder": {"nav.widgetBuilder", icon.PlusCircle},
	"/widget-manager": {"nav.widgetManager", icon.Dashboard},
	"/members":        {"nav.members", icon.Users},
	"/categories":     {"nav.categories", icon.Tag},
	"/rules":          {"nav.rules", icon.Tag},
	"/notifications":  {"nav.notifications", icon.Bell},
	"/appearance":     {"nav.appearance", icon.Appearance},
	"/about":          {"nav.about", icon.HelpCircle},
	"/admin":          {"nav.admin", icon.Settings},
	// IA-remap §5.6: three new hub routes on the Tools rail.
	"/assistant": {"nav.assistant", icon.Sparkles},
	"/household": {"nav.household", icon.Users},
	"/studio":    {"nav.studio", icon.Customize},
}

// navGroup builds the rail items for one screen group, in registry order. The
// screens registry (Route.Group) is the single source of truth for membership, so
// a newly registered screen can't be silently dropped from the rail (B7); if its
// path isn't in railMeta it still shows, with its registry label and a default icon.
// Routes with AdminOnly=true are excluded when the admin atom is false (non-admins
// never see the entry; the route is still registered so a direct URL load works).
func navGroup(group string) []railItem {
	adminAvailable := uistate.UseAdminConsoleAvailable()
	uistate.CaptureAdminConsole(adminAvailable)
	isAdmin := adminAvailable.Get()

	var items []railItem
	for _, r := range screens.All() {
		if r.Group != group {
			continue
		}
		if r.AdminOnly && !isAdmin {
			continue
		}
		if meta, ok := railMeta[r.Path]; ok {
			items = append(items, railItem{Key: meta.Key, Path: r.Path, Icon: meta.Icon, SubGroup: r.SubGroup})
		} else {
			items = append(items, railItem{Key: r.Label, Path: r.Path, Icon: icon.Page, SubGroup: r.SubGroup})
		}
	}
	return items
}

// primaryNav is the candidate-C rail's main navigation group.
func primaryNav() []railItem { return navGroup(screens.GroupPrimary) }

// primaryNavStatic enumerates the primary-group rail items WITHOUT calling any
// framework hook, so it is safe to call outside a component render (e.g. from
// wireKeyboardShortcuts at boot). navGroup calls UseAdminConsoleAvailable (a hook)
// to gate AdminOnly routes; invoking it at boot panics with "GoUseAtom called
// outside component context" and the whole app fails to start. Keyboard digit-nav
// only needs the ordered primary screen set, and AdminOnly routes never live in the
// primary group — so the hook-gated filter is irrelevant here. AdminOnly routes are
// excluded defensively to keep the digit order matching the visible (non-admin) rail.
func primaryNavStatic() []railItem {
	var items []railItem
	for _, r := range screens.All() {
		if r.Group != screens.GroupPrimary || r.AdminOnly {
			continue
		}
		if meta, ok := railMeta[r.Path]; ok {
			items = append(items, railItem{Key: meta.Key, Path: r.Path, Icon: meta.Icon, SubGroup: r.SubGroup})
		} else {
			items = append(items, railItem{Key: r.Label, Path: r.Path, Icon: icon.Page, SubGroup: r.SubGroup})
		}
	}
	return items
}

// toolsNav is the Phase-2 "Tools" group: the routed power-tool screens that were
// otherwise only reachable by URL.
func toolsNav() []railItem { return navGroup(screens.GroupTools) }

// systemNav is the "System" group: the household-configuration screens.
func systemNav() []railItem { return navGroup(screens.GroupSystem) }

// toolSubGroupLabel resolves a Tools sub-group id to its display label.
func toolSubGroupLabel(sg string) string {
	switch sg {
	case screens.SubGroupPlan:
		return uistate.T("nav.toolsPlan")
	case screens.SubGroupUnderstand:
		return uistate.T("nav.toolsUnderstand")
	case screens.SubGroupBuild:
		return uistate.T("rail.subBuild") // "Build" — key already defined in en.go
	case screens.SubGroupData:
		return uistate.T("nav.toolsData")
	case screens.SubGroupBills:
		return uistate.T("rail.subBills") // retained; no rail routes currently use it
	}
	return sg
}

type toolGroupHeaderProps struct {
	Label     string
	Collapsed bool
	OnToggle  func()
}

// toolGroupHeader is a collapsible Tools sub-section header: a small label with a
// chevron that toggles its section. Its own component so the click hook stays at a
// stable position across the sub-group list (C67).
func toolGroupHeader(props toolGroupHeaderProps) uic.Node {
	chev := icon.ChevronDown
	if props.Collapsed {
		chev = icon.ChevronRight
	}
	return Button(css.Class("rail-subhead rail-section", tw.Flex, tw.ItemsCenter, tw.Gap15, tw.WFull, tw.Px3, tw.Pt3, tw.Pb1, tw.Text11, tw.Uppercase, tw.Tracking008, tw.TextFaint, tw.HoverTextFg),
		Type("button"), Attr("aria-expanded", fmt.Sprintf("%v", !props.Collapsed)),
		OnClick(func() {
			if props.OnToggle != nil {
				props.OnToggle()
			}
		}),
		ui.Icon(chev, css.Class(tw.W3, tw.H3)),
		Span(props.Label),
	)
}

// railHeader renders a small uppercase section label inside the rail. The
// rail-section class lets the collapsed/mobile rules hide just these labels
// (not the nav items, which the framework also wraps in a <div>) — see C15.
func railHeader(label string) uic.Node {
	return Div(css.Class("rail-section", tw.Px3, tw.Pt4, tw.Pb1, tw.Text11, tw.Uppercase, tw.Tracking008, tw.TextFaint), label)
}

// Sidebar renders the left rail: brand header, primary navigation, the user's
// custom "My pages", the System group, and a household card that opens settings.
func Sidebar(props sidebarProps) uic.Node {
	current := props.ActivePath
	hidden := uistate.UseHiddenModules().Get()
	cls := "rail " + tw.Fold(tw.W60, tw.ShrinkO, tw.BorderR, tw.BorderLine, tw.Flex, tw.FlexCol)
	railCollapsed := uistate.UseRailCollapsed().Get()
	if railCollapsed {
		cls += " collapsed"
	}
	// Play the rail-toggle settle animation whenever the collapsed state changes — from
	// any toggle source (the panel chevron, the top-bar menu button, or a shortcut), since
	// they all flip this atom and re-render the Sidebar. Skipped on first render so it
	// doesn't fire on initial load.
	railAnimFirst := uic.UseRef(true)
	railAnimKey := "0"
	if railCollapsed {
		railAnimKey = "1"
	}
	uic.UseEffect(func() func() {
		if railAnimFirst.Get() {
			railAnimFirst.Set(false)
			return nil
		}
		triggerRailAnim()
		return nil
	}, railAnimKey)

	// Hide screens the user has switched off (locked screens stay visible).
	var visibleNav []railItem
	for _, it := range primaryNav() {
		if !hidden.IsHidden(it.Path) {
			visibleNav = append(visibleNav, it)
		}
	}
	// Apply the user's custom primary-nav order (B8): drag-reorder persists a path
	// sequence; navorder.Apply layers it over the live, hidden-filtered list.
	navOrder := uistate.UseNavOrder()
	dragSrc := uistate.UseNavDragSource()
	currentPaths := make([]string, len(visibleNav))
	for i, it := range visibleNav {
		currentPaths[i] = it.Path
	}
	orderedPaths := navorder.Apply(navOrder.Get(), currentPaths)
	byPath := make(map[string]railItem, len(visibleNav))
	for _, it := range visibleNav {
		byPath[it.Path] = it
	}
	ordered := make([]railItem, 0, len(visibleNav))
	for _, p := range orderedPaths {
		if it, ok := byPath[p]; ok {
			ordered = append(ordered, it)
		}
	}
	visibleNav = ordered
	// reorderNav moves the dragged item in front of the drop target, then persists.
	reorderNav := func(targetPath string) {
		src := dragSrc.Get()
		dragSrc.Set("")
		if src == "" || src == targetPath {
			return
		}
		ti := 0
		for i, p := range orderedPaths {
			if p == targetPath {
				ti = i
				break
			}
		}
		next := navorder.Move(orderedPaths, src, ti)
		navOrder.Set(next)
		uistate.PersistNavOrder(next)
	}
	var visibleTools []railItem
	for _, it := range toolsNav() {
		if !hidden.IsHidden(it.Path) {
			visibleTools = append(visibleTools, it)
		}
	}
	// Tools sub-sections (C67): group the Tools items by SubGroup into collapsible
	// accordion sections, in the registry's display order.
	collapsedGroups := uistate.UseCollapsedToolGroups()
	collapsed := collapsedGroups.Get()
	setCollapsed := func(sg string, val bool) {
		next := map[string]bool{}
		for k, v := range collapsed {
			next[k] = v
		}
		next[sg] = val
		collapsedGroups.Set(next)
		uistate.PersistCollapsedToolGroups(next)
	}
	toolsByGroup := map[string][]railItem{}
	for _, it := range visibleTools {
		toolsByGroup[it.SubGroup] = append(toolsByGroup[it.SubGroup], it)
	}
	var toolNodes []any
	if len(visibleTools) > 0 {
		toolNodes = append(toolNodes, railHeader(uistate.T("rail.tools")))
		for _, sg := range screens.ToolsSubGroups {
			sg := sg
			items := toolsByGroup[sg]
			if len(items) == 0 {
				continue
			}
			isCollapsed := collapsed[sg]
			toolNodes = append(toolNodes, uic.CreateElement(toolGroupHeader, toolGroupHeaderProps{
				Label:     toolSubGroupLabel(sg),
				Collapsed: isCollapsed,
				OnToggle:  func() { setCollapsed(sg, !isCollapsed) },
			}))
			if !isCollapsed {
				toolNodes = append(toolNodes, MapKeyed(items,
					func(it railItem) any { return it.Path },
					func(it railItem) uic.Node {
						return uic.CreateElement(navItem, navItemProps{
							Label: uistate.T(it.Key), Path: it.Path, Icon: it.Icon, Active: current == it.Path,
						})
					},
				))
			}
		}
	}

	var visibleSystem []railItem
	for _, it := range systemNav() {
		if !hidden.IsHidden(it.Path) {
			visibleSystem = append(visibleSystem, it)
		}
	}
	// System is a collapsible section too (C67), keyed "system" in the same store.
	var systemNodes []any
	if len(visibleSystem) > 0 {
		sysCollapsed := collapsed["system"]
		systemNodes = append(systemNodes, uic.CreateElement(toolGroupHeader, toolGroupHeaderProps{
			Label: uistate.T("rail.system"), Collapsed: sysCollapsed,
			OnToggle: func() { setCollapsed("system", !sysCollapsed) },
		}))
		if !sysCollapsed {
			systemNodes = append(systemNodes, MapKeyed(visibleSystem,
				func(it railItem) any { return it.Path },
				func(it railItem) uic.Node {
					return uic.CreateElement(navItem, navItemProps{
						Label: uistate.T(it.Key), Path: it.Path, Icon: it.Icon, Active: current == it.Path,
					})
				},
			))
		}
	}
	return Aside(ClassStr(cls),
		Div(css.Class("railhead", tw.H14, tw.Flex, tw.ItemsCenter, tw.Gap25, tw.Px5, tw.BorderB, tw.BorderLine),
			// Brand mark: accent-green square with a "C". (Was tw.BgFg + tw.TextBase — but TextBase
			// is a font-SIZE token, not a color, so the "C" inherited white --text on the white BgFg
			// square = 1.00:1, invisible. Accent fill + TextFg fixes it.)
			Span(css.Class(tw.ShrinkO, tw.Grid, tw.PlaceItemsCenter, tw.W7, tw.H7, tw.Rounded, tw.BgAccent, tw.TextFg, tw.FontDisplay, tw.FontSemibold, tw.Text13), "C"),
			Span(css.Class("brand-name", tw.FontDisplay, tw.TextLg, tw.FontSemibold, tw.TrackingTight), uistate.T("app.name")),
		),
		uic.CreateElement(WorkspaceSwitcher),
		// Cloud-sync status chip by the workspace switcher (§7.11) — invisible until
		// Cloud sync is in use; shows synced/syncing/offline/conflict/error + queue.
		uic.CreateElement(SyncChip),
		Nav(css.Class(tw.Flex1, tw.OverflowYAuto, tw.P3, tw.Flex, tw.FlexCol, tw.Gap05, tw.TextDim, tw.Text135), Attr("aria-label", uistate.T("nav.primaryLabel")),
			MapKeyed(visibleNav,
				func(it railItem) any { return it.Path },
				func(it railItem) uic.Node {
					p := it.Path
					// Find the 1-based position of this item in the ordered primary nav
					// so the Alt+N hint (L34) matches what Alt+N actually does.
					hint := 0
					for idx, v := range visibleNav {
						if v.Path == it.Path && idx < 9 {
							hint = idx + 1
							break
						}
					}
					return uic.CreateElement(navItem, navItemProps{
						Label:       uistate.T(it.Key),
						Path:        it.Path,
						Icon:        it.Icon,
						Active:      current == it.Path,
						AltHint:     hint,
						Draggable:   true,
						OnDragStart: func() { dragSrc.Set(p) },
						OnDrop:      func() { reorderNav(p) },
					})
				},
			),
			Fragment(toolNodes...),
			Fragment(systemNodes...),
			// The user's custom pages ("My pages"): listing, create, and reorder.
			uic.CreateElement(CustomPagesNav),
		),
		// One-time, calm Cloud mention (§7.11) — self-hides once dismissed or syncing.
		uic.CreateElement(CloudMention),
		// The household card is the single Settings entry point (opens the global panel).
		uic.CreateElement(HouseholdCard),
	)
}

type navItemProps struct {
	Label     string
	Path      string // empty = non-navigating placeholder (e.g. example pages)
	Icon      icon.Name
	IconClass string // defaults to "w-4 h-4 shrink-0"
	Active    bool
	Muted     bool // faint styling for low-emphasis actions ("New page")
	// AltHint is the digit shown at the trailing edge of the item to advertise the
	// Alt+<digit> jump shortcut (L34). 0 means no hint. Only the first nine primary
	// nav items receive a hint; the value is the 1-based position (1–9).
	AltHint int
	// Drag-reorder (B8): when Draggable, the item can be dragged onto another to
	// reorder the primary nav. OnDragStart marks this item as the drag source;
	// OnDrop fires when another item is dropped onto this one.
	Draggable   bool
	OnDragStart func()
	OnDrop      func()
}

// navItem is its own component so its click-handler hook stays stable regardless
// of how the nav list changes (the On*-hooks-in-loops rule).
func navItem(props navItemProps) uic.Node {
	nav := router.UseNavigate()
	base := tw.Fold(tw.Flex, tw.MinW10, tw.MinH10, tw.ItemsCenter, tw.Gap25, tw.Px3, tw.Py2, tw.Rounded4, tw.CursorPointer)
	cls := "nav nv " + base
	switch {
	case props.Active:
		cls = "nv active " + base + " " + tw.Fold(tw.BgHex1c, tw.TextFg, tw.FontMedium)
	case props.Muted:
		cls = "nav nv " + base + " " + tw.Fold(tw.TextFaint)
	}
	iconClass := props.IconClass
	if iconClass == "" {
		iconClass = tw.Fold(tw.W4, tw.H4, tw.ShrinkO)
	}
	path := props.Path
	args := []any{
		ClassStr(cls),
		Title(props.Label),              // native tooltip
		Attr("aria-label", props.Label), // C315: explicit accessible name (title alone is unreliable for SR)
		// A real href makes nav items keyboard-focusable links that screen readers
		// announce and that support middle-click / open-in-new-tab (L34/L19 a11y);
		// the click handler prevents the full-page load and does SPA navigation.
		OnClick(Prevent(func() {
			if path != "" {
				nav.Navigate(uistate.RoutePath(path))
			}
		})),
	}
	if path != "" {
		args = append(args, Attr("href", uistate.RoutePath(path)))
	}
	if props.Active {
		args = append(args, Attr("aria-current", "page"))
	}
	if props.Draggable {
		onStart, onDrop := props.OnDragStart, props.OnDrop
		args = append(args,
			Attr("draggable", "true"),
			OnDragStart(func() {
				if onStart != nil {
					onStart()
				}
			}),
			OnDragOver(Prevent(func() {})), // allow drop
			OnDrop(Prevent(func() {
				if onDrop != nil {
					onDrop()
				}
			})),
		)
	}
	args = append(args, ui.Icon(props.Icon, ClassStr(iconClass)), Span(css.Class(tw.Flex1), props.Label))
	// Alt+N digit badge (L34): shown in the expanded rail so users discover the
	// shortcut without opening the help overlay. Hidden via CSS when collapsed (the
	// badge class is omitted on the icon-only rail to avoid clutter). The kbd tag
	// is purely decorative (aria-hidden) because the Title tooltip already names
	// the shortcut. Only positions 1–9 are labeled; beyond that there's no shortcut.
	if props.AltHint >= 1 && props.AltHint <= 9 {
		args = append(args, Span(css.Class("nav-alt-hint"),
			Attr("aria-hidden", "true"),
			Attr("title", fmt.Sprintf("Alt + %d", props.AltHint)),
			Text(strconv.Itoa(props.AltHint)),
		))
	}
	return A(args...)
}

// HouseholdCard sits at the bottom of the rail, summarizing the household and
// opening the global settings flip panel on click. It also renders the
// on-panel rail-collapse toggle (C20) — a small chevron button anchored at
// the top-right of the footer area so users can collapse/expand the rail from
// within the panel rather than relying solely on the top-bar menu button.
func HouseholdCard() uic.Node {
	settings := uistate.UseSettings()
	collapsed := uistate.UseRailCollapsed()
	isCollapsed := collapsed.Get()
	name := uistate.T("household.title")
	summary := uistate.T("household.settings")
	if app := appstate.Default; app != nil {
		base := app.Settings().BaseCurrency
		if base == "" {
			base = "USD"
		}
		members := len(app.Members())
		noun := "members"
		if members == 1 {
			noun = "member"
		}
		summary = fmt.Sprintf("%d %s · %s base", members, noun, base)
	}
	collapseIcon := icon.ChevronLeft
	collapseTitle := uistate.T("rail.collapse")
	if isCollapsed {
		collapseIcon = icon.ChevronRight
		collapseTitle = uistate.T("rail.expand")
	}
	// The household card plus a small muted version line anchored at the rail foot
	// (mt-auto on the wrapper). One source of truth: internal/version (C80).
	// The horizontal inset lives on this wrapper's padding (not the button's margin):
	// a <button> is fit-content by default so it needs w-full to span the rail, and
	// w-full + horizontal margins would overflow (the margins add onto 100%).
	return Div(css.Class("rail-foot", tw.MtAuto, tw.Px3),
		// On-panel collapse toggle (C20): sits above the household card, right-aligned.
		// Using its own component (HouseholdCard) keeps this OnClick at a stable render
		// position — the On*-hooks-in-loops rule is satisfied because this is called via
		// uic.CreateElement, not inside a variable-length loop.
		Div(css.Class("rail-collapse-row", tw.Flex, tw.JustifyBetween, tw.ItemsCenter, tw.Pt2),
			Span(Attr("aria-hidden", "true")), // spacer so the button floats right (C315: decorative)
			Button(css.Class("rail-collapse-btn", tw.W7, tw.H7, tw.Flex, tw.ItemsCenter, tw.JustifyCenter, tw.Rounded4, tw.TextFaint, tw.HoverTextFg, tw.HoverBgHover),
				Type("button"),
				Title(collapseTitle),
				Attr("aria-label", collapseTitle),
				Attr("data-testid", "rail-collapse-btn"),
				OnClick(func() {
					next := !collapsed.Get()
					collapsed.Set(next)
					uistate.PersistRailCollapsed(next)
				}),
				ui.Icon(collapseIcon, css.Class(tw.W4, tw.H4)),
			),
		),
		Button(
			css.Class("hh", tw.Mt3, tw.Mb3, tw.P3, tw.Rounded4, tw.Border, tw.BorderLine, tw.Flex, tw.ItemsCenter, tw.Gap25, tw.TextLeft, tw.HoverBgHover, tw.WFull),
			// Tooltip/accessible name — keeps the "Settings" affordance (the gear icon
			// signals it visually) without repeating it in the visible summary line.
			Title(name+" · "+summary+" · "+uistate.T("household.settings")),
			Attr("aria-label", name+" · "+summary+" · "+uistate.T("household.settings")), // C315: explicit SR name
			OnClick(func() { settings.Set(uistate.Global()) }),
			ui.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)),
			Span(css.Class("hh-text", tw.LeadingTight),
				Span(css.Class(tw.FontDisplay, tw.Text14, tw.FontMedium, tw.Block), name),
				Span(css.Class(tw.TextXs, tw.TextFaint, tw.Block), summary),
			),
		),
		// Local-first trust line (C289 / R34-trust): the privacy differentiator was
		// only in the admin console; surface it where every user sees it. Muted so it
		// reassures without shouting.
		Span(css.Class(tw.TextDim, tw.Text11, tw.Block, tw.TextCenter, tw.Px3, tw.Pb1, tw.LeadingTight),
			uistate.T("trust.localFooter")),
		// C290: "About & privacy" footer link so users can reach the /about page
		// from the sidebar — a discoverable, persistent entry point that doesn't
		// depend on knowing the ? shortcut or the nav list order.
		A(css.Class(tw.TextFaint, tw.Text11, tw.Block, tw.TextCenter, tw.Pb1, tw.HoverTextFg, tw.Underline),
			Attr("href", uistate.RoutePath("/about")), uistate.T("nav.aboutPrivacyLink")),
		Span(css.Class("app-version", tw.TextFaint, tw.Text11, tw.Block, tw.TextCenter, tw.Pb2),
			Attr("title", "CashFlux "+version.Label()), version.Label()),
	)
}

type topBarProps struct {
	Title string
	// ActivePath is the logical route path, threaded from the route so the
	// breadcrumb "are we home" and period-aware checks react to navigation rather
	// than reading a frozen router snapshot.
	ActivePath string
}

// TopBar is the sticky page header inside the scrolling main pane: a (currently
// static) menu toggle, the page title, and an Add action.
func TopBar(props topBarProps) uic.Node {
	collapsed := uistate.UseRailCollapsed()
	nav := router.UseNavigate()
	// Breadcrumb: Dashboard (clickable) › current screen. Off the dashboard the
	// home crumb navigates back; on it, just the title shows.
	onHome := func() { nav.Navigate(uistate.RoutePath("/")) }
	curPath := props.ActivePath
	onDashboard := curPath == "/"
	// The time-resolution control only makes sense where there's a period concept;
	// on Members/Categories/Rules/etc. it does nothing, so hide it there (C4).
	periodAware := map[string]bool{
		"/": true, "/transactions": true, "/budgets": true, "/planning": true, "/insights": true, "/reports": true,
	}[curPath]
	// The bar is built from four zones — menu, title, a scope+period "context"
	// group, and the primary actions. On wide screens they sit on one flex row; below
	// 1536px the CSS switches to a two-row grid (title + actions on top, the context
	// group as a dedicated full-width bar beneath) so controls never wrap raggedly.
	return Div(css.Class("topbar", tw.BorderB, tw.BorderLine, tw.Flex, tw.ItemsCenter, tw.Px6, tw.Gap3, tw.Sticky, tw.Top0, tw.BgBase, tw.Z20),
		Button(css.Class("menu-btn tb-menu", tw.W7, tw.H7, tw.MlN1), Attr("title", uistate.T("topbar.menu")),
			// C315: icon-only button needs an accessible name (title alone isn't reliably
			// exposed as the AX name to screen readers).
			Attr("aria-label", uistate.T("topbar.menu")),
			OnClick(func() {
				next := !collapsed.Get()
				collapsed.Set(next)
				uistate.PersistRailCollapsed(next) // remember the choice across reloads (C20)
			}),
			ui.Icon(icon.Menu, css.Class(tw.W5, tw.H5)),
		),
		Nav(css.Class("breadcrumb tb-title", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FontDisplay, tw.MinW0), Attr("aria-label", uistate.T("topbar.breadcrumb")),
			If(!onDashboard, Button(css.Class(tw.TextDim, tw.HoverTextFg, tw.Text15), Type("button"), Attr("title", uistate.T("nav.dashboard")), Attr("aria-label", uistate.T("nav.dashboard")), OnClick(onHome), uistate.T("nav.dashboard"))), // C315
			If(!onDashboard, Span(css.Class(tw.TextFaint), "›")),
			// The current page's title is the screen's single <h1> — so every screen
			// has exactly one top-level heading for screen-reader heading navigation.
			H1(css.Class(tw.TextLg, tw.FontSemibold, tw.Truncate), Attr("aria-current", "page"), props.Title),
		),
		// Context zone: the view's scope (member) and period. On narrow screens this
		// becomes its own full-width row beneath the title, scrolling horizontally if
		// the date picker is wider than the viewport (rather than wrapping).
		Div(css.Class("tb-context", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.MinW0, tw.TextDim, tw.Text13),
			uic.CreateElement(OfflineIndicator),
			uic.CreateElement(MemberSwitcher),
			If(periodAware, uic.CreateElement(ResolutionControl)),
		),
		// Actions zone: stays on the title row at every size.
		Div(css.Class("tb-actions", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.TextDim, tw.Text13),
			// Secondary, low-frequency app toggles. Inline on the widest screens; folded
			// into the "More" menu below 1580px. They stay mounted when hidden so stateful
			// ones (e.g. MuzakToggle's player effect) keep running.
			Span(css.Class("topbar-secondary", tw.Flex, tw.ItemsCenter, tw.Gap25),
				If(onDashboard, uic.CreateElement(DashCustomizeButton)),
				uic.CreateElement(ThemeToggle),
				uic.CreateElement(HelpButton),
			),
			// Music on/off is a direct, always-visible top-bar action (not folded into
			// the More menu) so it's one click from anywhere.
			uic.CreateElement(MuzakToggle),
			uic.CreateElement(NotifyBell),
			uic.CreateElement(AddMenu),
			// The "⋯ More" overflow menu sits last, against the right edge.
			uic.CreateElement(MoreMenu, moreMenuProps{OnDashboard: onDashboard}),
		),
	)
}

// ThemeToggle (C317) is a top-bar button that cycles the color theme
// Dark → Light → System without opening Settings — the theme system existed
// (prefs.Theme + /appearance) but had no discoverable chrome affordance. It uses
// the exact persist+apply path the /appearance Segmented uses (ApplyPrefs +
// PersistPrefs + ApplyTheme(LoadTheme())) so inline CSS vars track the mode.
func ThemeToggle() uic.Node {
	pAtom := uistate.UsePrefs()
	p := pAtom.Get()
	next := prefs.ThemeLight
	switch p.Theme {
	case prefs.ThemeDark:
		next = prefs.ThemeLight
	case prefs.ThemeLight:
		next = prefs.ThemeSystem
	default:
		next = prefs.ThemeDark
	}
	cycle := uic.UseEvent(func() {
		np := pAtom.Get()
		np.Theme = next
		uistate.ApplyPrefs(np)
		uistate.PersistPrefs(np)
		pAtom.Set(np)
		uistate.ApplyTheme(uistate.LoadTheme())
	})
	label := uistate.T("topbar.themeToggle", uistate.T("settings.theme"+themeWord(p.Theme)), uistate.T("settings.theme"+themeWord(next)))
	return Button(css.Class("icon-btn", tw.W7, tw.H7, tw.TextDim, tw.HoverTextFg),
		Type("button"), Attr("title", label), Attr("aria-label", label),
		Attr("data-testid", "theme-toggle"), Attr("data-theme-current", string(p.Theme)),
		OnClick(cycle),
		ui.Icon(icon.Appearance, css.Class(tw.W5, tw.H5)),
	)
}

// themeWord maps a Theme to the i18n key suffix used by settings.theme{Dark,Light,System}.
func themeWord(t prefs.Theme) string {
	switch t {
	case prefs.ThemeLight:
		return "Light"
	case prefs.ThemeSystem:
		return "System"
	default:
		return "Dark"
	}
}

// HelpButton is the top-bar "?" that opens the help center (C327/C328): help was
// previously only reachable via the keyboard `?` overlay or the nav list, with no
// visible affordance. Routes to /help (topics, what's-new, setup checklist, and the
// bug-report path), keeping support one obvious click away on every screen.
func HelpButton() uic.Node {
	nav := router.UseNavigate()
	open := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/help")) })
	return Button(css.Class("icon-btn", tw.W7, tw.H7, tw.TextDim, tw.HoverTextFg),
		Type("button"), Attr("title", uistate.T("nav.help")), Attr("aria-label", uistate.T("nav.help")),
		Attr("data-testid", "help-button"), OnClick(open),
		ui.Icon(icon.HelpCircle, css.Class(tw.W5, tw.H5)),
	)
}

// DashCustomizeButton is the top-bar "Customize" icon (dashboard only): a quiet,
// standardized entry point to the widget manager (layout mode, show/hide, sizes,
// tile styles), grouped with the other page-level top-bar actions instead of a
// floating bar above the bento. Icon-only to stay out of the way.
func DashCustomizeButton() uic.Node {
	nav := router.UseNavigate()
	open := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/widget-manager")) })
	return Button(css.Class("icon-btn", tw.W7, tw.H7, tw.TextDim, tw.HoverTextFg),
		Type("button"),
		Attr("title", uistate.T("dashboard.customizeAria")), Attr("aria-label", uistate.T("dashboard.customizeAria")),
		Attr("data-testid", "dash-customize"), OnClick(open),
		ui.Icon(icon.Customize, css.Class(tw.W5, tw.H5)),
	)
}

// OfflineIndicator shows a calm "Offline · saved on this device" pill in the top
// bar when the browser loses connectivity — reassuring the user their data is safe
// locally (CashFlux is local-first). When online it renders nothing. It reads the
// shared online atom, which the boot wiring keeps in sync with navigator.onLine and
// the window online/offline events.
func OfflineIndicator() uic.Node {
	online := uistate.UseOnline()
	uistate.CaptureOnline(online)
	if online.Get() {
		return Fragment()
	}
	return Span(css.Class("offline-pill", tw.InlineFlex, tw.ItemsCenter, tw.Gap15, tw.Px2, tw.Py05, tw.Rounded4),
		Attr("role", "status"), Attr("aria-live", "polite"), Attr("data-testid", "offline-indicator"),
		Attr("title", uistate.T("offline.savedLocally")),
		Span(css.Class(tw.ColorClass("text-warn")), uistate.T("offline.label")),
	)
}

// NotifyBell is the top-bar bell that opens the Notification Center, with a count
// badge for unread items. The persisted feed drives the badge; clicking routes to
// /notifications (which marks everything read).
func NotifyBell() uic.Node {
	feed := uistate.UseNotifyFeed().Get()
	// C159: count unread over the VISIBLE feed (snoozed items are hidden in the
	// Notification Center), so the badge matches what the user actually sees when
	// they open it — previously a snoozed-but-unread item inflated the badge.
	unread := uistate.UnreadNotifyCount(uistate.VisibleFeed(feed, time.Now().Unix()))
	nav := router.UseNavigate()
	open := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/notifications")) })
	badge := Fragment()
	if unread > 0 {
		label := fmt.Sprintf("%d", unread)
		if unread > 9 {
			label = "9+"
		}
		badge = Span(css.Class("notify-badge"), label)
	}
	return Button(css.Class("notify-btn", tw.Relative), Type("button"),
		Attr("title", uistate.T("nav.notifications")), Attr("aria-label", uistate.T("nav.notifications")),
		OnClick(open),
		ui.Icon(icon.Bell, css.Class(tw.W18px, tw.H18px)),
		badge,
	)
}

// MuzakToggle is the top-bar ♪ button that turns the calming background music on
// or off. It drives the JS audio controller (web/muzak.js) from the persisted
// on/off atom: an effect keyed on the state (re)initializes the player and
// applies enabled/disabled, so the choice survives navigation and reloads.
func MuzakToggle() uic.Node {
	enabledAtom := uistate.UseMuzakEnabled()
	enabled := enabledAtom.Get()
	volume := uistate.UseMuzakVolume().Get()

	uic.UseEffect(func() func() {
		if m := js.Global().Get("cashfluxMuzak"); m.Truthy() {
			m.Call("init")
			m.Call("setVolume", volume)
			m.Call("setEnabled", enabled)
		}
		return nil
	}, fmt.Sprintf("muzak:%v:%.3f", enabled, volume))

	toggle := func() {
		next := !enabledAtom.Get()
		enabledAtom.Set(next)
		uistate.PersistMuzakEnabled(next)
		checkpointMusic() // mirror the on/off choice into the dataset
	}

	cls := "muzak-btn"
	titleKey := "muzak.turnOff"
	glyph := icon.Volume
	if !enabled {
		cls += " is-off"
		titleKey = "muzak.turnOn"
		glyph = icon.VolumeMute
	}
	return Button(ClassStr(cls), Type("button"),
		Attr("title", uistate.T(titleKey)),
		Attr("aria-label", uistate.T(titleKey)),
		Attr("aria-pressed", fmt.Sprintf("%v", enabled)),
		OnClick(toggle),
		ui.Icon(glyph, css.Class(tw.W18px, tw.H18px)),
	)
}

// ResolutionControl is the top bar's time-resolution control. The common case is
// a single period: a Week/Month/Quarter granularity toggle and one stepper that
// pages the whole window (‹ Jun 2026 ›). When the view has moved off the current
// period a "This period" reset appears; a "Custom range" toggle reveals the
// dual From/To steppers for advanced ranges. All date math lives in
// internal/period — every action just stores the next immutable Window.
func ResolutionControl() uic.Node {
	atom := uistate.UsePeriod()
	w := atom.Get()
	open := uic.UseState(false)
	rangeMode := uic.UseState(false)
	menuID := uic.UseId()
	closeMenu := func() { open.Set(false) }
	// Escape / outside-click dismissal, matching the +Add and More menus.
	ui.DismissPopover(open.Get(), menuID, closeMenu)

	// preset builds one quick-jump button in the popover. Called at fixed positions
	// (not a loop) so its OnClick hook stays at a stable position.
	preset := func(label, v string) uic.Node {
		return Button(css.Class("period-preset"), Type("button"), Attr("role", "menuitem"),
			OnClick(func() {
				now := time.Now()
				switch v {
				case "this":
					atom.Set(period.NewWindow(w.Res, now, w.WeekStart))
				case "last":
					atom.Set(period.Previous(w.Res, now, w.WeekStart))
				case "quarter":
					uistate.PersistResolution(period.Quarter)
					atom.Set(period.NewWindow(period.Quarter, now, w.WeekStart))
				case "ytd":
					uistate.PersistResolution(period.Month)
					atom.Set(period.YearToDate(now, w.WeekStart))
				case "lastyear":
					uistate.PersistResolution(period.Year)
					atom.Set(period.PriorYear(now, w.WeekStart))
				}
				closeMenu()
			}),
			label)
	}

	// The pill shows the current single period, or the from–to range.
	pillLabel := w.Label()
	if rangeMode.Get() {
		pillLabel = w.FromLabel() + " – " + w.ToLabel()
	}
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}

	// The dual From/To steppers only appear inside the popover in custom-range mode.
	rangeRow := Fragment()
	if rangeMode.Get() {
		rangeRow = Div(css.Class("period-rangerow", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.FlexWrap),
			ui.StepperPill(ui.StepperPillProps{Label: w.FromLabel(), OnPrev: func() { atom.Set(w.StepFrom(-1)) }, OnNext: func() { atom.Set(w.StepFrom(1)) }, PrevLabel: uistate.T("resolution.fromEarlier"), NextLabel: uistate.T("resolution.fromLater")}),
			Span(css.Class(tw.TextFaint), "–"),
			ui.StepperPill(ui.StepperPillProps{Label: w.ToLabel(), OnPrev: func() { atom.Set(w.StepTo(-1)) }, OnNext: func() { atom.Set(w.StepTo(1)) }, PrevLabel: uistate.T("resolution.toEarlier"), NextLabel: uistate.T("resolution.toLater")}),
		)
	}
	rangeLabel := uistate.T("resolution.customRange")
	if rangeMode.Get() {
		rangeLabel = uistate.T("resolution.singlePeriod")
	}

	// A single compact control: ‹ [period ⌄] › — the chevrons page the window; the
	// center pill opens a popover with the granularity, quick jumps and custom range.
	return Div(css.Class("period-control add-wrap"), Attr("id", menuID),
		Button(css.Class("period-step"), Type("button"), Attr("aria-label", uistate.T("resolution.prevPeriod")), Attr("title", uistate.T("resolution.prevPeriod")),
			OnClick(func() { atom.Set(w.Shift(-1)) }), ui.Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4))),
		Button(css.Class("period-pill"), Type("button"), Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
			Attr("data-testid", "period-pill"), Attr("title", uistate.T("resolution.jumpTo")),
			OnClick(func() { open.Set(!open.Get()) }),
			Span(css.Class("period-label"), pillLabel),
			ui.Icon(icon.ChevronDown, css.Class("period-caret", tw.W3, tw.H3)),
		),
		Button(css.Class("period-step"), Type("button"), Attr("aria-label", uistate.T("resolution.nextPeriod")), Attr("title", uistate.T("resolution.nextPeriod")),
			OnClick(func() { atom.Set(w.Shift(1)) }), ui.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))),
		Div(ClassStr("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(ClassStr("period-pop add-menu open-left"+hidden), Attr("role", "menu"),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("resolution.granularity"), // C318: name the radiogroup
				Options: []ui.SegOption{
					{Value: string(period.Week), Label: "Week"},
					{Value: string(period.Month), Label: "Month"},
					{Value: string(period.Quarter), Label: "Quarter"},
					{Value: string(period.Year), Label: "Year"},
				},
				Selected: string(w.Res),
				OnSelect: func(v string) {
					r := period.Resolution(v)
					uistate.PersistResolution(r)
					atom.Set(w.SetResolution(r, time.Now()))
				},
			}),
			Div(css.Class("period-presets", tw.Flex, tw.FlexWrap, tw.Gap15),
				preset(uistate.T("resolution.presetThis"), "this"),
				preset(uistate.T("resolution.presetLast"), "last"),
				preset(uistate.T("resolution.presetQuarter"), "quarter"),
				preset(uistate.T("resolution.presetYTD"), "ytd"),
				preset(uistate.T("resolution.presetPriorYear"), "lastyear"),
			),
			rangeRow,
			Button(css.Class("period-rangetoggle"), Type("button"),
				OnClick(func() {
					if rangeMode.Get() {
						atom.Set(w.Single())
						rangeMode.Set(false)
					} else {
						rangeMode.Set(true)
					}
				}),
				rangeLabel),
		),
	)
}
