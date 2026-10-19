// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CoverRequest moves explicit shares into a destination for the viewed period.
// BorrowNext instead moves money from that budget's following period. These
// alternatives cannot be combined in a single request.
type CoverRequest struct {
	DestinationID string
	Shares        map[string]money.Money
	BorrowNext    money.Money
	// Permanent applies the balanced change to standing limits from the viewed
	// period onward, through the application's history-aware write boundary.
	Permanent bool
}

// CoverSourceFunds resolves source room from the same effective cap and bill
// commitments as the budget card. Saving targets are never spending donors.
// Covers use Free, so a recurring bill is not silently left without funding.
func CoverSourceFunds(b domain.Budget, in ViewInput) (SourceFunds, error) {
	if b.IsSaving() {
		return SourceFunds{}, fmt.Errorf("%w: savings target %q cannot fund a cover", ErrInsufficientSource, b.ID)
	}
	v := EvaluateViewBudget(b, in)
	if !v.Valid {
		return SourceFunds{}, fmt.Errorf("evaluate cover source %q: %w", b.ID, v.Error)
	}
	if v.CommitmentError != nil {
		return SourceFunds{}, v.CommitmentError
	}
	return SourceFundsOf(v.Status.Limit, v.Status.Remaining, v.Committed.Required), nil
}

// PlanCover builds every debit and credit without mutating its inputs. All
// cover entry points use this room policy and the caller commits the returned
// set atomically. Moves never become copyable month-close top-ups.
func PlanCover(budgets []domain.Budget, requests []CoverRequest, in ViewInput) ([]domain.Budget, error) {
	if in.Period.Anchor.IsZero() {
		return nil, fmt.Errorf("cover period is required")
	}
	current := make(map[string]domain.Budget, len(budgets))
	for _, b := range budgets {
		if _, duplicate := current[b.ID]; duplicate {
			return nil, fmt.Errorf("duplicate cover budget %q", b.ID)
		}
		current[b.ID] = b.WithBaseCurrency(in.Base)
	}
	changed := map[string]bool{}
	for _, request := range requests {
		to, ok := current[request.DestinationID]
		if !ok || to.IsSaving() {
			return nil, fmt.Errorf("spending cover destination %q not found", request.DestinationID)
		}
		start, end := PeriodRangeAnchored(to.Period, in.Period.Anchor, in.Period.WeekStart, in.Period.PayCycleAnchor)
		total := money.Zero(to.Limit.Currency)
		ids := make([]string, 0, len(request.Shares))
		for id := range request.Shares {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			amount := request.Shares[id]
			if id == to.ID {
				return nil, ErrTransferSameBudget
			}
			if !amount.IsPositive() {
				return nil, ErrTransferNonPositive
			}
			from, ok := current[id]
			if !ok {
				return nil, fmt.Errorf("cover source %q not found", id)
			}
			if amount.Currency != to.Limit.Currency || from.Limit.Currency != to.Limit.Currency {
				return nil, ErrTransferCurrency
			}
			funds, err := CoverSourceFunds(from, in)
			if err != nil {
				return nil, err
			}
			if amount.Amount > funds.Free.Amount {
				return nil, fmt.Errorf("%w: %s", ErrInsufficientSource, from.Name)
			}
			fromStart, _ := PeriodRangeAnchored(from.Period, in.Period.Anchor, in.Period.WeekStart, in.Period.PayCycleAnchor)
			from, err = coverAdjustment(from, fromStart, -amount.Amount, request.Permanent)
			if err != nil {
				return nil, err
			}
			current[id], changed[id] = from, true
			total, err = total.Add(amount)
			if err != nil {
				return nil, fmt.Errorf("cover total: %w", err)
			}
		}
		if request.BorrowNext.Amount != 0 {
			amount := request.BorrowNext
			if len(ids) != 0 || !amount.IsPositive() || to.Rollover || in.PoolRollover || request.Permanent {
				return nil, fmt.Errorf("invalid next-period cover")
			}
			if amount.Currency != to.Limit.Currency {
				return nil, ErrTransferCurrency
			}
			next := in
			next.Period.Anchor = end
			funds, err := CoverSourceFunds(to, next)
			if err != nil {
				return nil, err
			}
			if amount.Amount > funds.Free.Amount {
				return nil, ErrInsufficientSource
			}
			to, err = to.WithPeriodMoveChecked(end, -amount.Amount)
			if err != nil {
				return nil, err
			}
			total = amount
		}
		if total.IsPositive() {
			var err error
			to, err = coverAdjustment(to, start, total.Amount, request.Permanent)
			if err != nil {
				return nil, err
			}
			if evaluated := EvaluateViewBudget(to, in); !evaluated.Valid {
				return nil, fmt.Errorf("evaluate covered destination %q: %w", to.ID, evaluated.Error)
			}
			to.CoveredAt = in.Period.Anchor
			current[to.ID], changed[to.ID] = to, true
		}
	}
	ids := make([]string, 0, len(changed))
	for id := range changed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	writes := make([]domain.Budget, 0, len(ids))
	for _, id := range ids {
		writes = append(writes, current[id])
	}
	return writes, nil
}

// coverAdjustment uses the same checked boost core for every move. Permanent
// drafts also update their local history, so subsequent requests evaluate the
// room left by earlier debits. Persistence rebuilds history from stored rows.
func coverAdjustment(b domain.Budget, start time.Time, delta int64, permanent bool) (domain.Budget, error) {
	if !permanent {
		return b.WithPeriodMoveChecked(start, delta)
	}
	base, err := BudgetBaseAt(b, start)
	if err != nil {
		return domain.Budget{}, err
	}
	amount, err := base.Add(money.New(delta, base.Currency))
	if err != nil {
		return domain.Budget{}, err
	}
	if !amount.IsPositive() {
		return domain.Budget{}, ErrInsufficientSource
	}
	return coverPermanentLimit(b, amount, start), nil
}

// The draft history lets a later request in the same plan evaluate room after
// an earlier permanent debit. Persistence rebuilds this from the stored history
// through PutBudgetsAtomicAt; this draft cannot replace earlier recorded limits.
func coverPermanentLimit(b domain.Budget, amount money.Money, start time.Time) domain.Budget {
	history := make(map[string]int64, len(b.LimitHistory)+1)
	for key, value := range b.LimitHistory {
		history[key] = value
	}
	history[start.Format(time.DateOnly)] = amount.Amount
	b.Limit, b.LimitHistory = amount, history
	return b
}

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
