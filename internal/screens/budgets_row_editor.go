// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// budgetRowLimitDraftResult carries the explicit results for useBudgetRowLimitDraft.
type budgetRowLimitDraftResult struct {
	limitEditing      ui.State[bool]
	limitRestoreFocus ui.State[bool]
	limitDraft        ui.State[string]
	limitInputRef     ui.DOMRef
	limitDec          int
	limitErrID        string
	limitErrMsg       string
	limitSaveTitle    string
	startLimitEdit    ui.Handler
	cancelLimitEdit   ui.Handler
}

// useBudgetRowLimitDraft owns the inline base-limit draft and edit/cancel state.
func useBudgetRowLimitDraft(s budgeting.Status) budgetRowLimitDraftResult {
	// G1: inline limit editing — clicking the limit figure swaps it for a compact
	// number input (Enter/✓ saves, ✕ cancels), so the single most frequent budget
	// edit never costs a modal round-trip. Edits the BASE limit; the effective-cap
	// line explains any carry/boost delta.
	limitEditing := ui.UseState(false)
	limitRestoreFocus := ui.UseState(false)
	limitDraft := ui.UseState("")
	limitInputRef := ui.UseDOMRef()
	limitDec := currency.Decimals(s.Budget.Limit.Currency)
	// C665: the field advertises min="0.01" and the browser's own validation does
	// nothing for it, because the value is read out of state on submit rather than
	// through form validity. Typing -1 left ✓ live, and pressing it silently threw
	// the edit away — a negative limit is refused, but so is any feedback that it
	// happened. The draft is now validated on every keystroke: ✓ goes dead, the
	// reason is stated inline, and the editor stays open with the text intact.
	_, limitProblem := budgeting.ParseLimitMajor(limitDraft.Get(), limitDec)
	limitErrID := "budget-limit-err-" + s.Budget.ID
	limitErrMsg := budgetLimitError(limitProblem, s.Budget.Limit.Currency)
	limitSaveTitle := uistate.T("action.save")
	if limitErrMsg != "" {
		limitSaveTitle = uistate.T("budgets.limitSaveBlocked")
	}
	ui.UseEffect(func() func() {
		if !limitEditing.Get() {
			return nil
		}
		limitInputRef.Focus()
		// The framework ref owns focus after the editor mounts. Selecting the
		// focused input preserves the existing quick-replace keyboard workflow.
		if doc := js.Global().Get("document"); doc.Truthy() {
			if active := doc.Get("activeElement"); active.Truthy() {
				active.Call("select")
			}
		}
		return nil
	}, limitEditing.Get())
	startLimitEdit := ui.UseEvent(Prevent(func() {
		// Seed from the STORED base limit — s.Budget here is the evaluation copy whose
		// Limit has rollover carry / period boosts folded in (the effective cap). If the
		// draft seeded from that, an untouched save would silently rewrite the base
		// limit to the cap, destroying the carry distinction. Look up the real budget.
		seed := s.Budget.Limit
		if app := appstate.Default; app != nil {
			for _, bb := range app.Budgets() {
				if bb.ID == s.Budget.ID {
					seed = bb.Limit
					break
				}
			}
		}
		limitDraft.Set(money.FormatMinor(seed.Amount, limitDec))
		limitEditing.Set(true)
	}))
	cancelLimitEdit := ui.UseEvent(Prevent(func() {
		limitRestoreFocus.Set(true)
		limitEditing.Set(false)
	}))
	return budgetRowLimitDraftResult{
		limitEditing: limitEditing, limitRestoreFocus: limitRestoreFocus, limitDraft: limitDraft, limitInputRef: limitInputRef, limitDec: limitDec,
		limitErrID: limitErrID, limitErrMsg: limitErrMsg, limitSaveTitle: limitSaveTitle, startLimitEdit: startLimitEdit,
		cancelLimitEdit: cancelLimitEdit,
	}
}

// budgetRowLimitKeyboardProps carries the explicit inputs for useBudgetRowLimitKeyboard.
type budgetRowLimitKeyboardProps struct {
	limitRestoreFocus ui.State[bool]
	limitEditing      ui.State[bool]
	limitDraft        ui.State[string]
	limitDec          int
	s                 budgeting.Status
}

