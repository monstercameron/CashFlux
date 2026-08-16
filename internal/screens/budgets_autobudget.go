// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
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
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseBudgetAutoOpen()

	base := "USD"
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
	suggestions, _ := budgeting.SuggestBudgets(app.Categories(), app.Transactions(), time.Now(), months, rates, m)

	// Existing MONTHLY budgets by category — so we update rather than duplicate, and can
	// mark a category as already-budgeted (and not overwrite it unless the user opts in).
	existing := make(map[string]domain.Budget)
	for _, b := range app.Budgets() {
		if b.Period == domain.PeriodMonthly {
			existing[b.CategoryID] = b
		}
	}

	// Seed per-category state: slider at 100% of the suggestion; include categories that
	// don't already have a monthly budget (so Save never silently overwrites a budget the
	// user tuned by hand — they can still tick those on deliberately).
	seedPct := make(map[string]int, len(suggestions))
	seedPicked := make(map[string]bool, len(suggestions))
	for _, s := range suggestions {
		seedPct[s.CategoryID] = 100
		_, has := existing[s.CategoryID]
		seedPicked[s.CategoryID] = !has
	}
	pct := ui.UseState(seedPct)
	picked := ui.UseState(seedPicked)

	setPct := func(cid string, v int) {
		m := pct.Get()
		nm := make(map[string]int, len(m))
		for k, val := range m {
			nm[k] = val
		}
		nm[cid] = v
		pct.Set(nm)
	}
	toggle := func(cid string) {
		m := picked.Get()
		nm := make(map[string]bool, len(m))
		for k, val := range m {
			nm[k] = val
		}
		nm[cid] = !nm[cid]
		picked.Set(nm)
	}

	// amountFor is the tuned monthly amount (suggestion × slider %).
	amountFor := func(s budgeting.BudgetSuggestion) int64 {
		return s.MonthlyMinor * int64(pct.Get()[s.CategoryID]) / 100
	}

	sel := picked.Get()
	nSel := 0
	var totalMinor int64
	for _, s := range suggestions {
		if sel[s.CategoryID] {
			nSel++
			totalMinor += amountFor(s)
		}
	}

	// C593: bulk controls, so a long list is not edited one row at a time. Select
	// all / none and a reset that returns every slider to the learned figure —
	// without them, undoing a run of tuning meant dragging each slider back by eye.
	setAll := func(on bool) {
		nm := make(map[string]bool, len(suggestions))
		for _, s := range suggestions {
			nm[s.CategoryID] = on
		}
		picked.Set(nm)
	}
	selectAll := ui.UseEvent(Prevent(func() { setAll(true) }))
	selectNone := ui.UseEvent(Prevent(func() { setAll(false) }))
	resetTuning := ui.UseEvent(Prevent(func() {
		nm := make(map[string]int, len(suggestions))
		for _, s := range suggestions {
			nm[s.CategoryID] = 100
		}
		pct.Set(nm)
	}))
	tuned := 0
	for _, s := range suggestions {
		if pct.Get()[s.CategoryID] != 100 {
			tuned++
		}
	}

	// C593: the financial consequence, in the modal, before Save. The list showed
	// a per-category total and nothing about what it did to the plan — so the one
	// question the screen exists to answer ("can I afford this?") was answered
	// after saving, on another page. This is the household's budgeted total and
	// income basis WITH the pending changes folded in.
	pending := map[string]int64{}
	for _, s := range suggestions {
		if sel[s.CategoryID] {
			if amt := amountFor(s); amt > 0 {
				pending[s.CategoryID] = amt
			}
		}
	}
	now := time.Now()
	monthStart := dateutil.MonthStart(now)
	autoIncome := budgeting.IncomeForBudgets(uistate.CurrentPrefs().MonthlyIncomeMinor,
		app.Transactions(), dateutil.AddMonths(monthStart, -1), monthStart, base, rates)
	impact := budgeting.AutoBudgetImpactOf(app.Budgets(), autoIncome, pending)

	apply := ui.UseEvent(Prevent(func() {
		saved := 0
		for _, s := range suggestions {
			if !picked.Get()[s.CategoryID] {
				continue
			}
			amt := amountFor(s)
			if amt <= 0 {
				continue
			}
			if b, ok := existing[s.CategoryID]; ok {
				b.Limit = money.New(amt, base)
				b.Period = domain.PeriodMonthly
				if err := app.PutBudget(b); err != nil {
					uistate.PostNotice(err.Error(), true)
					continue
				}
			} else {
				nb := domain.Budget{
					ID: id.New(), Name: s.CategoryName, CategoryID: s.CategoryID,
					Period: domain.PeriodMonthly, Limit: money.New(amt, base),
					Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
				}
				if err := app.PutBudget(nb); err != nil {
					uistate.PostNotice(err.Error(), true)
					continue
				}
			}
			saved++
		}
		if saved > 0 {
			uistate.PostNotice(uistate.T("budgets.autoSavedToast", plural(saved, "budget")), false)
		}
		uistate.BumpDataRevision()
		openAtom.Set(false)
	}))
	onCancel := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))

	// ---- render ----
	if len(suggestions) == 0 {
		return Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted"), Attr("data-testid", "autobudget-empty"), uistate.T("budgets.autoEmpty")),
			Div(css.Class("autobudget-footer"),
				Button(css.Class("btn"), Type("button"), OnClick(onCancel), uistate.T("action.close"))))
	}

	seg := uiw.Segmented(uiw.SegmentedProps{
		Label:    uistate.T("budgets.autoMethodLabel"),
		Selected: method.Get(),
		Options: []uiw.SegOption{
			{Value: string(budgeting.MethodRecent), Label: uistate.T("budgets.autoMethodRecent"), TestID: "autobudget-method-recent"},
			{Value: string(budgeting.MethodHealthy), Label: uistate.T("budgets.autoMethodHealthy"), TestID: "autobudget-method-healthy"},
		},
		OnSelect: func(v string) { method.Set(v) },
	})
	introKey := "budgets.autoIntroRecent"
	if m == budgeting.MethodHealthy {
		introKey = "budgets.autoIntroHealthy"
	}

	keyOf := func(s budgeting.BudgetSuggestion) any { return s.CategoryID }
	// C593: the list is split into what this run would CREATE and what it would
	// REWRITE. Both were one undifferentiated column, distinguished only by a
	// small tag on the rows that already had a budget — which is the difference
	// between "add these" and "overwrite the numbers you tuned by hand".
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
		return ui.CreateElement(autoBudgetRow, autoBudgetRowProps{
			CategoryID: s.CategoryID, CategoryName: s.CategoryName,
			SuggestedMinor: s.MonthlyMinor, Base: base,
			Pct: pct.Get()[s.CategoryID], Picked: sel[s.CategoryID], HasExisting: has,
			OnPct: setPct, OnToggle: toggle,
		})
	}
	group := func(titleKey string, items []budgeting.BudgetSuggestion, testID string) ui.Node {
		if len(items) == 0 {
			return Fragment()
		}
		return Fragment(
			Div(css.Class("autobudget-group"), Attr("data-testid", testID),
				uistate.T(titleKey, plural(len(items), "category"))),
			MapKeyed(items, keyOf, renderRow),
		)
	}
	rows := Fragment(
		group("budgets.autoGroupNew", fresh, "autobudget-group-new"),
		group("budgets.autoGroupExisting", rewrite, "autobudget-group-existing"),
	)

	// The bulk controls, above the list: tick or clear everything, and put every
	// slider back to what was learned.
	bulkBar := Div(css.Class("autobudget-bulk"), Attr("data-testid", "autobudget-bulk"),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-all"),
			OnClick(selectAll), uistate.T("budgets.autoSelectAll")),
		Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-none"),
			OnClick(selectNone), uistate.T("budgets.autoSelectNone")),
		If(tuned > 0, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "autobudget-reset"),
			OnClick(resetTuning), uistate.T("budgets.autoResetTuning", plural(tuned, "slider")))),
	)

	// The plan-level consequence, kept in view above the list: what the household
	// budgets now, what it would budget after saving, and what that leaves of the
	// income the plan is measured against.
	var impactNode ui.Node = Fragment()
	if nSel > 0 {
		var leftLine ui.Node = Fragment()
		if impact.HasIncome() {
			leftKey := "budgets.autoImpactLeft"
			leftCls := "autobudget-impact-left"
			if impact.LeftAfter() < 0 {
				leftKey, leftCls = "budgets.autoImpactOver", "autobudget-impact-left is-over"
			}
			leftLine = Span(ClassStr(leftCls), Attr("data-testid", "autobudget-impact-left"),
				uistate.T(leftKey, fmtMoney(money.New(absMinor(impact.LeftAfter()), base)),
					fmtMoney(money.New(impact.Income, base))))
		}
		impactNode = Div(css.Class("autobudget-impact"), Attr("data-testid", "autobudget-impact"),
			Span(css.Class("autobudget-impact-lbl"), uistate.T("budgets.autoImpactLabel")),
			Span(css.Class("autobudget-impact-before", tw.TextFaint), fmtMoney(money.New(impact.BudgetedBefore, base))),
			Span(css.Class(tw.TextFaint), "→"),
			Span(css.Class("autobudget-impact-after", tw.FontDisplay), Attr("data-testid", "autobudget-impact-after"),
				fmtMoney(money.New(impact.BudgetedAfter, base))),
			leftLine,
		)
	}

	// Use the standard modal shell (.acct-edit-form: min-height:100% flex column) so the
	// .modal-scroll body fills the FlushBody panel and the .modal-foot stays pinned to
	// the bottom even when few categories are listed — matching the Add-budget modal.
	return Div(css.Class("acct-edit-form"),
		Div(css.Class("modal-scroll"),
			seg,
			P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}),
				Attr("data-testid", "autobudget-intro"),
				uistate.T(introKey, uistate.T("budgets.autoMonths", strconv.Itoa(months)))),
			impactNode,
			bulkBar,
			Div(css.Class("autobudget-rows"), Attr("data-testid", "autobudget-rows"), rows)),
		Div(css.Class("modal-foot", "autobudget-footer"),
			Span(css.Class("autobudget-total", tw.TextDim), Attr("data-testid", "autobudget-total"),
				uistate.T("budgets.autoTotal", fmtMoney(money.New(totalMinor, base)))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "autobudget-cancel"), OnClick(onCancel), uistate.T("action.cancel")),
			buttonWithDisabled(nSel == 0, []any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "autobudget-save"), OnClick(apply)},
				uistate.T("budgets.autoSave", strconv.Itoa(nSel)))))
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
					Attr("aria-label", uistate.T("budgets.autoSliderAria", props.CategoryName)),
					Value(strconv.Itoa(props.Pct)), OnInput(onSlide)),
				Span(ClassStr(readoutCls), Attr("data-testid", "autobudget-readout-"+props.CategoryID),
					uistate.T("budgets.autoSliderReadout", strconv.Itoa(props.Pct), fmtMoney(money.New(props.SuggestedMinor, props.Base))))),
		),
		Span(css.Class("autobudget-amt", tw.FontDisplay), Attr("data-testid", "autobudget-amt-"+props.CategoryID),
			uistate.T("budgets.autoPerMo", fmtMoney(money.New(amt, props.Base)))),
	)
}
