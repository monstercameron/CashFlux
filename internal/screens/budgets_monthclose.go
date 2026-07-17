// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgets_monthclose.go is the guided month-close flow (#64): one flip modal that
// walks the period's loose ends — overspends to cover, unused money and what
// rollover will do with it, over-assignment with its honest resolution choices,
// actual vs expected income, and a one-click "copy last month's top-ups with
// exceptions". It composes the EXISTING primitives (cover-all, sweep, boosts,
// the income-basis modal) around the pure monthclose.Summary — no new money
// mechanics, just one place that walks them in order.

// buildMonthCloseSummary assembles the pure summary for the current budget view.
func buildMonthCloseSummary(app *appstate.App, v budgetView, bs, be time.Time) monthclose.Summary {
	rates := currency.Rates{Base: v.Base, Rates: app.Settings().FXRates}
	actual := int64(0)
	if lines, err := reports.IncomeByCategory(app.Transactions(), bs, be, rates); err == nil {
		for _, ln := range lines {
			actual += ln.Amount
		}
	}
	overAssigned := int64(0)
	switch v.Method {
	case budgeting.MethodZeroBased:
		if ta := budgeting.ToAssign(v.BannerIncome+v.RolledOver, v.TotalLimit+v.SavingsAssigned); ta < 0 {
			overAssigned = -ta
		}
	case budgeting.MethodSimple:
		if d := v.BannerIncome - v.TotalLimit; d < 0 {
			overAssigned = -d
		}
	}
	nameOf := func(b domain.Budget) string { return budgetTitle(b.Name, v.CatName[b.CategoryID]) }
	rolloverOn := uistate.CurrentPrefs().BudgetRolloverLeftover
	return monthclose.Build(v.Statuses, nameOf, v.BannerIncome, actual, overAssigned, rolloverOn)
}

// budgetsMonthCloseModal renders the month-close flip modal when its atom is set,
// as a surface-root sibling of the bento (no tile transform clips it).
func budgetsMonthCloseModal() ui.Node {
	openAtom := uistate.UseMonthCloseOpen()
	if !openAtom.Get() {
		return Fragment()
	}
	vw := uistate.UsePeriod().Get()
	return uiw.FlipPanel(uiw.FlipPanelProps{
		Title:     uistate.T("monthclose.title", vw.Label()),
		Width:     uiw.FlipLargeW,
		Height:    uiw.FlipLargeH,
		NoFooter:  true,
		FlushBody: true,
		OnClose:   func() { openAtom.Set(false) },
		Back:      ui.CreateElement(monthCloseBody, monthCloseBodyProps{}),
	})
}

type monthCloseBodyProps struct{}

