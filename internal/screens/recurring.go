// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — recurring.go holds the /recurring route: a three-tab hub
// (Scheduled / Bills / Subscriptions, FEATURE_MAP §5.3) whose Scheduled tab is a
// widgetized bento surface over the recurring cash-flow schedule. Adding/editing
// a flow happens in a shell-root flip modal (RecurringEditHost + RecurringForm).
package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/subscriptions"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// recurCadence localizes a recurring cadence (local to the recurring surface so it
// has no dependency on other screens' helpers).
func recurCadence(c domain.RecurringCadence) string {
	switch c {
	case domain.CadenceWeekly:
		return uistate.T("recurring.cadenceWeekly")
	case domain.CadenceBiweekly:
		return uistate.T("recurring.cadenceBiweekly")
	case domain.CadenceSemimonthly:
		return uistate.T("recurring.cadenceSemimonthly")
	case domain.CadenceQuarterly:
		return uistate.T("recurring.cadenceQuarterly")
	case domain.CadenceYearly:
		return uistate.T("recurring.cadenceYearly")
	default:
		return uistate.T("recurring.cadenceMonthly")
	}
}

// recurOccurrence is one concrete due date derived from a flow's schedule.
type recurOccurrence struct {
	R       domain.Recurring
	Date    time.Time
	Overdue bool
	// Paid is true when this occurrence carries a durable bill-match link (TX9) —
	// a real transaction settled it. Variance is that payment's actual magnitude
	// minus the expected amount (signed; positive = ran over), valid only when
	// Paid.
	Paid     bool
	Variance int64
}

// recurView is the derived render model every Scheduled-tab tile shares. Pure —
// built once per render from the live store.
type recurView struct {
	Base        string
	Dec         int
	Flows       []domain.Recurring // sorted soonest-due first
	MonthlyIn   int64              // Σ positive monthly equivalents
	MonthlyOut  int64              // Σ |negative| monthly equivalents (positive figure)
	MonthlyNet  int64
	Upcoming    []recurOccurrence // overdue + next 30 days, sorted by date
	UpcomingIn  int64
	UpcomingOut int64
	Detected    []subscriptions.Subscription // in history, not yet planned
	// VarPrefixByID maps a flow ID to its engine-variable prefix ("recurring_<slug>_")
	// — the flow's stable formula identity. Computed over the flows in STORE order
	// (the same order the engine disambiguates in), so the name shown on a card is
	// exactly the name the formula surface exposes.
	VarPrefixByID map[string]string
	// BudgetedCats holds the category IDs that have a budget, so a flow can offer a
	// "View budget" jump when its category is budgeted.
	BudgetedCats map[string]bool
}

// computeRecurView builds the shared model: monthly-equivalent totals, the derived
// due dates for the next 30 days (overdue included, capped per flow so a stale
// schedule can't flood the list), and the detected-but-unplanned charges.
func computeRecurView(app *appstate.App, now time.Time) recurView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	v := recurView{Base: base, Dec: currency.Decimals(base)}

	v.Flows = append(v.Flows, app.Recurring()...)
	// Formula identities are disambiguated in store order (matching the engine) —
	// capture them BEFORE the display sort so a card shows the exact variable name
	// the formula surface exposes.
	v.VarPrefixByID = map[string]string{}
	for _, b := range engineenv.RecurringVarBases(v.Flows) {
		v.VarPrefixByID[b.Recurring.ID] = b.Prefix
	}
	v.BudgetedCats = map[string]bool{}
	for _, bg := range app.Budgets() {
		if bg.CategoryID != "" {
			v.BudgetedCats[bg.CategoryID] = true
		}
	}
	sort.SliceStable(v.Flows, func(i, j int) bool { return v.Flows[i].NextDue.Before(v.Flows[j].NextDue) })

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, 30)
	for _, r := range v.Flows {
		me := r.MonthlyEquivalent()
		if me >= 0 {
			v.MonthlyIn += me
		} else {
			v.MonthlyOut += -me
		}
		v.MonthlyNet += me

		// Walk the schedule from NextDue to the cutoff. Cap iterations so a NextDue
		// far in the past (many missed occurrences) can't generate an unbounded list.
		d := r.NextDue
		for i := 0; i < 8 && !d.After(cutoff); i++ {
			occ := recurOccurrence{R: r, Date: d, Overdue: d.Before(today)}
			// TX9: surface whether a real transaction has already settled this
			// occurrence (paid check + variance).
			if r.Amount.IsNegative() {
				if _, ok := app.BillMatchForOccurrence(r.ID, d); ok {
					occ.Paid = true
					if variance, vok := app.BillMatchVariance(r.ID, d, r.Amount.Amount); vok {
						occ.Variance = variance
					}
				}
			}
			v.Upcoming = append(v.Upcoming, occ)
			if r.Amount.Amount >= 0 {
				v.UpcomingIn += r.Amount.Amount
			} else {
				v.UpcomingOut += -r.Amount.Amount
			}
			d = r.Cadence.Next(d)
		}
	}
	sort.SliceStable(v.Upcoming, func(i, j int) bool { return v.Upcoming[i].Date.Before(v.Upcoming[j].Date) })

	// Detected charges not already planned (and not liability payments, which would
	// double-count a loan/card autopay).
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	existing := map[string]bool{}
	for _, r := range v.Flows {
		existing[strings.ToLower(strings.TrimSpace(r.Label))] = true
	}
	detected, _ := subscriptions.Detect(app.Transactions(), rates, 3)
	for _, s := range detected {
		if existing[strings.ToLower(strings.TrimSpace(s.Name))] {
			continue
		}
		if subscriptions.IsLiabilityPayment(s, app.Transactions(), app.Accounts()) {
			continue
		}
		v.Detected = append(v.Detected, s)
	}
	return v
}

