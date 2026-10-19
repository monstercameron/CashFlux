// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// The trailing windows (full months) each method learns from. The Smart "recent" take
// looks at 3 months; the Smart+ "healthy" take reviews 6 so it has enough months to
// drop a spike and still average a sustainable target.
const (
	autoBudgetRecentMonths  = 3
	autoBudgetHealthyMonths = 6
)

// AutoBudgetBody is the "Auto budget" review flip modal (mounted at the shell root by
// app.AutoBudgetHost). It learns a suggested monthly budget for each expense category
// from spending history (deterministic — no AI), and lets the user tune each target up
// or down with a slider before creating or updating the budgets. Nothing is written
// until Save.
func AutoBudgetBody(_ struct{}) ui.Node {
	app := appstate.Default
	dataRevision := uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetAutoOpen()

	base := budgetBaseCurrency("")
	if b := app.Settings().BaseCurrency; b != "" {
		base = b
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}

	// The suggestion method (Smart "recent" vs Smart+ "healthy") is a live toggle; it
	// picks the window and the per-category estimator. The slider %s and include flags
	// (keyed by category) carry across a method switch — only the learned baselines change.
	method := ui.UseState(string(budgeting.MethodRecent))
	m := budgeting.SuggestMethod(method.Get())
	months := autoBudgetRecentMonths
	if m == budgeting.MethodHealthy {
		months = autoBudgetHealthyMonths
	}
	anchor := budgetViewAnchor(uistate.UsePeriod().Get(), time.Now())
	suggestions := ui.UseMemo(func() autoBudgetSuggestionMemo {
		rows, err := budgeting.SuggestBudgets(app.Categories(), budgetLedgerForApp(app), anchor, months, rates, m)
		return autoBudgetSuggestionMemo{Rows: rows, Err: err}
	}, dataRevision, anchor.Format("2006-01"), months, m, rates.Base, app.Rev())
	suggestionRows, suggestionErr := suggestions.Rows, suggestions.Err

	// Existing monthly budgets are keyed by category so updates never duplicate them.
	existing := budgeting.AutoBudgetMatches(app.Budgets())

	// Seed per-category state: slider at 100% of the suggestion; include categories that
	// don't already have a monthly budget (so Save never silently overwrites a budget the
	// user tuned by hand — they can still tick those on deliberately).
	seedPct := make(map[string]int, len(suggestionRows))
	seedPicked := make(map[string]bool, len(suggestionRows))
	for _, s := range suggestionRows {
		seedPct[s.CategoryID] = 100
		_, has := existing[s.CategoryID]
		seedPicked[s.CategoryID] = !has
	}
	pct := ui.UseState(seedPct)
	picked := ui.UseState(seedPicked)
	currentPct := budgeting.FillAutoBudgetPercentages(pct.Get(), suggestionRows)
	currentPicked := budgeting.FillAutoBudgetSelection(picked.Get(), suggestionRows, existing)

	setPct := func(cid string, v int) {
		m := budgeting.FillAutoBudgetPercentages(pct.Get(), suggestionRows)
		nm := make(map[string]int, len(m))
		for k, val := range m {
			nm[k] = val
		}
		nm[cid] = v
		pct.Set(nm)
	}
	toggle := func(cid string) {
		m := budgeting.FillAutoBudgetSelection(picked.Get(), suggestionRows, existing)
		nm := make(map[string]bool, len(m))
		for k, val := range m {
			nm[k] = val
		}
		nm[cid] = !nm[cid]
		picked.Set(nm)
	}

	// amountFor is the tuned monthly amount (suggestion × slider %).
	amountFor := func(s budgeting.BudgetSuggestion) int64 {
		return s.MonthlyMinor * int64(currentPct[s.CategoryID]) / 100
	}

	sel := currentPicked
	nSel := 0
	var totalMinor int64
	for _, s := range suggestionRows {
		if sel[s.CategoryID] {
			nSel++
			totalMinor += amountFor(s)
		}
	}

	// C593: bulk controls, so a long list is not edited one row at a time. Select
	// all / none and a reset that returns every slider to the learned figure —
	// without them, undoing a run of tuning meant dragging each slider back by eye.
	setAll := func(on bool) {
		nm := make(map[string]bool, len(suggestionRows))
		for _, s := range suggestionRows {
			nm[s.CategoryID] = on
		}
		picked.Set(nm)
	}
	resetTuning := func() {
		nm := make(map[string]int, len(suggestionRows))
		for _, s := range suggestionRows {
			nm[s.CategoryID] = 100
		}
		pct.Set(nm)
	}
	tuned := 0
	for _, s := range suggestionRows {
		if currentPct[s.CategoryID] != 100 {
			tuned++
		}
	}

	// C593: the financial consequence, in the modal, before Save. The list showed
	// a per-category total and nothing about what it did to the plan — so the one
	// question the screen exists to answer ("can I afford this?") was answered
	// after saving, on another page. This is the household's budgeted total and
	// income basis WITH the pending changes folded in.
	pending := map[string]int64{}
	for _, s := range suggestionRows {
		if sel[s.CategoryID] {
			if amt := amountFor(s); amt > 0 {
				pending[s.CategoryID] = amt
			}
		}
	}
	// The income basis has to be the PAGE's, not today's. Reading time.Now() here
	// meant that reviewing a closed July in August compared the plan against
	// July's income while the page behind the modal compared it against June's —
	// two different months' figures for the same question, on the same screen.
	// Same anchor rule as computeBudgetView: today when the viewed window contains
	// it, otherwise the window's start.
	viewPeriod := uistate.UsePeriod().Get()
	viewFrom, viewTo := viewPeriod.Range()
	pr := uistate.CurrentPrefs()
	period := budgeting.ResolveViewPeriod(viewFrom, viewTo, time.Now(), pr.WeekStartWeekday(), parsePayCycleAnchor(pr.PayCycleAnchor))
	period.Anchor = anchor
	autoIncome, incomeErr := budgeting.ResolveViewIncome(budgeting.ViewIncomeBasis{Mode: pr.BudgetIncomeMode, PaycheckMin: pr.BudgetPaycheckMinMinor, Fixed: pr.MonthlyIncomeMinor, CategoryIDs: pr.BudgetIncomeCategoryIDs, AverageMonths: pr.BudgetIncomeAvgMonths}, budgetLedgerForApp(app), app.Categories(), period, base, rates)
	if incomeErr != nil {
		slog.Error("calculate auto-budget income", "error", incomeErr)
	}
	impact, impactErr := budgeting.AutoBudgetImpactWithRates(app.Budgets(), autoIncome, pending, rates)

	apply := ui.UseEvent(Prevent(func() {
		saveAutoBudget(app, suggestionRows, existing, base, rates, anchor, amountFor,
			func() map[string]bool {
				return budgeting.FillAutoBudgetSelection(picked.Get(), suggestionRows, existing)
			}, suggestionErr, impactErr, incomeErr, func() { openAtom.Set(false) })
	}))
	onCancel := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))
	cancelOnClick := OnClick(onCancel)
	applyOnClick := OnClick(apply)
	bulkBar := ui.CreateElement(autoBudgetBulkControl, autoBudgetBulkProps{
		Tuned:    tuned,
		OnSetAll: setAll,
		OnReset:  resetTuning,
	})

	// ---- render ----
	if len(suggestionRows) == 0 {
		return autoBudgetEmptyState(cancelOnClick)
	}

	seg := autoBudgetMethodSelector(method.Get(), method.Set)
	introKey := autoBudgetIntroKey(m)

	return renderAutoBudgetRows(suggestionRows, existing, base, currentPct, sel, setPct, toggle, nSel,
		totalMinor, tuned, suggestionErr, impactErr, incomeErr, impact, seg, introKey, months,
		bulkBar, cancelOnClick, applyOnClick)
}

