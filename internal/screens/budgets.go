// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/categorytree"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/customfields"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/safespend"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
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
	EffMethod         map[string]budgeting.Methodology // budgetID → resolved method (override or global)
	OverCount         int
	NearCount         int
	TotalSpent        int64
	TotalLimit        int64
	TotalOver         int64
	TotalFundSetAside int64
	BannerIncome      int64 // monthly income (configured or derived) for the assign banner
}

// computeBudgetView runs the full budget evaluation for the active member scope and
// the given view window. It is the single source of truth the /budgets tiles share:
// the same statuses drive the summary totals, the banners, and the rows. Pure (no
// hooks) — the caller resolves the active member, period window, and prefs and passes
// them in, so it can be called from more than one tile without hook-ordering issues.
func computeBudgetView(app *appstate.App, activeMemberID string, vw period.Window, pr prefs.Prefs) budgetView {
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
	for _, b := range budgets {
		bs, be := budgeting.PeriodRangeAnchored(b.Period, anchor, weekStart, payCycleAnchor)
		eval := b
		if b.Rollover {
			ps, pe := budgeting.PreviousPeriodRange(b.Period, anchor, weekStart)
			if prev, perr := budgeting.EvaluateRollup(b, txns, ps, pe, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID)); perr == nil {
				if eff, cerr := budgeting.Carryover(prev.Remaining, b.Limit); cerr == nil {
					eval.Limit = eff
					if eff.Amount != b.Limit.Amount {
						rollEffCap[b.ID] = fmtMoney(eff)
					}
				}
				rollCarry[b.ID] = budgetRemainPhrase(prev.Remaining)
				rollNeg[b.ID] = prev.Remaining.IsNegative()
			}
		}
		st, err := budgeting.EvaluateRollup(eval, txns, bs, be, rates, budgeting.DefaultNearThreshold, categorytree.Descendants(cats, b.CategoryID))
		if err != nil {
			continue
		}
		statuses = append(statuses, st)
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
		if av, err := budgeting.EnvelopeAvailable(b, txns, anchor, weekStart, rates, categorytree.Descendants(cats, b.CategoryID)); err == nil {
			if av.IsNegative() {
				envAvail[b.ID] = fmtMoney(av.Abs()) + " " + uistate.T("budgets.overdrawnWord")
			} else {
				envAvail[b.ID] = fmtMoney(av)
			}
			envNeg[b.ID] = av.IsNegative()
		}
	}

	// Monthly income for the assign banner: prefer the configured figure, fall back to
	// the transaction-derived income (C22).
	ms, me := budgeting.PeriodRange(domain.PeriodMonthly, anchor, weekStart)
	bannerIncome := budgeting.IncomeForBudgets(pr.MonthlyIncomeMinor, txns, ms, me, base, rates)
	if bannerIncome == 0 {
		if raw, _, err := ledger.PeriodTotals(txns, ms, me, rates); err == nil {
			bannerIncome = raw.Amount
		}
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
		ProratedRest: proratedRest, EnvAvail: envAvail, EnvNeg: envNeg, EffMethod: effMethod,
		OverCount: overCount, NearCount: nearCount,
		TotalSpent: totalSpent, TotalLimit: totalLimit, TotalOver: totalOver,
		TotalFundSetAside: totalFundSetAside, BannerIncome: bannerIncome,
	}
}

// budgetRowCallbacks bundles the per-row mutation handlers the list tile hands to each
// BudgetRow. The edit / top-up / cover editors now live in the shell-root flip modal
// (BudgetEditForm) which mutates the store directly, so the row only needs delete.
type budgetRowCallbacks struct {
	OnDelete func(string)
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
	}
}

// --- shared row props + formatting helpers (used by BudgetRow + the add form) ------

type budgetRowProps struct {
	Status          budgeting.Status
	Category        string
	Members         []domain.Member
	BudgetDefs      []customfields.Def    // custom-field defs for the "budget" entity (display + inline edit)
	Envelope        string                // formatted envelope balance (envelope methodology); "" hides the line
	EnvelopeNeg     bool                  // envelope is overdrawn → danger tone
	PaceOver        string                // formatted projected overspend (pace, in-progress only); "" hides the line
	RolloverCarry   string                // formatted previous-period carry for per-budget rollover; "" hides the line
	RolloverNeg     bool                  // previous-period carry is negative → danger tone
	EffectiveCap    string                // C136: formatted effective cap for this period on rollover budgets; "" = not rollover
	ProratedRest    string                // C143: formatted even-pace amount left for the rest of the period; "" hides the line
	EffectiveMethod budgeting.Methodology // C118: this budget's resolved method (own override or global fallback)
	OnDelete        func(string)
	OnDrill         func(categoryID string) // open Transactions filtered to this budget's category
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
	if !checked {
		return nil
	}
	return []any{Attr("checked", "checked")}
}
