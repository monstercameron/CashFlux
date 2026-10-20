// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type primarySelectorProps struct {
	App  *appstate.App
	Txns []domain.Transaction
}

func primarySelectorFixture(p primarySelectorProps) ui.Node {
	_ = uistate.UseDataRevision().Get()
	accounts := p.App.Accounts()
	rates := currency.Rates{Base: "USD"}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	nw := useNetWorth(p.App, accounts, p.Txns, rates)
	income, _ := usePeriodTotals(p.App, p.Txns, start, start.AddDate(0, 1, 0), rates, "all")
	return P(fmt.Sprintf("%d/%d", nw.Net.Amount, income.Amount))
}

func TestPrimarySelectorsIsolateAppInstancesAtSameRevision(t *testing.T) {
	apps := make([]*appstate.App, 2)
	for i := range apps {
		app, err := appstate.New(io.Discard, false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = app.Store().Close() })
		if err := app.PutAccount(domain.Account{ID: "same", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
			t.Fatal(err)
		}
		if err := app.PutTransaction(domain.Transaction{ID: "income", AccountID: "same", Desc: "Pay", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Amount: money.New(int64((i+1)*100), "USD")}); err != nil {
			t.Fatal(err)
		}
		apps[i] = app
	}
	if apps[0].Rev() != apps[1].Rev() {
		t.Fatal("fixture needs same global revision")
	}
	f := render.New(t)
	defer f.Cleanup()
	for i, app := range apps {
		f.Render(ui.CreateElement(primarySelectorFixture, primarySelectorProps{App: app, Txns: app.Transactions()}))
		f.Stabilize()
		want := fmt.Sprintf("%d/%d", (i+1)*100, (i+1)*100)
		if f.Text() != want {
			t.Fatalf("app %d totals=%q, want %q", i, f.Text(), want)
		}
	}
}

func TestPrimarySelectorsInvalidateOnBrowserDataRevision(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Store().Close()
	if err := app.PutAccount(domain.Account{ID: "same", Name: "Checking", OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared, Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD"}); err != nil {
		t.Fatal(err)
	}
	transactions := []domain.Transaction{{ID: "income", AccountID: "same", Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Amount: money.New(100, "USD")}}
	f := render.New(t)
	defer f.Cleanup()
	// Read the external input at render time, just as a page reads live preferences
	// and data. BumpDataRevision can synchronously schedule a subscribed render.
	f.Render(ui.CreateElement(func(struct{}) ui.Node {
		return primarySelectorFixture(primarySelectorProps{App: app, Txns: transactions})
	}, struct{}{}))
	if f.Text() != "100/100" {
		t.Fatal(f.Text())
	}
	revision := app.Rev()
	transactions = append([]domain.Transaction(nil), transactions...)
	transactions[0].Amount = money.New(200, "USD")
	uistate.BumpDataRevision()
	f.Stabilize()
	if app.Rev() != revision {
		t.Fatal("store changed during browser-revision test")
	}
	if f.Text() != "200/200" {
		t.Fatalf("browser revision retained stale totals: %s", f.Text())
	}
}

func TestAccountSignatureIsOrderIndependentAndUnambiguous(t *testing.T) {
	first := []domain.Account{{ID: "a,b"}, {ID: "c"}}
	same := []domain.Account{{ID: "c"}, {ID: "a,b"}}
	different := []domain.Account{{ID: "a"}, {ID: "b,c"}}
	if acctSig(first) != acctSig(same) {
		t.Fatal("account ordering changed signature")
	}
	if acctSig(first) == acctSig(different) {
		t.Fatal("different scopes share a signature")
	}
}
