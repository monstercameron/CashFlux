// SPDX-License-Identifier: MIT

package currency

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestMinorFromMajor(t *testing.T) {
	tests := []struct {
		major float64
		code  string
		want  int64
	}{
		{12.34, "USD", 1234},
		{0.005, "USD", 1},     // rounds to nearest cent (math.Round, half away from zero)
		{12.344, "USD", 1234}, // rounds down
		{1000, "JPY", 1000},   // 0 decimals -> whole units, not *100
		{1.5, "JPY", 2},       // 0 decimals -> rounds to nearest whole
		{-4.20, "USD", -420},  // negative
		{5, "ZZZ", 500},       // unknown code defaults to 2 decimals
	}
	for _, tt := range tests {
		if got := MinorFromMajor(tt.major, tt.code); got != tt.want {
			t.Errorf("MinorFromMajor(%v, %q) = %d, want %d", tt.major, tt.code, got, tt.want)
		}
	}
}

func TestMajorFromMinor(t *testing.T) {
	tests := []struct {
		want float64
		code string
		in   int64
	}{
		{want: 12.34, code: "USD", in: 1234},
		{want: 1000, code: "JPY", in: 1000}, // 0 decimals -> not /100
		{want: -4.20, code: "USD", in: -420},
		{want: 5, code: "ZZZ", in: 500}, // unknown -> 2 decimals
		{want: 0, code: "USD", in: 0},
	}
	for _, tt := range tests {
		got := MajorFromMinor(tt.in, tt.code)
		if d := got - tt.want; d > 1e-9 || d < -1e-9 {
			t.Errorf("MajorFromMinor(%d, %q) = %v, want %v", tt.in, tt.code, got, tt.want)
		}
	}
	// Round-trip with MinorFromMajor.
	if got := MinorFromMajor(MajorFromMinor(1234, "USD"), "USD"); got != 1234 {
		t.Errorf("round-trip USD = %d, want 1234", got)
	}
}

func TestCodesSortedAndRegistered(t *testing.T) {
	codes := Codes()
	if len(codes) < 5 {
		t.Fatalf("expected several registered codes, got %d", len(codes))
	}
	for i := 1; i < len(codes); i++ {
		if codes[i-1] >= codes[i] {
			t.Errorf("codes not strictly sorted at %d: %q >= %q", i, codes[i-1], codes[i])
		}
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if _, ok := Lookup(c); !ok {
			t.Errorf("Codes returned unregistered %q", c)
		}
		seen[c] = true
	}
	if !seen["USD"] || !seen["EUR"] {
		t.Error("expected USD and EUR among codes")
	}
}

func TestLookupAndDefaults(t *testing.T) {
	if c, ok := Lookup("usd"); !ok || c.Decimals != 2 || c.Symbol != "$" {
		t.Fatalf("Lookup(usd) = %+v, ok=%v", c, ok)
	}
	if d := Decimals("JPY"); d != 0 {
		t.Errorf("Decimals(JPY) = %d, want 0", d)
	}
	if d := Decimals("ZZZ"); d != defaultDecimals {
		t.Errorf("Decimals(unknown) = %d, want %d", d, defaultDecimals)
	}
	if s := Symbol("ZZZ"); s != "ZZZ" {
		t.Errorf("Symbol(unknown) = %q, want ZZZ", s)
	}
}

func TestConvertSameCurrency(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{}}
	in := money.New(12345, "USD")
	got, err := r.Convert(in, "USD")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !got.Equal(in) {
		t.Errorf("Convert same = %v, want %v", got, in)
	}
}

func TestConvertToBase(t *testing.T) {
	// 1 EUR = 1.10 USD. 100.00 EUR -> 110.00 USD.
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	got, err := r.ToBase(money.New(10000, "EUR"))
	if err != nil {
		t.Fatalf("ToBase error: %v", err)
	}
	if !got.Equal(money.New(11000, "USD")) {
		t.Errorf("ToBase = %v, want 11000 USD", got)
	}
}

func TestConvertCrossCurrencyViaBase(t *testing.T) {
	// Base USD. 1 EUR = 1.10 USD, 1 GBP = 1.25 USD.
	// 50.00 EUR = 55.00 USD = 44.00 GBP.
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10, "GBP": 1.25}}
	got, err := r.Convert(money.New(5000, "EUR"), "GBP")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !got.Equal(money.New(4400, "GBP")) {
		t.Errorf("Convert = %v, want 4400 GBP", got)
	}
}

func TestConvertDifferentDecimals(t *testing.T) {
	// Base USD. 1 USD = 150 JPY (so Rates["JPY"] = 1/150 USD per yen).
	// 10.00 USD -> 1500 JPY (JPY has 0 decimals).
	r := Rates{Base: "USD", Rates: map[string]float64{"JPY": 1.0 / 150.0}}
	got, err := r.Convert(money.New(1000, "USD"), "JPY")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !got.Equal(money.New(1500, "JPY")) {
		t.Errorf("Convert = %v, want 1500 JPY", got)
	}
}

