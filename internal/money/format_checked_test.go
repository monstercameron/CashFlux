// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

func TestParseMinorBoundaries(t *testing.T) {
	tests := []struct {
		s        string
		decimals int
		want     int64
		err      error
	}{
		{"-9223372036854775808", 0, math.MinInt64, nil},
		{"9223372036854775807", 0, math.MaxInt64, nil},
		{"-92233720368547758.08", 2, math.MinInt64, nil},
		{"92233720368547758.07", 2, math.MaxInt64, nil},
		{"92233720368547758.08", 2, 0, ErrOverflow},
		{"-92233720368547758.09", 2, 0, ErrOverflow},
		{"9223372036854775808", 0, 0, ErrOverflow},
		{"-9223372036854775809", 0, 0, ErrOverflow},
		{"9223372036854775807", 2, 0, ErrOverflow},
		{"92233720368547758070.0", 2, 0, ErrOverflow},
		{".0001", 4, 1, nil},
		{"-.5", 2, -50, nil},
		{"-0.00", 2, 0, nil},
		{"1.", 0, 1, nil},
		{"+1.", 2, 100, nil},
		{"1.230", 2, 0, ErrInvalidAmount},
		{"0.00000", 4, 0, ErrInvalidAmount},
		{"1", -1, 0, ErrInvalidPrecision},
		{"1", MaxDecimals + 1, 0, ErrInvalidPrecision},
		{"1", math.MaxInt, 0, ErrInvalidPrecision},
		{"1", math.MinInt, 0, ErrInvalidPrecision},
		{strings.Repeat("0", MaxAmountLength), 2, 0, nil},
		{strings.Repeat("0", MaxAmountLength+1), 2, 0, ErrInvalidAmount},
		{strings.Repeat(" ", MaxAmountLength) + "1", 2, 0, ErrInvalidAmount},
	}
	for _, tt := range tests {
		got, err := ParseMinor(tt.s, tt.decimals)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("ParseMinor(%q,%d) = (%d,%v), want (%d,%v)", tt.s, tt.decimals, got, err, tt.want, tt.err)
		}
		if err != nil && !errors.Is(err, ErrInvalidAmount) {
			t.Error("parse errors must match ErrInvalidAmount")
		}
	}
	for _, s := range []string{"", "+", "-", ".", "+.", "-.", "--5", "++5", "-+5", "1 2", "1\x002", "1e2", "0x10", "1/2", "1,000", "1..2", "١", "１.０", "NaN", "Inf", "1.2.3", "1\n2"} {
		if _, err := ParseMinor(s, 2); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("malformed %q: %v", s, err)
		}
	}
}

func TestFormatExtremaAndPrecision(t *testing.T) {
	if got := FormatMinor(math.MinInt64, 2); got != "-92233720368547758.08" {
		t.Fatal(got)
	}
	if got := FormatAccounting(math.MinInt64, 2, "$"); got != "($92,233,720,368,547,758.08)" {
		t.Fatal(got)
	}
	for _, decimals := range []int{-1, MaxDecimals + 1, math.MaxInt, math.MinInt} {
		if _, err := FormatMinorChecked(1, decimals); !errors.Is(err, ErrInvalidPrecision) {
			t.Fatal(err)
		}
		if _, err := New(1, "USD").FormatChecked(decimals); !errors.Is(err, ErrInvalidPrecision) {
			t.Fatal(err)
		}
		if _, err := FormatAccountingChecked(1, decimals, "$"); !errors.Is(err, ErrInvalidPrecision) {
			t.Fatal(err)
		}
		for _, fn := range []func(){func() { FormatMinor(1, decimals) }, func() { New(1, "USD").Format(decimals) }, func() { FormatAccounting(1, decimals, "$") }} {
			func() {
				defer func() {
					err, ok := recover().(error)
					if !ok || !errors.Is(err, ErrInvalidPrecision) {
						t.Errorf("legacy precision panic = %v", err)
					}
				}()
				fn()
			}()
		}
	}
	for d := 0; d <= MaxDecimals; d++ {
		for _, a := range []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -1, 0, 1, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64} {
			checkFormatReference(t, a, d)
		}
	}
	random := rand.New(rand.NewPCG(23, 71))
	for range 10000 {
		a := int64(random.Uint64())
		for d := 0; d <= 4; d++ {
			checkFormatReference(t, a, d)
		}
	}
}

