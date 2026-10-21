// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/txncalendar"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnview"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func tv8gApp(t *testing.T, rows []domain.Transaction, accounts []domain.Account, rates currency.Rates) *appstate.App {
	t.Helper()
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Store().Load(store.Dataset{SchemaVersion: store.SchemaVersion, Transactions: rows, Accounts: accounts,
		Settings: store.Settings{BaseCurrency: rates.Base, FXRates: rates.Rates}}); err != nil {
		t.Fatal(err)
	}
	previous, previousFilter := appstate.Default, uistate.CurrentTxFilter()
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{From: "2026-10-01", To: "2026-10-31", PageSize: 25})
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.PersistTxFilter(previousFilter)
		app.Store().Close()
	})
	return app
}

func TestTV8GTableSummaryCalendarCountAndSelectAllAgree(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.25}}
	for _, name := range []string{"mortgage", "owners draw", "legacy pair", "ambiguous same day", "cross currency", "multiple accounts"} {
		t.Run(name, func(t *testing.T) {
			accounts := []domain.Account{{ID: "a", Name: "Joint Checking", Class: domain.ClassAsset, Currency: "USD"}, {ID: "b", Name: "Business Checking", Class: domain.ClassAsset, Currency: "USD"}}
			rows := []domain.Transaction{
				{ID: "out", AccountID: "a", TransferAccountID: "b", Date: day, Desc: name, Amount: money.New(-12500, "USD")},
				{ID: "in", AccountID: "b", TransferAccountID: "a", Date: day, Desc: name, Amount: money.New(12500, "USD")},
			}
			switch name {
			case "mortgage":
				accounts[1].Name, accounts[1].Class, accounts[1].LiabilitySign = "Mortgage", domain.ClassLiability, domain.LiabilityPositiveOwed
				rows[1].Amount.Amount = -12500
				// Exercise the migrated data that ingress will supply, with the
				// receiving liability first to catch account-order direction bugs.
				rows = txnclassify.BackfillLegacyTransferGroups(rows, accounts, rates)
				rows[0], rows[1] = rows[1], rows[0]
			case "owners draw":
				accounts[0].Name, accounts[1].Name = "Business Checking", "Joint Checking"
			case "ambiguous same day":
				another := rows[1]
				another.ID = "another-in"
				rows = append(rows, another)
			case "cross currency":
				rows[1].Amount = money.New(10000, "EUR")
			}
			tv8gApp(t, rows, accounts, rates)
			if name == "multiple accounts" {
				filter := uistate.CurrentTxFilter()
				filter.Accounts = "a,b"
				uistate.PersistTxFilter(filter)
			}
			f := render.New(t, render.WithQueuedScheduler())
			defer f.Cleanup()
			defer func() { f.Render(Div()) }()
			f.Render(ui.CreateElement(Transactions))
			want := 1
			if name == "ambiguous same day" {
				want = 3
			}
			visible := txnV5RowTransfers(rows, false, rates)
			view, err := txnview.Summarize(rows, rates, rates.Base)
			if err != nil {
				t.Fatal(err)
			}
			if len(visible) != want || view.MovedRows != want {
				t.Fatalf("folded=%d summary movements=%d want %d", len(visible), view.MovedRows, want)
			}
			for i, row := range rows {
				found := byTestID(f, "tr", "txn-row-"+row.ID) != nil
				if found != view.CountedTransfers[i] {
					t.Fatalf("row %s visible=%v, summary counted=%v", row.ID, found, view.CountedTransfers[i])
				}
				if found && want == 1 {
					label := accounts[0].Name + " → " + accounts[1].Name
					if node := byTestID(f, "tr", "txn-row-"+row.ID); !strings.Contains(node.Text(), label) {
						t.Fatalf("folded row direction does not read %q: %s", label, node.Text())
					}
				}
			}
			count := byTestID(f, "span", "txn-count-transaction-total")
			if count == nil || count.Text() != txnCountTransactionCount(want) {
				t.Fatalf("count line=%v want %d", count, want)
			}
			calendar, err := txncalendar.MonthCashflow(day, time.Monday, visible, nil, rates)
			if err != nil {
				t.Fatal(err)
			}
			var moved int64
			var calendarCount int
			for _, week := range calendar.Weeks {
				moved += week.Total.Moved
				calendarCount += week.Total.Count
			}
			if moved != view.Transferred || calendarCount != want {
				t.Fatalf("calendar moved=%d count=%d, want %d/%d", moved, calendarCount, view.Transferred, want)
			}
			selectAll := byTestID(f, "input", "txn-select-all-visible")
			if selectAll == nil {
				t.Fatal("select-all missing")
			}
			selectAll.Click()
			f.Stabilize()
			for _, row := range visible {
				if node := byTestID(f, "tr", "txn-row-"+row.ID); node == nil || !hasClass(node.Attr("class"), "selected") {
					t.Fatalf("select-all missed visible row %s", row.ID)
				}
			}
			// Clear before the next fixture, while the mounted header owns the atom.
			byTestID(f, "input", "txn-select-all-visible").Click()
			f.Stabilize()
		})
	}
}