type autoBudgetSuggestionMemo struct {
	Rows []budgeting.BudgetSuggestion
	Err  error
}

func parsePayCycleAnchor(raw string) time.Time {
	anchor, _ := time.Parse("2006-01-02", strings.TrimSpace(raw))
	return anchor
}

func autoBudgetEmptyState(cancelOnClick any) ui.Node {
	return Div(css.Class(tw.FlexCol, tw.Gap3),
		P(css.Class("muted"), Attr("data-testid", "autobudget-empty"), uistate.T("budgets.autoEmpty")),
		Div(css.Class("autobudget-footer"), Button(css.Class("btn"), Type("button"), cancelOnClick, uistate.T("action.close"))))
}

func autoBudgetMethodSelector(selected string, onSelect func(string)) ui.Node {
	return uiw.Segmented(uiw.SegmentedProps{
		Label: uistate.T("budgets.autoMethodLabel"), Selected: selected,
		Options: []uiw.SegOption{
			{Value: string(budgeting.MethodRecent), Label: uistate.T("budgets.autoMethodRecent"), TestID: "autobudget-method-recent"},
			{Value: string(budgeting.MethodHealthy), Label: uistate.T("budgets.autoMethodHealthy"), TestID: "autobudget-method-healthy"},
		}, OnSelect: onSelect,
	})
}

