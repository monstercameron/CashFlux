// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/attention"
	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/chartspec"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/freshness"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/insights"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/safespend"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartengine"
	"github.com/monstercameron/CashFlux/internal/tasksort"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Dashboard shows headline metrics in the candidate-C bento grid, driven by the
// live store and the shared time-resolution window.
func Dashboard() ui.Node {
	app := appstate.Default
	if app == nil {
		return Div(css.Class("bento"), Div(css.Class("w"), Div(css.Class("wbody"), P(css.Class("empty"), uistate.T("common.notReady")))))
	}
	_ = uistate.UseDataRevision().Get() // re-render after import / load-sample / wipe

	// Smoothly animate tiles when the bento arrangement changes (drag-reorder,
	// resize, auto-layout switch) via the FLIP shim (web/flip.js). Keyed on a
	// signature of the layout, so it fires exactly when the arrangement could
	// move — not on every data tick. (B2)
	layoutItems := uistate.UseLayoutItems().Get()
	layoutMode := uistate.UseLayoutMode().Get()
	flipSig := string(layoutMode)
	for _, it := range layoutItems {
		flipSig += fmt.Sprintf("|%s:%dx%d:%d", it.ID, it.ColSpan, it.RowSpan, it.Importance)
	}
	// Include the live drag preview so the FLIP also animates the reflow that
	// happens while dragging over tiles, not just on drop (B2).
	flipSig += "|" + uistate.UseDragSource().Get() + ">" + uistate.UseDragPreview().Get()
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxFlipBento"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, flipSig)

	accounts := app.Accounts()
	txns := app.Transactions()
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// L21: scope spending/income KPIs to the active member when one is selected
	// in the top-bar switcher. Net worth is account-based so it stays household-
	// wide regardless of the member view.
	activeMemberAtom := uistate.UseActiveMember()
	activeMemberID := activeMemberAtom.Get()
	kpiTxns := txns
	if activeMemberID != "" {
		filtered := make([]domain.Transaction, 0, len(txns))
		for _, t := range txns {
			if t.MemberID == activeMemberID {
				filtered = append(filtered, t)
			}
		}
		kpiTxns = filtered
	}

	// Memoized via state.UseComputed keyed on app.Rev() — recomputes only when the
	// dataset/FX actually changes, not on every re-render (§1.6).
	nw := useNetWorth(app, accounts, txns, rates)
	net, assets, liabilities := nw.Net, nw.Assets, nw.Liabilities
	w := uistate.UsePeriod().Get()
	widgetCfgs := uistate.UseWidgetConfigs().Get()
	start, end := w.Range()
	// Memoized (§1.6): keyed on app.Rev() + the period + the active-member filter,
	// so it recomputes only when one of those changes.
	income, expense := usePeriodTotals(app, kpiTxns, start, end, rates, activeMemberID)

	// W-15: trigger count-up animation on the KPI hero figures whenever the
	// underlying values change. The sig is keyed on the four headline amounts so
	// the effect fires exactly on mount and on genuine data changes — not on every
	// re-render that leaves the numbers unchanged. cashfluxCountUpScan (countup.js)
	// tracks per-element last-animated values so it skips elements whose text
	// hasn't changed and always restores the exact original string at end-of-tween.
	kpiSig := fmt.Sprintf("%d|%d|%d|%d", net.Amount, income.Amount, expense.Amount, liabilities.Amount)
	ui.UseEffect(func() func() {
		if fn := js.Global().Get("cashfluxCountUpScan"); fn.Type() == js.TypeFunction {
			fn.Invoke()
		}
		return nil
	}, kpiSig)

	// Cash flow = income − spending for the period (G1 §7): the surplus/deficit Elena
	// wants in one line. Shown as a signed sub-line on the Income tile so "what
	// changed?" is answerable above the fold without mental arithmetic.
	cashFlow := money.New(income.Amount-expense.Amount, income.Currency)
	cashFlowSub := "cash flow −" + fmtMoney(money.New(-cashFlow.Amount, income.Currency))
	if cashFlow.Amount >= 0 {
		cashFlowSub = "cash flow +" + fmtMoney(cashFlow)
	}
	periodLabel := w.FromLabel()
	if w.ToLabel() != w.FromLabel() {
		periodLabel += " – " + w.ToLabel()
	}

	incCount, expCount := 0, 0
	for _, t := range kpiTxns {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			incCount++
		case t.IsExpense():
			expCount++
		}
	}

	active := 0
	for _, a := range accounts {
		if !a.Archived {
			active++
		}
	}

	// "Remind me" on the freshness nudge → create a to-do and jump to the list.
	nav := router.UseNavigate()
	noticeAtom := uistate.UseNotice()
	freshnessDismissals := uistate.UseFreshnessDismissals()
	remindToUpdate := ui.UseEvent(func() {
		if _, err := app.CreateFreshnessReminderTask(uistate.T("dashboard.staleTaskTitle")); err != nil {
			noticeAtom.Set(noticeAtom.Get().With(uistate.T("dashboard.reminderErr", err.Error()), true))
			return
		}
		nav.Navigate(uistate.RoutePath("/todo"))
	})
	dismissFreshness := ui.UseEvent(func() {
		stale := freshness.StaleAccounts(accounts, app.FreshnessWindows(), time.Now())
		next := freshnessDismissals.Get().Dismiss(stale, time.Now())
		freshnessDismissals.Set(next)
		uistate.PersistFreshnessDismissals(next)
	})

	// Net-worth change since the start of this month (end of last month).
	nwSub, nwTone := uistate.T("dashboard.assetsSub", fmtMoney(assets)), "text-dim"
	if prev, _ := ledger.NetWorthSeries(accounts, txns, []time.Time{dateutil.MonthStart(time.Now())}, rates); len(prev) == 1 {
		if d, ok := ledger.PercentChange(net.Amount, prev[0].Amount); ok {
			// A flat 0% reads as "nothing moved" rather than "income == spending";
			// say so plainly with the absolute delta instead of a misleading "▲ 0%"
			// (G1 §7).
			delta := money.New(net.Amount-prev[0].Amount, net.Currency)
			switch {
			case d < 0:
				nwTone, nwSub = "text-down", fmt.Sprintf("▼ %d%% (%s) this month", -d, fmtMoney(delta))
			case d > 0:
				nwTone, nwSub = "text-up", fmt.Sprintf("▲ %d%% (+%s) this month", d, fmtMoney(delta))
			case delta.Amount != 0:
				nwTone, nwSub = "text-dim", fmt.Sprintf("%s this month", fmtMoney(delta))
			default:
				nwTone, nwSub = "text-dim", "No change this month"
			}
		}
	}
	// A missing FX rate excludes accounts from the total (L4) — say so on the tile,
	// rather than letting net worth silently collapse.
	if len(nw.MissingCurrencies) > 0 {
		nwTone = "text-down"
		nwSub = "excludes " + plural(len(nw.ExcludedAccounts), "account") + " — no " + strings.Join(nw.MissingCurrencies, ", ") + " rate"
	} else {
		// C82: when the total folds in non-base-currency accounts, disclose that a
		// conversion happened so the figure isn't read as a raw same-currency sum.
		for _, ac := range accounts {
			if !ac.Archived && ac.Currency != "" && ac.Currency != base {
				nwSub += " · " + uistate.T("dashboard.netWorthConverted", base)
				break
			}
		}
	}

	attnCol, attnRow := spanOf(layoutItems, "attention")
	// Each tile is built lazily by id, then the dashboard renders them in the saved
	// layout order, skipping any the user has hidden in the Widget Manager. This
	// single id-keyed path is what the manager's visibility/order controls hook
	// into (the widget shell still positions each tile from the layout atom).
	renderers := map[string]func() ui.Node{
		"attention": func() ui.Node {
			return attentionWidget(app, txns, rates, start, end, freshnessDismissals.Get(), widgetCfgs.For("attention"), attnCol, attnRow)
		},
		"kpi-networth": func() ui.Node {
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-networth", Title: uistate.T("dashboard.netWorth"), Draggable: true, Resizable: true,
				GridColumn: "1", GridRow: "2", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBodyHero(fmtMoney(net), figTone(net), nwSub, nwTone),
			})
		},
		"kpi-income": func() ui.Node {
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-income", Title: uistate.T("dashboard.income"), Draggable: true, Resizable: true,
				GridColumn: "2", GridRow: "2", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBody(fmtMoney(income), "text-up", periodLabel+" · "+plural(incCount, "deposit")+" · "+cashFlowSub, "text-dim"),
			})
		},
		"kpi-spending": func() ui.Node {
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-spending", Title: uistate.T("dashboard.spending"), Draggable: true, Resizable: true,
				GridColumn: "3", GridRow: "2", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBody(fmtMoney(expense), "text-down", periodLabel+" · "+plural(expCount, "transaction"), "text-dim"),
			})
		},
		"kpi-liabilities": func() ui.Node {
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-liabilities", Title: uistate.T("dashboard.liabilities"), Draggable: true, Resizable: true,
				GridColumn: "4", GridRow: "2", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBody(fmtMoney(liabilities), "", uistate.T("dashboard.accountsCount", active), "text-dim"),
			})
		},
		"kpi-assets": func() ui.Node {
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-assets", Title: uistate.T("dashboard.assets"), Draggable: true, Resizable: true,
				GridColumn: "1", GridRow: "3", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBody(fmtMoney(assets), "text-up", uistate.T("dashboard.accountsCount", active), "text-dim"),
			})
		},
		// R15/C139: glanceable Safe-to-spend KPI tile — ONE canonical pure formula
		// (liquid cash − bills due this period − prorated goal contributions), NO
		// Smart/AI gate. Shares the safespend package with planning.go so every
		// surface agrees (C140/C142). Red "−$X over" when the buffer is blown.
		"kpi-safetospend": func() ui.Node {
			liquid, _ := ledger.LiquidBalance(accounts, txns, rates)
			_, mEnd := dateutil.MonthRange(time.Now())
			toBase := safespend.ToBaseFunc(rates)
			billsDue := safespend.BillsDueBefore(accounts, app.Recurring(), time.Now(), mEnd, toBase)
			goalNeeds := safespend.GoalContributionsProrated(app.Goals(), time.Now(), toBase)
			bd := safespend.Compute(liquid.Amount, billsDue, goalNeeds, 0, base)
			fig, tone, sub := fmtMoney(money.New(bd.SafeToSpend, base)), "text-up", uistate.T("dashboard.safeToSpendSub")
			if bd.IsNegative {
				fig = "−" + fmtMoney(money.New(-bd.SafeToSpend, base))
				tone, sub = "text-down", uistate.T("dashboard.safeToSpendOver")
			}
			return uiw.Widget(uiw.WidgetProps{
				ID: "kpi-safetospend", Title: uistate.T("dashboard.safeToSpend"), Draggable: true, Resizable: true,
				GridColumn: "2", GridRow: "3", BodyClass: "kpi " + tw.Fold(tw.Flex, tw.FlexCol, tw.JustifyCenter),
				Body: kpiBody(fig, tone, sub, "text-dim"),
			})
		},
		"recent":   func() ui.Node { return recentWidget(txns, widgetCfgs.For("recent")) },
		"budgets":  func() ui.Node { return budgetsWidget(app, txns, rates, start, end, widgetCfgs.For("budgets")) },
		"goals":    func() ui.Node { return goalsWidget(app, widgetCfgs.For("goals")) },
		"todo":     func() ui.Node { return todoWidget(app, widgetCfgs.For("todo")) },
		"accounts": func() ui.Node { return accountsWidget(app, txns, widgetCfgs.For("accounts")) },
		"trend":    func() ui.Node { return netWorthTrendWidget(accounts, txns, rates, net, widgetCfgs.For("trend")) },
		"cashflow": func() ui.Node { return cashFlowWidget(txns, rates) },
		"savings":  func() ui.Node { return savingsRateWidget(income, expense, widgetCfgs.For("savings")) },
		"health":   func() ui.Node { return ui.CreateElement(healthWidgetNode, struct{}{}) },
		"breakdown": func() ui.Node {
			return spendingBreakdownWidget(app, txns, rates, start, end, widgetCfgs.For("breakdown"))
		},
		"bills": func() ui.Node { return upcomingBillsWidget(app) },
		"freshness": func() ui.Node {
			return freshnessWidget(accounts, app.FreshnessWindows(), freshnessDismissals.Get(), remindToUpdate, dismissFreshness)
		},
		"highlight":    func() ui.Node { return topHighlightWidget(txns, app.Categories(), rates) },
		"smart-digest": func() ui.Node { return smartDigestWidget(app) },
		// R25/C252: always-on SMART anomaly hub — four detector types (balance,
		// duplicates, spending spikes, missing transactions) surfaced on the dashboard
		// without any Smart opt-in gate. Shares buildSmartInput + EnableFreeOnly with
		// the /insights version so detection is identical on both surfaces.
		"anomaly-hub": func() ui.Node { return ui.CreateElement(anomalyHubWidget, app) },
	}

	hidden := uistate.UseHiddenWidgets().Get()
	tiles := make([]any, 0, len(layoutItems)+1)
	// no-touch-chrome: lets the CSS agent hide drag/resize affordances (.grip, .rz
	// buttons) under @media (hover:none) so they don't show on touch screens (L33).
	tiles = append(tiles, css.Class("bento no-touch-chrome"))
	for _, it := range layoutItems {
		if hidden.IsHidden(it.ID) {
			continue
		}
		if render, ok := renderers[it.ID]; ok {
			tiles = append(tiles, render())
		} else if strings.HasPrefix(it.ID, vbCardPrefix) {
			// User-published Widget Builder card: render the saved cardgraph tile.
			if w := vbPublishedWidget(strings.TrimPrefix(it.ID, vbCardPrefix), it.ColSpan, it.RowSpan); w != nil {
				tiles = append(tiles, w)
			}
		}
	}

	return Fragment(
		// Optional decorative banner band (B20) — shown only when the user picks a
		// banner; driven entirely by CSS vars/attribute set by uistate.ApplyBanner,
		// so it needs no state here. Decorative, hence aria-hidden.
		Div(css.Class("app-banner"), Attr("aria-hidden", "true")),
		// Home band (EC4): glanceable greeting + net-worth hero + this-month stats
		// + quick actions. Sits above the bento; data comes from the §1.6 selectors.
		ui.CreateElement(dashboardHero),
		// C329: first-run onboarding callout — a dismissible setup checklist with a
		// link to the help center. Self-hides once setup is complete or dismissed.
		ui.CreateElement(dashOnboardCard),
		// C271: "While you were away" catch-up card — shown when new notifications
		// have arrived since the last time the user opened the Notification Center.
		// Dismissed per session (the atom resets on reload). Only shown when
		// lastSeen > 0 (not the very first open) and newCount > 0.
		ui.CreateElement(dashCatchUpCard),
		// C319: discoverable "Customize" affordance on the dashboard canvas itself —
		// the layout/show-hide/size/style controls live on /widget-manager, but the
		// dashboard previously had no on-canvas entry point. Shown only with data
		// (alongside the bento).
		If(len(accounts) > 0 || len(txns) > 0, ui.CreateElement(dashCustomizeBar)),
		// C8: on a genuinely empty workspace (no accounts and no transactions) the
		// bento KPI grid is just a wall of $0 tiles with no hierarchy — suppress it
		// and let the welcome hero + onboarding checklist own the empty state. The
		// grid returns the moment there's any real data to summarise.
		If(len(accounts) > 0 || len(txns) > 0, Div(tiles...)),
		// L43: Quick Transfer shortcut — a persistent affordance on the dashboard so
		// users can initiate a transfer without hunting for the Transactions screen.
		// Accounts is the natural home for transfer creation (the full form lives
		// there); we navigate to /transactions which also hosts the add-transfer flow.
		ui.CreateElement(dashTransferFAB),
	)
}

