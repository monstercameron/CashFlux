// SPDX-License-Identifier: MIT
package reports

import (
	"errors"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"math"
	"reflect"
	"testing"
)

func TestPayeeReducersCheckedMagnitudeAndGroupOverflow(t *testing.T) {
	start, end := dt(2026, 6, 1), dt(2026, 7, 1)
	for _, tc := range []struct {
		name  string
		rows  []domain.Transaction
		rates currency.Rates
		want  error
	}{
		{"minimum", []domain.Transaction{statsCheckedRow("min", math.MinInt64, "USD", start)}, usdRates(), money.ErrOverflow},
		{"sum", []domain.Transaction{statsCheckedRow("max", -math.MaxInt64, "USD", start), statsCheckedRow("one", -1, "USD", start)}, usdRates(), money.ErrOverflow},
		{"FX", []domain.Transaction{statsCheckedRow("fx", -1, "EUR", start)}, usdRates(), currency.ErrUnknownRate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := TopPayees(tc.rows, start, end, tc.rates, 1)
			if rows != nil || !errors.Is(err, tc.want) {
				t.Fatalf("period rows%v err%v", rows, err)
			}
			trailing, err := TopPayeesTrailing(tc.rows, 31, end, tc.rates, 1)
			if trailing != nil || !errors.Is(err, tc.want) {
				t.Fatalf("trailing rows%v err%v", trailing, err)
			}
		})
	}
}
func TestPayeeReducersCheckedValidMaximumAndConvertedMinimum(t *testing.T) {
	start, end := dt(2026, 6, 1), dt(2026, 7, 1)
	for _, tc := range []struct {
		amount int64
		code   string
		rates  currency.Rates
		want   int64
	}{{-math.MaxInt64, "USD", usdRates(), math.MaxInt64}, {math.MinInt64, "EUR", currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 0.5}}, 1 << 62}} {
		txns := []domain.Transaction{statsCheckedRow("x", tc.amount, tc.code, start)}
		got, err := TopPayees(txns, start, end, tc.rates, 1)
		if err != nil || !reflect.DeepEqual(got, []PayeeTotal{{Name: "Shop", Amount: tc.want}}) {
			t.Fatalf("period %+v %v", got, err)
		}
		trailing, err := TopPayeesTrailing(txns, 31, end, tc.rates, 1)
		if err != nil || !reflect.DeepEqual(trailing, []PayeeSummary{{Name: "Shop", Amount: tc.want, Count: 1}}) {
			t.Fatalf("trailing %+v %v", trailing, err)
		}
	}
}
