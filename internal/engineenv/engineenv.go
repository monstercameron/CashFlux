// SPDX-License-Identifier: MIT

// Package engineenv builds the "app engine variable surface": the named numeric
// figures a sandboxed formula can reference. It is built compositionally from two
// layers so every figure is auditable:
//
//   - ATOMS are indivisible: a pure reduction over the fundamental data
//     (transactions, accounts, bills/recurring, goals) — e.g. "assets" is the sum
//     of FX-converted asset-account balances. An atom can't be expressed as a
//     formula over other variables; it's a leaf, computed in Go.
//   - MOLECULES are compound: defined as a FORMULA over atoms (and earlier
//     molecules) — e.g. net_worth = "assets - liabilities". Their derivation is
//     data, not code, so a figure can be traced down to its atoms (see Explain).
//     Built-ins are seeded by DefaultMolecules; overrides/additions are passed in
//     via Data.Molecules (persisted in the dataset).
//
// Pure Go, no syscall/js — unit-tested natively. The wasm layer gathers the Data
// (pre-scoped, with the active period window and the persisted molecule set).
package engineenv

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/billsched"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/formula"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/planning"
	"github.com/monstercameron/CashFlux/internal/safespend"
)

// Data is everything Vars needs. It is the raw dataset slices (pre-scoped by the
// caller) plus the FX rate table, the reference time, the active period window,
// the custom-field definitions, and the molecule set (empty → DefaultMolecules).
type Data struct {
	Accounts     []domain.Account
	Transactions []domain.Transaction
	Members      []domain.Member
	Budgets      []domain.Budget
	Goals        []domain.Goal
	Tasks        []domain.Task
	Recurring    []domain.Recurring
	// Categories are the household's category tree — needed by variables that
	// evaluate budgets with sub-category rollup (the health_* factor atoms).
	Categories  []domain.Category
	Rates       currency.Rates
	Now         time.Time
	PeriodStart time.Time
	PeriodEnd   time.Time
	WeekStart   time.Weekday // week anchor for per-budget period windows (default Sunday)
	CustomDefs  []customfields.Def
	Molecules   []domain.Molecule
	// Pools are user-defined groups of accounts (see the investments page); each becomes a
	// pool_<slug>_value engine variable so a group's combined value is usable in formulas.
	Pools []PoolDef
	// Alloc holds the persisted allocate-page plan (amount to put to work + split controls),
	// surfaced as the alloc_* engine variables so a plan figure is usable in formulas/widgets.
	Alloc AllocData
	// Plans are the user's saved what-if scenarios (the planning page); each becomes
	// plan_<slug>_end / _monthly / _runway engine variables so a scenario's projection is
	// usable in a formula or dashboard widget.
	Plans []domain.Plan
	// Planning holds the persisted planning-page policy (runway buffer/horizon, forecast
	// horizon), surfaced as the runway_* / forecast_horizon variables.
	Planning PlanningData
	// BillsSmart holds the smart-bill-schedule inputs (paydays + expected income per
	// payday + the keep floor, from prefs + BillsSmartConfig), surfaced as the
	// bills_* schedule variables via the billsched optimizer.
	BillsSmart BillsSmartData
	// Smart holds the Smart layer's enabled-feature counts (from the persisted
	// Smart settings), surfaced as the smart_* variables.
	Smart SmartCounts
}

// BillsSmartData is the smart-schedule input the wasm layer feeds in. Empty
// Paydays means "pay cycle not configured" — the variables still exist but the
// smart figures equal the raw ones.
type BillsSmartData struct {
	Paydays         []time.Time
	IncomePerPayday int64
	MinKeepMinor    int64
}

// PlanningData is the persisted planning policy the wasm layer feeds in (from PlanningConfig), so
// the pure engine can expose it as variables. Amounts are minor units of the base currency.
type PlanningData struct {
	RunwayBufferMinor int64
	RunwayDays        int
	ForecastMonths    int
}

// AllocData is the allocate-page plan the wasm layer feeds in (from the persisted AllocConfig),
// so the pure engine can expose the plan as variables. Amounts are minor units of the base
// currency.
type AllocData struct {
	AmountMinor  int64
	ReserveMinor int64
	MaxPerMinor  int64
}

// PoolDef is a named group of account IDs, passed in from the wasm layer (which holds the
// persisted pool config), so the pure engine can expose a pool's combined value.
type PoolDef struct {
	Name       string
	VarName    string
	AccountIDs []string
}

// atomNames lists the indivisible variables computeAtoms produces, in a stable
// order. Each is a reduction over the named fundamental source.
var atomNames = []string{
	"assets",             // Σ FX-converted balances of non-archived asset accounts
	"liabilities",        // Σ magnitudes of non-archived liability-account balances (positive)
	"liquid_cash",        // Σ balances of non-archived cash-type accounts (checking/debit/savings/cash)
	"income",             // Σ positive non-transfer transactions in the period
	"expense",            // Σ |negative non-transfer transactions| in the period (positive)
	"income_count",       // count of income (positive non-transfer) transactions in the period
	"expense_count",      // count of expense (negative non-transfer) transactions in the period
	"bills_due",          // Σ bills due before this calendar month-end
	"goal_needs",         // Σ prorated monthly goal contributions
	"earmarked_total",    // Σ virtual goal earmarks across all goals (reserved-in-place, uncommitted)
	"accounts",           // count of non-archived accounts
	"asset_accounts",     // count of non-archived asset-class accounts
	"liability_accounts", // count of non-archived liability-class accounts
	"debt_count",         // count of non-archived liability accounts (debts)
	"revolving_balance",  // Σ magnitudes of non-archived credit-card balances (positive)
	"credit_limit_total", // Σ credit limits of non-archived credit-card accounts
	"min_payments_total", // Σ minimum monthly payments across non-archived liabilities
	"transactions",       // count of transactions
	"members",            // count of members
	"budgets",            // count of budgets
	"goals",              // count of goals
	"tasks",              // count of tasks
}

