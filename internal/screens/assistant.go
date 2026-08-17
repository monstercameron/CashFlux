// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens holds the CashFlux screen registry.
//
// This file implements AssistantHub — the tabbed /assistant page that merges
// /insights (AI chat) and /smart (Free-engine cards + catalog) into a single
// surface with three distinct panels:
//
//   - Ask      — the full AI chat surface (delegates to the Insights component)
//   - Insights — the agent's briefing, a widgetized bento surface: the
//     month-to-date spending story (hero), flagged activity + category shifts
//     (the attention pair), the spending trend, top merchants + pinned
//     insights, and an opt-in FormulaBuilder over the assistant_* variables
//   - Smart    — the Free-insight cards, AI feature panel, and manage catalog
//     (delegates to the SmartHub component)
package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/flagverdict"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/insightsperiod"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/scope"
	"github.com/monstercameron/CashFlux/internal/smart"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// Assistant returns the top-level node for the /assistant hub. It is the
// View func the route registry references.
func Assistant() ui.Node {
	return ui.CreateElement(AssistantHub)
}

// AssistantHub is the registered component for the /assistant page. It owns
// the tab state and renders one of three isolated panel components — Ask,
// Insights, or Automations — via ui.CreateElement so each panel's hook tree is
// fully isolated from the tab-switching logic (GWC hook rule: no On* /
// UseState in variable-length loops or conditional parent positions).
//
// The three tabs were previously Ask / Insights / Smart, which a reviewer read
// as "conceptually overlapping" — and they were, because two of the three names
// described a technology rather than a job. Each tab now answers a different
// question, stated under the bar so the difference is on the page and not just
// in the designer's head (C392):
//
//	Ask         — you have a question
//	Insights    — the app noticed something you did not ask about
//	Automations — what is running by itself
//
// Tab defaults to Ask on first mount so returning users land in the chat.
func AssistantHub() ui.Node {
	// The active tab is a shared atom so surfaces inside a tab can move the
	// user to a sibling (the Ask rail's "See all in Insights" cross-link), and
	// so the /insights and /smart routes can open this hub on the right tab
	// rather than rendering a second copy of the same surface (C359).
	activeTab := uistate.UseAssistantTab()

	tabBar := Div(css.Class(tw.Mt2, tw.Mb3),
		uiw.Segmented(uiw.SegmentedProps{
			Label:    uistate.T("assistant.tabGroupLabel"),
			Selected: activeTab.Get(),
			OnSelect: func(v string) { activeTab.Set(v) },
			Options: []uiw.SegOption{
				{Value: uistate.AssistantTabAsk, Label: uistate.T("assistant.tabAsk")},
				{Value: uistate.AssistantTabInsights, Label: uistate.T("assistant.tabInsights")},
				{Value: uistate.AssistantTabAutomations, Label: uistate.T("assistant.tabAutomations")},
			},
		}),
		P(css.Class("asst-tab-job"), Attr("data-testid", "assistant-tab-job"),
			assistantTabJob(activeTab.Get())),
	)

	// Each branch is a ui.CreateElement call so the framework allocates a
	// fresh, isolated hook tree for whichever panel is active. Switching tabs
	// unmounts the old panel and mounts the new one — hooks never bleed across.
	var body ui.Node
	switch activeTab.Get() {
	case uistate.AssistantTabInsights:
		body = ui.CreateElement(assistantInsightsDataPanel)
	case uistate.AssistantTabAutomations:
		body = ui.CreateElement(SmartHub)
	default: // Ask
		body = ui.CreateElement(Insights)
	}

	return Div(
		Attr("data-testid", "assistant-hub"),
		tabBar,
		body,
	)
}

// assistantTabJob returns the one-line statement of what the active tab is for.
// It sits under the tab bar rather than in a tooltip because the confusion it
// answers ("which of these three do I want?") happens before anything is hovered.
func assistantTabJob(tab string) string {
	switch tab {
	case uistate.AssistantTabInsights:
		return uistate.T("assistant.jobInsights")
	case uistate.AssistantTabAutomations:
		return uistate.T("assistant.jobAutomations")
	default:
		return uistate.T("assistant.jobAsk")
	}
}

// AssistantInsightsRoute opens the hub on its Insights tab. The /insights route
// used to render the CHAT — the same component /assistant renders — so the two
// URLs were the same page under different names, and the one called "insights"
// showed the least analysis of the two. It now lands where its name promises.
func AssistantInsightsRoute() ui.Node {
	return ui.CreateElement(assistantRouteTo, assistantRouteProps{Tab: uistate.AssistantTabInsights})
}

