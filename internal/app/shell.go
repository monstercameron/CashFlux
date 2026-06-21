//go:build js && wasm

package app

import (
	"fmt"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/navorder"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/version"
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
	docTitle := props.Title + " · " + uistate.T("app.name")
	uic.UseEffect(func() func() {
		setDocumentTitle(docTitle)
		if firstRender.Get() {
			firstRender.Set(false)
			return nil
		}
		focusMain()
		return nil
	}, props.ActivePath)

	// The skip link must include the current path: a document <base href> is set so
	// deep-link refreshes resolve assets, but that also makes a bare "#main" resolve
	// against the base (navigating to the root). Anchoring it to the live path keeps
	// "skip to content" an in-page jump on every route.
	return Div(Class("flex h-screen overflow-hidden bg-base text-fg font-sans"),
		A(Class("skip-link"), Attr("href", uistate.RoutePath(props.ActivePath)+"#main"), uistate.T("a11y.skipToContent")),
		uic.CreateElement(Sidebar, sidebarProps{ActivePath: props.ActivePath}),
		Main(Class("cf-scroll flex-1 min-w-0 overflow-y-auto"), Attr("id", "main"), Attr("tabindex", "-1"),
			uic.CreateElement(TopBar, topBarProps{Title: props.Title, ActivePath: props.ActivePath}),
			Div(Class("p-[10px]"), uic.CreateElement(props.View)),
		),
		uic.CreateElement(SettingsHost),
		uic.CreateElement(QuickAddHost),
		uic.CreateElement(DialogHost),
		uic.CreateElement(Toast),
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
	"/allocate":       {"nav.allocate", icon.Allocate},
	"/reports":        {"nav.reports", icon.Reports},
	"/subscriptions":  {"nav.subscriptions", icon.Subscriptions},
	"/bills":          {"nav.bills", icon.Bills},
	"/split":          {"nav.split", icon.Split},
	"/insights":       {"nav.insights", icon.Insights},
	"/documents":      {"nav.documents", icon.Page},
	"/customize":      {"nav.customize", icon.Customize},
	"/artifacts":      {"nav.artifacts", icon.Page},
	"/workflows":      {"nav.workflows", icon.Customize},
	"/widget-builder": {"nav.widgetBuilder", icon.PlusCircle},
	"/widget-manager": {"nav.widgetManager", icon.Dashboard},
	"/members":        {"nav.members", icon.Users},
	"/categories":     {"nav.categories", icon.Tag},
	"/rules":          {"nav.rules", icon.Tag},
	"/notifications":  {"nav.notifications", icon.Bell},
}

// navGroup builds the rail items for one screen group, in registry order. The
// screens registry (Route.Group) is the single source of truth for membership, so
// a newly registered screen can't be silently dropped from the rail (B7); if its
// path isn't in railMeta it still shows, with its registry label and a default icon.
func navGroup(group string) []railItem {
	var items []railItem
	for _, r := range screens.All() {
		if r.Group != group {
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

// toolsNav is the Phase-2 "Tools" group: the routed power-tool screens that were
// otherwise only reachable by URL.
func toolsNav() []railItem { return navGroup(screens.GroupTools) }

// systemNav is the "System" group: the household-configuration screens.
func systemNav() []railItem { return navGroup(screens.GroupSystem) }

// toolSubGroupLabel resolves a Tools sub-group id to its display label.
func toolSubGroupLabel(sg string) string {
	switch sg {
	case screens.SubGroupPlan:
		return uistate.T("rail.subPlan")
	case screens.SubGroupBills:
		return uistate.T("rail.subBills")
	case screens.SubGroupData:
		return uistate.T("rail.subData")
	case screens.SubGroupBuild:
		return uistate.T("rail.subBuild")
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
	return Button(Class("rail-subhead rail-section flex items-center gap-1.5 w-full px-3 pt-3 pb-1 text-[11px] uppercase tracking-[0.08em] text-faint hover:text-fg"),
		Type("button"), Attr("aria-expanded", fmt.Sprintf("%v", !props.Collapsed)),
		OnClick(func() {
			if props.OnToggle != nil {
				props.OnToggle()
			}
		}),
		ui.Icon(chev, Class("w-3 h-3")),
		Span(props.Label),
	)
}

// railHeader renders a small uppercase section label inside the rail. The
// rail-section class lets the collapsed/mobile rules hide just these labels
// (not the nav items, which the framework also wraps in a <div>) — see C15.
func railHeader(label string) uic.Node {
	return Div(Class("rail-section px-3 pt-4 pb-1 text-[11px] uppercase tracking-[0.08em] text-faint"), label)
}

// Sidebar renders the left rail: brand header, primary navigation, the user's
// custom "My pages", the System group, and a household card that opens settings.
func Sidebar(props sidebarProps) uic.Node {
	current := props.ActivePath
	hidden := uistate.UseHiddenModules().Get()
	cls := "rail w-60 shrink-0 border-r border-line flex flex-col"
	if uistate.UseRailCollapsed().Get() {
		cls += " collapsed"
	}

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
	return Aside(Class(cls),
		Div(Class("railhead h-14 flex items-center gap-2.5 px-5 border-b border-line"),
			Span(Class("grid place-items-center w-7 h-7 rounded bg-fg text-base font-display font-semibold text-[13px] shrink-0"), "C"),
			Span(Class("brand-name font-display text-lg font-semibold tracking-tight"), uistate.T("app.name")),
		),
		uic.CreateElement(WorkspaceSwitcher),
		Nav(Class("flex-1 overflow-y-auto p-3 flex flex-col gap-0.5 text-dim text-[13.5px]"), Attr("aria-label", uistate.T("nav.primaryLabel")),
			MapKeyed(visibleNav,
				func(it railItem) any { return it.Path },
				func(it railItem) uic.Node {
					p := it.Path
					return uic.CreateElement(navItem, navItemProps{
						Label:       uistate.T(it.Key),
						Path:        it.Path,
						Icon:        it.Icon,
						Active:      current == it.Path,
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
	cls := "nav nv flex min-w-10 min-h-10 items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer"
	switch {
	case props.Active:
		cls = "nv flex min-w-10 min-h-10 items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer bg-[#1c1c1e] text-fg font-medium"
	case props.Muted:
		cls = "nav nv flex min-w-10 min-h-10 items-center gap-2.5 px-3 py-2 rounded-[4px] cursor-pointer text-faint"
	}
	iconClass := props.IconClass
	if iconClass == "" {
		iconClass = "w-4 h-4 shrink-0"
	}
	path := props.Path
	args := []any{
		Class(cls),
		Title(props.Label), // native tooltip + accessible name, esp. when collapsed to icons
		OnClick(func() {
			if path != "" {
				nav.Navigate(uistate.RoutePath(path))
			}
		}),
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
	args = append(args, ui.Icon(props.Icon, Class(iconClass)), Span(props.Label))
	return A(args...)
}

// HouseholdCard sits at the bottom of the rail, summarizing the household and
// opening the global settings flip panel on click. It reads live member count
// and base currency from app state.
func HouseholdCard() uic.Node {
	settings := uistate.UseSettings()
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
	// The household card plus a small muted version line anchored at the rail foot
	// (mt-auto on the wrapper). One source of truth: internal/version (C80).
	return Div(Class("mt-auto"),
		Button(
			Class("hh m-3 p-3 rounded-[4px] border border-line flex items-center gap-2.5 text-left hover:bg-hover w-full"),
			// Tooltip/accessible name — keeps the "Settings" affordance (the gear icon
			// signals it visually) without repeating it in the visible summary line.
			Title(name+" · "+summary+" · "+uistate.T("household.settings")),
			OnClick(func() { settings.Set(uistate.Global()) }),
			ui.Icon(icon.Settings, Class("w-4 h-4 shrink-0 text-dim")),
			Span(Class("hh-text leading-tight"),
				Span(Class("font-display text-[14px] font-medium block"), name),
				Span(Class("text-xs text-faint block"), summary),
			),
		),
		Span(Class("app-version text-faint text-[11px] block text-center pb-2"),
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
		"/": true, "/transactions": true, "/budgets": true, "/planning": true, "/insights": true,
	}[curPath]
	return Div(Class("topbar border-b border-line flex flex-wrap items-center px-6 gap-3 sticky top-0 bg-base z-20"),
		Button(Class("menu-btn w-7 h-7 -ml-1"), Attr("title", uistate.T("topbar.menu")),
			OnClick(func() {
				next := !collapsed.Get()
				collapsed.Set(next)
				uistate.PersistRailCollapsed(next) // remember the choice across reloads (C20)
			}),
			ui.Icon(icon.Menu, Class("w-5 h-5")),
		),
		Nav(Class("flex items-center gap-2 font-display min-w-0"), Attr("aria-label", uistate.T("topbar.breadcrumb")),
			If(!onDashboard, Button(Class("text-dim hover:text-fg text-[15px]"), Type("button"), Attr("title", uistate.T("nav.dashboard")), OnClick(onHome), uistate.T("nav.dashboard"))),
			If(!onDashboard, Span(Class("text-faint"), "›")),
			// The current page's title is the screen's single <h1> — so every screen
			// has exactly one top-level heading for screen-reader heading navigation.
			H1(Class("text-lg font-semibold truncate"), Attr("aria-current", "page"), props.Title),
		),
		Div(Class("topbar-controls ml-auto flex items-center gap-2.5 text-dim text-[13px]"),
			If(periodAware, uic.CreateElement(ResolutionControl)),
			uic.CreateElement(NotifyBell),
			uic.CreateElement(MuzakToggle),
			uic.CreateElement(AddMenu),
		),
	)
}

// NotifyBell is the top-bar bell that opens the Notification Center, with a count
// badge for unread items. The persisted feed drives the badge; clicking routes to
// /notifications (which marks everything read).
func NotifyBell() uic.Node {
	feed := uistate.UseNotifyFeed().Get()
	unread := uistate.UnreadNotifyCount(feed)
	nav := router.UseNavigate()
	open := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/notifications")) })
	badge := Fragment()
	if unread > 0 {
		label := fmt.Sprintf("%d", unread)
		if unread > 9 {
			label = "9+"
		}
		badge = Span(Class("notify-badge"), label)
	}
	return Button(Class("muzak-btn relative"), Type("button"),
		Attr("title", uistate.T("nav.notifications")), Attr("aria-label", uistate.T("nav.notifications")),
		OnClick(open),
		ui.Icon(icon.Bell, Class("w-[18px] h-[18px]")),
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
	return Button(Class(cls), Type("button"),
		Attr("title", uistate.T(titleKey)),
		Attr("aria-label", uistate.T(titleKey)),
		Attr("aria-pressed", fmt.Sprintf("%v", enabled)),
		OnClick(toggle),
		ui.Icon(glyph, Class("w-[18px] h-[18px]")),
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
	rangeMode := uic.UseState(false)
	isCurrent := w.IsCurrent(time.Now())

	// Quick presets. The select always shows its placeholder (it's an action menu,
	// not a persistent selection), so choosing an option applies it and the
	// control snaps back. Quarter/YTD also change the resolution, so persist it.
	onPreset := uic.UseEvent(func(e uic.Event) {
		now := time.Now()
		switch e.GetValue() {
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
		}
	})

	var stepper uic.Node
	if rangeMode.Get() {
		stepper = Span(Class("flex items-center gap-2.5"),
			ui.StepperPill(ui.StepperPillProps{
				Label:     w.FromLabel(),
				OnPrev:    func() { atom.Set(w.StepFrom(-1)) },
				OnNext:    func() { atom.Set(w.StepFrom(1)) },
				PrevLabel: uistate.T("resolution.fromEarlier"),
				NextLabel: uistate.T("resolution.fromLater"),
			}),
			Span(Class("text-faint"), "–"),
			ui.StepperPill(ui.StepperPillProps{
				Label:     w.ToLabel(),
				OnPrev:    func() { atom.Set(w.StepTo(-1)) },
				OnNext:    func() { atom.Set(w.StepTo(1)) },
				PrevLabel: uistate.T("resolution.toEarlier"),
				NextLabel: uistate.T("resolution.toLater"),
			}),
		)
	} else {
		stepper = ui.StepperPill(ui.StepperPillProps{
			Label:     w.Label(),
			OnPrev:    func() { atom.Set(w.Shift(-1)) },
			OnNext:    func() { atom.Set(w.Shift(1)) },
			PrevLabel: uistate.T("resolution.prevPeriod"),
			NextLabel: uistate.T("resolution.nextPeriod"),
		})
	}

	rangeLabel := uistate.T("resolution.customRange")
	if rangeMode.Get() {
		rangeLabel = uistate.T("resolution.singlePeriod")
	}

	return Span(Class("reso-control flex items-center gap-2.5"),
		ui.Segmented(ui.SegmentedProps{
			Options: []ui.SegOption{
				{Value: string(period.Week), Label: "Week"},
				{Value: string(period.Month), Label: "Month"},
				{Value: string(period.Quarter), Label: "Quarter"},
			},
			Selected: string(w.Res),
			OnSelect: func(v string) {
				r := period.Resolution(v)
				uistate.PersistResolution(r)
				atom.Set(w.SetResolution(r, time.Now()))
			},
		}),
		Select(Class("rstep text-[12px]"), Attr("aria-label", uistate.T("resolution.jumpTo")), Attr("title", uistate.T("resolution.jumpTo")), OnChange(onPreset),
			Option(Value(""), SelectedIf(true), uistate.T("resolution.jumpTo")),
			Option(Value("this"), uistate.T("resolution.presetThis")),
			Option(Value("last"), uistate.T("resolution.presetLast")),
			Option(Value("quarter"), uistate.T("resolution.presetQuarter")),
			Option(Value("ytd"), uistate.T("resolution.presetYTD")),
		),
		stepper,
		If(!isCurrent, Button(Class("px-2 py-1 text-dim hover:text-fg text-[12px]"), Type("button"),
			Attr("title", uistate.T("resolution.thisPeriodTitle")),
			OnClick(func() { atom.Set(period.NewWindow(w.Res, time.Now(), w.WeekStart)) }),
			uistate.T("resolution.thisPeriod"))),
		Button(Class("px-2 py-1 text-faint hover:text-fg text-[12px]"), Type("button"),
			OnClick(func() {
				if rangeMode.Get() {
					// Leaving range mode collapses back to a single period.
					atom.Set(w.Single())
					rangeMode.Set(false)
				} else {
					rangeMode.Set(true)
				}
			}),
			rangeLabel),
	)
}
