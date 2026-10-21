// SPDX-License-Identifier: MIT

package currency

import (
	"errors"
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestConvertRejectsNonFiniteRates(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for _, pair := range [][2]string{{"EUR", "USD"}, {"USD", "EUR"}} {
			r := Rates{Base: "USD", Rates: map[string]float64{"EUR": rate}}
			got, err := r.Convert(money.New(100, pair[0]), pair[1])
			if !errors.Is(err, ErrInvalidRate) || got != (money.Money{}) {
				t.Errorf("%s -> %s at %v = %v, %v; want zero result and error", pair[0], pair[1], rate, got, err)
			}
		}
	}
}

func BenchmarkConvertOrdinary(b *testing.B) {
	rates := Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10, "GBP": 1.25}}
	m := money.New(1_234_567, "EUR")
	b.Run("legacy", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			got, err := legacyConvert(rates, m, "GBP")
			if err != nil || got.Amount != 1_086_419 {
				b.Fatalf("conversion = %v, %v", got, err)
			}
		}
	})
	b.Run("checked", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			got, err := rates.Convert(m, "GBP")
			if err != nil || got.Amount != 1_086_419 {
				b.Fatalf("conversion = %v, %v", got, err)
			}
		}
	})
}

// legacyConvert retains the previous conversion for a comparable benchmark.
func legacyConvert(r Rates, m money.Money, toCurrency string) (money.Money, error) {
	to := normalize(toCurrency)
	if m.Currency == to {
		return m, nil
	}
	lookup := func(code string) (float64, error) {
		code = normalize(code)
		if code == normalize(r.Base) {
			return 1, nil
		}
		rate, ok := r.Rates[code]
		if !ok {
			return 0, ErrUnknownRate
		}
		if rate <= 0 {
			return 0, ErrInvalidRate
		}
		return rate, nil
	}
	fromRate, err := lookup(m.Currency)
	if err != nil {
		return money.Money{}, err
	}
	toRate, err := lookup(to)
	if err != nil {
		return money.Money{}, err
	}
	fromMajor := float64(m.Amount) / pow10(Decimals(m.Currency))
	baseMajor := fromMajor * fromRate
	toMajor := baseMajor / toRate
	return money.New(int64(math.Round(toMajor*pow10(Decimals(to)))), to), nil
}

func TestConvertChecksRoundedIntegerRange(t *testing.T) {
	for _, tt := range []struct {
		name                   string
		amount                 int64
		sourceRate, targetRate float64
		want                   int64
		overflow               bool
	}{
		{"positive overflow", math.MaxInt64, 2, 1, 0, true},
		{"negative overflow", math.MinInt64, 2, 1, 0, true},
		{"positive infinity intermediate", 100, math.MaxFloat64, 1, 0, true},
		{"tiny target rate", 100, 1, math.SmallestNonzeroFloat64, 0, true},
		{"max at parity", math.MaxInt64, 1, 1, math.MaxInt64, false},
		{"min at parity", math.MinInt64, 1, 1, math.MinInt64, false},
		{"max half rounds away", math.MaxInt64, 0.5, 1, 1 << 62, false},
		{"negative boundary overflow", math.MinInt64 + 1, math.Nextafter(1, 2), 1, 0, true},
		{"finite final despite intermediate infinity", 1_000_000_000_000_000_000, 1e308, 5e307, 2_000_000_000_000_000_000, false},
		{"finite final despite intermediate underflow", 1, math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64, 1, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := Rates{Base: "USD", Rates: map[string]float64{"EUR": tt.sourceRate, "GBP": tt.targetRate}}
			got, err := r.Convert(money.New(tt.amount, "EUR"), "GBP")
			if tt.overflow {
				if !errors.Is(err, money.ErrOverflow) || got != (money.Money{}) {
					t.Fatalf("got %v, %v; want zero result and wrapped overflow", got, err)
				}
				return
			}
			if err != nil || got != money.New(tt.want, "GBP") {
				t.Fatalf("got %v, %v; want %d GBP", got, err, tt.want)
			}
		})
	}
}

func TestConvertSameCurrencyDoesNotNeedValidRates(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"EUR": math.NaN()}}
	for _, amount := range []int64{math.MinInt64, 0, math.MaxInt64} {
		m := money.New(amount, "EUR")
		if got, err := r.Convert(m, "EUR"); err != nil || got != m {
			t.Fatalf("same-currency conversion = %v, %v; want %v", got, err, m)
		}
	}
}
