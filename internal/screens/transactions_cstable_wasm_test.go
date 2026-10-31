// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txntable"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func cstableRow() *txnFrameRowProps {
	return &txnFrameRowProps{ID: "cstable", Desc: "Corner shop", Date: "Oct 2", Amount: "$6.75", AmountMoney: money.New(-675, "USD"),
		Account: "Checking", Category: "Food", CategoryID: "food", Source: "Imported", OwnersResolved: true, MemberNames: []string{"Cam"},
		Vis:    uistate.TxnCols{Account: true, Category: true, Amount: true, Source: true, User: true},
		OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {}}
}

func TestCSTABLESelectionCellOwnsClickHook(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	calls := 0
	r.OnToggleSelect = func(id string, shift bool) {
		if id != r.ID || shift {
			t.Fatalf("selection %s %v", id, shift)
		}
		calls++
	}
	f.Render(Table(Tbody(Tr(ui.CreateElement(txnRowSelectionCell, r)))))
	boxes := f.AllByTag("input")
	if len(boxes) != 1 || boxes[0].Name() != uistate.T("v5row.select", r.Desc) {
		t.Fatal("checkbox markup changed")
	}
	boxes[0].Click()
	if calls != 1 {
		t.Fatalf("selection callbacks=%d", calls)
	}
	next := *r
	next.Selected = true
	f.Rerender(Table(Tbody(Tr(ui.CreateElement(txnRowSelectionCell, &next)))))
	f.AllByTag("input")[0].Click()
	if calls != 2 {
		t.Fatal("selection lost its handler after render")
	}
}

func TestCSTABLEDescriptionCellOwnsEditAndInfoActions(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	opened, receipts := 0, 0
	r.OnOpen = func(id string) {
		if id != r.ID {
			t.Fatal(id)
		}
		opened++
	}
	r.Queued = true
	r.InfoText = "One receipt"
	r.Receipts = 1
	r.Attachment = domain.AttachmentRef{ArtifactID: "receipt"}
	r.OnViewReceipt = func(ref domain.AttachmentRef) {
		if ref.ArtifactID != "receipt" {
			t.Fatal(ref)
		}
		receipts++
	}
	f.Render(Table(Tbody(Tr(ui.CreateElement(txnRowDescriptionCell, txnFrameRowCellsProps{Row: r, SaveStatus: "Saving..."})))))
	edit := findByTestID(f, "button", "txn-description-edit")
	if edit == nil || edit.Name() != txnRowActionName(uistate.T("transactions.rowEdit"), r.Desc, r.Date, r.Amount) {
		t.Fatal("edit label changed")
	}
	edit.Click()
	if opened != 1 {
		t.Fatal("description did not edit")
	}
	if findByTestID(f, "span", "txn-review-pill") == nil || f.ByRole("status", "") == nil {
		t.Fatal("description lost review or save status")
	}
	for _, button := range f.AllByTag("button") {
		if strings.Contains(button.Name(), "One receipt") {
			button.Click()
		}
	}
	if receipts != 0 {
		t.Fatalf("receipt opened from the info button; it belongs in the popover: %d", receipts)
	}
}

func TestCSTABLETagChildPreservesCapAndRerenders(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	r.Tags = []string{"one", "two", "three", "four", "five"}
	clicked := ""
	r.OnTagClick = func(tag string) { clicked = tag }
	f.Render(Table(Tbody(Tr(Td(ui.CreateElement(txnRowTags, r))))))
	if findByTestID(f, "button", "txn-tag-four") != nil || findByTestID(f, "button", "txn-tag-three") == nil {
		t.Fatal("tag cap changed")
	}
	tags := findByTestID(f, "span", "txn-row-tags")
	if tags == nil || !strings.Contains(tags.Text(), "+2") {
		t.Fatal("tag overflow changed")
	}
	findByTestID(f, "button", "txn-tag-two").Click()
	if clicked != "two" {
		t.Fatal(clicked)
	}
	next := *r
	next.Tags = nil
	f.Rerender(Table(Tbody(Tr(Td(ui.CreateElement(txnRowTags, &next))))))
	if findByTestID(f, "span", "txn-row-tags") != nil {
		t.Fatal("empty tags still render")
	}
	next2 := *r
	next2.Tags = []string{"again"}
	f.Rerender(Table(Tbody(Tr(Td(ui.CreateElement(txnRowTags, &next2))))))
	findByTestID(f, "button", "txn-tag-again").Click()
	if clicked != "again" {
		t.Fatal(clicked)
	}
}

func TestCSTABLECategoryCellOwnsChooserAndRetainsTooltip(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	r.CategoryTooltip = "Exact category detail"
	calls := 0
	f.Render(Table(Tbody(Tr(ui.CreateElement(txnRowCategoryCell, txnFrameRowCellsProps{Row: r, Choose: func(ui.Event) { calls++ }})))))
	button := findByTestID(f, "button", "txn-category-choose")
	if button == nil {
		t.Fatal("category chooser missing")
	}
	button.Click()
	if calls != 1 {
		t.Fatal("category callback missing")
	}
	cell := f.AllByTag("td")[0]
	if cell.Attr("title") != r.CategoryTooltip || cell.Attr("aria-label") != "Food" {
		t.Fatal("category markup changed")
	}
	next := *r
	next.Unfiled = true
	next.SuggestedCategory = "Dining"
	next.Category = "Uncategorized"
	next.CategoryTooltip = ""
	f.Rerender(Table(Tbody(Tr(ui.CreateElement(txnRowCategoryCell, txnFrameRowCellsProps{Row: &next})))))
	if findByTestID(f, "button", "txn-category-choose") != nil {
		t.Fatal("inline suggestion should replace chooser")
	}
	if got := f.AllByTag("td")[0].Attr("aria-label"); got != uistate.T("r5w.suggestedCategory", next.Category, next.SuggestedCategory) {
		t.Fatal(got)
	}
}

