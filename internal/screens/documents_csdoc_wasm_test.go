// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestWizardMappingChildControlsCSDOC(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	var values [5]string
	applied := 0
	props := wizardMappingProps{header: []string{"Date", "Description", "Amount"}, selDate: "0", selDesc: "1", selAmount: "2", selDebit: "-1", selCredit: "-1",
		onDate: func(e ui.Event) { values[0] = e.GetValue() }, onDesc: func(e ui.Event) { values[1] = e.GetValue() }, onAmount: func(e ui.Event) { values[2] = e.GetValue() }, onDebit: func(e ui.Event) { values[3] = e.GetValue() }, onCredit: func(e ui.Event) { values[4] = e.GetValue() }, onApply: func() { applied++ }}
	for _, header := range [][]string{props.header, nil, {"", "Posted", "Memo", "Debit", "Credit", "Balance"}} {
		props.header = header
		f.Render(ui.CreateElement(wizardMappingForm, props))
		selects := f.AllByTag("select")
		if len(selects) != 5 || len(f.AllByTag("option")) != 5*(len(header)+1) {
			t.Fatalf("mapping structure changed: %d controls", len(selects))
		}
		for i, selectNode := range selects {
			if selectNode.Attr("id") == "" || selectNode.Attr("aria-label") == "" || selectNode.Attr("class") != "field" {
				t.Fatal("mapping control attributes changed")
			}
			selectNode.Change("1")
			if values[i] != "1" {
				t.Fatalf("control %d did not forward its value", i)
			}
		}
		f.AllByTag("form")[0].Submit()
		f.Stabilize()
	}
	if applied != 3 {
		t.Fatalf("apply callbacks = %d", applied)
	}
}

func TestWizardProfileChildControlsCSDOC(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	name := ""
	saved := 0
	f.Render(ui.CreateElement(wizardProfileForm, wizardProfileProps{profileNameVal: "Bank mapping", onProfileName: func(value string) { name = value }, onSaveProfile: func() { saved++ }}))
	input := f.AllByTag("input")[0]
	if input.Attr("aria-label") == "" || input.Attr("placeholder") == "" || input.Attr("data-cf-value") != "Bank mapping" {
		t.Fatal("profile input contract changed")
	}
	input.Input("Changed bank")
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if name != "Changed bank" || saved != 1 {
		t.Fatalf("profile callbacks: %q %d", name, saved)
	}
}

func TestOrderInputChildControlsCSDOC(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	chosen, submitted := 0, 0
	paste := ""
	f.Render(ui.CreateElement(orderImportInput, orderImportInputProps{onChooseFile: func() { chosen++ }, onFindPaste: func() { submitted++ }, onPasteInput: func(value string) { paste = value }}))
	byTestID(f, "button", "order-file-picker").Click()
	field := byTestID(f, "textarea", "order-paste")
	if field.Attr("id") != "order-paste" || field.Attr("rows") != "5" || field.Attr("class") != "field field-wide" {
		t.Fatal("order paste attributes changed")
	}
	field.Input("Order 123")
	byTestID(f, "form", "order-paste-form").Submit()
	f.Stabilize()
	if chosen != 1 || submitted != 1 || paste != "Order 123" {
		t.Fatalf("order callbacks: %d %d %q", chosen, submitted, paste)
	}
}

func TestReviewMemoRefreshesSameLengthEditsCSDOC(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.appendRows([]extract.Row{{Date: "2026-03-01", Description: "Shop", Amount: "-2.00"}})
	f.Stabilize()
	c = current()
	if c.summary.Expense != 200 {
		t.Fatalf("initial summary: %+v", c.summary)
	}
	row := c.model.Rows[0]
	row.Amount = "-3.00"
	c.updateDraft(0, row)
	f.Stabilize()
	c = current()
	if c.summary.Expense != 300 || c.summary.Net != -300 || !strings.Contains(f.Text(), "$3.00") {
		t.Fatalf("same-length edit left stale summary: %+v", c.summary)
	}
	c.removeDraft(0)
	f.Stabilize()
	if len(current().model.Rows) != 0 || current().summary.Importable != 0 {
		t.Fatal("removed draft left stale review")
	}
}

func TestSpendSummaryDoesNotPublishInvalidTotalsCSDOC(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rows    []extract.Row
		visible bool
	}{
		{"signed rows", []extract.Row{{Date: "2026-01-31", Amount: "-1.00"}, {Date: "2026-01-31", Amount: "2.00"}}, true},
		{"overflow", []extract.Row{{Amount: "92233720368547758.07"}, {Amount: "1.00"}}, false},
		{"missing FX", []extract.Row{{Currency: "EUR", Amount: "1.00"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			f.Render(ui.CreateElement(SpendSummaryCard, spendSummaryCardProps{Rows: tc.rows, BaseCurrency: "USD"}))
			if (f.Text() != "") != tc.visible {
				t.Fatalf("summary visible=%v: %s", tc.visible, f.Text())
			}
		})
	}
}
