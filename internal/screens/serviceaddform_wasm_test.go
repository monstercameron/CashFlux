// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/accountedit"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// inputWithID returns the rendered input with the id, or nil when it is not rendered.
func inputWithID(f *render.Fixture, id string) *render.QueryNode {
	for _, n := range f.AllByTag("input") {
		if n.Attr("id") == id {
			return n
		}
	}
	return nil
}

// renderServiceAddForm mounts the form and returns it with a counter of OnDone calls.
func renderServiceAddForm(t *testing.T) (*render.Fixture, *int) {
	t.Helper()
	done := 0
	f := render.New(t, render.WithQueuedScheduler())
	t.Cleanup(f.Cleanup)
	f.Render(ui.CreateElement(ServiceAddForm, ServiceAddFormProps{OnDone: func() { done++ }}))
	f.Stabilize()
	return f, &done
}

// submitService fills the name, provider and monthly cost, then submits the form.
func submitService(f *render.Fixture, name, provider, monthly string) {
	inputWithID(f, serviceNameInputID).Input(name)
	f.Stabilize()
	inputWithID(f, "service-add-provider").Input(provider)
	f.Stabilize()
	inputWithID(f, serviceMonthlyInputID).Input(monthly)
	f.Stabilize()
	byTestID(f, "form", "service-add-form").Submit()
	f.Stabilize()
}

// TestServiceAddFormSavesAMonthlyService checks a valid submit: the service is stored as a monthly
// recurring bill marked as a service, the form resets for the next one, and OnDone is called.
func TestServiceAddFormSavesAMonthlyService(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	f, done := renderServiceAddForm(t)

	submitService(f, "  Xfinity ", " Comcast ", "80.50")

	recs := app.Services()
	if len(recs) != 1 {
		t.Fatalf("services = %d, want 1: %s", len(recs), f.Text())
	}
	svc := recs[0]
	wantCur := accountedit.BaseCurrencyOrDefault(app.Settings().BaseCurrency)
	if !svc.Service || svc.Label != "Xfinity" || svc.Provider != "Comcast" {
		t.Fatalf("saved service = %+v, want a service named Xfinity from Comcast", svc)
	}
	if svc.Amount.Amount != -8050 || svc.Amount.Currency != wantCur {
		t.Fatalf("saved amount = %v, want -8050 %s", svc.Amount, wantCur)
	}
	if *done != 1 {
		t.Fatalf("OnDone called %d times, want 1", *done)
	}
	for _, id := range []string{serviceNameInputID, "service-add-provider", serviceMonthlyInputID} {
		// data-cf-value is the field's contents as the form holds them.
		if got := inputWithID(f, id).Attr("data-cf-value"); got != "" {
			t.Fatalf("input %s kept %q after a successful add, want it cleared", id, got)
		}
	}
	if strings.Contains(f.Text(), uistate.T("services.nameRequired")) || strings.Contains(f.Text(), uistate.T("services.amountRequired")) {
		t.Fatalf("a successful add left an error showing: %s", f.Text())
	}
}

// TestServiceAddFormRefusesBadInput checks each refusal: nothing is saved, OnDone is not called, the
// message appears, and the offending input is marked invalid and tied to the message.
func TestServiceAddFormRefusesBadInput(t *testing.T) {
	tests := []struct {
		name, service, monthly string
		wantMessage            string
		wantInput              string
		otherInput             string
	}{
		{"empty name", "   ", "10", "services.nameRequired", serviceNameInputID, serviceMonthlyInputID},
		{"zero amount", "Gym", "0", "services.amountRequired", serviceMonthlyInputID, serviceNameInputID},
		{"negative amount", "Gym", "-5", "services.amountRequired", serviceMonthlyInputID, serviceNameInputID},
		{"unreadable amount", "Gym", "ten", "services.amountRequired", serviceMonthlyInputID, serviceNameInputID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := primaryAccountsGoalsApp(t)
			f, done := renderServiceAddForm(t)

			submitService(f, tt.service, "", tt.monthly)

			if got := len(app.Services()); got != 0 {
				t.Fatalf("a refused submit saved %d services", got)
			}
			if *done != 0 {
				t.Fatalf("OnDone called %d times for a refused submit", *done)
			}
			if !strings.Contains(f.Text(), uistate.T(tt.wantMessage)) {
				t.Fatalf("message %q is missing: %s", uistate.T(tt.wantMessage), f.Text())
			}
			bad := inputWithID(f, tt.wantInput)
			if bad.Attr("aria-invalid") != "true" || bad.Attr("aria-describedby") != serviceErrID {
				t.Fatalf("offending input aria-invalid=%q aria-describedby=%q, want true and %q",
					bad.Attr("aria-invalid"), bad.Attr("aria-describedby"), serviceErrID)
			}
			if inputWithID(f, tt.otherInput).Attr("aria-invalid") == "true" {
				t.Fatal("the other input was marked invalid too")
			}
		})
	}
}
