// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTXW1SortableHeaderExposesSortState(t *testing.T) {
	f := render.New(t)
	f.Render(uiw.DataTable(uiw.DataTableProps{
		Columns: []uiw.Column{{Label: "Description", SortKey: "description"}},
		Sort:    "description",
		Dir:     "desc",
		Body:    ui.Node(nil),
	}))

	var sortableHeader *render.QueryNode
	for _, header := range f.AllByTag("th") {
		if header.Attr("aria-sort") != "" {
			sortableHeader = header
			break
		}
	}
	if sortableHeader == nil {
		t.Fatal("sortable table header has no aria-sort state")
	}
	if got := sortableHeader.Attr("aria-sort"); got != "descending" {
		t.Fatalf("aria-sort = %q, want descending", got)
	}
}

func TestTXW1RowEditNamesDistinguishSameDescription(t *testing.T) {
	first := txnRowActionName("Edit", "Coffee", "Oct 2", "$7.25")
	second := txnRowActionName("Edit", "Coffee", "Oct 3", "$4.50")
	if first == second {
		t.Fatalf("same-description row actions collide: %q", first)
	}
	if first != "Edit Coffee, Oct 2, $7.25" {
		t.Fatalf("row action name = %q, want full row context", first)
	}
}