// recurTile wraps a tile body in the shared Widget chrome + the full-width bento column.
func recurTile(tid string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// recurSection wraps a tile body with a serif section title + optional action, reusing
// the debt-section chrome so /recurring matches the other redesigned surfaces.
func recurSection(sid, title string, action, body ui.Node) ui.Node {
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

// recurStatChip renders one headline figure (reuses the debt-stat chrome).
func recurStatChip(label, value, valueCls string) ui.Node {
	return Div(css.Class("debt-stat"),
		Div(css.Class("debt-stat-label", tw.TextDim), label),
		Div(ClassStr("debt-stat-value "+tw.Fold(tw.FontDisplay)+valueCls), value),
	)
}

// addRecurringButton is the tiny isolated subscriber that opens the add-flow modal
// (only it + the host re-render on open/close, never the surface tiles).
func addRecurringButton() ui.Node {
	edit := uistate.UseRecurringEditID()
	open := ui.UseEvent(Prevent(func() { edit.Set("new") }))
	return Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "recurring-add"), Title(uistate.T("recurring.addFlowTitle")), OnClick(open),
		uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("recurring.addFlow")))
}

// recurScheduledSurface is the Scheduled tab: a bento host of tiles over the shared
// view model — hero (monthly net + figures), toolbar (post due / add), the next-30-days
// schedule, the flow cards, and any detected-but-unplanned charges.
func recurScheduledSurface() ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	postMsg := ui.UseState("")
	postDue := ui.UseEvent(Prevent(func() {
		n, err := app.PostDueRecurring(time.Now())
		if err != nil {
			postMsg.Set(err.Error())
			return
		}
		postMsg.Set(uistate.T("recurring.posted", plural(n, "transaction")))
		uistate.BumpDataRevision()
	}))

	nav := router.UseNavigate()
	showFormulas := ui.UseState(false)
	toggleFormulas := ui.UseEvent(Prevent(func() { showFormulas.Set(!showFormulas.Get()) }))
	onViewAccount := func(string) { nav.Navigate(uistate.RoutePath("/accounts")) }
	// onViewTxns jumps to /transactions pre-filtered to where this flow's money
	// actually moves: its linked account and/or category, falling back to a text
	// match on the flow's label when neither is linked. The filter atom is captured
	// at render (hooks can't run inside a click handler).
	txFilter := uistate.UseTxFilter()
	onViewTxns := func(r domain.Recurring) {
		f := uistate.TxFilter{Account: r.AccountID, Category: r.CategoryID}
		if r.AccountID == "" && r.CategoryID == "" {
			f.Text = r.Label
		}
		nf := f.Normalize()
		txFilter.Set(nf)
		uistate.PersistTxFilter(nf)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	onViewBudget := func(domain.Recurring) { nav.Navigate(uistate.RoutePath("/budgets")) }
	edit := uistate.UseRecurringEditID()
	onEdit := func(rid string) { edit.Set(rid) }
	onDelete := func(rid string) {
		name := ""
		for _, r := range app.Recurring() {
			if r.ID == rid {
				name = r.Label
				break
			}
		}
		uistate.ConfirmModal(uistate.T("recurring.deleteConfirm", name), true, func(ok bool) {
			if ok {
				_ = app.DeleteRecurring(rid)
				uistate.BumpDataRevision()
			}
		})
	}
	v := computeRecurView(app, time.Now())

	// One-click "add to plan" for a detected charge (charges are expenses → stored
	// negative, matching the sign convention).
	onAddDetected := func(s subscriptions.Subscription) {
		nextDue := s.NextRenewal
		if nextDue.IsZero() {
			nextDue = time.Now()
		}
		r := domain.Recurring{
			ID: id.New(), Label: s.Name, Amount: money.New(-s.Amount, v.Base),
			Cadence: domain.RecurringCadence(string(s.Cadence)), NextDue: nextDue,
		}
		if err := app.PutRecurring(r); err == nil {
			uistate.BumpDataRevision()
		}
	}

	// How many auto-post flows are actually due — the Post-due button carries the
	// count so the action's scope is visible before clicking.
	dueNow := 0
	for _, r := range v.Flows {
		if r.Autopost && !r.NextDue.After(time.Now()) {
			dueNow++
		}
	}
	// Overdue occurrences (same figure the stats chip shows) include MANUAL flows,
	// which the Post-due button never posts — only auto-post flows with a linked
	// account are catch-up posted. The button carries the overdue count so it can
	// explain why "OVERDUE 3" can sit beside "Post due now (0)".
	overdue := 0
	for _, occ := range v.Upcoming {
		if occ.Overdue {
			overdue++
		}
	}

	tiles := []ui.Node{
		recurHeroTile(v),
		recurToolbarTile(postMsg.Get(), dueNow, overdue, showFormulas.Get(), postDue, toggleFormulas),
		recurUpcomingTile(v),
		recurFlowsTile(v, recurFlowActions{
			OnEdit: onEdit, OnDelete: onDelete,
			OnViewAccount: onViewAccount, OnViewTxns: onViewTxns, OnViewBudget: onViewBudget,
		}),
	}
	if len(v.Detected) > 0 {
		tiles = append(tiles, recurDetectedTile(v, onAddDetected))
	}
	if showFormulas.Get() {
		tiles = append(tiles, recurTile("recur-formula", Fragment(
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("recurring.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("recurring.metricsTitle"), ShowSaved: true}),
		)))
	}
	return Div(css.Class("bento bento-recurring"), tiles)
}

// recurFlowActions bundles the per-flow callbacks a card can invoke.
type recurFlowActions struct {
	OnEdit        func(string)
	OnDelete      func(string)
	OnViewAccount func(string)
	OnViewTxns    func(domain.Recurring)
	OnViewBudget  func(domain.Recurring)
}

// recurHeroTile is the headline: the net monthly figure in the display serif beside
// the figure chips. The in/out split lives ONLY in the chips (no duplicate sub-line);
// the fourth chip is the most timely fact — the overdue count when anything is
// overdue (danger-toned), else the next due date.
func recurHeroTile(v recurView) ui.Node {
	net := money.New(v.MonthlyNet, v.Base)
	tone := " " + tw.ColorClass("text-up")
	if v.MonthlyNet < 0 {
		tone = " " + tw.ColorClass("text-down")
	}

	overdue := 0
	var nextDue time.Time
	for _, occ := range v.Upcoming {
		if occ.Overdue {
			overdue++
		} else if nextDue.IsZero() {
			nextDue = occ.Date
		}
	}
	var timelyChip ui.Node = Fragment()
	switch {
	case overdue > 0:
		timelyChip = recurStatChip(uistate.T("recurring.figOverdue"), fmt.Sprintf("%d", overdue), " "+tw.ColorClass("text-down"))
	case !nextDue.IsZero():
		timelyChip = recurStatChip(uistate.T("recurring.figNextDue"), nextDue.Format("Jan 2"), "")
	}

	body := Div(css.Class("rec-hero"), Attr("id", "sec-overview"),
		Div(css.Class("rec-hero-main"),
			Div(css.Class("rec-hero-label", tw.TextDim), uistate.T("recurring.heroLabel")),
			Div(ClassStr("rec-hero-value "+tw.Fold(tw.FontDisplay)+tone), Attr("data-testid", "recurring-net"), fmtMoney(net)),
		),
		Div(css.Class("debt-chips"),
			recurStatChip(uistate.T("recurring.figIn"), fmtMoney(money.New(v.MonthlyIn, v.Base)), " "+tw.ColorClass("text-up")),
			recurStatChip(uistate.T("recurring.figOut"), fmtMoney(money.New(v.MonthlyOut, v.Base)), " "+tw.ColorClass("text-down")),
			recurStatChip(uistate.T("recurring.figFlows"), fmt.Sprintf("%d", len(v.Flows)), ""),
			timelyChip,
		),
	)
	return recurTile("recur-hero", recurSection("", uistate.T("recurring.title"), nil,
		Fragment(P(css.Class("muted"), uistate.T("recurring.overviewHint")), body)))
}

// recurToolbarTile holds the actions: Post due (labelled with how many auto-post
// flows it will act on, and accent-outlined when that's non-zero so the timely
// action has real affordance), the schedule-metrics toggle, and Add recurring.
func recurToolbarTile(postedMsg string, dueNow, overdue int, showFormulas bool, onPostDue, onToggleFormulas any) ui.Node {
	postCls := "btn"
	if dueNow > 0 {
		postCls += " rec-postdue-hot"
	}
	// The default title says what posting covers (auto-post items). When nothing
	// is auto-postable yet items are overdue, the overdue ones are manual — say so
	// plainly so the count mismatch isn't confusing.
	postTitle := uistate.T("recurring.postDueTitle")
	if dueNow == 0 && overdue > 0 {
		postTitle = uistate.T("recurring.postDueTitleManual", plural(overdue, "overdue item"))
	}
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("recurring.metricsShow")
	if showFormulas {
		metricsCls += " is-on"
		metricsLabel = uistate.T("recurring.metricsHide")
	}
	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(ClassStr(postCls), Type("button"), Attr("data-testid", "recurring-post-due"),
				Title(postTitle), OnClick(onPostDue),
				fmt.Sprintf("%s (%d)", uistate.T("recurring.postDue"), dueNow)),
			Button(ClassStr(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(showFormulas)),
				Attr("data-testid", "recurring-toggle-formulas"), Title(uistate.T("recurring.metricsTitle")),
				OnClick(onToggleFormulas), Text(metricsLabel)),
			If(postedMsg != "", Span(css.Class("muted"), Attr("data-testid", "recurring-post-msg"), Attr("role", "status"), postedMsg)),
		),
		ui.CreateElement(addRecurringButton),
	)
	return recurTile("recur-toolbar", toolbar)
}

