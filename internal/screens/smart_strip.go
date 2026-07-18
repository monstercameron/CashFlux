// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// smartStripProps selects which page's insights a strip surfaces. An empty Page
// means "all pages" — the cross-page summary used on the Dashboard.
type smartStripProps struct {
	Page smart.Page
}

// stripCap is the most insight cards a per-page strip shows inline; the full set
// lives on the /smart hub. Kept small so the strip stays glanceable, never a wall.
const stripCap = 3
const stripCapAll = 4 // the Dashboard cross-page strip can show a couple more

// SmartStrip renders a page's enabled, active Free-engine insights inline as a
// compact, glanceable strip — the SMART layer woven into each page rather than
// only on the hub. It is strictly additive: when the user has enabled nothing for
// the page, or there are no active insights, it renders nothing at all (no
// header, no spacing), so a page is untouched until the user opts in.
//
// It is its own component so its state hooks sit at stable positions. AI features
// (interactive ask/scan controls) stay on the /smart hub; this strip is the
// passive, glanceable half.
func SmartStrip(props smartStripProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	rev := uistate.UseDataRevision()
	_ = rev.Get() // re-render on data/settings/dismiss/snooze change
	nav := router.UseNavigate()
	pr := uistate.UsePrefs().Get()

	settings := uistate.LoadSmartSettings()
	// Panel-level snooze: the whole strip is hidden until the snooze expires.
	if settings.IsSnoozed(time.Now().Unix()) {
		return Fragment()
	}
	in := buildSmartInput(app, pr.WeekStartWeekday())

	var insights []smart.Insight
	limit := stripCap
	if props.Page == "" {
		insights = smartengine.Run(in, settings)
		limit = stripCapAll
	} else {
		insights = smartengine.RunPage(in, settings, props.Page)
	}

	// The page's enabled AI features surface inline as run-controls (the
	// click-before-run analysis on the page itself, not only the hub). Only on a
	// concrete page, not the Dashboard cross-page summary.
	var aiFeats []smart.Feature
	backendAI := pr.Normalize().BackendActive()
	if props.Page != "" {
		aiFeats = enabledPageAIFeatures(settings, props.Page)
	}

	if len(insights) == 0 && len(aiFeats) == 0 {
		return Fragment() // additive: nothing enabled or nothing to say → no footprint
	}
	total := len(insights)
	// Capture every active insight's key BEFORE truncation so "dismiss all" clears
	// the whole batch, not just the few shown inline.
	allKeys := make([]string, 0, len(insights))
	for _, ins := range insights {
		allKeys = append(allKeys, ins.Key)
	}
	if len(insights) > limit {
		insights = insights[:limit]
	}

	openHub := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/smart")) })

	// R38 (§3.1/§8.6): lead with the single most-severe insight (insights are
	// severity-sorted); the rest expand in place on request, so the strip is a
	// glanceable decision layer, not a stack that buries the page's primary content.
	// Collapse resets per page (the strip is keyed by path) → decision-first default.
	expanded := ui.UseState(false)
	toggleExpand := ui.UseEvent(func() { expanded.Set(!expanded.Get()) })

	// Collapsed by DEFAULT (audit P0 "reduce persistent vertical chrome"): the
	// in-page strip renders NOTHING until the top bar's Smart trigger (icon +
	// count — see SmartPeekForPath) opens it, so pages carry zero insight chrome
	// unless asked. The shared atom resets to collapsed on navigation.
	open := uistate.UseSmartStripOpen()
	collapse := ui.UseEvent(func() { open.Set(false) })

	pageKey := string(props.Page)
	if pageKey == "" {
		pageKey = "all"
	}
	// Render into a STABLE wrapper element (smartStripSlot) whose type never
	// changes between the collapsed (empty) and open (card) states, so GWC's
	// reconciler swaps only the inner child and the card opens in place at the
	// top of the content column.
	if !open.Get() {
		return smartStripSlot(pageKey, Fragment())
	}

	// "View all (N)" link in the card header — the count suffix only when more exist
	// than shown inline.
	viewAllLabel := uistate.T("smart.viewAll")
	if total > len(insights) {
		viewAllLabel += " (" + itoaStrip(total) + ")"
	}
	viewAllBtn := Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
		Attr("data-testid", "smart-strip-viewall"),
		Attr("aria-label", uistate.T("smart.viewAll")),
		OnClick(openHub),
		viewAllLabel,
	)

	// Panel actions overflow menu: snooze the whole strip, or dismiss the current
	// batch of nudges at once (complementing the per-nudge dismiss on each card).
	menuItems := make([]uiw.OverflowMenuItem, 0, 3)
	if len(allKeys) > 0 {
		menuItems = append(menuItems, uiw.OverflowMenuItem{
			Label:    uistate.T("smart.dismissAll"),
			TestID:   "smart-dismiss-all",
			OnSelect: func() { uistate.DismissAllSmartInsights(allKeys); rev.Set(rev.Get() + 1) },
		})
	}
	menuItems = append(menuItems,
		uiw.OverflowMenuItem{
			Label:    uistate.T("smart.snoozeDay"),
			TestID:   "smart-snooze-day",
			OnSelect: func() { uistate.SnoozeSmartPanel(time.Now().Add(24 * time.Hour)); rev.Set(rev.Get() + 1) },
		},
		uiw.OverflowMenuItem{
			Label:    uistate.T("smart.snoozeWeek"),
			TestID:   "smart-snooze-week",
			OnSelect: func() { uistate.SnoozeSmartPanel(time.Now().Add(7 * 24 * time.Hour)); rev.Set(rev.Get() + 1) },
		},
	)
	panelMenu := uiw.OverflowMenu(uiw.OverflowMenuProps{
		Items:         menuItems,
		TriggerLabel:  uistate.T("smart.panelActions"),
		TriggerTestID: "smart-strip-menu",
	})

	collapseBtn := Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
		Attr("data-testid", "smart-strip-collapse"), Attr("aria-label", uistate.T("smart.collapse")),
		Title(uistate.T("smart.collapse")), OnClick(collapse),
		uiw.Icon(icon.ChevronUp, css.Class(tw.W35, tw.H35)))
	headerAction := Div(ClassStr(tw.Fold(tw.Flex, tw.ItemsCenter, tw.Gap1)), viewAllBtn, collapseBtn, panelMenu)

	// Body: the Free insight cards, then the page's AI run-controls (gated on a
	// configured provider — an honest hint instead of dead controls otherwise).
	var bodyParts []any
	shown := insights
	if !expanded.Get() && len(insights) > 1 {
		shown = insights[:1] // collapsed: just the top insight
	}
	if len(shown) > 0 {
		// Flat list rows (not bordered cards) so the strip reads as one panel of
		// insights, consistent with the dashboard's flat tiles rather than nesting
		// cards inside the strip card.
		bodyParts = append(bodyParts, smartStripList(shown))
	}
	// In-place expand/collapse for the remaining inline insights (distinct from
	// "View all" which navigates to the /smart hub for the complete catalog).
	if len(insights) > 1 {
		ariaExp := "false"
		label := fmt.Sprintf(uistate.T("smart.stripMore"), len(insights)-1)
		if expanded.Get() {
			ariaExp, label = "true", uistate.T("smart.stripLess")
		}
		bodyParts = append(bodyParts, Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-strip-toggle"), Attr("aria-expanded", ariaExp),
			OnClick(toggleExpand), label))
	}
	if len(aiFeats) > 0 {
		conn := resolveAIConn(app, backendAI, pr.ServerURL, pr.ServerToken)
		if aiProviderConfigured(app, backendAI) {
			bodyParts = append(bodyParts,
				Div(ClassStr(tw.Fold(tw.FlexCol, tw.Gap3)),
					MapKeyed(aiFeats,
						func(f smart.Feature) any { return f.Code },
						func(f smart.Feature) ui.Node { return smartAIFeatureNode(f, conn) },
					),
				),
			)
		} else {
			bodyParts = append(bodyParts,
				P(ClassStr(tw.Fold(tw.Text13, tw.TextDim)), uistate.T("smart.aiNeedsProvider")),
			)
		}
	}

	// On the Dashboard (cross-page summary, Page==""), the strip sits with the bento
	// grid, so it drops the rounded-card look for square corners + the grid gap to
	// match the .w tiles. On concrete pages it stays the normal rounded card.
	classParts := []any{tw.Mb3}
	if props.Page == "" {
		classParts = []any{"smart-strip-bento"}
	}
	bodyArgs := append([]any{ClassStr(tw.Fold(tw.FlexCol, tw.Gap3))}, bodyParts...)
	return smartStripSlot(pageKey, uiw.Card(uiw.CardProps{
		Header:     smartBrandHeader(uistate.T("smart.stripTitle"), false, headerAction),
		TestID:     "smart-strip-" + pageKey,
		ClassParts: classParts,
		Body:       Div(bodyArgs...),
	}))
}

