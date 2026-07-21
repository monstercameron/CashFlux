// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/moneyleaks"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/resilience"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// resLookbackMonths is the trailing window used to average monthly income and
// spending for the stress tests — the same three-month basis the health score
// uses, so the two never disagree about "typical" cash flow.
const resLookbackMonths = 3

// liveResilienceInput builds the monthly cash-flow snapshot the stress tests run
// against, from the same sources the health score reads: trailing income/spend,
// liquid cash from the engine surface, and the revolving balance + balance-weighted
// APR from the credit cards.
func liveResilienceInput(app *appstate.App, now time.Time) resilience.Input {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	txns := app.Transactions()

	curMonth := dateutil.MonthStart(now)
	start := dateutil.AddMonths(curMonth, -resLookbackMonths)
	var monthlyIncome, monthlySpend int64
	if flow, err := reports.IncomeVsExpense(txns, start, curMonth, rates); err == nil {
		monthlyIncome = flow.Income / resLookbackMonths
		monthlySpend = flow.Expense / resLookbackMonths // all outflow, debt payments included
	}

	liquid := majorMoney(liveEngineVars(app)["liquid_cash"], base).Amount

	// Revolving balance + balance-weighted APR across the cards.
	var revBal int64
	var wAprNum, wAprDen float64
	for _, a := range app.Accounts() {
		if a.Archived || a.Type != domain.TypeCreditCard {
			continue
		}
		bal, err := ledger.Balance(a, txns)
		if err != nil {
			continue
		}
		owed := bal.Amount
		if owed < 0 {
			owed = -owed
		}
		conv, cerr := currency.ConvertBetween(owed, a.Currency, base, rates)
		if cerr != nil {
			continue
		}
		revBal += conv
		wAprNum += float64(conv) * a.InterestRateAPR
		wAprDen += float64(conv)
	}
	avgAPR := 0.0
	if wAprDen > 0 {
		avgAPR = wAprNum / wAprDen
	}

	return resilience.Input{
		LiquidCash:    liquid,
		MonthlyIncome: monthlyIncome,
		MonthlySpend:  monthlySpend,
		// Debt minimums are already inside MonthlySpend (real outflow), so they aren't
		// added again; the field stays zero to avoid double-counting.
		RevolvingBalance: revBal,
		AvgCardAPR:       avgAPR,
	}
}

// --- Hero: "why this score" contribution breakdown ------------------------------

// contribTone maps a factor score to the contribution-bar segment class.
func contribTone(score int) string {
	switch {
	case score >= 60:
		return "is-good"
	case score >= 40:
		return "is-warn"
	default:
		return "is-bad"
	}
}

// healthContribBar answers "why this number?" at a glance: a single 0–100 bar
// segmented by each applicable factor's actual points contributed (its score ×
// its weight), coloured by how strong that factor is, with the empty tail showing
// the points still on the table. It fills the hero's spare space so the score is
// legible without opening the six disclosures.
func healthContribBar(r healthscore.Result) ui.Node {
	if r.Band == healthscore.BandNoData {
		return Fragment()
	}
	type seg struct {
		label string
		pts   int
		tone  string
	}
	var segs []seg
	for _, f := range r.Factors {
		if !f.Applicable {
			continue
		}
		pts := int(float64(f.Score)*f.Weight + 0.5)
		if pts <= 0 {
			continue
		}
		segs = append(segs, seg{f.Label, pts, contribTone(f.Score)})
	}
	if len(segs) == 0 {
		return Fragment()
	}
	bars := make([]ui.Node, 0, len(segs))
	legend := make([]ui.Node, 0, len(segs))
	for _, s := range segs {
		bars = append(bars, Div(ClassStr("hlt-contrib-seg "+s.tone),
			Attr("title", fmt.Sprintf("%s +%d", s.label, s.pts)),
			Style(map[string]string{"width": fmt.Sprintf("%d%%", s.pts)})))
		legend = append(legend, Span(css.Class("hlt-contrib-key"),
			Span(ClassStr("hlt-contrib-dot "+s.tone), Attr("aria-hidden", "true")),
			Span(css.Class("hlt-contrib-name"), s.label),
			Span(css.Class("hlt-contrib-pts", tw.TextDim), fmt.Sprintf("+%d", s.pts))))
	}
	return Div(css.Class("hlt-contrib"),
		Span(css.Class("hlt-detail-label"), uistate.T("healthx.whyScore")),
		Div(withNodes([]any{css.Class("hlt-contrib-bar"), Attr("role", "img"), Attr("aria-label", uistate.T("healthx.whyScoreAria", r.Score))}, bars)...),
		Div(withNodes([]any{css.Class("hlt-contrib-legend")}, legend)...),
	)
}