func TestCSTABLEActionsCellOwnsEditAndMenu(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	edits, deletes := 0, 0
	r.OnOpen = func(string) { edits++ }
	r.OnDelete = func(string) { deletes++ }
	f.Render(Table(Tbody(Tr(ui.CreateElement(txnRowActionsCell, r)))))
	if findByTestID(f, "button", "txn-rowedit") != nil {
		t.Fatal("edit has its own button; it belongs in the row menu")
	}
	trigger := findByTestID(f, "button", "txn-kebab-"+r.ID)
	if trigger == nil {
		t.Fatal("row menu missing")
	}
	trigger.Click()
	f.Stabilize()
	edit := findByTestID(f, "button", "txn-rowedit")
	if edit == nil || edit.Name() != uistate.T("transactions.rowEdit") {
		t.Fatal("edit menu item missing or misnamed")
	}
	edit.Click()
	if edits != 1 {
		t.Fatal("edit handler")
	}
	trigger.Click()
	f.Stabilize()
	findByTestID(f, "button", "txn-delete").Click()
	if deletes != 1 {
		t.Fatal("delete handler")
	}
}

func TestCSTABLERowCellsRetainColumnOrderAndARIA(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	r := cstableRow()
	r.RowIndex = 42
	r.Selected = true
	r.Cleared = true
	r.ExcludedFromReports = true
	r.ShowBalance = true
	r.Balance = "$100.00"
	r.GroupSize = 2
	f.Render(Table(Tbody(ui.CreateElement(txnFrameRowCells, txnFrameRowCellsProps{Row: r, SaveStatus: "Saving..."}))))
	row := findByTestID(f, "tr", "txn-row-"+r.ID)
	if row == nil || row.Attr("aria-rowindex") != "42" || row.Attr("aria-busy") != "true" {
		t.Fatal("row markers changed")
	}
	for _, class := range []string{"row", "txn-ledger-row", "selected", "cleared", "txn-excluded", "txn-row-grouped"} {
		if !hasClass(row.Attr("class"), class) {
			t.Fatalf("row class=%q is missing %q", row.Attr("class"), class)
		}
	}
	want := []string{"td-select", "td-date", "row-desc-cell txn-col-description", "td-cat", "td-acct txn-col-account", "td-user txn-col-who", "td-amount txn-col-amount ", "td-amount ", "td-source", "td-actions txn-col-actions"}
	cells := row.Children()
	if len(cells) != len(want) {
		t.Fatalf("cells %d", len(cells))
	}
	for i, cell := range cells {
		for _, class := range strings.Fields(want[i]) {
			if !hasClass(cell.Attr("class"), class) {
				t.Fatalf("cell %d class=%q is missing %q", i, cell.Attr("class"), class)
			}
		}
	}
}

func TestCSTABLETableBodyPreservesPageAndRegisterHeaders(t *testing.T) {
	app, _ := tv7ScopeApp(t)
	for _, register := range []bool{false, true} {
		t.Run(map[bool]string{false: "sortable", true: "register"}[register], func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			r := cstableRow()
			p := txnTableBodyProps{App: app, Filter: txnfilter.Criteria{Sort: "date", Dir: txnfilter.Asc}, Columns: r.Vis,
				Page: txntable.Paginate(1, 25, 1), Total: 1, Visible: []string{r.ID}, Register: register, ShowBalance: register,
				RenderRow: func(int) ui.Node { return ui.CreateElement(txnFrameRow, r) }, IDAt: func(int) any { return r.ID }}
			f.Render(ui.CreateElement(txnTableBody, p))
			if len(f.AllByTag("table")) != 1 || findByTestID(f, "tr", "txn-row-"+r.ID) == nil {
				t.Fatal("table rows missing")
			}
			want := 9
			if register {
				want++
			}
			if len(f.AllByTag("th")) != want {
				t.Fatal("header count")
			}
			for _, th := range f.AllByTag("th") {
				if register && th.Attr("aria-sort") != "" {
					t.Fatal("register advertised sorting")
				}
			}
		})
	}
}

func TestCSTABLETableRegionRetainsCountAndChildBody(t *testing.T) {
	app, rows := tv7ScopeApp(t)
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnTableRegion, txnTableRegionProps{App: app, Shown: rows, Base: "USD", Body: P(Attr("data-testid", "cstable-body"), "body"), Category: txnTableCategoryTarget{Choose: func(string) {}, Close: func() {}}}))
	region := f.ByRole("region", uistate.T("txw1.tableRegion"))
	if region == nil || findByTestID(f, "p", "cstable-body") == nil || findByTestID(f, "div", "txn-table-count-facts") == nil {
		t.Fatal("table region lost child or count")
	}
}

func TestCSTABLESingleAccountScopeKeepsHookOrder(t *testing.T) {
	_, rows := tv7ScopeApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	defer func() { f.Render(Div()) }()
	f.Render(ui.CreateElement(Transactions))
	for _, account := range []string{"checking", "", "loan", ""} {
		filter := uistate.CurrentTxFilter()
		filter.Account = account
		uistate.PersistTxFilter(filter)
		f.Rerender(ui.CreateElement(Transactions))
		f.Stabilize()
		if len(f.AllByTag("table")) != 1 {
			t.Fatalf("account %q lost table after scope transition", account)
		}
	}
	tv7AssertRows(t, f, rows, true)
}