// smartStripSlot wraps the strip's content (collapsed peek or open card) in a
// single stable <div> so the SmartStrip component's ROOT element type never changes
// between renders. GWC's reconciler re-anchors a node when a component's root
// element type flips (here <button>↔<div>): opening the peek orphaned the card to
// the bottom of the page. Keeping the root a <div> and swapping only the inner child
// keeps it in place. The slot itself is layout-neutral (a plain block).
func smartStripSlot(pageKey string, inner ui.Node) ui.Node {
	return Div(css.Class("smart-strip-slot"),
		Attr("data-testid", "smart-strip-slot-"+pageKey), inner)
}

// stripPageForPath maps an app route path to the SMART page whose insights belong
// inline on it. The Dashboard ("/") maps to the empty page — the cross-page
// summary. ok is false for routes that get no strip (Settings, Members, etc.).
func stripPageForPath(path string) (smart.Page, bool) {
	switch path {
	case "/":
		return "", true // cross-page summary on the Dashboard
	case "/accounts":
		return smart.PageAccounts, true
	case "/transactions":
		return smart.PageTransactions, true
	case "/budgets":
		return smart.PageBudgets, true
	case "/goals":
		return smart.PageGoals, true
	case "/todo":
		return smart.PageTodos, true
	case "/planning":
		return smart.PagePlanning, true
	case "/allocate":
		return smart.PageAllocate, true
	case "/subscriptions":
		return smart.PageSubscriptions, true
	case "/bills":
		return smart.PageBills, true
	}
	return "", false
}

