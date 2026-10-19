// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

// ErrDivisionByZero is returned when a divisor is zero.
var ErrDivisionByZero = errors.New("money: division by zero")

// ErrRoundingMode is returned for an unsupported rounding mode.
var ErrRoundingMode = errors.New("money: invalid rounding mode")

// RoundingMode specifies how a fractional minor unit becomes an integer.
// Callers must choose a mode explicitly; zero is not a valid mode.
type RoundingMode uint8

const (
	// RoundTowardZero discards the fractional part.
	RoundTowardZero RoundingMode = iota + 1
	// RoundAwayFromZero rounds any fractional part away from zero.
	RoundAwayFromZero
	// RoundFloor rounds toward negative infinity.
	RoundFloor
	// RoundCeiling rounds toward positive infinity.
	RoundCeiling
	// RoundHalfAwayFromZero rounds to nearest, with ties away from zero.
	// This is CashFlux's conversion and allocation rounding policy.
	RoundHalfAwayFromZero
	// RoundHalfEven rounds to nearest, with ties to the even integer.
	RoundHalfEven
)

// Add returns a+b, or ErrOverflow if the result does not fit in int64.
func Add(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, fmt.Errorf("%w: %d + %d", ErrOverflow, a, b)
	}
	return a + b, nil
}

// Sub returns a-b, or ErrOverflow if the result does not fit in int64.
func Sub(a, b int64) (int64, error) {
	if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		return 0, fmt.Errorf("%w: %d - %d", ErrOverflow, a, b)
	}
	return a - b, nil
}

// Neg returns -a. Negating MinInt64 returns ErrOverflow.
func Neg(a int64) (int64, error) {
	if a == math.MinInt64 {
		return 0, fmt.Errorf("%w: negate %d", ErrOverflow, a)
	}
	return -a, nil
}

// Abs returns the magnitude of a. MinInt64 returns ErrOverflow.
func Abs(a int64) (int64, error) {
	if a < 0 {
		return Neg(a)
	}
	return a, nil
}

// Mul returns a*b, or ErrOverflow if the result does not fit in int64.
func Mul(a, b int64) (int64, error) {
	return MulDiv(a, b, 1, RoundTowardZero)
}

// MulDiv returns a*num/den rounded once according to mode. The product uses a
// wide intermediate; only the final rounded result must fit in int64. Negative
// divisors are supported. Zero divisors and invalid modes return errors even
// when a or num is zero.
func MulDiv(a, num, den int64, mode RoundingMode) (int64, error) {
	var product, multiplier, divisor big.Int
	product.SetInt64(a)
	multiplier.SetInt64(num)
	product.Mul(&product, &multiplier)
	divisor.SetInt64(den)
	result, err := roundQuotient(&product, &divisor, mode)
	if err != nil {
		return 0, fmt.Errorf("money: multiply %d by %d / %d: %w", a, num, den, err)
	}
	return result, nil
}

// roundQuotient is the single fractional rounding implementation. It does not
// mutate its inputs. QuoRem truncates toward zero for either divisor sign.
func roundQuotient(num, den *big.Int, mode RoundingMode) (int64, error) {
	if mode < RoundTowardZero || mode > RoundHalfEven {
		return 0, ErrRoundingMode
	}
	if den.Sign() == 0 {
		return 0, ErrDivisionByZero
	}
	var q, r big.Int
	q.QuoRem(num, den, &r)
	if r.Sign() != 0 {
		sign := num.Sign() * den.Sign()
		increment := false
		switch mode {
		case RoundAwayFromZero:
			increment = true
		case RoundFloor:
			increment = sign < 0
		case RoundCeiling:
			increment = sign > 0
		case RoundHalfAwayFromZero, RoundHalfEven:
			var twiceR, absDen big.Int
			twiceR.Abs(&r)
			twiceR.Lsh(&twiceR, 1)
			absDen.Abs(den)
			cmp := twiceR.Cmp(&absDen)
			increment = cmp > 0 || (cmp == 0 && (mode == RoundHalfAwayFromZero || q.Bit(0) != 0))
		}
		if increment {
			q.Add(&q, big.NewInt(int64(sign)))
		}
	}
	if !q.IsInt64() {
		return 0, ErrOverflow
	}
	return q.Int64(), nil
}

// CeilDiv returns the mathematical ceiling of a/den using quotient/remainder.
// Either sign is supported. Division by zero and MinInt64/-1 return errors.
func CeilDiv(a, den int64) (int64, error) {
	if den == 0 {
		return 0, ErrDivisionByZero
	}
	if a == math.MinInt64 && den == -1 {
		return 0, ErrOverflow
	}
	q, r := a/den, a%den
	if r != 0 && (a < 0) == (den < 0) {
		return Add(q, 1)
	}
	return q, nil
}

// SumMinor adds minor-unit integers in argument order, checking every partial
// sum. Callers must ensure that all values use the same currency and precision.
// With no arguments it returns zero.
func SumMinor(values ...int64) (int64, error) {
	var total int64
	for i, value := range values {
		var err error
		total, err = Add(total, value)
		if err != nil {
			return 0, fmt.Errorf("money: sum at index %d: %w", i, err)
		}
	}
	return total, nil
}
