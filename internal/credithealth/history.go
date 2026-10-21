// SPDX-License-Identifier: MIT

package credithealth

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// UtilizationHistoryLimit preserves the Credit page's latest-eight-record view.
const UtilizationHistoryLimit = 8

// ErrUnsupportedCurrency means a credit calculation cannot establish minor-unit
// precision from the supported currency registry.
var ErrUnsupportedCurrency = errors.New("credit health: unsupported currency")

// UtilizationHistoryPoint is one snapshot compared with the current credit limit
// and current saved FX. An erroneous point is a gap, never a zero reading.
type UtilizationHistoryPoint struct {
	ID                     string
	AsOf                   time.Time
	UtilPct                int64
	FillPercent            int
	Band                   UtilBand
	AssumedAccountCurrency bool
	Err                    error
}

// UtilizationHistory preserves selected snapshot identities and individual
// availability. Err includes invalid selected points or invalid model inputs.
// Ready requires two known points; MissingLimit differs from unavailable data.
type UtilizationHistory struct {
	Points                 []UtilizationHistoryPoint
	Total, Known           int
	Ready, MissingLimit    bool
	AssumedAccountCurrency bool
	Err                    error
}

// UtilizationChecked shares the checked model's magnitude, limit conversion and
// nearest-whole-percent rule, with explicit support checks for history inputs.
// The balance is already in the account currency; a foreign recorded limit is
// converted into that currency. Zero limit returns the no-limit sentinel -1.
func UtilizationChecked(balance, limit money.Money, rates currency.Rates) (int64, error) {
	if limit.Amount == 0 {
		return -1, nil
	}
	if limit.Amount < 0 {
		return -1, fmt.Errorf("credit history: credit limit must not be negative")
	}
	for _, code := range []string{balance.Currency, limit.Currency} {
		if !currency.Valid(code) {
			return -1, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, code)
		}
	}
	owed, err := balance.AbsChecked()
	if err != nil {
		return -1, err
	}
	converted, err := rates.Convert(limit, balance.Currency)
	if err != nil {
		return -1, err
	}
	if converted.Amount <= 0 {
		return -1, fmt.Errorf("credit history: converted credit limit is not positive")
	}
	return checkedUtilization(owed.Amount, converted.Amount)
}

// BuildUtilizationHistory copies and orders this account's snapshots, selects
// the latest eight before checking their values, and compares them with today's
// recorded limit using saved FX. It never mutates caller slices or hides a bad
// recent point by replacing it with an older valid point. Blank legacy snapshot
// currencies inherit the account currency with an explicit assumption flag.
func BuildUtilizationHistory(account domain.Account, snapshots []domain.BalanceSnapshot, rates currency.Rates) UtilizationHistory {
	var out UtilizationHistory
	for _, snapshot := range snapshots {
		if snapshot.AccountID == account.ID {
			out.Total++
		}
	}
	if account.CreditLimit.Amount == 0 {
		out.MissingLimit = true
		return out
	}
	// Validate the denominator even when no historical balances exist.
	if _, err := UtilizationChecked(money.Zero(account.Currency), account.CreditLimit, rates); err != nil {
		out.Err = err
		return out
	}
	selected := make([]domain.BalanceSnapshot, 0, out.Total)
	for _, snapshot := range snapshots {
		if snapshot.AccountID == account.ID {
			selected = append(selected, snapshot)
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		if !selected[i].AsOf.Equal(selected[j].AsOf) {
			return selected[i].AsOf.Before(selected[j].AsOf)
		}
		return selected[i].ID < selected[j].ID
	})
	if len(selected) > UtilizationHistoryLimit {
		selected = selected[len(selected)-UtilizationHistoryLimit:]
	}
	out.Points = make([]UtilizationHistoryPoint, 0, len(selected))
	for _, snapshot := range selected {
		point := UtilizationHistoryPoint{ID: snapshot.ID, AsOf: snapshot.AsOf, UtilPct: -1, Band: BandUtilNoData}
		code := snapshot.Currency
		if code == "" {
			code = account.Currency
			point.AssumedAccountCurrency = true
			out.AssumedAccountCurrency = true
		}
		if !currency.Valid(code) {
			point.Err = fmt.Errorf("%w: %q", ErrUnsupportedCurrency, code)
		}
		var balance money.Money
		if point.Err == nil {
			balance, point.Err = rates.Convert(money.New(snapshot.BalanceMinor, code), account.Currency)
		}
		if point.Err == nil {
			point.UtilPct, point.Err = UtilizationChecked(balance, account.CreditLimit, rates)
		}
		if point.Err != nil {
			point.Err = fmt.Errorf("credit history: snapshot %q: %w", snapshot.ID, point.Err)
			out.Err = errors.Join(out.Err, point.Err)
		} else {
			out.Known++
			if point.UtilPct > 100 {
				point.FillPercent = 100
			} else {
				point.FillPercent = int(point.UtilPct)
			}
			// Ratios above 80 all share the same worst band, so the int64 reading
			// never needs narrowing to the platform's int for threshold lookup.
			if point.UtilPct > 80 {
				point.Band = BandUtilWorst
			} else {
				point.Band = UtilBandFor(int(point.UtilPct))
			}
		}
		out.Points = append(out.Points, point)
	}
	out.Ready = out.Known >= 2
	return out
}
