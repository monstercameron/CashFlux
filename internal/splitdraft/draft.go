// SPDX-License-Identifier: MIT

// Package splitdraft owns the editable transaction split, its transitions,
// checked arithmetic, validation, and presentation decisions. It has no UI or
// browser dependencies. A draft always uses its parent transaction's currency;
// it never aggregates currencies or guesses an exchange rate.
package splitdraft

import (
	"errors"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
)

// Line is an editable category, unsigned amount (or percentage), and optional owner.
type Line struct {
	Cat, Amt, Owner string
}

// State is the immutable editor draft. Transitions copy Lines before changing them.
type State struct {
	Lines         []Line
	Percent       bool
	ErrorKey      string
	SaveAttempted bool
	Generation    int
	PickerLine    int
}

// ActionKind identifies one draft transition.
type ActionKind uint8

const (
	// SetCategory changes a row's category.
	SetCategory ActionKind = iota
	// SetAmount changes a row's raw input.
	SetAmount
	// SetOwner changes a row's member attribution.
	SetOwner
	// Add appends an untouched row.
	Add
	// Remove removes a row.
	Remove
	// ToAmounts converts percentage entries to amounts.
	ToAmounts
	// ToPercents converts amount entries to percentages.
	ToPercents
	// OpenPicker opens the new-category picker for a row.
	OpenPicker
	// ClosePicker dismisses the category picker.
	ClosePicker
	// PickCategory assigns the picked category and dismisses the picker.
	PickCategory
)

// Action carries a row index and raw value for a transition.
type Action struct {
	Kind  ActionKind
	Index int
	Value string
}

// New seeds saved splits or two unfinished rows for a new breakdown.
func New(tx domain.Transaction, generation int) State {
	s := State{Generation: generation, PickerLine: -1}
	if !tx.HasSplits() {
		s.Lines = []Line{{Cat: tx.CategoryID}, {}}
		return s
	}
	s.Lines = make([]Line, 0, len(tx.Splits))
	for _, line := range tx.Splits {
		s.Lines = append(s.Lines, Line{Cat: line.CategoryID,
			Amt: money.FormatMinor(Magnitude(line.Amount.Amount), currency.Decimals(tx.Amount.Currency)), Owner: line.MemberID})
	}
	return s
}

// Reseed accepts a new proposal only when its generation changes. Existing
// save-attempt and picker state intentionally survive, matching the editor.
func (s State) Reseed(tx domain.Transaction, generation int) State {
	if s.Generation == generation {
		return s
	}
	next := New(tx, generation)
	s.Lines, s.Percent, s.ErrorKey, s.Generation = next.Lines, false, "", generation
	return s
}

// Apply performs a draft transition. Invalid row edits and unknown actions are
// ignored; even a stale picker result dismisses the picker. Mode changes preserve
// balanced totals by largest remainders.
func (s State) Apply(a Action, tx domain.Transaction) State {
	switch a.Kind {
	case Add:
		s.Lines = append(append([]Line(nil), s.Lines...), Line{})
	case OpenPicker:
		s.PickerLine = a.Index
	case ClosePicker:
		s.PickerLine = -1
	case ToAmounts, ToPercents:
		percent := a.Kind == ToPercents
		total, err := money.Abs(tx.Amount.Amount)
		if percent == s.Percent || err != nil || total == 0 {
			return s
		}
		srcDec, srcTotal := precision(tx, s.Percent), total
		dstDec, dstTotal := precision(tx, percent), total
		if s.Percent {
			srcTotal = split.PercentScale
		} else {
			dstTotal = split.PercentScale
		}
		converted, ok := convert(s.Lines, srcDec, srcTotal, dstDec, dstTotal)
		if ok {
			s.Lines, s.Percent = converted, percent
		}
	case SetCategory, SetAmount, SetOwner, Remove, PickCategory:
		if a.Kind == PickCategory {
			s.PickerLine = -1
		}
		if a.Index < 0 || a.Index >= len(s.Lines) {
			return s
		}
		s.Lines = append([]Line(nil), s.Lines...)
		switch a.Kind {
		case SetCategory, PickCategory:
			s.Lines[a.Index].Cat = a.Value
		case SetAmount:
			s.Lines[a.Index].Amt = a.Value
		case SetOwner:
			s.Lines[a.Index].Owner = a.Value
		case Remove:
			s.Lines = append(s.Lines[:a.Index], s.Lines[a.Index+1:]...)
		}
	}
	return s
}

