// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
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

// budgetEditCoverSaveProps carries the explicit inputs for useBudgetEditCoverSave.
type budgetEditCoverSaveProps struct {
	app             *appstate.App
	done            func()
	amtFxS          ui.State[bool]
	pr              prefs.Prefs
	coverPayAnchor  time.Time
	amtFormulaS     ui.State[string]
	b               domain.Budget
	errS            ui.State[string]
	coverAmtS       ui.State[string]
	dec             int
	coverSrcs       []budgetCoverSource
	coverSelS       ui.State[map[string]bool]
	recurringS      ui.State[bool]
	coverWtS        ui.State[map[string]int]
	wtFxS           ui.State[bool]
	wtFormulaS      ui.State[string]
	coverMaxS       ui.State[map[string]bool]
	cur             string
	props           BudgetEditFormProps
	viewAnchor      time.Time
	coverCustomVals ui.State[map[string]string]
}

// useBudgetEditCoverSave registers the cover validation and persistence handler.
func useBudgetEditCoverSave(p budgetEditCoverSaveProps) ui.Handler {
	submitCover := ui.UseEvent(Prevent(func() {
		if p.app == nil {
			p.done()
			return
		}
		// Resolve the amount: a formula evaluated in this budget's context, else a number.
		var amt int64
		if p.amtFxS.Get() {
			m, ferr := buildCoverContext(p.app, p.pr.WeekStartWeekday(), p.coverPayAnchor).AmountMinor(p.amtFormulaS.Get(), p.b)
			if ferr != nil {
				p.errS.Set(uistate.T("budgets.coverFormulaErr", ferr.Error()))
				return
			}
			amt = m
		} else if m, perr := money.ParseMinor(strings.TrimSpace(p.coverAmtS.Get()), p.dec); perr == nil {
			amt = m
		}
		if amt < 0 {
			amt = 0
		}
		// Sources are required (that's where the money comes from), whether covering now
		// or saving a recurring rule.
		selectedAny := false
		for _, sc := range p.coverSrcs {
			if p.coverSelS.Get()[sc.ID] {
				selectedAny = true
				break
			}
		}
		if !selectedAny {
			p.errS.Set(uistate.T("budgets.coverPickSource"))
			return
		}
		// A one-time cover needs a positive amount; a recurring rule may evaluate to 0
		// this period (nothing to cover yet) and still be saved for future periods.
		if amt == 0 && !p.recurringS.Get() {
			p.errS.Set(uistate.T("budgets.limitRequired"))
			return
		}
		// Resolve the ratio weights once (formula-driven or fixed) so the applied split
		// and the saved recurring shares agree.
		effWeights := p.coverWtS.Get()
		if p.wtFxS.Get() {
			effWeights, _ = resolveCoverWeights(p.app, buildCoverContext(p.app, p.pr.WeekStartWeekday(), p.coverPayAnchor), p.coverSrcs, p.coverSelS.Get(), p.coverWtS.Get(), p.wtFormulaS.Get())
		}
		if amt > 0 {
			shares, shortMinor := splitCoverAmountShort(amt, p.coverSrcs, p.coverSelS.Get(), effWeights, p.coverMaxS.Get())
			// C606: the chosen sources cannot fund the amount. Refuse and say by how
			// much, rather than moving what fits and leaving the user to discover the
			// difference on the cards.
			if shortMinor > 0 {
				p.errS.Set(uistate.T("budgets.coverShortBy", fmtMoney(money.New(shortMinor, p.cur))))
				return
			}
			// Pre-validate so a partial failure can't leave a half-applied cover: each
			// source must keep a positive limit after giving its share.
			for _, sc := range p.coverSrcs {
				if share, ok := shares[sc.ID]; ok && share >= sc.LimitMinor {
					p.errS.Set(uistate.T("budgets.coverSourceShort", sc.Label))
					return
				}
			}
			for _, sc := range p.coverSrcs {
				share, ok := shares[sc.ID]
				if !ok || share <= 0 {
					continue
				}
				if err := p.app.CoverBudget(sc.ID, p.props.BudgetID, money.New(share, p.cur)); err != nil {
					p.errS.Set(budgetErrorText(err))
					return
				}
			}
		}
		// Save / clear the standing recurring arrangement per the toggle (re-fetch the
		// destination since CoverBudget just mutated the budget set).
		for _, nb := range p.app.Budgets() {
			if nb.ID != p.props.BudgetID {
				continue
			}
			if p.recurringS.Get() {
				wtFormula := ""
				if p.wtFxS.Get() {
					wtFormula = strings.TrimSpace(p.wtFormulaS.Get())
				}
				var srcs []domain.CoverShare
				for _, sc := range p.coverSrcs {
					if p.coverSelS.Get()[sc.ID] {
						w := effWeights[sc.ID]
						if w <= 0 {
							w = 1
						}
						// Store the shared per-source formula (re-evaluated each period in
						// that source's context) alongside the last-resolved weight as a record.
						srcs = append(srcs, domain.CoverShare{BudgetID: sc.ID, Weight: w, WeightFormula: wtFormula})
					}
				}
				rc := &domain.RecurringCover{Sources: srcs, LastAppliedPeriod: budgeting.PeriodKeyWithWeekStart(p.b.Period, p.viewAnchor, p.coverPayAnchor, p.pr.WeekStartWeekday())}
				if p.amtFxS.Get() {
					rc.AmountFormula = strings.TrimSpace(p.amtFormulaS.Get())
				}
				rc.AmountMinor = amt // fixed amount, or the last-evaluated value as a record
				if defs := p.app.CustomFieldDefsFor("cover"); len(defs) > 0 {
					rc.Custom = customValuesToMap(defs, p.coverCustomVals.Get())
				}
				nb.RecurringCover = rc
			} else {
				nb.RecurringCover = nil
			}
			// A rejected save here used to fall through to BumpDataRevision and an
			// undoable "saved" toast, so the arrangement was reported as standing when
			// the store had refused it. Reported inline like every other error in this
			// modal — a toast behind an open dialog is easy to miss entirely.
			if err := p.app.PutBudget(nb); err != nil {
				slog.Error("budgets: write rejected", "op", "save recurring cover", "err", err)
				p.errS.Set(budgetErrorText(err))
				return
			}
			break
		}
		uistate.BumpDataRevision()
		// G5: money moves are reversible — the toast offers a one-click Undo backed by
		// the global snapshot stack.
		if amt > 0 {
			uistate.PostUndoable(uistate.T("budgets.coveredUndoToast", fmtMoney(money.New(amt, p.cur))))
		}
		p.done()
	}))

	return submitCover
}
