// SPDX-License-Identifier: MIT

//go:build js && wasm

// Package screens — recurring.go holds the unified Bills & Recurring surface (the
// "month's rhythm" page). It replaces the old Scheduled | Bills | Subscriptions
// tabs with ONE full-width stack — a tideline hero, an overdue strip, a review
// strip, the up-next agenda (compact | calendar), the lineup roster, and a
// findings strip — served on /recurring, /bills, and /subscriptions alike. The
// vocabulary is from-scratch (rhy-*, internal/styles/rules_rhythm.go); this file
// owns the page shell, the shared render model, and the sections without
// per-row hooks. Interactive rows live in their own components
// (recurring_rows.go / _agenda.go / _roster.go / _review.go / _findings.go).
package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/recurdiscover"
	"github.com/monstercameron/CashFlux/internal/runway"
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

// recurView is the derived render model the roster/agenda/hero share. Pure —
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
	// (the same order the engine disambiguates in), so the name shown on a row is
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
	// capture them BEFORE the display sort so a row shows the exact variable name
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

		if !r.Active() {
			continue // paused flows keep their definition but leave the schedule
		}
		// Walk the schedule from NextDue to the cutoff. Cap iterations so a NextDue
		// far in the past (many missed occurrences) can't generate an unbounded list.
		d := r.NextDue
		for i := 0; i < 8 && !d.After(cutoff); i++ {
			occ := recurOccurrence{R: r, Date: d, Overdue: d.Before(today)}
			// Settled-ness is ONE test, shared with the calendar's past-day states
			// (rhySettled) — when the strip and the calendar answered this
			// differently the page contradicted itself about the same bill. It also
			// closes a real hole here: the strip previously honoured only the
			// bill-match link, so marking an overdue item paid from this very strip
			// left it sitting in the strip.
			if r.Amount.IsNegative() {
				occ.Paid = rhySettled(app, r, d)
				// TX9: the variance chip needs the matched transaction specifically.
				if _, ok := app.BillMatchForOccurrence(r.ID, d); ok {
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

// recurTile wraps a tile body in the shared Widget chrome + the full-width bento
// column. Retained for the bills/subscriptions helper tiles that still compose
// with the recurring smart-schedule modal.
func recurTile(tid string, body ui.Node) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: tid, Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// recurSection wraps a tile body with a serif section title + optional action.
// Retained for the shared bills/subscriptions helper tiles.
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
// (only it + the host re-render on open/close, never the surface).
func addRecurringButton() ui.Node {
	edit := uistate.UseRecurringEditID()
	open := ui.UseEvent(Prevent(func() { edit.Set("new") }))
	return Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "recurring-add"), Title(uistate.T("recurring.addFlowTitle")), OnClick(open),
		uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("recurring.addFlow")))
}

// ─── The unified surface ─────────────────────────────────────────────────────

// rhythmView is everything the "month's rhythm" page renders, gathered once per
// render from the tested engine (computeRecurView + runway.Tideline +
// recurdiscover.Discover + findings), so the sections stay declarative.
type rhythmView struct {
	recurView
	Now         time.Time
	PayCycle    runway.PayCycle
	Tide        []cashflow.Event
	Overdue     []recurOccurrence
	Agenda      []recurOccurrence
	Discover    recurdiscover.Result
	DiscoverTxn []recurdiscover.Txn
	LateCharges []subscriptions.LateCharge
	Stopped     []recurdiscover.StopSignal
	LiquidMinor int64
	Rates       currency.Rates
}

