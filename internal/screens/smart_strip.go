// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
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
	_ = uistate.UseDataRevision().Get() // re-render on data/settings/dismiss change
	nav := router.UseNavigate()
	pr := uistate.UsePrefs().Get()

	settings := uistate.LoadSmartSettings()
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

	// "View all (N)" link in the card header — only when more exist than shown.
	var headerAction ui.Node
	if total > len(insights) {
		headerAction = Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-strip-viewall"),
			Attr("aria-label", uistate.T("smart.viewAll")),
			OnClick(openHub),
			uistate.T("smart.viewAll")+" ("+itoaStrip(total)+")",
		)
	} else {
		headerAction = Button(css.Class("btn btn-sm btn-ghost"), Type("button"),
			Attr("data-testid", "smart-strip-viewall"),
			Attr("aria-label", uistate.T("smart.viewAll")),
			OnClick(openHub),
			uistate.T("smart.viewAll"),
		)
	}

	pageKey := string(props.Page)
	if pageKey == "" {
		pageKey = "all"
	}

	// Body: the Free insight cards, then the page's AI run-controls (gated on a
	// configured provider — an honest hint instead of dead controls otherwise).
	var bodyParts []any
	shown := insights
	if !expanded.Get() && len(insights) > 1 {
		shown = insights[:1] // collapsed: just the top insight
	}
	if len(shown) > 0 {
		bodyParts = append(bodyParts, smartInsightList(shown))
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

	bodyArgs := append([]any{ClassStr(tw.Fold(tw.FlexCol, tw.Gap3))}, bodyParts...)
	return uiw.Card(uiw.CardProps{
		Header:     smartBrandHeader(uistate.T("smart.stripTitle"), false, headerAction),
		TestID:     "smart-strip-" + pageKey,
		ClassParts: []any{tw.Mb3},
		Body:       Div(bodyArgs...),
	})
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