// dashCatchUpCard is a dismissible "While you were away" bento-adjacent card
// (C271). It appears above the bento grid when new notifications have arrived
// since the last time the user opened the Notification Center, giving a
// glanceable count with a direct link to the center. Dismissed per session
// (the dismissed state resets on reload); lastSeen is read from the KV store
// so the count is exactly what the Notification Center would show as "new".
func dashCatchUpCard() ui.Node {
	dismissedAtom := ui.UseState(false)
	nav := router.UseNavigate()

	feed := uistate.UseNotifyFeed().Get()
	now := time.Now().Unix()
	visible := uistate.VisibleFeed(feed, now)
	lastSeen := loadLastSeen()
	newCount := len(uistate.NewSinceLastSeen(visible, lastSeen))

	// Hide when: dismissed this session; first-ever open (lastSeen==0); no new items.
	if dismissedAtom.Get() || lastSeen == 0 || newCount == 0 {
		return nil
	}

	body := uistate.T("dashboard.catchUpBodyOne")
	if newCount > 1 {
		body = uistate.T("dashboard.catchUpBody", newCount)
	}

	onView := ui.UseEvent(func() {
		nav.Navigate(uistate.RoutePath("/notifications"))
	})
	onDismiss := ui.UseEvent(func() {
		dismissedAtom.Set(true)
	})

	return Div(
		css.Class("catchup-card"),
		Attr("role", "complementary"),
		Attr("aria-label", uistate.T("dashboard.catchUpTitle")),
		Div(css.Class("catchup-card-body"),
			Span(css.Class("catchup-card-icon"), "🔔"),
			Div(css.Class("catchup-card-text"),
				Strong(uistate.T("dashboard.catchUpTitle")),
				P(body),
			),
		),
		Div(css.Class("catchup-card-actions"),
			Button(css.Class("btn", "btn-primary"), Type("button"), OnClick(onView),
				uistate.T("dashboard.catchUpLink")),
			Button(css.Class("btn"), Type("button"), OnClick(onDismiss),
				uistate.T("notifications.catchUpDismiss")),
		),
	)
}