// DefaultMolecules are the built-in compound variables, defined as formulas over
// atoms. Editing or extending these (persisted in the dataset) reshapes the
// derived figures without code changes — and keeps them auditable.
func DefaultMolecules() []domain.Molecule {
	return []domain.Molecule{
		{Name: "net_worth", Formula: "assets - liabilities", Doc: "Everything you own minus everything you owe."},
		{Name: "cashflow_net", Formula: "income - expense", Doc: "Net cash flow for the period (income minus spending)."},
		{Name: "savings_rate", Formula: "clamp(safediv(income - expense, income, 0) * 100, -100, 100)", Doc: "Percent of income kept this period."},
		{Name: "safe_to_spend", Formula: "liquid_cash - max(bills_due, 0) - max(goal_needs, 0)", Doc: "Liquid cash after this month's bills and goal set-asides."},
		{Name: "unreserved_cash", Formula: "liquid_cash - max(earmarked_total, 0)", Doc: "Liquid cash not virtually reserved (earmarked) for any goal — what's genuinely unspoken-for."},
		{Name: "credit_utilization", Formula: "clamp(safediv(revolving_balance, credit_limit_total, 0) * 100, 0, 100)", Doc: "Percent of your total credit-card limit you're using (30%+ starts to weigh on a credit score)."},
		{Name: "debt_to_asset_pct", Formula: "clamp(safediv(liabilities, assets, 0) * 100, 0, 1000)", Doc: "What you owe as a percent of what you own — lower is healthier."},
		// The financial-health score IS this formula: each health_* factor is a 0–100
		// atom scored in Go (internal/healthscore), each _weight atom is that factor's
		// exact share after inapplicable factors are dropped and re-normalized, and
		// health_penalty is the flat deficit deduction. Because it's a molecule, the
		// derivation is auditable (Explain) and a household can even re-weight its own
		// score by editing this formula — the /health page reads THIS value.
		{Name: "health_score", Formula: "clamp(round(health_savings*health_savings_weight + health_emergency*health_emergency_weight + health_debt*health_debt_weight + health_budget*health_budget_weight + health_utilization*health_utilization_weight + health_trend*health_trend_weight) - health_penalty, 0, 100)", Doc: "Your 0–100 financial-health score — the weighted blend of the six factor scores, minus a penalty when spending exceeds income."},
		// The credit-health proxy IS this formula, like health_score above: three
		// credit_* factor atoms scored in Go (internal/credithealth) blended by
		// their exact normalized weights. floor (not round) mirrors the model's
		// truncation. The /credit page reads THIS value.
		{Name: "credit_proxy", Formula: "clamp(floor(credit_util_score*credit_util_weight + credit_ontime_score*credit_ontime_weight + credit_age_score*credit_age_weight), 0, 100)", Doc: "Your 0–100 credit-health estimate — utilization, on-time payments, and account age blended by weight. A local proxy, not a FICO score."},
	}
}

// Names lists the built-in surface: atoms followed by the default molecule names.
// The binding editor shows these (plus CustomFieldNames) so a user knows what they
// can reference.
var Names = func() []string {
	out := append([]string(nil), atomNames...)
	for _, m := range DefaultMolecules() {
		out = append(out, m.Name)
	}
	out = append(out, AllocVarNames...)
	out = append(out, PlanningVarNames...)
	out = append(out, RecurringVarNames...)
	out = append(out, BillsSmartVarNames...)
	return out
}()

// Vars computes the full variable surface: the atoms, then each molecule formula
// evaluated over the running map (so a molecule may reference atoms and any
// molecule declared before it). Money figures are major units of the base
// currency. Deterministic for a given Data.
func Vars(d Data) map[string]float64 {
	out := computeAtoms(d)
	mols := d.Molecules
	if len(mols) == 0 {
		mols = DefaultMolecules()
	}
	for _, m := range mols {
		if v, err := formula.Eval(m.Formula, formula.Env{Vars: out}); err == nil {
			if f, ok := v.(float64); ok {
				out[m.Name] = f
			} else if b, ok := v.(bool); ok {
				if b {
					out[m.Name] = 1
				} else {
					out[m.Name] = 0
				}
			}
		}
	}
	return out
}

// computeAtoms reduces the fundamental data to the indivisible atoms.
func computeAtoms(d Data) map[string]float64 {
	base := d.Rates.Base
	if base == "" {
		base = "USD"
	}
	div := 1.0
	for i := 0; i < currency.Decimals(base); i++ {
		div *= 10
	}
	major := func(amount int64) float64 { return float64(amount) / div }

	// Assets/liabilities: explaining variant excludes missing-FX accounts gracefully.
	nw, _ := ledger.NetWorthExplained(d.Accounts, d.Transactions, d.Rates)

	// Income/expense over the active period (falls back to the calendar month).
	start, end := d.PeriodStart, d.PeriodEnd
	if start.IsZero() || end.IsZero() {
		start, end = dateutil.MonthRange(d.Now)
	}
	income, expense, _ := ledger.PeriodTotals(d.Transactions, start, end, d.Rates)

	// Period transaction counts (by sign), for "N deposits / N transactions" labels.
	incCount, expCount := 0, 0
	for _, t := range d.Transactions {
		if !dateutil.InRange(t.Date, start, end) {
			continue
		}
		switch {
		case t.IsIncome():
			incCount++
		case t.IsExpense():
			expCount++
		}
	}

	// Safe-to-spend atoms — fundamental, FX-converted to base. Bills/goals are a
	// this-calendar-month commitment, independent of the dashboard's period.
	liquid, _ := ledger.LiquidBalance(d.Accounts, d.Transactions, d.Rates)
	_, monthEnd := dateutil.MonthRange(d.Now)
	toBase := safespend.ToBaseFunc(d.Rates)
	billsDue := safespend.BillsDueBefore(d.Accounts, d.Recurring, d.Now, monthEnd, toBase)
	goalNeeds := safespend.GoalContributionsProrated(d.Goals, d.Now, toBase)
	// earmarked_total: Σ virtual allocations across all goals (base minor units). Money
	// reserved-in-place for goals but NOT moved — the pool a decision layer can subtract
	// from liquid cash to see what's genuinely unspoken-for.
	var earmarkedTotal int64
	for _, g := range d.Goals {
		for _, al := range g.Allocations {
			earmarkedTotal += toBase(al.Amount.Amount, al.Amount.Currency)
		}
	}

	active, assetAccts, liabAccts, debtCount := 0, 0, 0, 0
	var revolvingBal, creditLimitTotal, minPaymentsTotal int64
	for _, a := range d.Accounts {
		if a.Archived {
			continue
		}
		active++
		if a.Class == domain.ClassLiability {
			liabAccts++
			debtCount++
			minPaymentsTotal += toBase(a.MinPayment.Amount, a.MinPayment.Currency)
			if a.Type == domain.TypeCreditCard {
				creditLimitTotal += toBase(a.CreditLimit.Amount, a.CreditLimit.Currency)
				if bal, err := ledger.Balance(a, d.Transactions); err == nil {
					mag := bal.Amount
					if mag < 0 {
						mag = -mag
					}
					revolvingBal += toBase(mag, bal.Currency)
				}
			}
		} else {
			assetAccts++
		}
	}

	out := map[string]float64{
		"assets":             major(nw.Assets.Amount),
		"liabilities":        major(nw.Liabilities.Amount),
		"liquid_cash":        major(liquid.Amount),
		"income":             major(income.Amount),
		"expense":            major(expense.Amount),
		"income_count":       float64(incCount),
		"expense_count":      float64(expCount),
		"bills_due":          major(billsDue),
		"goal_needs":         major(goalNeeds),
		"earmarked_total":    major(earmarkedTotal),
		"accounts":           float64(active),
		"asset_accounts":     float64(assetAccts),
		"liability_accounts": float64(liabAccts),
		"debt_count":         float64(debtCount),
		"revolving_balance":  major(revolvingBal),
		"credit_limit_total": major(creditLimitTotal),
		"min_payments_total": major(minPaymentsTotal),
		"transactions":       float64(len(d.Transactions)),
		"members":            float64(len(d.Members)),
		"budgets":            float64(len(d.Budgets)),
		"goals":              float64(len(d.Goals)),
		"tasks":              float64(len(d.Tasks)),
	}
	addCustomFieldVars(out, d, start, end)
	addBudgetVars(out, d, major)
	addAccountVars(out, d, major, toBase)
	addGoalVars(out, d, major, toBase)
	addDebtVars(out, d, major, toBase)
	addPoolVars(out, d, major, toBase)
	addAllocVars(out, d, major)
	addPlanningVars(out, d, major)
	addRecurringVars(out, d, major, toBase)
	addBillsSmartVars(out, d, major, toBase)
	addReportsVars(out, d, major)
	addNetWorthVars(out, d, major, toBase)
	addHealthVars(out, d)
	addCreditVars(out, d, major, toBase)
	addAssistantVars(out, d, major)
	addSmartVars(out, d)
	return out
}

