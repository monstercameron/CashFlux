// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// goalView is prepared by the native goalview package and shared by page tiles.
type goalView = goalview.View
type goalPace = goalview.Pace

func goalRates(app *appstate.App) currency.Rates {
	if app == nil {
		return currency.Rates{Base: "USD"}
	}
	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: settings.FXRates}
}

// Each model retains the full task/account snapshots, so keep only the current
// context rather than retaining those backing arrays across eight revisions.
var goalViewCache = newViewCache[goalView](1)

var goalFundingCache = newViewCache[goalview.FundingIndex](1)

func goalFunding(app *appstate.App) goalview.FundingIndex {
	return memoByRev(goalFundingCache, revKey(app), func() goalview.FundingIndex {
		settings := app.Settings()
		base := settings.BaseCurrency
		if base == "" {
			base = "USD"
		}
		return goalview.BuildFunding(app.Goals(), app.Accounts(), accountSnapshot(app).Balances,
			currency.Rates{Base: base, Rates: settings.FXRates}, base)
	})
}

// computeGoalView shares one immutable picture per store revision/member/day.
func computeGoalView(app *appstate.App, activeMemberID string) goalView {
	now := time.Now()
	return memoByRev(goalViewCache, revKey(app)+"|"+activeMemberID+"|"+goalview.TimeKey(app.Goals(), now), func() goalView {
		return computeGoalViewAt(app, activeMemberID, now)
	})
}

func computeGoalViewAt(app *appstate.App, activeMemberID string, now time.Time) goalView {
	settings := app.Settings()
	base := settings.BaseCurrency
	if base == "" {
		base = "USD"
	}
	funding := goalFunding(app)
	return goalview.Build(goalview.Input{
		Base: base, MemberID: activeMemberID,
		Rates: currency.Rates{Base: base, Rates: settings.FXRates},
		Goals: app.Goals(), Accounts: app.Accounts(), Members: app.Members(),
		Categories: app.Categories(), Tasks: app.Tasks(), Transactions: app.Transactions(), Now: now,
		Funding: &funding,
	})
}

// healthNeedsPlan reports whether a goal's shared pace verdict lands it in the
// "Needs a plan" lead section: Watch (a stretch — needs more than a fair share
// of free cash) or At risk (unreachable even with all free cash). On-track and
// no-verdict goals stay in their healthy sections. This ONLY drives sectioning
// — it reads the verdict goalview.Health already produced and never recomputes
// it, so the pace/health model is untouched.
func healthNeedsPlan(h goalsvc.Health) bool {
	return h == goalsvc.HealthWatch || h == goalsvc.HealthAtRisk
}

// needsPlanRank orders the "Needs a plan" section, most severe first (lower =
// earlier): a missed deadline leads — it's already past due, so the decision is
// overdue — then At risk (going to miss), then Watch (a stretch). Ties within a
// rank fall back to the existing most-actionable order (LessForList) at the call
// site. Purely an ordering read of the shared verdict; no pace math here.
func needsPlanRank(missed bool, h goalsvc.Health) int {
	switch {
	case missed:
		return 0
	case h == goalsvc.HealthAtRisk:
		return 1
	case h == goalsvc.HealthWatch:
		return 2
	default:
		return 3
	}
}

// overbookedGoals shares the view's reservation check across the list and drawers.
func overbookedGoals(app *appstate.App) map[string]bool {
	if app == nil {
		return map[string]bool{}
	}
	return goalFunding(app).Overbooked
}

type goalRowProps struct {
	Goal           domain.Goal
	Accounts       []domain.Account
	Members        []domain.Member
	Model          *goalview.RowModel // shared progress and bounded task preview
	OnDelete       func(string)
	OnDrillAccount func(accountID string)        // open Transactions filtered to the linked account
	OnArchive      func(id string, archive bool) // move goal to/from the Achieved section
	// OnRetarget moves a goal's deadline to a date it could actually be met by
	// (C401), so a "Needs a plan" row can be resolved without opening the goal.
	OnRetarget         func(id string, to time.Time)
	OnRedirect         func()       // a completed goal frees its monthly — jump to Allocate (L20)
	OnUndoContribution func(string) // reverse the goal's most recent contribution
	OnResetGoal        func(string) // reset the goal's saved progress to zero
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
	// PlanUrgency grades how far behind a "Needs a plan" card is (Far behind / Slipping
	// / Watch), refining the shared Watch verdict so the group is prioritised. Set ONLY
	// for cards in that section; UrgencyNone (the zero value) elsewhere leaves the normal
	// pace badge untouched.
	PlanUrgency goalsvc.PlanUrgency
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

// goalUrgencyBadge renders the finer "how far behind" grade for a goal in the "Needs a
// plan" section, so a cluster that all shares the Watch verdict no longer reads as one
// undifferentiated block: Far behind (danger), Slipping (warn), Watch (neutral). The
// grade comes from goalsvc.ClassifyPlanUrgency (required-monthly vs. fair share), never
// recomputed here. UrgencyNone renders nothing so callers can fall back to the pace
// badge. goalID gives the badge a stable testid.
func goalUrgencyBadge(u goalsvc.PlanUrgency, goalID string) ui.Node {
	var label, mod string
	switch u {
	case goalsvc.UrgencyFarBehind:
		label, mod = uistate.T("goals.urgencyFarBehind"), "atrisk"
	case goalsvc.UrgencySlipping:
		label, mod = uistate.T("goals.urgencySlipping"), "watch"
	case goalsvc.UrgencyWatch:
		label, mod = uistate.T("goals.urgencyWatch"), "planwatch"
	default:
		return Fragment()
	}
	return Span(ClassStr("pace-badge pace-"+mod), Attr("data-testid", "goal-urgency-"+goalID), label)
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
