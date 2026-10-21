// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
	"github.com/monstercameron/CashFlux/internal/reports"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
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
	if app == nil || app.Store() == nil {
		return monthclose.Summary{Error: fmt.Errorf("read month-close income: app is unavailable")}
	}
	if err := app.Store().CheckReady(); err != nil {
		return monthclose.Summary{Error: fmt.Errorf("read month-close income: %w", err)}
	}
	transactions, transactionsErr := app.TransactionsChecked()
	settings, settingsErr := app.SettingsChecked()
	if err := errors.Join(transactionsErr, settingsErr); err != nil {
		return monthclose.Summary{Error: fmt.Errorf("read month-close income: %w", err)}
	}
	rates := currency.Rates{Base: v.Base, Rates: settings.FXRates}
	actual := int64(0)
	if lines, err := reports.IncomeByCategoryChecked(transactions, bs, be, rates); err == nil {
		for _, ln := range lines {
			next, addErr := money.Add(actual, ln.Amount)
			if addErr != nil {
				return monthclose.Summary{Error: addErr}
			}
			actual = next
		}
	} else {
		return monthclose.Summary{Error: err}
	}
	// Same one definition the rail and the allocation bar use — see the note in
	// budgetSummaryWidget. This copy had the old two-method switch, so the review
	// could report a plan that fits while the page behind it said otherwise.
	overAssigned := int64(0)
	pool, err := money.Add(v.BannerIncome, v.RolledOver)
	if err != nil {
		return monthclose.Summary{Error: err}
	}
	planned, err := money.Add(v.TotalLimit, v.SavingsAssigned)
	if err != nil {
		return monthclose.Summary{Error: err}
	}
	if pool > 0 {
		toAssign, err := money.Sub(pool, planned)
		if err != nil {
			return monthclose.Summary{Error: err}
		}
		if toAssign < 0 {
			overAssigned, err = money.Neg(toAssign)
			if err != nil {
				return monthclose.Summary{Error: err}
			}
		}
	}
	nameOf := func(b domain.Budget) string { return budgetTitle(b.Name, v.CatName[b.CategoryID]) }
	rolloverOn := uistate.CurrentPrefs().BudgetRolloverLeftover
	return monthclose.Build(v.Statuses, nameOf, v.BannerIncome, actual, overAssigned, rolloverOn)
}

// budgetsMonthCloseModal renders the month-close flip modal when its atom is set,
// as a surface-root sibling of the bento (no tile transform clips it).
func budgetsMonthCloseModal(_ struct{}) ui.Node {
	openAtom := uistate.UseMonthCloseOpen()
	vw := uistate.UsePeriod().Get()
	if !openAtom.Get() {
		return Fragment()
	}
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

func monthCloseSection(testid, title string, kids ...any) ui.Node {
	args := []any{css.Class("mc-section"), Attr("data-testid", testid),
		Style(map[string]string{"padding": "0.85rem 1.1rem", "border-bottom": "1px solid color-mix(in srgb, var(--border) 60%, transparent)"}),
		H3(css.Class("mc-section-title"), Style(map[string]string{"margin": "0 0 0.4rem", "font-size": "0.95rem", "font-weight": "600", "color": "var(--text)"}), title)}
	return Div(append(args, kids...)...)
}

func monthClosePeriodEnded(end, now time.Time) bool {
	return !dateutil.WallClockUTC(now).Before(end)
}

type monthCloseLeftoverLine struct {
	name, amount, monthly string
	outcome               string
	rollover              bool
}

func monthCloseLeftovers(items []monthclose.Item, budgets []domain.Budget, base string) ([]monthCloseLeftoverLine, int64) {
	lines, total, _ := monthCloseLeftoversChecked(items, budgets, base)
	return lines, total
}

func monthCloseLeftoversChecked(items []monthclose.Item, budgets []domain.Budget, base string) ([]monthCloseLeftoverLine, int64, error) {
	byID := make(map[string]domain.Budget, len(budgets))
	for _, budget := range budgets {
		byID[budget.ID] = budget
	}
	lines := make([]monthCloseLeftoverLine, 0, len(items))
	var total int64
	for _, item := range items {
		budget, ok := byID[item.BudgetID]
		if !ok {
			continue
		}
		monthly, err := budgeting.PeriodEquivalent(money.New(item.Minor, base), budget.Period, domain.PeriodMonthly)
		if err != nil {
			return nil, 0, err
		}
		next, err := money.Add(total, monthly.Amount)
		if err != nil {
			return nil, 0, err
		}
		total = next
		lines = append(lines, monthCloseLeftoverLine{
			name: item.Name, amount: fmtMoney(money.New(item.Minor, base)), monthly: fmtMoney(monthly),
			outcome: monthCloseLeftoverOutcomeKey(budget.Rollover), rollover: budget.Rollover,
		})
	}
	return lines, total, nil
}

func monthCloseLeftoverOutcomeKey(rollover bool) string {
	if rollover {
		return "bv7g.monthclose.carries"
	}
	return "bv7g.monthclose.resets"
}

func monthCloseOverBody(s monthclose.Summary, base string, tense func(string, string) string, coverOnClick any) []any {
	if len(s.Overspends) == 0 {
		return []any{P(css.Class("budget-sub"), uistate.T(tense("monthclose.overNoneLive", "monthclose.overNone")))}
	}
	rows := make([]ui.Node, 0, len(s.Overspends))
	for _, it := range s.Overspends {
		rows = append(rows, Li(css.Class("wf-line"), Span(css.Class("wf-line-name"), it.Name),
			Span(css.Class("wf-line-amt"), fmtMoney(money.New(it.Minor, base)))))
	}
	return []any{
		P(css.Class("budget-sub"), uistate.TN(tense("monthclose.overIntroLiveOne", "monthclose.overIntroOne"),
			tense("monthclose.overIntroLiveMany", "monthclose.overIntroMany"), len(s.Overspends), fmtMoney(money.New(s.TotalOverMinor, base)))),
		Ul(css.Class("wf-lines"), rows),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
			Button(css.Class("btn btn-sm btn-primary"), Type("button"), Attr("data-testid", "monthclose-cover"),
				Title(uistate.T("bv7g.cover.action")), coverOnClick, uistate.T("bv7g.cover.action")),
			Span(css.Class("budget-sub"), uistate.T("monthclose.overLeaveNote"))),
	}
}