// BillsSmartVarNames are the fixed smart-bill-schedule variables addBillsSmartVars
// exposes — the schedule as referenceable figures.
var BillsSmartVarNames = []string{
	"bills_low_raw",          // projected 60-day low under the raw due dates (major units)
	"bills_check_load_raw",   // the heaviest pay period's billed total under raw dates
	"bills_check_load_smart", // …and under the pay-ahead smart schedule
	"bills_even_gain",        // how much lighter the heaviest paycheck gets (raw − smart)
	"bills_paid_ahead",       // payments the plan fronts a pay cycle ahead (CycleAhead moves)
	"bills_suggest_gain",     // the best single biller-side shift's low-point gain
}

// BillsSmartHorizonDays is the smart-schedule planning window: this month plus
// the next, so paying next month's occurrence on one of this month's paydays —
// the whole point of pay-ahead — is representable. Views that page the calendar
// further extend their own horizon; the engine variables use this fixed window.
const BillsSmartHorizonDays = 60

// addBillsSmartVars runs the billsched optimizer over the next 60 days of bill
// OCCURRENCES (every repeat of each liability minimum payment + expense
// recurring in the window, not just each bill's next due) against the
// configured pay cycle, exposing the schedule as engine variables. With no
// paydays configured the raw and smart figures coincide and the gains are 0.
func addBillsSmartVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	const horizonDays = BillsSmartHorizonDays
	occurrences := bills.OccurrencesWithin(d.Accounts, d.Recurring, d.Now, d.Now.AddDate(0, 0, horizonDays))
	items := make([]billsched.Item, 0, len(occurrences))
	for _, b := range occurrences {
		items = append(items, billsched.Item{
			ID:      b.AccountID + "|" + b.DueDate.Format("2006-01-02") + "|" + b.Name,
			Name:    b.Name,
			Amount:  toBase(b.Amount.Amount, b.Amount.Currency),
			Due:     b.DueDate,
			Movable: !b.Autopay,
		})
	}
	liquid, _ := ledger.LiquidBalance(d.Accounts, d.Transactions, d.Rates)
	res := billsched.Optimize(liquid.Amount, items, d.BillsSmart.Paydays, d.BillsSmart.IncomePerPayday, d.Now, horizonDays, d.BillsSmart.MinKeepMinor)

	maxLoad := func(loads []billsched.PeriodLoad) int64 {
		var m int64
		for _, l := range loads {
			if l.Total > m {
				m = l.Total
			}
		}
		return m
	}
	out["bills_low_raw"] = major(res.Raw.Low)
	out["bills_check_load_raw"] = major(maxLoad(res.Raw.Loads))
	out["bills_check_load_smart"] = major(maxLoad(res.Smart.Loads))
	out["bills_even_gain"] = major(res.EvenGainMinor)
	out["bills_paid_ahead"] = float64(len(res.AheadByID))
	var bestSuggest int64
	if len(res.Suggestions) > 0 {
		bestSuggest = res.Suggestions[0].LowGainMinor
	}
	out["bills_suggest_gain"] = major(bestSuggest)
}

// RecurringVarNames are the fixed recurring-schedule aggregates addRecurringVars exposes.
var RecurringVarNames = []string{
	"recurring_monthly_in",  // Σ positive monthly equivalents (major units, base currency)
	"recurring_monthly_out", // Σ |negative| monthly equivalents (positive figure)
	"recurring_monthly_net", // in − out
	"recurring_count",       // number of scheduled flows
}

// RecurringVarFields are the per-flow metric suffixes exposed on the surface.
var RecurringVarFields = []string{"monthly", "amount"}

// RecurringVarBase pairs a recurring flow with the disambiguated variable prefix its
// values are keyed under ("recurring_<slug>_"). Single source of truth for per-flow
// variable naming — the flow's stable identity on the formula surface.
type RecurringVarBase struct {
	Recurring domain.Recurring
	Prefix    string
}

// RecurringVarBases returns one entry per flow, in stable order, same-name flows
// disambiguated with a numeric suffix.
func RecurringVarBases(recs []domain.Recurring) []RecurringVarBase {
	used := map[string]bool{}
	out := make([]RecurringVarBase, 0, len(recs))
	for _, r := range recs {
		slug := budgetVarSlug(r.Label)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, RecurringVarBase{Recurring: r, Prefix: "recurring_" + slug + "_"})
	}
	return out
}