func TestConvertMissingRate(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{}}
	if _, err := r.Convert(money.New(100, "EUR"), "USD"); !errors.Is(err, ErrUnknownRate) {
		t.Errorf("err = %v, want ErrUnknownRate", err)
	}
	if _, err := r.Convert(money.New(100, "USD"), "EUR"); !errors.Is(err, ErrUnknownRate) {
		t.Errorf("target err = %v, want ErrUnknownRate", err)
	}
}

func TestConvertNonPositiveRate(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 0}}
	if _, err := r.Convert(money.New(100, "EUR"), "USD"); err == nil {
		t.Error("expected error for non-positive rate")
	}
}

func TestConvertRoundsToNearestMinor(t *testing.T) {
	// Conversion rounds to the target currency's nearest minor unit. (Exact
	// half-cents are not testable here because float64 rates can't represent
	// them precisely — see the .49/.60 cases below, which are float-stable.)
	tests := []struct {
		name string
		rate float64 // 1 EUR = rate USD
		want int64   // minor units after converting 1.00 EUR
	}{
		{"rounds down", 1.2349, 123}, // 123.49 -> 123
		{"rounds up", 1.236, 124},    // 123.60 -> 124
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Rates{Base: "USD", Rates: map[string]float64{"EUR": tt.rate}}
			got, err := r.Convert(money.New(100, "EUR"), "USD")
			if err != nil {
				t.Fatalf("Convert error: %v", err)
			}
			if got.Amount != tt.want {
				t.Errorf("amount = %d, want %d", got.Amount, tt.want)
			}
		})
	}
}

func TestConvertRoundsNegativeAmounts(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.236}}
	got, err := r.Convert(money.New(-100, "EUR"), "USD")
	if err != nil {
		t.Fatalf("Convert error: %v", err)
	}
	if !got.Equal(money.New(-124, "USD")) {
		t.Errorf("Convert negative = %v, want -124 USD", got)
	}
}

func TestConvertStableAcrossRepeatedCalls(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10, "GBP": 1.25}}
	in := money.New(5000, "EUR")
	want := money.New(4400, "GBP")
	for i := range 5 {
		got, err := r.Convert(in, "GBP")
		if err != nil {
			t.Fatalf("Convert call %d error: %v", i, err)
		}
		if !got.Equal(want) {
			t.Fatalf("Convert call %d = %v, want %v", i, got, want)
		}
		if !in.Equal(money.New(5000, "EUR")) {
			t.Fatalf("input mutated after call %d: %v", i, in)
		}
	}
}

// TestSymbolDisambiguation locks the C85 display-symbol fixes so that CAD, AUD,
// and MXN are never silently aliased to "$" again.
func TestSymbolDisambiguation(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"USD", "$"},
		{"CAD", "CA$"},
		{"AUD", "A$"},
		{"MXN", "MX$"},
	}
	for _, tt := range tests {
		if got := Symbol(tt.code); got != tt.want {
			t.Errorf("Symbol(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// TestConventionJPYRegressionC77 locks the FX-rate convention so the
// inverted-JPY bug cannot regress silently.
//
// Convention: Rates[code] = (USD per 1 foreign major unit).
// JPY has 0 decimal places, so 100 JPY = 100 minor units.
// At ~0.0066 USD/JPY (i.e. 1 JPY ≈ 0.66 US cents), 100 JPY ≈ $0.66 USD.
//
// The inverted value (151.0 USD/JPY) would produce a wildly large result,
// proving the convention is detectable by inspection.
func TestConventionJPYRegressionC77(t *testing.T) {
	// JPY has 0 decimal places: 100 JPY is represented as 100 minor units.
	jpyMinorUnits := int64(100) // 100.00 JPY

	t.Run("correct_rate_yields_small_USD", func(t *testing.T) {
		// Rates["JPY"] = 0.0066 means 1 JPY = 0.0066 USD.
		// 100 JPY * 0.0066 = 0.66 USD = 66 cents.
		r := Rates{Base: "USD", Rates: map[string]float64{"JPY": 0.0066}}
		got, err := ConvertBetween(jpyMinorUnits, "JPY", "USD", r)
		if err != nil {
			t.Fatalf("ConvertBetween error: %v", err)
		}
		// Expect 66 cents (±1 cent for rounding).
		if got < 65 || got > 67 {
			t.Errorf("JPY→USD (correct rate 0.0066): got %d cents, want ~66", got)
		}
	})

	t.Run("inverted_rate_yields_large_USD", func(t *testing.T) {
		// If someone mistakenly stores the rate as JPY-per-USD (e.g. 151.0),
		// ConvertBetween would produce a wildly large number — ~15100 cents
		// ($151.00) for only 100 JPY.  This sub-test documents that failure mode
		// so reviewers can distinguish it from the correct ~66-cent result above.
		r := Rates{Base: "USD", Rates: map[string]float64{"JPY": 151.0}}
		got, err := ConvertBetween(jpyMinorUnits, "JPY", "USD", r)
		if err != nil {
			t.Fatalf("ConvertBetween error: %v", err)
		}
		// With the inverted rate the result must be orders of magnitude larger.
		if got < 10000 {
			t.Errorf("inverted-rate sanity: got %d cents, expected ≥10000 to confirm the bug is detectable", got)
		}
	})
}