// computeRhythm assembles the whole surface model from the store and the pure
// engine. Deterministic and hook-free.
func computeRhythm(app *appstate.App, now time.Time) rhythmView {
	rv := rhythmView{recurView: recurViewOf(app, now), Now: now}
	base := rv.Base
	rv.Rates = currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// Split the derived occurrences: overdue lives in its own honest strip, the
	// rest are the forward agenda.
	for _, occ := range rv.Upcoming {
		if occ.Overdue && !occ.Paid {
			rv.Overdue = append(rv.Overdue, occ)
		} else {
			rv.Agenda = append(rv.Agenda, occ)
		}
	}

	// Tideline: liquid cash + the ACTIVE recurring flows drive the pay-cycle band.
	active := make([]domain.Recurring, 0, len(rv.Flows))
	for _, r := range rv.Flows {
		if r.Active() {
			active = append(active, r)
		}
	}
	if liquid, err := ledger.LiquidBalance(app.Accounts(), app.Transactions(), rv.Rates); err == nil {
		rv.LiquidMinor = liquid.Amount
	}
	if pc, err := runway.Tideline(rv.LiquidMinor, active, now, rv.Rates); err == nil {
		rv.PayCycle = pc
		// WindowDays+1: the window is sized to the next income event, and Events
		// is half-open (day < days), so asking for exactly WindowDays drops the
		// very paycheck that anchors the cycle — the band then renders with no
		// income up-tick at all.
		rv.Tide, _ = runway.Events(active, now, pc.WindowDays+1, rv.Rates)
	}

	// Discovery: evidence-carrying candidates + the cluster matches that belong to
	// existing commitments (for the review strip + death detection).
	rv.DiscoverTxn = discoverTxns(app, rv.Rates)
	rv.Discover = rhyDiscover(app, rv.Rates, now)

	// Findings: charged-after-cancel + "seems stopped" (from the cycle matches'
	// last-seen, so the death check reuses the engine's own rhythm).
	rv.LateCharges, _ = subscriptions.ChargedAfterCancel(app.Transactions(), app.Cancellations(), rv.Rates)
	for _, cm := range rv.Discover.CycleMatches {
		if ss, ok := recurdiscover.DetectStopped(cm.CommitmentID, cm.Evidence.Cadence, cm.Evidence.LastSeen, now, 7); ok {
			rv.Stopped = append(rv.Stopped, ss)
		}
	}
	return rv
}

// rhythmFocus is the deep-link intent: /recurring lands neutrally, /bills favours
// the agenda, /subscriptions opens the roster's Subscriptions lens.
type rhythmFocus int

const (
	focusAll rhythmFocus = iota
	focusAgenda
	focusSubs
)

// rhythmSurfaceProps carries the deep-link focus into the page component.
type rhythmSurfaceProps struct {
	Focus rhythmFocus
}

// RhythmSurface is the unified Bills & Recurring page. One component owns the
// page-level handlers + hooks; the stateful sections (review, agenda, roster) and
// every interactive row are their own components so hooks stay stable.
func RhythmSurface(props rhythmSurfaceProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return Fragment()
	}
	_ = uistate.UseDataRevision().Get()

	nav := router.UseNavigate()
	edit := uistate.UseRecurringEditID()
	txFilter := uistate.UseTxFilter()
	showMetrics := ui.UseState(false)
	toggleMetrics := ui.UseEvent(Prevent(func() { showMetrics.Set(!showMetrics.Get()) }))
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

	rv := computeRhythm(app, time.Now())

	acts := rhyActions{
		OnEdit: func(rid string) { edit.Set(rid) },
		OnDelete: func(rid string) {
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
		},
		OnPauseToggle: func(r domain.Recurring) {
			r.Paused = !r.Paused
			if err := app.PutRecurring(r); err == nil {
				uistate.BumpDataRevision()
			}
		},
		OnCancelWatch: func(r domain.Recurring) {
			if err := app.MarkSubscriptionCancelled(r.Label, time.Now()); err == nil {
				uistate.BumpDataRevision()
			}
		},
		OnCopyVar: func(v string) {
			copyToClipboard(v, uistate.T("rhythm.copiedVar", v))
		},
		OnViewAccount: func(accountID string) { nav.Navigate(uistate.RoutePath("/accounts")) },
		OnViewTxns: func(r domain.Recurring) {
			f := uistate.TxFilter{Account: r.AccountID, Category: r.CategoryID}
			if r.AccountID == "" && r.CategoryID == "" {
				f.Text = r.Label
			}
			nf := f.Normalize()
			txFilter.Set(nf)
			uistate.PersistTxFilter(nf)
			nav.Navigate(uistate.RoutePath("/transactions"))
		},
		OnViewBudget: func(domain.Recurring) { nav.Navigate(uistate.RoutePath("/budgets")) },
		OnMarkPaid: func(occ recurOccurrence) {
			b := occ.R
			billID := "recurring:" + b.ID
			if err := app.RecordBillPayment(billID, b.Label, b.Amount); err != nil {
				postMsg.Set(err.Error())
				return
			}
			if err := app.MarkOccurrencePaid(billID, occ.Date); err != nil {
				app.Log().Error("mark occurrence paid", "billID", billID, "err", err)
			}
			uistate.BumpDataRevision()
		},
	}

	addIncome := ui.UseEvent(Prevent(func() { edit.Set("new") }))
	csv := ui.UseEvent(rhyCsvExport(rv))

	return Div(css.Class("rhy"),
		// XC9: payday pre-flight ritual card keeps a quiet slot at the top.
		ui.CreateElement(paydayPreflightCard),
		rhyHeroSection(rv, addIncome),
		rhyOverdueSection(rv, acts),
		ui.CreateElement(rhyReviewSection, rhyReviewProps{}),
		ui.CreateElement(rhyAgendaSection, rhyAgendaProps{Focus: props.Focus, Acts: acts}),
		ui.CreateElement(rhyRosterSection, rhyRosterProps{Focus: props.Focus, Acts: acts}),
		rhyFindingsSection(rv, acts),
		rhyToolbar(rv, postMsg.Get(), showMetrics.Get(), postDue, toggleMetrics, csv),
		If(showMetrics.Get(), Div(css.Class("rhy-section"),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("recurring.formulaHint")),
			ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("recurring.metricsTitle"), ShowSaved: true}),
		)),
	)
}