func monthCloseLeftBody(s monthclose.Summary, budgets []domain.Budget, base string, sweepOnClick any) []any {
	if len(s.Leftovers) == 0 {
		return []any{P(css.Class("budget-sub"), uistate.T("monthclose.leftNoneLive"))}
	}
	leftovers, monthlyTotal, err := monthCloseLeftoversChecked(s.Leftovers, budgets, base)
	if err != nil {
		return []any{P(css.Class("err"), Attr("role", "alert"), uistate.T("budgets.incomplete"))}
	}
	rows := make([]ui.Node, 0, len(s.Leftovers))
	for _, item := range leftovers {
		rows = append(rows, Div(css.Class("mc-leftover-row"), Attr("role", "row"),
			Span(css.Class("mc-leftover-name"), Attr("role", "cell"), item.name),
			Span(css.Class("mc-leftover-amount"), Attr("role", "cell"), item.amount),
			Span(css.Class("mc-leftover-monthly"), Attr("role", "cell"), item.monthly),
			Span(css.Class("mc-leftover-outcome"), Attr("role", "cell"), uistate.T(item.outcome))))
	}
	body := []any{Div(css.Class("mc-leftovers"), Attr("role", "table"), Attr("aria-label", uistate.T("bv7g.monthclose.leftTitle")),
		Div(css.Class("mc-leftover-head"), Attr("role", "row"),
			Span(Attr("role", "columnheader"), uistate.T("bv7g.monthclose.budget")), Span(Attr("role", "columnheader"), uistate.T("bv7g.monthclose.left")),
			Span(Attr("role", "columnheader"), uistate.T("bv7g.monthclose.monthly")), Span(Attr("role", "columnheader"), uistate.T("bv7g.monthclose.nextPeriod"))),
		Div(css.Class("mc-leftover-rows"), rows)),
		P(css.Class("budget-sub"), Attr("data-testid", "monthclose-left-total"),
			uistate.T("bv7g.monthclose.leftTotal", fmtMoney(money.New(monthlyTotal, base))))}
	return append(body,
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mt2),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthclose-sweep"),
				Title(uistate.T("sweep.openConfig")), sweepOnClick, uistate.T("sweep.openConfig"))))
}

func monthCloseResolveBody(s monthclose.Summary, base string, deferred bool, allocateOnClick, basisOnClick, rolloverOnClick, deferOnClick any) []any {
	if s.OverAssignedMinor == 0 {
		return []any{P(css.Class("budget-sub"), uistate.T("monthclose.assignFits"))}
	}
	if deferred {
		return []any{P(css.Class("budget-sub"), Attr("data-testid", "monthclose-deferred"),
			uistate.T("monthclose.deferredNote", fmtMoney(money.New(s.OverAssignedMinor, base))))}
	}
	choices := make([]ui.Node, 0, 4)
	for _, r := range monthclose.Resolutions(s) {
		switch r {
		case monthclose.ResolveReduce:
			choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthclose-resolve-reduce"),
				Title(uistate.T("monthclose.reduceTitle")), allocateOnClick, uistate.T("monthclose.reduce")))
		case monthclose.ResolveIncome:
			choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthclose-resolve-income"),
				Title(uistate.T("monthclose.incomeTitle")), basisOnClick, uistate.T("monthclose.income")))
		case monthclose.ResolveRollover:
			choices = append(choices, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "monthclose-resolve-rollover"),
				Title(uistate.T("monthclose.rolloverTitle")), rolloverOnClick, uistate.T("monthclose.rollover")))
		case monthclose.ResolveDefer:
			choices = append(choices, Button(css.Class("btn btn-ghost btn-sm"), Type("button"), Attr("data-testid", "monthclose-resolve-defer"),
				Title(uistate.T("monthclose.deferTitle")), deferOnClick, uistate.T("monthclose.defer")))
		}
	}
	return []any{P(css.Class("budget-sub"), uistate.T("monthclose.assignOver", fmtMoney(money.New(s.OverAssignedMinor, base)))),
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.FlexWrap, tw.Mt2), choices)}
}