// RecurringVarSlug exposes the slugging used for per-flow variable names (for UI previews).
func RecurringVarSlug(s string) string { return budgetVarSlug(s) }

// addRecurringVars exposes the recurring schedule as engine variables: the fixed
// monthly aggregates (in/out/net/count) plus, per flow, recurring_<slug>_monthly
// (its signed monthly equivalent) and recurring_<slug>_amount (its signed
// per-occurrence amount). Money is FX-converted to the base currency, major units.
func addRecurringVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	var in, outflow int64
	for _, r := range d.Recurring {
		me := toBase(r.MonthlyEquivalent(), r.Amount.Currency)
		if me >= 0 {
			in += me
		} else {
			outflow += -me
		}
	}
	out["recurring_monthly_in"] = major(in)
	out["recurring_monthly_out"] = major(outflow)
	out["recurring_monthly_net"] = major(in - outflow)
	out["recurring_count"] = float64(len(d.Recurring))
	for _, base := range RecurringVarBases(d.Recurring) {
		r := base.Recurring
		out[base.Prefix+"monthly"] = major(toBase(r.MonthlyEquivalent(), r.Amount.Currency))
		out[base.Prefix+"amount"] = major(toBase(r.Amount.Amount, r.Amount.Currency))
	}
}

// PlanningVarNames are the fixed planning-policy variables addPlanningVars exposes.
var PlanningVarNames = []string{
	"runway_buffer",    // cash-runway liquidity floor (major units, base currency)
	"runway_days",      // cash-runway projection horizon in days
	"forecast_horizon", // net-worth forecast horizon in months
}

// PlanVarFields are the per-plan metric suffixes exposed on the surface.
var PlanVarFields = []string{"end", "monthly", "runway"}

// PlanVarBase pairs a plan with the disambiguated variable prefix its values are keyed under
// ("plan_<slug>_"). Single source of truth for per-plan variable naming.
type PlanVarBase struct {
	Plan   domain.Plan
	Prefix string
}

// PlanVarBases returns one entry per saved plan, in stable order, same-name plans disambiguated.
func PlanVarBases(plans []domain.Plan) []PlanVarBase {
	used := map[string]bool{}
	out := make([]PlanVarBase, 0, len(plans))
	for _, p := range plans {
		slug := budgetVarSlug(p.Name)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, PlanVarBase{Plan: p, Prefix: "plan_" + slug + "_"})
	}
	return out
}

// PlanVarSlug exposes the slugging used for per-plan variable names (for UI previews).
func PlanVarSlug(s string) string { return budgetVarSlug(s) }

// addPlanningVars exposes the planning policy + each saved what-if plan as engine variables:
// runway_buffer / runway_days / forecast_horizon from the config, and per plan
// plan_<slug>_end (projected end-of-horizon balance, major units), plan_<slug>_monthly (net
// monthly change), plan_<slug>_runway (months until the balance depletes, 0 if it never does).
func addPlanningVars(out map[string]float64, d Data, major func(int64) float64) {
	out["runway_buffer"] = major(d.Planning.RunwayBufferMinor)
	out["runway_days"] = float64(d.Planning.RunwayDays)
	out["forecast_horizon"] = float64(d.Planning.ForecastMonths)
	for _, base := range PlanVarBases(d.Plans) {
		p := base.Plan
		out[base.Prefix+"end"] = major(planning.EndBalance(p))
		out[base.Prefix+"monthly"] = major(planning.MonthlyNet(p))
		months, depletes := planning.RunwayMonths(p)
		if !depletes {
			months = 0
		}
		out[base.Prefix+"runway"] = months
	}
}

// AllocVarNames are the fixed allocate-plan variables addAllocVars exposes, in a stable order.
var AllocVarNames = []string{
	"alloc_amount",            // total the user is putting to work (major units, base currency)
	"alloc_reserve",           // amount held back from the split (emergency buffer)
	"alloc_max_per",           // per-destination cap (0 = uncapped)
	"alloc_allocatable",       // amount − reserve, floored at 0 (what actually gets split)
	"alloc_reserved_pct",      // reserve ÷ amount × 100 (0 when amount is 0)
	"alloc_destination_count", // eligible places to put money (asset accounts + interest-bearing debts + goals)
}

// addAllocVars exposes the persisted allocate plan as fixed alloc_* variables, so a plan figure
// (how much you're allocating, what you're holding back, how many destinations qualify) can be
// referenced in a formula or dashboard widget. Derived purely from Data — amounts from the
// plan config, the destination count from the eligible accounts/goals.
func addAllocVars(out map[string]float64, d Data, major func(int64) float64) {
	amount := major(d.Alloc.AmountMinor)
	reserve := major(d.Alloc.ReserveMinor)
	allocatable := max(d.Alloc.AmountMinor-d.Alloc.ReserveMinor, 0)
	reservedPct := 0.0
	if d.Alloc.AmountMinor > 0 {
		reservedPct = float64(d.Alloc.ReserveMinor) / float64(d.Alloc.AmountMinor) * 100
		if reservedPct < 0 {
			reservedPct = 0
		}
		if reservedPct > 100 {
			reservedPct = 100
		}
	}
	// Eligible destinations: non-archived asset accounts + interest-bearing liabilities +
	// non-archived goals — the same universe the allocate ranker draws from.
	destinations := 0
	for _, a := range d.Accounts {
		if a.Archived {
			continue
		}
		if a.Class == domain.ClassLiability {
			if a.InterestRateAPR > 0 {
				destinations++
			}
			continue
		}
		destinations++
	}
	destinations += len(d.Goals)

	out["alloc_amount"] = amount
	out["alloc_reserve"] = reserve
	out["alloc_max_per"] = major(d.Alloc.MaxPerMinor)
	out["alloc_allocatable"] = major(allocatable)
	out["alloc_reserved_pct"] = reservedPct
	out["alloc_destination_count"] = float64(destinations)
}