// monthCloseBody is the modal's content: the five ordered sections. Its own
// component so every hook (state + click handlers) sits at a stable position.
func monthCloseBody(_ monthCloseBodyProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	openAtom := uistate.UseMonthCloseOpen()
	nav := router.UseNavigate()

	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr, false)
	bs, be := vw.Range()
	s := buildMonthCloseSummary(app, v, bs, be)

	// Over-assignment: "leave unresolved" is an acknowledged choice, not a dismissal
	// of the modal — the section collapses to a quiet note for this open/close cycle.
	deferred := ui.UseState(false)
	// Copy-with-exceptions: excluded budget IDs, comma-joined (state must be a value).
	excluded := ui.UseState("")
	toggleExclude := func(id string) {
		set := monthCloseExcludeSet(excluded.Get())
		if set[id] {
			delete(set, id)
		} else {
			set[id] = true
		}
		var ids []string
		for k := range set {
			ids = append(ids, k)
		}
		excluded.Set(strings.Join(ids, ","))
	}

	coverAllAtom := uistate.UseCoverAllOpen()
	sweepAtom := uistate.UseSweepConfigOpen()
	basisAtom := uistate.UseBudgetBasisOpen()
	basisDraft := uistate.UseBudgetBasisDraft()

	openCoverAll := ui.UseEvent(Prevent(func() { openAtom.Set(false); coverAllAtom.Set(true) }))
	openSweep := ui.UseEvent(Prevent(func() { openAtom.Set(false); sweepAtom.Set(true) }))
	openBasis := ui.UseEvent(Prevent(func() {
		basisDraft.Set(uistate.NewBudgetBasisDraft(uistate.CurrentPrefs()))
		openAtom.Set(false)
		basisAtom.Set(true)
	}))
	goAllocate := ui.UseEvent(Prevent(func() {
		openAtom.Set(false)
		nav.Navigate(uistate.RoutePath("/allocate"))
	}))
	enableRollover := ui.UseEvent(Prevent(func() {
		p := uistate.CurrentPrefs()
		p.BudgetRolloverLeftover = true
		uistate.SetPrefs(p)
		uistate.RequestPersist()
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("monthclose.rolloverEnabled"), false)
	}))
	deferResolve := ui.UseEvent(Prevent(func() { deferred.Set(true) }))
	closeDone := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))

	// The copy plan (per-budget period keys — budgets can run weekly/quarterly).
	ws := pr.WeekStartWeekday()
	anchor := v.Anchor
	if anchor.IsZero() {
		anchor = time.Now()
	}
	periodStarts := func(b domain.Budget) (time.Time, time.Time) {
		thisStart, _ := budgeting.PeriodRange(b.Period, anchor, ws)
		lastStart, _ := budgeting.PeriodRange(b.Period, thisStart.AddDate(0, 0, -1), ws)
		return lastStart, thisStart
	}
	var visibleBudgets []domain.Budget
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			visibleBudgets = append(visibleBudgets, b)
		}
	}
	copyAll := monthclose.CopyBoosts(visibleBudgets, periodStarts, nil)
	copyPlan := monthclose.CopyBoosts(visibleBudgets, periodStarts, monthCloseExcludeSet(excluded.Get()))
	applyCopy := ui.UseEvent(Prevent(func() {
		plan := monthclose.CopyBoosts(visibleBudgets, periodStarts, monthCloseExcludeSet(excluded.Get()))
		n := 0
		for _, b := range visibleBudgets {
			amt, ok := plan[b.ID]
			if !ok {
				continue
			}
			_, thisStart := periodStarts(b)
			if err := app.PutBudget(b.WithPeriodBoost(thisStart, amt)); err == nil {
				n++
			}
		}
		if n > 0 {
			uistate.BumpDataRevision()
			uistate.RequestPersist()
			uistate.PostUndoable(uistate.T("monthclose.copyApplied", plural(n, "budget")))
		}
	}))

	// Inline-styled section chrome (a handful of elements — not worth a styles-
	// registry entry that would entangle a shared file under concurrent lanes).
	section := func(testid, title string, kids ...any) ui.Node {
		args := []any{css.Class("mc-section"), Attr("data-testid", testid),
			Style(map[string]string{"padding": "0.85rem 1.1rem", "border-bottom": "1px solid color-mix(in srgb, var(--border) 60%, transparent)"}),
			H3(css.Class("mc-section-title"),
				Style(map[string]string{"margin": "0 0 0.4rem", "font-size": "0.95rem", "font-weight": "600", "color": "var(--text)"}),
				title)}
		return Div(append(args, kids...)...)
	}

	// 1 — Overspending.
	var overBody []any
	if len(s.Overspends) == 0 {
		overBody = append(overBody, P(css.Class("budget-sub"), uistate.T("monthclose.overNone")))
	} else {
		overBody = append(overBody, P(css.Class("budget-sub"),
			uistate.T("monthclose.overIntro", len(s.Overspends), fmtMoney(money.New(s.TotalOverMinor, v.Base)))))
		var rows []ui.Node
		for _, it := range s.Overspends {
			rows = append(rows, Li(css.Class("wf-line"),
				Span(css.Class("wf-line-name"), it.Name),
				Span(css.Class("wf-line-amt"), fmtMoney(money.New(it.Minor, v.Base)))))
		}
		overBody = append(overBody, Ul(css.Class("wf-lines"), rows),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "monthclose-cover"),
					Title(uistate.T("coverAll.title")), OnClick(openCoverAll), uistate.T("monthclose.coverAction")),
				Span(css.Class("budget-sub"), uistate.T("monthclose.overLeaveNote"))))
	}

	// 2 — Unused money + what rollover will do with it (the explanation BEFORE the
	// month changes, not a surprise after).
	rolloverNote := uistate.T("monthclose.rolloverOff")
	if s.RolloverOn {
		rolloverNote = uistate.T("monthclose.rolloverOn")
	}
	var leftBody []any
	if len(s.Leftovers) == 0 {
		leftBody = append(leftBody, P(css.Class("budget-sub"), uistate.T("monthclose.leftNone")))
	} else {
		leftBody = append(leftBody, P(css.Class("budget-sub"),
			uistate.T("monthclose.leftIntro", fmtMoney(money.New(s.TotalLeftMinor, v.Base)), len(s.Leftovers))))
		leftBody = append(leftBody,
			P(css.Class("budget-sub"), Attr("data-testid", "monthclose-rollover-note"), rolloverNote),
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthclose-sweep"),
					Title(uistate.T("sweep.openConfig")), OnClick(openSweep), uistate.T("sweep.openConfig"))))
	}

	// 3 — Over-assignment with the honest choices inline.
	var resolveBody []any
	switch {
	case s.OverAssignedMinor == 0:
		resolveBody = append(resolveBody, P(css.Class("budget-sub"), uistate.T("monthclose.assignFits")))
	case deferred.Get():
		resolveBody = append(resolveBody, P(css.Class("budget-sub"), Attr("data-testid", "monthclose-deferred"),
			uistate.T("monthclose.deferredNote", fmtMoney(money.New(s.OverAssignedMinor, v.Base)))))
	default:
		resolveBody = append(resolveBody, P(css.Class("budget-sub"),
			uistate.T("monthclose.assignOver", fmtMoney(money.New(s.OverAssignedMinor, v.Base)))))
		var choices []ui.Node
		for _, r := range monthclose.Resolutions(s) {
			switch r {
			case monthclose.ResolveReduce:
				choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"),
					Attr("data-testid", "monthclose-resolve-reduce"), Title(uistate.T("monthclose.reduceTitle")),
					OnClick(goAllocate), uistate.T("monthclose.reduce")))
			case monthclose.ResolveIncome:
				choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"),
					Attr("data-testid", "monthclose-resolve-income"), Title(uistate.T("monthclose.incomeTitle")),
					OnClick(openBasis), uistate.T("monthclose.income")))
			case monthclose.ResolveRollover:
				choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"),
					Attr("data-testid", "monthclose-resolve-rollover"), Title(uistate.T("monthclose.rolloverTitle")),
					OnClick(enableRollover), uistate.T("monthclose.rollover")))
			case monthclose.ResolveDefer:
				choices = append(choices, Button(css.Class("btn btn-ghost btn-sm"), Type("button"),
					Attr("data-testid", "monthclose-resolve-defer"), Title(uistate.T("monthclose.deferTitle")),
					OnClick(deferResolve), uistate.T("monthclose.defer")))
			}
		}
		resolveBody = append(resolveBody, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap, tw.Mt2), choices))
	}

	// 4 — Actual vs expected income.
	delta := s.IncomeDeltaMinor()
	deltaTxt := uistate.T("monthclose.incomeMatched")
	if delta > 0 {
		deltaTxt = uistate.T("monthclose.incomeAhead", fmtMoney(money.New(delta, v.Base)))
	} else if delta < 0 {
		deltaTxt = uistate.T("monthclose.incomeBehind", fmtMoney(money.New(-delta, v.Base)))
	}
	incomeBody := []any{
		Ul(css.Class("wf-lines"),
			Li(css.Class("wf-line"),
				Span(css.Class("wf-line-name"), uistate.T("monthclose.incomeExpected")),
				Span(css.Class("wf-line-amt"), fmtMoney(money.New(s.ExpectedIncomeMinor, v.Base)))),
			Li(css.Class("wf-line"),
				Span(css.Class("wf-line-name"), uistate.T("monthclose.incomeActual")),
				Span(css.Class("wf-line-amt"), fmtMoney(money.New(s.ActualIncomeMinor, v.Base)))),
		),
		P(css.Class("budget-sub"), Attr("data-testid", "monthclose-income-delta"), deltaTxt),
	}

	// 5 — Copy last month's one-time top-ups, with per-budget exceptions.
	var copyBody []any
	if len(copyAll) == 0 {
		copyBody = append(copyBody, P(css.Class("budget-sub"), uistate.T("monthclose.copyNone")))
	} else {
		copyBody = append(copyBody, P(css.Class("budget-sub"), uistate.T("monthclose.copyIntro")))
		exSet := monthCloseExcludeSet(excluded.Get())
		var rows []ui.Node
		for _, b := range visibleBudgets {
			amt, ok := copyAll[b.ID]
			if !ok {
				continue
			}
			rows = append(rows, ui.CreateElement(monthCloseCopyRow, monthCloseCopyRowProps{
				ID: b.ID, Name: budgetTitle(b.Name, v.CatName[b.CategoryID]),
				Amount: fmtMoney(money.New(amt, v.Base)), Included: !exSet[b.ID], OnToggle: toggleExclude,
			}))
		}
		copyBody = append(copyBody, Div(css.Class("mc-copy-rows"), rows),
			Button(css.Class("btn btn-sm btn-primary", tw.Mt2), Type("button"), Attr("data-testid", "monthclose-copy-apply"),
				Title(uistate.T("monthclose.copyApplyTitle")), OnClick(applyCopy),
				uistate.T("monthclose.copyApply", len(copyPlan))))
	}

	return Div(css.Class("modal-scroll", "mc-body"), Attr("data-testid", "monthclose-body"),
		P(css.Class("budget-sub"), uistate.T("monthclose.intro", vw.Label())),
		section("monthclose-overspends", uistate.T("monthclose.overTitle"), overBody...),
		section("monthclose-leftovers", uistate.T("monthclose.leftTitle"), leftBody...),
		section("monthclose-assign", uistate.T("monthclose.assignTitle"), resolveBody...),
		section("monthclose-income", uistate.T("monthclose.incomeTitle2"), incomeBody...),
		section("monthclose-copy", uistate.T("monthclose.copyTitle"), copyBody...),
		Div(css.Class(tw.Flex, tw.JustifyEnd, tw.Mt3),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "monthclose-done"),
				OnClick(closeDone), uistate.T("monthclose.done"))),
	)
}