// recurUpcomingTile lists every derived due date in the next 30 days (overdue first).
func recurUpcomingTile(v recurView) ui.Node {
	const maxRows = 10
	var body ui.Node
	if len(v.Upcoming) == 0 {
		body = P(css.Class("empty"), Attr("data-testid", "recurring-upcoming-none"), uistate.T("recurring.upcomingNone"))
	} else {
		rows := []any{css.Class("rec-up-list"), Attr("role", "list")}
		prevDay := ""
		for i, occ := range v.Upcoming {
			if i >= maxRows {
				rows = append(rows, P(css.Class("muted rec-up-more"), uistate.T("recurring.upcomingMore", len(v.Upcoming)-maxRows)))
				break
			}
			// Same-day rows share one date medallion: only the first of a day shows it
			// (the rest carry a ghost spacer), so the date isn't re-stamped per line.
			day := occ.Date.Format("2006-01-02")
			rows = append(rows, recurUpcomingRow(occ, day != prevDay))
			prevDay = day
		}
		body = Fragment(
			P(css.Class("muted rec-up-meta"), Attr("data-testid", "recurring-upcoming-meta"),
				uistate.T("recurring.upcomingMeta", plural(len(v.Upcoming), "payment"),
					fmtMoney(money.New(v.UpcomingOut, v.Base)), fmtMoney(money.New(v.UpcomingIn, v.Base)))),
			Div(rows...),
		)
	}
	return recurTile("recur-upcoming", recurSection("sec-upcoming", uistate.T("recurring.upcomingTitle"), nil,
		Fragment(P(css.Class("muted"), uistate.T("recurring.upcomingHint")), body)))
}