func precision(tx domain.Transaction, percent bool) int {
	if percent {
		return 2
	}
	return currency.Decimals(tx.Amount.Currency)
}

func convert(lines []Line, srcDec int, srcTotal int64, dstDec int, dstTotal int64) ([]Line, bool) {
	out := append([]Line(nil), lines...)
	indices := make([]int, 0, len(lines))
	weights := make([]int64, 0, len(lines))
	var sum int64
	exact := true
	for i, line := range lines {
		if line.Amt == "" {
			continue
		}
		v, err := money.ParseMinor(line.Amt, srcDec)
		if err != nil || v <= 0 {
			exact = false
			continue
		}
		indices, weights = append(indices, i), append(weights, v)
		sum, err = money.Add(sum, v)
		if err != nil {
			return nil, false
		}
	}
	if len(indices) == 0 {
		return out, true
	}
	if exact && split.Balanced(srcTotal, weights) {
		shares, err := money.Allocate(dstTotal, weights)
		if err != nil {
			return nil, false
		}
		for k, i := range indices {
			out[i].Amt = money.FormatMinor(shares[k], dstDec)
		}
		return out, true
	}
	for k, i := range indices {
		v, err := money.MulDiv(weights[k], dstTotal, srcTotal, money.RoundHalfAwayFromZero)
		if err != nil {
			return nil, false
		}
		out[i].Amt = money.FormatMinor(v, dstDec)
	}
	return out, true
}

// AttemptSave validates the complete rows, applies the transaction's sign and
// currency, and returns a proposal. It never persists or acknowledges a write.
// Pending prevents both the proposal and any change to validation state.
func (s State) AttemptSave(tx domain.Transaction, pending bool) (State, domain.Transaction, bool) {
	if pending {
		return s, tx, false
	}
	s.SaveAttempted = true
	built, key := build(tx, s.Lines, s.Percent)
	s.ErrorKey = key
	return s, built, key == ""
}

func build(tx domain.Transaction, draft []Line, percent bool) (domain.Transaction, string) {
	lines := make([]Line, 0, len(draft))
	values := make([]int64, 0, len(draft))
	badKey := "splitEditor.badAmount"
	if percent {
		badKey = "splitEditor.badPercent"
	}
	for _, line := range draft {
		if line.Cat == "" || line.Amt == "" {
			continue
		}
		v, err := money.ParseMinor(line.Amt, precision(tx, percent))
		if err != nil || v <= 0 {
			return tx, badKey
		}
		lines, values = append(lines, line), append(values, v)
	}
	if len(lines) < split.MinSplitLines {
		return tx, "splitEditor.needTwo"
	}
	total, err := money.Abs(tx.Amount.Amount)
	if err != nil {
		return tx, badKey
	}
	amounts, key := allocateValues(values, total, percent)
	if key != "" {
		return tx, key
	}
	built := make([]domain.CategorySplit, len(lines))
	for i, line := range lines {
		amount := amounts[i]
		if tx.Amount.IsNegative() {
			// amount is nonnegative and <= MaxInt64 after allocation/validation.
			amount, _ = money.Neg(amount)
		}
		built[i] = domain.CategorySplit{CategoryID: line.Cat,
			Amount: money.New(amount, tx.Amount.Currency), MemberID: line.Owner}
	}
	tx.Splits = built
	return tx, ""
}

func allocateValues(values []int64, total int64, percent bool) ([]int64, string) {
	shares, err := split.Reconcile(total, values, percent)
	if errors.Is(err, split.ErrTooSmall) {
		return nil, "splitEditor.pctTooSmall"
	}
	if err != nil {
		if percent {
			return nil, "splitEditor.pctMustBalance"
		}
		return nil, "splitEditor.mustBalance"
	}
	return shares, ""
}

// Clear proposes removing the breakdown without changing the current inputs.
// As with saving, pending suppresses the proposal; persistence belongs to the caller.
func (s State) Clear(tx domain.Transaction, pending bool) (State, domain.Transaction, bool) {
	if pending {
		return s, tx, false
	}
	s.ErrorKey = ""
	tx.Splits = nil
	return s, tx, true
}
