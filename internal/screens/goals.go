// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/cashflow"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// goalView is the shared computed picture of the goals surface — the owner-scoped,
// partitioned, sorted goal lists plus the headline totals. Both the summary and the
// list tiles compute it, mirroring computeBudgetView on /budgets.
type goalView struct {
	Base        string
	Accounts    []domain.Account
	Members     []domain.Member
	Categories  []domain.Category
	Tasks       []domain.Task // for checklist-goal progress (linked to-do counts)
	All         []domain.Goal // owner-visible goals (all sections)
	Active      []domain.Goal // non-archived, non-fund, not missed; sorted most-actionable first
	Missed      []domain.Goal // dated goals whose deadline passed unreached (Classify=missed); longest-missed first
	Fund        []domain.Goal // sinking funds; alphabetical
	Achieved    []domain.Goal // archived; alphabetical
	SavedTotal  money.Money   // Σ saved across active goals (base currency)
	TargetTotal money.Money   // Σ target across active goals (base currency)
	OverallPct  int           // combined saved/target percent across active goals
	// Health is the shared pace verdict per goal id (On track / Watch / At risk) plus
	// the figures behind it, computed from required-contribution vs. available monthly
	// cash — the SAME model the Smart assistant uses, so the card badge and Smart never
	// contradict. Only deadlined, fundable goals appear; absent = HealthNone (no badge).
	Health map[string]goalPace
}

// goalViewCache memoizes computeGoalView by store revision + active member (the only
// input beyond stored data), so the goals surface's tiles share one aggregation.
var goalViewCache = map[string]goalView{}

// computeGoalView assembles the goalView for the active member view (memoized).
func computeGoalView(app *appstate.App, activeMemberID string) goalView {
	return memoByRev(goalViewCache, revKey(app)+"|"+activeMemberID, func() goalView {
		return computeGoalViewRaw(app, activeMemberID)
	})
}

// computeGoalViewRaw assembles the goalView for the active member view: owner-scoped,
// partitioned into active / sinking-fund / achieved, sorted, with the headline totals.
// Pure aggregation over the internal/goals package (no hooks, no mutation).
func computeGoalViewRaw(app *appstate.App, activeMemberID string) goalView {
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	v := goalView{Base: base, Accounts: app.Accounts(), Members: app.Members(), Categories: app.Categories(), Tasks: app.Tasks()}
	for _, g := range app.Goals() {
		if ownerVisibleTo(g.OwnerID, activeMemberID) {
			v.All = append(v.All, g)
		}
	}
	now := time.Now()
	for _, g := range v.All {
		switch {
		case g.Archived:
			v.Achieved = append(v.Achieved, g)
		case g.IsSinkingFund:
			v.Fund = append(v.Fund, g)
		case goalsvc.Classify(g, v.Tasks, now) == goalsvc.StateMissed:
			// A dated goal whose deadline passed unreached gets its own section — the
			// dashboard widget counts these, so the page must be able to SHOW them.
			v.Missed = append(v.Missed, g)
		default:
			v.Active = append(v.Active, g)
		}
	}
	// Active: most actionable first (nearest date → highest %); missed: longest-missed
	// first (the most overdue decision leads); funds/achieved: alpha.
	sort.SliceStable(v.Active, func(i, j int) bool { return goalsvc.LessForList(v.Active[i], v.Active[j]) })
	sort.SliceStable(v.Missed, func(i, j int) bool { return v.Missed[i].TargetDate.Before(v.Missed[j].TargetDate) })
	sort.SliceStable(v.Fund, func(i, j int) bool { return v.Fund[i].Name < v.Fund[j].Name })
	sort.SliceStable(v.Achieved, func(i, j int) bool { return v.Achieved[i].Name < v.Achieved[j].Name })

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	// Headline totals span active + missed (a missed goal is still money in flight —
	// sectioning it must not shrink the summary figures).
	inFlight := append(append([]domain.Goal(nil), v.Active...), v.Missed...)
	v.SavedTotal, v.TargetTotal = goalsvc.Totals(inFlight, rates, base, false)
	v.OverallPct, _ = goalsvc.OverallProgress(inFlight, false)
	v.Health = computeGoalHealth(app, v.All, base, rates, now)
	return v
}

// goalPace is the shared pace verdict PLUS the figures that justify it, so the goal
// card can show WHY it's Watch/At risk (required monthly, the goal's fair share of the
// household's free cash), not just a decorative badge. All money is base-currency
// minor units. RequiredMinor is MonthlyNeeded; FairMinor is the goal's fair share of
// SurplusMinor across the deadlined goals — the "available-money constraint".
type goalPace struct {
	Health        goalsvc.Health
	RequiredMinor int64
	FairMinor     int64
	SurplusMinor  int64
}

