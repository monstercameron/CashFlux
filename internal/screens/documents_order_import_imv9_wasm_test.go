// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/orderimport"
	"github.com/monstercameron/CashFlux/internal/ordermatch"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestOrderImportReviewShowsChargeAndItemsBeforeApply(t *testing.T) {
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	row := ordermatch.ReviewMatch{
		Match: orderimport.Match{
			Order: orderimport.Order{
				ID: "111-2222222-3333333", TotalMinor: 4510, Currency: "USD",
				Items: []orderimport.Item{{Name: "USB-C cable", UnitMinor: 2255, Qty: 2}},
			},
			Kind: orderimport.MatchSingle, TxnIDs: []string{"charge-1"},
			MatchedMinor: 3200, DriftMinor: 1310,
		},
		Charges: []domain.Transaction{{
			ID: "charge-1", Date: day, Payee: "Amazon Marketplace", Desc: "Amazon order",
			Amount: money.New(-3200, "USD"),
		}},
	}
	f := render.New(t)
	f.Render(ui.CreateElement(orderImportRow, orderImportRowProps{Review: row, Base: "USD"}))
	got := f.Text()
	for _, want := range []string{"Jun 1, 2026", "Amazon Marketplace", "$32.00", "USB-C cable", "2 ×", "$45.10", "Amounts differ by $13.10", "partial charge?", "Apply"} {
		if !strings.Contains(got, want) {
			t.Errorf("review is missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "covered by gift card") {
		t.Errorf("review claims a cause for the amount difference:\n%s", got)
	}
	itemsAt, applyAt := strings.Index(got, "Order items"), strings.Index(got, "Apply")
	if itemsAt < 0 || applyAt < itemsAt {
		t.Errorf("Apply must follow the item review:\n%s", got)
	}
}

func TestOrderImportEmptyPasteAsksForOrders(t *testing.T) {
	previous := appstate.Default
	appstate.Default = nil
	t.Cleanup(func() { appstate.Default = previous })
	f := render.New(t)
	f.Render(ui.CreateElement(OrderImportCard, orderImportCardProps{}))
	byTestID(f, "form", "order-paste-form").Dispatch("onsubmit", render.Event{})
	f.Stabilize()
	if !strings.Contains(f.Text(), "Paste your orders first.") {
		t.Fatalf("empty paste should ask for input:\n%s", f.Text())
	}
}
