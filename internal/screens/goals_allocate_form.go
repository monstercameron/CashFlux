// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	goalsvc "github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// GoalAllocateFormProps drives the virtual-allocation modal.
type GoalAllocateFormProps struct {
	GoalID string
	OnDone func()
}

type goalAllocationSnapshot struct {
	Goal  domain.Goal
	Draft goalview.AllocationDraft
	Found bool
}

var goalAllocationCache = newViewCache[goalAllocationSnapshot](1)

// The editor's local text changes reuse one immutable model. A store/settings
// revision rebuilds its account-native headroom from the shared balance index.
func goalAllocationSnapshotFor(app *appstate.App, id string) goalAllocationSnapshot {
	if app == nil {
		return goalAllocationSnapshot{}
	}
	return memoByRev(goalAllocationCache, revKey(app)+"|"+id, func() goalAllocationSnapshot {
		all := app.Goals()
		for _, g := range all {
			if g.ID == id {
				return goalAllocationSnapshot{Goal: g, Found: true, Draft: goalview.BuildAllocationDraft(g, all, app.Accounts(), accountSnapshot(app).Balances, goalRates(app))}
			}
		}
		return goalAllocationSnapshot{}
	})
}

// GoalAllocateForm edits reservations without moving money. Unchanged records
// retain their original currencies; explicit edits use the labeled goal currency.
func GoalAllocateForm(props GoalAllocateFormProps) ui.Node {
	return ui.CreateElement(goalAllocateForm, props)
}