// computeGoalHealth derives the shared pace verdict (On track / Watch / At risk) for
// every deadlined, fundable goal, using the SAME inputs the Smart assistant uses: the
// household's free monthly cash (one shared cashflow figure) split as a fair share
// across the deadlined goals, against each goal's required monthly contribution. A
// goal within its fair share is On track; one needing more than its share is a stretch
// (Watch); one that can't be met even with all the free cash is At risk. Goals with no
// deadline / already covered are absent (no pace claim — the card shows no badge). Each
// entry also carries the figures behind the verdict so the card can explain it.
func computeGoalHealth(app *appstate.App, all []domain.Goal, base string, rates currency.Rates, now time.Time) map[string]goalPace {
	toBase := func(m money.Money) int64 {
		if m.Currency == base || m.Currency == "" {
			return m.Amount
		}
		if v, err := currency.ConvertBetween(m.Amount, m.Currency, base, rates); err == nil {
			return v
		}
		return 0
	}
	surplus := cashflow.TrailingMonthlySurplus(app.Transactions(), rates, base, now, cashflow.DefaultTrailingMonths)
	type need struct {
		id  string
		req int64
	}
	var needs []need
	for _, g := range all {
		if g.Archived {
			continue
		}
		req, ok, err := goalsvc.MonthlyNeeded(g, now)
		if err != nil || !ok {
			continue
		}
		needs = append(needs, need{id: g.ID, req: toBase(req)})
	}
	fair := int64(0)
	if surplus > 0 && len(needs) > 0 {
		fair = surplus / int64(len(needs))
	}
	out := make(map[string]goalPace, len(needs))
	for _, n := range needs {
		if h := goalsvc.AssessHealth(n.req, surplus, len(needs)); h != goalsvc.HealthNone {
			out[n.id] = goalPace{Health: h, RequiredMinor: n.req, FairMinor: fair, SurplusMinor: surplus}
		}
	}
	return out
}

// overbookedGoals returns the set of goal IDs whose virtual earmarks no longer fit the
// live account balances backing them — i.e. an account they reserve from has been spent
// down so the total earmarked against it now exceeds its current balance. Computed once
// per render (all figures normalised to base currency) so a card never trusts a stale
// reservation. Goals that don't earmark are absent from the map (false).
func overbookedGoals(app *appstate.App) map[string]bool {
	out := map[string]bool{}
	if app == nil {
		return out
	}
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	toBase := func(m money.Money) int64 {
		if m.Currency == base || m.Currency == "" {
			return m.Amount
		}
		if conv, err := rates.Convert(m, base); err == nil {
			return conv.Amount
		}
		return m.Amount
	}
	goalsList := app.Goals()
	// Total earmarked against each account (base minor units), across all goals.
	earmark := map[string]int64{}
	for _, g := range goalsList {
		for _, al := range g.Allocations {
			if al.AccountID != "" {
				earmark[al.AccountID] += toBase(al.Amount)
			}
		}
	}
	// An account is over-earmarked when its current balance can't back the reservations.
	txns := app.Transactions()
	overAcct := map[string]bool{}
	for _, a := range app.Accounts() {
		if earmark[a.ID] <= 0 {
			continue
		}
		bal, _ := ledger.Balance(a, txns)
		if earmark[a.ID] > toBase(bal) {
			overAcct[a.ID] = true
		}
	}
	for _, g := range goalsList {
		for _, al := range g.Allocations {
			if overAcct[al.AccountID] {
				out[g.ID] = true
				break
			}
		}
	}
	return out
}

// sortGoals re-orders the active-goals list in place for the toolbar's Sort picker.
// It is kind-agnostic — percent, step counts and deadlines are read the same way the
// cards render them (EvaluateProgress / TaskCounts) so the on-screen order matches the
// figures. Ties fall back to LessForList then name for a stable, sensible result.
func sortGoals(gs []domain.Goal, key string, tasks []domain.Task, now time.Time) {
	pct := func(g domain.Goal) int { return goalsvc.EvaluateProgress(g, tasks, now).Percent }
	steps := func(g domain.Goal) int { _, total := goalsvc.TaskCounts(tasks, g.ID); return total }
	switch key {
	case uistate.GoalSortClosest:
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := pct(gs[i]), pct(gs[j]); pi != pj {
				return pi > pj // nearly-there first
			}
			return goalsvc.LessForList(gs[i], gs[j])
		})
	case uistate.GoalSortFarthest:
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := pct(gs[i]), pct(gs[j]); pi != pj {
				return pi < pj // just-getting-started first
			}
			return goalsvc.LessForList(gs[i], gs[j])
		})
	case uistate.GoalSortComplexity:
		sort.SliceStable(gs, func(i, j int) bool {
			if si, sj := steps(gs[i]), steps(gs[j]); si != sj {
				return si > sj // most steps to work through first
			}
			return goalsvc.LessForList(gs[i], gs[j])
		})
	case uistate.GoalSortDeadline:
		sort.SliceStable(gs, func(i, j int) bool {
			ai, aj := gs[i].TargetDate.IsZero(), gs[j].TargetDate.IsZero()
			if ai != aj {
				return !ai // dated goals before undated
			}
			if !ai && !aj && !gs[i].TargetDate.Equal(gs[j].TargetDate) {
				return gs[i].TargetDate.Before(gs[j].TargetDate)
			}
			return gs[i].Name < gs[j].Name
		})
	case uistate.GoalSortName:
		sort.SliceStable(gs, func(i, j int) bool { return gs[i].Name < gs[j].Name })
	case uistate.GoalSortPriority:
		sort.SliceStable(gs, func(i, j int) bool {
			if pi, pj := gs[i].PriorityRank(), gs[j].PriorityRank(); pi != pj {
				return pi < pj // high (1) first; unprioritized last
			}
			return goalsvc.LessForList(gs[i], gs[j])
		})
	default: // GoalSortActionable
		sort.SliceStable(gs, func(i, j int) bool { return goalsvc.LessForList(gs[i], gs[j]) })
	}
}

