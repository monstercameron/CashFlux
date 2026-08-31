// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"fmt"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/favorites"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/modules"
	"github.com/monstercameron/CashFlux/internal/navorder"
	"github.com/monstercameron/CashFlux/internal/navsearch"
	"github.com/monstercameron/CashFlux/internal/pages"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
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
	// ContentKey, when set, overrides ActivePath as the View subtree's remount
	// key (see the WithKey call below) without affecting anything else that
	// reads ActivePath (rail highlighting, the topbar title, data-route,
	// scroll memory). Routes where several distinct URLs should share ONE
	// chrome identity but still force the content itself to remount on every
	// change — "/settings/:tab": every tab is still "Settings" for
	// highlighting/breadcrumb purposes, but each tab's content must remount
	// (WithKey is how "/p/:slug" avoids two custom pages sharing one stale
	// render — same mechanism, just decoupled from the chrome identity here).
	ContentKey string
}

// contentKeyOrActivePath returns props.ContentKey when set, else props.ActivePath —
// the fallback that makes ContentKey opt-in: every route except "/settings/:tab"
// leaves it empty and keeps today's one-key-per-route behavior unchanged.
func contentKeyOrActivePath(props ShellProps) string {
	if props.ContentKey != "" {
		return props.ContentKey
	}
	return props.ActivePath
}

// sidebarProps carries the active route path so the rail re-renders and the
// highlight follows each navigation.
type sidebarProps struct {
	ActivePath string
}

// mobileTabItemProps configures one item in the mobile bottom tab bar (or, with
// Sheet set, one row of the More sheet). OnPick, when non-nil, runs after the
// navigation — the More sheet passes a closer so picking a destination dismisses it.
type mobileTabItemProps struct {
	Label  string
	Path   string
	Icon   icon.Name
	Active bool
	Sheet  bool
	OnPick func()
}