func goalAllocateForm(props GoalAllocateFormProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	app := appstate.Default
	snapshot := goalAllocationSnapshotFor(app, props.GoalID)
	g, draft := snapshot.Goal, snapshot.Draft
	dec := currency.Decimals(draft.Currency)
	prefill := ""
	if draft.PrefillKnown && draft.Prefill.Amount > 0 {
		prefill = money.FormatMinor(draft.Prefill.Amount, dec)
	}
	changesS := ui.UseState(map[string]goalview.AllocationChange{})
	totalS := ui.UseState(prefill)
	totalCurrencyS := ui.UseState(draft.Currency)
	errS := ui.UseState("")
	done := props.OnDone
	if done == nil {
		done = func() {}
	}
	selected := func(row goalview.AllocationDraftRow) bool {
		if change, ok := changesS.Get()[row.AccountID]; ok {
			return change.Selected
		}
		return row.Selected
	}
	copyChanges := func() map[string]goalview.AllocationChange {
		next := make(map[string]goalview.AllocationChange, len(changesS.Get())+1)
		for id, change := range changesS.Get() {
			next[id] = change
		}
		return next
	}
	onTotal := ui.UseEvent(func(value string) { totalS.Set(value); totalCurrencyS.Set(draft.Currency); errS.Set("") })
	onClearAll := ui.UseEvent(Prevent(func() {
		next := copyChanges()
		for id, change := range next {
			change.Selected = false
			next[id] = change
		}
		for _, row := range draft.Rows {
			change := next[row.AccountID]
			change.Selected = false
			next[row.AccountID] = change
		}
		changesS.Set(next)
		errS.Set("")
	}))
	onToggleAcct := func(id string) {
		r, _ := draft.Row(id)
		next := copyChanges()
		change := next[id]
		change.Selected = !selected(r)
		next[id] = change
		changesS.Set(next)
		errS.Set("")
	}
	onAmount := func(id, value string) {
		r, _ := draft.Row(id)
		next := copyChanges()
		change := next[id]
		change.Selected = selected(r)
		change.AmountText, change.AmountChanged, change.Currency = value, true, draft.Currency
		next[id] = change
		changesS.Set(next)
		errS.Set("")
	}
	cancel := ui.UseEvent(Prevent(func() { done() }))
	doSplit := func(mode goalsvc.SplitMode) {
		live := goalAllocationSnapshotFor(app, props.GoalID)
		if !live.Found {
			errS.Set(uistate.T("common.notReady"))
			return
		}
		if totalCurrencyS.Get() != live.Draft.Currency {
			errS.Set(uistate.T("goals.allocCurrencyChanged"))
			return
		}
		next, err := live.Draft.Split(totalS.Get(), changesS.Get(), mode)
		if err != nil {
			errS.Set(goalAllocationErrorText(err, live.Draft))
			return
		}
		changesS.Set(next)
		errS.Set("")
	}
	onSplitEven := ui.UseEvent(Prevent(func() { doSplit(goalsvc.SplitEven) }))
	onSplitProp := ui.UseEvent(Prevent(func() { doSplit(goalsvc.SplitProportional) }))
	save := ui.UseEvent(Prevent(func() {
		// Re-read the store revision even if a balance/rate mutation has not yet
		// notified this component. Only explicit edits replace the live originals.
		live := goalAllocationSnapshotFor(app, props.GoalID)
		if !live.Found {
			errS.Set(uistate.T("common.notReady"))
			return
		}
		allocations, err := live.Draft.Apply(changesS.Get())
		if err != nil {
			errS.Set(goalAllocationErrorText(err, live.Draft))
			return
		}
		updated := live.Goal
		updated.Allocations = allocations
		if err := app.PutGoal(updated); err != nil {
			errS.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		if len(allocations) == 0 {
			uistate.PostUndoable(uistate.T("goals.earmarksCleared"))
		} else {
			uistate.PostUndoable(uistate.T("goals.allocationsSaved"))
		}
		done()
	}))
	if app == nil || !snapshot.Found {
		return Div(css.Class("acct-edit-form"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

	preview := draft.Preview(changesS.Get(), time.Now())
	selectedCount := 0
	for _, row := range draft.Rows {
		if selected(row) {
			selectedCount++
		}
	}
	for id, change := range changesS.Get() {
		if _, ok := draft.Row(id); !ok && change.Selected {
			selectedCount++
		}
	}
	var errLine ui.Node = Fragment()
	if errS.Get() != "" {
		errLine = P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "goal-alloc-error"), errS.Get())
	}
	freeText := uistate.T("goals.allocFreeTotal", fmtMoney(draft.Free))
	if draft.FreeErr != nil {
		freeText = uistate.T("goals.allocTotalUnavailable")
	}
	var picker ui.Node = P(css.Class("budget-sub"), uistate.T("goals.allocateNoAccts"))
	if len(draft.Rows) > 0 {
		picker = Div(css.Class("goal-alloc-list"), MapKeyed(draft.Rows, func(row goalview.AllocationDraftRow) any { return row.AccountID }, func(row goalview.AllocationDraftRow) ui.Node {
			name := row.Account.Name
			if name == "" {
				name = uistate.T("goals.allocMissingAccount", row.AccountID)
			}
			value := ""
			if row.Selected && row.AmountKnown {
				value = money.FormatMinor(row.Amount.Amount, dec)
			}
			var rowErr error
			if change, ok := changesS.Get()[row.AccountID]; ok {
				if change.AmountChanged {
					value = change.AmountText
				}
				rowErr = draft.CheckChange(row.AccountID, change)
			}
			available := uistate.T("goals.allocAvailableUnknown")
			if row.AvailableKnown {
				available = uistate.T("goals.allocateAvail", fmtMoney(row.Available))
			}
			typeTag := ""
			if row.Exists && !earmarkEligibleType(row.Account.Type) {
				typeTag = uistate.T("goals.allocHeldTag", humanizeType(string(row.Account.Type)))
			}
			if !row.CanEdit && row.Selected {
				typeTag = uistate.T("goals.allocExistingSourceUnavailable")
			}
			stored := ""
			if len(row.Original) > 0 {
				amounts := make([]string, 0, len(row.Original))
				for _, allocation := range row.Original {
					amounts = append(amounts, fmtMoney(allocation.Amount))
				}
				stored = uistate.T("goals.allocStoredAmounts", strings.Join(amounts, ", "))
			}
			message := ""
			if rowErr != nil {
				message = goalAllocationErrorText(rowErr, draft)
			}
			return ui.CreateElement(goalAllocateRow, goalAllocateRowProps{AccountID: row.AccountID, AccountName: name, AvailStr: available, Value: value, TypeTag: typeTag, StoredText: stored, Selected: selected(row), Editable: row.CanEdit, CanSelect: row.CanEdit || len(row.Original) > 0, ErrorText: message, Step: money.FormatMinor(1, dec), Currency: draft.Currency, OnToggle: onToggleAcct, OnChange: onAmount})
		}))
	}
	var clearNote ui.Node = Fragment()
	if selectedCount == 0 && len(g.Allocations) > 0 {
		clearNote = P(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "goal-alloc-clearnote"), uistate.T("goals.allocClearNote"))
	}
	var summary ui.Node = Span(uistate.T("goals.coverageUnavailableShort"))
	if preview.Err == nil {
		summary = Fragment(Span(uistate.T("goals.allocateFooter", fmtMoney(preview.Total), fmtMoney(g.TargetAmount))), Span(css.Class("goal-alloc-cover"), uistate.T("goals.coverageChip", preview.Evaluation.Coverage.CoveragePercent)))
	}
	return Form(css.Class("acct-edit-form", "goal-allocate"), Attr("data-testid", "goal-allocate-form"), OnSubmit(save),
		Div(css.Class("modal-scroll"),
			P(css.Class("t-caption", "muted"), Style(map[string]string{"margin": "0 0 0.5rem"}), uistate.T("goals.allocateIntro")),
			P(css.Class("budget-sub"), Attr("data-testid", "goal-alloc-free-total"), Style(map[string]string{"margin": "0 0 0.35rem"}), freeText),
			If(draft.Incomplete, P(css.Class("budget-sub", tw.TextWarn), Attr("data-testid", "goal-alloc-unavailable"), uistate.T("goals.allocUnavailableExplanation"))),
			Div(css.Class("goal-alloc-split"), Attr("data-testid", "goal-alloc-split"),
				Div(css.Class("goal-alloc-split-row"),
					Div(css.Class("goal-alloc-split-field"), Span(css.Class("t-caption", tw.TextDim), uistate.T("goals.allocSplitLabel")),
						Input(css.Class("field", "goal-alloc-total"), Type("number"), Attr("data-testid", "goal-alloc-total"), Attr("aria-label", uistate.T("goals.allocSplitLabel")), Attr("min", "0"), Step(money.FormatMinor(1, dec)), OnInput(onTotal), uiw.FieldValue(totalS.Get()))),
					Div(css.Class("goal-alloc-split-btns"),
						Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-alloc-split-even"), OnClick(onSplitEven), uistate.T("goals.allocSplitEven")),
						Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "goal-alloc-split-prop"), OnClick(onSplitProp), uistate.T("goals.allocSplitProp")))),
				Span(css.Class("budget-sub"), uistate.T("goals.allocSplitHint"))),
			picker,
			If(selectedCount > 0, Button(css.Class("btn btn-sm", "goal-alloc-clear"), Type("button"), Attr("data-testid", "goal-alloc-clear"), OnClick(onClearAll), uistate.T("goals.allocClearAll"))),
			clearNote,
			Div(css.Class("goal-alloc-summary"), Attr("data-testid", "goal-alloc-summary"), summary), errLine),
		Div(css.Class("modal-foot"), Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
			Button(css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "goal-alloc-save"), uistate.T("goals.allocateSave"))))
}

