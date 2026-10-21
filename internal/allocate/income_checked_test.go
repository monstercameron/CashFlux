// SPDX-License-Identifier: MIT

package allocate

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPeriodIncomeCheckedReductionAndConverterShortCircuit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amounts []int64
	}{
		{"positive overflow", []int64{math.MaxInt64, 1, 7}},
		{"negative underflow", []int64{math.MinInt64, -1, 7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			txns := make([]Transaction, len(tc.amounts))
			for i, amount := range tc.amounts {
				txns[i] = inc(amount, "USD", "2026-06-10")
			}
			before := append([]Transaction(nil), txns...)
			calls := []int64{}
			convert := func(amount int64, _, _ string) (int64, error) { calls = append(calls, amount); return amount, nil }
			got, err := PeriodIncome(txns, d("2026-06-01"), d("2026-07-01"), "USD", convert)
			if !errors.Is(err, money.ErrOverflow) || got != 0 || !reflect.DeepEqual(calls, tc.amounts[:2]) {
				t.Fatalf("overflow/short-circuit mismatch: %d,%v calls%v", got, err, calls)
			}
			if !reflect.DeepEqual(txns, before) {
				t.Fatal("income calculation mutated transactions")
			}
		})
	}
	// A single negative income retains the existing final clamp, rather than
	// treating a representable negative sum as an arithmetic error.
	got, err := PeriodIncome([]Transaction{inc(math.MinInt64, "USD", "2026-06-10")}, d("2026-06-01"), d("2026-07-01"), "USD", NoConvert)
	if err != nil || got != 0 {
		t.Fatalf("legacy negative-income clamp changed: %d,%v", got, err)
	}
}

func TestPeriodIncomeCheckedSelectionAndConverterError(t *testing.T) {
	boom := errors.New("conversion unavailable")
	txns := []Transaction{
		inc(math.MaxInt64, "USD", "2026-05-31"),
		inc(math.MaxInt64, "USD", "2026-07-01"),
		{Amount: math.MinInt64, Currency: "USD", Date: d("2026-06-10")},
		inc(123, "USD", "2026-06-01"),
		inc(456, "XYZ", "2026-06-30"),
		inc(789, "USD", "2026-06-30"),
	}
	var calls []int64
	convert := func(amount int64, from, _ string) (int64, error) {
		calls = append(calls, amount)
		if from == "XYZ" {
			return 0, boom
		}
		return amount, nil
	}
	got, err := PeriodIncome(txns, d("2026-06-01"), d("2026-07-01"), "USD", convert)
	if !errors.Is(err, boom) || got != 0 || !reflect.DeepEqual(calls, []int64{123, 456}) {
		t.Fatalf("selection/converter error changed: %d,%v calls%v", got, err, calls)
	}
}

func TestSplitPeriodIncomeCheckedHugeAndOverflow(t *testing.T) {
	start, end := d("2026-06-01"), d("2026-07-01")
	members := []domain.Member{{ID: "bob", Name: "Bob", Custom: map[string]any{"x": "keep"}}, {ID: domain.GroupOwnerID, Name: "Shared"}, {ID: "alice", Name: "Alice"}}
	txns := []domain.Transaction{
		incomeTxn(math.MaxInt64, "USD", "2026-06-01"),
		{ID: "expense", Date: start, Amount: money.New(math.MinInt64, "USD")},
		{ID: "transfer", Date: start, Amount: money.New(math.MaxInt64, "USD"), TransferAccountID: "other"},
		incomeTxn(math.MaxInt64, "USD", "2026-07-01"),
	}
	before, _ := json.Marshal(struct {
		Members []domain.Member
		Txns    []domain.Transaction
	}{members, txns})
	got, err := SplitPeriodIncome(txns, members, start, end, "USD", currency.Rates{Base: "USD"})
	want := []MemberIncomeSplit{{MemberID: "alice", Name: "Alice", Amount: math.MaxInt64/2 + 1}, {MemberID: "bob", Name: "Bob", Amount: math.MaxInt64 / 2}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("huge equal split/tie order changed: %v,%v", got, err)
	}
	after, _ := json.Marshal(struct {
		Members []domain.Member
		Txns    []domain.Transaction
	}{members, txns})
	if string(before) != string(after) {
		t.Fatal("split mutated member order/custom data or transactions")
	}
	txns = append(txns, incomeTxn(1, "USD", "2026-06-02"))
	got, err = SplitPeriodIncome(txns, members, start, end, "USD", currency.Rates{Base: "USD"})
	if !errors.Is(err, money.ErrOverflow) || got != nil {
		t.Fatalf("income overflow became successful split/zero: %v,%v", got, err)
	}
	// No recipients intentionally avoids an income calculation, even if its
	// currency would otherwise be unavailable; this legacy contract is retained.
	got, err = SplitPeriodIncome([]domain.Transaction{incomeTxn(1, "XYZ", "2026-06-01")}, nil, start, end, "USD", currency.Rates{Base: "USD"})
	if err != nil || got != nil {
		t.Fatalf("no-recipient contract changed: %v,%v", got, err)
	}
}