// dashTransferFAB is the "Transfer" floating shortcut on the dashboard (L43).
// It routes to /transactions (where the transfer form lives) so the main transfer
// logic stays in one place — this is a navigation affordance, not a duplicate form.
// Its own component keeps the navigate hook at a stable position (the On* rule).
func dashTransferFAB() ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/transactions")) })
	return Button(css.Class("dash-transfer-fab"), Type("button"),
		Attr("title", uistate.T("dashboard.transferTitle")),
		Attr("aria-label", uistate.T("dashboard.transferTitle")),
		Attr("data-testid", "dash-transfer-btn"),
		OnClick(open),
		uistate.T("dashboard.transfer"),
	)
}

// freshnessWidget is the full-width Freshness nudge: a friendly reminder of which
// account balances look stale (via internal/freshness), with how long since each
// was last updated.
func freshnessWidget(accounts []domain.Account, windows freshness.Windows, dismissals freshness.Dismissals, onRemind, onDismiss ui.Handler) ui.Node {
	now := time.Now()
	stale := freshness.VisibleStaleAccounts(accounts, windows, dismissals, now)
	var body ui.Node
	if len(stale) == 0 {
		body = P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.allFresh"))
	} else {
		chips := make([]ui.Node, 0, len(stale))
		for _, a := range stale {
			chips = append(chips, Span(css.Class("member-chip"),
				Span(a.Name),
				Span(css.Class("fig", tw.TextWarn), fmt.Sprintf("· %dd", freshness.DaysSinceUpdate(a, now))),
			))
		}
		body = Div(
			P(css.Class("t-body", tw.TextDim, tw.Mb2), uistate.T("dashboard.staleCount", len(stale))),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), chips),
			Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
				Button(css.Class("btn"), Type("button"), Title(uistate.T("dashboard.remindTitle")), OnClick(onRemind), uistate.T("dashboard.remind")),
				Button(css.Class("btn"), Type("button"), OnClick(onDismiss), uistate.T("action.dismiss")),
			),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "freshness", Title: uistate.T("dashboard.freshness"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "8", Body: body,
	})
}

// upcomingBillsWidget is the 2×1 Upcoming bills widget: the next due date and
// minimum payment for each liability account that has them, soonest first.
func upcomingBillsWidget(app *appstate.App) ui.Node {
	now := time.Now()
	pr := uistate.UsePrefs().Get()
	// Share the exact bill-derivation the Bills screen uses (B22), so the widget
	// and the screen always agree (incl. month-end due-date clamping).
	upcoming := bills.UpcomingAll(app.Accounts(), app.Recurring(), now)

	var body ui.Node
	if len(upcoming) == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noUpcomingBills"))
	} else {
		if len(upcoming) > 4 {
			upcoming = upcoming[:4]
		}
		rows := make([]ui.Node, 0, len(upcoming))
		for _, b := range upcoming {
			dueTone := "text-faint"
			if b.DaysUntil <= 7 {
				dueTone = "text-warn"
			}
			rows = append(rows, Div(css.Class(tw.Flex, tw.JustifyBetween),
				Span(b.Name),
				Span(ClassStr(dueTone), pr.FormatDate(b.DueDate)),
				Span(css.Class("fig", tw.FontDisplay, tw.TextDown, tw.W24, tw.TextRight), fmtMoney(b.Amount.Neg())),
			))
		}
		body = Div(css.Class("t-body", tw.SpaceY25), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "bills", Title: uistate.T("dashboard.upcomingBills"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "6",
		Body: body,
	})
}

// spendingBreakdownWidget is the 2×1 Spending breakdown widget: a segmented bar
// of the period's expenses by category (top three plus "Other") with a legend.
func spendingBreakdownWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time, cfg widgetcfg.Config) ui.Node {
	topN := 3
	if sch, ok := widgetcfg.SchemaFor("breakdown"); ok {
		if f, ok := sch.FieldByKey("topN"); ok {
			topN = f.Int(cfg)
		}
	}
	cats := app.Categories()
	catName := make(map[string]string, len(cats))
	parent := make(map[string]string, len(cats))
	exists := make(map[string]bool, len(cats))
	for _, c := range cats {
		catName[c.ID] = c.Name
		parent[c.ID] = c.ParentID
		exists[c.ID] = true
	}
	// rootOf walks up to the top-level ancestor so sub-category spend rolls up to
	// its parent. Cycle/orphan-safe (stops at a missing parent or a repeat).
	rootOf := func(id string) string {
		seen := map[string]bool{}
		for {
			p := parent[id]
			if p == "" || !exists[p] || seen[id] {
				break
			}
			seen[id] = true
			id = p
		}
		return id
	}

	totals := make(map[string]int64)
	var total int64
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			continue
		}
		amt := conv.Amount
		if amt < 0 {
			amt = -amt
		}
		totals[rootOf(t.CategoryID)] += amt
		total += amt
	}

	if total == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
			Body: P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noSpending")),
		})
	}

	// Rank categories by spend, collapsing the tail into "Other" (pure logic in
	// ledger.RankSpending); resolve display names here.
	type seg struct {
		name string
		amt  int64
	}
	ranked, other := ledger.RankSpending(totals, topN)
	segs := make([]seg, 0, len(ranked)+1)
	for _, ct := range ranked {
		name := catName[ct.CategoryID]
		if name == "" {
			name = uistate.T("dashboard.uncategorized")
		}
		segs = append(segs, seg{name: name, amt: ct.Amount})
	}
	if other > 0 {
		segs = append(segs, seg{name: uistate.T("dashboard.other"), amt: other})
	}

	tones := []string{"bg-up", "bg-warn", "bg-dim", "bg-down"}
	barParts := make([]ui.Node, 0, len(segs))
	legend := make([]ui.Node, 0, len(segs))
	for i, s := range segs {
		tone := tones[i%len(tones)]
		pct := int(s.amt * 100 / total)
		barParts = append(barParts, Div(ClassStr(tone), Style(map[string]string{"width": fmt.Sprintf("%d%%", pct)})))
		legend = append(legend, Span(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap15),
			Span(ClassStr(tw.Fold(tw.W2, tw.H2, tw.RoundedFull)+" "+tw.ColorClass(tone))),
			Textf("%s %d%%", s.name, pct),
		))
	}

	body := Div(
		Div(css.Class(tw.H25, tw.RoundedFull, tw.OverflowHidden, tw.Flex), barParts),
		Div(css.Class("t-caption", tw.Flex, tw.FlexWrap, tw.GapX4, tw.GapY1, tw.Mt3, tw.TextDim), legend),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "breakdown", Title: uistate.T("dashboard.breakdown"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "7",
		Body: body,
	})
}

// savingsRateWidget is the 2×1 Savings rate widget: the share of the period's
// income that wasn't spent, as a big figure and a bar.
func savingsRateWidget(income, expense money.Money, cfg widgetcfg.Config) ui.Node {
	pct := ledger.SavingsRate(income.Amount, expense.Amount)

	// Widget settings (gear → flip): target savings rate and whether to show the bar.
	target, showBar := 20, true
	if sch, ok := widgetcfg.SchemaFor("savings"); ok {
		if f, ok := sch.FieldByKey("target"); ok {
			target = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("showBar"); ok {
			showBar = f.Bool(cfg)
		}
	}

	// Tone reflects performance against the user's target: at/above = good,
	// positive-but-short = warning, negative = bad.
	tone, bar := "text-up", "bg-up"
	switch {
	case pct < 0:
		tone, bar = "text-down", "bg-down"
	case pct < target:
		tone, bar = "text-warn", "bg-warn"
	}

	left := Div(
		Div(ClassStr("fig t-figure-lg "+tw.Fold(tw.FontDisplay, tw.LeadingNone)+" "+tw.ColorClass(tone)), fmt.Sprintf("%d%%", pct)),
		Div(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("dashboard.savingsSub", target)),
	)
	var right ui.Node = Fragment()
	if showBar {
		right = Div(css.Class(tw.Flex1),
			uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: bar}),
			Div(css.Class("t-caption", tw.TextFaint, tw.Mt2), uistate.T("dashboard.thisPeriod")),
		)
	}
	body := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap5), left, right)
	return uiw.Widget(uiw.WidgetProps{
		ID: "savings", Title: uistate.T("dashboard.savingsRate"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "7",
		Body: body,
	})
}

