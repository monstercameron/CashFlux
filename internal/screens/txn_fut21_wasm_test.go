// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestFUT21ScheduledRowsUseLedgerTableColumns(t *testing.T) {
	for _, tc := range []struct {
		name string
		cols uistate.TxnCols
	}{
		{name: "all optional columns", cols: uistate.TxnCols{Amount: true, Account: true, Category: true, Source: true, User: true}},
		{name: "hidden optional columns", cols: uistate.TxnCols{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, period := setupFUT21TableFixture(t)
			uistate.PersistTxnCols(tc.cols)
			t.Cleanup(func() { uistate.PersistTxnCols(uistate.DefaultTxnCols()) })
			previousApp := appstate.Default
			previousFilter := uistate.CurrentTxFilter()
			appstate.Default = app
			uistate.PersistTxFilter(period)
			t.Cleanup(func() {
				appstate.Default = previousApp
				uistate.PersistTxFilter(previousFilter)
			})

			frame := domain.NewFrame(
				domain.Field{Name: "id", Type: domain.FieldString},
				domain.Field{Name: "date", Type: domain.FieldDate},
				domain.Field{Name: "amount", Type: domain.FieldMoney},
				domain.Field{Name: "currency", Type: domain.FieldString},
				domain.Field{Name: "payee", Type: domain.FieldString},
				domain.Field{Name: "desc", Type: domain.FieldString},
				domain.Field{Name: "account", Type: domain.FieldString},
				domain.Field{Name: "category", Type: domain.FieldString},
				domain.Field{Name: "cleared", Type: domain.FieldBool},
				domain.Field{Name: "source", Type: domain.FieldString},
			)
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(txnTableWidget, txnTableProps{Frame: frame, App: app, Base: "USD"}))

			tables := f.AllByTag("table")
			if len(tables) != 1 {
				t.Fatalf("rendered %d tables, want the real ledger table", len(tables))
			}
			var row *render.QueryNode
			for _, candidate := range f.AllByTag("tr") {
				if candidate.Attr("data-testid") == "txn-scheduled-ghost" {
					row = candidate
					break
				}
			}
			if row == nil {
				t.Fatal("scheduled item is not rendered in the ledger table")
			}
			body := tables[0].Children()[2]
			if body.Tag() != "tbody" || len(body.Children()) != 1 || body.Children()[0].Attr("data-testid") != "txn-scheduled-ghost" {
				var children []string
				for _, child := range body.Children() {
					children = append(children, child.Tag()+":"+child.Attr("data-testid"))
				}
				t.Fatalf("scheduled rows are not inside the table tbody: tag=%s children=%v", body.Tag(), children)
			}
			headCells := tables[0].Children()[1].Children()[0].Children()
			if got := len(row.Children()); got != len(headCells) {
				t.Fatalf("scheduled row has %d cells for %d header columns", got, len(headCells))
			}
			if !strings.Contains(row.Text(), "Rent") || !strings.Contains(row.Text(), "Scheduled") {
				t.Fatalf("scheduled row content is incomplete: %q", row.Text())
			}
			if len(row.Children()) == 0 || !strings.Contains(row.Children()[0].Attr("class"), "td-select") ||
				len(row.Children()[0].Children()) != 0 || findByTestID(f, "button", "txn-rowedit") != nil {
				t.Fatal("scheduled row unexpectedly exposes selection or edit controls")
			}
			menu := findByTestID(f, "button", "txn-scheduled-menu-toggle")
			if menu == nil {
				t.Fatal("scheduled row menu is missing")
			}
			menu.Click()
			open := findByTestID(f, "a", "txn-scheduled-open")
			if open == nil || open.Text() != "Open in Bills & repeating payments" {
				t.Fatalf("schedule menu item = %v, want Open in Bills & repeating payments", open)
			}
		})
	}
}

func setupFUT21TableFixture(t *testing.T) (*appstate.App, txnfilter.Criteria) {
	t.Helper()
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatalf("create FUT21 app: %v", err)
	}
	if err := app.PutAccount(domain.Account{
		ID: "fut21-checking", Name: "Checking", Currency: "USD",
		OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
		Class: domain.ClassAsset, Type: domain.TypeChecking,
	}); err != nil {
		t.Fatalf("add FUT21 account: %v", err)
	}
	now := time.Now()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	endMonth := nextMonth.AddDate(0, 1, -1)
	if err := app.PutRecurring(domain.Recurring{
		ID: "fut21-rent", Label: "Rent", Amount: money.New(-150000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: nextMonth,
	}); err != nil {
		t.Fatalf("add FUT21 scheduled item: %v", err)
	}
	posted := now.AddDate(0, -1, 0)
	if err := app.PutTransaction(domain.Transaction{
		ID: "fut21-old-posted", AccountID: "fut21-checking", Date: posted, Desc: "Earlier posted transaction", Amount: money.New(-100, "USD"),
	}); err != nil {
		t.Fatalf("add out-of-period transaction: %v", err)
	}
	return app, txnfilter.Criteria{
		From: nextMonth.Format("2006-01-02"), To: endMonth.Format("2006-01-02"),
		PeriodPreset: txnfilter.MonthPeriodPreset(nextMonth.Year(), nextMonth.Month()),
		Page:         1, PageSize: 25,
	}
}