// addGoalVars exposes each goal as its own named variables, so a formula or widget can
// reference a specific goal — e.g. goal_emergency_remaining. Each goal contributes,
// keyed by a slug of its name (or explicit VarName):
//
//   - goal_<slug>_target      the goal's target amount (major units, base currency)
//   - goal_<slug>_saved       the amount saved so far
//   - goal_<slug>_remaining   target − saved (0 when reached)
//   - goal_<slug>_percent     saved ÷ target × 100 (financial money %; 0 when target is 0)
//   - goal_<slug>_progress    KIND-AWARE percent complete (money %, to-do %, milestone 0/100,
//     or habit check-in %) — the right progress figure whatever the goal kind
//   - goal_<slug>_tasks_done  number of the goal's linked to-dos that are done
//   - goal_<slug>_tasks_total number of to-dos linked to the goal (all kinds)
//   - goal_<slug>_done        1 when the goal has reached its objective, else 0
//   - goal_<slug>_streak      current habit check-in streak (0 for non-habit goals)
//   - goal_<slug>_earmarked   virtually reserved toward the goal (no txn posted)
//   - goal_<slug>_coverage    saved + earmarked (committed plus reserved-in-place)
//   - goal_<slug>_covered_pct coverage ÷ target × 100 (0 when target is 0)
//
// Money amounts are FX-converted to the base currency. Name collisions are disambiguated
// with a numeric suffix in stable goal order; archived goals still expose their variables.
func addGoalVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	for _, base := range GoalVarBases(d.Goals) {
		g := base.Goal
		target := major(toBase(g.TargetAmount.Amount, g.TargetAmount.Currency))
		saved := major(toBase(g.CurrentAmount.Amount, g.CurrentAmount.Currency))
		remaining := target - saved
		if remaining < 0 {
			remaining = 0
		}
		percent := 0.0
		if target != 0 {
			percent = saved / target * 100
		}
		prog := goals.EvaluateProgress(g, d.Tasks, d.Now)
		// tasks_done/_total are the LITERAL linked to-do counts (any goal can have
		// linked to-dos), distinct from prog.Done/Total which for a habit/milestone
		// counts check-ins / the milestone step rather than to-dos.
		tasksDone, tasksTotal := goals.TaskCounts(d.Tasks, g.ID)
		done := 0.0
		if prog.Complete {
			done = 1
		}
		// Virtual allocation ("earmarks") as first-class formula context: how much of the
		// goal is reserved in place, committed+earmarked coverage, and that coverage as a
		// percent of target — so assessments can reason about reserved-but-uncommitted funds.
		earmarked := major(toBase(g.AllocatedMinor(), g.TargetAmount.Currency))
		coverage := saved + earmarked
		coveredPct := 0.0
		if target != 0 {
			coveredPct = coverage / target * 100
		}
		out[base.Prefix+"target"] = target
		out[base.Prefix+"saved"] = saved
		out[base.Prefix+"remaining"] = remaining
		out[base.Prefix+"percent"] = percent
		out[base.Prefix+"progress"] = float64(prog.Percent)
		out[base.Prefix+"tasks_done"] = float64(tasksDone)
		out[base.Prefix+"tasks_total"] = float64(tasksTotal)
		out[base.Prefix+"done"] = done
		out[base.Prefix+"streak"] = float64(prog.Streak)
		out[base.Prefix+"earmarked"] = earmarked
		out[base.Prefix+"coverage"] = coverage
		out[base.Prefix+"covered_pct"] = coveredPct
	}
}

// GoalVarFields are the per-goal metric suffixes exposed on the surface. Shared with the
// widget/formula catalog so the picker matches the surface.
var GoalVarFields = []string{"target", "saved", "remaining", "percent", "progress", "tasks_done", "tasks_total", "done", "streak", "earmarked", "coverage", "covered_pct"}

// GoalVarBase pairs a goal with the disambiguated variable prefix its values are keyed
// under ("goal_<slug>_"). Single source of truth for per-goal naming.
type GoalVarBase struct {
	Goal   domain.Goal
	Prefix string // e.g. "goal_emergency_"
}

// GoalVarBases returns one entry per named goal, in stable order, with same-name goals
// disambiguated by a numeric suffix. Goals whose name yields no slug are skipped. An
// explicit VarName wins over the display name (stable across renames).
func GoalVarBases(goals []domain.Goal) []GoalVarBase {
	used := map[string]bool{}
	out := make([]GoalVarBase, 0, len(goals))
	for _, g := range goals {
		src := g.Name
		if g.VarName != "" {
			src = g.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, GoalVarBase{Goal: g, Prefix: "goal_" + slug + "_"})
	}
	return out
}

// GoalVarSlug exposes the slugging used for per-goal variable names (for UI previews).
func GoalVarSlug(s string) string { return budgetVarSlug(s) }

// addPoolVars exposes each account pool as a pool_<slug>_value variable: the combined
// current balance of its member accounts (FX-converted to base), so a custom group like
// "Retirement" (401k + Roth IRA) can be referenced by name in any formula or widget.
func addPoolVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	if len(d.Pools) == 0 {
		return
	}
	byID := make(map[string]domain.Account, len(d.Accounts))
	for _, a := range d.Accounts {
		byID[a.ID] = a
	}
	for _, base := range PoolVarBases(d.Pools) {
		var total int64
		for _, aid := range base.Pool.AccountIDs {
			a, ok := byID[aid]
			if !ok || a.Archived {
				continue
			}
			if bal, err := ledger.Balance(a, d.Transactions); err == nil {
				total += toBase(bal.Amount, bal.Currency)
			}
		}
		out[base.Prefix+"value"] = major(total)
	}
}

// PoolVarFields are the per-pool metric suffixes exposed on the surface.
var PoolVarFields = []string{"value"}

// PoolVarBase pairs a pool with the disambiguated variable prefix its values are keyed
// under ("pool_<slug>_"). Single source of truth for per-pool variable naming.
type PoolVarBase struct {
	Pool   PoolDef
	Prefix string // e.g. "pool_retirement_"
}

// PoolVarBases returns one entry per named pool, in stable order, with same-name pools
// disambiguated by a numeric suffix. An explicit VarName wins over the display name.
func PoolVarBases(pools []PoolDef) []PoolVarBase {
	used := map[string]bool{}
	out := make([]PoolVarBase, 0, len(pools))
	for _, p := range pools {
		src := p.Name
		if p.VarName != "" {
			src = p.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, PoolVarBase{Pool: p, Prefix: "pool_" + slug + "_"})
	}
	return out
}

// PoolVarSlug exposes the slugging used for per-pool variable names (for UI previews).
func PoolVarSlug(s string) string { return budgetVarSlug(s) }