// topHighlightWidget surfaces the single most significant spending change this
// month (via the shared anomaly detection) as a one-line plain-English highlight,
// or a calm "nothing notable" message when there's nothing to flag. It links the
// dashboard to the fuller Spending highlights card on the Insights screen.
func topHighlightWidget(txns []domain.Transaction, categories []domain.Category, rates currency.Rates) ui.Node {
	anomalies := detectSpendingAnomalies(txns, categories, rates)
	var body ui.Node
	if len(anomalies) == 0 {
		body = P(css.Class("t-body", tw.TextDim), uistate.T("dashboard.noHighlights"))
	} else {
		a := anomalies[0]
		body = Div(css.Class(tw.Flex, tw.ItemsStart, tw.Gap2),
			Span(ClassStr("insight-dot "+highlightTone(a)), Text(highlightArrow(a))),
			Span(css.Class("t-body"), highlightText(a, rates.Base)),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "highlight", Title: uistate.T("dashboard.highlight"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "9", Body: body,
	})
}

// cashFlowWidget is the 2×1 Cash flow widget: income (up) vs expense (down) bars
// for the last four months, scaled to the largest bar, with the current month's
// net to the right. Totals via ledger.PeriodTotals.
func cashFlowWidget(txns []domain.Transaction, rates currency.Rates) ui.Node {
	type monthBar struct {
		label           string
		income, expense int64
	}
	start := dateutil.MonthStart(time.Now())
	months := make([]monthBar, 0, 4)
	var maxv int64 = 1
	for i := 0; i < 4; i++ {
		ms := dateutil.AddMonths(start, i-3) // three months ago … current
		s, e := dateutil.MonthRange(ms)
		inc, exp, _ := ledger.PeriodTotals(txns, s, e, rates)
		mb := monthBar{label: ms.Format("Jan"), income: inc.Amount, expense: exp.Amount}
		if mb.income > maxv {
			maxv = mb.income
		}
		if mb.expense > maxv {
			maxv = mb.expense
		}
		months = append(months, mb)
	}

	bars := make([]ui.Node, 0, len(months))
	for i, mb := range months {
		labelTone := "text-faint"
		if i == len(months)-1 {
			labelTone = "text-fg"
		}
		bars = append(bars, Div(css.Class(tw.Flex, tw.FlexCol, tw.ItemsCenter, tw.Gap15),
			Div(css.Class(tw.Flex, tw.ItemsEnd, tw.Gap1, tw.H14),
				Div(css.Class(tw.W3, tw.BgUp), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.income*100/maxv))})),
				Div(css.Class(tw.W3, tw.BgDown), Style(map[string]string{"height": fmt.Sprintf("%d%%", int(mb.expense*100/maxv))})),
			),
			Span(ClassStr("t-caption "+labelTone), mb.label),
		))
	}

	last := months[len(months)-1]
	netMoney := money.New(last.income-last.expense, rates.Base)
	netTone := "text-up"
	if last.income-last.expense < 0 {
		netTone = "text-down"
	}
	netBlock := Div(css.Class(tw.MlAuto, tw.TextRight),
		Div(css.Class("t-caption", tw.TextFaint), "net · "+last.label),
		Div(ClassStr("fig "+tw.Fold(tw.FontDisplay, tw.TextLg)+" "+tw.ColorClass(netTone)), fmtMoney(netMoney)),
	)

	// R52(a): a one-sentence plain-English takeaway under the bars, so the widget
	// states what the income-vs-expense bars mean instead of leaving an unlabeled
	// mini-chart to interpret. Toned to match the net (kept = up, short = down).
	netAmt := last.income - last.expense
	var caption ui.Node
	switch {
	case netAmt > 0:
		caption = P(ClassStr("t-caption "+tw.ColorClass("text-up")), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowKept", fmtMoney(netMoney), last.label))
	case netAmt < 0:
		caption = P(ClassStr("t-caption "+tw.ColorClass("text-down")), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowShort", fmtMoney(money.New(-netAmt, rates.Base)), last.label))
	default:
		caption = P(css.Class("t-caption", tw.TextDim), Attr("data-testid", "cashflow-caption"),
			uistate.T("dashboard.cashFlowEven", last.label))
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "cashflow", Title: uistate.T("dashboard.cashFlow"), Draggable: true, Resizable: true, GridColumn: "1 / span 2", GridRow: "6",
		Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
			Div(css.Class(tw.Flex, tw.ItemsEnd, tw.Gap5), bars, netBlock),
			caption,
		),
	})
}

// netWorthTrendWidget is the 1×2 Net worth trend widget: the current figure over
// a six-month end-of-month area chart (via ledger.NetWorthSeries + the chart
// geometry helpers).
func netWorthTrendWidget(accounts []domain.Account, txns []domain.Transaction, rates currency.Rates, net money.Money, cfg widgetcfg.Config) ui.Node {
	months := 6
	showXAxis := true
	if sch, ok := widgetcfg.SchemaFor("trend"); ok {
		if f, ok := sch.FieldByKey("months"); ok {
			months = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("showXAxis"); ok {
			showXAxis = f.Bool(cfg)
		}
	}
	start := dateutil.MonthStart(time.Now())
	cutoffs := make([]time.Time, 0, months)
	for i := 0; i < months; i++ {
		cutoffs = append(cutoffs, dateutil.AddMonths(start, i-(months-2))) // window ends at the current month +1
	}
	series, _ := ledger.NetWorthSeries(accounts, txns, cutoffs, rates)
	deltaLabel, deltaTone := "No change", "text-dim"
	rangeLabel := ""
	if len(series) > 0 {
		first := series[0]
		last := series[len(series)-1]
		delta := money.New(last.Amount-first.Amount, last.Currency)
		switch {
		case delta.IsPositive():
			deltaLabel = "Up " + fmtMoney(delta)
			deltaTone = "text-up"
		case delta.IsNegative():
			deltaLabel = "Down " + fmtMoney(delta.Abs())
			deltaTone = "text-down"
		}
		low, high := first, first
		for _, m := range series[1:] {
			if m.Amount < low.Amount {
				low = m
			}
			if m.Amount > high.Amount {
				high = m
			}
		}
		rangeLabel = fmt.Sprintf("%s - %s", fmtMoney(low), fmtMoney(high))
	}
	// Plot in major units (dollars), not raw minor units (cents): feeding cents
	// made the Y axis read "2,000,000 / 1,500,000 …" and clip in the narrow
	// widget (C16). The Y-axis format hint renders ticks as compact currency
	// ("$20k"); see web/chart.js.
	div := 1.0
	for i := 0; i < currency.Decimals(net.Currency); i++ {
		div *= 10
	}
	pts := make([]chartspec.Point, len(series))
	for i, m := range series {
		label := trendPointLabel(cutoffs[i], months)
		// C215: the final cutoff is next-month-start, so it captures the current
		// month's data "so far" — label it as the current month + "(so far)" instead
		// of the next month's name, which read as a confusing unlabeled partial point.
		if i == len(series)-1 {
			label = trendPointLabel(start, months) + " " + uistate.T("dashboard.trendSoFar")
		}
		pts[i] = chartspec.Point{X: float64(i), Y: float64(m.Amount) / div, Label: label}
	}
	yFmt := ".3~s" // compact SI w/ enough precision to keep narrow-range ticks distinct, e.g. "21.4k"
	if currency.Symbol(net.Currency) == "$" {
		yFmt = "$.3~s" // "$21.4k" for dollar currencies
	}
	spec := chartspec.Spec{
		Kind:   chartspec.Area,
		Series: []chartspec.Series{{Name: "Net worth", Points: pts}}, // empty Color → theme accent
		X:      chartspec.Axis{Label: "Time"},
		Y:      chartspec.Axis{Format: yFmt},
	}
	if !showXAxis {
		spec.X.Format = "hidden"
	}
	body := Div(css.Class("trend-body"),
		Div(css.Class("trend-head"),
			Div(css.Class("trend-figure fig t-figure", tw.FontDisplay), fmtMoney(net)),
			Div(css.Class("trend-standard t-caption", tw.TextDim), trendWindowLabel(months)),
		),
		Div(css.Class("trend-expanded"),
			Div(css.Class("trend-stat"),
				Span(css.Class("t-caption", tw.TextFaint), "Change"),
				Span(ClassStr("fig t-body "+deltaTone), deltaLabel),
			),
			Div(css.Class("trend-stat"),
				Span(css.Class("t-caption", tw.TextFaint), "Range"),
				Span(css.Class("fig t-body", tw.TextDim), rangeLabel),
			),
		),
		uiw.Chart(uiw.ChartProps{
			Spec:   spec,
			Height: "100%",
			Class:  "trend-chart",
			Label:  uistate.T("dashboard.netWorthChartLabel", fmtMoney(net)),
		}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "trend", Title: uistate.T("dashboard.netWorthTrend"), Draggable: true, Resizable: true, GridColumn: "4", GridRow: "3 / span 2",
		BodyClass: tw.Fold(tw.Flex, tw.FlexCol, tw.MinH0), Body: body,
	})
}

// accountsWidget is the 2×1 Accounts widget: a small grid of active account
// balances (accounting figures, negatives toned red) via ledger.Balance. How
// many accounts to show, and whether to show only cleared balances, are
// configurable.
type emptyAddProps struct {
	Message string
	Label   string
	Path    string
}

// emptyAddCTA renders a dashboard widget's empty state with an in-context "add"
// button that routes to the relevant screen, so a user can create data from the
// dashboard instead of hunting for the screen (C23). Its own component so the
// navigate hook stays at a stable position.
func emptyAddCTA(props emptyAddProps) ui.Node {
	nav := router.UseNavigate()
	path := props.Path
	return Div(css.Class("empty t-body", tw.TextDim, tw.Flex, tw.FlexCol, tw.ItemsStart, tw.Gap2),
		Span(props.Message),
		Button(css.Class("btn btn-primary"), Type("button"), OnClick(func() { nav.Navigate(uistate.RoutePath(path)) }), props.Label),
	)
}

func trendWindowLabel(months int) string {
	if months >= 24 && months%12 == 0 {
		return fmt.Sprintf("%d years", months/12)
	}
	if months == 12 {
		return "1 year"
	}
	return fmt.Sprintf("%d months", months)
}

func trendPointLabel(t time.Time, months int) string {
	if months > 36 {
		if t.Month() == time.January {
			return t.Format("2006")
		}
		return t.Format("Jan '06")
	}
	return t.Format("Jan '06")
}

func accountsWidget(app *appstate.App, txns []domain.Transaction, cfg widgetcfg.Config) ui.Node {
	limit, cleared := 6, false
	if sch, ok := widgetcfg.SchemaFor("accounts"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			limit = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("cleared"); ok {
			cleared = f.Bool(cfg)
		}
	}
	cells := make([]ui.Node, 0, limit)
	for _, a := range app.Accounts() {
		if a.Archived {
			continue
		}
		var bal money.Money
		if cleared {
			bal, _ = ledger.ClearedBalance(a, txns)
		} else {
			bal, _ = ledger.Balance(a, txns)
		}
		tone := ""
		if bal.IsNegative() {
			tone = "text-down"
		}
		cells = append(cells, Div(
			Div(css.Class(tw.TextDim), a.Name),
			Div(ClassStr("fig t-body "+tw.Fold(tw.FontDisplay, tw.Mt05)+" "+tw.ColorClass(tone)), fmtMoney(bal)),
		))
		if len(cells) >= limit {
			break
		}
	}
	var body ui.Node
	if len(cells) == 0 {
		body = ui.CreateElement(emptyAddCTA, emptyAddProps{Message: "No accounts yet.", Label: uistate.T("dashboard.addAccount"), Path: "/accounts"})
	} else {
		body = Div(css.Class("t-body", tw.Grid, tw.GridCols3, tw.Gap4), cells)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "accounts", Title: uistate.T("nav.accounts"), Draggable: true, Resizable: true, GridColumn: "3 / span 2", GridRow: "5",
		Body: body,
	})
}