// rhyActions bundles the page-level callbacks the sections and rows invoke.
type rhyActions struct {
	OnEdit        func(string)
	OnDelete      func(string)
	OnPauseToggle func(domain.Recurring)
	OnCancelWatch func(domain.Recurring)
	OnCopyVar     func(string)
	OnViewAccount func(string)
	OnViewTxns    func(domain.Recurring)
	OnViewBudget  func(domain.Recurring)
	OnMarkPaid    func(recurOccurrence)
}

// rhySection is the from-scratch section chrome: a titled card that fills the
// content column. flush drops the card frame for the hero (which draws its own).
func rhySection(id, title, note string, action, body ui.Node) ui.Node {
	head := Fragment()
	if title != "" || action != nil {
		head = Div(css.Class("rhy-sec-head"),
			If(title != "", H2(css.Class("rhy-sec-title"), title)),
			If(action != nil, action),
		)
	}
	args := []any{css.Class("rhy-section")}
	if id != "" {
		args = append(args, Attr("id", id))
	}
	args = append(args, head)
	if note != "" {
		args = append(args, P(css.Class("rhy-sec-note"), note))
	}
	args = append(args, body)
	return Div(args...)
}

// rhyHeroSection is the tideline hero: the SVG pay-cycle band + pinch caption
// beside the compact net/in/out stat rail. onAddIncome is a stable page-level
// handler (the no-income degradation prompts to add a paycheck).
func rhyHeroSection(rv rhythmView, onAddIncome any) ui.Node {
	netTone := " " + tw.ColorClass("text-up")
	if rv.MonthlyNet < 0 {
		netTone = " " + tw.ColorClass("text-down")
	}
	stats := Div(css.Class("rhy-stats"),
		rhyStat(uistate.T("rhythm.statNet"), fmtMoney(money.New(rv.MonthlyNet, rv.Base)), netTone, "recurring-net"),
		rhyStat(uistate.T("rhythm.statIn"), fmtMoney(money.New(rv.MonthlyIn, rv.Base)), " "+tw.ColorClass("text-up"), ""),
		rhyStat(uistate.T("rhythm.statOut"), fmtMoney(money.New(rv.MonthlyOut, rv.Base)), " "+tw.ColorClass("text-down"), ""),
	)
	// The household's own "keep at least this much" floor (from the smart pay
	// schedule) is the honest threshold for calling a cycle tight.
	keepFloor := uistate.BillsSmartConfigGet().MinKeepMinor
	band := Div(css.Class("rhy-tide"),
		rhyTideline(rv.PayCycle, rv.Tide, rv.Base, rv.Dec, rv.Now),
		rhyPinchNote(rv.PayCycle, rv.Base, keepFloor),
		If(!rv.PayCycle.HasIncome,
			Button(css.Class("btn btn-sm", tw.Mt2), Type("button"), Attr("data-testid", "rhy-add-income"),
				OnClick(onAddIncome), uistate.T("rhythm.tideAddIncome"))),
	)
	return rhySection("sec-overview", uistate.T("rhythm.heroTitle"), uistate.T("rhythm.heroNote"), nil,
		Div(css.Class("rhy-hero"), band, stats))
}