func autoBudgetIntroKey(method budgeting.SuggestMethod) string {
	if method == budgeting.MethodHealthy {
		return "budgets.autoIntroHealthy"
	}
	return "budgets.autoIntroRecent"
}

func renderAutoBudgetRows(suggestions []budgeting.BudgetSuggestion, existing map[string]domain.Budget, base string,
	pct map[string]int, selected map[string]bool, setPct func(string, int), toggle func(string), selectedCount int, totalMinor int64,
	tuned int, suggestionErr, impactErr, incomeErr error, impact budgeting.AutoBudgetImpact, seg ui.Node, introKey string, months int,
	bulkBar, cancelOnClick, applyOnClick any) ui.Node {
	var fresh, rewrite []budgeting.BudgetSuggestion
	for _, s := range suggestions {
		if _, has := existing[s.CategoryID]; has {
			rewrite = append(rewrite, s)
		} else {
			fresh = append(fresh, s)
		}
	}
	renderRow := func(s budgeting.BudgetSuggestion) ui.Node {
		_, has := existing[s.CategoryID]
		return ui.CreateElement(autoBudgetRow, autoBudgetRowProps{CategoryID: s.CategoryID, CategoryName: s.CategoryName,
			SuggestedMinor: s.MonthlyMinor, Base: base, Pct: pct[s.CategoryID], Picked: selected[s.CategoryID], HasExisting: has, OnPct: setPct, OnToggle: toggle})
	}
	group := func(titleKey string, items []budgeting.BudgetSuggestion, testID string) ui.Node {
		if len(items) == 0 {
			return Fragment()
		}
		return Fragment(Div(css.Class("autobudget-group"), Attr("data-testid", testID), uistate.T(titleKey, uistate.T(budgetCountKey("bc4b4.categoryCount", len(items)), len(items)))),
			MapKeyed(items, func(s budgeting.BudgetSuggestion) any { return s.CategoryID }, renderRow))
	}
	rows := Fragment(group("budgets.autoGroupNew", fresh, "autobudget-group-new"), group("budgets.autoGroupExisting", rewrite, "autobudget-group-existing"))
	var impactNode ui.Node = Fragment()
	if selectedCount > 0 && suggestionErr == nil && impactErr == nil && incomeErr == nil {
		var leftLine ui.Node = Fragment()
		if impact.HasIncome() {
			left, err := impact.LeftAfterChecked()
			if err != nil {
				leftLine = autoBudgetErrorNode(err, "autobudget-left-error")
			} else {
				magnitude, absErr := money.Abs(left)
				if absErr != nil {
					leftLine = autoBudgetErrorNode(absErr, "autobudget-left-error")
				} else {
					leftKey, leftCls := "budgets.autoImpactLeft", "autobudget-impact-left"
					if left < 0 {
						leftKey, leftCls = "budgets.autoImpactOver", "autobudget-impact-left is-over"
					}
					leftLine = Span(ClassStr(leftCls), Attr("data-testid", "autobudget-impact-left"),
						uistate.T(leftKey, fmtMoney(money.New(magnitude, base)), fmtMoney(money.New(impact.Income, base))))
				}
			}
		}
		impactNode = Div(css.Class("autobudget-impact"), Attr("data-testid", "autobudget-impact"),
			Span(css.Class("autobudget-impact-lbl"), uistate.T("budgets.autoImpactLabel")),
			Span(css.Class("autobudget-impact-before", tw.TextFaint), fmtMoney(money.New(impact.BudgetedBefore, base))),
			Span(css.Class(tw.TextFaint), "→"),
			Span(css.Class("autobudget-impact-after", tw.FontDisplay), Attr("data-testid", "autobudget-impact-after"), fmtMoney(money.New(impact.BudgetedAfter, base))), leftLine)
	}
	return Div(css.Class("acct-edit-form"),
		Div(css.Class("modal-scroll"), seg,
			P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), Attr("data-testid", "autobudget-intro"), uistate.T(introKey, uistate.T("budgets.autoMonths", strconv.Itoa(months)))),
			autoBudgetErrorNode(suggestionErr, "autobudget-suggestion-error"),
			autoBudgetErrorNode(impactErr, "autobudget-impact-error"),
			autoBudgetErrorNode(incomeErr, "autobudget-income-incomplete"),
			impactNode, bulkBar, Div(css.Class("autobudget-rows"), Attr("data-testid", "autobudget-rows"), rows)),
		Div(css.Class("modal-foot", "autobudget-footer"),
			Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "autobudget-total"), uistate.T("budgets.autoTotal", fmtMoney(money.New(totalMinor, base)))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "autobudget-cancel"), cancelOnClick, uistate.T("action.cancel")),
			buttonWithDisabled(selectedCount == 0 || suggestionErr != nil || impactErr != nil || incomeErr != nil,
				[]any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "autobudget-save"), applyOnClick}, uistate.T("budgets.autoSave", strconv.Itoa(selectedCount)))))
}

