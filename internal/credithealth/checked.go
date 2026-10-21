// SPDX-License-Identifier: MIT

package credithealth

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// ErrMissingBalance distinguishes an unavailable card balance from a known zero.
var ErrMissingBalance = errors.New("credit health: card balance unavailable")

// CardSnapshot retains each card's recorded balance and limit currencies.
// Targets are in the account currency. Err means its monetary facts cannot be
// computed; UtilErr applies only to the displayed native utilization ratio.
// Preserved source fields are never substituted for unavailable results.
type CardSnapshot struct {
	AccountID, Name    string
	Balance, Limit     money.Money
	Target30, Target10 money.Money
	UtilPct            int64
	Err, UtilErr       error
}

// CheckedResult separates independent factors from unavailable monetary facts.
// Balance, Limit and PayTo30/10 are in the rate table's base currency. Each
// monetary result is usable only when its corresponding error is nil.
// ProxyKnown requires valid aggregate utilization; missing on-time/age factors
// retain the established -1 sentinel and zero weight.
type CheckedResult struct {
	Cards                            []CardSnapshot
	Balance, Limit, PayTo30, PayTo10 money.Money
	UtilPct                          int64
	UtilScore                        int
	UtilErr, PayTo30Err, PayTo10Err  error
	CardsMissingLimit                int
	OnTimeScore, AgeScore            int
	Weights                          ProxyWeights
	ProxyScore                       int
	ProxyKnown                       bool
}

func checkedUtilization(owed, limit int64) (int64, error) {
	if limit <= 0 {
		return -1, nil
	}
	return money.MulDiv(owed, 100, limit, money.RoundHalfAwayFromZero)
}
func checkedTarget(owed, limit, pct int64) (int64, error) {
	threshold, err := money.MulDiv(limit, pct, 100, money.RoundTowardZero)
	if err != nil {
		return 0, err
	}
	if owed <= threshold {
		return 0, nil
	}
	return money.Sub(owed, threshold)
}
func checkedCard(a domain.Account, balances map[string]int64, rates currency.Rates) CardSnapshot {
	c := CardSnapshot{AccountID: a.ID, Name: a.Name, Balance: money.Zero(a.Currency), Limit: a.CreditLimit, UtilPct: -1, Target30: money.Zero(a.Currency), Target10: money.Zero(a.Currency)}
	balance, found := balances[a.ID]
	if !found {
		c.Err = ErrMissingBalance
		return c
	}
	c.Balance = money.New(balance, a.Currency)
	if a.CreditLimit.Amount == 0 {
		return c
	}
	if a.CreditLimit.Amount < 0 {
		c.Err = fmt.Errorf("credit health: credit limit must not be negative")
		return c
	}
	owed, err := money.Abs(balance)
	if err != nil {
		c.Err = err
		return c
	}
	if a.Currency == "" || a.CreditLimit.Currency == "" {
		c.Err = fmt.Errorf("credit health: account and limit currencies are required")
		return c
	}
	limit, err := rates.Convert(a.CreditLimit, a.Currency)
	if err != nil {
		c.Err = err
		return c
	}
	if limit.Amount <= 0 {
		c.Err = fmt.Errorf("credit health: converted limit is not positive")
		return c
	}
	c.UtilPct, c.UtilErr = checkedUtilization(owed, limit.Amount)
	if c.UtilErr != nil {
		c.UtilPct = -1
	}
	c.Target30.Amount, err = checkedTarget(owed, limit.Amount, 30)
	if err == nil {
		c.Target10.Amount, err = checkedTarget(owed, limit.Amount, 10)
	}
	if err != nil {
		c.Err = err
	}
	return c
}