// addBudgetVars exposes each budget as its own set of named variables, so a formula or
// dashboard widget can reference a specific budget directly — e.g. budget_groceries_remaining
// or budget_rent_percent. Each budget contributes, keyed by a slug of its name:
//
//   - budget_<slug>_limit      the budget's limit (major units, base currency)
//   - budget_<slug>_spent      spent in the budget's own current period
//   - budget_<slug>_remaining  limit − spent (may be negative when overspent)
//   - budget_<slug>_over       overspend = max(0, spent − limit)
//   - budget_<slug>_percent    spent ÷ limit × 100 (0 when limit is 0)
//
// Spent is measured over the budget's OWN period window (monthly/weekly/…), anchored to
// the caller's week start, so it matches what the Budgets screen shows. Name collisions
// are disambiguated with a numeric suffix (…_2, …_3) in stable budget order.
func addBudgetVars(out map[string]float64, d Data, major func(int64) float64) {
	for _, base := range BudgetVarBases(d.Budgets) {
		b := base.Budget
		start, end := budgeting.PeriodRange(b.Period, d.Now, d.WeekStart)
		spent := 0.0
		if s, err := budgeting.Spent(b, d.Transactions, start, end, d.Rates); err == nil {
			spent = major(s.Amount)
		}
		limit := major(b.Limit.Amount)
		remaining := limit - spent
		over := 0.0
		if spent > limit {
			over = spent - limit
		}
		percent := 0.0
		if limit != 0 {
			percent = spent / limit * 100
		}
		out[base.Prefix+"limit"] = limit
		out[base.Prefix+"spent"] = spent
		out[base.Prefix+"remaining"] = remaining
		out[base.Prefix+"over"] = over
		out[base.Prefix+"percent"] = percent
	}
}

// BudgetVarFields are the per-budget metric suffixes exposed on the surface, so a
// budget "Groceries" contributes budget_groceries_limit / _spent / _remaining / _over
// / _percent. Shared with the widget/formula catalog so the picker matches the surface.
var BudgetVarFields = []string{"limit", "spent", "remaining", "over", "percent"}

// BudgetVarBase pairs a budget with the disambiguated variable prefix its values are
// keyed under ("budget_<slug>_"). It is the single source of truth for per-budget
// variable naming — both the surface builder (addBudgetVars) and the discoverability
// catalog build from it, so the names they show always match the names that resolve.
type BudgetVarBase struct {
	Budget domain.Budget
	Prefix string // e.g. "budget_groceries_"
}

// BudgetVarBases returns one entry per named budget, in stable order, with same-name
// budgets disambiguated by a numeric suffix (…_2, …_3). Budgets whose name has no
// alphanumerics (so no usable slug) are skipped.
func BudgetVarBases(budgets []domain.Budget) []BudgetVarBase {
	used := map[string]bool{}
	out := make([]BudgetVarBase, 0, len(budgets))
	for _, b := range budgets {
		// An explicit VarName wins over the display name, so a budget's variable handle is
		// stable across renames; both are slugged so the result is always formula-safe.
		src := b.Name
		if b.VarName != "" {
			src = b.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, BudgetVarBase{Budget: b, Prefix: "budget_" + slug + "_"})
	}
	return out
}

// BudgetVarSlug exposes the slugging used for per-budget variable names, so the UI can
// preview the handle a name/var-name will produce (must match what the surface resolves).
func BudgetVarSlug(s string) string { return budgetVarSlug(s) }

// addAccountVars exposes each account as its own named variables, so a formula or widget
// can reference a specific account — e.g. account_checking_balance. Each account
// contributes, keyed by a slug of its name (or explicit VarName):
//
//   - account_<slug>_balance   the account's current balance (major units, base currency)
//   - account_<slug>_cleared   the balance counting only cleared transactions
//   - account_<slug>_earmarked how much of this account is reserved toward goals (no txn)
//   - account_<slug>_free      balance minus earmarks — what's not spoken for (≥ 0)
//
// Balances are FX-converted to the base currency (same as net worth) so accounts in
// different currencies compare on one scale. Name collisions are disambiguated with a
// numeric suffix (…_2, …_3) in stable account order.
func addAccountVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	// How much of each account is virtually earmarked toward goals (base minor units),
	// precomputed once so a formula can ask "how much of this account is spoken for?".
	earmarkByAcct := map[string]int64{}
	for _, g := range d.Goals {
		for _, al := range g.Allocations {
			if al.AccountID != "" {
				earmarkByAcct[al.AccountID] += toBase(al.Amount.Amount, al.Amount.Currency)
			}
		}
	}
	for _, base := range AccountVarBases(d.Accounts) {
		a := base.Account
		balBase := 0.0
		if bal, err := ledger.Balance(a, d.Transactions); err == nil {
			balBase = major(toBase(bal.Amount, bal.Currency))
			out[base.Prefix+"balance"] = balBase
		}
		if cl, err := ledger.ClearedBalance(a, d.Transactions); err == nil {
			out[base.Prefix+"cleared"] = major(toBase(cl.Amount, cl.Currency))
		}
		earmarked := major(earmarkByAcct[a.ID])
		out[base.Prefix+"earmarked"] = earmarked
		free := balBase - earmarked
		if free < 0 {
			free = 0
		}
		out[base.Prefix+"free"] = free // balance not earmarked toward any goal (never negative)
	}
}

// AccountVarFields are the per-account metric suffixes exposed on the surface. Shared
// with the widget/formula catalog so the picker matches the surface.
var AccountVarFields = []string{"balance", "cleared", "earmarked", "free"}

// AccountVarBase pairs an account with the disambiguated variable prefix its values are
// keyed under ("account_<slug>_"). Single source of truth for per-account naming —
// both the surface builder and the catalog build from it.
type AccountVarBase struct {
	Account domain.Account
	Prefix  string // e.g. "account_checking_"
}

// AccountVarBases returns one entry per named account, in stable order, with same-name
// accounts disambiguated by a numeric suffix. Accounts whose name yields no slug are
// skipped. An explicit VarName wins over the display name (stable across renames).
func AccountVarBases(accounts []domain.Account) []AccountVarBase {
	used := map[string]bool{}
	out := make([]AccountVarBase, 0, len(accounts))
	for _, a := range accounts {
		src := a.Name
		if a.VarName != "" {
			src = a.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, AccountVarBase{Account: a, Prefix: "account_" + slug + "_"})
	}
	return out
}

