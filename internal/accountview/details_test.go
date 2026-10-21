// SPDX-License-Identifier: MIT

package accountview_test

import (
	"fmt"
	"io"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountflow"
	"github.com/monstercameron/CashFlux/internal/accountview"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparedAccountHistoryMatchesEveryLegacyWindow(t *testing.T) {
	zone := time.FixedZone("west", -5*60*60)
	now := time.Date(2026, 10, 20, 15, 30, 0, 0, zone)
	account := domain.Account{ID: "checking", Currency: "USD", OpeningBalance: money.New(12000, "USD")}
	rows := []domain.Transaction{
		{ID: "future", AccountID: account.ID, Date: now.AddDate(0, 0, 1), Amount: money.New(5000, "USD")},
		{ID: "last", AccountID: account.ID, Date: time.Date(2026, 10, 20, 23, 59, 0, 0, time.UTC), Amount: money.New(-50, "USD")},
		{ID: "foreign", AccountID: account.ID, Date: now.AddDate(0, -3, 0), Amount: money.New(800, "EUR")},
		{ID: "other", AccountID: "other", Date: now, Amount: money.New(900000, "USD")},
		{ID: "old", AccountID: account.ID, Date: now.AddDate(-24, 0, 0), Amount: money.New(-500, "USD")},
		{ID: "excluded", AccountID: account.ID, Date: now.AddDate(0, 0, -1), Amount: money.New(100, "USD"), ExcludeFromReports: true},
	}
	for i := 0; i < 200; i++ {
		rows = append(rows, domain.Transaction{ID: fmt.Sprintf("row-%d", i), AccountID: account.ID,
			Date: now.AddDate(0, 0, -i*7), Amount: money.New(int64(i%17-8), "USD")})
	}
	before := append([]domain.Transaction(nil), rows...)
	history := accountview.PrepareHistory(account, rows)
	for _, days := range []int{-1, 0, 1, 90, 365, 366 * 20} {
		for _, asOf := range []time.Time{now, now.AddDate(0, 0, 1), now.AddDate(0, -1, 0)} {
			if got, want := history.Series(asOf, days), accountflow.BalanceSeries(account, rows, asOf, days); !reflect.DeepEqual(got, want) {
				t.Fatalf("prepared history differs for %s/%d days", asOf, days)
			}
		}
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("preparing history reordered or changed the shared transaction index")
	}
	// The preparation owns its scalar history. Neither source edits nor a caller's
	// editable returned chart window may mutate subsequent windows.
	want := history.Series(now, 90)
	rows[1].Amount.Amount++
	got := history.Series(now, 90)
	got[0]++
	if !reflect.DeepEqual(history.Series(now, 90), want) {
		t.Fatal("prepared history shares mutable source or output data")
	}
	account.OpeningBalance.Amount = math.MaxInt64
	wrapping := []domain.Transaction{{AccountID: account.ID, Date: now, Amount: money.New(10, "USD")}}
	if got, want := accountview.PrepareHistory(account, wrapping).Series(now, 1), accountflow.BalanceSeries(account, wrapping, now, 1); !reflect.DeepEqual(got, want) {
		t.Fatal("history changed existing overflow behavior")
	}
}

func TestIndexedAccountProjectionMatchesAppWithExactOccurrenceTimes(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	accounts := []domain.Account{
		{ID: "checking", Name: "Checking", Currency: "USD", OpeningBalance: money.New(5000, "USD")},
		{ID: "other", Name: "Other", Currency: "EUR", OpeningBalance: money.New(1000, "EUR")},
		{ID: "invalid", Name: "Invalid", Currency: "USD", OpeningBalance: money.New(1000, "EUR")},
	}
	for _, account := range accounts {
		if err := app.Store().PutAccount(account); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []domain.Transaction{
		{ID: "one", AccountID: "checking", Amount: money.New(-100, "USD"), Date: now, Cleared: true},
		{ID: "excluded", AccountID: "checking", Amount: money.New(200, "USD"), Date: now, ExcludeFromReports: true},
		{ID: "uncleared", AccountID: "checking", Amount: money.New(-300, "USD"), Date: now},
	} {
		if err := app.Store().PutTransaction(row); err != nil {
			t.Fatal(err)
		}
	}
	recurring := []domain.Recurring{
		{ID: "rent", Label: "Rent", AccountID: "checking", Amount: money.New(-3000, "USD"), NextDue: now.AddDate(0, 0, 1), Cadence: domain.CadenceMonthly},
		{ID: "pay", Label: "Pay", AccountID: "checking", Amount: money.New(1000, "USD"), NextDue: now.AddDate(0, -1, 0), Cadence: domain.CadenceWeekly},
		{ID: "intraday", Label: "Today later", AccountID: "checking", Amount: money.New(-25, "USD"), NextDue: now.Add(time.Hour), Cadence: domain.CadenceDaily},
		{ID: "earlier", Label: "Today earlier", AccountID: "checking", Amount: money.New(-35, "USD"), NextDue: now.Add(-time.Hour), Cadence: domain.CadenceMonthly},
		{ID: "other", Label: "Other", AccountID: "other", Amount: money.New(-400, "EUR"), NextDue: now.AddDate(0, 0, 2), Cadence: domain.CadenceMonthly},
		{ID: "zero", Label: "Zero", AccountID: "checking", Amount: money.Zero("USD"), NextDue: now, Cadence: domain.CadenceMonthly},
		{ID: "invalid", Label: "Invalid", AccountID: "invalid", Amount: money.New(-10, "USD"), NextDue: now, Cadence: domain.CadenceMonthly},
	}
	for _, row := range recurring {
		if err := app.Store().PutRecurring(row); err != nil {
			t.Fatal(err)
		}
	}
	before := append([]domain.Recurring(nil), recurring...)
	snapshot := accountview.Build(accounts, app.Transactions())
	prepared := accountview.PrepareProjectionInputs(recurring)
	for _, asOf := range []time.Time{now, now.Add(2 * time.Hour), now.AddDate(0, 0, 1)} {
		for _, horizon := range []int{-1, 0, 30, 90} {
			for _, account := range accounts {
				got := prepared.Project(account.ID, snapshot.Balances[account.ID], asOf, horizon)
				want := app.ProjectAccount(account.ID, asOf, horizon)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s at %s horizon=%d: got %+v; want %+v", account.ID, asOf, horizon, got, want)
				}
			}
		}
	}
	if !reflect.DeepEqual(before, recurring) {
		t.Fatal("projection changed the supplied recurring index")
	}
}
