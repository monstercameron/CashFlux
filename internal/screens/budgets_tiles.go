// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// budgets_tiles.go holds the Native widget bodies the /budgets surface host composes
// (see budgets_widget.go). Each is a self-contained engine tile: it reads the live
// store (props.App) and the shared budgets-page atoms, never surface-local closures.
// The rich per-budget row (BudgetRow) is reused verbatim — these tiles only restructure
// the page around it, matching the /accounts + /transactions surface pattern.

type budgetSummaryProps struct{ App *appstate.App }

type budgetToolbarProps struct{ App *appstate.App }

type budgetListProps struct{ App *appstate.App }

type budgetFormulaProps struct{ App *appstate.App }

// --- budget-summary --------------------------------------------------------------

// budgetSummaryWidget is the health-summary tile: the spent / budgeted / left stat
// grid, the income-vs-methodology assign banner, the sinking-fund set-aside note, and
// the over/near alert banner + count badges. It renders nothing when there are no
// budgets (the list tile owns the first-run CTA), so the surface stays clean.
func budgetSummaryWidget(props budgetSummaryProps) ui.Node {
	// Subscribe to the data revision so totals/badges refresh after any mutation — the
	// tile's props are the same *App pointer across host renders.
	_ = uistate.UseDataRevision().Get()
	app := props.App
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr)
	if len(v.Statuses) == 0 {
		return Fragment()
	}
	smartSettings := uistate.LoadSmartSettings()

	// "Spent" is only red once there's actually spending — red on $0.00 reads as an
	// error rather than a healthy "nothing spent yet" (design critique).
	spentTone := ""
	if v.TotalSpent > 0 {
		spentTone = "neg"
	}
	// The summary is a single big "loader" — an overall spent-of-budgeted progress bar
	// with the spent / budgeted / left figures rendered INSIDE it, so the numbers and the
	// visual fill read as one unit. The fill grows with spending and turns amber/red as it
	// nears/exceeds the total; "Left" (safe-to-spend) is the hero figure on the right.
	leftM := money.New(v.TotalLimit-v.TotalSpent, v.Base)
	over := v.TotalSpent > v.TotalLimit
	fillPct := 0
	if v.TotalLimit > 0 {
		fillPct = int(v.TotalSpent * 100 / v.TotalLimit)
	}
	fillW := fillPct
	if fillW > 100 {
		fillW = 100
	}
	if fillW < 0 {
		fillW = 0
	}
	fillCls := "budget-loader-fill"
	switch {
	case over:
		fillCls += " is-over"
	case fillPct >= 85:
		fillCls += " is-near"
	}
	loaderCls := "budget-loader"
	if over {
		loaderCls += " is-over"
	}
	statGrid := Div(ClassStr(loaderCls),
		Attr("role", "progressbar"), Attr("aria-valuenow", strconv.Itoa(fillW)),
		Attr("aria-valuemin", "0"), Attr("aria-valuemax", "100"),
		Attr("aria-label", uistate.T("budgets.progressLabel")),
		Div(ClassStr(fillCls), Attr("style", fmt.Sprintf("width:%d%%", fillW))),
		Div(css.Class("budget-loader-figs"),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("budgets.spent")),
				Div(ClassStr("budget-loader-value "+spentTone), fmtMoney(money.New(v.TotalSpent, v.Base))),
			),
			Div(css.Class("budget-loader-fig"),
				Div(css.Class("budget-loader-label"), uistate.T("budgets.budgeted")),
				Div(css.Class("budget-loader-value"), fmtMoney(money.New(v.TotalLimit, v.Base))),
			),
			// "Left" (safe-to-spend) is the key figure — annotated with a smart explainer.
			Div(css.Class("budget-loader-fig", "is-right"),
				Div(css.Class("budget-loader-label "+tw.Fold(tw.InlineFlex, tw.ItemsCenter, tw.Gap1)),
					uistate.T("budgets.left"),
					smartTooltipFor(smartSettings, "budget-safe", uistate.T("budgets.left"), uistate.T("smart.tipBudgetSafe")),
				),
				Div(ClassStr("budget-loader-value is-hero "+accentFor(leftM)), budgetLeftValue(leftM)),
			),
		),
	)

	body := Div(
		statGrid,
		// C130: clarify a custom top-bar range only changes the view window — it doesn't
		// redefine each budget's own period.
		If(!vw.IsSinglePeriod(), P(css.Class("muted"), Attr("data-testid", "budgets-custom-range-hint"),
			uistate.T("budgets.customRangeHint"))),
		budgetAssignBanner(v),
		budgetFundSetAsideNode(v),
		// C125: lead with a salient over-spend banner, with the count/near pills below.
		If(v.OverCount > 0, Div(css.Class("card-alert", "budget-over-banner", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Attr("role", "status"), Attr("data-testid", "budgets-over-banner"),
			Span(css.Class("budget-over-icon"), Attr("aria-hidden", "true"), "⚠"),
			Span(css.Class("budget-over-text"), overBannerText(v.OverCount, fmtMoney(money.New(v.TotalOver, v.Base)))),
		)),
		If(v.OverCount > 0 || v.NearCount > 0, P(css.Class("budget-sub", tw.Flex, tw.ItemsCenter, tw.Gap2),
			If(v.OverCount > 0, Span(css.Class("pill is-danger"), uistate.T("budgets.overBadge", v.OverCount))),
			If(v.NearCount > 0, Span(css.Class("pill is-warn"), uistate.T("budgets.nearBadge", v.NearCount))),
		)),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// budgetAssignBanner renders the method-specific income context line: simple mode
// shows income · budgeted · the unbudgeted/over gap; zero-based shows the amount left
// to assign; envelope shows a short note. Pure (no hooks) — a plain node builder.
// overBannerText renders the over-budget banner copy with correct grammar for a
// single over-budget category (the plural string read "1 budgets are over").
func overBannerText(count int, total string) string {
	if count == 1 {
		return uistate.T("budgets.overBannerOne", total)
	}
	return uistate.T("budgets.overBanner", count, total)
}

func budgetAssignBanner(v budgetView) ui.Node {
	switch v.Method {
	case budgeting.MethodSimple:
		unbudgeted := v.BannerIncome - v.TotalLimit
		var diff ui.Node
		switch {
		case unbudgeted > 0:
			diff = Span(css.Class(tw.TextUp), uistate.T("budgets.simpleUnbudgeted", fmtMoney(money.New(unbudgeted, v.Base))))
		case unbudgeted == 0:
			diff = Span(uistate.T("budgets.simpleFullyAllocated"))
		default:
			diff = Span(css.Class(tw.TextDown), uistate.T("budgets.simpleOverAllocated", fmtMoney(money.New(-unbudgeted, v.Base))))
		}
		return P(css.Class("budget-sub", tw.FontDisplay),
			uistate.T("budgets.simpleIncome", fmtMoney(money.New(v.BannerIncome, v.Base))), " · ",
			uistate.T("budgets.simpleBudgeted", fmtMoney(money.New(v.TotalLimit, v.Base))), " · ", diff)
	case budgeting.MethodZeroBased:
		toAssign := budgeting.ToAssign(v.BannerIncome, v.TotalLimit)
		switch {
		case toAssign > 0:
			return P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.toAssign", fmtMoney(money.New(toAssign, v.Base))))
		case toAssign == 0:
			return P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.allAssigned"))
		default:
			return P(css.Class("budget-sub", tw.FontDisplay, tw.TextDown), uistate.T("budgets.overAssigned", fmtMoney(money.New(-toAssign, v.Base))))
		}
	case budgeting.MethodEnvelope:
		return P(css.Class("budget-sub", tw.FontDisplay), uistate.T("budgets.envelopeNote"))
	}
	return Fragment()
}

// budgetFundSetAsideNode shows the household's total monthly sinking-fund commitment,
// when non-zero — placed after the income context so it reads as a committed slice.
func budgetFundSetAsideNode(v budgetView) ui.Node {
	if v.TotalFundSetAside <= 0 {
		return Fragment()
	}
	return P(css.Class("budget-sub", tw.FontDisplay), Attr("data-testid", "budgets-fund-setaside"),
		uistate.T("budgets.fundSetAside", fmtMoney(money.New(v.TotalFundSetAside, v.Base))))
}

// --- budget-toolbar --------------------------------------------------------------

// budgetToolbarWidget is the actions tile: the in-context methodology picker, the
// one-click 50/30/20 starter template, an "Add budget" button, a Formulas reveal
// toggle, and the smart-insights action. It writes the shared method setting + the
// Formulas atom so the summary/list/formula tiles react in step.
func budgetToolbarWidget(props budgetToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	formulasAtom := uistate.UseBudgetsShowFormulas()
	smartSettings := uistate.LoadSmartSettings()
	activeMemberID := uistate.UseActiveMember().Get()

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	method := budgeting.ParseMethodology(app.Settings().BudgetMethodology)

	// Whether any budget is visible in the current scope — the add button shows only
	// then (the empty-state CTA owns the first add otherwise).
	hasBudgets := false
	for _, b := range app.Budgets() {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			hasBudgets = true
			break
		}
	}

	// Open the add-budget modal (G4: discoverable add).
	addBudget := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("budget") }))
	// Open the "Auto budget" review modal (suggests budgets from spending history).
	autoBudgetAtom := uistate.UseBudgetAutoOpen()
	openAutoBudget := ui.UseEvent(Prevent(func() { autoBudgetAtom.Set(true) }))
	// C112: switch the budgeting methodology right from /budgets.
	onMethod := ui.UseEvent(func(e ui.Event) {
		s := app.Settings()
		s.BudgetMethodology = e.GetValue()
		_ = app.PutSettings(s)
		uistate.BumpDataRevision()
	})
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	// C114: one-click 50/30/20 starter template over last full month's income.
	// v1.0: this is a bulk mutation (up to ~10 new budgets), so it previews the
	// count and confirms before creating anything.
	apply503020 := ui.UseEvent(Prevent(func() {
		txns := app.Transactions()
		rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
		now := time.Now()
		curStart := dateutil.MonthStart(now)
		prevStart := dateutil.AddMonths(curStart, -1)
		configuredIncome := uistate.CurrentPrefs().MonthlyIncomeMinor
		income := budgeting.IncomeForBudgets(configuredIncome, txns, prevStart, curStart, base, rates)
		if income <= 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNoIncome"), true)
			return
		}
		res := budgeting.Generate5030(income, app.Categories(), txns, now)
		existing := map[string]bool{}
		for _, b := range app.Budgets() {
			existing[b.CategoryID] = true
		}
		var toAdd []domain.Budget
		for _, prop := range res.Proposals {
			if prop.LimitMinor <= 0 || existing[prop.Category.ID] {
				continue
			}
			toAdd = append(toAdd, domain.Budget{
				ID: id.New(), Name: prop.Category.Name, CategoryID: prop.Category.ID,
				Scope: domain.ScopeShared, OwnerID: domain.GroupOwnerID,
				Period: domain.PeriodMonthly, Limit: money.New(prop.LimitMinor, base),
			})
		}
		if len(toAdd) == 0 {
			uistate.PostNotice(uistate.T("budgets.tmplNothingToAdd"), false)
			return
		}
		uistate.ConfirmModalLabeled(uistate.T("budgets.tmplConfirm", plural(len(toAdd), "budget")), uistate.T("budgets.tmplConfirmBtn"), false, func(ok bool) {
			if !ok {
				return
			}
			n := 0
			for _, nb := range toAdd {
				if err := app.PutBudget(nb); err == nil {
					n++
				}
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("budgets.tmplApplied", plural(n, "budget")), false)
		})
	}))

	formulasLabel := uistate.T("budgets.showFormulas")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("budgets.hideFormulas")
	}

	toolbar := Div(css.Class("budgets-toolbar"),
		// Left: a compact, labelled methodology picker — no longer a full-width bar that
		// read like a search box (C112: standard / zero-based / envelope).
		Div(css.Class("budgets-toolbar-method"),
			Span(css.Class("budgets-toolbar-label"), uistate.T("settings.budgetMethod")),
			Select(css.Class("field", "budgets-method-select"), Attr("data-testid", "budgets-method"),
				Attr("aria-label", uistate.T("settings.budgetMethod")), Title(uistate.T("settings.budgetMethod")), OnChange(onMethod),
				Option(Value(string(budgeting.MethodSimple)), SelectedIf(method == budgeting.MethodSimple), uistate.T("settings.budgetMethodSimple")),
				Option(Value(string(budgeting.MethodZeroBased)), SelectedIf(method == budgeting.MethodZeroBased), uistate.T("settings.budgetMethodZero")),
				Option(Value(string(budgeting.MethodEnvelope)), SelectedIf(method == budgeting.MethodEnvelope), uistate.T("settings.budgetMethodEnvelope")),
			),
		),
		// Right: the actions, uniform-height and right-aligned, with the primary
		// "+ Add budget" last so it clearly outranks the ghost controls.
		Div(css.Class("budgets-toolbar-actions"),
			smartSectionAction(smartSettings),
			Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "budgets-autobudget"),
				Title(uistate.T("budgets.autoTitleAction")), OnClick(openAutoBudget),
				uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("budgets.autoTitle"))),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "budgets-template-503020"),
				Title(uistate.T("budgets.tmplTitle")), OnClick(apply503020), uistate.T("budgets.tmpl503020")),
			Button(css.Class("btn"), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "budgets-toggle-formulas"), Title(uistate.T("budgets.formulaTitle")),
				OnClick(onToggleFormulas), Text(formulasLabel)),
			If(hasBudgets, Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
				Attr("data-testid", "budgets-add"), Title(uistate.T("budgets.add")), OnClick(addBudget),
				uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("budgets.addBudget")))),
		),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- budget-list -----------------------------------------------------------------

