// SPDX-License-Identifier: MIT

package dashboardview

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/acctproject"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
)

// CashForecast is available cash today and at the 30, 60 and 90 day horizons,
// in base-currency minor units. Counted is the number of eligible accounts whose
// currency could be converted; a missing FX rate excludes the entire account.
type CashForecast struct {
	Today   int64
	Totals  [3]int64
	Counted int
}

// BuildCashForecast sums the account service's projections under one explicit
// scope and clock. project is the same account-projection function used by the
// accounts page, keeping future balances consistent across both presentations.
func BuildCashForecast(accounts []domain.Account, rates currency.Rates, now time.Time, project func(string, time.Time, int) acctproject.Projection) CashForecast {
	var out CashForecast
	for _, a := range accounts {
		if a.Archived || a.Class == domain.ClassLiability || liquidity.Of(a, now) != liquidity.Available {
			continue
		}
		first := project(a.ID, now, 30)
		start, err := rates.Convert(money.New(first.Start, a.Currency), rates.Base)
		if err != nil {
			continue
		}
		out.Today += start.Amount
		out.Counted++
		for i, horizon := range [3]int{30, 60, 90} {
			p := first
			if i > 0 {
				p = project(a.ID, now, horizon)
			}
			converted, err := rates.Convert(money.New(p.End, a.Currency), rates.Base)
			if err == nil {
				out.Totals[i] += converted.Amount
			}
		}
	}
	return out
}
