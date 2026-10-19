// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR2FHouseholdDateFormatsRoundTrip(t *testing.T) {
	tests := []struct {
		style prefs.DateStyle
		shown string
	}{
		{style: prefs.DateUS, shown: "10/18/2026"},
		{style: prefs.DateEU, shown: "18/10/2026"},
		{style: prefs.DateLong, shown: "Oct 18, 2026"},
		{style: prefs.DateISO, shown: "2026-10-18"},
	}
	for _, tc := range tests {
		t.Run(string(tc.style), func(t *testing.T) {
			if got := formatFilterDate(tc.style, "2026-10-18"); got != tc.shown {
				t.Fatalf("display = %q, want %q", got, tc.shown)
			}
			if got, ok := parseFilterDate(tc.style, tc.shown); !ok || got != "2026-10-18" {
				t.Fatalf("parse = %q, %v, want ISO date", got, ok)
			}
		})
	}
	if _, ok := parseFilterDate(prefs.DateUS, "13/40/2026"); ok {
		t.Fatal("invalid household date was accepted")
	}
}

func TestR2FAccountCategoryAndTagGroupsAreSearchableMultiSelects(t *testing.T) {
	for _, field := range []txnfilter.FilterField{txnfilter.FieldAccount, txnfilter.FieldCategory, txnfilter.FieldTag} {
		t.Run(string(field), func(t *testing.T) {
			selected := ""
			f := render.New(t)
			f.Render(filterMultiGroup("Filter by "+string(field), field, nil, []uiw.SelectOption{
				{Value: "one", Label: "First option"},
				{Value: "two", Label: "Second option"},
			}, func(_ txnfilter.FilterField, value string) { selected = value }))
			inputs := f.AllByTag("input")
			if len(inputs) != 1 || inputs[0].Attr("role") != "combobox" {
				t.Fatalf("search control = %#v, want searchable combobox", inputs)
			}
			if strings.Contains(f.Text(), "Show all") {
				t.Fatalf("searchable %s group still has a Show all disclosure", field)
			}
			inputs[0].Dispatch("oninput", render.Event{Value: "Second"})
			f.Stabilize()
			options := 0
			for _, button := range f.AllByTag("button") {
				if button.Attr("role") == "option" {
					options++
					if button.Text() == "Second option" {
						button.Click()
					}
				}
			}
			if options != 1 || selected != "two" {
				t.Fatalf("rendered %d filtered options and selected %q, want one match and live selection", options, selected)
			}
		})
	}
}

func TestR2FAmountRangeShowsCurrencyPrefix(t *testing.T) {
	for _, label := range []string{"Min amount", "Max amount"} {
		t.Run(label, func(t *testing.T) {
			f := render.New(t)
			f.Render(amountField(label, "10", "", ui.Handler{}))
			found := false
			for _, node := range f.AllByTag("span") {
				if node.Attr("class") == "filter-amount-prefix" && node.Text() == "$" {
					found = true
				}
			}
			if !found {
				t.Fatal("amount range has no visible dollar prefix")
			}
		})
	}
}

func TestR2FQuickFilterPresetsDoNotDuplicateThePeriod(t *testing.T) {
	f := render.New(t)
	f.Render(txnPresetRow(2, 3, 4, false, false, false, func() {}, func() {}, func() {}))
	if strings.Contains(f.Text(), "This month") {
		t.Fatalf("quick filter presets duplicate the period selector: %q", f.Text())
	}
	for _, button := range f.AllByTag("button") {
		if button.Attr("data-testid") == "txn-preset-month" {
			t.Fatal("month quick filter is still rendered")
		}
	}
}
