// SPDX-License-Identifier: MIT

package reports

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSpendingByMemberCheckedMagnitudeAndReductions(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	for _, tc := range []struct {
		name    string
		amounts []int64
		owners  []string
	}{
		{"minimum expense magnitude", []int64{math.MinInt64}, []string{"alice"}},
		{"unattributed minimum magnitude", []int64{math.MinInt64}, nil},
		{"member overflow", []int64{-math.MaxInt64, -1}, []string{"alice"}},
		{"unattributed overflow", []int64{-math.MaxInt64, -1}, nil},
		{"shared member overflow", []int64{-math.MaxInt64, -math.MaxInt64}, []string{"alice", "bob"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			txns := make([]domain.Transaction, len(tc.amounts))
			for i, amount := range tc.amounts {
				txns[i] = domain.Transaction{ID: tc.name, Date: start, Amount: money.New(amount, "USD"), MemberIDs: tc.owners}
			}
			before, _ := json.Marshal(txns)
			got, err := SpendingByMember(txns, start, end, usdRates())
			if !errors.Is(err, money.ErrOverflow) || got != nil {
				t.Fatalf("got %v, %v; want no partial rows and overflow", got, err)
			}
			after, _ := json.Marshal(txns)
			if string(before) != string(after) {
				t.Fatal("failed spending preparation mutated transactions/owner slices")
			}
		})
	}
}

func TestSpendingByMemberLargeIndependentMembersAndSelection(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{
		{ID: "alice", Date: start, Amount: money.New(-math.MaxInt64, "USD"), MemberID: "alice"},
		{ID: "bob", Date: start, Amount: money.New(-math.MaxInt64, "USD"), MemberID: "bob"},
		{ID: "before", Date: start.Add(-time.Nanosecond), Amount: money.New(math.MinInt64, "USD")},
		{ID: "end", Date: end, Amount: money.New(math.MinInt64, "USD")},
		{ID: "transfer", Date: start, Amount: money.New(math.MinInt64, "USD"), TransferAccountID: "other"},
		{ID: "excluded", Date: start, Amount: money.New(math.MinInt64, "USD"), ExcludeFromReports: true},
		{ID: "income", Date: start, Amount: money.New(math.MaxInt64, "USD")},
	}
	got, err := SpendingByMember(txns, start, end, usdRates())
	want := []MemberSpend{{MemberID: "alice", Amount: math.MaxInt64}, {MemberID: "bob", Amount: math.MaxInt64}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("independent representable rows or selection changed: %v, %v", got, err)
	}
}

func TestSpendingByMemberRetainsFXErrorRoundingAndInputs(t *testing.T) {
	start, end := dt(2026, time.June, 1), dt(2026, time.July, 1)
	txns := []domain.Transaction{{ID: "foreign", Date: start, Amount: money.New(-1001, "EUR"), MemberIDs: []string{"bob", "alice"}}}
	before, _ := json.Marshal(txns)
	got, err := SpendingByMember(txns, start, end, usdRates())
	if !errors.Is(err, currency.ErrUnknownRate) || got != nil {
		t.Fatalf("missing rate became a partial/zero result: %v,%v", got, err)
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.5}}
	got, err = SpendingByMember(txns, start, end, rates)
	// Convert -1001 EUR minor once to -1502 USD minor, then split equally.
	if err != nil || !reflect.DeepEqual(got, []MemberSpend{{MemberID: "alice", Amount: 751}, {MemberID: "bob", Amount: 751}}) {
		t.Fatalf("conversion/split order changed: %v,%v", got, err)
	}
	after, _ := json.Marshal(txns)
	if string(before) != string(after) || rates.Rates["EUR"] != 1.5 {
		t.Fatal("spending calculation mutated source data")
	}
	empty, err := SpendingByMember(nil, start, end, usdRates())
	if err != nil || len(empty) != 0 {
		t.Fatalf("known empty spending is unavailable: %v,%v", empty, err)
	}
}
