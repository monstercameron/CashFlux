// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens holds the CashFlux screen registry.
//
// This file implements AssistantHub — the tabbed /assistant page that merges
// /insights (AI chat) and /smart (Free-engine cards + catalog) into a single
// surface with three distinct panels:
//
//   - Ask      — the full AI chat surface (delegates to the Insights component)
//   - Insights — spending highlights, top merchants, monthly trend chart,
//                anomaly flags, and pinned insights — no chat
//   - Smart    — the Free-insight cards, AI feature panel, and manage catalog
//                (delegates to the SmartHub component)
//
// The route /assistant is NOT registered here; a later rail-regroup commit
// wires it into screens.All(). Existing /insights and /smart routes remain
// intact and continue to render their own screens.
package screens

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/scope"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Assistant returns the top-level node for the /assistant hub. It is the
// View func the route registry will reference in a later commit.
func Assistant() ui.Node {
	return ui.CreateElement(AssistantHub)
}

// AssistantHub is the registered component for the /assistant page. It owns
// the tab state and renders one of three isolated panel components — Ask,
// Insights, or Smart — via ui.CreateElement so each panel's hook tree is
// fully isolated from the tab-switching logic (GWC hook rule: no On* /
// UseState in variable-length loops or conditional parent positions).
//
// Tab defaults to "ask" on first mount so returning users land in the AI
// chat straight away.
func AssistantHub() ui.Node {
	activeTab := ui.UseState("ask")

	tabBar := Div(css.Class(tw.Mt2, tw.Mb3),
		uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("assistant.tabGroupLabel"),
			Selected: activeTab.Get(),
			OnSelect: func(v string) { activeTab.Set(v) },
			Options: []uiw.SegOption{
				{Value: "ask", Label: uistate.T("assistant.tabAsk")},
				{Value: "insights", Label: uistate.T("assistant.tabInsights")},
				{Value: "smart", Label: uistate.T("assistant.tabSmart")},
			},
		}),
	)

	// Each branch is a ui.CreateElement call so the framework allocates a
	// fresh, isolated hook tree for whichever panel is active. Switching tabs
	// unmounts the old panel and mounts the new one — hooks never bleed across.
	var body ui.Node
	switch activeTab.Get() {
	case "insights":
		body = ui.CreateElement(assistantInsightsDataPanel)
	case "smart":
		body = ui.CreateElement(SmartHub)
	default: // "ask"
		body = ui.CreateElement(Insights)
	}

	return Div(
		Attr("data-testid", "assistant-hub"),
		tabBar,
		body,
	)
}

// assistantInsightsDataPanel is the Insights tab body for AssistantHub. It
// shows spending highlights, top merchants, the monthly spending trend chart,
// anomaly flags, and pinned insights — the data visualization half of what
// /insights renders — without the AI chat surface.
//
// It reconstructs the same scoped data pipeline as Insights() so the figures
// stay in sync with the active scope. All hooks are called unconditionally at
// stable positions before any conditional logic.
func assistantInsightsDataPanel() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Hook 1: subscribe to data-revision so this panel re-renders on any
	// app state change (transactions added, insights pinned, etc.).
	_ = uistate.UseDataRevision().Get()

	// Hook 2: router navigation (also consumed by smartAnomalyHighlights
	// below, which adds its own UseNavigate call as hook 7).
	nav := router.UseNavigate()

	// Hook 3: transaction filter atom — shared with /transactions for
	// the category and merchant drill-through pattern.
	txFilterAtom := uistate.UseTxFilter()

	// Hook 4: active scope — mirrors the scope used by /insights.
	scopeAtom := uistate.UseActiveScope()

	// Hook 5: prefs — need weekStart for the anomaly detectors.
	pr := uistate.UsePrefs().Get()

	// Hook 6: local revision bump so a pinned-insight delete triggers
	// a re-render of this panel without depending on a global atom.
	rev := ui.UseState(0)
	bump := func() { rev.Set(rev.Get() + 1) }

	// ── Data setup ────────────────────────────────────────────────────────────

	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: settings.FXRates}

	accounts := app.Accounts()
	txns := app.Transactions()

	sc := scopeAtom.Get()
	instOf := func(a domain.Account) string { return a.Institution }
	scopeIDs := scope.ResolveScope(accounts, sc, instOf)
	scopedTxns := scope.ApplyScopeToTxns(txns, scopeIDs)

	cats := app.Categories()
	catsByName := categoryNameToIDMap(cats)

	// ── Drill callbacks ────────────────────────────────────────────────────────

	viewCategoryTransactions := func(catName string) {
		catID := catsByName[catName]
		f := uistate.TxFilter{Category: catID}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	viewMerchantTransactions := func(merchantName string) {
		f := uistate.TxFilter{Text: merchantName}.Normalize()
		txFilterAtom.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	// ── Panel content ──────────────────────────────────────────────────────────

	// Spending highlights: categories whose spend deviated materially from
	// their recent monthly norm. Empty when there is nothing notable.
	highlights := spendingHighlights(scopedTxns, cats, base, rates, viewCategoryTransactions)

	// Top merchants by spend over the last 90 days.
	topMerchantsCard := topMerchantsSpendCard(scopedTxns, base, rates, viewMerchantTransactions)

	// Monthly spending trend chart — last 6 months of expense outflow.
	spendTrendCard := monthlySpendingChart(scopedTxns, base, rates)

	// Hook 7 (inside smartAnomalyHighlights): anomaly detectors — duplicates,
	// spend spikes, missing charges, balance anomalies. Always shown, no
	// SMART opt-in gate required. smartAnomalyHighlights calls
	// router.UseNavigate() internally at a stable position.
	flagged := smartAnomalyHighlights(app, pr.WeekStartWeekday())

	// Pinned insights, newest first.
	pins := app.SavedInsights()
	sort.Slice(pins, func(i, j int) bool { return pins[i].CreatedAt.After(pins[j].CreatedAt) })

	deletePinned := func(pid string) {
		_ = app.DeleteSavedInsight(pid)
		bump()
	}

	pinnedCard := Fragment()
	if len(pins) > 0 {
		pinnedCard = uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("insights.pinnedTitle"),
			Rows: MapKeyed(pins,
				func(p domain.SavedInsight) any { return p.ID },
				func(p domain.SavedInsight) ui.Node {
					return ui.CreateElement(PinnedInsightRow, pinnedInsightRowProps{
						Insight:  p,
						OnDelete: deletePinned,
					})
				},
			),
		})
	}

	return Div(
		highlights,
		topMerchantsCard,
		spendTrendCard,
		flagged,
		pinnedCard,
	)
}
