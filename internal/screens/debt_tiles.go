// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// debtSection wraps a debt tile's body with a serif section title instead of the boxy
// EntityListSection card — the tile's own .w frame is the container, so the panels inside
// (which build their own grouping cards) don't sit in a redundant double frame.
func debtSection(title string, body ui.Node) ui.Node {
	return Div(css.Class("debt-section"),
		If(title != "", H2(css.Class("debt-section-title"), title)),
		body,
	)
}

type debtSummaryProps struct{ App *appstate.App }
type debtToolbarProps struct{ App *appstate.App }
type debtListProps struct{ App *appstate.App }
type debtPanelProps struct{ App *appstate.App }

// --- shared debt view -----------------------------------------------------------

// debtView is the derived render model the debt tiles share: the liability accounts in
// payoff order, each debt's owed/utilization/rank, the engine variable surface, and the
// persisted DebtConfig. Every figure is either read from the engine (Vars) or banded by
// the config — nothing the tiles render is a hardcoded threshold or inline computation.
type debtView struct {
	Base      string
	Cfg       uistate.DebtConfig
	Vars      map[string]float64
	Liabs     []domain.Account
	OwedByID  map[string]money.Money
	UtilByID  map[string]float64 // -1 = not a line of credit
	AvailByID map[string]money.Money
	RankByID  map[string]int
	InPayByID map[string]bool
	Defs      []customfields.Def
	TotalOwed money.Money
	DebtFree  string // projection date ("January 2026"), or ""
	HasCC     bool
	HasLoans  bool
}

// strategyFromConfig maps the config's default-strategy string to a payoff.Strategy.
func strategyFromConfig(cfg uistate.DebtConfig) payoff.Strategy {
	if cfg.DefaultStrategy == "snowball" {
		return payoff.Snowball
	}
	return payoff.Avalanche
}

// majorMoney converts a base-currency major-unit engine value into a money.Money, rounding
// to the currency's minor unit. Lets a tile render an engine figure (e.g. the "liabilities"
// atom) as currency without re-summing the ledger.
func majorMoney(v float64, base string) money.Money {
	dec := currency.Decimals(base)
	mult := 1.0
	for i := 0; i < dec; i++ {
		mult *= 10
	}
	return money.New(int64(math.Round(v*mult)), base)
}

// computeDebtView builds the shared model over the live store. Pure (no hooks) so any tile
// can call it after subscribing to the data revision.
func computeDebtView(app *appstate.App) debtView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	cfg := uistate.DebtConfigGet()
	vars := liveEngineVars(app)
	txns := app.Transactions()

	v := debtView{
		Base: base, Cfg: cfg, Vars: vars,
		OwedByID: map[string]money.Money{}, UtilByID: map[string]float64{},
		AvailByID: map[string]money.Money{}, RankByID: map[string]int{}, InPayByID: map[string]bool{},
		Defs:      app.CustomFieldDefsFor("account"),
		TotalOwed: majorMoney(vars["liabilities"], base),
	}

	for _, ac := range app.Accounts() {
		if ac.Archived || ac.Class != domain.ClassLiability {
			continue
		}
		bal, _ := ledger.Balance(ac, txns)
		owed := bal.Abs()
		v.OwedByID[ac.ID] = owed
		v.InPayByID[ac.ID] = ac.IncludedInPayoff()
		util := -1.0
		if ac.Type == domain.TypeCreditCard && ac.CreditLimit.Amount > 0 {
			util = float64(owed.Amount) / float64(ac.CreditLimit.Amount) * 100
			avail := ac.CreditLimit.Amount - owed.Amount
			if avail < 0 {
				avail = 0
			}
			v.AvailByID[ac.ID] = money.New(avail, ac.Currency)
			v.HasCC = true
		}
		v.UtilByID[ac.ID] = util
		if isInstallmentLoan(ac.Type) {
			v.HasLoans = true
		}
		v.Liabs = append(v.Liabs, ac)
	}

	// Payoff order (avalanche/snowball per config) → per-debt rank. Built from the same
	// AggregateDebts the strategy panel uses, so the ladder matches the plan.
	debts, _ := payoff.AggregateDebts(app.Accounts(), txns, base, rates)
	if plan, ok := payoff.BuildPlan(debts, cfg.DefaultExtraMinor, strategyFromConfig(cfg)); ok {
		rankByName := map[string]int{}
		for i, name := range plan.Order {
			if _, seen := rankByName[name]; !seen {
				rankByName[name] = i + 1
			}
		}
		for i := range v.Liabs {
			if r, ok := rankByName[v.Liabs[i].Name]; ok {
				v.RankByID[v.Liabs[i].ID] = r
			}
		}
		if plan.Months >= 0 && len(debts) > 0 {
			v.DebtFree = payoff.DebtFreeMonth(time.Now(), plan.Months).Format("January 2006")
		}
	}

	// Order the ladder: in-plan debts by ascending payoff rank, then everything else by
	// amount owed descending (heaviest first) — the same convention as /accounts.
	sort.SliceStable(v.Liabs, func(i, j int) bool {
		ri, rj := v.RankByID[v.Liabs[i].ID], v.RankByID[v.Liabs[j].ID]
		switch {
		case ri > 0 && rj > 0:
			return ri < rj
		case ri > 0:
			return true
		case rj > 0:
			return false
		default:
			return v.OwedByID[v.Liabs[i].ID].Amount > v.OwedByID[v.Liabs[j].ID].Amount
		}
	})
	return v
}

