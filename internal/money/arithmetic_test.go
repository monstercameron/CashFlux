// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"testing"
)

func TestCheckedArithmetic(t *testing.T) {
	tests := []struct {
		name       string
		fn         func(int64, int64) (int64, error)
		a, b, want int64
		wantErr    error
	}{
		{"add max", Add, math.MaxInt64, 0, math.MaxInt64, nil},
		{"add positive overflow", Add, math.MaxInt64, 1, 0, ErrOverflow},
		{"add negative overflow", Add, math.MinInt64, -1, 0, ErrOverflow},
		{"add opposite extrema", Add, math.MinInt64, math.MaxInt64, -1, nil},
		{"sub min", Sub, math.MinInt64, 0, math.MinInt64, nil},
		{"sub positive overflow", Sub, math.MaxInt64, -1, 0, ErrOverflow},
		{"sub negative overflow", Sub, math.MinInt64, 1, 0, ErrOverflow},
		{"sub min from min", Sub, math.MinInt64, math.MinInt64, 0, nil},
		{"sub min from negative", Sub, -1, math.MinInt64, math.MaxInt64, nil},
		{"multiply zero", Mul, math.MinInt64, 0, 0, nil},
		{"multiply identity", Mul, math.MinInt64, 1, math.MinInt64, nil},
		{"multiply min by negative", Mul, math.MinInt64, -1, 0, ErrOverflow},
		{"multiply product overflow", Mul, math.MaxInt64, 2, 0, ErrOverflow},
		{"multiply signs", Mul, -7, -3, 21, nil},
		{"ceiling max", CeilDiv, math.MaxInt64, 2, 4611686018427387904, nil},
		{"ceiling min", CeilDiv, math.MinInt64, 3, -3074457345618258602, nil},
		{"ceiling negative denominator", CeilDiv, 7, -3, -2, nil},
		{"ceiling both negative", CeilDiv, -7, -3, 3, nil},
		{"ceiling exact", CeilDiv, -6, 3, -2, nil},
		{"ceiling unit fraction", CeilDiv, -1, math.MinInt64, 1, nil},
		{"ceiling zero", CeilDiv, 0, -3, 0, nil},
		{"ceiling overflow", CeilDiv, math.MinInt64, -1, 0, ErrOverflow},
		{"ceiling divide by zero", CeilDiv, 0, 0, 0, ErrDivisionByZero},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn(tt.a, tt.b)
			if got != tt.want || !errors.Is(err, tt.wantErr) {
				t.Fatalf("got (%d, %v), want (%d, %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
	for _, v := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64} {
		neg, ne := Neg(v)
		abs, ae := Abs(v)
		if v == math.MinInt64 {
			if !errors.Is(ne, ErrOverflow) || !errors.Is(ae, ErrOverflow) {
				t.Fatal("minimum magnitude must overflow")
			}
		} else if ne != nil || ae != nil || neg != -v || abs < 0 || (v < 0 && abs != -v) || (v >= 0 && abs != v) {
			t.Fatalf("Neg/Abs(%d) = (%d,%v)/(%d,%v)", v, neg, ne, abs, ae)
		}
	}
}

func TestMulDivRounding(t *testing.T) {
	tests := []struct {
		a, num, den int64
		mode        RoundingMode
		want        int64
		err         error
	}{
		{1, 1, 2, RoundHalfAwayFromZero, 1, nil},
		{-1, 1, 2, RoundHalfAwayFromZero, -1, nil},
		{1, 1, -2, RoundHalfAwayFromZero, -1, nil},
		{-1, 1, -2, RoundHalfAwayFromZero, 1, nil},
		{5, 1, 2, RoundHalfEven, 2, nil},
		{-5, 1, 2, RoundHalfEven, -2, nil},
		{7, 1, 2, RoundHalfEven, 4, nil},
		{-7, 1, 2, RoundHalfEven, -4, nil},
		{5, 1, 3, RoundFloor, 1, nil},
		{-5, 1, 3, RoundFloor, -2, nil},
		{5, 1, 3, RoundCeiling, 2, nil},
		{-5, 1, 3, RoundCeiling, -1, nil},
		{5, 1, 3, RoundTowardZero, 1, nil},
		{-5, 1, 3, RoundTowardZero, -1, nil},
		{1, 1, 3, RoundAwayFromZero, 1, nil},
		{-1, 1, 3, RoundAwayFromZero, -1, nil},
		{math.MaxInt64, math.MaxInt64, math.MaxInt64, RoundHalfAwayFromZero, math.MaxInt64, nil},
		{math.MinInt64, math.MinInt64, math.MinInt64, RoundHalfAwayFromZero, math.MinInt64, nil},
		{math.MaxInt64, 2, 2, RoundHalfAwayFromZero, math.MaxInt64, nil},
		{math.MinInt64, -1, 1, RoundHalfAwayFromZero, 0, ErrOverflow},
		{0, 0, 0, RoundHalfAwayFromZero, 0, ErrDivisionByZero},
		{0, 0, 1, 0, 0, ErrRoundingMode},
		{1, 1, 1, 255, 0, ErrRoundingMode},
	}
	for _, tt := range tests {
		got, err := MulDiv(tt.a, tt.num, tt.den, tt.mode)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("MulDiv(%d,%d,%d,%d) = (%d,%v), want (%d,%v)", tt.a, tt.num, tt.den, tt.mode, got, err, tt.want, tt.err)
		}
	}
	// Rounding alone can cause overflow after a representable truncated quotient.
	for _, mode := range []RoundingMode{RoundHalfAwayFromZero, RoundHalfEven, RoundAwayFromZero, RoundCeiling} {
		got, err := MulDiv(math.MaxInt64-2147483648, math.MaxInt64-2147483648, math.MaxInt64-4294967296, mode)
		if got != 0 || !errors.Is(err, ErrOverflow) {
			t.Errorf("rounded overflow: (%d,%v), mode %d", got, err, mode)
		}
	}
}

func TestCheckedMoneyMethods(t *testing.T) {
	m := New(math.MinInt64, "USD")
	for _, fn := range []func() (Money, error){m.NegChecked, m.AbsChecked} {
		got, err := fn()
		if got != (Money{}) || !errors.Is(err, ErrOverflow) {
			t.Fatalf("got (%v,%v)", got, err)
		}
	}
	for _, fn := range []func() Money{m.Neg, m.Abs} {
		func() {
			defer func() {
				value := recover()
				err, ok := value.(error)
				if !ok || !errors.Is(err, ErrOverflow) {
					t.Errorf("legacy overflow panic = %v", value)
				}
			}()
			fn()
		}()
	}
	for _, v := range []int64{-250, 0, 250} {
		m := New(v, "EUR")
		a, ae := m.AbsChecked()
		n, ne := m.NegChecked()
		if ae != nil || ne != nil || a != m.Abs() || n != m.Neg() || a.Currency != "EUR" || n.Currency != "EUR" {
			t.Fatal("checked currency/preservation")
		}
	}
	got, err := New(5, "JPY").MulDiv(1, 2, RoundHalfAwayFromZero)
	if err != nil || got != New(3, "JPY") {
		t.Fatalf("MulDiv = (%v,%v)", got, err)
	}
	got, err = New(-5, "KWD").Mul(3)
	if err != nil || got != New(-15, "KWD") {
		t.Fatalf("Mul = (%v,%v)", got, err)
	}
	got, err = New(math.MaxInt64, "USD").Mul(2)
	if got != (Money{}) || !errors.Is(err, ErrOverflow) {
		t.Fatal("Money.Mul must propagate overflow")
	}
	got, err = New(1, "USD").MulDiv(1, 0, RoundHalfAwayFromZero)
	if got != (Money{}) || !errors.Is(err, ErrDivisionByZero) {
		t.Fatal("Money.MulDiv must propagate division error")
	}
}

func TestCheckedSums(t *testing.T) {
	for _, values := range [][]int64{nil, {math.MaxInt64, -math.MaxInt64, 1}, {math.MinInt64, math.MaxInt64}, {math.MaxInt64, 1, -1}, {math.MinInt64, -1, 1}} {
		monies := make([]Money, len(values))
		var expected big.Int
		overflow := false
		for i, v := range values {
			monies[i] = New(v, "USD")
			expected.Add(&expected, big.NewInt(v))
			overflow = overflow || !expected.IsInt64()
		}
		got, err := SumMinor(values...)
		mg, me := Sum(monies...)
		if overflow {
			if !errors.Is(err, ErrOverflow) || !errors.Is(me, ErrOverflow) {
				t.Fatal("sum must check every intermediate")
			}
		} else if err != nil || me != nil || got != expected.Int64() || mg.Amount != got {
			t.Fatalf("sum %v = (%d,%v)/(%v,%v)", values, got, err, mg, me)
		}
	}
}

// referenceRound uses rational floor and distances to adjacent integers, rather
// than the production truncation/sign-based rounding implementation.
func referenceRound(r *big.Rat, mode RoundingMode) (*big.Int, error) {
	if mode < RoundTowardZero || mode > RoundHalfEven {
		return nil, ErrRoundingMode
	}
	var floor, rem big.Int
	floor.DivMod(r.Num(), r.Denom(), &rem)
	if rem.Sign() == 0 {
		return &floor, nil
	}
	ceil := new(big.Int).Add(&floor, big.NewInt(1))
	switch mode {
	case RoundFloor:
		return &floor, nil
	case RoundCeiling:
		return ceil, nil
	case RoundTowardZero:
		if r.Sign() < 0 {
			return ceil, nil
		}
		return &floor, nil
	case RoundAwayFromZero:
		if r.Sign() < 0 {
			return &floor, nil
		}
		return ceil, nil
	default:
		lowDistance := new(big.Rat).Sub(r, new(big.Rat).SetInt(&floor))
		highDistance := new(big.Rat).Sub(new(big.Rat).SetInt(ceil), r)
		cmp := lowDistance.Cmp(highDistance)
		if cmp < 0 {
			return &floor, nil
		}
		if cmp > 0 {
			return ceil, nil
		}
		if mode == RoundHalfEven {
			if floor.Bit(0) == 0 {
				return &floor, nil
			}
			return ceil, nil
		}
		if r.Sign() < 0 {
			return &floor, nil
		}
		return ceil, nil
	}
}

func checkIntegerResult(t *testing.T, name string, got int64, err error, want *big.Int) {
	t.Helper()
	if !want.IsInt64() {
		if got != 0 || !errors.Is(err, ErrOverflow) {
			t.Fatalf("%s = (%d,%v), want overflow for %s", name, got, err, want)
		}
	} else if err != nil || got != want.Int64() {
		t.Fatalf("%s = (%d,%v), want %s", name, got, err, want)
	}
}

func checkArithmeticReference(t *testing.T, a, b int64) {
	t.Helper()
	aa, bb := big.NewInt(a), big.NewInt(b)
	got, err := Add(a, b)
	checkIntegerResult(t, "Add", got, err, new(big.Int).Add(aa, bb))
	got, err = Sub(a, b)
	checkIntegerResult(t, "Sub", got, err, new(big.Int).Sub(aa, bb))
	got, err = Mul(a, b)
	checkIntegerResult(t, "Mul", got, err, new(big.Int).Mul(aa, bb))
	got, err = Neg(a)
	checkIntegerResult(t, "Neg", got, err, new(big.Int).Neg(aa))
	got, err = Abs(a)
	checkIntegerResult(t, "Abs", got, err, new(big.Int).Abs(aa))
	got, err = CeilDiv(a, b)
	if b == 0 {
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatal("CeilDiv zero denominator")
		}
		return
	}
	want, _ := referenceRound(new(big.Rat).SetFrac(aa, bb), RoundCeiling)
	checkIntegerResult(t, "CeilDiv", got, err, want)
}

