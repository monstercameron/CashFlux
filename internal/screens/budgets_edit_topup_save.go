// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditTopupSaveProps carries the explicit inputs for useBudgetEditTopupSave.
type budgetEditTopupSaveProps struct {
	app             *appstate.App
	done            func()
	topupAmt        ui.State[string]
	dec             int
	errS            ui.State[string]
	topupPermanentS ui.State[bool]
	b               domain.Budget
	viewAnchor      time.Time
	coverPayAnchor  time.Time
	pr              prefs.Prefs
	coverSelS       ui.State[map[string]bool]
	coverSrcs       []budgetCoverSource
	cur             string
	props           BudgetEditFormProps
}

// useBudgetEditTopupSave registers funded and unfunded limit increases.
func useBudgetEditTopupSave(p budgetEditTopupSaveProps) ui.Handler {
	submitTopup := ui.UseEvent(Prevent(func() {
		if p.app == nil {
			p.done()
			return
		}
		amt, err := money.ParseMinor(strings.TrimSpace(p.topupAmt.Get()), p.dec)
		if err != nil || amt <= 0 {
			p.errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		permanent := p.topupPermanentS.Get()
		periodStart := budgetPeriodKeyStartWithWeekStart(p.b.Period, p.viewAnchor, p.coverPayAnchor, p.pr.WeekStartWeekday())

		// Optional funding: pull the amount from other budgets (equal split across the
		// checked sources). Without any source, the top-up adds new limit from income.
		sel := p.coverSelS.Get()
		var funded bool
		for _, sc := range p.coverSrcs {
			if sel[sc.ID] {
				funded = true
				break
			}
		}
		if funded {
			weights := map[string]int{}
			for _, sc := range p.coverSrcs {
				if sel[sc.ID] {
					weights[sc.ID] = 1
				}
			}
			shares, shortMinor := splitCoverAmountShort(amt, p.coverSrcs, sel, weights, map[string]bool{})
			// C606: refuse a top-up the chosen sources cannot fund. The this-period
			// path had NO guard at all — it applied a negative period boost per
			// source, so an over-large share silently pushed a source's effective cap
			// below what it had already spent and left it overspent.
			if shortMinor > 0 {
				p.errS.Set(uistate.T("budgets.coverShortBy", fmtMoney(money.New(shortMinor, p.cur))))
				return
			}
			// A permanent move must leave each source with a positive base limit.
			if permanent {
				for _, sc := range p.coverSrcs {
					if share, ok := shares[sc.ID]; ok && share >= sc.LimitMinor {
						p.errS.Set(uistate.T("budgets.coverSourceShort", sc.Label))
						return
					}
				}
			}
			for _, sc := range p.coverSrcs {
				share, ok := shares[sc.ID]
				if !ok || share <= 0 {
					continue
				}
				if permanent {
					// CoverBudget moves limit source→dest permanently (adds to dest.Limit too).
					if err := p.app.CoverBudget(sc.ID, p.props.BudgetID, money.New(share, p.cur)); err != nil {
						p.errS.Set(budgetErrorText(err))
						return
					}
				} else {
					// This period only: reduce the source's effective cap for this period.
					// The error is fatal to the whole operation, not to this source: the
					// destination is raised further down, so continuing past a rejected
					// source write would raise the destination without lowering anybody —
					// which does not fail to move the money, it invents it. The permanent
					// branch above already returned here; this one used to discard.
					for _, nb := range p.app.Budgets() {
						if nb.ID == sc.ID {
							if err := p.app.PutBudget(nb.WithPeriodBoost(periodStart, -share)); err != nil {
								p.errS.Set(budgetErrorText(err))
								return
							}
							break
						}
					}
				}
			}
		}

		// Raise the destination. For a permanent FUNDED top-up, CoverBudget already added
		// the amount to dest.Limit, so don't double-count — only the unfunded-permanent and
		// the this-month cases still need to bump the destination here.
		if !(permanent && funded) {
			for _, nb := range p.app.Budgets() {
				if nb.ID != p.props.BudgetID {
					continue
				}
				if permanent {
					nb.Limit = money.New(nb.Limit.Amount+amt, p.cur)
				} else {
					nb = nb.WithPeriodBoost(periodStart, amt)
				}
				if err := p.app.PutBudget(nb); err != nil {
					p.errS.Set(budgetErrorText(err))
					return
				}
				break
			}
		}
		uistate.BumpDataRevision()
		toastKey := "budgets.toppedUpMonthToast"
		if permanent {
			toastKey = "budgets.toppedUpToast"
		}
		// G5: undoable — a mis-sized or mis-funded top-up reverts in one click.
		uistate.PostUndoable(uistate.T(toastKey, fmtMoney(money.New(amt, p.cur))))
		p.done()
	}))

	return submitTopup
}