// recurFlowsTile is the schedule itself: one card per flow.
func recurFlowsTile(v recurView, acts recurFlowActions) ui.Node {
	var body ui.Node
	if len(v.Flows) == 0 {
		body = Div(css.Class("rec-empty"), Attr("data-testid", "recurring-empty"),
			P(css.Class("muted"), uistate.T("recurring.empty")),
			ui.CreateElement(addRecurringButton),
		)
	} else {
		body = Div(css.Class("rec-flow-list"), Attr("role", "list"), MapKeyed(v.Flows,
			func(r domain.Recurring) any { return r.ID },
			func(r domain.Recurring) ui.Node {
				return ui.CreateElement(recurFlowCard, recurFlowCardProps{
					R: r, Base: v.Base, OutTotal: v.MonthlyOut,
					VarPrefix: v.VarPrefixByID[r.ID], HasBudget: r.CategoryID != "" && v.BudgetedCats[r.CategoryID],
					Actions: acts,
				})
			}))
	}
	return recurTile("recur-flows", recurSection("sec-flows", uistate.T("recurring.flowsTitle"), nil,
		Fragment(P(css.Class("muted"), uistate.T("recurring.flowsHint")), body)))
}

// recurDetectedTile surfaces repeating charges found in history but not yet planned.
func recurDetectedTile(v recurView, onAdd func(subscriptions.Subscription)) ui.Node {
	rows := []any{css.Class("rec-detected-list"), Attr("data-testid", "detected-recurring")}
	for _, s := range v.Detected {
		sub := s
		rows = append(rows, ui.CreateElement(recurDetectedRow, recurDetectedRowProps{
			Name:  sub.Name,
			Meta:  uistate.T("recurring.detectedMonthly", fmtMoney(money.New(sub.MonthlyAmount(), v.Base)), recurCadence(domain.RecurringCadence(string(sub.Cadence)))),
			OnAdd: func() { onAdd(sub) },
		}))
	}
	return recurTile("recur-detected", recurSection("sec-detected",
		uistate.T("recurring.detectedTitle", plural(len(v.Detected), "charge")), nil,
		Fragment(P(css.Class("muted"), uistate.T("recurring.detectedHint")), Div(rows...))))
}