// AssistantAutomationsRoute opens the hub on its Automations tab, so /smart and
// the hub's third tab are one surface rather than two copies of the catalog.
func AssistantAutomationsRoute() ui.Node {
	return ui.CreateElement(assistantRouteTo, assistantRouteProps{Tab: uistate.AssistantTabAutomations})
}

type assistantRouteProps struct{ Tab string }

// assistantRouteTo renders the hub with the tab preselected. The selection happens
// in an effect rather than during render: setting an atom mid-render would fight
// the render that is already in flight, and the hub reads the atom itself.
func assistantRouteTo(p assistantRouteProps) ui.Node {
	tab := uistate.UseAssistantTab()
	ui.UseEffect(func() func() {
		if tab.Get() != p.Tab {
			tab.Set(p.Tab)
		}
		return nil
	}, p.Tab)
	return ui.CreateElement(AssistantHub)
}

type insightsPeriodPickerProps struct {
	Selected string
	OnPick   func(string)
}

// insightsPeriodPicker chooses the briefing's time range (G2-C8). Its own
// component so the select's change hook sits at a stable position.
func insightsPeriodPicker(p insightsPeriodPickerProps) ui.Node {
	onChange := ui.UseEvent(func(e ui.Event) { p.OnPick(e.GetValue()) })
	return Label(css.Class("todo-ctrl"), Title(uistate.T("assistant.periodPick")),
		Span(css.Class("todo-ctrl-label"), uistate.T("assistant.periodLabel")),
		Select(css.Class("todo-select"), Attr("aria-label", uistate.T("assistant.periodPick")),
			Attr("data-testid", "assistant-period"), OnChange(onChange),
			MapKeyed(insightsperiod.All(),
				func(x insightsperiod.Period) any { return string(x) },
				func(x insightsperiod.Period) ui.Node {
					return Option(Value(string(x)), SelectedIf(string(x) == p.Selected),
						uistate.T(insightsperiod.LabelKey(x)))
				},
			),
		),
	)
}