// --- debt-summary ---------------------------------------------------------------

// debtSummaryWidget is the headline tile: the total owed in the display serif, the
// projected debt-free date, and engine-computed ratio chips (credit utilization,
// minimum payments per month, debt-to-asset, debt count). Renders nothing when there are
// no debts (the list tile owns the celebratory empty state).
func debtSummaryWidget(props debtSummaryProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	v := computeDebtView(props.App)
	if len(v.Liabs) == 0 {
		return Fragment()
	}

	var freeLine ui.Node = Fragment()
	if v.DebtFree != "" {
		freeLine = P(css.Class("debt-hero-sub", tw.FontDisplay), uistate.T("debt.debtFreeBy", v.DebtFree))
	}

	// Ratio chips read straight off the engine surface (credit_utilization &
	// debt_to_asset_pct are formula molecules; min_payments_total & debt_count are atoms).
	util := v.Vars["credit_utilization"]
	utilBand := v.Cfg.UtilizationBand(util)
	chips := Div(css.Class("debt-chips"),
		If(v.Vars["credit_limit_total"] > 0,
			Div(ClassStr("debt-stat debt-band-"+utilBand),
				Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("debt.creditUtilization")),
				Div(css.Class("debt-stat-value", tw.FontDisplay), fmt.Sprintf("%.0f%%", util)))),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("debt.minPaymentsMonthly")),
			Div(css.Class("debt-stat-value", tw.FontDisplay), fmtMoney(majorMoney(v.Vars["min_payments_total"], v.Base)))),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("debt.debtToAsset")),
			Div(css.Class("debt-stat-value", tw.FontDisplay), fmt.Sprintf("%.0f%%", v.Vars["debt_to_asset_pct"]))),
		Div(css.Class("debt-stat"),
			Div(css.Class("debt-stat-label", tw.TextDim), uistate.T("debt.debtsCount")),
			Div(css.Class("debt-stat-value", tw.FontDisplay), fmt.Sprintf("%.0f", v.Vars["debt_count"]))),
	)

	body := Div(css.Class("debt-hero"),
		Div(css.Class("debt-hero-main"),
			Div(css.Class("debt-hero-label", tw.TextDim), uistate.T("debt.totalOwed")),
			Div(css.Class("debt-hero-value neg", tw.FontDisplay), Attr("data-testid", "debt-total-owed"), fmtMoney(v.TotalOwed)),
			freeLine,
		),
		chips,
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-summary", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- debt-toolbar ---------------------------------------------------------------

// debtToolbarWidget is the actions row: a "Debt metrics" FormulaBuilder reveal toggle
// (parity with budgets/goals), a link to manage the underlying accounts, and the primary
// "Add debt" action.
func debtToolbarWidget(props debtToolbarProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	formulasAtom := uistate.UseDebtShowFormulas()
	onToggleFormulas := ui.UseEvent(Prevent(func() { formulasAtom.Set(!formulasAtom.Get()) }))
	addDebt := ui.UseEvent(Prevent(func() { uistate.SetAddTarget("account") }))

	formulasLabel := uistate.T("debt.metricsShow")
	if formulasAtom.Get() {
		formulasLabel = uistate.T("debt.metricsHide")
	}
	metricsCls := "strip-toggle"
	if formulasAtom.Get() {
		metricsCls += " is-on"
	}

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "debt-toggle-formulas"), Title(uistate.T("debt.metricsTitle")),
				OnClick(onToggleFormulas), Text(formulasLabel)),
			A(css.Class("btn btn-ghost"), Href(uistate.RoutePath("/accounts")), uistate.T("debt.manageAccounts")),
		),
		Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "debt-add"), Title(uistate.T("debt.addDebtTitle")), OnClick(addDebt),
			uiw.Icon(icon.PlusCircle, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("debt.addDebt"))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: toolbar,
	})
}

