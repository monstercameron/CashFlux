// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"math"
	"sort"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/engineenv"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payoff"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// debtOwnerLink renders a section's "manage this on its own page" link — a quiet anchor to
// the screen that owns the section's data (accounts, net worth, allocate, planning), so a
// user can jump from a read-only debt view to where they edit the underlying records.
func debtOwnerLink(route, label string) ui.Node {
	return A(css.Class("debt-owner-link"), Href(uistate.RoutePath(route)),
		Attr("data-testid", "debt-link-"+route),
		Span(label),
		uiw.Icon(icon.ChevronRight, css.Class(tw.ShrinkO, tw.W3, tw.H3)),
	)
}

// debtSection wraps a debt tile's body with a serif section title (and an optional
// owning-page link on the right) instead of the boxy EntityListSection card — the tile's
// own .w frame is the container, so the panels inside (which build their own grouping cards)
// don't sit in a redundant double frame. The id is the scroll anchor the jump-nav targets.
func debtSection(id, title string, action, body ui.Node) ui.Node {
	args := []any{css.Class("debt-section")}
	if id != "" {
		args = append(args, Attr("id", id))
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

// smoothScrollToSection immediately smooth-scrolls the section with the given id to the top
// of the viewport. Unlike the shared scrollToID (insights.go), it has no 400ms delay and no
// highlight flash — a jump-nav click should move the moment it's pressed and land the
// section heading at the top (scroll-margin-top gives it room under the sticky topbar).
func smoothScrollToSection(id string) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		return
	}
	el := doc.Call("getElementById", id)
	if !el.Truthy() {
		return
	}
	el.Call("scrollIntoView", js.ValueOf(map[string]any{"behavior": "smooth", "block": "start"}))
}

// debtJumpItem is one jump-nav link. Its own component so the per-link OnClick hook sits at
// a stable position when the parent maps over a variable-length section list.
type debtJumpProps struct {
	Label    string
	TargetID string
}