func monthCloseIncomeBody(s monthclose.Summary, base string, ended bool) []any {
	deltaText := monthCloseIncomeDeltaText(s, base, ended)
	return []any{
		Ul(css.Class("wf-lines"),
			Li(css.Class("wf-line"), Span(css.Class("wf-line-name"), uistate.T("monthclose.incomeExpected")),
				Span(css.Class("wf-line-amt"), fmtMoney(money.New(s.ExpectedIncomeMinor, base)))),
			Li(css.Class("wf-line"), Span(css.Class("wf-line-name"), uistate.T("monthclose.incomeActual")),
				Span(css.Class("wf-line-amt"), fmtMoney(money.New(s.ActualIncomeMinor, base))))),
		P(css.Class("budget-sub"), Attr("data-testid", "monthclose-income-delta"), deltaText),
	}
}

func monthCloseIncomeDeltaText(s monthclose.Summary, base string, ended bool) string {
	delta, err := s.IncomeDeltaChecked()
	if err != nil {
		return uistate.T("budgets.incomplete")
	}
	if !ended {
		return uistate.T("bv6g.monthclose.incomeLive", fmtMoney(money.New(s.ActualIncomeMinor, base)), fmtMoney(money.New(s.ExpectedIncomeMinor, base)))
	}
	if delta > 0 {
		return uistate.T("monthclose.incomeAhead", fmtMoney(money.New(delta, base)))
	}
	if delta < 0 {
		return uistate.T("monthclose.incomeBehind", fmtMoney(money.New(-delta, base)))
	}
	return uistate.T("monthclose.incomeMatched")
}

func monthCloseCopyBody(budgets []domain.Budget, catNames map[string]string, copyAll, copyPlan map[string]int64, excluded map[string]bool,
	toggleExclude func(string), applyCopyOnClick any, base string) []any {
	if len(copyAll) == 0 {
		return []any{P(css.Class("budget-sub"), uistate.T("monthclose.copyNone"))}
	}
	rows := make([]ui.Node, 0, len(copyAll))
	for _, b := range budgets {
		amt, ok := copyAll[b.ID]
		if !ok {
			continue
		}
		rows = append(rows, ui.CreateElement(monthCloseCopyRow, monthCloseCopyRowProps{
			ID: b.ID, Name: budgetTitle(b.Name, catNames[b.CategoryID]), Amount: fmtMoney(money.New(amt, b.WithBaseCurrency(base).Limit.Currency)),
			Included: !excluded[b.ID], OnToggle: toggleExclude,
		}))
	}
	return []any{
		P(css.Class("budget-sub"), uistate.T("bcs2.copy.intro")),
		Div(css.Class("mc-copy-rows"), rows),
		Button(css.Class("btn btn-sm btn-primary", tw.Mt2), Type("button"), Attr("data-testid", "monthclose-copy-apply"),
			Title(uistate.T("monthclose.copyApplyTitle")), applyCopyOnClick, uistate.T("monthclose.copyApply", len(copyPlan))),
	}
}