// astTile wraps a tile body in the shared Widget chrome at an explicit bento
// column placement ("1 / span 4" full-width, "span 2" for a half-width pair).
func astTile(tid, col string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: col, Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// astSection wraps a tile body with a serif section title + optional header
// action, reusing the debt-section chrome so the briefing matches the other
// redesigned surfaces (/networth, /health).
func astSection(sid, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if sid != "" {
		args = append(args, Attr("id", sid))
	}
	if title != "" {
		args = append(args, Div(css.Class("debt-section-head"),
			H2(css.Class("debt-section-title"), title),
			If(action != nil, action),
		))
	}
	args = append(args, body)
	return Div(args...)
}

// assistantBriefLine composes the hero's agent-voiced briefing sentence: what
// was spent so far and — when spending is ahead of pace and a rising category
// exists — what is doing the pushing. The ahead/behind judgment itself lives
// only in the pace pill, so the two never repeat the same figure.
func assistantBriefLine(mtd, pace int64, base string, anomalies []insights.Anomaly) string {
	if mtd == 0 {
		return uistate.T("assistant.briefQuiet")
	}
	line := uistate.T("assistant.briefSpent", fmtMoney(money.New(mtd, base)))
	if mtd > pace {
		for _, a := range anomalies {
			if a.Direction == insights.Up {
				return line + " " + uistate.T("assistant.briefPush", a.Category)
			}
		}
	}
	return line
}

// assistantInsightsDataPanel is the Insights tab body for AssistantHub — the
// agent's briefing, rendered as a widgetized bento surface in the app's
// redesigned-page pattern: a hero tile (the month-to-date spend in the display
// serif, a pace delta pill, the briefing sentence, and figure chips), a
// toolbar (custom-values toggle + report/transaction drills), the attention
// pair (flagged activity and category shifts, each with a designed all-clear
// state), the spending trend (theme-accent chart + takeaway), the top-merchant
// and pinned-insight pair, and an opt-in FormulaBuilder tile. The same figures
// are exposed as assistant_* engine variables (engineenv.addAssistantVars) via
// the shared AssistantSpendStory/AssistantHighlights helpers, so a formula
// figure always matches the page.
func assistantInsightsDataPanel() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	// Hooks — all called unconditionally at stable positions, before any
	// data-dependent early return.
	_ = uistate.UseDataRevision().Get()
	nav := router.UseNavigate()
	txFilterAtom := uistate.UseTxFilter()
	scopeAtom := uistate.UseActiveScope()
	pr := uistate.UsePrefs().Get()
	rev := ui.UseState(0)
	showFormulas := ui.UseState(false)
	// WF-SM1: flags the household has already judged are hidden, but never
	// silently — showJudged is the way back to them, and every suppression this
	// app grants has to have one.
	showJudged := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))
	// G2-C8: the briefing was fixed to month-to-date, so the tab called Insights
	// could only ever show one insight. The selection is an atom rather than local
	// state so it survives a trip to another tab and back.
	periodAtom := uistate.UseInsightsPeriod()

	// ── Data setup ────────────────────────────────────────────────────────────

	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: settings.FXRates}

	accounts := app.Accounts()
	if len(accounts) == 0 {
		return ui.CreateElement(EmptyStateCTA, emptyCTAProps{
			Message:   uistate.T("assistant.emptyData"),
			CTALabel:  uistate.T("accounts.addFirst"),
			AddTarget: "account",
		})
	}

	sc := scopeAtom.Get()
	instOf := func(a domain.Account) string { return a.Institution }
	scopeIDs := scope.ResolveScope(accounts, sc, instOf)
	scopedTxns := scope.ApplyScopeToTxns(app.Transactions(), scopeIDs)
	cats := app.Categories()
	catsByName := categoryNameToIDMap(cats)
	now := time.Now()
	accent := chartLineColor(uistate.CurrentAccent())

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

	// ── Briefing figures — the SAME derivations as the assistant_* variables ──

	window := insightsperiod.Resolve(insightsperiod.Period(periodAtom.Get()), now)
	mtd, prev, _ := engineenv.AssistantSpendStoryRange(scopedTxns, rates, window.Start, window.End, window.PriorStart, window.PriorEnd)
	// The comparison figure IS the pace for every window: each period is measured
	// against the equivalent span before it, so "ahead of pace" means the same
	// thing whichever period is chosen.
	pace := prev
	anomalies := engineenv.AssistantHighlights(scopedTxns, cats, rates, now, uistate.T("insights.uncategorized"))
	merchants, _ := reports.TopPayeesTrailing(scopedTxns, engineenv.AssistantMerchantWindowDays, now, rates, 7)
	flaggedIns := runAnomalyDetectors(app, pr.WeekStartWeekday())

	// ── Hero tile: the month's spending story ─────────────────────────────────

	var pacePill ui.Node = Fragment()
	if delta := mtd - pace; delta != 0 {
		// More spend than last month's pace reads in the money-down tone.
		arrow, tone, key := "▲", "neg", "assistant.paceAhead"
		mag := delta
		if delta < 0 {
			arrow, tone, key, mag = "▼", "pos", "assistant.paceBehind", -delta
		}
		pacePill = Span(ClassStr("rpt-delta "+tone), Attr("data-testid", "ast-pace-delta"),
			// The tooltip names WHAT the figure is compared against, which now
			// varies by period. Leaving it fixed at "the same day last month"
			// would misdescribe every window except the default.
			Attr("title", uistate.T(insightsperiod.ComparisonKey(window.Period))),
			arrow+" "+uistate.T(key, fmtMoney(money.New(mag, base))))
	}

	chips := []ui.Node{
		rptChip(uistate.T(insightsperiod.ComparisonKey(window.Period)), fmtMoney(money.New(prev, base)), ""),
	}
	// The hero is a month-to-date story, so its top-merchant chip must be
	// month-scoped — the trailing-90-day `merchants` ranking feeds the Top
	// merchants tile below, not this chip (2026-07-18 assessment, High: the
	// "This month" summary presented a 90-day aggregate as current-month spend).
	monthStart := dateutil.MonthStart(now)
	if monthMerchants, _ := reports.TopPayees(scopedTxns, monthStart, dateutil.AddMonths(monthStart, 1), rates, 1); len(monthMerchants) > 0 {
		chips = append(chips, rptChip(uistate.T("assistant.chipTopMerchant", monthMerchants[0].Name),
			fmtMoney(money.New(monthMerchants[0].Amount, base)), ""))
	}
	flaggedTone := ""
	if len(flaggedIns) > 0 {
		flaggedTone = rptToneCls("neg")
	}
	chips = append(chips, rptChip(uistate.T("assistant.chipFlagged"), fmt.Sprintf("%d", len(flaggedIns)), flaggedTone))

	// The selector sits in the hero's own header, beside the figure it governs —
	// a range control anywhere else leaves the reader guessing which numbers moved
	// when it changes.
	periodControl := ui.CreateElement(insightsPeriodPicker, insightsPeriodPickerProps{
		Selected: string(window.Period),
		OnPick:   func(v string) { periodAtom.Set(v) },
	})
	heroTile := astTile("ast-hero", "1 / span 4", astSection("sec-ast-hero", uistate.T(insightsperiod.LabelKey(window.Period)), periodControl,
		Div(css.Class("rpt-hero"),
			P(css.Class("rpt-hero-eyebrow", tw.TextDim), Attr("data-testid", "ast-hero-range"),
				uistate.T("assistant.heroRange",
					uistate.LoadPrefs().FormatDate(window.Start),
					uistate.LoadPrefs().FormatDate(window.End.AddDate(0, 0, -1)))),
			Div(css.Class("rpt-hero-main"),
				Div(
					Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), Attr("data-countup", ""), Attr("data-testid", "ast-hero-value"), fmtMoney(money.New(mtd, base))),
					pacePill,
				),
			),
			P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "ast-brief"),
				assistantBriefLine(mtd, pace, base, anomalies)),
			Div(css.Class("debt-chips"), chips),
		)))

	// ── Toolbar tile ───────────────────────────────────────────────────────────

	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("assistant.metricsShow")
	if showFormulas.Get() {
		metricsCls += " is-on"
		metricsLabel = uistate.T("assistant.metricsHide")
	}
	toolbar := astTile("ast-toolbar", "1 / span 4", Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas.Get())),
				Attr("data-testid", "ast-toggle-formulas"), Title(uistate.T("assistant.metricsTitle")),
				OnClick(toggleFormulas), Text(metricsLabel)),
		),
		Div(css.Class("filter-strip-controls"),
			A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/reports")), Attr("data-testid", "ast-reports-link"), uistate.T("assistant.viewReports")),
			A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/transactions")), Attr("data-testid", "ast-transactions-link"), uistate.T("assistant.viewTransactions")),
		),
	))

	// ── Attention pair: flagged activity + category shifts ────────────────────

	var flaggedBody ui.Node
	if len(flaggedIns) == 0 {
		flaggedBody = Div(css.Class("ast-clear"), Attr("data-testid", "ast-all-clear"),
			Span(css.Class("ast-clear-mark"), "✓"),
			P(css.Class("muted"), uistate.T("assistant.flaggedClear")))
	} else {
		// Fold consecutive runs of the same finding kind into one collapsible
		// summary row when a run is long enough to clutter (detail5). This is a
		// presentation-layer fold — the detectors are untouched; short runs still
		// list individually. A folded "expected payments" group also offers a
		// Review bills action to the recurring surface.
		gotoSource := func(route string) { nav.Navigate(uistate.RoutePath(route)) }
		reviewBills := func() { nav.Navigate(uistate.RoutePath("/recurring")) }
		rows := make([]ui.Node, 0, len(flaggedIns))
		for _, run := range groupInsightRuns(flaggedIns) {
			if len(run.Items) < flaggedGroupMin {
				for _, ins := range run.Items {
					route := "/transactions"
					if ins.Page == smart.PageAccounts {
						route = "/accounts"
					}
					capturedRoute := route
					rows = append(rows, ui.CreateElement(SmartAnomalyInsightRow, smartAnomalyInsightRowProps{
						Insight: ins,
						Route:   capturedRoute,
						OnClick: func() { nav.Navigate(uistate.RoutePath(capturedRoute)) },
					}))
				}
				continue
			}
			props := smartAnomalyGroupRowProps{
				Run:        run,
				Heading:    flaggedGroupHeading(run.Feature, len(run.Items)),
				OnNavigate: gotoSource,
			}
			if run.Feature == "SMART-T7" { // missing / expected payments → the bills surface
				props.ReviewLabel = uistate.T("detail5.reviewBills")
				props.ReviewAria = uistate.T("detail5.reviewBillsAria")
				props.OnReview = reviewBills
			}
			rows = append(rows, ui.CreateElement(SmartAnomalyGroupRow, props))
		}
		flaggedBody = Fragment(
			P(css.Class("muted"), uistate.T("insights.flaggedHint")),
			Div(css.Class("insight-list"), rows),
		)
	}
	flaggedTile := astTile("ast-flagged", "span 2", astSection("sec-ast-flagged", uistate.T("insights.flaggedTitle"), nil, flaggedBody))

	// WF-SM1 second clause: a flag the household has judged one-time, expected or
	// a new normal stops being news. "Wrong category" and "Investigate" are NOT
	// suppressed — the first means the money still needs refiling and the second
	// means somebody is still looking, and hiding either would answer a question
	// the reader had not finished asking.
	verdicts := uistate.LoadFlagVerdicts()
	shown := make([]insights.Anomaly, 0, len(anomalies))
	judged := make([]insights.Anomaly, 0, len(anomalies))
	for _, a := range anomalies {
		if verdicts.Suppressed(flagKey(a.Category, now), a.Category, now) {
			judged = append(judged, a)
			continue
		}
		shown = append(shown, a)
	}
	if showJudged.Get() {
		shown = append(shown, judged...)
	}
	highlightRow := func(a insights.Anomaly) ui.Node {
		key := flagKey(a.Category, now)
		cur, _ := verdicts.For(key)
		return ui.CreateElement(insightsHighlightRow, insightsHighlightRowProps{
			Anomaly:     a,
			Base:        base,
			Attribution: anomalyAttributionText(a, catsByName[a.Category], scopedTxns, rates, base, now),
			Verdict:     cur.Verdict,
			OnJudge: func(v flagverdict.Verdict) {
				if v == "" {
					uistate.ForgetFlagVerdict(key)
				} else {
					uistate.RecordFlagVerdict(key, a.Category, v, now)
				}
				rev.Set(rev.Get() + 1)
			},
			OnDrill: viewCategoryTransactions,
		})
	}

	var highlightsBody ui.Node
	switch {
	case len(shown) == 0 && len(judged) == 0:
		highlightsBody = P(css.Class("empty"), Attr("data-testid", "ast-highlights-empty"), uistate.T("assistant.highlightsEmpty"))
	default:
		rows := MapKeyed(shown, func(a insights.Anomaly) any { return a.Category }, highlightRow)
		highlightsBody = Fragment(
			P(css.Class("muted"), uistate.T("insights.highlightsHint")),
			Div(css.Class("insight-list"), rows),
			If(len(judged) > 0, ui.CreateElement(judgedFlagsNote, judgedFlagsNoteProps{
				Count: len(judged), Shown: showJudged.Get(),
				OnToggle: func() { showJudged.Set(!showJudged.Get()) },
			})),
		)
	}
	highlightsTile := astTile("ast-highlights", "span 2", astSection("sec-ast-highlights", uistate.T("insights.highlightsTitle"), nil, highlightsBody))

	// ── Trend tile: the last six complete months, theme-accent, with takeaway ──

	trendTile := astTile("ast-trend", "1 / span 4", astSection("sec-ast-trend", uistate.T("insights.spendTrendTitle"), nil,
		assistantSpendTrend(scopedTxns, base, rates, accent)))

	// ── Merchants + pinned pair ────────────────────────────────────────────────

	var merchantsBody ui.Node
	if len(merchants) == 0 {
		merchantsBody = P(css.Class("empty"), uistate.T("assistant.merchantsEmpty"))
	} else {
		ms := make([]merchantSpend, len(merchants))
		for i, s := range merchants {
			ms[i] = merchantSpend{Name: s.Name, Total: s.Amount, Count: s.Count}
		}
		rows := MapKeyed(ms,
			func(m merchantSpend) any { return m.Name },
			func(m merchantSpend) ui.Node {
				rank := 0
				for i, other := range ms {
					if other.Name == m.Name {
						rank = i + 1
						break
					}
				}
				return ui.CreateElement(insightsMerchantRow, insightsMerchantRowProps{
					Merchant: m,
					Base:     base,
					Rank:     rank,
					OnDrill:  viewMerchantTransactions,
				})
			},
		)
		merchantsBody = Fragment(
			P(css.Class("muted"), uistate.T("insights.topMerchantsHint")),
			Div(css.Class("insight-list"), rows),
		)
	}
	merchantsTile := astTile("ast-merchants", "span 2", astSection("sec-ast-merchants", uistate.T("insights.topMerchantsTitle"), nil, merchantsBody))

	pins := app.SavedInsights()
	sort.Slice(pins, func(i, j int) bool { return pins[i].CreatedAt.After(pins[j].CreatedAt) })
	deletePinned := func(pid string) {
		_ = app.DeleteSavedInsight(pid)
		rev.Set(rev.Get() + 1)
	}
	var pinnedBody ui.Node
	if len(pins) == 0 {
		pinnedBody = P(css.Class("empty"), uistate.T("assistant.pinnedEmpty"))
	} else {
		pinnedBody = Div(css.Class("rows"), MapKeyed(pins,
			func(p domain.SavedInsight) any { return p.ID },
			func(p domain.SavedInsight) ui.Node {
				return ui.CreateElement(PinnedInsightRow, pinnedInsightRowProps{
					Insight:  p,
					OnDelete: deletePinned,
				})
			},
		))
	}
	pinnedTile := astTile("ast-pinned", "span 2", astSection("sec-ast-pinned", uistate.T("insights.pinnedTitle"), nil, pinnedBody))

	// ── Habits: what repeats, as opposed to what happened once (WF-SM2) ───────
	habits := spendHabitLines(scopedTxns, rates, base, now)
	var habitsBody ui.Node
	if len(habits) == 0 {
		habitsBody = P(css.Class("empty"), Attr("data-testid", "ast-habits-empty"), uistate.T("habits.empty"))
	} else {
		rows := make([]ui.Node, 0, len(habits))
		for _, line := range habits {
			rows = append(rows, P(css.Class("insight-row"), Attr("data-testid", "ast-habit"), line))
		}
		habitsBody = Div(css.Class("insight-list"), rows)
	}
	habitsTile := astTile("ast-habits", "span 2", astSection("sec-ast-habits", uistate.T("habits.title"), nil, habitsBody))

	// ── Assemble the surface ───────────────────────────────────────────────────

	tiles := []ui.Node{heroTile, toolbar, flaggedTile, highlightsTile, habitsTile, trendTile, merchantsTile, pinnedTile}
	if showFormulas.Get() {
		tiles = append(tiles, astTile("ast-formula", "1 / span 4", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("assistant.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("assistant.metricsShow"), ShowSaved: true}),
		)))
	}
	return Fragment(
		// AG10: the guided month-end review sits ABOVE the briefing, because it is
		// a thing to do and the briefing is a thing to read. It renders nothing at
		// all when there is nothing to review, when it has been done this month, or
		// when it was dismissed — the never-naggy rule lives in one place.
		ui.CreateElement(MonthlyReviewCard),
		Div(css.Class("bento bento-assistant"), Attr("data-testid", "assistant-insights-surface"), tiles),
	)
}