// --- debt-list ------------------------------------------------------------------

// debtListWidget is the payoff-ladder tile: the liability DebtRow cards in payoff order,
// or the celebratory "no debts" empty state. It owns the per-row callbacks (view / edit /
// toggle-in-plan).
func debtListWidget(props debtListProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := props.App
	nav := router.UseNavigate()
	txFilter := uistate.UseTxFilter()
	acctEditAtom := uistate.UseAccountEdit()
	v := computeDebtView(app)

	if len(v.Liabs) == 0 {
		body := debtSection(uistate.T("debt.whatYouOwe"),
			P(css.Class("empty"), Attr("data-testid", "debt-empty"), uistate.T("debt.noDebts")))
		return uiw.Widget(uiw.WidgetProps{
			ID: "debt-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: body,
		})
	}

	onView := func(accountID string) {
		f := uistate.TxFilter{Account: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
	}
	onEdit := func(accountID string) {
		acctEditAtom.Set(uistate.AccountEdit{ID: accountID, Mode: uistate.AcctEditModeEdit})
	}
	onTogglePay := func(ac domain.Account, include bool) {
		next := ac
		val := include
		next.IncludeInPayoff = &val
		if err := app.PutAccount(next); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		uistate.BumpDataRevision()
	}

	rows := MapKeyed(v.Liabs, func(ac domain.Account) any { return ac.ID }, func(ac domain.Account) ui.Node {
		avail := v.AvailByID[ac.ID]
		return ui.CreateElement(DebtRow, debtRowProps{
			Account: ac, Owed: v.OwedByID[ac.ID], Rank: v.RankByID[ac.ID],
			Utilization: v.UtilByID[ac.ID], Available: avail,
			Band:     v.Cfg.UtilizationBand(v.UtilByID[ac.ID]),
			InPayoff: v.InPayByID[ac.ID], Defs: v.Defs,
			OnEdit: onEdit, OnView: onView, OnTogglePay: onTogglePay,
		})
	})

	body := debtSection(uistate.T("debt.payoffLadder"), Div(css.Class("debt-list"), rows))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// --- panel-wrapping tiles -------------------------------------------------------

// debtStrategyWidget hosts the shared snowball-vs-avalanche planner as a surface tile.
func debtStrategyWidget(props debtPanelProps) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-strategy", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: ui.CreateElement(DebtStrategyPanel, DebtStrategyPanelProps{}),
	})
}

// debtCreditWidget hosts the credit-card health panel (shown only when a card exists).
func debtCreditWidget(props debtPanelProps) ui.Node {
	body := debtSection(uistate.T("nav.credit"), Fragment(
		P(css.Class("muted"), uistate.T("screen.creditSub")),
		ui.CreateElement(CreditHealthPanel, CreditHealthPanelProps{}),
	))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-credit", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// debtLoansWidget hosts the installment-loans panel (shown only when a loan exists).
func debtLoansWidget(props debtPanelProps) ui.Node {
	body := debtSection(uistate.T("nav.loans"), Fragment(
		P(css.Class("muted"), uistate.T("screen.loansSub")),
		ui.CreateElement(LoansPanel, LoansPanelProps{}),
	))
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-loans", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}

// debtPayoffWidget hosts the manual single-debt payoff what-if calculator.
func debtPayoffWidget(props debtPanelProps) ui.Node {
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-payoff", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: ui.CreateElement(PayoffCalculatorPanel, PayoffCalculatorPanelProps{}),
	})
}

// debtFormulaWidget is the opt-in "Debt metrics" tile (revealed by the toolbar toggle):
// the reusable FormulaBuilder over the live engine surface, so the debt_* variables (owed
// / APR / utilization / min payment), the debt aggregate atoms, and account custom fields
// (cf_acct_<key>) can all be computed over.
func debtFormulaWidget(props debtPanelProps) ui.Node {
	body := Div(
		P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("debt.formulaHint")),
		ui.CreateElement(FormulaBuilder, FormulaBuilderProps{Title: uistate.T("debt.metricsTitle"), ShowSaved: true}),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-formula", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: body,
	})
}
