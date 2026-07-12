// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/reports"
	"github.com/monstercameron/CashFlux/internal/safespend"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
)

// budgets.go holds the shared logic behind the widgetized /budgets surface: the
// per-period status computation every tile reads (computeBudgetView), the row
// mutation handlers (buildBudgetRowCallbacks), and the small formatting helpers the
// budget rows and add form share. The surface host lives in budgets_widget.go and the
// tile bodies in budgets_tiles.go, mirroring the /accounts split.

// budgetView is the fully computed budget picture for the current view window: the
// health-sorted statuses plus every per-budget annotation (pace, rollover carry,
// prorated pace, envelope balance, effective method) and the roll-up totals the
// summary tile shows. It is a pure value — computeBudgetView takes the resolved hook
// values as plain arguments, so both the summary and list tiles can build it without
// sharing state.
type budgetView struct {
	Base              string
	Method            budgeting.Methodology
	Statuses          []budgeting.Status
	CatName           map[string]string
	PaceOver          map[string]string                // budgetID → projected overspend (in-progress only)
	RollCarry         map[string]string                // budgetID → previous-period carry phrase
	RollNeg           map[string]bool                  // budgetID → carry is negative
	RollEffCap        map[string]string                // budgetID → effective cap (rollover, when it differs)
	ProratedRest      map[string]string                // budgetID → even-pace amount left
	EnvAvail          map[string]string                // budgetID → envelope balance (envelope method)
	EnvNeg            map[string]bool                  // budgetID → envelope overdrawn
	Covered           map[string]bool                  // budgetID → received cover money this period
	EffMethod         map[string]budgeting.Methodology // budgetID → resolved method (override or global)
	OverCount         int
	NearCount         int
	TotalSpent        int64
	TotalLimit        int64
	TotalOver         int64
	TotalFundSetAside int64
	BannerIncome      int64 // monthly income (per the chosen basis) for the assign banner
	// SavingsAssigned is the total monthly income assigned to savings/investments —
	// the sum of every savings/investment account's monthly savings budget
	// (Account.MonthlySavings), FX-converted to base minor units. Counts toward
	// "assigned" in the zero-based view so To Assign spans expenses + savings.
	// SavingsAccts is the per-account breakdown the savings section renders.
	SavingsAssigned int64
	SavingsAccts    []savingsAcct
	// RolledOver is last month's unspent budget carried into this month's assignable
	// pool (zero-based view), when the roll-leftover option is on. Raises To Assign.
	RolledOver int64
	// LastMonth holds the "Last month's spend" overlay per budget (keyed by budget ID),
	// populated only when that toggle is on; empty otherwise. LastMonthMode is that
	// toggle, and LastTotalSpent is last period's total spend across all budgets (base
	// minor units) so the summary graph can show last month too.
	LastMonth      map[string]budgetLastMonth
	LastMonthMode  bool
	LastTotalSpent int64
}

// budgetLastMonth is the "Last month's spend" overlay for one budget: the formatted
// actual spend last period, how it lines up against this month's budget (a short
// "$X under"/"$X over" phrase + whether it exceeded it), Pct — last month's spend as a
// percent of this month's budget (uncapped, for the "%" figure) — and Fill, the same
// clamped to 0..100 for the bar width.
type budgetLastMonth struct {
	Spent string
	Delta string
	Over  bool
	Pct   int
	Fill  int
}

// incomeSource is one selectable income category in the "by source" budget basis: its
// id, display name, and how much it brought in last full month (base minor units).
type incomeSource struct {
	CategoryID string
	Name       string
	Minor      int64
}