func goalAllocationErrorText(err error, draft goalview.AllocationDraft) string {
	var detail *goalview.AllocationDraftError
	errors.As(err, &detail)
	name := ""
	if detail != nil {
		name = detail.AccountID
		if row, ok := draft.Row(detail.AccountID); ok && row.Account.Name != "" {
			name = row.Account.Name
		}
	}
	switch {
	case errors.Is(err, goalview.ErrAllocationCurrency):
		return uistate.T("goals.allocCurrencyChanged")
	case errors.Is(err, money.ErrOverflow):
		return uistate.T("goals.allocAmountsOverflow")
	case errors.Is(err, goalview.ErrAllocationHeadroom):
		if name != "" {
			return uistate.T("goals.allocOverAvail", name, fmtMoney(detail.Available))
		}
		return uistate.T("goals.allocNoCapacity")
	case errors.Is(err, goalview.ErrAllocationAmount):
		if name != "" {
			return uistate.T("goals.allocAmountInvalid", name)
		}
		return uistate.T("goals.allocSplitBad")
	case errors.Is(err, goalview.ErrAllocationUnavailable):
		return uistate.T("goals.allocSourceUnavailable", name)
	default:
		return uistate.T("goals.allocReviewAmounts")
	}
}

// earmarkEligibleType reports whether an account holds liquid spendable cash.
func earmarkEligibleType(t domain.AccountType) bool { return goalview.LiquidSource(t) }