// todoWidget is the 1×1 To-do widget: the next few open tasks (how many is
// configurable, default 3), dot-toned by priority (high = amber, others =
// dim/faint).
func todoWidget(app *appstate.App, cfg widgetcfg.Config) ui.Node {
	count, sortMode, showCompleted := 3, tasksort.ModeSmart, false
	if sch, ok := widgetcfg.SchemaFor("todo"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			count = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("sort"); ok {
			sortMode = tasksort.ParseMode(f.Str(cfg))
		}
		if f, ok := sch.FieldByKey("showCompleted"); ok {
			showCompleted = f.Bool(cfg)
		}
	}

	all := app.Tasks()
	var openTasks, doneTasks []domain.Task
	for _, t := range all {
		if t.Status == domain.StatusDone {
			doneTasks = append(doneTasks, t)
		} else {
			openTasks = append(openTasks, t)
		}
	}
	// Overdue first, then the chosen order — an overdue cue belongs at the top (C52).
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	overdue := func(t domain.Task) bool { return !t.Due.IsZero() && t.Due.Before(today) }
	ordered := tasksort.OrderBy(openTasks, sortMode)
	var od, rest []domain.Task
	for _, t := range ordered {
		if overdue(t) {
			od = append(od, t)
		} else {
			rest = append(rest, t)
		}
	}
	openOrdered := append(od, rest...)

	if len(openOrdered) == 0 && !(showCompleted && len(doneTasks) > 0) {
		return uiw.Widget(uiw.WidgetProps{
			ID: "todo", Title: uistate.T("nav.todo"), Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
			Body: ui.CreateElement(emptyAddCTA, emptyAddProps{Message: "Nothing to do — nice.", Label: uistate.T("dashboard.addTodo"), Path: "/todo"}),
		})
	}

	shown := openOrdered
	truncated := 0
	if len(shown) > count {
		truncated = len(shown) - count
		shown = shown[:count]
	}

	rows := make([]ui.Node, 0, len(shown)+len(doneTasks)+2)
	for _, t := range shown {
		rows = append(rows, ui.CreateElement(dashTaskRow, dashTaskRowProps{Task: t, Overdue: overdue(t)}))
	}
	if showCompleted {
		done := tasksort.OrderBy(doneTasks, sortMode)
		if len(done) > count {
			done = done[:count]
		}
		for _, t := range done {
			rows = append(rows, ui.CreateElement(dashTaskRow, dashTaskRowProps{Task: t}))
		}
	}
	if truncated > 0 {
		rows = append(rows, ui.CreateElement(todoMoreLink, todoMoreProps{N: truncated}))
	}

	progress := uistate.T("dashboard.todoProgress", len(openOrdered), len(doneTasks))
	body := Div(
		P(css.Class("t-caption", tw.TextDim, tw.Mb2), progress),
		Div(css.Class("t-body", tw.SpaceY15), rows),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "todo", Title: uistate.T("nav.todo"), Draggable: true, Resizable: true, GridColumn: "2", GridRow: "5",
		Body: body,
	})
}

type dashTaskRowProps struct {
	Task    domain.Task
	Overdue bool
}

// dashTaskRow renders one dashboard To-do row with an inline complete checkbox and
// a title that drills into /todo. Its own component so the toggle/nav hooks stay
// at stable positions across the list (the On* loop gotcha). Toggling completion
// writes the task and bumps the data revision (content change, not layout — the
// bento FLIP signature is undisturbed).
func dashTaskRow(props dashTaskRowProps) ui.Node {
	t := props.Task
	nav := router.UseNavigate()
	app := appstate.Default
	rev := uistate.UseDataRevision()
	done := t.Status == domain.StatusDone

	toggle := ui.UseEvent(func() {
		if app == nil {
			return
		}
		nt := t
		if done {
			nt.Status = domain.StatusOpen
		} else {
			nt.Status = domain.StatusDone
		}
		if err := app.PutTask(nt); err == nil {
			rev.Set(rev.Get() + 1)
		}
	})
	openTodo := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/todo")) })

	dotTone, prio := "text-faint", "Low priority"
	var dotContent any = "○"
	switch t.Priority {
	case domain.PriorityHigh:
		dotTone, dotContent, prio = "text-warn", uiw.Icon(icon.AlertTriangle, css.Class(tw.W4, tw.H4, tw.ShrinkO)), "High priority"
	case domain.PriorityMedium:
		dotTone, dotContent, prio = "text-dim", "●", "Medium priority"
	}
	titleCls := tw.Fold(tw.Flex1, tw.TextLeft, tw.Truncate)
	if done {
		titleCls += " " + tw.Fold(tw.LineThrough, tw.TextFaint)
	} else if props.Overdue {
		titleCls += " " + tw.Fold(tw.TextDown)
	}
	checkLabel := uistate.T("dashboard.todoComplete", t.Title)
	return Div(css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter),
		Button(css.Class("dash-check"), Type("button"), Attr("role", "checkbox"), Attr("aria-checked", boolStr(done)),
			Attr("aria-label", checkLabel), Attr("title", checkLabel), OnClick(toggle),
			Text(checkGlyph(done))),
		Span(ClassStr(dotTone), Attr("title", prio), Attr("aria-label", prio), dotContent),
		Button(ClassStr("dash-task "+titleCls), Type("button"), OnClick(openTodo), t.Title),
	)
}

func checkGlyph(done bool) string {
	if done {
		return "☑"
	}
	return "☐"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

type todoMoreProps struct{ N int }

// todoMoreLink is the "+N more →" footer linking to the full To-do screen (no
// silent truncation). Its own component for a stable nav hook.
func todoMoreLink(props todoMoreProps) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/todo")) })
	return Button(css.Class("t-caption", tw.TextDim, tw.HoverTextFg, tw.Mt1), Type("button"), OnClick(open),
		uistate.T("dashboard.todoMore", props.N))
}