// --- Stress-test tile -----------------------------------------------------------

type healthStressProps struct {
	In   resilience.Input
	Base string
}

// healthChipProps is one selectable value chip in the stress controls; its own
// component so the per-chip OnClick hook sits at a stable position.
type healthChipProps struct {
	Label  string
	Aria   string // full accessible name (the bare "10%" doesn't say which shock)
	Active bool
	Key    string
	OnPick func()
}

func healthStressChip(p healthChipProps) ui.Node {
	on := ui.UseEvent(Prevent(func() { p.OnPick() }))
	cls := "chip-btn"
	if p.Active {
		cls += " is-active"
	}
	args := []any{ClassStr(cls), Type("button"), Attr("aria-pressed", ariaBool(p.Active)),
		Attr("data-testid", "stress-chip-"+p.Key), OnClick(on)}
	if p.Aria != "" {
		args = append(args, Attr("aria-label", p.Aria))
	}
	args = append(args, p.Label)
	return Button(args...)
}

// healthStressTile is the interactive what-if surface: pick a shock (a pay cut, a
// surprise bill, a rate hike) and read the concrete outcome, all recomputed live
// from the resilience engine. The runway (no income at all) is the fixed headline.
func healthStressTile(props healthStressProps) ui.Node {
	incomeDrop := ui.UseState(20)
	ratePts := ui.UseState(5)
	surpriseSel := ui.UseState(1) // index into the surprise presets

	in := props.In
	base := props.Base
	dec := currency.Decimals(base)
	unit := int64(1)
	for i := 0; i < dec; i++ {
		unit *= 10
	}
	fmtB := func(minor int64) string { return fmtMoney(money.New(minor, base)) }

	// Runway headline.
	runway := resilience.RunwayMonths(in)
	runwayLine := P(css.Class("hlt-stress-lead"), Attr("data-testid", "stress-runway"),
		uistate.T("healthx.runwayLead", fmtB(in.LiquidCash), fmtMonthsHuman(int(runway+0.0001))))

	// Income cut.
	dropPcts := []int{10, 20, 30, 50}
	dropChips := make([]ui.Node, 0, len(dropPcts))
	for _, p := range dropPcts {
		v := p
		dropChips = append(dropChips, ui.CreateElement(healthStressChip, healthChipProps{
			Label: fmt.Sprintf("%d%%", v), Aria: uistate.T("healthx.dropAria", v),
			Active: incomeDrop.Get() == v, Key: fmt.Sprintf("drop-%d", v),
			OnPick: func() { incomeDrop.Set(v) },
		}))
	}
	drop := resilience.IncomeDrop(in, incomeDrop.Get())
	var dropOut string
	if drop.GoesNegative {
		dropOut = uistate.T("healthx.dropNegative", drop.DropPct, fmtMonthsHuman(drop.MonthsToNegative))
	} else {
		dropOut = uistate.T("healthx.dropOk", drop.DropPct, fmtB(drop.NewSurplus))
	}

	// Surprise expense. A final "over the buffer" preset (a hair more than the liquid
	// cash) guarantees the scary branch is reachable — otherwise a large buffer makes
	// every fixed preset land on the same reassuring outcome (review finding #6).
	surprisePresets := []int64{500 * unit, 1000 * unit, 2500 * unit, 5000 * unit}
	if over := in.LiquidCash + in.MonthlySpend; over > 5000*unit {
		surprisePresets = append(surprisePresets, over)
	}
	surChips := make([]ui.Node, 0, len(surprisePresets))
	for i, amt := range surprisePresets {
		idx := i
		a := amt
		surChips = append(surChips, ui.CreateElement(healthStressChip, healthChipProps{
			Label: fmtB(a), Aria: uistate.T("healthx.surpriseAria", fmtB(a)),
			Active: surpriseSel.Get() == idx, Key: fmt.Sprintf("sur-%d", idx),
			OnPick: func() { surpriseSel.Set(idx) },
		}))
	}
	sIdx := surpriseSel.Get()
	if sIdx < 0 || sIdx >= len(surprisePresets) {
		sIdx = 0
	}
	sur := resilience.SurpriseExpense(in, surprisePresets[sIdx])
	var surOut string
	if sur.PushedToDebt > 0 {
		surOut = uistate.T("healthx.surpriseDebt", fmtB(sur.Amount), fmtB(sur.PushedToDebt), fmtB(sur.ExtraMonthlyInterest))
	} else {
		surOut = uistate.T("healthx.surpriseOk", fmtB(sur.Amount), fmtB(sur.BufferAfter))
	}

	// Rate hike.
	ratePresets := []int{3, 5, 10}
	rateChips := make([]ui.Node, 0, len(ratePresets))
	for _, p := range ratePresets {
		v := p
		rateChips = append(rateChips, ui.CreateElement(healthStressChip, healthChipProps{
			Label: fmt.Sprintf("+%d", v), Aria: uistate.T("healthx.rateAria", v),
			Active: ratePts.Get() == v, Key: fmt.Sprintf("rate-%d", v),
			OnPick: func() { ratePts.Set(v) },
		}))
	}
	hike := resilience.RateHike(in, float64(ratePts.Get()))
	var rateOut string
	if in.RevolvingBalance > 0 {
		rateOut = uistate.T("healthx.rateOut", ratePts.Get(), fmtB(hike.ExtraMonthlyInterest), fmtB(hike.ExtraAnnualInterest))
	} else {
		rateOut = uistate.T("healthx.rateNoCards")
	}

	block := func(label string, chips []ui.Node, out, testid string) ui.Node {
		return Div(css.Class("hlt-stress-row"),
			Div(css.Class("hlt-stress-ctrl"),
				Span(css.Class("hlt-stress-label", tw.TextDim), label),
				Div(withNodes([]any{css.Class("hlt-stress-chips")}, chips)...),
			),
			P(css.Class("hlt-stress-out"), Attr("data-testid", testid), out),
		)
	}

	body := Fragment(
		P(css.Class("muted"), uistate.T("healthx.stressHint")),
		runwayLine,
		block(uistate.T("healthx.dropLabel"), dropChips, dropOut, "stress-drop"),
		block(uistate.T("healthx.surpriseLabel"), surChips, surOut, "stress-surprise"),
		block(uistate.T("healthx.rateLabel"), rateChips, rateOut, "stress-rate"),
	)
	return hltTile("hlt-stress", "1 / span 4",
		hltSection("sec-health-stress", uistate.T("healthx.stressTitle"), nil, body))
}