// savingsAcct is one savings/investment account in the zero-based "Savings &
// investments" section: its identity, the per-account monthly savings budget
// (Account.MonthlySavings, in the account's own currency) and its base-currency
// equivalent, plus — when the account funds a goal — the plan-vs-reality read
// (the goal's target-date pace vs. where this monthly rate actually lands).
type savingsAcct struct {
	AccountID    string
	Name         string
	Type         string // account type value, humanized for the row sublabel
	Currency     string // account currency (also the monthly-savings currency)
	Monthly      int64  // monthly savings budget, account-currency minor units
	AssignedBase int64  // Monthly converted to base currency (0 when unconvertible)

	// Plan-vs-reality against the account's funded goal (populated only when the
	// account funds a financial goal visible to the active member).
	HasGoal       bool
	GoalID        string
	GoalName      string
	GoalComplete  bool   // every linked goal is already met — show the win, not a projection
	MoreGoals     int    // additional incomplete linked goals beyond the one shown (0 = none)
	PlannedMonths int    // whole months from the viewed date to the goal's target (0 = undated)
	RateMonths    int    // months to reach the goal at this monthly rate (0 = can't project)
	DeltaMonths   int    // RateMonths − PlannedMonths (>0 later than planned, <0 sooner)
	SyncMinor     int64  // this monthly expressed in the goal's currency — what Sync writes
	SyncCurrency  string // the goal's currency (money-correct target for MonthlyContribution)
	Synced        bool   // goal.MonthlyContribution already equals SyncMinor/SyncCurrency
}

// monthsUntil counts whole calendar months from `from` to `target`, rounding a
// partial final month up and flooring at 1, matching goals.MonthlyNeeded's
// convention. It returns 0 for an undated or already-past target so callers can
// treat "no plan" uniformly.
func monthsUntil(from, target time.Time) int {
	if target.IsZero() || !target.After(from) {
		return 0
	}
	m := (target.Year()-from.Year())*12 + int(target.Month()) - int(from.Month())
	if target.Day() > from.Day() {
		m++ // a partial final month still needs a contribution
	}
	if m < 1 {
		m = 1
	}
	return m
}

