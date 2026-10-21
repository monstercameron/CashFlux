// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestAccountSummaryShowsUnavailableForOverflowingMonthDelta(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	account := domain.Account{
		ID: "extreme-networth", Name: "Extreme", OwnerID: domain.GroupOwnerID,
		Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking,
		Currency: "USD", OpeningBalance: money.New(math.MinInt64, "USD"),
	}
	if err := app.PutAccount(account); err != nil {
		t.Fatal(err)
	}
	for i, amount := range []int64{math.MaxInt64, math.MaxInt64} {
		txn := domain.Transaction{
			ID: "extreme-networth-" + string(rune('a'+i)), AccountID: account.ID,
			Date: time.Now(), Desc: "Current-month balance change", Amount: money.New(amount, "USD"),
		}
		if err := app.PutTransaction(txn); err != nil {
			t.Fatalf("PutTransaction(%s): %v", txn.ID, err)
		}
	}
	if balance, err := ledger.Balance(account, app.Transactions()); err != nil || balance.Amount != math.MaxInt64-1 {
		t.Fatalf("fixture current balance = %d, %v; want MaxInt64-1, nil", balance.Amount, err)
	}

	f := render.New(t)
	defer func() {
		f.Render(ui.Fragment())
		f.Stabilize()
		f.Cleanup()
	}()
	f.Render(ui.CreateElement(acctSummaryWidget, acctSummaryProps{
		App: app, Base: "USD", Rates: currency.Rates{Base: "USD"},
	}))
	f.Stabilize()
	if unavailable := byTestID(f, "span", "acct-networth-delta-unavailable"); unavailable == nil {
		t.Fatalf("overflowing month delta was not marked unavailable: %s", f.Text())
	}
	if zero := byTestID(f, "span", "acct-networth-delta-zero"); zero != nil {
		t.Fatalf("overflowing month delta was presented as known zero: %s", f.Text())
	}
	if strings.Contains(f.Text(), "No change this month") {
		t.Fatalf("overflowing month delta was presented as no change: %s", f.Text())
	}
}