// goalsWidget is the 1×1 Goals widget: one goal's progress (% + saved / target)
// via internal/goals. By default it features the first goal; configurably it can
// feature the goal nearest completion, and the target-date caption is optional.
func goalsWidget(app *appstate.App, cfg widgetcfg.Config) ui.Node {
	pr := uistate.UsePrefs().Get()
	byProgress, showDate := false, true
	if sch, ok := widgetcfg.SchemaFor("goals"); ok {
		if f, ok := sch.FieldByKey("byProgress"); ok {
			byProgress = f.Bool(cfg)
		}
		if f, ok := sch.FieldByKey("showDate"); ok {
			showDate = f.Bool(cfg)
		}
	}
	list := app.Goals()
	if len(list) == 0 {
		return uiw.Widget(uiw.WidgetProps{
			ID: "goals", Title: uistate.T("nav.goals"), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
			Body: ui.CreateElement(emptyAddCTA, emptyAddProps{Message: "No goals yet.", Label: uistate.T("dashboard.addGoal"), Path: "/goals"}),
		})
	}
	g := list[0]
	if byProgress {
		// Feature the goal nearest completion (highest percent; first wins ties).
		best := goals.Percent(g)
		for _, cand := range list[1:] {
			if p := goals.Percent(cand); p > best {
				best, g = p, cand
			}
		}
	}
	pct := goals.Percent(g)
	caption := fmt.Sprintf("%d%%", pct)
	if showDate && !g.TargetDate.IsZero() {
		caption += " · by " + pr.FormatDate(g.TargetDate)
	}
	body := Div(
		Div(css.Class("t-body", tw.Flex, tw.JustifyBetween),
			Span(css.Class(tw.TextDim), "saved"),
			Span(css.Class("fig t-body", tw.FontDisplay), fmtMoney(g.CurrentAmount)+" / "+fmtMoney(g.TargetAmount)),
		),
		uiw.ProgressBar(uiw.ProgressBarProps{Percent: pct, Tone: "bg-fg", Class: "mt-2"}),
		Div(css.Class("t-caption", tw.TextDim, tw.Mt15), caption),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "goals", Title: uistate.T("dashboard.goalPrefix", g.Name), Draggable: true, Resizable: true, GridColumn: "1", GridRow: "5",
		Body: body,
	})
}

// budgetsWidget is the 1×2 Budgets widget: spend vs limit per budget with an
// ok/near/over progress bar (via internal/budgeting). It evaluates the shared
// dashboard period window (start/end) so it stays in sync with the top-bar
// time selector. Over-budget rows are clickable links to the Budgets screen.
func budgetsWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time, cfg widgetcfg.Config) ui.Node {
	limit, atRisk := 6, false
	if sch, ok := widgetcfg.SchemaFor("budgets"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			limit = f.Int(cfg)
		}
		if f, ok := sch.FieldByKey("atRisk"); ok {
			atRisk = f.Bool(cfg)
		}
	}
	budgets := app.Budgets()
	// Parent-category budgets roll up their sub-categories' spend (D5).
	cats := app.Categories()
	statuses := make([]budgeting.Status, 0, len(budgets))
	for _, b := range budgets {
		if st, err := budgeting.EvaluateRollup(b, txns, start, end, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); err == nil {
			statuses = append(statuses, st)
		}
	}

	// When "at-risk only" is on, drop budgets that are comfortably on track.
	if atRisk {
		kept := statuses[:0]
		for _, s := range statuses {
			if s.State == budgeting.StateNear || s.State == budgeting.StateOver {
				kept = append(kept, s)
			}
		}
		statuses = kept
	}

	catName := make(map[string]string)
	for _, c := range app.Categories() {
		catName[c.ID] = c.Name
	}

	var body ui.Node
	if len(statuses) == 0 {
		if len(app.Budgets()) == 0 {
			// Genuinely no budgets — offer to add one in context.
			body = ui.CreateElement(emptyAddCTA, emptyAddProps{Message: "No budgets yet.", Label: uistate.T("dashboard.addBudget"), Path: "/budgets"})
		} else {
			// Budgets exist but none match the at-risk filter — not an add case.
			body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noBudgetAlerts"))
		}
	} else {
		if len(statuses) > limit {
			statuses = statuses[:limit]
		}
		rows := make([]ui.Node, 0, len(statuses))
		for _, s := range statuses {
			tone, bar := "text-dim", "bg-up"
			switch s.State {
			case budgeting.StateNear:
				tone, bar = "text-warn", "bg-warn"
			case budgeting.StateOver:
				tone, bar = "text-down", "bg-down"
			}
			label := s.Budget.Name
			if label == "" {
				label = catName[s.Budget.CategoryID]
			}
			rows = append(rows, ui.CreateElement(dashBudgetRow, dashBudgetRowProps{
				Label:   label,
				Percent: s.Percent,
				Tone:    tone,
				Bar:     bar,
				Over:    s.State == budgeting.StateOver,
			}))
		}
		body = Div(css.Class("t-body", tw.SpaceY4), rows)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "budgets", Title: uistate.T("nav.budgets"), Draggable: true, Resizable: true,
		GridColumn: "3", GridRow: "3 / span 2", Body: body,
	})
}

// recentWidget is the 2×2 Recent transactions widget: newest activity as a
// compact table with accounting amounts. Display-only, so rows build in a loop.
func recentWidget(txns []domain.Transaction, cfg widgetcfg.Config) ui.Node {
	pr := uistate.UsePrefs().Get()
	count := 6
	if sch, ok := widgetcfg.SchemaFor("recent"); ok {
		if f, ok := sch.FieldByKey("count"); ok {
			count = f.Int(cfg)
		}
	}
	recent := ledger.Recent(txns, count)
	var body ui.Node
	if len(recent) == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noTransactions"))
	} else {
		rows := make([]ui.Node, 0, len(recent))
		for _, t := range recent {
			rows = append(rows, Tr(css.Class(tw.BorderB, tw.BorderLine70),
				Td(css.Class("fig", tw.Py25, tw.TextDim, tw.W16), pr.FormatDate(t.Date)),
				Td(css.Class(tw.Py25), t.Desc),
				Td(ClassStr("fig "+tw.Fold(tw.Py25, tw.TextRight, tw.FontDisplay)+" "+tw.ColorClass(figTone(t.Amount))), fmtMoney(t.Amount)),
			))
		}
		body = Table(css.Class("t-body", tw.WFull), Tbody(rows))
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: "recent", Title: uistate.T("dashboard.recent"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "3 / span 2", BodyClass: tw.Fold(tw.OverflowHidden),
		Body: body,
	})
}

// dashCustomizeBar (C319) is a compact, right-aligned "Customize" action above
// the bento grid that navigates to /widget-manager (layout mode, show/hide,
// sizes, tile styles). It gives the dashboard canvas a discoverable, keyboard-
// reachable entry point to rearranging itself. Its own component so the nav hook
// sits at a stable position.
func dashCustomizeBar() ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/widget-manager")) })
	return Div(css.Class(tw.Flex, tw.JustifyEnd, tw.ItemsCenter, tw.Mb2),
		Button(css.Class("btn btn-sm", tw.Flex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "dash-customize"),
			Attr("aria-label", uistate.T("dashboard.customizeAria")),
			Attr("title", uistate.T("dashboard.customizeAria")),
			OnClick(open),
			uiw.Icon(icon.Customize, css.Class(tw.W4, tw.H4)),
			uistate.T("dashboard.customize"),
		),
	)
}

// DashboardLayoutControls renders the dashboard layout manager — the Custom/Auto
// mode selector (C24) and a Reset-layout action — for the Settings modal. Custom
// keeps your hand-arranged order; the auto modes reorder the tiles (sizes stay as
// you set them). Switching to Custom bakes the current auto order in so nothing
// jumps. It lived in a wasted full-width header cell on the dashboard; it now
// lives in Settings so the canvas is all widgets.
func DashboardLayoutControls() ui.Node {
	layoutAtom := uistate.UseLayoutItems()
	modeAtom := uistate.UseLayoutMode()
	reset := func() {
		d := dashlayout.DefaultItems()
		layoutAtom.Set(d)
		uistate.PersistItems(d)
	}
	onMode := ui.UseEvent(func(e ui.Event) {
		m := dashlayout.Mode(e.GetValue())
		if !m.Valid() {
			return
		}
		if m == dashlayout.ModeCustom {
			baked := dashlayout.Arrange(layoutAtom.Get(), modeAtom.Get())
			layoutAtom.Set(baked)
			uistate.PersistItems(baked)
		}
		modeAtom.Set(m)
		uistate.PersistLayoutMode(m)
	})
	mode := modeAtom.Get()
	return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3, tw.FlexWrap),
		Select(css.Class("rstep t-caption"), Attr("title", uistate.T("dashboard.layoutMode")), OnChange(onMode),
			Option(Value(string(dashlayout.ModeCustom)), SelectedIf(mode == dashlayout.ModeCustom), uistate.T("dashboard.layoutCustom")),
			Option(Value(string(dashlayout.ModeAutoDefault)), SelectedIf(mode == dashlayout.ModeAutoDefault), uistate.T("dashboard.layoutAutoDefault")),
			Option(Value(string(dashlayout.ModeAutoImportance)), SelectedIf(mode == dashlayout.ModeAutoImportance), uistate.T("dashboard.layoutAutoImportance")),
		),
		Button(css.Class("data-btn"), Type("button"), OnClick(reset), uistate.T("dashboard.reset")),
	)
}