func autoBudgetErrorNode(err error, testID string) ui.Node {
	if err == nil {
		return Fragment()
	}
	return P(css.Class("form-err autobudget-error"), Attr("role", "status"), Attr("data-testid", testID), uistate.T("budgets.autoErrorGeneric"))
}

func saveAutoBudget(app *appstate.App, suggestions []budgeting.BudgetSuggestion, existing map[string]domain.Budget, base string, rates currency.Rates, anchor time.Time,
	amountFor func(budgeting.BudgetSuggestion) int64, selected func() map[string]bool, suggestionErr, impactErr, incomeErr error, close func()) {
	if suggestionErr != nil || impactErr != nil || incomeErr != nil {
		return
	}
	var writes []domain.Budget
	matchedAmounts := make(map[string]int64)
	matchedBudgets := make(map[string]domain.Budget)
	for _, s := range suggestions {
		if !selected()[s.CategoryID] {
			continue
		}
		amt := amountFor(s)
		if amt <= 0 {
			continue
		}
		if b, ok := existing[s.CategoryID]; ok {
			nextAmount, err := money.Add(matchedAmounts[b.ID], amt)
			if err != nil {
				uistate.PostNotice(budgetErrorText(err), true)
				return
			}
			matchedAmounts[b.ID] = nextAmount
			matchedBudgets[b.ID] = b
		} else {
			nb := domain.Budget{ID: id.New(), Name: s.CategoryName, CategoryID: s.CategoryID,
				Period: domain.PeriodMonthly, Limit: money.New(amt, base), Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID}
			writes = append(writes, nb)
		}
	}
	for budgetID, monthlyAmount := range matchedAmounts {
		b := matchedBudgets[budgetID]
		converted, err := budgeting.LimitForPeriod(money.New(monthlyAmount, base), domain.PeriodMonthly, b.Period, b.WithBaseCurrency(base).Limit.Currency, rates)
		if err != nil {
			uistate.PostNotice(budgetErrorText(err), true)
			return
		}
		b.Limit = converted
		writes = append(writes, b)
	}
	if len(writes) == 0 {
		return
	}
	if err := app.PutBudgetsAtomicAt(writes, anchor); err != nil {
		uistate.PostNotice(budgetErrorText(err), true)
		return
	}
	uistate.PostNotice(uistate.T("budgets.autoSavedToast", uistate.T(budgetCountKey("bc4b4.budgetCount", len(writes)), len(writes))), false)
	uistate.BumpDataRevision()
	close()
}

type autoBudgetBulkProps struct {
	Tuned    int
	OnSetAll func(bool)
	OnReset  func()
}