// AccountVarSlug exposes the slugging used for per-account variable names, so the UI can
// preview the handle a name/var-name will produce (must match what the surface resolves).
func AccountVarSlug(s string) string { return budgetVarSlug(s) }

// addDebtVars exposes each liability (a debt) as its own named variables, so a formula or
// widget can reference a specific debt — e.g. debt_visa_utilization or debt_car_loan_apr.
// Debts are the non-archived liability accounts; each contributes, keyed by a slug of its
// name (or explicit VarName), the debt-specific figures that the generic account_<slug>_*
// surface does not carry (APR, minimum payment, credit limit, utilization):
//
//   - debt_<slug>_balance      the amount currently owed (positive magnitude, base currency)
//   - debt_<slug>_apr          the interest rate as an annual percentage (as-entered, e.g. 19.99)
//   - debt_<slug>_min_payment  the required minimum monthly payment (major units, base currency)
//   - debt_<slug>_limit        the credit limit (major units, base currency; 0 when not a line of credit)
//   - debt_<slug>_available    remaining credit = max(0, limit − balance) (0 when no limit)
//   - debt_<slug>_utilization  balance ÷ limit × 100 (0 when there is no limit)
//
// Money amounts are FX-converted to the base currency so debts in different currencies
// compare on one scale. Name collisions are disambiguated with a numeric suffix in stable
// account order (same scheme as accounts/goals).
func addDebtVars(out map[string]float64, d Data, major func(int64) float64, toBase func(int64, string) int64) {
	for _, base := range DebtVarBases(d.Accounts) {
		a := base.Account
		var balance float64
		if bal, err := ledger.Balance(a, d.Transactions); err == nil {
			mag := bal.Amount
			if mag < 0 {
				mag = -mag
			}
			balance = major(toBase(mag, bal.Currency))
		}
		limit := major(toBase(a.CreditLimit.Amount, a.CreditLimit.Currency))
		available := 0.0
		utilization := 0.0
		if limit > 0 {
			available = limit - balance
			if available < 0 {
				available = 0
			}
			utilization = balance / limit * 100
		}
		out[base.Prefix+"balance"] = balance
		out[base.Prefix+"apr"] = a.InterestRateAPR
		out[base.Prefix+"min_payment"] = major(toBase(a.MinPayment.Amount, a.MinPayment.Currency))
		out[base.Prefix+"limit"] = limit
		out[base.Prefix+"available"] = available
		out[base.Prefix+"utilization"] = utilization
	}
}

// DebtVarFields are the per-debt metric suffixes exposed on the surface. Shared with the
// widget/formula catalog so the picker matches the surface.
var DebtVarFields = []string{"balance", "apr", "min_payment", "limit", "available", "utilization"}

// DebtVarBase pairs a liability account with the disambiguated variable prefix its values
// are keyed under ("debt_<slug>_"). Single source of truth for per-debt variable naming —
// both the surface builder (addDebtVars) and the discoverability catalog build from it.
type DebtVarBase struct {
	Account domain.Account
	Prefix  string // e.g. "debt_visa_"
}

// DebtVarBases returns one entry per non-archived liability account, in stable order, with
// same-name debts disambiguated by a numeric suffix. Accounts that are archived, not a
// liability, or whose name yields no slug are skipped. An explicit VarName wins over the
// display name (stable across renames).
func DebtVarBases(accounts []domain.Account) []DebtVarBase {
	used := map[string]bool{}
	out := make([]DebtVarBase, 0)
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability {
			continue
		}
		src := a.Name
		if a.VarName != "" {
			src = a.VarName
		}
		slug := budgetVarSlug(src)
		if slug == "" {
			continue
		}
		for n := 1; ; n++ {
			candidate := slug
			if n > 1 {
				candidate = slug + "_" + strconv.Itoa(n)
			}
			if !used[candidate] {
				slug = candidate
				used[candidate] = true
				break
			}
		}
		out = append(out, DebtVarBase{Account: a, Prefix: "debt_" + slug + "_"})
	}
	return out
}

// budgetVarSlug turns a budget name into a formula-safe variable segment: lowercase,
// with every run of non-alphanumeric characters collapsed to a single underscore and
// edges trimmed. "Baby & Childcare" → "baby_childcare".
func budgetVarSlug(name string) string {
	var sb strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			sb.WriteRune(r)
			prevUnderscore = false
		default:
			if !prevUnderscore && sb.Len() > 0 {
				sb.WriteByte('_')
				prevUnderscore = true
			}
		}
	}
	return strings.TrimRight(sb.String(), "_")
}

// Derivation explains how one variable is produced — the audit record. For a
// molecule it carries the formula and the value of each variable the formula
// references (the atoms it's built from); for an atom it carries a source note.
type Derivation struct {
	Name    string
	Kind    string             // "atom" | "molecule" | "custom"
	Formula string             // molecule only
	Source  string             // atom/custom only
	Value   float64            // the resolved value
	Inputs  map[string]float64 // molecule only: referenced var → its value
}

// atomSources documents each atom's fundamental reduction (for the audit).
var atomSources = map[string]string{
	"assets":             "Σ FX-converted balances of non-archived asset accounts",
	"liabilities":        "Σ magnitudes of non-archived liability-account balances",
	"liquid_cash":        "Σ balances of non-archived cash-type accounts",
	"income":             "Σ positive non-transfer transactions in the period",
	"expense":            "Σ |negative non-transfer transactions| in the period",
	"income_count":       "count of income transactions in the period",
	"expense_count":      "count of expense transactions in the period",
	"bills_due":          "Σ bills due before this calendar month-end",
	"goal_needs":         "Σ prorated monthly goal contributions",
	"earmarked_total":    "Σ virtual goal earmarks (reserved-in-place, uncommitted)",
	"accounts":           "count of non-archived accounts",
	"asset_accounts":     "count of non-archived asset-class accounts",
	"liability_accounts": "count of non-archived liability-class accounts",
	"debt_count":         "count of non-archived liability accounts",
	"revolving_balance":  "Σ magnitudes of non-archived credit-card balances",
	"credit_limit_total": "Σ credit limits of non-archived credit-card accounts",
	"min_payments_total": "Σ minimum monthly payments across non-archived liabilities",
	"transactions":       "count of transactions",
	"members":            "count of members",
	"budgets":            "count of budgets",
	"goals":              "count of goals",
	"tasks":              "count of tasks",
}