// RecurringHubProps holds configuration for RecurringHub (empty today; the struct
// exists so future props can be added without altering callers).
type RecurringHubProps struct{}

// RecurringHub owns the three-tab /recurring hub (FEATURE_MAP §5.3): Scheduled (the
// redesigned bento surface), Bills, and Subscriptions. Each tab body is its own
// component so hooks stay isolated across tab switches.
func RecurringHub(p RecurringHubProps) ui.Node {
	activeTab := ui.UseState("scheduled")

	tab := activeTab.Get()
	var content ui.Node
	switch tab {
	case "bills":
		content = ui.CreateElement(BillsPanel, BillsPanelProps{})
	case "subscriptions":
		content = ui.CreateElement(SubscriptionsPanel, SubscriptionsPanelProps{})
	default:
		content = ui.CreateElement(recurScheduledSurface)
	}

	return Div(
		// XC9: payday pre-flight ritual card, shown once per pay cycle.
		ui.CreateElement(paydayPreflightCard),
		// XC5: price-creep notices with the in-place accept flow.
		ui.CreateElement(priceCreepNotices),
		Div(css.Class(tw.Mb2),
			uiw.Segmented(uiw.SegmentedProps{
				Label:    uistate.T("recurring.viewAria"),
				Selected: tab,
				OnSelect: func(v string) { activeTab.Set(v) },
				Options: []uiw.SegOption{
					{Value: "scheduled", Label: uistate.T("recurring.tabScheduled"), TestID: "recurring-tab-scheduled"},
					{Value: "bills", Label: uistate.T("recurring.tabBills"), TestID: "recurring-tab-bills"},
					{Value: "subscriptions", Label: uistate.T("recurring.tabSubscriptions"), TestID: "recurring-tab-subscriptions"},
				},
			}),
		),
		content,
	)
}

// Recurring is the /recurring route — the dedicated "Money that repeats" page.
func Recurring() ui.Node {
	return ui.CreateElement(RecurringHub, RecurringHubProps{})
}
