// SPDX-License-Identifier: MIT

package budgetview

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func incomeSourceInput() IncomeSourceInput {
	return IncomeSourceInput{MonthStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Rates: currency.Rates{Base: "USD"}, Months: 3,
		Categories: []domain.Category{{ID: "a", Name: "Alpha", Kind: domain.KindIncome}, {ID: "b", Name: "Beta", Kind: domain.KindIncome}, {ID: "empty", Name: "Zero", Kind: domain.KindIncome}, {ID: "expense", Name: "Expense", Kind: domain.KindExpense}}, UncategorizedName: "Uncategorized"}
}

func incomeSourceTransaction(id, category string, minor int64, at time.Time) domain.Transaction {
	return domain.Transaction{ID: id, CategoryID: category, Date: at, Amount: money.New(minor, "USD")}
}

func TestIncomeSourcesPreserveAverageRoundingAndLastMonthRanking(t *testing.T) {
	in := incomeSourceInput()
	in.Transactions = []domain.Transaction{
		incomeSourceTransaction("a-last", "a", 100, in.MonthStart.AddDate(0, -1, 9)),
		incomeSourceTransaction("a-old", "a", 400, in.MonthStart.AddDate(0, -3, 9)),
		incomeSourceTransaction("b-last", "b", 200, in.MonthStart.AddDate(0, -1, 9)),
		incomeSourceTransaction("uncat", "", 10, in.MonthStart.AddDate(0, -1, 9)),
	}
	got := PrepareIncomeSources(in)
	want := []IncomeSource{{"b", "Beta", 66, true}, {"a", "Alpha", 166, true}, {"", "Uncategorized", 3, true}, {"empty", "Zero", 0, true}}
	if got.AmountErr != nil || got.RankErr != nil || !reflect.DeepEqual(got.Rows, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	for _, months := range []int{-3, 0, 1} {
		in.Months = months
		one := PrepareIncomeSources(in)
		if one.Rows[0].Minor != 200 || one.Rows[1].Minor != 100 || one.Rows[2].Minor != 10 {
			t.Fatalf("months%d must normalize to one: %+v", months, one)
		}
	}
	in.Months = 3
	if again := PrepareIncomeSources(in); !reflect.DeepEqual(again.Rows, got.Rows) {
		t.Fatal("averaging changes or repeated calls changed stable ordering")
	}
}

func TestIncomeSourcesUseCanonicalSplitsFiltersBoundariesAndFXWithoutMutation(t *testing.T) {
	in := incomeSourceInput()
	in.Months = 1
	in.Rates.Rates = map[string]float64{"EUR": 1.5}
	start := in.MonthStart.AddDate(0, -1, 0)
	row := incomeSourceTransaction("split", "a", 100, start)
	row.Amount.Currency = "EUR"
	row.Splits = []domain.CategorySplit{{CategoryID: "a", Amount: money.New(40, "EUR")}, {CategoryID: "b", Amount: money.New(30, "EUR")}}
	excluded := incomeSourceTransaction("excluded", "a", 10000, start)
	excluded.ExcludeFromReports = true
	transfer := incomeSourceTransaction("transfer", "a", 10000, start)
	transfer.TransferAccountID = "other"
	in.Transactions = []domain.Transaction{row, excluded, transfer, incomeSourceTransaction("expense", "a", -500, start), incomeSourceTransaction("end-exclusive", "a", 10000, in.MonthStart), incomeSourceTransaction("before-start", "a", 10000, start.Add(-time.Nanosecond))}
	before, _ := json.Marshal(in)
	got := PrepareIncomeSources(in)
	if got.AmountErr != nil || got.RankErr != nil || got.Rows[0].CategoryID != "a" || got.Rows[0].Minor != 105 || got.Rows[1].CategoryID != "b" || got.Rows[1].Minor != 45 {
		t.Fatalf("canonical report attribution changed: %+v", got)
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("source preparation mutated transactions/splits/categories/rates")
	}
}

func TestIncomeSourcesRetainIndependentUnknownAndRankingFacts(t *testing.T) {
	for _, lastMonthFailure := range []bool{false, true} {
		in := incomeSourceInput()
		at := in.MonthStart.AddDate(0, -2, 9)
		if lastMonthFailure {
			at = in.MonthStart.AddDate(0, -1, 9)
		}
		foreign := incomeSourceTransaction("foreign", "a", 10, at)
		foreign.Amount.Currency = "EUR"
		in.Transactions = []domain.Transaction{foreign, incomeSourceTransaction("known-last-month", "b", 5, in.MonthStart.AddDate(0, -1, 9))}
		got := PrepareIncomeSources(in)
		if !errors.Is(got.AmountErr, currency.ErrUnknownRate) || (got.RankErr != nil) != lastMonthFailure {
			t.Fatalf("lost independent error availability: %+v", got)
		}
		for _, source := range got.Rows {
			if source.Known || source.Minor != 0 {
				t.Fatalf("unknown income became known/partial zero: %+v", source)
			}
		}
		if !lastMonthFailure && got.Rows[0].CategoryID != "b" {
			t.Fatal("known last-month ranking was discarded", got.Rows)
		}
		if lastMonthFailure && got.Rows[0].Name != "Alpha" {
			t.Fatal("failed ranking must retain deterministic name fallback", got.Rows)
		}
	}
	in := incomeSourceInput()
	in.Transactions = []domain.Transaction{incomeSourceTransaction("large", "a", math.MaxInt64, in.MonthStart.AddDate(0, -1, 9)), incomeSourceTransaction("one-more", "a", 1, in.MonthStart.AddDate(0, -2, 9))}
	got := PrepareIncomeSources(in)
	if !errors.Is(got.AmountErr, money.ErrOverflow) || got.RankErr != nil || got.Rows[0].Known {
		t.Fatalf("sum overflow became history or contaminated valid rank: %+v", got)
	}
}

func TestIncomeSourcesKeepSeparateTruncationFromCanonicalCombinedAverage(t *testing.T) {
	in := incomeSourceInput()
	in.Categories = []domain.Category{{ID: "a", Name: "A", Kind: domain.KindIncome}, {ID: "b", Name: "B", Kind: domain.KindIncome}, {ID: "c", Name: "C", Kind: domain.KindIncome}}
	for _, id := range []string{"a", "b", "c"} {
		in.Transactions = append(in.Transactions, incomeSourceTransaction(id, id, 1, in.MonthStart.AddDate(0, -1, 9)))
	}
	got := PrepareIncomeSources(in)
	if got.AmountErr != nil || got.RankErr != nil || len(got.Rows) != 3 {
		t.Fatal(got)
	}
	for _, row := range got.Rows {
		if row.Minor != 0 || !row.Known {
			t.Fatal("source truncation policy changed", row)
		}
	}
	total, err := budgeting.AveragedIncomeChecked(budgeting.IncomeModeAll, 0, 0, budgeting.IncomeSources{}, in.Transactions, in.MonthStart, 3, "USD", in.Rates)
	if err != nil || total != 1 {
		t.Fatalf("combined canonical average changed to force row reconciliation: %d %v", total, err)
	}
}

func TestIncomeSourcesNameTiesAndZeroHistoryStayStable(t *testing.T) {
	in := incomeSourceInput()
	in.Categories = []domain.Category{{ID: "two", Name: "Equal", Kind: domain.KindIncome}, {ID: "one", Name: "Equal", Kind: domain.KindIncome}, {ID: "first", Name: "Alpha", Kind: domain.KindIncome}}
	got := PrepareIncomeSources(in)
	if got.AmountErr != nil || got.RankErr != nil || len(got.Rows) != 3 || got.Rows[0].CategoryID != "first" || got.Rows[1].CategoryID != "two" || got.Rows[2].CategoryID != "one" {
		t.Fatal("name ordering/stable ties changed", got)
	}
	for _, row := range got.Rows {
		if !row.Known || row.Minor != 0 {
			t.Fatal("empty history must remain known zero", row)
		}
	}
}