// rhyStat renders one compact stat in the hero rail.
func rhyStat(label, value, tone, testid string) ui.Node {
	valArgs := []any{ClassStr("rhy-stat-value " + tw.Fold(tw.FontDisplay) + tone)}
	if testid != "" {
		valArgs = append(valArgs, Attr("data-testid", testid))
	}
	valArgs = append(valArgs, value)
	return Div(css.Class("rhy-stat"),
		Div(css.Class("rhy-stat-label"), label),
		Div(valArgs...),
	)
}

// rhyOverdueSection is the honest overdue strip — rendered ONLY when something is
// overdue, never folded into the forward agenda.
func rhyOverdueSection(rv rhythmView, acts rhyActions) ui.Node {
	if len(rv.Overdue) == 0 {
		return Fragment()
	}
	var total int64
	for _, occ := range rv.Overdue {
		if occ.R.Amount.IsNegative() {
			total += -occ.R.Amount.Amount
		}
	}
	rows := []any{}
	for _, occ := range rv.Overdue {
		o := occ
		rows = append(rows, ui.CreateElement(rhyOverdueRow, rhyOverdueRowProps{Occ: o, Base: rv.Base, OnMarkPaid: acts.OnMarkPaid}))
	}
	head := Div(css.Class("rhy-overdue-head"),
		uiw.Icon(icon.AlertTriangle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("rhythm.overdueSummary", plural(len(rv.Overdue), "item"), fmtMoney(money.New(total, rv.Base)))),
	)
	body := append([]any{css.Class("rhy-section rhy-overdue"), Attr("data-testid", "rhy-overdue"), head}, rows...)
	return Div(body...)
}

// rhyFindingsSection surfaces charged-after-cancel, price creep (via the existing
// accept flow), and "seems stopped" — each with a one-click verb.
func rhyFindingsSection(rv rhythmView, acts rhyActions) ui.Node {
	var rows []any
	for _, lc := range rv.LateCharges {
		c := lc
		rows = append(rows, ui.CreateElement(rhyFindingRow, rhyFindingRowProps{
			Kind: findingCharged, Name: c.SubName,
			Text: uistate.T("rhythm.findCharged", c.SubName, fmtMoney(money.New(c.Amount, rv.Base))),
			Late: c,
		}))
	}
	for _, ss := range rv.Stopped {
		name := ss.CommitmentID
		for _, r := range rv.Flows {
			if r.ID == ss.CommitmentID {
				name = r.Label
				break
			}
		}
		s := ss
		rows = append(rows, ui.CreateElement(rhyFindingRow, rhyFindingRowProps{
			Kind: findingStopped, Name: name, CommitmentID: s.CommitmentID,
			Text:          uistate.T("rhythm.findStopped", name, s.MissedCount),
			OnPauseToggle: acts.OnPauseToggle, Flows: rv.Flows,
		}))
	}
	// Price creep keeps its own in-place accept flow (XC5), rendered inline.
	creep := ui.CreateElement(priceCreepNotices)
	if len(rows) == 0 {
		return rhySection("sec-findings", "", "", nil, creep)
	}
	body := append([]any{}, rows...)
	body = append(body, creep)
	return rhySection("sec-findings", uistate.T("rhythm.findingsTitle"), "", nil, Div(body...))
}

