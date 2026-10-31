// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestUXT3TransactionRowCarriesResponsiveColumnHooks(t *testing.T) {
	p := &txnFrameRowProps{
		ID: "uxt3-row", Date: "Oct 19", Amount: "-$12.00", Desc: "Market",
		Account: "Joint Checking", Category: "Food", Member: "Cam",
		Vis:    uistate.TxnCols{Category: true, Account: true, User: true, Amount: true},
		OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {},
	}
	columns := []uiw.Column{
		{Label: "Description", Class: "row-desc-col txn-col-description"},
		{Label: "Account", Class: "td-acct txn-col-account"},
		{Label: "Who", Class: "td-user txn-col-who"},
		{Label: "Amount", Class: "td-amount txn-col-amount"},
		{Label: "Actions", Class: "td-actions txn-col-actions"},
	}
	f := render.New(t)
	f.Render(uiw.DataTable(uiw.DataTableProps{Class: "txn-table", Columns: columns, Body: ui.CreateElement(txnFrameRow, p)}))

	seenCells, seenHeaders := make(map[string]bool), make(map[string]bool)
	var visit func(*render.QueryNode)
	visit = func(node *render.QueryNode) {
		if node == nil {
			return
		}
		for _, class := range []string{"txn-col-description", "txn-col-account", "txn-col-who", "txn-col-amount", "txn-col-actions", "txn-desc-account"} {
			if hasClass(node.Attr("class"), class) {
				if node.Tag() == "th" {
					seenHeaders[class] = true
				} else {
					seenCells[class] = true
				}
			}
		}
		for _, child := range node.Children() {
			visit(child)
		}
	}
	visit(f.Container())
	for _, class := range []string{"txn-col-description", "txn-col-account", "txn-col-who", "txn-col-amount", "txn-col-actions", "txn-desc-account"} {
		if !seenCells[class] {
			t.Errorf("rendered transaction cell is missing responsive hook %q", class)
		}
		if class != "txn-desc-account" && !seenHeaders[class] {
			t.Errorf("rendered transaction header is missing responsive hook %q", class)
		}
	}
}

func TestUXT3ToolbarHasPrimaryAndActionsRows(t *testing.T) {
	f := render.New(t)
	toolbar := uiw.FilterToolbar(uiw.FilterToolbarProps{
		SearchLabel: "Search transactions", FiltersLabel: "Filters",
		OnSearch: func(string) {},
		Actions: []ui.Node{
			shorthand.Div(shorthand.ClassStr("txn-period-control")),
			shorthand.Div(shorthand.ClassStr("txn-viewtoggle")),
			shorthand.Div(shorthand.ClassStr("txn-toolbar-add")),
		},
	})
	f.Render(txnResponsiveToolbar(toolbar))

	var primary, actions *render.QueryNode
	responsive := false
	var visit func(*render.QueryNode)
	visit = func(node *render.QueryNode) {
		if node == nil {
			return
		}
		switch node.Attr("class") {
		case "txn-responsive-toolbar":
			responsive = true
		case "filter-toolbar-primary":
			primary = node
		case "filter-toolbar-actions":
			actions = node
		}
		for _, child := range node.Children() {
			visit(child)
		}
	}
	visit(f.Container())
	if primary == nil || actions == nil {
		t.Fatalf("toolbar rows missing: primary=%t actions=%t", primary != nil, actions != nil)
	}
	if !responsive {
		t.Fatal("toolbar is missing its responsive layout scope")
	}
	if len(actions.Children()) != 3 || !hasClass(actions.Children()[0].Attr("class"), "txn-period-control") ||
		!hasClass(actions.Children()[1].Attr("class"), "txn-viewtoggle") ||
		!hasClass(actions.Children()[2].Attr("class"), "txn-toolbar-add") {
		t.Fatalf("period, view, and Add are not together in the actions row: %#v", actions.Children())
	}
}

func hasClass(classes, want string) bool {
	for _, class := range strings.Fields(classes) {
		if class == want {
			return true
		}
	}
	return false
}