// spanOf returns the intrinsic column/row span of the widget with the given id
// in the current layout, defaulting to 1×1 when absent. The attention widget uses
// it to choose how much detail to render (responsive-by-span).
func spanOf(items []dashlayout.Item, id string) (col, row int) {
	for _, it := range items {
		if it.ID == id {
			c, r := it.ColSpan, it.RowSpan
			if c < 1 {
				c = 1
			}
			if r < 1 {
				r = 1
			}
			return c, r
		}
	}
	return 1, 1
}

// attentionWidget is the headline "Needs attention" digest: the urgent, act-now
// signals (bills due soon, near/over budgets, stale balances, overdue &
// high-priority to-dos, the biggest spending spike), ranked by the pure
// internal/attention package under the widget's gear/flip settings. It is
// responsive-by-span: at 1×1 it shows the single most-urgent item plus a count;
// wider/taller it shows more. Default placement is 4×1 at the top of the grid.
func attentionWidget(app *appstate.App, txns []domain.Transaction, rates currency.Rates, start, end time.Time, dismissals freshness.Dismissals, cfg widgetcfg.Config, spanCol, spanRow int) ui.Node {
	now := time.Now()

	// Budget statuses (near/over are what the digest keeps), rolled up like the
	// Budgets widget so parent budgets include sub-category spend.
	cats := app.Categories()
	statuses := make([]budgeting.Status, 0, len(app.Budgets()))
	bs, be := dateutil.MonthRange(now)
	for _, b := range app.Budgets() {
		if st, err := budgeting.EvaluateRollup(b, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); err == nil {
			statuses = append(statuses, st)
		}
	}

	var anomalyPtr *insights.Anomaly
	if anomalies := detectSpendingAnomalies(txns, cats, rates); len(anomalies) > 0 {
		anomalyPtr = &anomalies[0]
	}

	items := attention.Rank(attention.Inputs{
		Now:     now,
		Bills:   bills.UpcomingAll(app.Accounts(), app.Recurring(), now),
		Budgets: statuses,
		Stale:   freshness.VisibleStaleAccounts(app.Accounts(), app.FreshnessWindows(), dismissals, now),
		Tasks:   app.Tasks(),
		Anomaly: anomalyPtr,
	}, attentionConfig(cfg))

	base := rates.Base
	var body ui.Node
	switch {
	case len(items) == 0:
		body = P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.attentionClear"))
	case spanCol < 2 && spanRow < 2:
		// Compact 1×1: the single most-urgent item, plus a count of the rest.
		rows := []ui.Node{ui.CreateElement(attentionRow, attentionRowProps{Item: items[0], Base: base})}
		if crit, warn := attention.Counts(items); crit+warn > 1 {
			rows = append(rows, P(css.Class("t-caption", tw.TextDim, tw.Mt1), uistate.T("dashboard.attentionMore", crit+warn-boolToInt(items[0].Severity >= attention.SeverityWarning))))
		}
		body = Div(css.Class("attention-list"), rows)
	default:
		rows := make([]ui.Node, 0, len(items))
		for _, it := range items {
			rows = append(rows, ui.CreateElement(attentionRow, attentionRowProps{Item: it, Base: base}))
		}
		// Wide-and-short (e.g. the default 4×1) flows items as wrapping chips; any
		// layout with height stacks them as a list.
		cls := "attention-list"
		if spanRow < 2 {
			cls = "attention-chips"
		}
		body = Div(ClassStr(cls), rows)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: "attention", Title: uistate.T("dashboard.attention"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 4", GridRow: "1", Body: body,
	})
}