func checkFormatReference(t *testing.T, a int64, d int) {
	t.Helper()
	got, err := FormatMinorChecked(a, d)
	if err != nil {
		t.Fatal(err)
	}
	// Independent division/remainder reference using an arbitrary-width magnitude.
	magnitude := new(big.Int).Abs(big.NewInt(a))
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d)), nil)
	var whole, frac big.Int
	whole.QuoRem(magnitude, divisor, &frac)
	want := whole.String()
	if d > 0 {
		want += "." + strings.Repeat("0", d-len(frac.String())) + frac.String()
	}
	if a < 0 {
		want = "-" + want
	}
	if got != want {
		t.Fatalf("Format(%d,%d) = %q, want %q", a, d, got, want)
	}
	parsed, err := ParseMinor(got, d)
	if err != nil || parsed != a {
		t.Fatalf("roundtrip %d,%d: %q -> (%d,%v)", a, d, got, parsed, err)
	}
	if FormatMinor(a, d) != got {
		t.Fatal("legacy formatter differs")
	}
	method, err := New(a, "USD").FormatChecked(d)
	if err != nil || method != got {
		t.Fatal("method formatter differs")
	}
	accounting, err := FormatAccountingChecked(a, d, "$")
	expectedAccounting := "$" + Group(strings.TrimPrefix(want, "-"))
	if a < 0 {
		expectedAccounting = "(" + expectedAccounting + ")"
	}
	if err != nil || accounting != expectedAccounting {
		t.Fatalf("accounting: %q,%v", accounting, err)
	}
}

var decimalGrammar = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)

func parseReference(s string, d int) (*big.Int, bool) {
	if d < 0 || d > MaxDecimals || len(s) > MaxAmountLength {
		return nil, false
	}
	s = strings.TrimSpace(s)
	if !decimalGrammar.MatchString(s) {
		return nil, false
	}
	fracDigits := 0
	if i := strings.IndexByte(s, '.'); i >= 0 {
		fracDigits = len(s) - i - 1
		s = s[:i] + s[i+1:]
	}
	if fracDigits > d {
		return nil, false
	}
	value, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, false
	}
	value.Mul(value, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(d-fracDigits)), nil))
	return value, value.IsInt64()
}

func FuzzParseMinor(f *testing.F) {
	for _, s := range []string{"-9223372036854775808", "92233720368547758.08", "-92233720368547758.08", ".5", "1.", "-0", "1.234", "--1", "1e2", "", strings.Repeat("9", 129)} {
		for d := 0; d <= 4; d++ {
			f.Add(s, d)
		}
	}
	f.Add("1", math.MaxInt)
	f.Add("1", -1)
	f.Fuzz(func(t *testing.T, s string, d int) {
		want, valid := parseReference(s, d)
		got, err := ParseMinor(s, d)
		if valid {
			if err != nil || got != want.Int64() {
				t.Fatalf("ParseMinor(%q,%d)=(%d,%v), want %s", s, d, got, err, want)
			}
		} else if got != 0 || !errors.Is(err, ErrInvalidAmount) {
			t.Fatalf("invalid ParseMinor(%q,%d)=(%d,%v)", s, d, got, err)
		}
	})
}

func FuzzFormatRoundTrip(f *testing.F) {
	for _, a := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64} {
		for d := uint8(0); d <= 4; d++ {
			f.Add(a, d)
		}
	}
	f.Fuzz(func(t *testing.T, a int64, d uint8) { checkFormatReference(t, a, int(d)%(MaxDecimals+1)) })
}