type goalRowProps struct {
	Goal               domain.Goal
	Accounts           []domain.Account
	Members            []domain.Member
	Tasks              []domain.Task // linked-to-do source for checklist-goal progress
	OnDelete           func(string)
	OnDrillAccount     func(accountID string)        // open Transactions filtered to the linked account
	OnArchive          func(id string, archive bool) // move goal to/from the Achieved section
	OnRedirect         func()                        // a completed goal frees its monthly — jump to Allocate (L20)
	OnUndoContribution func(string)                  // reverse the goal's most recent contribution
	OnResetGoal        func(string)                  // reset the goal's saved progress to zero
	// C189/C192: sinking-fund display data (zero values = not a fund / no link).
	// FundSetAside is the monthly set-aside in minor units (from FundSetAsideMinor).
	// LinkedCategoryName is the resolved name of CategoryID (empty when unlinked).
	FundSetAside       int64
	LinkedCategoryName string
	// EarmarkOverbooked is true when this goal earmarks from an account whose CURRENT
	// balance no longer covers the total earmarked against it (e.g. the account was spent
	// down after the earmark) — the card flags the stale reservation instead of trusting it.
	EarmarkOverbooked bool
	// Health is the shared pace verdict (On track / Watch / At risk) plus the figures
	// behind it (required monthly, fair-share of slack) — the same model the Smart
	// assistant uses. A zero Health.Health means no verdict (no pace badge / reason).
	Health goalPace
	// Base is the household base currency, used to format the Health figures (they are
	// base-currency minor units).
	Base string
}

// goalKindOptions builds the goal-type SelectOptions (savings / checklist /
// milestone / habit) for the add & edit forms.
func goalKindOptions() []uiw.SelectOption {
	return []uiw.SelectOption{
		{Value: string(domain.GoalKindFinancial), Label: uistate.T("goals.kindFinancial")},
		{Value: string(domain.GoalKindChecklist), Label: uistate.T("goals.kindChecklist")},
		{Value: string(domain.GoalKindMilestone), Label: uistate.T("goals.kindMilestone")},
		{Value: string(domain.GoalKindHabit), Label: uistate.T("goals.kindHabit")},
	}
}

// goalKindHint returns the one-line explainer shown under the goal-type picker.
func goalKindHint(k domain.GoalKind) string {
	switch k {
	case domain.GoalKindChecklist:
		return uistate.T("goals.kindChecklistHint")
	case domain.GoalKindMilestone:
		return uistate.T("goals.kindMilestoneHint")
	case domain.GoalKindHabit:
		return uistate.T("goals.kindHabitHint")
	default:
		return uistate.T("goals.kindFinancialHint")
	}
}

// habitCadenceOptions builds the check-in rhythm SelectOptions for a habit goal.
func habitCadenceOptions() []uiw.SelectOption {
	cs := []domain.RecurringCadence{domain.CadenceWeekly, domain.CadenceBiweekly, domain.CadenceMonthly, domain.CadenceQuarterly, domain.CadenceYearly}
	out := make([]uiw.SelectOption, 0, len(cs))
	for _, c := range cs {
		out = append(out, uiw.SelectOption{Value: string(c), Label: cadenceLabel(c)})
	}
	return out
}

// goalAccountOptions builds the linked-account SelectOptions for a goal, with a
// leading "no link" choice. C7: only LIQUID cash accounts are offered (the same
// eligibility the earmark picker uses) — a goal's money doesn't live in a mortgage
// or a 401(k). An already-linked ineligible account is grandfathered so an existing
// choice never silently vanishes from its own picker.
func goalAccountOptions(accounts []domain.Account, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noLink")}}
	for _, a := range accounts {
		if a.Archived && a.ID != selected {
			continue
		}
		if !earmarkEligibleType(a.Type) && a.ID != selected {
			continue
		}
		opts = append(opts, uiw.SelectOption{Value: a.ID, Label: a.Name})
	}
	return opts
}

