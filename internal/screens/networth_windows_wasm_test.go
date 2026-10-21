// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestNetWorthUnavailableAccountWindowDoesNotRenderAsZero(t *testing.T) {
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		return nwsWindowMoveCell(nwsAcctRow{WindowKnown: false}, "USD")
	}))
	f.Stabilize()
	text := f.Text()
	if !strings.Contains(text, "—") || strings.Contains(text, "$0.00") {
		t.Fatalf("unavailable account movement rendered as a numeric zero: %q", text)
	}
}

func TestNetWorthScreenAdapterUsesPreparedWindowFacts(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	settings := app.Settings()
	settings.BaseCurrency, settings.FXRates = "USD", nil
	if err := app.PutSettings(settings); err != nil { t.Fatal(err) }
	account := netWorthFixtureAccount("adapter-cash", "USD", 10000, false)
	if err := app.PutAccount(account); err != nil { t.Fatal(err) }
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	if err := app.PutTransaction(domain.Transaction{ID: "adapter-flow", AccountID: account.ID,
		Date: now, Amount: money.New(-250, "USD")}); err != nil { t.Fatal(err) }

	view := computeNwsView(app, 6, now)
	if view.PreparationErr != nil || view.SeriesErr != nil || view.BridgeErr != nil {
		t.Fatalf("prepared facts unavailable: preparation=%v series=%v bridge=%v", view.PreparationErr, view.SeriesErr, view.BridgeErr)
	}
	if len(view.Accounts) != 1 || !view.Accounts[0].WindowKnown || view.Accounts[0].MoveMinor != -250 {
		t.Fatalf("screen adapter lost account window facts: %+v", view.Accounts)
	}
	if view.Latest().NetMinor != 9750 || view.Bridge.DeltaMinor() != -250 {
		t.Fatalf("screen adapter facts disagree: latest=%+v bridge=%+v", view.Latest(), view.Bridge)
	}
}
