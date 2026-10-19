// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTV7FBulkToolbarHasRoleAndSelectedCount(t *testing.T) {
	f := render.New(t)
	f.Render(Div(append([]any{ClassStr("bulk-bar")}, txnBulkToolbarAttrs(10)...)...))
	for _, node := range f.AllByTag("div") {
		if node.Attr("role") == "toolbar" {
			if got := node.Attr("aria-label"); got != "10 selected" {
				t.Fatalf("bulk toolbar label = %q, want selected count", got)
			}
			return
		}
	}
	t.Fatal("bulk bar did not render with role=toolbar")
}

func TestTV7FSearchableFilterGroupCollapsesAndExpands(t *testing.T) {
	options := make([]uiw.SelectOption, 10)
	for i := range options {
		options[i] = uiw.SelectOption{Value: fmt.Sprintf("category-%d", i+1), Label: fmt.Sprintf("Category %d", i+1)}
	}
	f := render.New(t)
	f.Render(ui.CreateElement(filterSearchGroup, filterGroupProps{
		Label: "Category", Field: txnfilter.FieldCategory, Opts: options,
		Selected: []string{"category-10"},
	}))
	if got := len(f.AllByTag("button")); got != 10 {
		t.Fatalf("collapsed searchable group has %d buttons, want 8 options, selected option, and disclosure", got)
	}
	if text := f.Text(); !strings.Contains(text, "Category 10") || strings.Contains(text, "Category 9") {
		t.Fatalf("collapsed searchable options are incorrect: %q", text)
	}
	var disclosure *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if strings.HasPrefix(button.Attr("data-testid"), "filter-showall-") {
			disclosure = button
			break
		}
	}
	if disclosure == nil {
		t.Fatal("searchable group omitted its Show all disclosure")
	}
	disclosure.Click()
	f.Stabilize()
	if text := f.Text(); !strings.Contains(text, "Category 9") {
		t.Fatalf("expanded searchable options omit Category 9: %q", text)
	}
}
