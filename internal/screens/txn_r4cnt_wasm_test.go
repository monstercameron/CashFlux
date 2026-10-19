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
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func TestR4CNTCountLineHidesUpcomingBillsDuringTextSearch(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := app.PutRecurring(domain.Recurring{
		ID: "r4cnt-bill", Label: "Internet", Amount: money.New(-22000, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 2),
	}); err != nil {
		t.Fatal(err)
	}
	previousApp, previousFilter := appstate.Default, uistate.CurrentTxFilter()
	appstate.Default = app
	uistate.PersistTxFilter(txnfilter.Criteria{
		Text: "coffee", From: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02"),
		To: time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Format("2006-01-02"),
	})
	t.Cleanup(func() {
		appstate.Default = previousApp
		uistate.PersistTxFilter(previousFilter)
	})

	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{Shown: 1, Total: 1}))
	line := r2aByTestID(f, "span", "txn-count-line")
	if line == nil {
		t.Fatal("count line is missing")
	}
	if strings.Contains(line.Text(), "bill due") {
		t.Fatalf("count line includes an unrelated scheduled bill while searching: %q", line.Text())
	}
}

func TestR4CNTFutureCountLineEndsWithScheduledAmountDue(t *testing.T) {
	app, err := appstate.New(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	for i := 0; i < 9; i++ {
		amount := int64(40078)
		if i == 8 {
			amount = 40076
		}
		if err := app.PutRecurring(domain.Recurring{
			ID: "r4cnt-future-" + string(rune('a'+i)), Label: "Scheduled bill", Amount: money.New(-amount, "USD"),
			Cadence: domain.CadenceMonthly, NextDue: start.AddDate(0, 0, i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	previousApp, previousFilter := appstate.Default, uistate.CurrentTxFilter()
	appstate.Default = app
	end := start.AddDate(0, 1, -1)
	uistate.PersistTxFilter(txnfilter.Criteria{From: start.Format("2006-01-02"), To: end.Format("2006-01-02")})
	t.Cleanup(func() {
		appstate.Default = previousApp
		uistate.PersistTxFilter(previousFilter)
	})

	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{}))
	line := r2aByTestID(f, "span", "txn-count-line")
	if line == nil || !strings.Contains(line.Text(), "9 scheduled. Due $3,607.00.") {
		if line == nil {
			t.Fatal("future count line is missing")
		}
		t.Fatalf("future count line = %q, want scheduled count and positive amount due", line.Text())
	}
}

func TestR4CNTUpcomingDisclosurePredicateOnlyRejectsTextSearch(t *testing.T) {
	if txnCountShouldShowUpcoming(txnfilter.Criteria{Text: "  coffee  "}) {
		t.Fatal("a non-empty text search should suppress the unrelated upcoming disclosure")
	}
	if !txnCountShouldShowUpcoming(txnfilter.Criteria{Category: "food"}) {
		t.Fatal("non-text filters should keep the upcoming disclosure available")
	}
}