// computeSavingsAccounts builds the per-account savings lines for the zero-based
// view: every non-archived savings/investment account VISIBLE TO THE ACTIVE MEMBER,
// its monthly savings budget converted to base, and — when the account funds a
// financial goal — the plan-vs-reality timeline (how many months the goal is planned
// for vs. how many it takes at this monthly rate). asOf is the viewed reference date
// (the page's period anchor), so paging months moves the "planned" horizon with the
// rest of the view. Accounts are returned sorted by name for a stable list.
func computeSavingsAccounts(app *appstate.App, activeMemberID string, asOf time.Time, base string, rates currency.Rates) []savingsAcct {
	// Index active, member-visible financial goals by the account they fund, nearest
	// target first, so each account picks its most time-sensitive goal. Sinking-fund
	// goals are excluded here — they have their own monthly set-aside line, so counting
	// them a second time via the account row would double-report the same commitment.
	goalsByAcct := map[string][]domain.Goal{}
	for _, g := range app.Goals() {
		if g.Archived || g.IsSinkingFund || !g.IsFinancial() || g.AccountID == "" {
			continue
		}
		if !ownerVisibleTo(g.OwnerID, activeMemberID) {
			continue
		}
		goalsByAcct[g.AccountID] = append(goalsByAcct[g.AccountID], g)
	}
	for id, gs := range goalsByAcct {
		sort.SliceStable(gs, func(i, j int) bool { return goalsvc.LessForList(gs[i], gs[j]) })
		goalsByAcct[id] = gs
	}

	var out []savingsAcct
	for _, ac := range app.Accounts() {
		if ac.Archived || !ac.Type.IsSavingsLike() || !ownerVisibleTo(ac.OwnerID, activeMemberID) {
			continue
		}
		cur := ac.Currency
		if cur == "" {
			cur = base
		}
		monthly := ac.MonthlySavings.Amount
		mCur := ac.MonthlySavings.Currency
		if mCur == "" {
			mCur = cur
		}
		assignedBase := monthly
		if monthly != 0 {
			if conv, err := currency.ConvertBetween(monthly, mCur, base, rates); err == nil {
				assignedBase = conv
			}
		}
		sa := savingsAcct{
			AccountID: ac.ID, Name: ac.Name, Type: humanizeType(string(ac.Type)),
			Currency: cur, Monthly: monthly, AssignedBase: assignedBase,
		}
		// Plan-vs-reality against this account's funded goals: prefer the nearest still-
		// incomplete goal for the projection; if every linked goal is already met, show a
		// "fully funded" state rather than silently reverting to an unlinked-looking row.
		linked := goalsByAcct[ac.ID]
		incomplete := 0
		var picked domain.Goal
		var pickedRem money.Money
		havePick := false
		for _, g := range linked {
			rem, err := goalsvc.Remaining(g)
			if err != nil || rem.Amount <= 0 {
				continue
			}
			incomplete++
			if !havePick {
				picked, pickedRem, havePick = g, rem, true
			}
		}
		switch {
		case havePick:
			sa.HasGoal = true
			sa.GoalID = picked.ID
			sa.GoalName = picked.Name
			sa.MoreGoals = incomplete - 1
			sa.PlannedMonths = monthsUntil(asOf, picked.TargetDate)
			// The monthly rate expressed in the goal's currency for the projection. When
			// an FX rate is missing the conversion fails; rather than silently treat the
			// raw amount as if it were in the goal's currency (which would then be
			// PERSISTED by Sync), leave the projection unset so the row shows no estimate.
			rate, rateOK := monthly, true
			if mCur != pickedRem.Currency {
				if conv, err := currency.ConvertBetween(monthly, mCur, pickedRem.Currency, rates); err == nil {
					rate = conv
				} else {
					rateOK = false
				}
			}
			if rateOK && rate > 0 {
				sa.RateMonths = int((pickedRem.Amount + rate - 1) / rate) // ceil division
				sa.SyncMinor = rate
				sa.SyncCurrency = pickedRem.Currency
				sa.Synced = monthly > 0 && picked.MonthlyContribution.Amount == rate && picked.MonthlyContribution.Currency == pickedRem.Currency
				if sa.PlannedMonths > 0 {
					sa.DeltaMonths = sa.RateMonths - sa.PlannedMonths
				}
			}
		case len(linked) > 0:
			// Every linked goal is already met — surface the win, don't vanish.
			sa.HasGoal = true
			sa.GoalComplete = true
			sa.GoalName = linked[0].Name
		}
		out = append(out, sa)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// computeBudgetView runs the full budget evaluation for the active member scope and
// the given view window. It is the single source of truth the /budgets tiles share:
// the same statuses drive the summary totals, the banners, and the rows. Pure (no
// hooks) — the caller resolves the active member, period window, and prefs and passes
// them in, so it can be called from more than one tile without hook-ordering issues.
// budgetViewCache memoizes computeBudgetView within/across renders. The budgets surface
// renders several tiles (summary, list, savings, formula) that each call it with the same
// inputs in one frame — without this, the full budget-over-all-transactions evaluation
// ran once per tile. The key captures everything the result depends on: the data revision
// (bumps on any dataset mutation), the member scope, the period window, the last-month
// toggle, the base currency + FX rates, and the prefs — so a hit is always current. The
// map is cleared once it grows past a small cap (old data-revision keys are dead weight).
var budgetViewCache = map[string]budgetView{}

// computeBudgetView is the memoized entry point; computeBudgetViewRaw does the work.
func computeBudgetView(app *appstate.App, activeMemberID string, vw period.Window, pr prefs.Prefs, showLastMonth bool) budgetView {
	s := app.Settings()
	key := fmt.Sprintf("%d|%s|%v|%t|%s|%v|%v",
		uistate.CurrentDataRevision(), activeMemberID, vw, showLastMonth, s.BaseCurrency, s.FXRates, pr)
	if v, ok := budgetViewCache[key]; ok {
		return v
	}
	if len(budgetViewCache) > 8 {
		budgetViewCache = map[string]budgetView{}
	}
	v := computeBudgetViewRaw(app, activeMemberID, vw, pr, showLastMonth)
	budgetViewCache[key] = v
	return v
}

func computeBudgetViewRaw(app *appstate.App, activeMemberID string, vw period.Window, pr prefs.Prefs, showLastMonth bool) budgetView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	weekStart := pr.WeekStartWeekday()
	// C128: parse the pay-cycle anchor from prefs. When set, biweekly budget periods
	// snap to the user's actual payday instead of the internal epoch.
	var payCycleAnchor time.Time
	if pr.PayCycleAnchor != "" {
		if t, err := time.Parse("2006-01-02", pr.PayCycleAnchor); err == nil {
			payCycleAnchor = t
		}
	}

	categories := app.Categories()
	catName := make(map[string]string, len(categories))
	for _, c := range categories {
		catName[c.ID] = c.Name
	}

	allBudgets := app.Budgets()
	// C278: when a member view is active, show only that member's budgets plus shared
	// (group) budgets. ownerVisibleTo keeps group-owned budgets visible in every view.
	budgets := make([]domain.Budget, 0, len(allBudgets))
	for _, b := range allBudgets {
		if ownerVisibleTo(b.OwnerID, activeMemberID) {
			budgets = append(budgets, b)
		}
	}
	txns := app.Transactions()
	now := time.Now()
	// Anchor each budget's period to today when the viewed window includes today,
	// otherwise to the window's start (C40).
	viewFrom, viewTo := vw.Range()
	anchor := viewFrom
	if !now.Before(viewFrom) && now.Before(viewTo) {
		anchor = now
	}
	cats := app.Categories()

	statuses := make([]budgeting.Status, 0, len(budgets))
	paceOver := map[string]string{}
	rollCarry := map[string]string{}
	rollNeg := map[string]bool{}
	rollEffCap := map[string]string{}
	proratedRest := map[string]string{}
	covered := map[string]bool{}
	// lastMonth (populated only when the "Last month's spend" overlay is on): each
	// budget's ACTUAL spend last period plus how it compares to this month's budget, so
	// the user can plan this month's amounts against what they really spent.
	lastMonth := map[string]budgetLastMonth{}
	var lastTotalSpent int64 // last period's total spend across budgets (base minor), for the summary graph
	// Pooled leftover: last month's unspent budget (limit − spent, clamped ≥ 0)
	// summed across budgets that DON'T carry their own remaining, when the user opts
	// to roll leftover into next month's assignable pool (zero-based view).
	var pooledRollover int64
	for _, b := range budgets {
		bs, be := budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payCycleAnchor)
		// Flag budgets that received cover money this period (quick ref on the row).
		if !b.CoveredAt.IsZero() && !b.CoveredAt.Before(bs) && b.CoveredAt.Before(be) {
			covered[b.ID] = true
		}
		eval := b
		if b.Rollover {
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, perr := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())); perr == nil {
				if eff, cerr := budgeting.Carryover(prev.Remaining, b.Limit); cerr == nil {
					eval.Limit = eff
					if eff.Amount != b.Limit.Amount {
						rollEffCap[b.ID] = fmtMoney(eff)
					}
				}
				rollCarry[b.ID] = budgetRemainPhrase(prev.Remaining)
				rollNeg[b.ID] = prev.Remaining.IsNegative()
			}
		} else if pr.BudgetRolloverLeftover {
			// This budget doesn't carry its own remaining, so its last-month unspent
			// feeds the pooled leftover that raises next month's assignable budget.
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, perr := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())); perr == nil && prev.Remaining.Amount > 0 {
				if conv, cerr := currency.ConvertBetween(prev.Remaining.Amount, prev.Remaining.Currency, base, rates); cerr == nil {
					pooledRollover += conv
				}
			}
		}
		// This-month top-up: a one-time boost recorded for THIS period only raises the
		// effective cap without touching the base Limit (so it reverts next period). It
		// stacks on any rollover carry-in already folded into eval.Limit above.
		if boost := eval.PeriodBoost(bs); boost != 0 {
			eval.Limit = money.New(eval.Limit.Amount+boost, eval.Limit.Currency)
		}
		st, err := budgeting.EvaluateRollup(eval, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs()))
		if err != nil {
			continue
		}
		statuses = append(statuses, st)
		// Last-month spend overlay (planning): what was actually spent in this budget's
		// categories LAST period, and how that lines up against this month's budget.
		if showLastMonth {
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, perr := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())); perr == nil {
				lastTotalSpent += prev.Spent.Amount
				limitMinor := st.Spent.Amount + st.Remaining.Amount // this period's effective budget
				lm := budgetLastMonth{Spent: fmtMoney(prev.Spent)}
				if limitMinor > 0 {
					if lm.Pct = int(prev.Spent.Amount * 100 / limitMinor); lm.Pct < 0 {
						lm.Pct = 0
					}
					if lm.Fill = lm.Pct; lm.Fill > 100 {
						lm.Fill = 100
					}
				}
				if delta := limitMinor - prev.Spent.Amount; delta >= 0 {
					lm.Delta = uistate.T("budgets.lastMonthUnder", fmtMoney(money.New(delta, base)))
				} else {
					lm.Over = true
					lm.Delta = uistate.T("budgets.lastMonthOver", fmtMoney(money.New(-delta, base)))
				}
				lastMonth[b.ID] = lm
			}
		}
		// Pace projection (D2): warn only while the period is genuinely in progress and
		// the budget isn't already over.
		if p := budgeting.ProjectPace(st, bs, be, now); !p.OnTrack && p.Elapsed > 0 && p.Elapsed < 1 && st.State != budgeting.StateOver {
			paceOver[b.ID] = fmtMoney(p.OverBy)
		}
		// C143: even-pace per-category safe-to-spend, only while in progress and there's
		// still room (paceOver owns the over case).
		if !st.Remaining.IsNegative() && st.Remaining.Amount > 0 {
			daysInPeriod := int(be.Sub(bs).Hours() / 24)
			daysLeft := int(be.Sub(now).Hours() / 24)
			if daysLeft > 0 && daysLeft < daysInPeriod {
				if v := safespend.ComputeCategory(st.Remaining.Amount, daysLeft, daysInPeriod); v > 0 && v < st.Remaining.Amount {
					proratedRest[b.ID] = fmtMoney(money.New(v, base))
				}
			}
		}
	}

	// Health-first ordering (G4): Over → Near/At-risk → On track, then percent-used desc.
	healthRank := func(s budgeting.Status) int {
		switch s.State {
		case budgeting.StateOver:
			return 0
		case budgeting.StateNear:
			return 1
		}
		if paceOver[s.Budget.ID] != "" {
			return 1
		}
		return 2
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		ri, rj := healthRank(statuses[i]), healthRank(statuses[j])
		if ri != rj {
			return ri < rj
		}
		return statuses[i].Percent > statuses[j].Percent
	})

	overCount, nearCount := 0, 0
	var totalSpent, totalLimit, totalOver int64
	for _, s := range statuses {
		switch s.State {
		case budgeting.StateOver:
			overCount++
			if s.Remaining.IsNegative() {
				totalOver += -s.Remaining.Amount
			}
		case budgeting.StateNear:
			nearCount++
		}
		totalSpent += s.Spent.Amount
		totalLimit += s.Spent.Amount + s.Remaining.Amount // limit = spent + remaining
	}

	// The methodology shapes the view (D6). Resolve each budget's effective method and,
	// for envelope budgets, its carried-forward balance.
	method := budgeting.ParseMethodology(app.Settings().BudgetMethodology)
	effectiveMethod := func(b domain.Budget) budgeting.Methodology {
		if m := budgeting.ParseMethodology(b.Methodology); b.Methodology != "" && m.Valid() {
			return m
		}
		return method
	}
	effMethod := map[string]budgeting.Methodology{}
	envAvail := map[string]string{}
	envNeg := map[string]bool{}
	for _, b := range budgets {
		em := effectiveMethod(b)
		effMethod[b.ID] = em
		if em != budgeting.MethodEnvelope {
			continue
		}
		if av, err := budgeting.EnvelopeAvailable(b, txns, anchor, weekStart, rates, categorytree.DescendantsOfAll(cats, b.TrackedCategoryIDs())); err == nil {
			if av.IsNegative() {
				envAvail[b.ID] = fmtMoney(av.Abs()) + " " + uistate.T("budgets.overdrawnWord")
			} else {
				envAvail[b.ID] = fmtMoney(av)
			}
			envNeg[b.ID] = av.IsNegative()
		}
	}

	// Monthly income for the assign banner: prefer the configured figure, otherwise
	// derive it from LAST full month's income. The current period is partial — this
	// month's paychecks may not all have landed yet — so deriving from it under-reports
	// what you have to budget; the most recent complete month is the honest basis.
	ms, _ := budgeting.PeriodRange(domain.PeriodMonthly, anchor, weekStart)
	prevStart := dateutil.AddMonths(ms, -1)
	var bannerIncome int64
	switch pr.BudgetIncomeMode {
	case budgeting.IncomeModeAll, budgeting.IncomeModePaychecks, budgeting.IncomeModeFixed, budgeting.IncomeModeCategories:
		// Explicit basis (zero-based income control): all income, paychecks-only, a fixed
		// figure, or a chosen set of income categories — averaged over BudgetIncomeAvgMonths
		// recent months (last month only when 0/1).
		bannerIncome = budgeting.AveragedIncome(pr.BudgetIncomeMode, pr.BudgetPaycheckMinMinor, pr.MonthlyIncomeMinor, pr.BudgetIncomeCategoryIDs, txns, ms, pr.BudgetIncomeAvgMonths, base, rates)
	default:
		// Unset basis — preserve prior behaviour: configured figure wins, else last month's income.
		bannerIncome = budgeting.IncomeForBudgets(pr.MonthlyIncomeMinor, txns, prevStart, ms, base, rates)
		if bannerIncome == 0 {
			if raw, _, err := ledger.PeriodTotals(txns, prevStart, ms, rates); err == nil {
				bannerIncome = raw.Amount
			}
		}
	}

	// Savings/investments assigned this month: each savings/investment account's own
	// monthly savings budget (Account.MonthlySavings), FX-converted to base. Counts
	// toward "assigned" in the zero-based view (To Assign spans expenses + savings).
	savingsAccts := computeSavingsAccounts(app, activeMemberID, anchor, base, rates)
	var savingsAssigned int64
	for _, sa := range savingsAccts {
		savingsAssigned += sa.AssignedBase
	}

	// C190: sum the monthly set-aside across all active sinking-fund goals.
	var totalFundSetAside int64
	for _, g := range app.Goals() {
		if g.IsSinkingFund && !g.Archived {
			totalFundSetAside += goalsvc.FundSetAsideMinor(g, now)
		}
	}

	return budgetView{
		Base: base, Method: method, Statuses: statuses, CatName: catName,
		PaceOver: paceOver, RollCarry: rollCarry, RollNeg: rollNeg, RollEffCap: rollEffCap,
		ProratedRest: proratedRest, EnvAvail: envAvail, EnvNeg: envNeg, Covered: covered, EffMethod: effMethod,
		OverCount: overCount, NearCount: nearCount,
		TotalSpent: totalSpent, TotalLimit: totalLimit, TotalOver: totalOver,
		TotalFundSetAside: totalFundSetAside, BannerIncome: bannerIncome,
		SavingsAssigned: savingsAssigned, SavingsAccts: savingsAccts,
		RolledOver:     pooledRollover,
		LastMonth:      lastMonth,
		LastMonthMode:  showLastMonth,
		LastTotalSpent: lastTotalSpent,
	}
}

