// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
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

var errBudgetTopupCurrencyMismatch = errors.New("funding budgets must use the destination currency")

// planFundedBudgetTopup creates every source and destination update before a
// funded top-up is persisted. Period-only source boosts use that source's own
// period key for the period containing viewAnchor.
func planFundedBudgetTopup(budgets []domain.Budget, shares map[string]int64, destinationID string, amount int64, permanent bool, viewAnchor, payAnchor time.Time, weekStart time.Weekday) ([]domain.Budget, error) {
	byID := make(map[string]domain.Budget, len(budgets))
	for _, budget := range budgets {
		byID[budget.ID] = budget
	}
	destination, ok := byID[destinationID]
	if !ok {
		return nil, fmt.Errorf("destination budget %q not found", destinationID)
	}
	writes := make([]domain.Budget, 0, len(shares)+1)
	for id, share := range shares {
		if share <= 0 {
			continue
		}
		source, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("source budget %q not found", id)
		}
		if source.Limit.Currency != destination.Limit.Currency {
			return nil, errBudgetTopupCurrencyMismatch
		}
		if permanent {
			if share >= source.Limit.Amount {
				return nil, fmt.Errorf("source budget %q would have nothing left", id)
			}
			source.Limit.Amount -= share
		} else {
			key := budgeting.PeriodKeyWithWeekStart(source.Period, viewAnchor, payAnchor, weekStart)
			start, err := time.Parse("2006-01-02", key)
			if err != nil {
				return nil, fmt.Errorf("source budget %q period key: %w", id, err)
			}
			source, err = source.WithPeriodBoostChecked(start, -share)
			if err != nil {
				return nil, fmt.Errorf("source budget %q period boost: %w", id, err)
			}
		}
		writes = append(writes, source)
	}
	if permanent {
		if amount > 0 && destination.Limit.Amount > math.MaxInt64-amount {
			return nil, fmt.Errorf("destination budget limit would overflow")
		}
		destination.Limit.Amount += amount
	} else {
		key := budgeting.PeriodKeyWithWeekStart(destination.Period, viewAnchor, payAnchor, weekStart)
		if destination.PeriodBoosts[key] > math.MaxInt64-amount {
			return nil, fmt.Errorf("destination budget period boost would overflow")
		}
		destinationStart, err := time.Parse(time.DateOnly, key)
		if err != nil {
			return nil, fmt.Errorf("destination budget period key: %w", err)
		}
		destination, err = destination.WithPeriodBoostChecked(destinationStart, amount)
		if err != nil {
			return nil, fmt.Errorf("destination budget period boost: %w", err)
		}
	}
	writes = append(writes, destination)
	return writes, nil
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
		viewAnchor := p.viewAnchor

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
			writes, planErr := planFundedBudgetTopup(p.app.Budgets(), shares, p.props.BudgetID,
				amt, permanent, viewAnchor, p.coverPayAnchor, p.pr.WeekStartWeekday())
			if errors.Is(planErr, errBudgetTopupCurrencyMismatch) {
				p.errS.Set(uistate.T("budgets.coverCurrencyMismatch"))
				return
			}
			if planErr != nil {
				p.errS.Set(budgetErrorText(planErr))
				return
			}
			if err := p.app.PutBudgetsAtomicAt(writes, p.viewAnchor); err != nil {
				p.errS.Set(budgetErrorText(err))
				return
			}
		}

		// Raise the destination. For a permanent FUNDED top-up, CoverBudget already added
		// the amount to dest.Limit, so don't double-count — only the unfunded-permanent and
		// the this-month cases still need to bump the destination here.
		if !funded {
			for _, nb := range p.app.Budgets() {
				if nb.ID != p.props.BudgetID {
					continue
				}
				if permanent {
					if amt > 0 && nb.Limit.Amount > math.MaxInt64-amt {
						p.errS.Set(budgetErrorText(fmt.Errorf("destination budget limit would overflow")))
						return
					}
					nb.Limit = money.New(nb.Limit.Amount+amt, p.cur)
				} else {
					var boostErr error
					periodStart := budgetPeriodKeyStartWithWeekStart(nb.Period, viewAnchor, p.coverPayAnchor, p.pr.WeekStartWeekday())
					nb, boostErr = nb.WithPeriodBoostChecked(periodStart, amt)
					if boostErr != nil {
						p.errS.Set(budgetErrorText(boostErr))
						return
					}
				}
				if err := p.app.PutBudgetAt(nb, p.viewAnchor); err != nil {
					p.errS.Set(budgetErrorText(err))
					return
				}
				break
			}
		}
		uistate.BumpDataRevision()
		// G5: undoable — a mis-sized or mis-funded top-up reverts in one click.
		uistate.PostUndoable(uistate.T("br3dlg.raisedToast", p.b.Name))
		p.done()
	}))

	return submitTopup
}