func autoBudgetBulkControl(props autoBudgetBulkProps) ui.Node {
	selectAll := ui.UseEvent(Prevent(func() { props.OnSetAll(true) }))
	selectNone := ui.UseEvent(Prevent(func() { props.OnSetAll(false) }))
	reset := ui.UseEvent(Prevent(props.OnReset))
	resetOnClick := OnClick(reset)
	return Div(css.Class("autobudget-bulk"), Attr("data-testid", "autobudget-bulk"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-all"), OnClick(selectAll), uistate.T("budgets.autoSelectAll")),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-none"), OnClick(selectNone), uistate.T("budgets.autoSelectNone")),
		If(props.Tuned > 0, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-reset"),
			resetOnClick, uistate.T("budgets.autoResetTuning", uistate.T(budgetCountKey("bc4b4.sliderCount", props.Tuned), props.Tuned)))))
}

// buttonWithDisabled builds a <button> that is disabled only when `disabled` is true —
// the empty-string `disabled` attribute still disables in HTML, so it must be appended
// conditionally rather than bound to a value.
func buttonWithDisabled(disabled bool, args []any, children ...any) ui.Node {
	if disabled {
		args = append(args, Attr("disabled", "disabled"))
	}
	args = append(args, children...)
	return Button(args...)
}

// autoBudgetRowProps drives one category's suggested-budget row: name, the learned
// monthly average, a percentage slider to tune it, and an include checkbox.
type autoBudgetRowProps struct {
	CategoryID     string
	CategoryName   string
	SuggestedMinor int64
	Base           string
	Pct            int
	Picked         bool
	HasExisting    bool
	OnPct          func(cid string, pct int)
	OnToggle       func(cid string)
}

// autoBudgetRow renders one tunable suggested budget. It is its own component so its
// slider/checkbox hooks are never registered inside the results loop (the On*-in-loop
// rule). The slider is 0–200% of the learned average, so tuning is uniform regardless
// of the category's magnitude; the resulting monthly amount updates live as it drags.
func autoBudgetRow(props autoBudgetRowProps) ui.Node {
	onSlide := ui.UseEvent(func(v string) {
		n, _ := strconv.Atoi(v)
		props.OnPct(props.CategoryID, n)
	})
	onToggle := ui.UseEvent(func() { props.OnToggle(props.CategoryID) })

	amt := props.SuggestedMinor * int64(props.Pct) / 100
	rowCls := "autobudget-row"
	if !props.Picked {
		rowCls += " is-off"
	}

	// The control line reads the slider as a percentage of the learned average, so the
	// up/down adjustment is legible even when the target equals the suggestion; the
	// readout highlights a deliberate move off 100%.
	readoutCls := "autobudget-readout"
	if props.Pct != 100 {
		readoutCls += " is-tuned"
	}
	return Div(ClassStr(rowCls), Attr("data-testid", "autobudget-row-"+props.CategoryID),
		// The pick label holds only the input, so without an explicit name the
		// accessibility tree exposed a wall of anonymous checkboxes (QA CF-13).
		Label(css.Class("autobudget-pick"),
			Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "autobudget-pick-"+props.CategoryID),
				Attr("aria-label", uistate.T("budgets.autoPickAria", props.CategoryName, fmtMoney(money.New(props.SuggestedMinor, props.Base)))),
				OnChange(onToggle)}, checkedAttr(props.Picked)...)...)),
		Div(css.Class("autobudget-main"),
			Div(css.Class("autobudget-head"),
				Span(css.Class("autobudget-name"), props.CategoryName),
				If(props.HasExisting, Span(css.Class("autobudget-tag", tw.TextDim), uistate.T("budgets.autoHasBudget")))),
			Div(css.Class("autobudget-controls"),
				Input(Type("range"), css.Class("set-range autobudget-slider"),
					Attr("min", "0"), Attr("max", "200"), Attr("step", "5"),
					Attr("data-testid", "autobudget-slider-"+props.CategoryID),
					Attr("aria-label", uistate.T("budgets.autoSliderAria", props.CategoryName)), OnInput(onSlide), uiw.FieldValue(strconv.Itoa(props.Pct))),
				Span(ClassStr(readoutCls), Attr("data-testid", "autobudget-readout-"+props.CategoryID),
					uistate.T("budgets.autoSliderReadout", strconv.Itoa(props.Pct), fmtMoney(money.New(props.SuggestedMinor, props.Base))))),
		),
		Span(css.Class("autobudget-amt", tw.FontDisplay), Attr("data-testid", "autobudget-amt-"+props.CategoryID),
			uistate.T("budgets.autoPerMo", fmtMoney(money.New(amt, props.Base)))),
	)
}
