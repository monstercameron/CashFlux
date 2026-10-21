// SPDX-License-Identifier: MIT

package accountview

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/liquidity"
	"github.com/monstercameron/CashFlux/internal/money"
)

// LiquidityView groups convertible asset balances by current spendability.
// Invalid balances, FX conversions, or overflowing sums are explicitly omitted.
type LiquidityView struct {
	Totals             map[liquidity.Class]int64
	Incomplete         bool
	ExcludedCurrencies []string
}

// Liquidity reuses computed balances and evaluates lock expiry at the supplied
// instant. Archived accounts and liabilities do not contribute to the breakdown.
func Liquidity(accounts []domain.Account, balances map[string]money.Money, rates currency.Rates, base string, now time.Time) LiquidityView {
	v := LiquidityView{Totals: map[liquidity.Class]int64{}}
	excluded := map[string]bool{}
	for _, a := range accounts {
		if a.Archived || a.Class == domain.ClassLiability {
			continue
		}
		balance, valid := balances[a.ID]
		valid = valid && balance.Currency == a.Currency
		converted, err := rates.Convert(balance, base)
		class := liquidity.Of(a, now)
		var total int64
		if valid && err == nil {
			total, err = money.Add(v.Totals[class], converted.Amount)
		}
		if !valid || err != nil {
			v.Incomplete = true
			if a.Currency != "" {
				excluded[a.Currency] = true
			}
			continue
		}
		v.Totals[class] = total
	}
	for code := range excluded {
		v.ExcludedCurrencies = append(v.ExcludedCurrencies, code)
	}
	sort.Strings(v.ExcludedCurrencies)
	return v
}