// EvaluateWithRates is the checked, currency-aware sibling of Evaluate. It uses
// the same native balance-magnitude, on-time, age, factor curves and normalized
// weighting contract. Native per-card targets are converted once for paydown
// totals; aggregate utilization converts original owed balances and recorded
// limits directly to base, avoiding mixed-unit sums or double conversion.
// Failed monetary facts do not discard independent scores or known weights.
func EvaluateWithRates(in Inputs, rates currency.Rates) CheckedResult {
	r := CheckedResult{Balance: money.Zero(rates.Base), Limit: money.Zero(rates.Base), PayTo30: money.Zero(rates.Base), PayTo10: money.Zero(rates.Base), UtilPct: -1}
	var accounts []domain.Account
	for _, a := range in.Accounts {
		if a.Type == domain.TypeCreditCard && !a.Archived {
			accounts = append(accounts, a)
		}
	}
	r.OnTimeScore = computeOnTime(accounts, in.Transactions, in.Now)
	r.AgeScore = computeAge(accounts, in.Now)
	r.Weights = proxyWeights(r.OnTimeScore, r.AgeScore)
	if rates.Base == "" {
		err := fmt.Errorf("credit health: base currency is required")
		r.UtilErr, r.PayTo30Err, r.PayTo10Err = err, err, err
	}
	for _, a := range accounts {
		card := checkedCard(a, in.Balances, rates)
		if card.Err != nil {
			card.Err = fmt.Errorf("credit health: card %q: %w", a.ID, card.Err)
		}
		r.Cards = append(r.Cards, card)
		if a.CreditLimit.Amount == 0 {
			r.CardsMissingLimit++
			// A missing limit has always supplied no utilization or paydown target.
			// An absent balance is still explicit on the card snapshot.
			continue
		}
		if card.Err != nil {
			r.UtilErr = errors.Join(r.UtilErr, card.Err)
			r.PayTo30Err = errors.Join(r.PayTo30Err, card.Err)
			r.PayTo10Err = errors.Join(r.PayTo10Err, card.Err)
			continue
		}
		owed, _ := card.Balance.AbsChecked() // checkedCard already checked this magnitude.
		balance, err := rates.Convert(owed, rates.Base)
		var limit money.Money
		if err == nil {
			limit, err = rates.Convert(card.Limit, rates.Base)
		}
		if err == nil {
			r.Balance, err = r.Balance.Add(balance)
		}
		if err == nil {
			r.Limit, err = r.Limit.Add(limit)
		}
		if err != nil {
			r.UtilErr = errors.Join(r.UtilErr, fmt.Errorf("credit health: card %q aggregate: %w", a.ID, err))
		}
		addTarget := func(total money.Money, target money.Money) (money.Money, error) {
			// A known native zero needs no exchange-rate quote to remain zero.
			if target.Amount == 0 {
				return total, nil
			}
			converted, err := rates.Convert(target, rates.Base)
			if err != nil {
				return money.Money{}, err
			}
			return total.Add(converted)
		}
		if r.PayTo30Err == nil {
			var err error
			r.PayTo30, err = addTarget(r.PayTo30, card.Target30)
			if err != nil {
				r.PayTo30Err = fmt.Errorf("credit health: pay to 30: %w", err)
			}
		}
		if r.PayTo10Err == nil {
			var err error
			r.PayTo10, err = addTarget(r.PayTo10, card.Target10)
			if err != nil {
				r.PayTo10Err = fmt.Errorf("credit health: pay to 10: %w", err)
			}
		}
	}
	if r.UtilErr == nil && len(accounts) > r.CardsMissingLimit && r.Limit.Amount <= 0 {
		r.UtilErr = fmt.Errorf("credit health: aggregate limit rounds to zero in base currency")
	}
	if r.UtilErr == nil {
		var err error
		r.UtilPct, err = checkedUtilization(r.Balance.Amount, r.Limit.Amount)
		if err != nil {
			r.UtilErr = err
		}
	}
	if r.UtilErr == nil {
		if r.UtilPct >= 80 {
			r.UtilScore = 0
		} else {
			r.UtilScore = UtilScore(int(r.UtilPct))
		}
		weighted := float64(r.UtilScore) * r.Weights.Util
		if r.OnTimeScore >= 0 {
			weighted += float64(r.OnTimeScore) * r.Weights.OnTime
		}
		if r.AgeScore >= 0 {
			weighted += float64(r.AgeScore) * r.Weights.Age
		}
		r.ProxyScore = clamp(int(weighted))
		r.ProxyKnown = true
	} else {
		r.Balance = money.Zero(rates.Base)
		r.Limit = money.Zero(rates.Base)
		r.UtilPct = -1
	}
	if r.PayTo30Err != nil {
		r.PayTo30 = money.Zero(rates.Base)
	}
	if r.PayTo10Err != nil {
		r.PayTo10 = money.Zero(rates.Base)
	}
	return r
}