func debtJumpItem(props debtJumpProps) ui.Node {
	onClick := ui.UseEvent(Prevent(func() { smoothScrollToSection(props.TargetID) }))
	return Button(css.Class("debt-jump-link"), Type("button"),
		Attr("data-testid", "debt-jump-"+props.TargetID), OnClick(onClick), props.Label)
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
	TotalOwed money.Money // every non-archived liability (base)
	PlanOwed  money.Money // only the debts included in the payoff plan (base)
	DebtFree  string      // projection date ("January 2026"), or ""
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

// debtViewCache memoizes computeDebtView. Keyed on the store revision AND the debt
// config (payoff strategy/extra/thresholds), which lives in uistate and can change
// without a store mutation.
var debtViewCache = map[string]debtView{}

// computeDebtView returns the shared debt model, memoized so the debt surface's tiles
// don't each re-aggregate the ledger (and re-run the payoff engine) per render.
func computeDebtView(app *appstate.App) debtView {
	key := revKey(app) + "|" + fmt.Sprintf("%v", uistate.DebtConfigGet())
	return memoByRev(debtViewCache, key, func() debtView { return computeDebtViewRaw(app) })
}

// computeDebtViewRaw builds the shared model over the live store. Pure (no hooks) so any tile
// can call it after subscribing to the data revision.
func computeDebtViewRaw(app *appstate.App) debtView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	cfg := uistate.DebtConfigGet()
	vars := liveEngineVars(app)
	txns := app.Transactions()

	// Map each debt account to its engine variable prefix (debt_<slug>_) so the card figures
	// come from the same formula surface the "Debt metrics" builder computes over — the
	// widgets read the engine, not a parallel inline calc.
	prefixByID := map[string]string{}
	for _, dvb := range engineenv.DebtVarBases(app.Accounts()) {
		prefixByID[dvb.Account.ID] = dvb.Prefix
	}

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
			// Utilization comes from the engine's debt_<slug>_utilization variable (a
			// currency-independent %, so it matches the owed/limit ratio exactly) — the meter
			// is formula-driven. Fall back to the direct ratio if the account has no slug.
			if pfx, ok := prefixByID[ac.ID]; ok {
				util = vars[pfx+"utilization"]
			} else {
				util = float64(owed.Amount) / float64(ac.CreditLimit.Amount) * 100
			}
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

	// Payoff order = the strategy's ATTACK order (avalanche = highest APR first,
	// snowball = smallest balance first), so the ladder answers "pay this first"
	// honestly — NOT the order debts happen to clear (a tiny low-rate debt clears
	// first on its minimum alone, which made an avalanche ladder look unsorted).
	// AggregateDebts already filters to the debts the user includes in the plan, so
	// their sum is the plan's scope (distinct from TotalOwed, which counts every
	// liability including an excluded mortgage).
	debts, _ := payoff.AggregateDebts(app.Accounts(), txns, base, rates)
	var includedOwed int64
	for _, d := range debts {
		includedOwed += d.Balance
	}
	v.PlanOwed = money.New(includedOwed, base)
	rankByName := map[string]int{}
	for i, name := range payoff.FocusOrder(debts, strategyFromConfig(cfg)) {
		if _, seen := rankByName[name]; !seen {
			rankByName[name] = i + 1
		}
	}
	for i := range v.Liabs {
		if r, ok := rankByName[v.Liabs[i].Name]; ok {
			v.RankByID[v.Liabs[i].ID] = r
		}
	}
	if plan, ok := payoff.BuildPlan(debts, cfg.DefaultExtraMinor, strategyFromConfig(cfg)); ok && plan.Months >= 0 && len(debts) > 0 {
		v.DebtFree = payoff.DebtFreeMonth(time.Now(), plan.Months).Format("January 2006")
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

	// The debt-free date is the plan's date, and the plan covers only the included
	// debts. When something is excluded (a mortgage, by default), pairing the full
	// total with that date reads as "all of it clears by then" — so say the scope
	// out loud: what the plan clears, and how much sits outside it.
	var freeLine ui.Node = Fragment()
	if v.DebtFree != "" {
		excluded := v.TotalOwed.Amount - v.PlanOwed.Amount
		if excluded > 0 {
			freeLine = Fragment(
				P(css.Class("debt-hero-sub", tw.FontDisplay), Attr("data-testid", "debt-plan-scope"),
					uistate.T("debt.planClearsBy", fmtMoney(v.PlanOwed), v.DebtFree)),
				P(css.Class("debt-hero-note", tw.TextDim),
					uistate.T("debt.planExcludes", fmtMoney(money.New(excluded, v.Base)))),
			)
		} else {
			freeLine = P(css.Class("debt-hero-sub", tw.FontDisplay), Attr("data-testid", "debt-plan-scope"),
				uistate.T("debt.debtFreeBy", v.DebtFree))
		}
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

	body := Div(css.Class("debt-hero"), Attr("id", "sec-overview"),
		Div(css.Class("debt-hero-main"),
			Div(css.Class("debt-hero-label", tw.TextDim), uistate.T("debt.totalOwed")),
			Div(css.Class("debt-hero-value neg", tw.FontDisplay), Attr("data-testid", "debt-total-owed"), fmtMoney(v.TotalOwed)),
			freeLine,
			// The Net worth page owns the assets-vs-liabilities picture the debt-to-asset
			// chip summarizes — link there.
			debtOwnerLink("/networth", uistate.T("debt.linkNetWorth")),
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

	// Jump-nav: a row of section links so a user can skip straight to the widget they want
	// (only the sections that actually render are listed). Each link scrolls its anchor into
	// view. Built as a variable-length list, so each link is its own hook-owning component.
	var hasCC, hasLoans, hasDebts bool
	for _, ac := range props.App.Accounts() {
		if ac.Archived || ac.Class != domain.ClassLiability {
			continue
		}
		hasDebts = true
		if ac.Type == domain.TypeCreditCard {
			hasCC = true
		}
		if isInstallmentLoan(ac.Type) {
			hasLoans = true
		}
	}
	jumps := []debtJumpProps{{uistate.T("debt.jumpOverview"), "sec-overview"}}
	if hasDebts {
		jumps = append(jumps,
			debtJumpProps{uistate.T("debt.jumpWatchouts"), "sec-watchouts"},
			debtJumpProps{uistate.T("debt.jumpLadder"), "sec-ladder"},
			debtJumpProps{uistate.T("debt.jumpTune"), "sec-tuner"},
			debtJumpProps{uistate.T("debt.jumpStrategy"), "sec-strategy"})
		if hasCC {
			jumps = append(jumps, debtJumpProps{uistate.T("debt.jumpCredit"), "sec-credit"})
		}
		if hasLoans {
			jumps = append(jumps, debtJumpProps{uistate.T("debt.jumpLoans"), "sec-loans"})
		}
		jumps = append(jumps, debtJumpProps{uistate.T("debt.jumpCalculator"), "sec-calculator"})
	}
	jumps = append(jumps, debtJumpProps{uistate.T("debt.jumpLearn"), "sec-learn"})
	var jumpNav ui.Node = Fragment()
	if len(jumps) > 1 {
		jumpNav = Div(css.Class("debt-jump"), Attr("aria-label", uistate.T("debt.jumpTo")),
			Span(css.Class("debt-jump-label"), uistate.T("debt.jumpTo")),
			MapKeyed(jumps, func(j debtJumpProps) any { return j.TargetID },
				func(j debtJumpProps) ui.Node { return ui.CreateElement(debtJumpItem, j) }),
		)
	}

	toolbar := Div(css.Class("filter-strip"),
		Div(css.Class("filter-strip-controls"),
			Button(css.Class(metricsCls), Type("button"), Attr("aria-pressed", ariaBool(formulasAtom.Get())),
				Attr("data-testid", "debt-toggle-formulas"), Title(uistate.T("debt.metricsTitle")),
				OnClick(onToggleFormulas), Text(formulasLabel)),
			A(css.Class("btn btn-tool"), Href(uistate.RoutePath("/accounts")),
				uiw.Icon(icon.Landmark, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
				Span(uistate.T("debt.manageAccounts")),
				Span(css.Class("bt-kind"), Attr("aria-hidden", "true"), "↗")),
		),
		Button(css.Class("btn btn-primary btn-tool", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("data-testid", "debt-add"), Title(uistate.T("debt.addDebtTitle")), OnClick(addDebt),
			uiw.Icon(icon.Plus, css.Class(tw.ShrinkO, tw.W4, tw.H4)),
			Span(uistate.T("debt.addDebt"))),
	)
	return uiw.Widget(uiw.WidgetProps{
		ID: "debt-toolbar", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
		Body: Fragment(jumpNav, toolbar),
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
		body := debtSection("sec-ladder", uistate.T("debt.whatYouOwe"),
			debtOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
			P(css.Class("empty"), Attr("data-testid", "debt-empty"), uistate.T("debt.noDebts")))
		return uiw.Widget(uiw.WidgetProps{
			ID: "debt-list", Title: "", GridColumn: "1 / span 4", Draggable: false, Resizable: false, Preview: true,
			Body: body,
		})
	}

	txns := app.Transactions()
	// viewBills drills to the transactions the user marked as bill payments toward a
	// liability — the "proof of payment" linkage shown on each debt card.
	onViewBills := func(accountID string) {
		f := uistate.TxFilter{BillAccount: accountID}.Normalize()
		txFilter.Set(f)
		uistate.PersistTxFilter(f)
		nav.Navigate(uistate.RoutePath("/transactions"))
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
			BillPayment: ledger.BillPaymentForAccount(ac.ID, txns), OnViewBills: onViewBills,
		})
	})

	body := debtSection("sec-ladder", uistate.T("debt.payoffLadder"),
		debtOwnerLink("/accounts", uistate.T("debt.linkAccounts")),
		Div(css.Class("debt-list"), rows))
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
		Body: Div(Attr("id", "sec-strategy"), ui.CreateElement(DebtStrategyPanel, DebtStrategyPanelProps{HideExtraInput: true})),
	})
}

// debtCreditWidget hosts the credit-card health panel (shown only when a card exists).
func debtCreditWidget(props debtPanelProps) ui.Node {
	body := debtSection("sec-credit", uistate.T("nav.credit"), debtOwnerLink("/accounts", uistate.T("debt.linkCards")), Fragment(
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
	body := debtSection("sec-loans", uistate.T("nav.loans"), debtOwnerLink("/accounts", uistate.T("debt.linkLoans")), Fragment(
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
		Body: Div(Attr("id", "sec-calculator"), ui.CreateElement(PayoffCalculatorPanel, PayoffCalculatorPanelProps{})),
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
