// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// budgetRowLimitDraftResult carries the explicit results for useBudgetRowLimitDraft.
type budgetRowLimitDraftResult struct {
	limitEditing      ui.State[bool]
	limitRestoreFocus ui.State[bool]
	limitDraft        ui.State[string]
	limitInputRef     ui.DOMRef
	limitButtonRef    ui.DOMRef
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
	limitButtonRef := ui.UseDOMRef()
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
		// The displayed period can precede later edits to the standing plan.
		seed := s.Base
		if seed.Currency == "" {
			seed = s.Budget.Limit
		}
		limitDraft.Set(money.FormatMinor(seed.Amount, limitDec))
		limitEditing.Set(true)
	}))
	cancelLimitEdit := ui.UseEvent(Prevent(func() {
		limitRestoreFocus.Set(true)
		limitEditing.Set(false)
	}))
	return budgetRowLimitDraftResult{
		limitEditing: limitEditing, limitRestoreFocus: limitRestoreFocus, limitDraft: limitDraft, limitInputRef: limitInputRef, limitButtonRef: limitButtonRef, limitDec: limitDec,
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
	viewedPeriod := uistate.UsePeriod().Get()
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
				queueBudgetLimitSave(p.s, amt, viewedPeriod, p.limitEditing, p.limitRestoreFocus)
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
	limitButtonRef    ui.DOMRef
}

// useBudgetRowLimitSave registers the submit handler and focus-restoration effect.
func useBudgetRowLimitSave(p budgetRowLimitSaveProps) ui.Handler {
	viewedPeriod := uistate.UsePeriod().Get()
	saveLimitEdit := ui.UseEvent(Prevent(func() {
		amt, prob := budgeting.ParseLimitMajor(p.limitDraft.Get(), p.limitDec)
		if !prob.OK() {
			// Hold the editor open on a bad value. Closing it discarded the typed
			// text and left the old figure back on the card as though nothing had
			// been attempted — the failure mode that made a rejected negative
			// indistinguishable from a saved one.
			return
		}
		queueBudgetLimitSave(p.s, amt, viewedPeriod, p.limitEditing, p.limitRestoreFocus)
	}))
	ui.UseEffect(func() func() {
		if !p.limitRestoreFocus.Get() {
			return nil
		}
		p.limitButtonRef.Focus()
		p.limitRestoreFocus.Set(false)
		return nil
	}, p.limitRestoreFocus.Get())

	return saveLimitEdit
}

// queueBudgetLimitSave closes the editor before scheduling the write. The
// store update and its dependent budget calculations then run after the browser
// has had a chance to paint the completed interaction. Success remains withheld
// until RequestDurable confirms the strict persistence commit.
type budgetLimitSaveKey struct {
	app      *appstate.App
	budgetID string
}

type budgetLimitSaveRequest struct {
	amount       int64
	viewedPeriod period.Window
}

// budgetLimitSaveQueue serializes durable writes for one app and budget while
// retaining only the most recent edit made during an in-flight write.
type budgetLimitSaveQueue struct {
	active  bool
	pending *budgetLimitSaveRequest
}

func (q *budgetLimitSaveQueue) enqueue(request budgetLimitSaveRequest) bool {
	if q.active {
		q.pending = &request
		return false
	}
	q.active = true
	return true
}

func (q *budgetLimitSaveQueue) complete() (budgetLimitSaveRequest, bool) {
	if q.pending != nil {
		request := *q.pending
		q.pending = nil
		return request, true
	}
	q.active = false
	return budgetLimitSaveRequest{}, false
}

var budgetLimitSaveInFlight = map[budgetLimitSaveKey]*budgetLimitSaveQueue{}

func queueBudgetLimitSave(s budgeting.Status, amount int64, viewedPeriod period.Window, editing, restoreFocus ui.State[bool]) {
	app := appstate.Default
	if app == nil {
		return
	}
	key := budgetLimitSaveKey{app: app, budgetID: s.Budget.ID}
	request := budgetLimitSaveRequest{amount: amount, viewedPeriod: viewedPeriod}
	if queue := budgetLimitSaveInFlight[key]; queue != nil {
		queue.enqueue(request)
		editing.Set(false)
		restoreFocus.Set(true)
		return
	}
	queue := &budgetLimitSaveQueue{}
	queue.enqueue(request)
	budgetLimitSaveInFlight[key] = queue
	startBudgetLimitSave(key, request, &editing, &restoreFocus)
}

func startBudgetLimitSave(key budgetLimitSaveKey, request budgetLimitSaveRequest, editing, restoreFocus *ui.State[bool]) {
	app := key.app
	var budget domain.Budget
	found := false
	for _, candidate := range app.Budgets() {
		if candidate.ID == key.budgetID {
			budget, found = candidate, true
			break
		}
	}
	if !found {
		completeBudgetLimitSave(key)
		return
	}
	priorBase, err := budgeting.BudgetBaseAt(budget, budgetViewAnchor(request.viewedPeriod, budgetNow()))
	if err != nil {
		uistate.PostNotice(budgetErrorText(err), true)
		completeBudgetLimitSave(key)
		return
	}
	if priorBase.Amount == request.amount {
		if editing != nil && restoreFocus != nil {
			editing.Set(false)
			restoreFocus.Set(true)
		}
		completeBudgetLimitSave(key)
		return
	}

	budget.Limit = money.New(request.amount, budget.Limit.Currency)
	if err := app.PutBudgetAt(budget, budgetViewAnchor(request.viewedPeriod, budgetNow())); err != nil {
		uistate.PostNotice(budgetErrorText(err), true)
		completeBudgetLimitSave(key)
		return
	}
	if editing != nil && restoreFocus != nil {
		editing.Set(false)
		restoreFocus.Set(true)
	}

	var slowTimer js.Func
	saveFinished := false
	slowTimer = js.FuncOf(func(js.Value, []js.Value) any {
		if !saveFinished {
			uistate.PostNotice(uistate.T("budgets.limitSaving"), false)
		}
		slowTimer.Release()
		return nil
	})
	js.Global().Call("setTimeout", slowTimer, 300)

	var write js.Func
	write = js.FuncOf(func(js.Value, []js.Value) any {
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		app.RequestDurable(func(result appstate.DurableResult) {
			saveFinished = true
			if result.Err != nil {
				uistate.PostNotice(budgetErrorText(result.Err), true)
			} else {
				uistate.PostUndoable(uistate.T("budgets.limitChangedToast", fmtMoney(budget.Limit)))
			}
			completeBudgetLimitSave(key)
		})
		write.Release()
		return nil
	})
	js.Global().Call("setTimeout", write, 0)
}

func completeBudgetLimitSave(key budgetLimitSaveKey) {
	queue := budgetLimitSaveInFlight[key]
	if queue == nil {
		return
	}
	request, hasPending := queue.complete()
	if !hasPending {
		delete(budgetLimitSaveInFlight, key)
		return
	}
	startBudgetLimitSave(key, request, nil, nil)
}
