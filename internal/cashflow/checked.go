// SPDX-License-Identifier: MIT

package cashflow

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// MaxCheckedProjectionDays bounds checked daily projections.
const MaxCheckedProjectionDays = 3660

// MaxCheckedProjectionEvents bounds events accumulated by a checked projection.
const MaxCheckedProjectionEvents = 100_000

// DailyBalancesChecked applies the same daily event and breach policy as
// DailyBalances, returning an error instead of wrapping when any money operation
// overflows. It returns no partial projection on error.
func DailyBalancesChecked(startBal int64, events []Event, days int, buffer int64) (Projection, error) {
	if days <= 0 {
		p := Projection{BreachDay: -1, MinBalance: startBal}
		return p, nil
	}
	if days > MaxCheckedProjectionDays || len(events) > MaxCheckedProjectionEvents {
		return Projection{}, fmt.Errorf("cashflow: checked projection exceeds limits (%d days, %d events)", MaxCheckedProjectionDays, MaxCheckedProjectionEvents)
	}
	daily := make([]int64, days)
	for _, event := range events {
		if event.Day < 0 || event.Day >= days {
			continue
		}
		next, err := money.Add(daily[event.Day], event.Amount)
		if err != nil {
			return Projection{}, fmt.Errorf("cashflow: day %d event total: %w", event.Day, err)
		}
		daily[event.Day] = next
	}

	p := Projection{Daily: make([]DailyBalance, days), BreachDay: -1, MinBalance: startBal, MinDay: 0}
	balance := startBal
	for day, amount := range daily {
		next, err := money.Add(balance, amount)
		if err != nil {
			return Projection{}, fmt.Errorf("cashflow: day %d balance: %w", day, err)
		}
		balance = next
		p.Daily[day] = DailyBalance{Day: day, Balance: balance}
		if balance < p.MinBalance {
			p.MinBalance, p.MinDay = balance, day
		}
		if p.BreachDay < 0 && balance < buffer {
			shortfall, err := money.Sub(buffer, balance)
			if err != nil {
				return Projection{}, fmt.Errorf("cashflow: day %d breach shortfall: %w", day, err)
			}
			p.BreachDay, p.BreachShortfall = day, shortfall
		}
	}
	return p, nil
}
