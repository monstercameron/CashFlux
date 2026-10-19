// SPDX-License-Identifier: MIT

package splitdraft_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
)

func FuzzBalancedConversion(f *testing.F) {
	f.Add(uint64(100), uint64(33))
	f.Add(uint64(math.MaxInt64), uint64(1))
	f.Add(uint64(3), uint64(1))
	f.Fuzz(func(t *testing.T, rawTotal, rawPart uint64) {
		total := int64(rawTotal%uint64(math.MaxInt64-1)) + 2
		part := int64(rawPart%uint64(total-1)) + 1
		tx := transaction(-total, "JPY")
		s := splitdraft.State{Lines: []splitdraft.Line{{Cat: "a", Amt: money.FormatMinor(part, 0)}, {Cat: "b", Amt: money.FormatMinor(total-part, 0)}}}
		pct := s.Apply(splitdraft.Action{Kind: splitdraft.ToPercents}, tx)
		if !pct.Percent || pct.View(tx).Remainder != 0 {
			t.Fatalf("conversion lost total: %+v", pct)
		}
		back := pct.Apply(splitdraft.Action{Kind: splitdraft.ToAmounts}, tx)
		// An extremely small share can round to zero basis points. The existing
		// converter leaves zero entries untouched; all positive represented shares
		// must still carry the entire parent after conversion back.
		if back.Percent || back.View(tx).Remainder != 0 {
			t.Fatalf("reverse conversion lost total: %+v", back)
		}
		_, saved, ok := s.AttemptSave(tx, false)
		if !ok {
			t.Fatal("balanced source refused")
		}
		reseeded := splitdraft.New(saved, 0)
		if !reflect.DeepEqual(reseeded.Lines, s.Lines) {
			t.Fatalf("save/seed round trip = %+v, want %+v", reseeded.Lines, s.Lines)
		}
	})
}

func FuzzRawDraftNeverMutates(f *testing.F) {
	f.Add("1.00", "bad", int64(-100))
	f.Add("92233720368547758.07", "0.01", int64(math.MinInt64))
	f.Fuzz(func(t *testing.T, a, b string, total int64) {
		s := splitdraft.State{Lines: []splitdraft.Line{{Cat: "a", Amt: a}, {Cat: "b", Amt: b}}}
		before := append([]splitdraft.Line(nil), s.Lines...)
		tx := transaction(total, "USD")
		_ = s.View(tx)
		converted := s.Apply(splitdraft.Action{Kind: splitdraft.ToPercents}, tx)
		_ = converted.View(tx)
		_, saved, ok := s.AttemptSave(tx, false)
		if !reflect.DeepEqual(before, s.Lines) {
			t.Fatal("draft input mutated")
		}
		if ok {
			values := make([]money.Money, len(saved.Splits))
			for i, line := range saved.Splits {
				values[i] = line.Amount
			}
			sum, err := money.Sum(values...)
			if err != nil || !sum.Equal(tx.Amount) {
				t.Fatalf("accepted split does not reconcile: %v, %v", sum, err)
			}
		}
	})
}
