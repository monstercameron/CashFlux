// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetEditSaveProps carries the explicit inputs for useBudgetEditSave.
type budgetEditSaveProps struct {
	app            *appstate.App
	done           func()
	props          BudgetEditFormProps
	nameS          ui.State[string]
	ev             entityVarState
	errS           ui.State[string]
	limitS         ui.State[string]
	dec            int
	cur            string
	periodS        ui.State[string]
	ownerS         ui.State[string]
	rolloverS      ui.State[bool]
	rolloverCapS   ui.State[string]
	methodologyS   ui.State[string]
	customEditVals ui.State[map[string]string]
	targetKindS    ui.State[string]
	targetAmtS     ui.State[string]
	targetDateS    ui.State[string]
	linkedGoalS    ui.State[string]
}

// useBudgetEditSave registers full-edit validation and persistence.
func useBudgetEditSave(p budgetEditSaveProps) ui.Handler {
	saveEdit := ui.UseEvent(Prevent(func() {
		if p.app == nil {
			p.done()
			return
		}
		for _, bb := range p.app.Budgets() {
			if bb.ID != p.props.BudgetID {
				continue
			}
			if n := strings.TrimSpace(p.nameS.Get()); n != "" {
				bb.Name = n
			}
			// Refuse a variable name that collides with another budget's handle.
			if warn := entityVarCollision(budgetVarKind, budgetVarEntities(p.app.Budgets()), p.props.BudgetID, p.ev.VarName.Get(), p.nameS.Get()); warn != "" {
				p.errS.Set(warn)
				return
			}
			bb.VarName = strings.TrimSpace(p.ev.VarName.Get())
			// C665: the save boundary refuses the same values the field does, and
			// says which one it refused. "Enter a positive limit" was the only
			// answer for blank, "abc" and -1 alike.
			amt, prob := budgeting.ParseLimitMajor(p.limitS.Get(), p.dec)
			if !prob.OK() {
				p.errS.Set(budgetLimitError(prob, p.cur))
				return
			}
			bb.Limit = money.New(amt, p.cur)
			if p := domain.Period(p.periodS.Get()); p.Valid() {
				bb.Period = p
			}
			bb.OwnerID = p.ownerS.Get()
			if p.ownerS.Get() == domain.GroupOwnerID {
				bb.Scope = domain.ScopeShared
			} else {
				bb.Scope = domain.ScopeIndividual
			}
			bb.Rollover = p.rolloverS.Get()
			// BG5: the rollover surplus cap (0 = no cap). Only meaningful with rollover on,
			// but persisting it regardless keeps a re-enable from losing the prior choice.
			if n, cerr := strconv.Atoi(p.rolloverCapS.Get()); cerr == nil && n >= 0 {
				bb.RolloverCapPeriods = n
			}
			if m := budgeting.Methodology(p.methodologyS.Get()); m.Valid() {
				bb.Methodology = p.methodologyS.Get()
			} else {
				bb.Methodology = ""
			}
			if defs := p.app.CustomFieldDefsFor("budget"); len(defs) > 0 {
				bb.Custom = customValuesToMap(defs, p.customEditVals.Get())
			}
			// BG1: funding target. An invalid/none kind clears every target field so a
			// budget switched back to "no target" doesn't keep a stale amount or date.
			if tk := domain.TargetKind(p.targetKindS.Get()); tk.Valid() && tk != domain.TargetNone {
				bb.TargetKind = tk
				if tamt, terr := money.ParseMinor(strings.TrimSpace(p.targetAmtS.Get()), p.dec); terr == nil && tamt >= 0 {
					bb.TargetAmount = money.New(tamt, p.cur)
				} else {
					bb.TargetAmount = money.Zero(p.cur)
				}
				if tk == domain.TargetByDate {
					if td, derr := time.Parse("2006-01-02", strings.TrimSpace(p.targetDateS.Get())); derr == nil {
						bb.TargetDate = td
					} else {
						bb.TargetDate = time.Time{}
					}
					bb.LinkedGoalID = strings.TrimSpace(p.linkedGoalS.Get())
				} else {
					bb.TargetDate = time.Time{}
					bb.LinkedGoalID = ""
				}
			} else {
				bb.TargetKind = domain.TargetNone
				bb.TargetAmount = money.Zero(p.cur)
				bb.TargetDate = time.Time{}
				bb.LinkedGoalID = ""
			}
			// Tracked categories are managed in their own editor (C1) — this save leaves
			// CategoryID/CategoryIDs untouched.
			if err := p.app.PutBudget(bb); err != nil {
				p.errS.Set(budgetErrorText(err))
				return
			}
			break
		}
		uistate.BumpDataRevision()
		p.done()
	}))

	return saveEdit
}
