// SPDX-License-Identifier: MIT

package taxgather

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reports"
)

// netIncomeGatherReference retains the previously committed composition only
// as a parity oracle. It is not part of the production path or benchmark.
func netIncomeGatherReference(in NetIncomeInput) (NetIncome, error) {
	if err := validateRefunds(in.Transactions, in.Links, in.From, in.To); err != nil {
		return NetIncome{}, err
	}
	all := budgeting.NewNettedLedger(in.Transactions, in.Links)
	flows, err := reports.IncomeExpenseSeriesFromLedger(all, []time.Time{in.From, in.To}, in.Rates)
	if err != nil {
		return NetIncome{}, err
	}
	collection, err := gatherLedger(all, in.Categories, in.Year, in.From, in.To, in.Rates)
	if err != nil {
		return NetIncome{}, err
	}
	net, err := money.Sub(flows[0].Income, collection.Deductible.Total)
	if err != nil {
		return NetIncome{}, err
	}
	return NetIncome{Year: in.Year, From: in.From, To: in.To, Currency: in.Rates.Base, IncomeMinor: flows[0].Income, DeductibleMinor: collection.Deductible.Total, NetMinor: net}, nil
}

func TestNetIncomeSelectedFoldMatchesGatheringAndErrorIdentities(t *testing.T) {
	in := netIncomeFixture()
	in.Rates.Rates = map[string]float64{"EUR": 2}
	for i := 0; i < 300; i++ {
		at := in.From.Add(time.Duration(i) * time.Hour)
		row := netIncomeRow(string(rune(1000+i)), "other", "USD", at, -1000)
		switch i % 4 {
		case 0:
			row.Amount = money.New(5000, "USD")
		case 1:
			row.CategoryID = "ded"
			row.Amount = money.New(-500, "EUR")
		case 2:
			row.Splits = []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-600, "USD")}, {CategoryID: "other", Amount: money.New(-500, "USD")}, {CategoryID: "ded", Amount: money.New(100, "USD")}}
		case 3:
			row.ExcludeFromReports = true
			row.Amount = money.New(math.MinInt64, "NOFX")
		}
		in.Transactions = append(in.Transactions, row)
	}
	got, err := NetIncomeWithLinks(in)
	want, wantErr := netIncomeGatherReference(in)
	if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("whole-period parity: %+v/%v vs %+v/%v", got, err, want, wantErr)
	}
	for _, tc := range []struct {
		name string
		rows []domain.Transaction
		want error
	}{
		{"missing split FX", []domain.Transaction{{Date: in.From, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "other", Amount: money.New(-1, "NOFX")}}}}, currency.ErrUnknownRate},
		{"unused same-record overflow", []domain.Transaction{{Date: in.From, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "other", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64, "USD")}, {CategoryID: "other", Amount: money.New(-1, "USD")}}}}, money.ErrOverflow},
		{"selected total overflow", []domain.Transaction{{Date: in.From, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "ded", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64-1, "USD")}}}, {Date: in.From, CategoryID: "ded", Amount: money.New(-1, "USD")}}, money.ErrOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			local := in
			local.Transactions = tc.rows
			got, err := NetIncomeWithLinks(local)
			old, oldErr := netIncomeGatherReference(local)
			if !errors.Is(err, tc.want) || !errors.Is(oldErr, tc.want) || !reflect.DeepEqual(got, NetIncome{}) || !reflect.DeepEqual(old, NetIncome{}) {
				t.Fatalf("error identity/partial facts changed: %+v/%v vs %+v/%v", got, err, old, oldErr)
			}
		})
	}
}

func TestNetIncomeSelectedFoldPreservesUnusedCategoryAvailability(t *testing.T) {
	in := netIncomeFixture()
	for _, id := range []string{"a", "b"} {
		in.Transactions = append(in.Transactions, domain.Transaction{ID: id, Date: in.From, Amount: money.New(-1, "USD"), Splits: []domain.CategorySplit{{CategoryID: "other", Amount: money.New(-math.MaxInt64, "USD")}, {CategoryID: "credit", Amount: money.New(math.MaxInt64-1, "USD")}}})
	}
	in.Transactions = append(in.Transactions, netIncomeRow("income", "", "USD", in.From, 1000), netIncomeRow("ded", "ded", "USD", in.From, -100))
	got, err := NetIncomeWithLinks(in)
	want, wantErr := netIncomeGatherReference(in)
	if err != nil || wantErr != nil || got.NetMinor != 900 || !reflect.DeepEqual(got, want) {
		t.Fatalf("unused category aggregation changed availability: %+v/%v vs %+v/%v", got, err, want, wantErr)
	}
}