func checkMulDivReference(t *testing.T, a, num, den int64, mode RoundingMode) {
	t.Helper()
	got, err := MulDiv(a, num, den, mode)
	if mode < RoundTowardZero || mode > RoundHalfEven {
		if !errors.Is(err, ErrRoundingMode) {
			t.Fatal("invalid rounding mode")
		}
		return
	}
	if den == 0 {
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatal("zero denominator")
		}
		return
	}
	ratio := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(a), big.NewInt(num)), big.NewInt(den))
	want, _ := referenceRound(ratio, mode)
	checkIntegerResult(t, "MulDiv", got, err, want)
}

func TestArithmeticBigReference(t *testing.T) {
	extrema := []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -2, -1, 0, 1, 2, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
	for _, a := range extrema {
		for _, b := range extrema {
			checkArithmeticReference(t, a, b)
			for _, den := range extrema {
				for mode := RoundTowardZero; mode <= RoundHalfEven; mode++ {
					checkMulDivReference(t, a, b, den, mode)
				}
			}
		}
	}
	random := rand.New(rand.NewPCG(19, 37))
	for range 10000 {
		checkArithmeticReference(t, int64(random.Uint64()), int64(random.Uint64()))
		checkMulDivReference(t, int64(random.Uint64()), int64(random.Uint64()), int64(random.Uint64()), RoundingMode(1+random.IntN(6)))
	}
}

func FuzzArithmetic(f *testing.F) {
	for _, a := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64} {
		for _, b := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
			f.Add(a, b)
		}
	}
	f.Fuzz(func(t *testing.T, a, b int64) { checkArithmeticReference(t, a, b) })
}

func FuzzMulDiv(f *testing.F) {
	for _, a := range []int64{math.MinInt64, -5, -1, 0, 1, 5, math.MaxInt64} {
		for mode := uint8(0); mode <= 7; mode++ {
			f.Add(a, int64(2), int64(3), mode)
		}
	}
	f.Add(int64(math.MinInt64), int64(-1), int64(1), uint8(RoundHalfAwayFromZero))
	f.Add(int64(math.MaxInt64), int64(math.MaxInt64), int64(math.MaxInt64), uint8(RoundHalfAwayFromZero))
	f.Fuzz(func(t *testing.T, a, num, den int64, mode uint8) {
		checkMulDivReference(t, a, num, den, RoundingMode(mode))
	})
}
