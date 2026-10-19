// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// budgetEditDataResult carries the explicit results for useBudgetEditData.
type budgetEditDataResult struct {
	vw                period.Window
	pr                prefs.Prefs
	coverPayAnchor    time.Time
	app               *appstate.App
	done              func()
	b                 domain.Budget
	found             bool
	cur               string
	dec               int
	limitMajor        string
	coverSrcs         []budgetCoverSource
	coverIncomplete   bool
	coverDefaultStr   string
	coverShortfallStr string
	selfStatus        budgeting.Status
	selfFound         bool
	selfEffCap        string
	selfCapMath       string
	viewAnchor        time.Time
}

// useBudgetEditData resolves the editor subscriptions, budget, period, and funding sources.
func useBudgetEditData(props BudgetEditFormProps) budgetEditDataResult {
	// Re-render on data mutations so a stale figure can't linger while open.
	_ = uistate.UseDataRevision().Get()
	// Resolved for the cover editor's source list (hooks stay unconditional).
	activeMemberID := uistate.UseActiveMember().Get()
	vw := uistate.UsePeriod().Get()
	pr := uistate.UsePrefs().Get()
	coverPayAnchor := budgetPayCycleAnchor(pr.PayCycleAnchor)

	app := appstate.Default
	done := props.OnDone
	if done == nil {
		done = func() {}
	}

	// Resolve the budget (before the hooks so useState seeds from it).
	var b domain.Budget
	found := false
	if app != nil {
		for _, bb := range app.Budgets() {
			if bb.ID == props.BudgetID {
				b, found = bb, true
				break
			}
		}
	}
	cur := b.Limit.Currency
	if cur == "" {
		if app != nil {
			cur = app.Settings().BaseCurrency
		}
		if cur == "" {
			cur = budgetBaseCurrency(cur)
		}
	}
	dec := currency.Decimals(cur)
	limitMajor := ""
	if found {
		limitMajor = money.FormatMinor(b.Limit.Amount, dec)
	}

	// Cover-editor data: the over-budget's shortfall (prefills the amount) and every
	// other budget that has limit to give, so the user can spread the cover across
	// several. Computed for every mode so the useState seeds below can read it (cheap;
	// keeps hooks stable).
	var coverSrcs []budgetCoverSource
	coverIncomplete := false
	coverDefaultStr, coverShortfallStr := "", ""
	var selfStatus budgeting.Status // this budget's live status (for the formulas view)
	selfFound := false
	// C669: the effective cap this period and the arithmetic that produced it —
	// the very figures the card shows — so the top-up dialog can name the number it
	// is about to change instead of quoting the base limit under the word
	// "currently" beside a card reading a different one.
	selfEffCap, selfCapMath := "", ""
	// C667: the period the card is reporting on. computeBudgetView anchors every
	// budget to this, so anything that quotes history beside those figures — the
	// quick-fill chips — has to walk back from here, not from the wall clock.
	viewAnchor := budgetViewAnchor(vw, time.Now())
	if app != nil {
		cv := computeBudgetView(app, activeMemberID, vw, pr, false)
		selfEffCap, selfCapMath = cv.RollEffCap[props.BudgetID], cv.RollEffCapMath[props.BudgetID]
		input, inputErr := app.BudgetViewInput(cv.Anchor)
		for _, s := range cv.Statuses {
			if s.Budget.ID == props.BudgetID {
				selfStatus, selfFound = s, true
				sf := budgeting.CoverAmount(s)
				coverShortfallStr = fmtMoney(sf)
				if sf.IsPositive() {
					coverDefaultStr = money.FormatMinor(sf.Amount, currency.Decimals(sf.Currency))
				}
				continue
			}
			if s.Budget.IsSaving() || s.Limit.Amount <= 0 {
				continue // nothing to give
			}
			// C606: what this source can actually give, computed from the SAME
			// model the card shows — the effective limit (rollover carry and
			// boosts folded in), what is left after spending, and the part of that
			// already claimed by this period's recurring charges. The list used to
			// offer the whole limit under the word "available", which is the plan's
			// size, not money.
			if inputErr != nil {
				coverIncomplete = true
				continue
			}
			funds, err := budgeting.CoverSourceFunds(s.Budget, input)
			if err != nil {
				coverIncomplete = true
				continue
			}
			if funds.Free.Amount <= 0 {
				continue // an overspent or exhausted budget has nothing to offer
			}
			coverSrcs = append(coverSrcs, budgetCoverSource{
				ID:             s.Budget.ID,
				Label:          budgetTitle(s.Budget.Name, cv.CatName[s.Budget.CategoryID]),
				RemainMinor:    s.Remaining.Amount,
				LimitMinor:     s.Budget.Limit.Amount,
				MovableMinor:   funds.Free.Amount,
				CommittedMinor: funds.Committed.Amount,
				FreeMinor:      funds.Free.Amount,
			})
		}
	}

	// All hooks unconditionally at stable positions (before any branch/return).
	return budgetEditDataResult{
		vw: vw, pr: pr, coverPayAnchor: coverPayAnchor, app: app, done: done, b: b, found: found, cur: cur, dec: dec,
		limitMajor: limitMajor, coverSrcs: coverSrcs, coverIncomplete: coverIncomplete, coverDefaultStr: coverDefaultStr,
		coverShortfallStr: coverShortfallStr, selfStatus: selfStatus, selfFound: selfFound, selfEffCap: selfEffCap,
		selfCapMath: selfCapMath, viewAnchor: viewAnchor,
	}
}
