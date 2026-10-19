// SPDX-License-Identifier: MIT

package txnclassify_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnview"
)

func TestTV8GBackfillAndReportingParity(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}}
	for _, tt := range []struct {
		name      string
		class     domain.AccountClass
		sign      domain.LiabilitySign
		in        money.Money
		ambiguous bool
	}{
		{name: "mortgage payment positive owed", class: domain.ClassLiability, sign: domain.LiabilityPositiveOwed, in: money.New(-12500, "USD")},
		{name: "mortgage payment negative owed", class: domain.ClassLiability, sign: domain.LiabilityNegativeOwed, in: money.New(12500, "USD")},
		{name: "owners draw equity", class: domain.ClassAsset, in: money.New(12500, "USD")},
		{name: "legacy savings", class: domain.ClassAsset, in: money.New(12500, "USD")},
		{name: "ambiguous same day", class: domain.ClassAsset, in: money.New(12500, "USD"), ambiguous: true},
		{name: "cross currency", class: domain.ClassAsset, in: money.New(10000, "EUR")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			accounts := []domain.Account{{ID: "cash", Class: domain.ClassAsset}, {ID: "destination", Class: tt.class, LiabilitySign: tt.sign}}
			rows := []domain.Transaction{
				{ID: "out", AccountID: "cash", TransferAccountID: "destination", Date: day, Amount: money.New(-12500, "USD")},
				{ID: "in", AccountID: "destination", TransferAccountID: "cash", Date: day.AddDate(0, 0, 1), Amount: tt.in},
			}
			if tt.ambiguous {
				rows[1].Date = day
				duplicate := rows[1]
				duplicate.ID = "another-in"
				rows = append(rows, duplicate)
			}
			before := append([]domain.Transaction(nil), rows...)
			migrated := txnclassify.BackfillLegacyTransferGroups(rows, accounts, rates)
			if !reflect.DeepEqual(rows, before) {
				t.Fatal("migration mutated input")
			}
			if again := txnclassify.BackfillLegacyTransferGroups(migrated, accounts, rates); !reflect.DeepEqual(again, migrated) {
				t.Fatal("migration is not idempotent")
			}
			reversed := append([]domain.Transaction(nil), rows...)
			for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
				reversed[i], reversed[j] = reversed[j], reversed[i]
			}
			reversed = txnclassify.BackfillLegacyTransferGroups(reversed, accounts, rates)
			for i := range migrated {
				if migrated[i].TransferGroupID != reversed[len(reversed)-1-i].TransferGroupID {
					t.Fatal("group identity depends on input order")
				}
			}
			wantCount, wantUnpaired, wantAmount := 1, 0, int64(12500)
			if tt.ambiguous {
				wantCount, wantUnpaired, wantAmount = 3, 3, 37500
				for _, row := range migrated {
					if row.TransferGroupID != "" {
						t.Fatal("ambiguous row acquired a group")
					}
				}
			} else if migrated[0].TransferGroupID == "" || migrated[0].TransferGroupID != migrated[1].TransferGroupID {
				t.Fatal("unique pair was not backfilled")
			}
			shared := txnclassify.SummarizeVisibleTransfersWithAccounts(rows, accounts, rates)
			view, err := txnview.Summarize(migrated, rates, rates.Base)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(shared.CountedRows, view.CountedTransfers) || view.MovedRows != wantCount || view.Transferred != wantAmount || view.UnpairedRows != wantUnpaired {
				t.Fatalf("summary parity: before=%+v after=%+v", shared, view)
			}
			calendar, err := txncalendar.MonthCashflow(day, time.Monday, migrated, nil, rates)
			if err != nil {
				t.Fatal(err)
			}
			var moved int64
			for _, week := range calendar.Weeks {
				moved += week.Total.Moved
			}
			if moved != view.Transferred {
				t.Fatalf("calendar moved=%d, summary=%d", moved, view.Transferred)
			}
		})
	}
}

func TestTV8GBackfillLeavesUnprovenRowsUntouched(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	base := []domain.Transaction{
		{ID: "out", AccountID: "a", TransferAccountID: "b", Date: day, Amount: money.New(-100, "USD")},
		{ID: "in", AccountID: "b", TransferAccountID: "a", Date: day, Amount: money.New(100, "USD")},
	}
	for _, name := range []string{"different amount", "outside window", "other target", "existing group", "zero", "missing rate", "duplicate identity", "missing identity"} {
		t.Run(name, func(t *testing.T) {
			rows := append([]domain.Transaction(nil), base...)
			switch name {
			case "different amount":
				rows[1].Amount.Amount++
			case "outside window":
				rows[1].Date = day.Add(txnclassify.PairWindow + time.Second)
			case "other target":
				rows[1].TransferAccountID = "elsewhere"
			case "existing group":
				rows[0].TransferGroupID = "already-linked"
			case "zero":
				rows[0].Amount.Amount = 0
			case "missing rate":
				rows[1].Amount.Currency = "EUR"
			case "duplicate identity":
				rows[1].ID = rows[0].ID
			case "missing identity":
				rows[1].ID = ""
			}
			if got := txnclassify.BackfillLegacyTransferGroups(rows, nil, currency.Rates{Base: "USD"}); !reflect.DeepEqual(got, rows) {
				t.Fatalf("unproven rows changed: %+v", got)
			}
		})
	}
}

func TestTV8GTransferOutgoingUsesFixedConvention(t *testing.T) {
	for _, tt := range []struct {
		class domain.AccountClass
		sign  domain.LiabilitySign
		minor int64
		want  bool
	}{
		{domain.ClassAsset, "", -100, true}, {domain.ClassAsset, "", 100, false},
		{domain.ClassLiability, domain.LiabilityPositiveOwed, -100, false},
		{domain.ClassLiability, domain.LiabilityPositiveOwed, 100, true},
		{domain.ClassLiability, domain.LiabilityNegativeOwed, -100, true},
		{domain.ClassLiability, domain.LiabilityNegativeOwed, 100, false},
		{domain.ClassLiability, domain.LiabilityPositiveOwed, 0, false},
	} {
		row := domain.Transaction{Amount: money.New(tt.minor, "USD")}
		if got := txnclassify.TransferOutgoing(row, domain.Account{Class: tt.class, LiabilitySign: tt.sign}); got != tt.want {
			t.Errorf("class=%s convention=%s amount=%d outgoing=%v, want %v", tt.class, tt.sign, tt.minor, got, tt.want)
		}
	}
}
