// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/subscriptions"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestSubscriptionRowUsesPreparedCheckedMonthlyFacts(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(SubscriptionRow, subscriptionRowProps{
		Sub:  subscriptions.Subscription{Name: "Annual plan", Cadence: subscriptions.CadenceYearly, Amount: 12000},
		Base: "USD", FinancialAvailable: true, MonthlyAmount: 1000, SharePercent: 25,
	}))
	f.Stabilize()
	got := f.Text()
	if !strings.Contains(got, "10.00") {
		t.Fatalf("row did not render the prepared $10/month average: %s", got)
	}
	shareBarFound := false
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "share-bar") && node.Attr("title") == "25%" {
			shareBarFound = true
		}
	}
	if !shareBarFound {
		t.Fatalf("row did not render the prepared 25%% share bar: %s", got)
	}
}

func TestSubscriptionRowMarksUnavailableMonthlyFacts(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(SubscriptionRow, subscriptionRowProps{
		Sub:  subscriptions.Subscription{Name: "Annual plan", Cadence: subscriptions.CadenceYearly, Amount: 12000},
		Base: "USD", FinancialAvailable: false,
	}))
	f.Stabilize()
	if got := f.Text(); !strings.Contains(got, uistate.T("common.notReady")) {
		t.Fatalf("row hid unavailable checked amount: %s", got)
	}
}
