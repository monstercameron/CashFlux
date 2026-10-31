// SPDX-License-Identifier: MIT

package networthview

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// AccountWindow contains the balance-sheet facts for one account. Balance and
// window availability are independent: a balance can be known in the account's
// currency even when the selected window cannot be converted to the base.
type AccountWindow struct {
	AccountID        string
	Balance          money.Money
	BaseBalance      money.Money
	Move, Start, End money.Money
	Flow, Adjustment money.Money
	BalanceErr       error
	BaseBalanceErr   error
	WindowErr        error
	Manual           bool
}

// BalanceKnown reports whether the account's current balance is available.
func (w AccountWindow) BalanceKnown() bool { return w.BalanceErr == nil }

// BaseBalanceKnown reports whether the current balance converted to base currency.
func (w AccountWindow) BaseBalanceKnown() bool { return w.BalanceErr == nil && w.BaseBalanceErr == nil }

// WindowKnown reports whether all selected-window facts are available.
func (w AccountWindow) WindowKnown() bool { return w.WindowErr == nil }

type accountWindowSums struct {
	account                                              domain.Account
	current, start, end, flow, adjustment                int64
	balanceErr, startErr, endErr, flowErr, adjustmentErr error
	newestDate                                           time.Time
	newestAdjustment, hasTransactions                    bool
}

// AccountWindows prepares current balance and selected-window movement facts
// for every account in one indexed transaction pass. Window bounds use the
// half-open [since, until) convention. Transactions affect balances regardless
// of report classification; the predicate only separates flow from adjustments.
// The returned slice follows input account order and source slices are untouched.
func AccountWindows(accounts []domain.Account, transactions []domain.Transaction, rates currency.Rates,
	since, until time.Time, isAdjustment func(domain.Transaction) bool) []AccountWindow {
	byID := make(map[string]*accountWindowSums, len(accounts))
	ordered := make([]*accountWindowSums, 0, len(accounts))
	for _, account := range accounts {
		s := &accountWindowSums{account: account}
		byID[account.ID] = s
		ordered = append(ordered, s)
		if account.OpeningBalance.Currency != "" && account.OpeningBalance.Currency != account.Currency {
			s.balanceErr = fmt.Errorf("networthview: account %s opening balance currency %q != account currency %q", account.ID, account.OpeningBalance.Currency, account.Currency)
			s.startErr, s.endErr = s.balanceErr, s.balanceErr
			continue
		}
		opening := account.OpeningBalance.Amount
		if account.OpeningBalance.Currency == "" {
			opening = 0
		}
		s.current, s.start, s.end = opening, opening, opening
	}
	for _, transaction := range transactions {
		s := byID[transaction.AccountID]
		if s == nil {
			continue
		}
		if !s.hasTransactions || !transaction.Date.Before(s.newestDate) {
			s.newestDate, s.hasTransactions = transaction.Date, true
			s.newestAdjustment = isAdjustment != nil && isAdjustment(transaction)
		}
		if transaction.Amount.Currency != s.account.Currency {
			err := fmt.Errorf("networthview: account %s transaction currency %q != account currency %q", s.account.ID, transaction.Amount.Currency, s.account.Currency)
			s.balanceErr, s.startErr, s.endErr = err, err, err
			continue
		}
		if s.balanceErr == nil {
			s.current, s.balanceErr = addAccountAmount(s.current, transaction.Amount.Amount, s.account.ID, "balance")
		}
		if transaction.Date.Before(since) && s.startErr == nil {
			s.start, s.startErr = addAccountAmount(s.start, transaction.Amount.Amount, s.account.ID, "window opening")
		}
		if transaction.Date.Before(until) && s.endErr == nil {
			s.end, s.endErr = addAccountAmount(s.end, transaction.Amount.Amount, s.account.ID, "window closing")
		}
		if !dateutil.InRange(transaction.Date, since, until) {
			continue
		}
		if isAdjustment != nil && isAdjustment(transaction) {
			if s.adjustmentErr == nil {
				s.adjustment, s.adjustmentErr = addAccountAmount(s.adjustment, transaction.Amount.Amount, s.account.ID, "adjustments")
			}
		} else if s.flowErr == nil {
			s.flow, s.flowErr = addAccountAmount(s.flow, transaction.Amount.Amount, s.account.ID, "flow")
		}
	}

	out := make([]AccountWindow, 0, len(ordered))
	for _, s := range ordered {
		w := AccountWindow{AccountID: s.account.ID, BalanceErr: s.balanceErr,
			Manual: !s.hasTransactions || s.newestAdjustment}
		if s.balanceErr == nil {
			w.Balance = money.New(s.current, s.account.Currency)
			w.BaseBalance, w.BaseBalanceErr = rates.Convert(w.Balance, rates.Base)
		}
		w.WindowErr = firstError(s.startErr, s.endErr, s.flowErr, s.adjustmentErr)
		if w.WindowErr == nil {
			move, err := liabilityDelta(s.account, s.start, s.end)
			if err != nil {
				w.WindowErr = fmt.Errorf("networthview: account %s window movement: %w", s.account.ID, err)
			}
			factor := int64(1)
			if s.account.Class == domain.ClassLiability && !(s.start < 0 || (s.start == 0 && s.end < 0)) {
				factor = -1
			}
			if w.WindowErr == nil {
				w.Move, w.WindowErr = convertAmount(move, s.account.Currency, rates, s.account.ID, "movement")
				if w.WindowErr == nil {
					w.Start, w.WindowErr = convertAmount(s.start, s.account.Currency, rates, s.account.ID, "opening balance")
				}
				if w.WindowErr == nil {
					w.End, w.WindowErr = convertAmount(s.end, s.account.Currency, rates, s.account.ID, "closing balance")
				}
				if w.WindowErr == nil {
					v, err := money.Mul(s.flow, factor)
					if err != nil {
						w.WindowErr = err
					} else {
						w.Flow, w.WindowErr = convertAmount(v, s.account.Currency, rates, s.account.ID, "flow")
					}
				}
				if w.WindowErr == nil {
					v, err := money.Mul(s.adjustment, factor)
					if err != nil {
						w.WindowErr = err
					} else {
						w.Adjustment, w.WindowErr = convertAmount(v, s.account.Currency, rates, s.account.ID, "adjustments")
					}
				}
			}
		}
		out = append(out, w)
	}
	return out
}

func addAccountAmount(current, amount int64, id, fact string) (int64, error) {
	next, err := money.Add(current, amount)
	if err != nil {
		return current, fmt.Errorf("networthview: account %s %s: %w", id, fact, err)
	}
	return next, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func liabilityDelta(account domain.Account, start, end int64) (int64, error) {
	if account.Class != domain.ClassLiability {
		return money.Sub(end, start)
	}
	startAbs, err := money.Abs(start)
	if err != nil {
		return 0, err
	}
	endAbs, err := money.Abs(end)
	if err != nil {
		return 0, err
	}
	delta, err := money.Sub(endAbs, startAbs)
	if err != nil {
		return 0, err
	}
	return money.Neg(delta)
}

func convertAmount(amount int64, source string, rates currency.Rates, id, fact string) (money.Money, error) {
	converted, err := rates.Convert(money.New(amount, source), rates.Base)
	if err != nil {
		return money.Money{}, fmt.Errorf("networthview: account %s %s conversion: %w", id, fact, err)
	}
	return converted, nil
}