// useBudgetRowLimitKeyboard registers the inline editor keyboard and input handlers.
func useBudgetRowLimitKeyboard(p budgetRowLimitKeyboardProps) (ui.Handler, ui.Handler) {
	limitKeyDown := ui.UseEvent(func(e ui.KeyboardEvent) {
		switch e.GetKey() {
		case "Escape":
			e.PreventDefault()
			p.limitRestoreFocus.Set(true)
			p.limitEditing.Set(false)
		case "Enter":
			e.PreventDefault()
			// The form submit owns validation and persistence.
			amt, prob := budgeting.ParseLimitMajor(p.limitDraft.Get(), p.limitDec)
			if prob.OK() {
				p.limitRestoreFocus.Set(true)
				// Invoke the same save path as the submit button.
				if app := appstate.Default; app != nil {
					for _, bb := range app.Budgets() {
						if bb.ID == p.s.Budget.ID {
							if bb.Limit.Amount != amt {
								bb.Limit = money.New(amt, bb.Limit.Currency)
								if err := app.PutBudget(bb); err == nil {
									uistate.PostUndoable(uistate.T("budgets.limitChangedToast", fmtMoney(bb.Limit)))
									uistate.BumpDataRevision()
								} else {
									uistate.PostNotice(budgetErrorText(err), true)
									return
								}
							}
							break
						}
					}
				}
				if p.limitEditing.Get() {
					p.limitEditing.Set(false)
				}
			}
		}
	})
	onLimitDraft := ui.UseEvent(func(v string) { p.limitDraft.Set(v) })

	return limitKeyDown, onLimitDraft
}

// budgetRowLimitSaveProps carries the explicit inputs for useBudgetRowLimitSave.
type budgetRowLimitSaveProps struct {
	limitDraft        ui.State[string]
	limitDec          int
	s                 budgeting.Status
	limitEditing      ui.State[bool]
	limitRestoreFocus ui.State[bool]
}

// useBudgetRowLimitSave registers the submit handler and focus-restoration effect.
func useBudgetRowLimitSave(p budgetRowLimitSaveProps) ui.Handler {
	saveLimitEdit := ui.UseEvent(Prevent(func() {
		amt, prob := budgeting.ParseLimitMajor(p.limitDraft.Get(), p.limitDec)
		if !prob.OK() {
			// Hold the editor open on a bad value. Closing it discarded the typed
			// text and left the old figure back on the card as though nothing had
			// been attempted — the failure mode that made a rejected negative
			// indistinguishable from a saved one.
			return
		}
		if app := appstate.Default; app != nil {
			for _, bb := range app.Budgets() {
				if bb.ID != p.s.Budget.ID {
					continue
				}
				if bb.Limit.Amount != amt {
					bb.Limit = money.New(amt, bb.Limit.Currency)
					if err := app.PutBudget(bb); err == nil {
						uistate.PostUndoable(uistate.T("budgets.limitChangedToast", fmtMoney(bb.Limit)))
						uistate.BumpDataRevision()
					} else {
						uistate.PostNotice(budgetErrorText(err), true)
						return
					}
				}
				break
			}
		}
		p.limitEditing.Set(false)
		p.limitRestoreFocus.Set(true)
	}))
	ui.UseEffect(func() func() {
		if !p.limitRestoreFocus.Get() {
			return nil
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any {
			doc := js.Global().Get("document")
			if doc.Truthy() {
				button := doc.Call("querySelector", `[data-testid="budget-limit-btn-`+p.s.Budget.ID+`"]`)
				if button.Truthy() {
					button.Call("focus")
				}
			}
			p.limitRestoreFocus.Set(false)
			return nil
		})
		handle := js.Global().Call("setTimeout", cb, 0)
		return func() {
			js.Global().Call("clearTimeout", handle)
			cb.Release()
		}
	}, p.limitRestoreFocus.Get())

	return saveLimitEdit
}
