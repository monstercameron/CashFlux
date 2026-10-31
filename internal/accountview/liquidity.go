// SPDX-License-Identifier: MIT

package accountview

import (
	"errors"
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
	// Overflow reports that a conversion or a bucket total did not fit in int64.
	// The balance is left out and the view is Incomplete, but its currency is not
	// listed in ExcludedCurrencies, since a rate was not what failed.
	Overflow bool
}

// Bucket is one non-empty liquidity class and its base-currency total.
type Bucket struct {
	Class  liquidity.Class
	Amount int64
}

// bucketOrder is the order the buckets read in, from most to least spendable.
var bucketOrder = []liquidity.Class{liquidity.Available, liquidity.Restricted, liquidity.Investments, liquidity.Held}

// Buckets returns the non-empty buckets in reading order.
func (v LiquidityView) Buckets() []Bucket {
	var out []Bucket
	for _, class := range bucketOrder {
		if amount := v.Totals[class]; amount != 0 {
			out = append(out, Bucket{Class: class, Amount: amount})
		}
	}
	return out
}

// Visible reports whether the breakdown teaches anything: at least two buckets hold
// money, or some balance could not be counted and the gap needs disclosing. A
// single-bucket household learns nothing from the split.
func (v LiquidityView) Visible() bool {
	return len(v.Buckets()) >= 2 || v.Incomplete
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
		if valid && errors.Is(err, money.ErrOverflow) {
			v.Incomplete, v.Overflow = true, true
			continue
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
