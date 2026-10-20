// SPDX-License-Identifier: MIT

package vitals

import (
	"fmt"
	"math"

	"github.com/monstercameron/CashFlux/internal/money"
)

type checkedCalculation struct{ err error }

func (c *checkedCalculation) take(name string, value int64, err error) int64 {
	if c.err == nil && err != nil {
		c.err = fmt.Errorf("vitals %s: %w", name, err)
	}
	return value
}

func (c *checkedCalculation) add(name string, a, b int64) int64 {
	value, err := money.Add(a, b)
	return c.take(name, value, err)
}

func (c *checkedCalculation) sub(name string, a, b int64) int64 {
	value, err := money.Sub(a, b)
	return c.take(name, value, err)
}

func (c *checkedCalculation) mul(name string, a, b int64) int64 {
	value, err := money.Mul(a, b)
	return c.take(name, value, err)
}

func (c *checkedCalculation) ratio(name string, amount, multiplier, divisor int64, mode money.RoundingMode) int64 {
	value, err := money.MulDiv(amount, multiplier, divisor, mode)
	return c.take(name, value, err)
}

func (c *checkedCalculation) integer(name string, value int64) int {
	if int64(int(value)) != value {
		c.take(name, 0, money.ErrOverflow)
		return 0
	}
	return int(value)
}

func (c *checkedCalculation) roundedInterest(value float64) int64 {
	value = math.Round(value)
	if math.IsNaN(value) || math.IsInf(value, 0) || value >= float64(math.MaxInt64) || value < float64(math.MinInt64) {
		return c.take("interest charge", 0, money.ErrOverflow)
	}
	return int64(value)
}

func (c *checkedCalculation) tenths(num, den int64) int64 {
	if den <= 0 || num <= 0 {
		return 0
	}
	return c.ratio("months of coverage", num, 10, den, money.RoundTowardZero)
}

func failedCalculation(err error) (Result, error) {
	return Result{Error: err}, err
}