// --- Leaks / recurring-load tile ------------------------------------------------

type healthLeaksProps struct{ App *appstate.App }

// healthLeaksTile analyses the money that quietly leaves every month: the total
// recurring commitment load (with the biggest few named) and the categories whose
// recent spend has crept above their own norm (reusing reports.TrimTargets). Both
// read off the user's own data; nothing here is a hardcoded budget.
func healthLeaksTile(props healthLeaksProps) ui.Node {
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	// Drill a creep category to its contributing transactions (review finding #4).
	openCat := func(catID string) {
		f := uistate.TxFilter{Category: catID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	now := time.Now()
	fmtB := func(minor int64) string { return fmtMoney(money.New(minor, base)) }

	// Recurring load: monthly-equivalent of every recurring OUTFLOW, base-converted.
	monthlyIncome := majorMoney(liveEngineVars(app)["income"], base).Amount
	var subs []moneyleaks.Sub
	for _, r := range app.Recurring() {
		me := r.MonthlyEquivalent()
		if me >= 0 {
			continue // income / inflow — not a commitment
		}
		conv, err := currency.ConvertBetween(-me, r.Amount.Currency, base, rates)
		if err != nil || conv <= 0 {
			continue
		}
		subs = append(subs, moneyleaks.Sub{Label: r.Label, MonthlyMinor: conv, Autopay: r.Autopay})
	}
	rep := moneyleaks.Subscriptions(subs, monthlyIncome, 3)

	var loadNode ui.Node = P(css.Class("empty"), uistate.T("healthx.noRecurring"))
	if rep.Count > 0 {
		shareNote := Fragment()
		if rep.SharePct > 0 {
			shareNote = Span(css.Class("t-caption", tw.TextFaint),
				uistate.T("healthx.recurringShare", fmt.Sprintf("%.0f", rep.SharePct)))
		}
		topParts := make([]string, 0, len(rep.Top))
		for _, s := range rep.Top {
			topParts = append(topParts, s.Label+" "+fmtB(s.MonthlyMinor))
		}
		loadNode = Fragment(
			Div(css.Class("hlt-leak-head"),
				Span(ClassStr("hlt-leak-figure "+tw.Fold(tw.FontDisplay)), uistate.T("healthx.recurringPerMo", fmtB(rep.TotalMonthly))),
				Span(css.Class("t-caption", tw.TextDim), uistate.T("healthx.recurringCount", rep.Count, fmtB(rep.TotalAnnual))),
			),
			shareNote,
			If(len(topParts) > 0, P(css.Class("t-caption", tw.TextDim, tw.Mt1),
				uistate.T("healthx.recurringBiggest")+" "+joinMid(topParts))),
		)
	}

	// Spending creep: categories whose recent 3-month average runs above their own
	// median — reuses reports.TrimTargets over a trailing 12-month series.
	bounds := make([]time.Time, 0, 13)
	as := dateutil.AddMonths(dateutil.MonthStart(now), -12)
	for k := 0; k <= 12; k++ {
		bounds = append(bounds, dateutil.AddMonths(as, k))
	}
	trends, _ := reports.CategoryTrends(app.Transactions(), bounds, rates)
	// Ignore trivial creep — a $20/mo floor keeps the list to categories worth acting on.
	minMonthly := 20 * powInt64(10, currency.Decimals(base))
	targets := reports.TrimTargets(trends, minMonthly, 3)
	var creepNode ui.Node = P(css.Class("t-caption", tw.TextFaint), uistate.T("healthx.noCreep"))
	if len(targets) > 0 {
		rows := make([]ui.Node, 0, len(targets))
		for _, t := range targets {
			cid := t.CategoryID
			rows = append(rows, ui.CreateElement(healthCreepRow, healthCreepRowProps{
				Name:   budgetCategoryName(app, cid),
				Detail: uistate.T("healthx.creepDetail", fmtB(t.RecentAvgMinor), fmtB(t.MedianMinor)),
				Save:   uistate.T("healthx.creepSave", fmtB(t.MonthlySaveMinor)),
				OnOpen: func() { openCat(cid) },
			}))
		}
		creepNode = Fragment(withNodes(nil, rows)...)
	}

	body := Fragment(
		Div(css.Class("hlt-leak-block"),
			Span(css.Class("hlt-detail-label"), uistate.T("healthx.recurringSubtitle")),
			loadNode,
		),
		Div(css.Class("hlt-leak-block", tw.Mt3),
			Span(css.Class("hlt-detail-label"), uistate.T("healthx.creepSubtitle")),
			P(css.Class("muted"), uistate.T("healthx.creepHint")),
			creepNode,
		),
	)
	return hltTile("hlt-leaks", "1 / span 4",
		hltSection("sec-health-leaks", uistate.T("healthx.leaksTitle"),
			debtOwnerLink("/recurring", uistate.T("healthx.manageRecurring")), body))
}

// healthCreepRowProps drives one spending-creep row; its own component so the
// per-row drill hook sits at a stable position (rows render in a loop).
type healthCreepRowProps struct {
	Name, Detail, Save string
	OnOpen             func()
}

// healthCreepRow renders a creep finding as a button that drills to the category's
// contributing transactions — turning the insight into an action (review finding #4).
func healthCreepRow(p healthCreepRowProps) ui.Node {
	open := ui.UseEvent(Prevent(func() { p.OnOpen() }))
	return Button(css.Class("hlt-creep-row", tw.WFull, tw.TextLeft, tw.HoverBgHover), Type("button"),
		Attr("data-testid", "health-creep-row"),
		Attr("aria-label", uistate.T("healthx.creepAria", p.Name)), OnClick(open),
		Span(css.Class("hlt-creep-name", tw.Fold(tw.FontMedium)), p.Name),
		Span(css.Class("t-caption", tw.TextDim), p.Detail),
		Span(ClassStr("hlt-creep-save "+tw.ColorClass("text-down")), p.Save),
		uiw.Icon(icon.ChevronRight, css.Class("hlt-creep-chev", tw.ShrinkO, tw.W4, tw.H4, tw.TextFaint)),
	)
}

// withNodes appends a []ui.Node onto a head []any so it can be spread into an
// element constructor (which takes ...any, not ...ui.Node).
func withNodes(head []any, nodes []ui.Node) []any {
	for _, n := range nodes {
		head = append(head, n)
	}
	return head
}

// joinMid joins parts with a middot separator.
func joinMid(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " · "
		}
		out += p
	}
	return out
}

// powInt64 returns base**exp for small non-negative exp (currency decimal scaling).
func powInt64(base int64, exp int) int64 {
	out := int64(1)
	for i := 0; i < exp; i++ {
		out *= base
	}
	return out
}