func applyMonthCloseCopy(app *appstate.App, budgets []domain.Budget, anchor time.Time, weekStart time.Weekday, payAnchor time.Time, excluded func() string) error {
	writes, err := monthclose.ApplyCopyBoosts(budgets, func(b domain.Budget) (time.Time, time.Time) {
		return monthclose.CarryTargetsAnchored(b.Period, anchor, weekStart, payAnchor)
	}, monthCloseExcludeSet(excluded()))
	if err != nil {
		return err
	}
	if len(writes) == 0 {
		return nil
	}
	if err := app.PutBudgetsAtomicAt(writes, anchor); err != nil {
		return fmt.Errorf("save month-close top-ups: %w", err)
	}
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	uistate.PostUndoable(uistate.T("bcs2.copy.applied", budgetCountPhrase(len(writes))))
	return nil
}

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
	summaryError := s.Error != nil
	// The offer appears in a period's LAST FIVE DAYS as well as after it closes, so
	// this flow is usually read while the month is still running. Every line was
	// written in the past tense regardless — "went over", "went unspent", "next
	// month simply starts honest" — which on the 27th describes money that has not
	// been spent YET as money that was left over. The rest of this surface already
	// switches tense on a closed period (budgets.histUnspent, incomeActualEnded);
	// this flow never did (Cam, 2026-08-31). Half-open range: the period is closed
	// only once now is at or past its exclusive end.
	ended := monthClosePeriodEnded(be, time.Now())
	tense := func(live, past string) string {
		if ended {
			return past
		}
		return live
	}

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

	// These three open ANOTHER modal, and they used to dismiss this one first — so a
	// flow whose whole premise is walking the month's loose ends in order ejected you
	// at the first thing you acted on, and finishing meant re-opening it and
	// re-finding your place. Four of its six actions did this. They now leave the
	// review standing underneath, so closing the sub-modal returns you to the step
	// you were on (Cam, 2026-08-31). goAllocate below still dismisses, because it
	// leaves the page entirely and a modal cannot survive that.
	openCoverAll := ui.UseEvent(Prevent(func() { coverAllAtom.Set(true) }))
	openSweep := ui.UseEvent(Prevent(func() { sweepAtom.Set(true) }))
	openBasis := ui.UseEvent(Prevent(func() {
		basisDraft.Set(uistate.NewBudgetBasisDraft(uistate.CurrentPrefs()))
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
		anchor = dateutil.WallClockUTC(time.Now())
	}
	// Carry FORWARD: the reviewed period's top-ups into the next one. See
	// monthclose.CarryTargets for why the old (last, this) pair could never land
	// usefully in the window this flow is offered.
	periodStarts := func(b domain.Budget) (time.Time, time.Time) {
		return monthclose.CarryTargetsAnchored(b.Period, anchor, ws, budgetPayCycleAnchor(pr.PayCycleAnchor))
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
		if err := applyMonthCloseCopy(app, visibleBudgets, anchor, ws, budgetPayCycleAnchor(pr.PayCycleAnchor), excluded.Get); err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
		}
	}))
	coverOnClick := OnClick(openCoverAll)
	sweepOnClick := OnClick(openSweep)
	allocateOnClick := OnClick(goAllocate)
	basisOnClick := OnClick(openBasis)
	rolloverOnClick := OnClick(enableRollover)
	deferOnClick := OnClick(deferResolve)
	applyCopyOnClick := OnClick(applyCopy)
	doneOnClick := OnClick(closeDone)

	// 1 — Overspending.
	overBody := monthCloseOverBody(s, v.Base, tense, coverOnClick)

	// 2 — Unused money + what rollover will do with it (the explanation BEFORE the
	// month changes, not a surprise after).
	leftBody := monthCloseLeftBody(s, app.Budgets(), v.Base, sweepOnClick)

	// 3 — Over-assignment with the honest choices inline.
	resolveBody := monthCloseResolveBody(s, v.Base, deferred.Get(), allocateOnClick, basisOnClick, rolloverOnClick, deferOnClick)

	// 4 — Actual vs expected income.
	incomeBody := monthCloseIncomeBody(s, v.Base, ended)

	// 5 — Copy last month's one-time top-ups, with per-budget exceptions.
	copyBody := monthCloseCopyBody(visibleBudgets, v.CatName, copyAll, copyPlan, monthCloseExcludeSet(excluded.Get()), toggleExclude, applyCopyOnClick, v.Base)
	if summaryError {
		return Div(css.Class("modal-scroll", "mc-body"),
			P(css.Class("err"), Attr("role", "alert"), uistate.T("budgets.incomplete")),
			Button(css.Class("btn"), Type("button"), doneOnClick, uistate.T("action.done")))
	}

	return Div(css.Class("modal-scroll", "mc-body"), Attr("data-testid", "monthclose-body"),
		P(css.Class("budget-sub"), uistate.T("monthclose.intro", vw.Label())),
		monthCloseSection("monthclose-overspends", uistate.T("bv7g.monthclose.step1"), overBody...),
		monthCloseSection("monthclose-leftovers", uistate.T("bv7g.monthclose.step2"), leftBody...),
		monthCloseSection("monthclose-assign", uistate.T("bv7g.monthclose.step3"), resolveBody...),
		monthCloseSection("monthclose-income", uistate.T("bv7g.monthclose.step4"), incomeBody...),
		monthCloseSection("monthclose-copy", uistate.T("bv7g.monthclose.step5"), copyBody...),
		Div(css.Class(tw.Flex, tw.JustifyEnd, tw.Mt3),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "monthclose-done"),
				doneOnClick, uistate.T("monthclose.done"))),
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
