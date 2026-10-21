// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/untracked"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTrackIncludeCheckboxUsesItsCheckedValue(t *testing.T) {
	row := trackRow{Cand: untracked.Candidate{CategoryID: "car", Name: "Car insurance"}, Include: true}
	var changed trackRow
	f := render.New(t)
	f.Render(ui.CreateElement(trackUntrackedRow, trackUntrackedRowProps{
		Row: row, Base: "USD", OnChange: func(next trackRow) { changed = next },
	}))

	checkbox := findByTestID(f, "input", "track-include-car")
	if checkbox == nil {
		t.Fatal("include checkbox did not render")
	}
	if got := prop(checkbox, "checked"); got != "true" {
		t.Fatalf("initial checked property = %q, want true for an included row", got)
	}

	checkbox.Dispatch("onchange", render.Event{Checked: true})
	if !changed.Include {
		t.Fatal("checking the row removed it from the selected state")
	}
	checkbox.Dispatch("onchange", render.Event{Checked: false})
	if changed.Include {
		t.Fatal("unchecking the row left it selected")
	}
}

func TestTrackFooterCountMatchesIncludedCheckboxes(t *testing.T) {
	rows := []trackRow{
		{Cand: untracked.Candidate{CategoryID: "car", Name: "Car insurance"}, Include: true},
		{Cand: untracked.Candidate{CategoryID: "tax", Name: "Property tax"}, Include: false},
	}
	var selected []untracked.Choice
	for _, row := range rows {
		if row.Include {
			selected = append(selected, untracked.Choice{CategoryID: row.Cand.CategoryID, AmountMinor: 100})
		}
	}
	effect, err := untracked.Impact(selected, 1000, 0)
	if err != nil {
		t.Fatalf("untracked.Impact: %v", err)
	}
	f := render.New(t)
	f.Render(renderTrackUntrackedSheet(
		renderTrackUntrackedRows(rows, "USD", nil, nil, func(string, trackRow) {}),
		effect, "USD", nil, nil, nil, nil,
	))

	checked := 0
	for _, checkbox := range f.AllByTag("input") {
		if strings.HasPrefix(checkbox.Attr("data-testid"), "track-include-") && prop(checkbox, "checked") == "true" {
			checked++
		}
	}
	if checked != effect.Categories {
		t.Fatalf("checked rows = %d, footer tracking count = %d", checked, effect.Categories)
	}
	if !strings.Contains(f.Text(), "Tracking 1 category") || !strings.Contains(f.Text(), "Track 1") {
		t.Fatalf("footer does not describe the one checked category: %q", f.Text())
	}
}