// Explain returns how name is derived, given the computed vars and the molecule
// set (empty → defaults). It traces a molecule to the atoms it reads, so any
// figure on screen can be audited down to its indivisible inputs.
func Explain(name string, vars map[string]float64, molecules []domain.Molecule) (Derivation, bool) {
	if len(molecules) == 0 {
		molecules = DefaultMolecules()
	}
	for _, m := range molecules {
		if m.Name != name {
			continue
		}
		d := Derivation{Name: name, Kind: "molecule", Formula: m.Formula, Source: m.Doc, Value: vars[name], Inputs: map[string]float64{}}
		if refs, err := formula.References(m.Formula); err == nil {
			for _, r := range refs {
				if v, ok := vars[r]; ok {
					d.Inputs[r] = v
				}
			}
		}
		return d, true
	}
	if src, ok := atomSources[name]; ok {
		return Derivation{Name: name, Kind: "atom", Source: src, Value: vars[name]}, true
	}
	if _, ok := vars[name]; ok {
		return Derivation{Name: name, Kind: "custom", Source: "custom field sum", Value: vars[name]}, true
	}
	return Derivation{}, false
}

// addCustomFieldVars folds each NUMBER custom field into the surface as
// cf_<entity>_<key>, summed over that entity's collection (NOT scaled — values are
// taken as-is). Transaction fields are period-scoped; account/goal fields exclude
// archived.
func addCustomFieldVars(out map[string]float64, d Data, start, end time.Time) {
	for _, def := range d.CustomDefs {
		if def.Type != customfields.TypeNumber {
			continue
		}
		short := entityTypeShort(def.EntityType)
		if short == "" {
			continue
		}
		name := "cf_" + short + "_" + def.Key
		var total float64
		switch def.EntityType {
		case "transaction":
			for _, t := range d.Transactions {
				if !dateutil.InRange(t.Date, start, end) {
					continue
				}
				if v, ok := numFrom(t.Custom, def.Key); ok {
					total += v
				}
			}
		case "account":
			for _, a := range d.Accounts {
				if a.Archived {
					continue
				}
				if v, ok := numFrom(a.Custom, def.Key); ok {
					total += v
				}
			}
		case "budget":
			for _, b := range d.Budgets {
				if v, ok := numFrom(b.Custom, def.Key); ok {
					total += v
				}
			}
		case "goal":
			for _, g := range d.Goals {
				if g.Archived {
					continue
				}
				if v, ok := numFrom(g.Custom, def.Key); ok {
					total += v
				}
			}
		case "member":
			for _, m := range d.Members {
				if v, ok := numFrom(m.Custom, def.Key); ok {
					total += v
				}
			}
		case "task":
			for _, tk := range d.Tasks {
				if v, ok := numFrom(tk.Custom, def.Key); ok {
					total += v
				}
			}
		default:
			continue
		}
		out[name] = total
	}
}

// BudgetVars returns the per-budget variable overrides for a formula evaluated in a
// single budget's CONTEXT (a cover amount is evaluated in the destination's context; a
// source weight in that source's). It exposes the budget's own spent / limit /
// remaining / overspend / percent (major units), plus each of its NUMBER custom fields
// as cf_budget_<key> bound to THIS budget's value (rather than the household sum that
// Vars folds in). Layer these on top of Vars() with Merge so a cover formula can
// reference both household aggregates and this budget's own specifics.
//
//   - remaining = limit − spent (negative when over)
//   - overspend = max(0, spent − limit)  (0 when on/under budget)
//   - percent   = spent / limit × 100     (0 when limit is 0)
func BudgetVars(b domain.Budget, spentMajor, limitMajor float64, defs []customfields.Def) map[string]float64 {
	remaining := limitMajor - spentMajor
	overspend := 0.0
	if remaining < 0 {
		overspend = -remaining
	}
	percent := 0.0
	if limitMajor != 0 {
		percent = spentMajor / limitMajor * 100
	}
	out := map[string]float64{
		"spent":     spentMajor,
		"limit":     limitMajor,
		"remaining": remaining,
		"overspend": overspend,
		"percent":   percent,
	}
	for _, def := range defs {
		if def.EntityType != "budget" || def.Type != customfields.TypeNumber {
			continue
		}
		if v, ok := numFrom(b.Custom, def.Key); ok {
			out["cf_budget_"+def.Key] = v
		}
	}
	return out
}

// Merge returns a new map with over's entries layered on top of base (base is copied
// first, so neither input is mutated). Used to overlay per-budget context vars on the
// global surface for a contextual formula evaluation.
func Merge(base, over map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// entityTypeShort maps a custom-field EntityType to the cf_ namespace segment.
func entityTypeShort(entityType string) string {
	switch entityType {
	case "transaction":
		return "txn"
	case "account":
		return "acct"
	case "budget":
		return "budget"
	case "goal":
		return "goal"
	case "member":
		return "member"
	case "task":
		return "task"
	}
	return ""
}

// numFrom reads a numeric value for key from a custom map. Store-loaded JSON
// numbers arrive as float64; ints are accepted too. Non-numeric values are skipped.
func numFrom(custom map[string]any, key string) (float64, bool) {
	v, ok := custom[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// CustomFieldVar returns the engine variable name a numeric custom-field definition
// produces (cf_<entity>_<key>), or "" if the def is not a numeric field the surface
// folds in. Lets callers map a Def back to its metric without re-deriving the scheme.
func CustomFieldVar(def customfields.Def) string {
	if def.Type != customfields.TypeNumber {
		return ""
	}
	short := entityTypeShort(def.EntityType)
	if short == "" {
		return ""
	}
	return "cf_" + short + "_" + def.Key
}

// CustomFieldNames returns the cf_* variable names the given defs produce, in
// definition order — for the binding editor's reference list.
func CustomFieldNames(defs []customfields.Def) []string {
	var out []string
	for _, def := range defs {
		if def.Type != customfields.TypeNumber {
			continue
		}
		if short := entityTypeShort(def.EntityType); short != "" {
			out = append(out, "cf_"+short+"_"+def.Key)
		}
	}
	return out
}

// AtomNames returns the indivisible variable names (the leaves of the surface).
func AtomNames() []string { return append([]string(nil), atomNames...) }

// SortedNames returns the built-in variable names in alphabetical order.
func SortedNames() []string {
	out := append([]string(nil), Names...)
	sort.Strings(out)
	return out
}
