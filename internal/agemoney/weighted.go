// SPDX-License-Identifier: MIT

package agemoney

import (
	"math"
	"math/big"
)

// weightedAmount keeps the common case in int64. Dollar-days can exceed that
// range even when the final spending total and average are both representable.
type weightedAmount struct {
	small int64
	wide  *big.Int
}

// addProduct accepts nonnegative matched amounts and ages.
func (sum *weightedAmount) addProduct(amount, age int64) {
	if sum.wide == nil && (age == 0 || amount <= math.MaxInt64/age) {
		product := amount * age
		if sum.small <= math.MaxInt64-product {
			sum.small += product
			return
		}
	}
	var left, right, product big.Int
	product.Mul(left.SetInt64(amount), right.SetInt64(age))
	wide := sum.promote()
	wide.Add(wide, &product)
}

func (sum *weightedAmount) promote() *big.Int {
	if sum.wide == nil {
		sum.wide = big.NewInt(sum.small)
	}
	return sum.wide
}

func (sum *weightedAmount) add(value weightedAmount) {
	if value.wide == nil {
		sum.addProduct(value.small, 1)
		return
	}
	wide := sum.promote()
	wide.Add(wide, value.wide)
}

// rounded implements (weighted + matched/2) / matched without overflowing the
// rounding increment. The caller supplies a positive, representable denominator.
func (sum weightedAmount) rounded(matched int64) int64 {
	if sum.wide == nil {
		whole, remainder := sum.small/matched, sum.small%matched
		if remainder >= matched-matched/2 {
			whole++
		}
		return whole
	}
	var numerator, half, denominator big.Int
	numerator.Add(sum.wide, half.SetInt64(matched/2))
	numerator.Quo(&numerator, denominator.SetInt64(matched))
	return numerator.Int64() // A weighted mean is bounded by the caller's int maxAge.
}