// mobileTabItem is a single tappable entry in the mobile bottom tab bar / More
// sheet. Its own component so its click-handler hook stays at a stable position
// regardless of how many items are in the bar (the On*-hooks-in-loops rule).
func mobileTabItem(props mobileTabItemProps) uic.Node {
	nav := router.UseNavigate()
	path, onPick := props.Path, props.OnPick
	cls := "mobile-tab-item"
	if props.Sheet {
		cls = "mobile-sheet-item"
	}
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
			if onPick != nil {
				onPick()
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

// MobileTabBar renders the phone-width bottom navigation (CSS controls the
// breakpoint; hidden on desktop). UX-01 redesign: FIVE fixed slots — Home,
// Transactions, Budgets, Goals, and a More toggle — so labels never clip in a
// scrolling strip, plus a floating quick-add button above the bar. The
// remaining primary destinations (Accounts, To-do, Notifications, Assistant,
// Reports) live in the More bottom sheet; when the active route is one of
// them, the More tab itself lights up so the current destination always shows
// with color AND label.
func MobileTabBar(props mobileTabBarProps) uic.Node {
	cur := props.ActivePath
	quickAdd := uistate.UseQuickAdd()
	openAdd := uic.UseEvent(func() { quickAdd.Set(true) })
	moreOpen := uic.UseState(false)
	toggleMore := uic.UseEvent(Prevent(func() { moreOpen.Set(!moreOpen.Get()) }))
	closeMore := uic.UseEvent(Prevent(func() { moreOpen.Set(false) }))
	closeSheet := func() { moreOpen.Set(false) }

	moreDests := []mobileTabItemProps{
		{Label: uistate.T("nav.accounts"), Path: "/accounts", Icon: icon.Accounts, Active: cur == "/accounts", Sheet: true, OnPick: closeSheet},
		{Label: uistate.T("nav.todo"), Path: "/todo", Icon: icon.Todo, Active: cur == "/todo", Sheet: true, OnPick: closeSheet},
		{Label: uistate.T("nav.notifications"), Path: "/notifications", Icon: icon.Bell, Active: cur == "/notifications", Sheet: true, OnPick: closeSheet},
		{Label: uistate.T("nav.assistant"), Path: "/assistant", Icon: icon.Sparkles, Active: cur == "/assistant", Sheet: true, OnPick: closeSheet},
		{Label: uistate.T("nav.reports"), Path: "/reports", Icon: icon.Reports, Active: cur == "/reports", Sheet: true, OnPick: closeSheet},
	}
	moreActive := false
	for _, d := range moreDests {
		if d.Active {
			moreActive = true
			break
		}
	}
	moreCls := "mobile-tab-item mobile-tab-more"
	if moreActive || moreOpen.Get() {
		moreCls += " active"
	}
	sheetRows := make([]any, 0, len(moreDests)+2)
	sheetRows = append(sheetRows, css.Class("mobile-more-sheet"), Attr("role", "menu"),
		Attr("aria-label", uistate.T("nav.mobileMoreSheet")), Attr("data-testid", "mobile-more-sheet"))
	for _, d := range moreDests {
		sheetRows = append(sheetRows, uic.CreateElement(mobileTabItem, d))
	}

	expanded := "false"
	if moreOpen.Get() {
		expanded = "true"
	}
	return Fragment(
		If(moreOpen.Get(), Div(css.Class("mobile-more-backdrop"), Attr("data-testid", "mobile-more-backdrop"), OnClick(closeMore))),
		If(moreOpen.Get(), Div(sheetRows...)),
		// Floating quick-add: one thumb-reach action above the bar, replacing the
		// old sixth pinned slot so the bar stays five equal destinations.
		Button(css.Class("mobile-tab-fab"), Type("button"),
			Attr("aria-label", uistate.T("action.quickAdd")),
			Title(uistate.T("action.quickAdd")),
			Attr("data-testid", "mobile-tab-fab"),
			OnClick(openAdd),
			ui.Icon(icon.Plus, css.Class(tw.W5, tw.H5)),
		),
		Nav(css.Class("mobile-tabbar"), Attr("aria-label", uistate.T("nav.mobileTabLabel")),
			uic.CreateElement(mobileTabItem, mobileTabItemProps{Label: uistate.T("nav.mobileHome"), Path: "/", Icon: icon.Dashboard, Active: cur == "/"}),
			uic.CreateElement(mobileTabItem, mobileTabItemProps{Label: uistate.T("nav.transactions"), Path: "/transactions", Icon: icon.Transactions, Active: cur == "/transactions"}),
			uic.CreateElement(mobileTabItem, mobileTabItemProps{Label: uistate.T("nav.budgets"), Path: "/budgets", Icon: icon.Budgets, Active: cur == "/budgets"}),
			uic.CreateElement(mobileTabItem, mobileTabItemProps{Label: uistate.T("nav.goals"), Path: "/goals", Icon: icon.Goals, Active: cur == "/goals"}),
			Button(ClassStr(moreCls), Type("button"),
				Attr("aria-label", uistate.T("nav.mobileMore")),
				Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
				Attr("data-testid", "mobile-tab-more"),
				OnClick(toggleMore),
				ui.Icon(icon.MoreH, css.Class(tw.W5, tw.H5)),
				Span(css.Class("mobile-tab-label"), uistate.T("nav.mobileMore")),
			),
		),
	)
}

// Shell renders the candidate-C application chrome: a fixed left rail and an
// independently scrolling main pane with a sticky top bar, wrapping the active
// screen's content. (Ported from design/candidate-c.html.)
// ScrollToTopButton is a global floating "back to top" control. It sits fixed at the
// bottom-right and reveals itself once the main scroll region (#main) is scrolled down a
// screenful, then smooth-scrolls back to the top on click. A native scroll listener toggles
// the .is-visible class directly (no Go re-render per scroll event), set up once on mount and
// torn down on unmount.
func ScrollToTopButton() uic.Node {
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		main := doc.Call("getElementById", "main")
		btn := doc.Call("getElementById", "cf-scrolltop")
		if !main.Truthy() || !btn.Truthy() {
			return nil
		}
		var handler js.Func
		handler = js.FuncOf(func(js.Value, []js.Value) any {
			if main.Get("scrollTop").Float() > 320 {
				btn.Get("classList").Call("add", "is-visible")
			} else {
				btn.Get("classList").Call("remove", "is-visible")
			}
			return nil
		})
		main.Call("addEventListener", "scroll", handler, js.ValueOf(map[string]any{"passive": true}))
		return func() {
			main.Call("removeEventListener", "scroll", handler)
			handler.Release()
		}
	}, "cf-scrolltop")

	onClick := uic.UseEvent(Prevent(func() {
		main := js.Global().Get("document").Call("getElementById", "main")
		if main.Truthy() {
			main.Call("scrollTo", js.ValueOf(map[string]any{"top": 0, "behavior": "smooth"}))
		}
	}))
	return Button(css.Class("cf-scrolltop"), Attr("id", "cf-scrolltop"), Type("button"),
		Attr("data-testid", "scroll-to-top"),
		Attr("aria-label", uistate.T("app.scrollToTop")), Title(uistate.T("app.scrollToTop")),
		OnClick(onClick),
		ui.Icon(icon.ArrowUp, css.Class(tw.W5, tw.H5)),
	)
}

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
		// The Smart strip resets to collapsed on navigation so the decision-first
		// default holds page to page (its trigger lives in the top bar).
		uistate.SetSmartStripOpen(false)
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
		// data-route reflects the logical path of the screen currently mounted here.
		// It's the deterministic signal the regression suite waits on after an SPA
		// navigation (the new route's content has rendered once this equals the
		// target), replacing content-change guessing. Harmless in production.
		Main(css.Class("cf-scroll", tw.Flex1, tw.MinW0, tw.OverflowYAuto), Attr("id", "main"), Attr("data-route", props.ActivePath), Attr("tabindex", "-1"),
			uic.CreateElement(TopBar, topBarProps{Title: props.Title, ActivePath: props.ActivePath}),
			uic.CreateElement(SubscriptionBanner),
			// C281: "Viewing as <member>" scope banner — shown whenever the top-bar
			// member switcher has a member selected. Renders nothing for the default
			// everyone/"" view. Placed after the other global status banners so the
			// stacking order reads: sample → subscription → member scope.
			uic.CreateElement(ScopeBanner),
			// C581: the one-hop way back out of a cross-page correction (ledger →
			// Rules / Activity / To-do). Shown only on the route the trip was for, and
			// it names the task it returns to rather than just saying "Back".
			uic.CreateElement(ReturnBanner, returnBannerProps{ActivePath: props.ActivePath}),
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
				WithKey(uic.CreateElement(props.View), contentKeyOrActivePath(props))),
		),
		// Mobile bottom tab bar (L11): shown only at phone widths (CSS agent controls
		// the breakpoint). The desktop left rail is unchanged — this is additive.
		uic.CreateElement(MobileTabBar, mobileTabBarProps{ActivePath: props.ActivePath}),
		// #60: per-route scroll memory — Back/Forward restores where you were.
		uic.CreateElement(scrollMemoryHost, scrollMemoryProps{ActivePath: props.ActivePath}),
		uic.CreateElement(SettingsHost),
		uic.CreateElement(QuickAddHost),
		uic.CreateElement(AddHost),
		uic.CreateElement(TxnEditHost),
		uic.CreateElement(TxnSplitHost),
		uic.CreateElement(TxnHistoryHost),
		uic.CreateElement(TxnColumnsHost),
		uic.CreateElement(TxnSmartCatHost),
		uic.CreateElement(TxnLinkHost),
		uic.CreateElement(RefundPairHost),
		uic.CreateElement(ImportPanelHost),
		uic.CreateElement(DuplicatesHost),
		uic.CreateElement(TransferReviewHost),
		uic.CreateElement(AccountEditHost),
		uic.CreateElement(AccountTransferHost),
		uic.CreateElement(BudgetEditHost),
		uic.CreateElement(AutoBudgetHost),
		uic.CreateElement(AdjustAllHost),
		uic.CreateElement(BudgetBasisHost),
		uic.CreateElement(BudgetCategoriesHost),
		uic.CreateElement(GoalEditHost),
		uic.CreateElement(GoalCompareHost),
		uic.CreateElement(TaskEditHost),
		uic.CreateElement(PayeeCleanHost),
		uic.CreateElement(CatSuggestHost),
		uic.CreateElement(ReviewInboxHost),
		uic.CreateElement(MemberEditHost),
		uic.CreateElement(CategoryEditHost),
		uic.CreateElement(RuleEditHost),
		uic.CreateElement(ArtifactEditHost),
		uic.CreateElement(InvestAddHost),
		uic.CreateElement(InvestPoolEditHost),
		uic.CreateElement(AccountGroupsEditHost),
		uic.CreateElement(InstitutionsManagerHost),
		uic.CreateElement(SweepRulesHost),
		uic.CreateElement(AllocProfileHost),
		uic.CreateElement(RecurringEditHost),
		uic.CreateElement(SubsPrefsHost),
		uic.CreateElement(BillsSmartHost),
		uic.CreateElement(CredentialVaultHost),
		uic.CreateElement(DialogHost),
		// C274: profile-switch modal — "Who's using CashFlux?" device user-switching.
		uic.CreateElement(ProfileSwitchHost),
		// C309 (#464): sync-conflict resolve modal — "Keep my changes" (force-push)
		// or "Use server version" (pull + discard stash). Opened by the amber chip.
		uic.CreateElement(SyncConflictHost),
		uic.CreateElement(UpgradeSheet),
		uic.CreateElement(Toast),
		// Global floating "back to top" button — reveals on scroll, jumps #main to the top.
		uic.CreateElement(ScrollToTopButton),
		// Deep-link focus: after a notification jumps to an account/budget, scroll to
		// and pulse that exact card. Mounted once so its hook depth is constant.
		uic.CreateElement(DeepLinkFocusHost),
		// Headless SMART proactive digest driver: fires on cadence when opted in,
		// posting a brief insight summary to the notification feed. Mounted once
		// here (not in a loop) so its hook depth is always constant.
		uic.CreateElement(screens.SmartDigestDriver),
		// PS8: the assistant's guided highlight. Mounted at the shell so the ring
		// survives the navigation that puts the control on screen — mounting it
		// inside a screen would unmount it at the moment it is needed.
		uic.CreateElement(screens.SpotlightHost),
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
	"/events":         {"nav.events", icon.Calendar},
	"/subscriptions":  {"nav.subscriptions", icon.Subscriptions},
	"/bills":          {"nav.bills", icon.Bills},
	"/split":          {"nav.split", icon.Split},
	"/insights":       {"nav.insights", icon.Insights},
	"/documents":      {"nav.documents", icon.Page},
	"/artifacts":      {"nav.artifacts", icon.Page},
	"/widget-builder": {"nav.widgetBuilder", icon.PlusCircle},
	"/widget-manager": {"nav.widgetManager", icon.Dashboard},
	"/members":        {"nav.members", icon.Users},
	"/categories":     {"nav.categories", icon.Tag},
	"/rules":          {"nav.rules", icon.Tag},
	"/notifications":  {"nav.notifications", icon.Bell},
	"/settings":       {"nav.settings", icon.Settings},
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

