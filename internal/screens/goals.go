// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
	Active      []domain.Goal // non-archived, non-fund; sorted most-actionable first
	Fund        []domain.Goal // sinking funds; alphabetical
	Achieved    []domain.Goal // archived; alphabetical
	SavedTotal  money.Money   // Σ saved across active goals (base currency)
	TargetTotal money.Money   // Σ target across active goals (base currency)
	OverallPct  int           // combined saved/target percent across active goals
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
	for _, g := range v.All {
		switch {
		case g.Archived:
			v.Achieved = append(v.Achieved, g)
		case g.IsSinkingFund:
			v.Fund = append(v.Fund, g)
		default:
			v.Active = append(v.Active, g)
		}
	}
	// Active: most actionable first (nearest date → highest %); funds/achieved: alpha.
	sort.SliceStable(v.Active, func(i, j int) bool { return goalsvc.LessForList(v.Active[i], v.Active[j]) })
	sort.SliceStable(v.Fund, func(i, j int) bool { return v.Fund[i].Name < v.Fund[j].Name })
	sort.SliceStable(v.Achieved, func(i, j int) bool { return v.Achieved[i].Name < v.Achieved[j].Name })

	rates := currency.Rates{Base: base, Rates: app.Settings().FXRates}
	v.SavedTotal, v.TargetTotal = goalsvc.Totals(v.Active, rates, base, false)
	v.OverallPct, _ = goalsvc.OverallProgress(v.Active, false)
	return v
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
// leading "no link" choice.
func goalAccountOptions(accounts []domain.Account, selected string) []uiw.SelectOption {
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("goals.noLink")}}
	for _, a := range accounts {
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
// driven by a CSS state class (see paceBarClass) so a near-complete, behind, or
// on-track goal reads differently at a glance instead of one flat accent (G5/C51).
func barFillStyle(pct int) string {
	return fmt.Sprintf("width:%d%%", pct)
}

// paceBarClass maps a goal's pace to a progress-bar fill modifier class. The
// classes (final/overdue/soon) are defined in the shared stylesheet; an empty
// modifier keeps the default accent for on-track / undated goals.
func paceBarClass(p goalsvc.Pace) string {
	switch p {
	case goalsvc.PaceComplete:
		return "done"
	case goalsvc.PaceFinalStretch:
		return "final"
	case goalsvc.PaceOverdue:
		return "overdue"
	case goalsvc.PaceDueSoon:
		return "soon"
	default:
		return ""
	}
}

// paceBadge renders a compact colored badge for a goal's pace, or an empty
// fragment when there's nothing to flag (undated, comfortably on track without a
// near-term signal). It answers Aaliyah's "am I on pace?" at a glance (G5).
func paceBadge(p goalsvc.Pace) ui.Node {
	var label, mod string
	switch p {
	case goalsvc.PaceFinalStretch:
		label, mod = uistate.T("goals.paceFinal"), "final"
	case goalsvc.PaceOverdue:
		label, mod = uistate.T("goals.paceOverdue"), "overdue"
	case goalsvc.PaceDueSoon:
		label, mod = uistate.T("goals.paceDueSoon"), "soon"
	case goalsvc.PaceOnTrack:
		label, mod = uistate.T("goals.paceOnTrack"), "ontrack"
	default:
		return Fragment()
	}
	return Span(ClassStr("pace-badge pace-"+mod), label)
}