// SmartStripForPath renders the inline Smart strip for the given route, or
// nothing when the route has no associated SMART page. It is the single entry
// point the app Shell calls to intersperse insights above every relevant page —
// one wiring point instead of editing each screen. Strictly additive: it renders
// nothing unless the user has enabled features that produce active insights for
// the page.
func SmartStripForPath(path string) ui.Node {
	page, ok := stripPageForPath(path)
	if !ok {
		return Fragment()
	}
	return ui.CreateElement(SmartStrip, smartStripProps{Page: page})
}

// SmartPeekForPath renders the top bar's compact Smart trigger for the given
// route: the sparkle glyph plus an insight-count badge (audit P0 — the Smart
// layer integrates into the title row as an icon/count instead of spending a
// row above every page's content). Renders nothing when the route has no SMART
// page, the strip is snoozed, or nothing is enabled/active — strictly additive,
// exactly like the strip itself. Clicking toggles the shared open atom that the
// in-page strip card reads.
func SmartPeekForPath(path string) ui.Node {
	// Always mount the component — even on routes with no SMART page — so the
	// trigger occupies a stable child position in the top bar's actions row (the
	// zero↔one node flip would shift its siblings in the positional child list).
	return ui.CreateElement(smartPeekTrigger, smartPeekProps{Path: path})
}

// smartPeekProps carries the active route into the top-bar Smart trigger.
type smartPeekProps struct {
	Path string
}

// smartPeekTrigger is the top-bar Smart trigger component. It runs the same
// Free-engine pass the strip does to know whether (and how urgently) to show
// itself; when the strip is open it stays mounted with aria-expanded="true" so
// it reads as the toggle it is.
func smartPeekTrigger(props smartPeekProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get() // re-render on data/settings/dismiss/snooze change
	pr := uistate.UsePrefs().Get()
	open := uistate.UseSmartStripOpen()
	toggle := ui.UseEvent(func() { open.Set(!open.Get()) })

	page, ok := stripPageForPath(props.Path)
	if !ok {
		return Fragment()
	}
	settings := uistate.LoadSmartSettings()
	if settings.IsSnoozed(time.Now().Unix()) {
		return Fragment()
	}
	in := buildSmartInput(app, pr.WeekStartWeekday())
	var insights []smart.Insight
	if page == "" {
		insights = smartengine.Run(in, settings)
	} else {
		insights = smartengine.RunPage(in, settings, page)
	}
	var aiFeats []smart.Feature
	if page != "" {
		aiFeats = enabledPageAIFeatures(settings, page)
	}
	if len(insights) == 0 && len(aiFeats) == 0 {
		return Fragment()
	}

	pageKey := string(page)
	if pageKey == "" {
		pageKey = "all"
	}
	alerts := len(insights)
	// A clear noun + a 9+-capped badge (like the notify bell) so it reads as "a
	// few insights to look at", never an alarming raw count.
	title := uistate.T("smart.peekInsights")
	aria := uistate.T("smart.peekToolsAria")
	var badge ui.Node = Fragment()
	if alerts > 0 {
		badgeText := itoaStrip(alerts)
		if alerts > 9 {
			badgeText = "9+"
		}
		var sev smart.Severity
		if len(insights) > 0 {
			sev = insights[0].Severity
		}
		badge = Span(ClassStr("smart-peek-badge "+tw.ColorClass(severityTone(sev))), badgeText)
		// The accessible name mirrors the capped badge ("9+"), not the raw count —
		// a header reading "249 smart insights" turns a glanceable surface into a
		// backlog (2026-07-18 assessment); the full set stays on the Smart hub.
		aria = fmt.Sprintf(uistate.T("smart.peekAlertsAria"), badgeText)
	} else {
		title = uistate.T("smart.peekTools")
	}
	expanded := "false"
	if open.Get() {
		expanded = "true"
	}
	return Button(css.Class("smart-peek smart-peek-tb"), Type("button"),
		Attr("data-testid", "smart-peek-"+pageKey), Attr("aria-expanded", expanded),
		Attr("aria-label", aria), Title(aria+" · "+title), OnClick(toggle),
		smartGlyph(false, tw.Fold(tw.W35, tw.H35)),
		badge,
		uiw.Icon(icon.ChevronDown, css.Class("smart-peek-chev", tw.W3, tw.H3)),
	)
}

// itoaStrip formats a small non-negative count for the "(N)" badge.
func itoaStrip(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
