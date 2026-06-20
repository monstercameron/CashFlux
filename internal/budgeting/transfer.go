package budgeting

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Errors returned by Transfer. They are sentinel values so callers (and tests)
// can branch on the failure kind rather than parsing strings.
var (
	// ErrTransferSameBudget is returned when source and destination are the same budget.
	ErrTransferSameBudget = errors.New("budgeting: cannot transfer a budget to itself")
	// ErrTransferNonPositive is returned when the amount is zero or negative.
	ErrTransferNonPositive = errors.New("budgeting: transfer amount must be positive")
	// ErrTransferCurrency is returned when the two budgets' limits use different currencies.
	ErrTransferCurrency = errors.New("budgeting: budgets have different currencies")
	// ErrInsufficientSource is returned when the move would drive the source limit
	// negative and that is not allowed.
	ErrInsufficientSource = errors.New("budgeting: source budget has insufficient room")
)

// TransferResult is the explainable outcome of moving budgeted money from one
// budget to another. It records both legs so the UI can show exactly what changed
// (determinism rule) and so persistence/export can store the adjustment.
type TransferResult struct {
	From            domain.Budget // source budget after the move
	To              domain.Budget // destination budget after the move
	Amount          money.Money   // amount moved (positive)
	FromLimitBefore money.Money
	FromLimitAfter  money.Money
	ToLimitBefore   money.Money
	ToLimitAfter    money.Money
}

// Transfer moves amt of budgeted money from the source budget's limit to the
// destination budget's limit. It is balanced — the household's total budgeted
// amount is unchanged — and explainable via the returned TransferResult. The
// source limit may not go negative unless allowNegativeSource is set (e.g. an
// explicit "allow overdraw"). Both budgets must share a currency.
//
// The inputs are not mutated; the adjusted budgets are returned in the result.
func Transfer(from, to domain.Budget, amt money.Money, allowNegativeSource bool) (TransferResult, error) {
	if from.ID == to.ID {
		return TransferResult{}, ErrTransferSameBudget
	}
	if !amt.IsPositive() {
		return TransferResult{}, ErrTransferNonPositive
	}
	if from.Limit.Currency != to.Limit.Currency {
		return TransferResult{}, fmt.Errorf("%w: %q vs %q", ErrTransferCurrency, from.Limit.Currency, to.Limit.Currency)
	}
	if amt.Currency != from.Limit.Currency {
		return TransferResult{}, fmt.Errorf("%w: amount %q vs budgets %q", ErrTransferCurrency, amt.Currency, from.Limit.Currency)
	}

	newFrom, err := from.Limit.Sub(amt)
	if err != nil {
		return TransferResult{}, fmt.Errorf("budgeting: debit source: %w", err)
	}
	if newFrom.IsNegative() && !allowNegativeSource {
		return TransferResult{}, fmt.Errorf("%w: %s has %s but %s requested", ErrInsufficientSource, from.Name, from.Limit, amt)
	}
	newTo, err := to.Limit.Add(amt)
	if err != nil {
		return TransferResult{}, fmt.Errorf("budgeting: credit destination: %w", err)
	}

	fromOut := from
	fromOut.Limit = newFrom
	toOut := to
	toOut.Limit = newTo
	return TransferResult{
		From:            fromOut,
		To:              toOut,
		Amount:          amt,
		FromLimitBefore: from.Limit,
		FromLimitAfter:  newFrom,
		ToLimitBefore:   to.Limit,
		ToLimitAfter:    newTo,
	}, nil
}

// CoverAmount returns how much must move into a budget to clear an overspend: the
// shortfall by which spent exceeds the limit, or zero when the budget is within
// its limit. It is the natural default amount for a "cover the full $X over"
// one-tap action. It reads Status.Remaining (limit minus spent, in the budget's
// normalized currency), so it is correct even when the raw limit currency is empty.
func CoverAmount(status Status) money.Money {
	if !status.Remaining.IsNegative() {
		return money.Zero(status.Remaining.Currency)
	}
	return status.Remaining.Neg()
}