// computeIncomeSources builds the income-source menu the "by source" basis presents:
// every income-kind category with how much it earned per month over the last `months`
// full months ending at monthStart (a plain average when months > 1), largest first,
// plus any uncategorized income that actually landed. months < 1 is treated as 1 (last
// month only). A category with no income in the window shows a zero amount, so the user
// can still pre-include it; the checked rows sum to the AveragedIncome figure.
func computeIncomeSources(app *appstate.App, base string, rates currency.Rates, monthStart time.Time, months int) []incomeSource {
	if months < 1 {
		months = 1
	}
	start := dateutil.AddMonths(monthStart, -months)
	amtByCat := map[string]int64{}
	if lines, err := reports.IncomeByCategory(app.Transactions(), start, monthStart, rates); err == nil {
		for _, ln := range lines {
			amtByCat[ln.CategoryID] = ln.Amount / int64(months)
		}
	}
	// Rank rows by LAST month's amount — a window independent of `months` — so toggling
	// the averaging window changes only the figures shown, never the row order (no
	// reshuffle under the user's cursor). Sources that only earned in prior months stay
	// pinned at the bottom, showing their average, instead of jumping up when averaging
	// surfaces them.
	rankByCat := map[string]int64{}
	rankStart := dateutil.AddMonths(monthStart, -1)
	if lines, err := reports.IncomeByCategory(app.Transactions(), rankStart, monthStart, rates); err == nil {
		for _, ln := range lines {
			rankByCat[ln.CategoryID] = ln.Amount
		}
	}
	var out []incomeSource
	for _, c := range app.Categories() {
		if c.Kind != domain.KindIncome {
			continue
		}
		out = append(out, incomeSource{CategoryID: c.ID, Name: c.Name, Minor: amtByCat[c.ID]})
	}
	if amt := amtByCat[""]; amt > 0 {
		out = append(out, incomeSource{CategoryID: "", Name: uistate.T("budgets.incomeSourceUncat"), Minor: amt})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rankByCat[out[i].CategoryID], rankByCat[out[j].CategoryID]; ri != rj {
			return ri > rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// budgetRowCallbacks bundles the per-row mutation handlers the list tile hands to each
// BudgetRow. The edit / top-up / cover editors now live in the shell-root flip modal
// (BudgetEditForm) which mutates the store directly, so the row only needs delete.
type budgetRowCallbacks struct {
	OnDelete          func(string)
	OnRemoveRecurring func(string)
}

// buildBudgetRowCallbacks wires the store mutations for BudgetRow, mirroring the
// /accounts buildAcctRowCallbacks convention.
func buildBudgetRowCallbacks(app *appstate.App, base string, catName map[string]string) budgetRowCallbacks {
	return budgetRowCallbacks{
		OnDelete: func(budgetID string) {
			// Guard the destructive delete with a confirm (matches the transactions delete
			// pattern) — a single misclick was previously unrecoverable.
			name := uistate.T("budgets.thisBudget")
			for _, b := range app.Budgets() {
				if b.ID == budgetID {
					if n := catName[b.CategoryID]; n != "" {
						name = n
					}
					break
				}
			}
			uistate.ConfirmModal(uistate.T("budgets.deleteConfirm", name), true, func(ok bool) {
				if !ok {
					return
				}
				if err := app.DeleteBudget(budgetID); err != nil {
					uistate.PostNotice(err.Error(), true)
					return
				}
				uistate.BumpDataRevision()
			})
		},
		OnRemoveRecurring: func(budgetID string) {
			name := uistate.T("budgets.thisBudget")
			for _, b := range app.Budgets() {
				if b.ID == budgetID {
					if n := catName[b.CategoryID]; n != "" {
						name = n
					}
					break
				}
			}
			uistate.ConfirmModal(uistate.T("budgets.removeRecurringConfirm", name), true, func(ok bool) {
				if !ok {
					return
				}
				for _, b := range app.Budgets() {
					if b.ID != budgetID {
						continue
					}
					b.RecurringCover = nil
					if err := app.PutBudget(b); err != nil {
						uistate.PostNotice(err.Error(), true)
						return
					}
					break
				}
				uistate.BumpDataRevision()
			})
		},
	}
}

// --- shared row props + formatting helpers (used by BudgetRow + the add form) ------

type budgetRowProps struct {
	Status            budgeting.Status
	Category          string
	TrackedCats       string // comma-joined category names for a multi-category budget; "" for single
	Members           []domain.Member
	BudgetDefs        []customfields.Def    // custom-field defs for the "budget" entity (display + inline edit)
	Envelope          string                // formatted envelope balance (envelope methodology); "" hides the line
	EnvelopeNeg       bool                  // envelope is overdrawn → danger tone
	PaceOver          string                // formatted projected overspend (pace, in-progress only); "" hides the line
	RolloverCarry     string                // formatted previous-period carry for per-budget rollover; "" hides the line
	RolloverNeg       bool                  // previous-period carry is negative → danger tone
	EffectiveCap      string                // C136: formatted effective cap for this period on rollover budgets; "" = not rollover
	ProratedRest      string                // C143: formatted even-pace amount left for the rest of the period; "" hides the line
	EffectiveMethod   budgeting.Methodology // C118: this budget's resolved method (own override or global fallback)
	Covered           bool                  // received one-time cover money this period
	LastMonthSpent    string                // "Last month's spend" overlay: last period's actual spend; "" hides the row
	LastMonthDelta    string                // short "$X under" / "$X over" vs this month's budget
	LastMonthOver     bool                  // last month's spend exceeded this month's budget → danger tone
	LastMonthPct      int                   // last month's spend as % of this month's budget (uncapped) — the "%" figure
	LastMonthFill     int                   // same, clamped 0..100 — the bar width
	OnDelete          func(string)
	OnRemoveRecurring func(string)               // clear this budget's recurring cover (confirmed)
	OnDrill           func(categoryIDs []string) // open Transactions filtered to this budget's tracked categories (all of them, for a multi-category budget)
}

// budgetLeftValue formats a budget's remaining amount for the summary "Left" stat
// (C124): a positive remaining shows plainly ("$50.00"), but an overspend reads as
// "$50.00 over" instead of the ambiguous accounting parens "($50.00)".
func budgetLeftValue(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m)
}

// budgetRemainPhrase is budgetLeftValue plus the trailing "left"/"over" word, for the
// per-row summary line ("$50.00 left" / "$50.00 over"). (C124)
func budgetRemainPhrase(m money.Money) string {
	if m.IsNegative() {
		return fmtMoney(m.Abs()) + " " + uistate.T("budgets.overWord")
	}
	return fmtMoney(m) + " " + uistate.T("budgets.leftWord")
}

// budgetTitle renders a budget's display title: its name, or its category when
// unnamed, or "name · category" when both add information (never "Food · Food").
func budgetTitle(name, category string) string {
	switch {
	case name == "":
		return category
	case category != "" && !strings.EqualFold(category, name):
		return name + " · " + category
	default:
		return name
	}
}

// periodLabel localizes a budget period for the UI (the domain layer's Period.Label()
// is hardcoded English by design — domain stays i18n-free). C116.
func periodLabel(p domain.Period) string {
	switch p {
	case domain.PeriodWeekly:
		return uistate.T("budgets.periodWeekly")
	case domain.PeriodBiweekly:
		return uistate.T("budgets.periodBiweekly")
	case domain.PeriodSemimonthly:
		return uistate.T("budgets.periodSemimonthly")
	case domain.PeriodQuarterly:
		return uistate.T("budgets.periodQuarterly")
	case domain.PeriodYearly:
		return uistate.T("budgets.periodYearly")
	default:
		return uistate.T("budgets.periodMonthly")
	}
}

func periodOptions(selected string) []uiw.SelectOption {
	opts := make([]uiw.SelectOption, 0, len(domain.AllPeriods))
	for _, p := range domain.AllPeriods {
		opts = append(opts, uiw.SelectOption{Value: string(p), Label: periodLabel(p)})
	}
	return opts
}

// ownerSelectOptions builds owner SelectOptions (the shared group plus each member) —
// used wherever an entity's owner can be chosen.
func ownerSelectOptions(members []domain.Member, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: domain.GroupOwnerID, Label: uistate.T("owner.group")}}
	for _, m := range members {
		opts = append(opts, uiw.SelectOption{Value: m.ID, Label: m.Name})
	}
	return opts
}

func checkedAttr(checked bool) []any {
	// Use the Checked() boolean prop (sets the DOM `checked` PROPERTY), not the
	// `checked` content attribute — an attribute only seeds defaultChecked and doesn't
	// update the live property on a keyed re-render, so the tick wouldn't show/clear.
	return []any{Checked(checked)}
}