// navGroupStatic enumerates one screen group's rail items WITHOUT calling any
// framework hook, so it is safe outside a component render (boot wiring, the
// Ctrl+K palette build — navGroup's UseAdminConsoleAvailable hook panics there
// with "GoUseAtom called outside component context", killing the whole app on
// the first palette command). AdminOnly routes are gated by the captured-atom
// read, which is fail-closed until a render captures it.
func navGroupStatic(group string) []railItem {
	isAdmin := uistate.AdminConsoleAvailable()
	var items []railItem
	for _, r := range screens.All() {
		if r.Group != group || (r.AdminOnly && !isAdmin) {
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

// primaryNavStatic is the hook-free primary group (keyboard digit-nav, palette).
func primaryNavStatic() []railItem { return navGroupStatic(screens.GroupPrimary) }

// toolsNav is the Phase-2 "Tools" group: the routed power-tool screens that were
// otherwise only reachable by URL.
func toolsNav() []railItem { return navGroup(screens.GroupTools) }

// systemNav is the "System" group: the household-configuration screens.
func systemNav() []railItem { return navGroup(screens.GroupSystem) }

// toolSubGroupLabel resolves a Tools sub-group id to its display label.
func toolSubGroupLabel(sg string) string {
	switch sg {
	case screens.SubGroupDaily:
		return uistate.T("nav.sectionDaily")
	case screens.SubGroupPlan:
		return uistate.T("nav.sectionPlan")
	case screens.SubGroupInsights:
		return uistate.T("nav.sectionInsights")
	case screens.SubGroupNext:
		return uistate.T("nav.sectionNext")
	case screens.SubGroupSetup:
		return uistate.T("nav.sectionSetup")
	}
	return sg
}

// railSections returns every rail destination the household can see, grouped by
// section in NavSections order.
//
// It walks all three registry Groups together on purpose. Group used to decide
// where a screen appeared in the rail, which is why the daily money screens ended
// up beside a chat assistant and an inbox — they shared a Group, not a purpose.
// Section decides placement now; Group is left to the command palette and the
// keyboard fallbacks, which do still care about it.
func railSections(hidden modules.Hidden) map[string][]railItem {
	out := map[string][]railItem{}
	for _, g := range []string{screens.GroupPrimary, screens.GroupTools, screens.GroupSystem} {
		for _, it := range navGroup(g) {
			if hidden.IsHidden(it.Path) {
				continue
			}
			sg := it.SubGroup
			if sg == "" {
				// A screen added without a section still appears, in Setup, rather
				// than being silently dropped from the menu (B7).
				sg = screens.SubGroupSetup
			}
			out[sg] = append(out[sg], it)
		}
	}
	return out
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
	var visibleSystem []railItem
	for _, it := range systemNav() {
		if !hidden.IsHidden(it.Path) {
			visibleSystem = append(visibleSystem, it)
		}
	}

	// ── Pinned destinations ─────────────────────────────────────────────────
	//
	// The number keys used to be positional: Alt+1..9 went to the first nine
	// PRIMARY screens in registry order, and nobody could change that. The screens
	// a household actually lives in are not the same nine for any two of them, so
	// the fastest keys in the app were spent on a list nobody chose.
	//
	// A browser that has never pinned anything is seeded with exactly those nine,
	// so the keys keep doing what they always did and the badges appear where they
	// already were. Someone who has deliberately unpinned everything is a different
	// state, which is why the seed is gated on FavoritesChosen rather than on the
	// list being empty — otherwise clearing it would silently restore the defaults
	// and the pins would look impossible to remove.
	favAtom := uistate.UseFavorites()
	favRaw := favAtom.Get()
	if favRaw == nil && !uistate.FavoritesChosen() {
		seed := make([]string, 0, favorites.Max)
		for _, it := range visibleNav {
			if len(seed) == favorites.Max {
				break
			}
			seed = append(seed, it.Path)
		}
		favRaw = seed
	}
	// Everything the rail can reach, so a pin to something since deleted or hidden
	// does not hold a number key that silently does nothing.
	reachable := map[string]railItem{}
	for _, it := range visibleNav {
		reachable[it.Path] = it
	}
	for _, it := range visibleTools {
		reachable[it.Path] = it
	}
	for _, it := range visibleSystem {
		reachable[it.Path] = it
	}
	// Custom pages count as reachable. Without this, Clean treats a pinned page as
	// pointing at nothing and silently drops it — so pinning one would appear to do
	// nothing at all, which is the same symptom as a broken button.
	if a := appstate.Default; a != nil {
		for _, cp := range pages.Ordered(pages.Visible(a.CustomPages())) {
			reachable[uistate.RoutePath("/p/"+cp.Slug)] = railItem{
				Key: cp.Name, Path: uistate.RoutePath("/p/" + cp.Slug), Icon: icon.FileText,
			}
		}
	}
	// TWO lists, deliberately. favPaths is what the rail draws — Clean drops
	// anything it cannot currently reach, so a stale pin never holds a number key
	// that opens nothing. favRaw is what gets STORED, and it keeps those entries.
	//
	// The difference is not academic. A custom page that has not loaded yet is not
	// a page that is gone, and every edit below used to be applied to the cleaned
	// list and then persisted — so one render where a destination was briefly
	// unreachable deleted that pin permanently. Edits address the raw list BY PATH
	// now; a display index means nothing once anything has been cleaned out.
	favPaths := favorites.Clean(favRaw, func(p string) bool { _, ok := reachable[p]; return ok })

	// The pin being dragged. Deliberately NOT the folder rows' drag source: one
	// shared atom would let a pin dropped on a folder row run that section's
	// reorder with a path the section does not contain.
	favDrag := uic.UseState("")
	setFavorites := func(next []string) {
		favAtom.Set(next)
		uistate.PersistFavorites(next)
		publishFavorites(next)
	}
	// The keyboard reorder (Alt+Arrow) runs in the boot-time keydown listener,
	// which cannot read a hook — so the Sidebar hands it a mover closed over the
	// same favPaths the rail renders.
	publishPinnedMover(func(from, to int) bool {
		if from < 0 || from >= len(favPaths) || to < 0 || to >= len(favPaths) || from == to {
			return false
		}
		setFavorites(favorites.MoveBefore(favRaw, favPaths[from], favPaths[to]))
		return true
	})
	// Published for the boot-time keydown listener, which lives outside any
	// component and cannot read a hook.
	uic.UseEffect(func() func() {
		publishFavorites(favPaths)
		return nil
	}, strings.Join(favPaths, "|"))
	// The path most recently unpinned, so the folder it landed in can reveal it.
	// Cleared on the next pin, and never persisted: it describes one action, not a
	// preference.
	justUnpinned := uic.UseState("")
	// The screen waiting for a slot. Set when someone pins an eleventh: the list is
	// full, so instead of refusing, the rail asks which pinned screen it replaces.
	// Never persisted — an unanswered question should not survive a reload.
	pendingSwap := uic.UseState("")
	pinFull := favorites.Full(favRaw)
	// A pinned destination is LIFTED out of its folder rather than copied to the
	// top. The first cut showed it in both places, which doubled the rail's length
	// and left the reader deciding whether the two rows were the same thing — they
	// were. Pinning reads as "move this within reach", so the row moves.
	lifted := func(items []railItem) []railItem {
		out := make([]railItem, 0, len(items))
		for _, it := range items {
			if !favorites.Contains(favPaths, it.Path) {
				out = append(out, it)
			}
		}
		return out
	}
	// pinProps decorates a row with its pin control and slot badge. Every rendered
	// destination gets one, so a screen can be pinned from wherever it is found
	// rather than only from the section it happens to live in.
	pinProps := func(path string) (bool, string, bool, func()) {
		pinned := favorites.Contains(favPaths, path)
		slot := ""
		if i := favorites.IndexOf(favPaths, path); i >= 0 {
			if d, ok := favorites.DigitFor(i); ok {
				slot = d
			}
		}
		return pinned, slot, pinFull, func() {
			next, nowPinned := favorites.Toggle(favRaw, path)
			if nowPinned && !favorites.Contains(next, path) {
				// Full. Ask which slot to give up rather than refusing: the
				// eleventh screen is exactly the one the user just said they want
				// within reach, and a dead button says nothing about how to get it.
				pendingSwap.Set(path)
				return
			}
			if !nowPinned {
				// Unpinning MOVES the row back into its folder, and every folder
				// defaults to collapsed — so without this the row simply vanished
				// when you unpinned it, with nothing on screen to say where it went.
				// Naming it here lets the folder that received it open itself.
				justUnpinned.Set(path)
			} else {
				justUnpinned.Set("")
			}
			setFavorites(next)
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
	// #60 lean sidebar: sections default to COLLAPSED — the rail leads with the
	// nine primary destinations, and the advanced sections open on demand. An
	// explicit stored value (either way) always wins over the default.
	groupCollapsed := func(sg string) bool {
		if v, ok := collapsed[sg]; ok {
			return v
		}
		return true
	}
	// Keep the shared active-nav indicator glued to the selected item (motion spec
	// §4: one indicator that SLIDES to the selection). Re-measure whenever the
	// active route, rail collapse, an accordion section, or the nav order shifts
	// vertical layout; the DOM write happens post-render inside a rAF.
	// The pinned order is part of the rail's vertical layout, so it has to be in
	// this key: without it, reordering a pin moved every row below it while the
	// indicator stayed where it was, and the highlight drifted further with each
	// move because the effect never re-ran (Cam 2026-08-31).
	indKey := current + "|" + railAnimKey + "|" + fmt.Sprint(collapsed) +
		"|" + fmt.Sprint(orderedPaths) + "|" + strings.Join(favPaths, ",")
	uic.UseEffect(func() func() {
		positionRailIndicator()
		return nil
	}, indKey)
	// One loop for every section. The rail used to render three different ways —
	// a flat primary list, a sub-grouped tools accordion, and a system block — which
	// is why the daily money screens could not sit beside anything else and why
	// "Build" ended up as a heading over one row. Placement is data now: a screen's
	// section decides where it appears, and adding one needs no code here.
	sectionItems := railSections(hidden)
	// "You are here", and "there it went": a section reveals itself when it holds
	// the current route or the row just unpinned, without touching the stored
	// preference — otherwise deep-linking to a filed screen leaves no active cue
	// anywhere in the rail, and unpinning drops a row into a folder that is shut.
	revealSection := map[string]bool{}
	for sg, items := range sectionItems {
		for _, it := range items {
			if it.Path == current || it.Path == justUnpinned.Get() {
				revealSection[sg] = true
				break
			}
		}
	}
	var sectionNodes []any
	for _, sg := range screens.NavSections {
		sg := sg
		items := lifted(sectionItems[sg])
		if len(items) == 0 {
			// Every row pinned, or nothing here: an empty folder is a heading that
			// opens onto nothing.
			continue
		}
		// The user's drag order applies WITHIN a section, so reordering still means
		// something once destinations are filed rather than in one flat list.
		paths := make([]string, len(items))
		for i, it := range items {
			paths[i] = it.Path
		}
		byPath := make(map[string]railItem, len(items))
		for _, it := range items {
			byPath[it.Path] = it
		}
		ordered := make([]railItem, 0, len(items))
		for _, pth := range navorder.Apply(navOrder.Get(), paths) {
			if it, ok := byPath[pth]; ok {
				ordered = append(ordered, it)
			}
		}
		isCollapsed := groupCollapsed(sg) && !revealSection[sg]
		sectionNodes = append(sectionNodes, uic.CreateElement(toolGroupHeader, toolGroupHeaderProps{
			Label:     toolSubGroupLabel(sg),
			Collapsed: isCollapsed,
			OnToggle:  func() { setCollapsed(sg, !isCollapsed) },
		}))
		if isCollapsed {
			continue
		}
		sectionNodes = append(sectionNodes, MapKeyed(ordered,
			func(it railItem) any { return it.Path },
			func(it railItem) uic.Node {
				pth := it.Path
				pinned, slot, full, onPin := pinProps(pth)
				return uic.CreateElement(navItem, navItemProps{
					Label: uistate.T(it.Key), Path: pth, Icon: it.Icon, Active: current == pth,
					Pinned: pinned, Slot: slot, PinFull: full, OnPin: onPin,
					Draggable:   true,
					OnDragStart: func() { dragSrc.Set(pth) },
					OnDrop:      func() { reorderNav(pth) },
				})
			},
		))
	}

	swapping := pendingSwap.Get()
	if swapping != "" {
		// The question is only answerable while the incoming screen still exists and
		// the list is still full; anything else and it is stale.
		if _, ok := reachable[swapping]; !ok || !pinFull || favorites.Contains(favPaths, swapping) {
			swapping = ""
		}
	}
	labelFor := func(path string) string {
		it, ok := reachable[path]
		if !ok {
			return path
		}
		if strings.HasPrefix(path, uistate.RoutePath("/p/")) {
			return it.Key
		}
		return uistate.T(it.Key)
	}
	cancelSwap := uic.UseEvent(func() { pendingSwap.Set("") })
	// Escape backs out. The question steals the rail's meaning while it stands, so
	// it must be dismissible by the key people already press to leave things.
	uic.UseEffect(func() func() {
		if pendingSwap.Get() == "" {
			return nil
		}
		doc := js.Global().Get("document")
		cb := js.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Get("key").String() == "Escape" {
				pendingSwap.Set("")
			}
			return nil
		})
		doc.Call("addEventListener", "keydown", cb)
		return func() {
			doc.Call("removeEventListener", "keydown", cb)
			cb.Release()
		}
	}, pendingSwap.Get())
	var pinnedNodes []any
	if swapping != "" {
		// The prompt is a live region: the list below it does not visibly change
		// when the mode opens, so nothing would otherwise tell a screen-reader user
		// that their click asked a question.
		pinnedNodes = append(pinnedNodes, Div(css.Class("rail-swap"),
			Attr("data-testid", "rail-swap-prompt"), Attr("role", "status"), Attr("aria-live", "polite"),
			P(css.Class("rail-swap-q"), uistate.T("rail.swapPrompt", labelFor(swapping))),
			P(css.Class("rail-swap-hint"), uistate.T("rail.swapHint")),
			Button(css.Class("rail-swap-cancel"), Type("button"),
				Attr("data-testid", "rail-swap-cancel"), OnClick(cancelSwap),
				uistate.T("rail.swapCancel")),
		))
	}
	if len(favPaths) > 0 {
		pinnedNodes = append(pinnedNodes, MapKeyed(favPaths,
			func(p string) any { return p },
			func(p string) uic.Node {
				it, ok := reachable[p]
				if !ok {
					return Fragment()
				}
				if swapping != "" {
					// In swap mode a pinned row is not a link — it is the slot being
					// offered. Rendering it as a button rather than restyling the link
					// is what keeps a click from navigating away mid-question.
					idx := favorites.IndexOf(favPaths, p)
					digit, _ := favorites.DigitFor(idx)
					incoming, victim := swapping, p
					return uic.CreateElement(swapTarget, swapTargetProps{
						Label: labelFor(p), Slot: digit, Icon: it.Icon,
						Aria: uistate.T("rail.swapRowAria", labelFor(p), labelFor(incoming), digit),
						OnPick: func() {
							next := favorites.ReplacePath(favRaw, victim, incoming)
							pendingSwap.Set("")
							justUnpinned.Set(victim)
							setFavorites(next)
							uistate.PostNotice(uistate.T("rail.swapDone",
								labelFor(incoming), labelFor(victim), uistate.T("rail.jumpHint", digit)), false)
						},
					})
				}
				pinned, slot, full, onPin := pinProps(p)
				// A custom page's Key IS its name — the household typed it — so it
				// is used verbatim; registry items carry an i18n key instead.
				label := uistate.T(it.Key)
				if strings.HasPrefix(p, uistate.RoutePath("/p/")) {
					label = it.Key
				}
				src := p
				return uic.CreateElement(navItem, navItemProps{
					Label: label, Path: p, Icon: it.Icon, Active: current == p,
					// The row says how to move it. A drag is invisible to anyone not
					// holding a mouse, so the keyboard route has to be announced
					// rather than discovered.
					AriaSuffix: uistate.T("rail.reorderHint"),
					Pinned:     pinned, Slot: slot, PinFull: full, OnPin: onPin,
					// Dragging rearranges the SLOTS: the digits stay 1..9,0 down the
					// list and the rows move between them. That is the right way
					// round — the number is a position, and moving a row to second
					// place is the whole reason for moving it.
					Draggable:   true,
					OnDragStart: func() { favDrag.Set(src) },
					OnDrop: func() {
						moving := favDrag.Get()
						favDrag.Set("")
						// A drop that lands on nothing, or on the row that started it,
						// is not a reorder — leave the list exactly as it was.
						if moving == "" || moving == src {
							return
						}
						setFavorites(favorites.MoveBefore(favRaw, moving, src))
					},
				})
			},
		))
	} else {
		// An empty section reads as broken unless it says what it is for.
		pinnedNodes = append(pinnedNodes, Div(css.Class("rail-pinned-empty"),
			Attr("data-testid", "rail-pinned-empty"),
			uistate.T("rail.pinnedEmpty", uistate.T("rail.pinnedKeys"))))
	}
	// The destination filter. When the query is blank this whole block is inert and
	// the rail below renders exactly as it always has — the filtered list is an
	// alternative to the sections, never a modification of them.
	railQuery := uic.UseState("")
	q := strings.TrimSpace(railQuery.Get())
	searching := q != "" && !railCollapsed
	var matches []navsearch.Item
	if searching {
		matches = navsearch.Filter(railSearchable(visibleNav, visibleTools, visibleSystem), q)
	}
	navTo := router.UseNavigate()
	onRailSearch := uic.UseEvent(func(e uic.Event) { railQuery.Set(e.GetValue()) })
	clearRailSearch := uic.UseEvent(func() { railQuery.Set("") })
	// Enter goes to the top match, which is the point of ranking it. Escape clears
	// rather than blurring: a filter you cannot see the end of is a rail you cannot
	// use, so the exit has to be the key people already press.
	onRailSearchKey := uic.UseEvent(func(e uic.KeyboardEvent) {
		switch e.GetKey() {
		case "Enter":
			e.PreventDefault()
			if len(matches) > 0 {
				railQuery.Set("")
				navTo.Navigate(matches[0].Path)
			}
		case "Escape":
			e.PreventDefault()
			railQuery.Set("")
		}
	})
	var railSearchNodes []any
	if searching {
		if len(matches) == 0 {
			railSearchNodes = append(railSearchNodes,
				Div(css.Class("railsearch-empty"), Attr("data-testid", "railsearch-empty"),
					P(uistate.T("railsearch.noMatch", q)),
					P(css.Class("railsearch-empty-hint"), uistate.T("railsearch.hintShorter"))))
		} else {
			railSearchNodes = append(railSearchNodes, MapKeyed(matches,
				func(it navsearch.Item) any { return it.Path },
				func(it navsearch.Item) uic.Node {
					pinned, slot, full, onPin := pinProps(it.Path)
					return uic.CreateElement(navItem, navItemProps{
						Label: it.Label, Path: it.Path, Icon: railIconFor(it.Path),
						Active: current == it.Path,
						Pinned: pinned, Slot: slot, PinFull: full, OnPin: onPin,
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
		// Hidden while the rail is collapsed to icons: there is no width for a text
		// field, and a filter you cannot read the results of is worse than none.
		If(!railCollapsed, uic.CreateElement(railSearchBox, railSearchBoxProps{
			Query: railQuery.Get(), Matches: len(matches),
			OnInput: onRailSearch, OnKey: onRailSearchKey, OnClear: clearRailSearch,
		})),
		Nav(css.Class("rail-nav", tw.Flex1, tw.MinH0, tw.OverflowYAuto, tw.P3, tw.Flex, tw.FlexCol, tw.Gap05, tw.TextDim, tw.Text135), Attr("aria-label", uistate.T("nav.primaryLabel")),
			// The single shared active-page indicator (motion spec §4). Positioned
			// absolutely by positionRailIndicator(); CSS slides top/height between
			// items over the standard token. Decorative — aria-current on the item
			// itself carries the semantics.
			// Hidden during a search: it is positioned from the active item's
			// offset, and a filtered list either does not contain that item or has
			// moved it, so leaving it visible parks a highlight over an unrelated
			// row.
			If(!searching, Div(Attr("id", "cf-rail-ind"), ClassStr("rail-ind"), Attr("aria-hidden", "true"))),
			Fragment(railSearchNodes...),
			If(!searching, Fragment(
				Div(css.Class("rail-sec-head rail-pinned-head"), Attr("data-testid", "rail-pinned-head"),
					Span(uistate.T("rail.pinnedSection"))),
				Nav(css.Class("rail-pinned"), Attr("data-testid", "rail-pinned"),
					Attr("aria-label", uistate.T("rail.pinnedSection")),
					Fragment(pinnedNodes...)),
				Fragment(sectionNodes...),
				// The user's custom pages ("My pages"): listing, create, and reorder.
				uic.CreateElement(CustomPagesNav, CustomPagesNavProps{PinFor: pinProps}),
			)),
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
	// Pinning. OnPin nil means this row has no pin control at all — the search
	// results reuse this component and a pin there would sit under a list that is
	// about to disappear.
	OnPin func()
	// AriaSuffix is appended to the row's accessible name — used to announce an
	// affordance that has no visible label, like the keyboard reorder.
	AriaSuffix string
	// Pinned drives the toggle's state and keeps its star filled and visible even
	// when the row is not hovered; Slot is the digit key this row answers to ("" if
	// unpinned). PinFull disables the control on an unpinned row once ten are taken.
	Pinned  bool
	Slot    string
	PinFull bool
}

type swapTargetProps struct {
	Label  string
	Slot   string
	Icon   icon.Name
	Aria   string
	OnPick func()
}

// swapTarget is one pinned row while the rail is asking which slot an eleventh
// screen should take.
//
// It is a BUTTON, not the usual link restyled. During the question a click has to
// answer it, and a link whose click has been repurposed still announces itself as
// a link, still offers open-in-new-tab, and still shows a URL in the status bar —
// three promises the row cannot keep while it means "give up this slot". Its own
// component so the click hook stays at a stable position across the list.
func swapTarget(props swapTargetProps) uic.Node {
	onPick := props.OnPick
	return Div(css.Class("nav-row"),
		Button(css.Class("nv nav-swap-target "+tw.Fold(tw.Flex, tw.Flex1, tw.MinH10, tw.ItemsCenter, tw.Gap25, tw.Px3, tw.Py2, tw.Rounded4, tw.CursorPointer)),
			Type("button"), Attr("data-testid", "rail-swap-target"),
			Attr("aria-label", props.Aria), Title(props.Aria),
			OnClick(Prevent(func() {
				if onPick != nil {
					onPick()
				}
			})),
			ui.Icon(props.Icon, ClassStr(tw.Fold(tw.W4, tw.H4, tw.ShrinkO))),
			Span(css.Class(tw.Flex1), props.Label),
			// The slot the incoming screen inherits. Shown because the promise being
			// made is about this number, not about the row.
			Span(css.Class("nav-alt-hint"), Attr("aria-hidden", "true"), Text(props.Slot)),
		),
	)
}

// boolAttr renders a Go bool as the string an ARIA state attribute expects.
func boolAttr(b bool) string {
	if b {
		return "true"
	}
	return "false"
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
		Title(props.Label), // native tooltip
		Attr("aria-label", strings.TrimSpace(props.Label+" "+props.AriaSuffix)), // C315: explicit accessible name (title alone is unreliable for SR)
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
		// The RAW route alongside the href. The href carries whatever prefix
		// RoutePath adds, so it cannot be compared against the paths the
		// favorites list stores — the keyboard reorder reads this instead of
		// parsing the prefix back off.
		args = append(args, Attr("href", uistate.RoutePath(path)), Attr("data-path", path))
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
	// The jump badge. It used to be a POSITION — the item's index in the primary
	// nav — which meant the digit shown was a fact about the menu's order rather
	// than about anything the user chose. It is now the pinned slot, so the badge
	// and the key always agree, and rows without a pin show nothing.
	if props.Slot != "" {
		args = append(args, Span(css.Class("nav-alt-hint"),
			// Decorative: the link's accessible name already states the shortcut
			// (rail.pinnedAs), so announcing the bare digit again would read as a
			// stray number between the destination and the next row.
			Attr("aria-hidden", "true"),
			Attr("title", uistate.T("rail.jumpHint", props.Slot)),
			Text(props.Slot),
		))
	} else if props.AltHint >= 1 && props.AltHint <= 9 {
		args = append(args, Span(css.Class("nav-alt-hint"),
			Attr("aria-hidden", "true"),
			Attr("title", uistate.T("rail.jumpHint", strconv.Itoa(props.AltHint))),
			Text(strconv.Itoa(props.AltHint)),
		))
	}
	link := A(args...)
	if props.OnPin == nil {
		return link
	}
	// The pin is a SIBLING of the link, never inside it: a button nested in an
	// anchor is invalid, and nesting two controls makes the row a single confusing
	// stop for anyone navigating by keyboard or screen reader. The wrapper is
	// deliberately unpositioned — the active indicator measures the link's
	// offsetTop against the nav, and a positioned ancestor here would reparent that
	// measurement and park the highlight at the top of the rail.
	pinLabel := uistate.T("rail.pinAdd", props.Label)
	if props.Pinned {
		pinLabel = uistate.T("rail.pinRemove", props.Label)
	} else if props.PinFull {
		// Full is no longer a refusal: the control still acts, and what it does is
		// ask which slot this screen should take. The name says so.
		pinLabel = uistate.T("rail.pinFullAria", props.Label)
	}
	pinCls := "nav-pin"
	glyph := icon.Star
	if props.Pinned {
		pinCls += " is-pinned"
		glyph = icon.StarFilled
	}
	onPin := props.OnPin
	pinArgs := []any{
		ClassStr(pinCls), Type("button"),
		Attr("data-testid", "nav-pin-"+props.Path),
		// A toggle, so it reports pressed rather than renaming itself into a
		// destination. The name says which row it belongs to, because "Pin" alone
		// is thirty identical controls to a screen reader.
		Attr("aria-pressed", boolAttr(props.Pinned)),
		Attr("aria-label", pinLabel),
		Title(pinLabel),
		OnClick(Prevent(func() {
			if onPin != nil {
				onPin()
			}
		})),
		ui.Icon(glyph, ClassStr(tw.Fold(tw.W35, tw.H35, tw.ShrinkO))),
	}
	// Deliberately NOT disabled when full. Disabling it was the first design —
	// refuse the eleventh pin — and it left the one screen the user had just asked
	// for as the only one they could not reach by key, behind a control that said
	// nothing about how to get it. It now opens the swap question instead.

	return Div(css.Class("nav-row"), link, Button(pinArgs...))
}

// HouseholdCard sits at the bottom of the rail: a quiet, non-interactive
// household summary (Settings itself is the routed /settings page, reached
// from the System nav group or the top bar's ⋯ menu; the old click-to-open
// card is kept disabled below). It also renders the on-panel rail-collapse
// toggle (C20) — a small chevron button anchored at the top-right of the
// footer area so users can collapse/expand the rail from within the panel
// rather than relying solely on the top-bar menu button.
func HouseholdCard() uic.Node {
	collapsed := uistate.UseRailCollapsed()
	// The quiet household summary reads live data (member count, base currency)
	// — subscribe to the shared revision so a settings change updates it live.
	_ = uistate.UseDataRevision().Get()
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
		// The household settings card is intentionally out of the rail markup —
		// Settings moved to the top bar's ⋯ menu (goal 2026-07-05: "remove the flip
		// modal settings from the side nav bar"). The markup is kept here, disabled,
		// so restoring the rail entry point is a one-line change.
		If(false, Button(
			css.Class("hh", tw.Mt3, tw.Mb3, tw.P3, tw.Rounded4, tw.Border, tw.BorderLine, tw.Flex, tw.ItemsCenter, tw.Gap25, tw.TextLeft, tw.HoverBgHover, tw.WFull),
			// Tooltip/accessible name — keeps the "Settings" affordance (the gear icon
			// signals it visually) without repeating it in the visible summary line.
			Title(name+" · "+summary+" · "+uistate.T("household.settings")),
			Attr("aria-label", name+" · "+summary+" · "+uistate.T("household.settings")), // C315: explicit SR name
			OnClick(func() { uistate.OpenGlobalSettings() }),
			ui.Icon(icon.Settings, css.Class(tw.ShrinkO, tw.W4, tw.H4, tw.TextDim)),
			Span(css.Class("hh-text", tw.LeadingTight),
				Span(css.Class(tw.FontDisplay, tw.Text14, tw.FontMedium, tw.Block), name),
				Span(css.Class(tw.TextXs, tw.TextFaint, tw.Block), summary),
			),
		)),
		// The footer used to carry the household name, the member/currency summary,
		// the privacy line, an About link and the version — four lines of text that
		// never changed, permanently occupying the bottom of a rail whose menu had
		// just grown a Pinned section and five folders. All of it moved onto the
		// workspace control at the top, which is the thing that actually answers
		// "which household am I in": the summary reads on the trigger, the rest is
		// one click away inside its menu. What is left here is the collapse toggle,
		// which is a control rather than a caption.
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
	curPath := props.ActivePath
	onDashboard := curPath == "/"
	// The time-resolution control only makes sense where there's a period concept;
	// on Members/Categories/Rules/etc. it does nothing, so hide it there (C4).
	//
	// /transactions is NOT one of them (C560). The pill stores a snapped
	// period.Window, which cannot express what the ledger's date filter can — a
	// single day from a calendar click, a hand-typed range — so the two states could
	// never be reconciled: the pill read "Jul 2026" over August rows and stepping it
	// changed neither the rows, the totals, nor the calendar. The ledger now carries
	// its own period control (screens.txnScopeBar), driven by the same From/To it
	// filters on, so the label it shows cannot disagree with what it lists.
	periodAware := map[string]bool{
		"/": true, "/budgets": true, "/planning": true, "/insights": true, "/reports": true,
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
		// UX-04: no "Dashboard ›" prefix — the rail/bottom bar already own "go
		// home", and the crumb was the first thing squeezing the title into
		// "Dashboa…". The page title stands alone and gets first claim on space.
		Div(css.Class("tb-title", tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FontDisplay, tw.MinW0),
			// The current page's title is the screen's single <h1> — so every screen
			// has exactly one top-level heading for screen-reader heading navigation.
			H1(css.Class(tw.TextLg, tw.FontSemibold, tw.Truncate), Attr("aria-current", "page"), props.Title),
		),
		// Context zone: the view's scope (member) and period, LEFT-ANCHORED beside the
		// title (the actions zone owns the row's slack via margin-left:auto). Order is
		// permanent-controls-first — scope, then period — with the transient status
		// chips (offline, sample data) LAST, so a chip appearing or disappearing never
		// shifts the controls the user aims for.
		Div(css.Class("tb-context", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.MinW0, tw.TextDim, tw.Text13),
			uic.CreateElement(MemberSwitcher),
			// QA CF-24: on /reports the compact month pill silently controls a rolling
			// ANNUAL window — the prop makes the pill say so ("Year ending Jul 2026").
			If(periodAware, uic.CreateElement(ResolutionControl, resolutionControlProps{YearEnding: curPath == "/reports"})),
			uic.CreateElement(OfflineIndicator),
			// Sample-data status chip (audit P0): lives in the bar's context zone —
			// "Sample data · Start fresh" — instead of a banner row above content.
			uic.CreateElement(SampleDataBanner),
			// DP-header refinement (2026-07-19): the freshness/activity stamp is a
			// low-frequency, ambient control — relocated into the ⋯ More overflow so
			// the context strip reads as just scope + period. Still mounted (its ticker
			// runs) and clickable, only demoted; see MoreMenu.
		),
		// Actions zone: stays on the title row at every size.
		Div(css.Class("tb-actions", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.TextDim, tw.Text13),
			// Secondary, low-frequency app toggles. Permanently folded into the "More"
			// menu at every width (.topbar-secondary is display:none — the DP-header
			// refinement demoted them; the ⋯ menu is their only surface). They stay
			// mounted here so stateful ones (e.g. MuzakToggle's player effect) keep
			// running.
			Span(css.Class("topbar-secondary", tw.Flex, tw.ItemsCenter, tw.Gap25),
				If(onDashboard, uic.CreateElement(DashCustomizeButton)),
				uic.CreateElement(ThemeToggle),
				uic.CreateElement(HelpButton),
			),
			// Lock-now button — shown only when an app-lock passcode is set, so the app
			// can be locked in one click from anywhere.
			uic.CreateElement(LockToggle),
			// The music/audio toggle lives INLINE (Cam 2026-07-19: one-click un/mute
			// beats a two-click trip through ⋯ More — mute is a reflex action). Its
			// player effect runs from this single mounted instance. The Smart-insights
			// peek and activity stamp remain in the ⋯ More overflow.
			uic.CreateElement(MuzakToggle),
			// Cloud-sync liveness: quiet and dim at rest, one flash per real sync
			// (see SyncPulse). Sits with the other icon controls rather than in the
			// context strip because it is clickable — the strip's contract is that
			// nothing there moves what the user is aiming for.
			uic.CreateElement(SyncPulse),
			uic.CreateElement(NotifyBell),
			uic.CreateElement(AddMenu),
			// The "⋯ More" overflow menu sits last, against the right edge. It now hosts
			// the relocated ambient controls (activity/history, Smart peek, music).
			uic.CreateElement(MoreMenu, moreMenuProps{OnDashboard: onDashboard, ActivePath: curPath}),
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
		// Re-base a saved custom theme that disagrees with the new mode — same
		// rule as the /settings Appearance segmented (the theme's luminance is
		// what actually paints the shell).
		uistate.SyncThemeToMode(np)
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

// NotifyBell is the top-bar bell that opens the Notification Center, badged with
// how many visible items arrived since the center was last VIEWED (QA CF-04:
// the badge used to count unread and the center bulk-marked everything read on
// open to calm it — which destroyed per-item triage state; the last-seen stamp
// calms the bell without touching read flags).
func NotifyBell() uic.Node {
	feed := uistate.UseNotifyFeed().Get()
	lastSeen := uistate.UseNotifyLastSeen().Get()
	// C159: count over the VISIBLE feed (snoozed items are hidden in the
	// Notification Center), so the badge matches what the user actually sees when
	// they open it — previously a snoozed-but-unread item inflated the badge.
	visible := uistate.VisibleFeed(feed, time.Now().Unix())
	// The badge advertises only the "Needs you" bucket (UI/UX task #24): it
	// counted the WHOLE feed (17) while the page lands on the Needs-you tab
	// (14), so the number clicked never matched the number arrived at. Calm
	// Watching material no longer inflates the bell.
	needsYou, _ := uistate.PartitionTriage(visible)
	fresh := len(uistate.NewSinceLastSeen(needsYou, lastSeen))
	if lastSeen == 0 {
		// First-ever open: everything is "new"; fall back to unread so a fresh
		// install still advertises the seeded feed sensibly.
		fresh = uistate.UnreadNotifyCount(needsYou)
	}
	nav := router.UseNavigate()
	open := uic.UseEvent(func() { nav.Navigate(uistate.RoutePath("/notifications")) })
	badge := Fragment()
	if fresh > 0 {
		label := fmt.Sprintf("%d", fresh)
		if fresh > 99 {
			label = "99+" // exact counts up to two digits (QA CF-04 flagged "9+" vs the page's 16)
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

// LockToggle is the top-bar lock button (beside the music toggle). One click locks
// the app — showing the passcode gate — from anywhere. The lock button is only
// rendered when an app-lock passcode is set (locking is meaningless otherwise),
// appearing/disappearing live as the lock is added/removed. Same icon-button
// styling as the music toggle.
//
// Structure: a stable wrapper <span> always occupies this slot, and the lock button
// is conditionally rendered INSIDE it. Returning a bare Fragment when disabled — vs
// a Button when enabled — makes the component flip between zero and one node, which
// shifts its position in the reconciler's positional child list (the button ended up
// at the far right instead of beside the mute toggle). The always-present wrapper
// pins the slot while still keeping the icon out of the DOM when the lock is off; it
// is display:none when empty so it contributes no flex gap. The button node (and thus
// its OnClick hook) is constructed unconditionally — only its inclusion is gated — so
// the hook stays at a stable position across renders.
func LockToggle() uic.Node {
	// Re-render as the shell does (e.g. when Settings closes after enabling/removing
	// the lock) so the button shows/hides without a reload.
	_ = uistate.UseDataRevision().Get()
	enabled := loadAppLock().Enabled
	lock := func() { showAppLockGate() }
	lockBtn := Button(ClassStr("muzak-btn"), Type("button"),
		Attr("title", uistate.T("applock.cmdLock")),
		Attr("aria-label", uistate.T("applock.cmdLock")),
		Attr("data-testid", "topbar-lock-btn"),
		OnClick(lock),
		ui.Icon(icon.Lock, css.Class(tw.W18px, tw.H18px)),
	)
	slot := []any{ClassStr("lock-toggle-slot")}
	if !enabled {
		slot = append(slot, Attr("style", "display:none"))
	}
	return Span(append(slot, If(enabled, lockBtn))...)
}

// resolutionControlProps configures the period pill per page. YearEnding marks
// pages (the /reports annual review) where stepping the month shifts a rolling
// twelve-month window — the pill then reads "Year ending Jul 2026" so the
// control's true scope is self-evident (QA CF-24: a bare month pill beside an
// annual heading read as a broken monthly filter).
type resolutionControlProps struct {
	YearEnding bool
}

// ResolutionControl is the top bar's time-resolution control. The common case is
// a single period: a Week/Month/Quarter granularity toggle and one stepper that
// pages the whole window (‹ Jun 2026 ›). When the view has moved off the current
// period a "This period" reset appears; a "Custom range" toggle opens an explicit
// range workflow with its own draft, preview and Apply. All date math lives in
// internal/period — every action just stores the next immutable Window.
//
// C589 — why the range is a DRAFT, not a live edit. "Custom range" used to be a
// mode flag: clicking it relabelled the pill "Jul 2026 – Jul 2026" while the
// window was still one month (a label describing a range nobody had chosen yet),
// and each stepper click then mutated the live view, so every intermediate state
// of "June through September" was applied and re-queried on the way there. The
// mode also lived in component state, so navigating away lost it and an existing
// range could not be edited again without hunting.
//
// Now: the editor is open whenever the window IS a range or the user asked for
// one; the steppers move a draft; the pill keeps describing what is actually
// applied until Apply commits it; and Cancel leaves the view exactly as it was.
func ResolutionControl(props resolutionControlProps) uic.Node {
	atom := uistate.UsePeriod()
	w := atom.Get()
	open := uic.UseState(false)
	// rangeAsked is only the user's REQUEST to see the range editor. Whether the
	// editor shows is derived below from that plus the window itself, so an
	// applied range always brings its own editor back — no hidden mode.
	rangeAsked := uic.UseState(false)
	// The uncommitted range. Zero anchors mean "not started"; the editor seeds it
	// from the live window the first time it renders.
	draftFrom := uic.UseState(time.Time{})
	draftTo := uic.UseState(time.Time{})
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
					uistate.SetPeriod(atom, period.NewWindow(w.Res, now, w.WeekStart))
				case "last":
					uistate.SetPeriod(atom, period.Previous(w.Res, now, w.WeekStart))
				case "quarter":
					uistate.PersistResolution(period.Quarter)
					uistate.SetPeriod(atom, period.NewWindow(period.Quarter, now, w.WeekStart))
				case "ytd":
					uistate.PersistResolution(period.Month)
					uistate.SetPeriod(atom, period.YearToDate(now, w.WeekStart))
				case "lastyear":
					uistate.PersistResolution(period.Year)
					uistate.SetPeriod(atom, period.PriorYear(now, w.WeekStart))
				}
				closeMenu()
			}),
			label)
	}

	// The pill describes what is APPLIED — a single period, or the real range —
	// and never the range being drafted. On a year-ending page the single-period
	// label says what stepping really does.
	pillLabel := w.Label()
	if props.YearEnding {
		pillLabel = uistate.T("resolution.yearEnding", w.Label())
	}
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	hidden := ""
	if !open.Get() {
		hidden = " hidden-menu"
	}

	// The editor is open when the user asked for it OR the window already is a
	// range — so an applied range is always editable, and the mode can never be
	// lost behind a navigation while its effect stays on screen.
	rangeOpen := rangeAsked.Get() || !w.IsSinglePeriod()
	// Seed the draft from the live window the first time the editor renders.
	dFrom, dTo := draftFrom.Get(), draftTo.Get()
	if rangeOpen && (dFrom.IsZero() || dTo.IsZero()) {
		dFrom, dTo = w.From, w.To
	}
	draft := period.Window{Res: w.Res, From: dFrom, To: dTo, WeekStart: w.WeekStart}
	// liveDraft rebuilds the draft from CURRENT state, for use inside handlers.
	//
	// A handler closes over the render that created it, and this control re-renders
	// on every draft change; a click landing before the new handlers are installed
	// would otherwise step from a stale base — two fast clicks on "later" moving the
	// endpoint once, or Apply committing the previous draft. Reading state at click
	// time removes the window entirely rather than narrowing it.
	liveDraft := func() period.Window {
		cur := atom.Get()
		f, t := draftFrom.Get(), draftTo.Get()
		if f.IsZero() || t.IsZero() {
			f, t = cur.From, cur.To
		}
		return period.Window{Res: cur.Res, From: f, To: t, WeekStart: cur.WeekStart}
	}
	setDraft := func(next period.Window) {
		draftFrom.Set(next.From)
		draftTo.Set(next.To)
	}
	clearDraft := func() {
		draftFrom.Set(time.Time{})
		draftTo.Set(time.Time{})
	}
	dirty := rangeOpen && !dFrom.IsZero() && (!dFrom.Equal(w.From) || !dTo.Equal(w.To))

	// The range editor: both endpoints, the range it describes in words, what it
	// does and does not change, and explicit Apply / Cancel. Nothing here touches
	// the live window until Apply.
	rangeRow := Fragment()
	if rangeOpen {
		rangeRow = Div(css.Class("period-rangeedit"), Attr("data-testid", "period-range-editor"),
			Div(css.Class("period-rangerow", tw.Flex, tw.ItemsCenter, tw.Gap25, tw.FlexWrap),
				ui.StepperPill(ui.StepperPillProps{Label: draft.FromLabel(), OnPrev: func() { setDraft(liveDraft().StepFrom(-1)) }, OnNext: func() { setDraft(liveDraft().StepFrom(1)) }, PrevLabel: uistate.T("resolution.fromEarlier"), NextLabel: uistate.T("resolution.fromLater")}),
				Span(css.Class(tw.TextFaint), "–"),
				ui.StepperPill(ui.StepperPillProps{Label: draft.ToLabel(), OnPrev: func() { setDraft(liveDraft().StepTo(-1)) }, OnNext: func() { setDraft(liveDraft().StepTo(1)) }, PrevLabel: uistate.T("resolution.toEarlier"), NextLabel: uistate.T("resolution.toLater")}),
			),
			// The range in one sentence, so the user reads what they are about to
			// apply rather than inferring it from two pills.
			P(css.Class("period-rangepreview"), Attr("data-testid", "period-range-preview"),
				rangePreviewText(draft)),
			P(css.Class("period-rangenote"), Attr("data-testid", "period-range-note"),
				uistate.T("resolution.rangeScopeNote")),
			Div(css.Class("period-rangeacts", tw.Flex, tw.ItemsCenter, tw.Gap15, tw.FlexWrap),
				Button(css.Class("btn btn-primary btn-sm"), Type("button"), Attr("data-testid", "period-range-apply"),
					attrDisabledIf(!dirty),
					OnClick(func() {
						uistate.SetPeriod(atom, liveDraft())
						rangeAsked.Set(true)
					}),
					uistate.T("resolution.rangeApply")),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "period-range-cancel"),
					OnClick(func() {
						clearDraft()
						rangeAsked.Set(false)
						// Read the LIVE window, not the one this closure captured when it
						// rendered. Apply re-renders the control, and a click landing before
						// that render's handlers replace these ones would otherwise test a
						// window from before the range existed, conclude there was nothing
						// to collapse, and leave the range applied with its editor gone.
						if cur := atom.Get(); !cur.IsSinglePeriod() {
							uistate.SetPeriod(atom, cur.Single())
						}
					}),
					uistate.T(rangeCancelKey(w, dirty))),
			),
		)
	}

	// A single compact control: ‹ [period ⌄] › — the chevrons page the window; the
	// center pill opens a popover with the granularity, quick jumps and custom range.
	return Div(css.Class("period-control add-wrap"), Attr("id", menuID),
		Button(css.Class("period-step"), Type("button"), Attr("aria-label", uistate.T("resolution.prevPeriod")), Attr("title", uistate.T("resolution.prevPeriod")),
			OnClick(func() { uistate.SetPeriod(atom, w.Shift(-1)) }), ui.Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4))),
		Button(css.Class("period-pill"), Type("button"), Attr("aria-haspopup", "menu"), Attr("aria-expanded", expanded),
			Attr("data-testid", "period-pill"), Attr("title", uistate.T("resolution.jumpTo")),
			OnClick(func() { open.Set(!open.Get()) }),
			Span(css.Class("period-label"), pillLabel),
			ui.Icon(icon.ChevronDown, css.Class("period-caret", tw.W3, tw.H3)),
		),
		Button(css.Class("period-step"), Type("button"), Attr("aria-label", uistate.T("resolution.nextPeriod")), Attr("title", uistate.T("resolution.nextPeriod")),
			OnClick(func() { uistate.SetPeriod(atom, w.Shift(1)) }), ui.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))),
		Div(ClassStr("add-backdrop"+hidden), OnClick(closeMenu)),
		Div(ClassStr("period-pop add-menu open-left"+hidden), Attr("role", "menu"),
			ui.Segmented(ui.SegmentedProps{
				Label: uistate.T("resolution.granularity"), // C318: name the radiogroup
				Options: []ui.SegOption{
					{Value: string(period.Week), Label: uistate.T("period.week")},
					{Value: string(period.Month), Label: uistate.T("period.month")},
					{Value: string(period.Quarter), Label: uistate.T("period.quarter")},
					{Value: string(period.Year), Label: uistate.T("period.year")},
				},
				Selected: string(w.Res),
				OnSelect: func(v string) {
					r := period.Resolution(v)
					uistate.PersistResolution(r)
					uistate.SetPeriod(atom, w.SetResolution(r, time.Now()))
				},
			}),
			Div(css.Class("period-presets", tw.Flex, tw.FlexWrap, tw.Gap15),
				preset(uistate.T("resolution.presetThis"), "this"),
				preset(uistate.T("resolution.presetLast"), "last"),
				preset(uistate.T("resolution.presetQuarter"), "quarter"),
				preset(uistate.T("resolution.presetYTD"), "ytd"),
				preset(uistate.T("resolution.presetPriorYear"), "lastyear"),
			),
			// The editor's own Cancel closes it, so the toggle only ever OPENS the
			// range workflow — one control, one direction, nothing to mis-read.
			If(!rangeOpen, Button(css.Class("period-rangetoggle"), Type("button"),
				Attr("data-testid", "period-range-open"),
				Attr("aria-expanded", "false"),
				OnClick(func() {
					draftFrom.Set(w.From)
					draftTo.Set(w.To)
					rangeAsked.Set(true)
				}),
				uistate.T("resolution.customRange"))),
			rangeRow,
		),
	)
}

// attrDisabledIf returns the disabled attribute when cond holds, else a no-op —
// the empty-string `disabled` attribute still disables in HTML, so it has to be
// omitted rather than set to a falsy value.
func attrDisabledIf(cond bool) any {
	if cond {
		return Attr("disabled", "disabled")
	}
	return Fragment()
}

// rangePreviewText states the drafted range in words — the sentence a user reads
// before pressing Apply (C589). A one-unit draft is not yet a range, and saying
// "Jun 2026 through Jun 2026 — 1 periods" of it is both ungrammatical and
// misleading about what would be applied.
func rangePreviewText(draft period.Window) string {
	if n := period.UnitsIn(draft); n > 1 {
		return uistate.T("resolution.rangePreview", draft.FromLabel(), draft.ToLabel(), n)
	}
	return uistate.T("resolution.rangePreviewOne", draft.FromLabel())
}

// rangeCancelKey names the range editor's secondary action for what it will
// actually do. With unsaved edits it discards them; with an applied range and
// nothing pending it is the way back to a single period; with neither it just
// closes the editor. One button, three honest labels — better than a "Single
// period" toggle that sometimes discarded a draft and sometimes changed the view.
func rangeCancelKey(w period.Window, dirty bool) string {
	switch {
	case dirty:
		return "resolution.rangeDiscard"
	case !w.IsSinglePeriod():
		return "resolution.rangeBackToSingle"
	}
	return "resolution.rangeClose"
}