// budgetListWidget is the rows tile: the health-sorted BudgetRow list inside an
// EntityListSection (with the smart section action), or the first-run empty-state CTA.
// It owns the per-row callbacks, the "Cover…" funding sources, and the drill-to-
// transactions navigation.
func budgetListWidget(props budgetListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	v := computeBudgetView(app, activeMemberID, vw, pr)
	smartSettings := uistate.LoadSmartSettings()

	// Drill from a budget to its spending: open Transactions filtered to the budget's
	// category (mirrors Accounts→Transactions, C30/C50).
	viewTransactions := func(categoryID string) {
		f := uistate.TxFilter{Category: categoryID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}

	var body ui.Node
	if len(v.Statuses) == 0 {
		body = ui.CreateElement(EmptyStateCTA, emptyCTAProps{Message: uistate.T("budgets.empty"), CTALabel: uistate.T("budgets.addFirst"), AddTarget: "budget", Icon: icon.Budgets})
	} else {
		cbs := buildBudgetRowCallbacks(app, v.Base, v.CatName)
		budgetDefs := app.CustomFieldDefsFor("budget")
		members := app.Members()
		rows := MapKeyed(v.Statuses,
			func(s budgeting.Status) any { return s.Budget.ID },
			func(s budgeting.Status) ui.Node {
				return ui.CreateElement(BudgetRow, budgetRowProps{
					Status: s, Category: v.CatName[s.Budget.CategoryID], Members: members, BudgetDefs: budgetDefs,
					Envelope: v.EnvAvail[s.Budget.ID], EnvelopeNeg: v.EnvNeg[s.Budget.ID], PaceOver: v.PaceOver[s.Budget.ID],
					RolloverCarry: v.RollCarry[s.Budget.ID], RolloverNeg: v.RollNeg[s.Budget.ID], EffectiveCap: v.RollEffCap[s.Budget.ID],
					ProratedRest: v.ProratedRest[s.Budget.ID], EffectiveMethod: v.EffMethod[s.Budget.ID],
					Covered:  v.Covered[s.Budget.ID],
					OnDelete: cbs.OnDelete, OnRemoveRecurring: cbs.OnRemoveRecurring, OnDrill: viewTransactions,
				})
			},
		)
		// Lay the budget cards out in a responsive grid so each is a compact 1-column
		// block (several per row) rather than a full-width bar — budgets don't need the
		// whole width, and a grid shows far more at a glance.
		body = Div(css.Class("budget-grid"), rows)
	}

	section := uiw.EntityListSection(uiw.EntityListSectionProps{
		Title:        uistate.T("nav.budgets"),
		HeaderAction: smartSectionAction(smartSettings),
		Body:         body,
	})
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: section,
	})
}

// --- budget-formula --------------------------------------------------------------

// budgetFormulaWidget is the opt-in "Budget metrics" tile (revealed by the toolbar's
// Formulas toggle). It embeds the reusable FormulaBuilder, which evaluates against the
// live engine variable surface — so it ties custom fields and formulas together for
// budgets: every number-typed budget custom field surfaces as a cf_budget_<key>
// variable (alongside budget count + the household aggregates), ready to compute over.
func budgetFormulaWidget(props budgetFormulaProps) ui.Node {
	body := Div(
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("budgets.formulaTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "budget-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