func TestTV8GReceivingAccountLabelUsesMoneyDirection(t *testing.T) {
	for _, tt := range []struct {
		name    string
		account domain.Account
		minor   int64
	}{
		{"mortgage positive owed", domain.Account{ID: "b", Name: "Mortgage", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityPositiveOwed}, -12500},
		{"mortgage negative owed", domain.Account{ID: "b", Name: "Mortgage", Class: domain.ClassLiability, LiabilitySign: domain.LiabilityNegativeOwed}, 12500},
		{"owners draw", domain.Account{ID: "b", Name: "Joint Checking", Class: domain.ClassAsset}, 12500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			row := domain.Transaction{ID: "receiving", AccountID: "b", TransferAccountID: "a", Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Amount: money.New(tt.minor, "USD")}
			label := txnV5RowTransferLabel(tt.account.Name, "Source", txnclassify.TransferOutgoing(row, tt.account))
			if want := "Source → " + tt.account.Name; label != want {
				t.Fatalf("receiving leg label=%q want %q", label, want)
			}
			if got := txnV5RowTransferDisplay(label, "Source", row.Amount.Amount < 0); got != label {
				t.Fatalf("display reversed the account-aware label: %q", got)
			}
			tv8gApp(t, []domain.Transaction{row}, []domain.Account{{ID: "a", Name: "Source"}, tt.account}, currency.Rates{Base: "USD"})
			filter := uistate.CurrentTxFilter()
			filter.Account = "b"
			uistate.PersistTxFilter(filter)
			f := render.New(t, render.WithQueuedScheduler())
			defer f.Cleanup()
			defer func() { f.Render(Div()) }()
			f.Render(ui.CreateElement(Transactions))
			if node := byTestID(f, "tr", "txn-row-receiving"); node == nil || !strings.Contains(node.Text(), label) {
				t.Fatalf("account-scoped receiving row did not render %q: %s", label, f.Text())
			}
		})
	}
}

func TestTV8GPagerCountsFoldedMovements(t *testing.T) {
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	var rows []domain.Transaction
	for i := 0; i < 26; i++ {
		rows = append(rows,
			domain.Transaction{ID: fmt.Sprintf("out-%02d", i), TransferGroupID: fmt.Sprint(i), AccountID: "a", TransferAccountID: "b", Date: day, Amount: money.New(-100, "USD")},
			domain.Transaction{ID: fmt.Sprintf("in-%02d", i), TransferGroupID: fmt.Sprint(i), AccountID: "b", TransferAccountID: "a", Date: day, Amount: money.New(100, "USD")})
	}
	tv8gApp(t, rows, []domain.Account{{ID: "a", Name: "Checking"}, {ID: "b", Name: "Savings"}}, currency.Rates{Base: "USD"})
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	defer func() { f.Render(Div()) }()
	f.Render(ui.CreateElement(Transactions))
	if text := f.Text(); !strings.Contains(text, "26 transactions") || !strings.Contains(text, "1–25 of 26") || !strings.Contains(text, "of 2") {
		t.Fatalf("pager/count must use 26 movements, not 52 legs: %s", text)
	}
	for _, button := range f.AllByTag("button") {
		if button.Text() == "Next" {
			button.Click()
			f.Stabilize()
			if text := f.Text(); !strings.Contains(text, "26–26 of 26") {
				t.Fatalf("second page must contain only the last movement: %s", text)
			}
			return
		}
	}
	t.Fatal("next-page button missing")
}