// attentionConfig maps the widget's stored gear settings to a typed
// attention.Config, falling back to the schema defaults.
func attentionConfig(cfg widgetcfg.Config) attention.Config {
	out := attention.DefaultConfig()
	sch, ok := widgetcfg.SchemaFor("attention")
	if !ok {
		return out
	}
	boolField := func(key string, dst *bool) {
		if f, ok := sch.FieldByKey(key); ok {
			*dst = f.Bool(cfg)
		}
	}
	boolField("bills", &out.Bills)
	boolField("budgets", &out.Budgets)
	boolField("stale", &out.Stale)
	boolField("tasks", &out.Tasks)
	boolField("spending", &out.Spending)
	if f, ok := sch.FieldByKey("billsDays"); ok {
		out.BillsWindowDays = f.Int(cfg)
	}
	if f, ok := sch.FieldByKey("maxItems"); ok {
		out.MaxItems = f.Int(cfg)
	}
	if f, ok := sch.FieldByKey("minSeverity"); ok {
		switch f.Str(cfg) {
		case "warn":
			out.MinSeverity = attention.SeverityWarning
		case "critical":
			out.MinSeverity = attention.SeverityCritical
		default:
			out.MinSeverity = attention.SeverityInfo
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// digestCap is the most cross-page insights the digest widget shows inline.
// Kept small so the widget is glanceable, not a wall — the full set lives on /smart.
const digestCap = 3

// smartDigestWidget is the "Smart digest" dashboard widget: a compact, cross-page
// glance at the top active insights from all enabled Free engines. Gated by
// AffordanceWidget (Standard density), so it only appears when the user's density
// dial permits dashboard widgets. Strictly additive: if no features are enabled or
// no insights are active, it renders the neutral empty hint rather than nothing,
// so the tile still makes sense in the widget manager. Returns a full bento tile.
func smartDigestWidget(app *appstate.App) ui.Node {
	pr := uistate.UsePrefs().Get()
	settings := uistate.LoadSmartSettings()

	const widgetID = "smart-digest"

	if !settings.DensityOrDefault().Shows(smart.AffordanceWidget) {
		return uiw.Widget(uiw.WidgetProps{
			ID: widgetID, Title: uistate.T("smart.digestTitle"), Draggable: true, Resizable: true,
			GridColumn: "1 / span 2", GridRow: "10",
			Body: P(css.Class("empty t-body", tw.TextDim), uistate.T("smart.digestEmpty")),
		})
	}

	in := buildSmartInput(app, pr.WeekStartWeekday())
	all := smartengine.Run(in, settings)
	if len(all) > digestCap {
		all = all[:digestCap]
	}

	var body ui.Node
	if len(all) == 0 {
		body = P(css.Class("empty t-body", tw.TextDim), uistate.T("smart.digestEmpty"))
	} else {
		body = Div(css.Class("t-body", tw.Flex, tw.FlexCol, tw.Gap2),
			Attr("data-testid", "smart-digest-list"),
			smartInsightList(all),
		)
	}
	return uiw.Widget(uiw.WidgetProps{
		ID: widgetID, Title: uistate.T("smart.digestTitle"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "10",
		Body: body,
	})
}

// attentionRowProps configures one attention digest row.
type attentionRowProps struct {
	Item attention.Item
	Base string
}

// attentionRow renders one urgent item as a clickable line — a severity dot, the
// plain-English detail, and (when one exists) a deep link that navigates to the
// item's screen and scrolls to it. Its own component so the navigate hook stays
// at a stable position across the list.
func attentionRow(props attentionRowProps) ui.Node {
	nav := router.UseNavigate()
	it := props.Item
	open := func() {
		if it.Route == "" {
			return
		}
		nav.Navigate(uistate.RoutePath(it.Route))
		if it.AnchorID != "" {
			scrollToID(it.AnchorID)
		}
	}
	return Button(ClassStr("attention-item "+attentionTone(it.Severity)), Type("button"), OnClick(open),
		Attr("title", uistate.T("dashboard.attentionOpen")),
		Span(css.Class("attention-dot"), Attr("aria-hidden", "true"), attentionGlyph(it.Severity)),
		Span(css.Class("attention-text"), attentionText(it, props.Base)),
	)
}

// attentionText renders the plain-English line for an item from its structured
// fields, localizing at the edge.
func attentionText(it attention.Item, base string) string {
	switch it.Kind {
	case attention.KindBill:
		when := uistate.T("dashboard.attentionDueToday")
		switch {
		case it.Days == 1:
			when = uistate.T("dashboard.attentionDueTomorrow")
		case it.Days > 1:
			when = uistate.T("dashboard.attentionDueInDays", it.Days)
		}
		return uistate.T("dashboard.attentionBill", it.Label, when, fmtMoney(it.Amount))
	case attention.KindBudget:
		if it.Severity >= attention.SeverityCritical {
			return uistate.T("dashboard.attentionBudgetOver", it.Label, it.Pct)
		}
		return uistate.T("dashboard.attentionBudgetNear", it.Label, it.Pct)
	case attention.KindStale:
		return uistate.T("dashboard.attentionStale", it.Label, it.Days)
	case attention.KindTask:
		if it.Severity >= attention.SeverityCritical {
			return uistate.T("dashboard.attentionTaskOverdue", it.Label, it.Days)
		}
		return uistate.T("dashboard.attentionTaskHigh", it.Label)
	case attention.KindSpending:
		if it.Anomaly != nil {
			return highlightText(*it.Anomaly, base)
		}
	}
	return it.Label
}

func attentionTone(s attention.Severity) string {
	switch s {
	case attention.SeverityCritical:
		return "is-critical"
	case attention.SeverityWarning:
		return "is-warning"
	default:
		return "is-info"
	}
}

func attentionGlyph(s attention.Severity) ui.Node {
	switch s {
	case attention.SeverityCritical:
		return uiw.Icon(icon.AlertTriangle, css.Class(tw.W4, tw.H4, tw.ShrinkO))
	case attention.SeverityWarning:
		return Text("●")
	default:
		return Text("○")
	}
}

// plural renders a count with a singular/plural noun, e.g. "1 deposit" or
// "3 deposits".
func plural(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

// kpiBody renders a KPI tile's body: a large accounting figure with a small
// subline. figTone/subTone are color classes (e.g. "text-up", "text-dim").
func kpiBody(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(ClassStr("fig t-figure "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(figTone)),
			Attr("data-countup", ""), figure),
		Div(ClassStr("t-caption "+tw.Fold(tw.Pt15)+" "+tw.ColorClass(subTone)), subline),
	)
}

// kpiBodyHero renders the visual-hero variant of a KPI tile body: a larger
// figure (t-figure-lg) so the headline number draws the eye first. Used for
// Net Worth, the most important single number on the dashboard (L33).
func kpiBodyHero(figure, figTone, subline, subTone string) ui.Node {
	return Div(
		Div(ClassStr("fig t-figure-lg "+tw.Fold(tw.FontDisplay, tw.LeadingTight)+" "+tw.ColorClass(figTone)),
			Attr("data-countup", ""), figure),
		Div(ClassStr("t-caption "+tw.Fold(tw.Pt15)+" "+tw.ColorClass(subTone)), subline),
	)
}

// dashBudgetRowProps configures one budget row in the dashboard budgets widget.
type dashBudgetRowProps struct {
	Label   string
	Percent int
	Tone    string // color class for the figure, e.g. "text-down"
	Bar     string // progress bar tone class, e.g. "bg-down"
	Over    bool   // true when the budget is over-limit (drives the drill link)
}

// dashBudgetRow renders one budget progress row in the dashboard budgets
// widget. When the budget is over its limit the row is a button that navigates
// to /budgets so the user can act immediately. Its own component so the
// navigate hook stays at a stable position across the variable-length list
// (the On* loop gotcha).
func dashBudgetRow(props dashBudgetRowProps) ui.Node {
	nav := router.UseNavigate()
	openBudgets := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath("/budgets")) })

	header := Div(css.Class(tw.Flex, tw.JustifyBetween),
		Span(props.Label),
		Span(ClassStr("fig "+tw.Fold(tw.FontDisplay)+" "+tw.ColorClass(props.Tone)), fmt.Sprintf("%d%%", props.Percent)),
	)
	bar := uiw.ProgressBar(uiw.ProgressBarProps{Percent: props.Percent, Tone: props.Bar, Class: "mt-1.5"})

	if props.Over {
		// Over-budget rows are actionable: clicking opens the Budgets screen.
		return Button(css.Class("budget-over-row", tw.WFull, tw.TextLeft),
			Type("button"),
			Attr("aria-label", uistate.T("dashboard.budgetDrillTitle")),
			Attr("title", uistate.T("dashboard.budgetDrillTitle")),
			OnClick(openBudgets),
			header, bar,
		)
	}
	return Div(header, bar)
}

// anomalyHubRowProps carries one SMART anomaly finding to its per-row component.
type anomalyHubRowProps struct {
	Insight smart.Insight
	Route   string
	OnClick func()
}

// anomalyHubRow renders one flagged-activity row on the dashboard anomaly-hub
// widget. It is its own component so OnClick registers at a stable hook position
// across the list (no On* in loops). Reuses the same visual treatment as
// SmartAnomalyInsightRow on /insights.
func anomalyHubRow(p anomalyHubRowProps) ui.Node {
	navigate := ui.UseEvent(func() { p.OnClick() })
	iconName := icon.AlertTriangle
	if p.Insight.Severity == smart.SeverityInfo {
		iconName = icon.AlertCircle
	}
	return Button(
		css.Class("insight-row insight-row-action"),
		Type("button"),
		Attr("aria-label", p.Insight.Title),
		OnClick(navigate),
		Span(ClassStr("insight-dot text-down"), uiw.Icon(iconName, css.Class(tw.W4, tw.H4))),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.ItemsStart, tw.MinW0),
			Span(css.Class(tw.Text14, tw.FontMedium, tw.Truncate), p.Insight.Title),
			Span(css.Class("muted", tw.Text13, tw.Truncate), p.Insight.Detail),
		),
	)
}

// anomalyHubViewAllProps carries the navigation callback to the drill-through button.
type anomalyHubViewAllProps struct {
	OnClick func()
}

// anomalyHubViewAll is the "View full analysis" link at the bottom of the widget.
// Its own component keeps the navigate hook at a stable position outside any loop.
func anomalyHubViewAll(p anomalyHubViewAllProps) ui.Node {
	open := ui.UseEvent(func() { p.OnClick() })
	return Button(
		css.Class("btn-link t-caption", tw.Mt2, tw.SelfStart),
		Type("button"),
		Attr("aria-label", uistate.T("dashboard.anomalyHubViewAllAria")),
		OnClick(open),
		uistate.T("dashboard.anomalyHubViewAll"),
	)
}

// anomalyHubWidget is the R25 "Flagged activity" dashboard tile. It runs the four
// anomaly-type SMART detectors (SMART-A1 balance, SMART-T2 duplicates, SMART-T6
// spending spikes, SMART-T7 missing transaction) unconditionally — no Smart opt-in
// gate — and surfaces the top 1–3 findings as a compact bento widget. Drill-through
// navigates to /insights for the full analysis. Returns a full bento tile.
func anomalyHubWidget(app *appstate.App) ui.Node {
	nav := router.UseNavigate()

	const widgetID = "anomaly-hub"
	const maxRows = 3

	pr := uistate.UsePrefs().Get()
	in := buildSmartInput(app, pr.WeekStartWeekday())
	freeSettings := smart.EnableFreeOnly(smart.Settings{})
	all := smartengine.Run(in, freeSettings)

	// Keep only the four anomaly detector codes (same filter as /insights).
	anomalyCodes := map[string]bool{
		"SMART-A1": true,
		"SMART-T2": true,
		"SMART-T6": true,
		"SMART-T7": true,
	}
	var flagged []smart.Insight
	for _, ins := range all {
		if anomalyCodes[ins.Feature] {
			flagged = append(flagged, ins)
		}
	}

	// Cap at maxRows so the widget stays glanceable.
	if len(flagged) > maxRows {
		flagged = flagged[:maxRows]
	}

	toInsights := func() { nav.Navigate(uistate.RoutePath("/insights")) }

	var body ui.Node
	if len(flagged) == 0 {
		body = Div(
			P(css.Class("t-body", tw.TextUp), uistate.T("dashboard.anomalyHubClear")),
			ui.CreateElement(anomalyHubViewAll, anomalyHubViewAllProps{OnClick: toInsights}),
		)
	} else {
		rows := make([]ui.Node, 0, len(flagged))
		for _, ins := range flagged {
			route := "/transactions"
			if ins.Page == smart.PageAccounts {
				route = "/accounts"
			}
			capturedIns := ins
			capturedRoute := route
			rows = append(rows, ui.CreateElement(anomalyHubRow, anomalyHubRowProps{
				Insight: capturedIns,
				Route:   capturedRoute,
				OnClick: func() { nav.Navigate(uistate.RoutePath(capturedRoute)) },
			}))
		}
		body = Div(
			P(css.Class("t-caption", tw.TextDim, tw.Mb2), uistate.T("dashboard.anomalyHubHint")),
			Div(css.Class("insight-list"), rows),
			ui.CreateElement(anomalyHubViewAll, anomalyHubViewAllProps{OnClick: toInsights}),
		)
	}

	return uiw.Widget(uiw.WidgetProps{
		ID: widgetID, Title: uistate.T("dashboard.anomalyHubTitle"), Draggable: true, Resizable: true,
		GridColumn: "1 / span 2", GridRow: "11",
		Body: body,
	})
}