// earmarkSourceAccount reports whether an account can back a virtual reservation.
func earmarkSourceAccount(a domain.Account) bool { return goalview.EarmarkSource(a) }

type goalAllocateRowProps struct {
	AccountID, AccountName, AvailStr, Value        string
	TypeTag, StoredText, ErrorText, Step, Currency string
	Selected, Editable, CanSelect                  bool
	OnToggle                                       func(string)
	OnChange                                       func(string, string)
}

// goalAllocateRow owns its input hooks; only checked native draft facts are rendered.
func goalAllocateRow(props goalAllocateRowProps) ui.Node {
	toggle := ui.UseEvent(func() { props.OnToggle(props.AccountID) })
	onInput := ui.UseEvent(func(value string) { props.OnChange(props.AccountID, value) })
	rowClass := "goal-alloc-row"
	if props.Selected {
		rowClass += " is-on"
	}
	if props.ErrorText != "" {
		rowClass += " is-over"
	}
	amountArgs := []any{css.Class("field", "goal-alloc-input"), Type("number"), Attr("data-testid", "goal-alloc-"+props.AccountID), Attr("aria-label", uistate.T("goals.allocAccountAmount", props.AccountName, props.Currency)), Placeholder("0"), Step(props.Step), Attr("min", "0"), OnInput(onInput)}
	amountArgs = append(amountArgs, DisabledIf(!props.Selected || !props.Editable))
	if props.ErrorText != "" {
		amountArgs = append(amountArgs, Attr("aria-invalid", "true"))
	}
	pickArgs := []any{Type("checkbox"), Attr("data-testid", "goal-alloc-pick-"+props.AccountID), OnChange(toggle)}
	pickArgs = append(pickArgs, DisabledIf(!props.CanSelect))
	pickArgs = append(pickArgs, checkedAttr(props.Selected)...)
	return Div(css.Class(rowClass), Attr("data-testid", "goal-alloc-row-"+props.AccountID),
		Label(css.Class("goal-alloc-pick"), Input(pickArgs...), Div(css.Class("goal-alloc-row-main"),
			Span(css.Class("goal-alloc-acct"), props.AccountName),
			If(props.TypeTag != "", Span(css.Class("goal-alloc-type", tw.TextFaint), props.TypeTag)),
			Span(css.Class("goal-alloc-avail", tw.TextDim), props.AvailStr),
			If(props.StoredText != "", Span(css.Class("goal-alloc-avail", tw.TextDim), props.StoredText)),
			If(props.ErrorText != "", Span(css.Class("goal-alloc-over", tw.TextWarn), Attr("data-testid", "goal-alloc-over-"+props.AccountID), props.ErrorText)))),
		Input(append(amountArgs, uiw.FieldValue(props.Value))...))
}