// rhyToolbar is the quiet utilities row: Add recurring (primary), Post due, the
// schedule-metrics toggle, Detection preferences, Smart pay schedule, and a
// demoted CSV download — no permanent nag banner.
func rhyToolbar(rv rhythmView, postMsg string, showMetrics bool, onPostDue, onToggleMetrics, onCsv any) ui.Node {
	dueNow := 0
	for _, r := range rv.Flows {
		if r.Active() && r.Autopost && !r.NextDue.After(time.Now()) {
			dueNow++
		}
	}
	metricsCls := "strip-toggle"
	metricsLabel := uistate.T("rhythm.toolsMetrics")
	if showMetrics {
		metricsCls += " is-on"
		metricsLabel = uistate.T("rhythm.toolsMetricsHide")
	}
	tools := Div(css.Class("rhy-tools"),
		ui.CreateElement(addRecurringButton),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "recurring-post-due"),
			Title(uistate.T("recurring.postDueTitle")), OnClick(onPostDue),
			fmt.Sprintf("%s (%d)", uistate.T("recurring.postDue"), dueNow)),
		ui.CreateElement(rhyDetectPrefsButton),
		ui.CreateElement(rhySmartPayButton),
		Button(ClassStr(metricsCls+" btn-sm"), Type("button"), Attr("aria-pressed", ariaBool(showMetrics)),
			Attr("data-testid", "recurring-toggle-formulas"), Title(uistate.T("recurring.metricsTitle")),
			OnClick(onToggleMetrics), Text(metricsLabel)),
		Span(css.Class("rhy-tools-spacer")),
		If(postMsg != "", Span(css.Class("muted"), Attr("data-testid", "recurring-post-msg"), Attr("role", "status"), postMsg)),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "rhy-download-csv"),
			Title(uistate.T("bills.downloadCsvTitle")), OnClick(onCsv), uistate.T("rhythm.toolsCsv")),
	)
	return Div(css.Class("rhy-section"), tools)
}

// rhyDetectPrefsButton opens the subscription-detection preferences modal.
func rhyDetectPrefsButton() ui.Node {
	open := uistate.UseSubsPrefsOpen()
	click := ui.UseEvent(Prevent(func() { open.Set(true) }))
	return Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "subs-detect-prefs-toggle"),
		Title(uistate.T("rhythm.toolsDetection")), OnClick(click), uistate.T("rhythm.toolsDetection"))
}

// rhySmartPayButton opens the smart pay-schedule modal.
func rhySmartPayButton() ui.Node {
	open := uistate.UseBillsSmartOpen()
	click := ui.UseEvent(Prevent(func() { open.Set(true) }))
	return Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "bills-smart-open"),
		Title(uistate.T("bills.smartEnableTitle")), OnClick(click), uistate.T("bills.smartTitle"))
}

// rhyCsvExport builds the agenda-download callback (demoted to a quiet toolbar
// action, not a permanent nag banner).
func rhyCsvExport(rv rhythmView) func() {
	return func() {
		var b strings.Builder
		b.WriteString("date,name,amount\n")
		for _, occ := range rv.Agenda {
			fmt.Fprintf(&b, "%s,%q,%s\n", occ.Date.Format("2006-01-02"), occ.R.Label,
				money.FormatMinor(occ.R.Amount.Amount, rv.Dec))
		}
		downloadBytes("recurring.csv", "text/csv", []byte(b.String()))
	}
}

// postingMode classifies how a commitment gets paid, for the agenda/roster badge.
func postingMode(r domain.Recurring) (label, hint, cls string) {
	switch {
	case r.Autopost:
		return uistate.T("rhythm.modeAuto"), uistate.T("rhythm.modeAutoHint"), "is-auto"
	case r.Autopay:
		return uistate.T("rhythm.modeWatch"), uistate.T("rhythm.modeWatchHint"), "is-watch"
	default:
		return uistate.T("rhythm.modeManual"), uistate.T("rhythm.modeManualHint"), ""
	}
}

// rhyModeBadge renders the posting-mode badge (display-only).
func rhyModeBadge(r domain.Recurring) ui.Node {
	label, hint, cls := postingMode(r)
	return Span(ClassStr("rhy-badge "+cls), Title(hint), label)
}

// Recurring is the /recurring route — the unified Bills & Recurring surface. The
// /bills and /subscriptions thin shells (Bills()/Subscriptions(), in
// bills_screen.go / subscriptions_screen.go) render the same surface with their
// deep-link focus.
func Recurring() ui.Node {
	return ui.CreateElement(RhythmSurface, rhythmSurfaceProps{Focus: focusAll})
}

// rhythmSurfaceFocused renders the unified surface for a deep-link route.
func rhythmSurfaceFocused(f rhythmFocus) ui.Node {
	return ui.CreateElement(RhythmSurface, rhythmSurfaceProps{Focus: f})
}
