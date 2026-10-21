// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestRecurringAgendaCalendarShowsUnavailableFXWithoutSourceAmount(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatalf("appstate.New: %v", err)
	}
	t.Cleanup(func() { _ = app.Store().Close() })
	settings := app.Settings()
	settings.BaseCurrency = "USD"
	settings.FXRates = nil
	if err := app.PutSettings(settings); err != nil {
		t.Fatalf("PutSettings: %v", err)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if err := app.PutRecurring(domain.Recurring{
		ID: "euro-bill", Label: "Euro bill", Amount: money.New(-12345, "EUR"),
		Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("PutRecurring: %v", err)
	}

	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(recurringAgendaUnavailableHost, recurringAgendaUnavailableProps{App: app, Now: now}))
	f.Stabilize()

	cell := recurringAgendaHostNode(f, "rhy-cal-2026-10-22")
	if cell == nil {
		t.Fatalf("calendar did not render the scheduled bill day: %s", f.Text())
	}
	label := uistate.T("transactions.calScheduledAmountUnavailable")
	if !strings.Contains(cell.Text(), "Euro bill") || !strings.Contains(cell.Text(), label) {
		t.Fatalf("calendar did not label the amount as unavailable: %s", cell.Text())
	}
	if strings.Contains(cell.Text(), "123.45") {
		t.Fatalf("calendar displayed source minor units as a base amount: %s", cell.Text())
	}
}

type recurringAgendaUnavailableProps struct {
	App *appstate.App
	Now time.Time
}

func recurringAgendaUnavailableHost(props recurringAgendaUnavailableProps) ui.Node {
	return rhyAgendaCalendar(props.App, props.Now, 0, "USD", func() {}, func() {}, func() {})
}

func recurringAgendaHostNode(f *render.Fixture, testID string) *render.QueryNode {
	for _, node := range f.AllByTag("div") {
		if node.Attr("data-testid") == testID {
			return node
		}
	}
	return nil
}
