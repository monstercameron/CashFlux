// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestV5BulkCollapsedFilterShowsEightOptionsAndSelectedOutsideRange(t *testing.T) {
	options := make([]uiw.SelectOption, 10)
	for i := range options {
		options[i] = uiw.SelectOption{Value: fmt.Sprintf("value-%d", i+1), Label: fmt.Sprintf("Option %d", i+1)}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(filterGroup, filterGroupProps{
		Label: "Category", Field: txnfilter.FieldCategory,
		Opts: options, Selected: []string{"value-10"},
	}))

	if got := len(f.AllByTag("button")); got != 10 {
		t.Fatalf("collapsed group has %d buttons, want 8 options, selected option, and disclosure", got)
	}
	text := f.Text()
	for _, want := range []string{"Option 1", "Option 8", "Option 10", "Show all 10"} {
		if !strings.Contains(text, want) {
			t.Errorf("collapsed filter group text %q does not include %q", text, want)
		}
	}
	if strings.Contains(text, "Option 9") {
		t.Errorf("collapsed filter group unexpectedly showed unselected option 9: %q", text)
	}
}

func TestV5BulkFilterGroupCollapseThresholdIsEight(t *testing.T) {
	if filterGroupCollapseThreshold != 8 {
		t.Fatalf("filter group collapse threshold = %d, want 8", filterGroupCollapseThreshold)
	}
	options := make([]uiw.SelectOption, 9)
	for i := range options {
		options[i] = uiw.SelectOption{Value: fmt.Sprintf("value-%d", i+1), Label: fmt.Sprintf("Option %d", i+1)}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(filterGroup, filterGroupProps{Label: "Category", Field: txnfilter.FieldCategory, Opts: options}))
	if len(f.AllByTag("button")) != 9 {
		t.Fatal("nine-option filter group should collapse and show the disclosure")
	}
}