// monthCloseExcludeSet parses the comma-joined excluded-ID state into a set.
func monthCloseExcludeSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, id := range strings.Split(s, ",") {
		if id != "" {
			out[id] = true
		}
	}
	return out
}

// monthCloseCopyRowProps drives one copy-with-exceptions row.
type monthCloseCopyRowProps struct {
	ID       string
	Name     string
	Amount   string
	Included bool
	OnToggle func(string)
}

// monthCloseCopyRow is one budget in the copy plan: a checkbox (the exception
// control), the budget's name, and last period's top-up amount. Its own
// component so the toggle hook never registers inside a loop.
func monthCloseCopyRow(props monthCloseCopyRowProps) ui.Node {
	onToggle := ui.UseEvent(func(e ui.Event) {
		if props.OnToggle != nil {
			props.OnToggle(props.ID)
		}
	})
	return Label(css.Class("mc-copy-row", tw.Flex, tw.ItemsCenter, tw.Gap2),
		Attr("data-testid", "monthclose-copy-row-"+props.ID),
		Input(Type("checkbox"), Attr("data-testid", "monthclose-copy-check-"+props.ID),
			checkedAttr(props.Included), OnChange(onToggle)),
		Span(css.Class("wf-line-name"), props.Name),
		Span(css.Class("wf-line-amt"), props.Amount),
	)
}

// monthCloseOfferChip is the near-month-end entry point on the budgets summary:
// visible during the last days of a live period (or on a just-viewed closed one),
// it opens the guided close flow. Own component for a stable click hook.
func monthCloseOfferChip(_ struct{}) ui.Node {
	openAtom := uistate.UseMonthCloseOpen()
	onOpen := ui.UseEvent(Prevent(func() { openAtom.Set(true) }))
	return Button(css.Class("btn btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "budgets-monthclose-offer"), Title(uistate.T("monthclose.offerTitle")),
		OnClick(onOpen),
		uiw.Icon(icon.Check, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
		Span(uistate.T("monthclose.offer")))
}