// assistantSpendTrend renders the briefing's spending trend: total expense
// outflow for each of the last six complete months as a theme-accent area
// chart, with a serif takeaway comparing the latest complete month to the
// six-month average. Returns a quiet empty note with fewer than 2 points.
func assistantSpendTrend(txns []domain.Transaction, base string, rates currency.Rates, accent string) ui.Node {
	const buckets = 6
	curMonth := dateutil.MonthStart(time.Now())
	bounds := make([]time.Time, buckets+1)
	for k := 0; k <= buckets; k++ {
		bounds[k] = dateutil.AddMonths(curMonth, k-buckets)
	}
	flows, err := reports.IncomeExpenseSeries(txns, bounds, rates)
	if err != nil || len(flows) < 2 {
		return P(css.Class("empty"), uistate.T("assistant.trendEmpty"))
	}
	vals := make([]float64, len(flows))
	labels := make([]string, 0, len(flows))
	valLabels := make([]string, 0, len(flows))
	var total int64
	for i, f := range flows {
		vals[i] = float64(f.Expense)
		total += f.Expense
		labels = append(labels, bounds[i].Format("Jan"))
		valLabels = append(valLabels, fmtMoney(money.New(f.Expense, base)))
	}

	// Takeaway: the latest complete month vs the six-month average.
	takeaway := ""
	avg := total / int64(len(flows))
	last := flows[len(flows)-1].Expense
	lastName := bounds[len(flows)-1].Format("January")
	diff := last - avg
	switch {
	case diff > 0:
		takeaway = uistate.T("assistant.trendAbove", lastName, fmtMoney(money.New(diff, base)))
	case diff < 0:
		takeaway = uistate.T("assistant.trendBelow", lastName, fmtMoney(money.New(-diff, base)))
	default:
		takeaway = uistate.T("assistant.trendEven", lastName)
	}

	return Fragment(
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "ast-trend-takeaway"), takeaway),
		P(css.Class("muted"), uistate.T("insights.spendTrendHint")),
		uiw.AreaChart(uiw.AreaChartProps{
			Values:      vals,
			Stroke:      accent,
			GradientID:  "cf-assistant-spend",
			Label:       uistate.T("insights.spendTrendTitle"),
			Labels:      labels,
			ValueLabels: valLabels,
		}),
	)
}