// goalCategoryOptions builds the linked-category SelectOptions for a goal/fund,
// with a leading "no link" choice. Used by goaladdform and the inline edit (C192).
func goalCategoryOptions(categories []domain.Category, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noCategoryLink")}}
	for _, c := range categories {
		opts = append(opts, uiw.SelectOption{Value: c.ID, Label: c.Name})
	}
	return opts
}

// accountName returns an account's name by id, or "" when not found.
func accountName(accounts []domain.Account, id string) string {
	if a, ok := domain.AccountByID(accounts, id); ok {
		return a.Name
	}
	return ""
}

// categoryNameByID returns a category's name by id, or "" when not found.
func categoryNameByID(categories []domain.Category, id string) string {
	for _, c := range categories {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// csvImportSummary builds the confirmation message after a CSV import, naming the
// destination account when one is known (C10): "Imported N transactions into
// <Account>." Falls back to the account-less phrasing for an unselected target.
func csvImportSummary(accounts []domain.Account, acctID string, n int) string {
	txns := plural(n, "transaction")
	if name := accountName(accounts, acctID); name != "" {
		return uistate.T("documents.importedCsvInto", txns, name)
	}
	return uistate.T("documents.importedCsv", txns)
}

// barFillStyle is the inline width for a goal's progress bar. The fill *tone* is
// driven by a CSS state class (see goalCardState) so a near-complete, at-risk, or
// on-track goal reads differently at a glance instead of one flat accent (G5/C51).
func barFillStyle(pct int) string {
	return fmt.Sprintf("width:%d%%", pct)
}

// goalPaceBadge renders a goal's pace badge, layering the SHARED health verdict
// (On track / Watch / At risk — the same one the Smart assistant computes from
// required-contribution vs. available cash) on top of the calendar signals so the
// badge is honest. A goal reads "On track" ONLY when its funding actually keeps pace;
// one the assistant would flag as tight reads "Watch" or "At risk" here instead of the
// old false "On track"; and a goal with no fundable verdict shows no badge rather than
// an unearned reassurance. Priority is most-actionable first: overdue and near-done
// calendar states win, then the money verdict, then due-soon, then on-track.
func goalPaceBadge(p goalsvc.Pace, h goalsvc.Health) ui.Node {
	var label, mod string
	switch {
	case p == goalsvc.PaceComplete:
		return Fragment()
	case p == goalsvc.PaceOverdue:
		label, mod = uistate.T("goals.paceOverdue"), "overdue"
	case p == goalsvc.PaceFinalStretch:
		label, mod = uistate.T("goals.paceFinal"), "final"
	case h == goalsvc.HealthAtRisk:
		label, mod = uistate.T("goals.paceAtRisk"), "atrisk"
	case h == goalsvc.HealthWatch:
		label, mod = uistate.T("goals.paceWatch"), "watch"
	case p == goalsvc.PaceDueSoon:
		label, mod = uistate.T("goals.paceDueSoon"), "soon"
	case h == goalsvc.HealthOnTrack:
		label, mod = uistate.T("goals.paceOnTrack"), "ontrack"
	default:
		return Fragment()
	}
	return Span(ClassStr("pace-badge pace-"+mod), label)
}

// goalPaceReason renders the one-line justification behind a goal's pace verdict — the
// monthly contribution the deadline needs and the available-money constraint that
// produced the verdict (the goal's fair share of the household's free cash, or the
// whole slack when At risk). This makes the status diagnostic rather than decorative,
// so "Watch"/"At risk" says WHY. Empty when there's no verdict. base formats the
// figures (they are base-currency minor units).
func goalPaceReason(p goalPace, base, goalID string) ui.Node {
	if base == "" {
		base = "USD"
	}
	req := fmtMoney(money.New(p.RequiredMinor, base))
	var text string
	switch p.Health {
	case goalsvc.HealthAtRisk:
		text = uistate.T("goals.paceReasonAtRisk", req, fmtMoney(money.New(p.SurplusMinor, base)))
	case goalsvc.HealthWatch:
		text = uistate.T("goals.paceReasonWatch", req, fmtMoney(money.New(p.FairMinor, base)))
	case goalsvc.HealthOnTrack:
		text = uistate.T("goals.paceReasonOnTrack", req, fmtMoney(money.New(p.FairMinor, base)))
	default:
		return Fragment()
	}
	return Div(ClassStr("goal-pace-reason goal-pace-reason-"+string(p.Health)),
		Attr("data-testid", "goal-pace-reason-"+goalID), text)
}
