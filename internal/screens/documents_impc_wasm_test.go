// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestImportWizardUsesCSVQuotedFieldsAndMultilineRecordsIMPC(t *testing.T) {
	input := "\"Posted, date\",Description,Amount\r\n2026-01-02,\"Cafe, Main St\",-3.00\r\n2026-01-03,\"Cafe\nNorth\",-4.00"
	header, wrapped := extractRawHeaderAndRows(input, ',')
	if len(header) != 3 || header[0] != "Posted, date" {
		t.Fatalf("header = %#v", header)
	}
	if len(wrapped) != 2 || len(wrapped[0][0]) != 3 || wrapped[0][0][1] != "Cafe, Main St" || wrapped[1][0][1] != "Cafe\nNorth" {
		t.Fatalf("records = %#v", wrapped)
	}
}

func TestImportRowLimitAndArtifactFileCapsIMPC(t *testing.T) {
	if err := validateCSVRowLimit([]byte("date,desc,amount\n2026-01-01,Coffee,-1\n2026-01-02,Tea,-2"), 2); err != nil {
		t.Fatalf("two rows at cap: %v", err)
	}
	if err := validateCSVRowLimit([]byte("date,desc,amount\n2026-01-01,Coffee,-1\n2026-01-02,Tea,-2"), 1); err == nil {
		t.Fatal("expected row-cap error")
	}
	if got := artifactFileLimit("receipt.jpg", "image/jpeg"); got != 10*1024*1024 {
		t.Fatalf("image cap = %d", got)
	}
	if got := artifactFileLimit("statement.csv", "text/csv"); got != 25*1024*1024 {
		t.Fatalf("CSV cap = %d", got)
	}
	if got := artifactFileLimit("statement.pdf", "application/pdf"); got != 23*1024*1024 {
		t.Fatalf("PDF cap = %d", got)
	}
	if got := artifactFileLimit("statement.json", "application/json"); got != 45*1024*1024 {
		t.Fatalf("JSON cap = %d", got)
	}
}

func TestStatementFormatControlsFeedParserOverridesIMPC(t *testing.T) {
	options := statementParseOptions(string(statement.DayMonth), "comma")
	parsed, err := statement.ParseWithOptions(
		"Date;Description;Amount\n01/02/2026;Shop;1.234,56", 2, options,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Rows) != 1 || parsed.Rows[0].Date.Format("2006-01-02") != "2026-02-01" || parsed.Rows[0].Amount != 123456 {
		t.Fatalf("override parse = %#v", parsed)
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(importFormatControlsFixture, struct{}{}))
	if findByTestID(f, "select", "statement-date-order") == nil || findByTestID(f, "select", "statement-number-format") == nil {
		t.Fatal("date-order and number-format controls were not rendered")
	}
	if !strings.Contains(f.Text(), "2 ambiguous dates, 1 ambiguous numbers, 3 invalid rows") {
		t.Fatalf("row quality counts missing: %q", f.Text())
	}
}

func importFormatControlsFixture(_ struct{}) ui.Node {
	onChange := ui.UseEvent(func(ui.Event) {})
	onApply := ui.UseEvent(func() {})
	return statementFormatControls(string(statement.DayMonth), "comma", 2, 1, 3, onChange, onChange, onApply)
}

func TestImageImportDisclosesOpenAITransferBeforeActionIMPC(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(ImageImportCard, imageImportCardProps{}))
	if !strings.Contains(f.Text(), "image is sent to OpenAI") {
		t.Fatalf("AI transfer disclosure missing from image card: %q", f.Text())
	}
}

func TestDraftRowCategoryQuickAddOpensSharedPickerAndSelectsCreatedCategoryIMPC(t *testing.T) {
	app := newR3EEditApp(t)
	if err := app.PutCategory(domain.Category{ID: "cat-home", Name: "Home", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	var updated extract.Row
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(DraftRow, draftRowProps{
		Index:    0,
		Row:      extract.Row{Date: "2026-10-01", Description: "Repair", Amount: "-10.00"},
		Currency: "USD",
		OnRemove: func(int) {},
		OnUpdate: func(_ int, row extract.Row) { updated = row },
	}))
	var addButton *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if strings.Contains(button.Text(), "+ Category") {
			addButton = button
			break
		}
	}
	if addButton == nil {
		t.Fatal("draft row does not render the + Category affordance")
	}
	addButton.Click()
	f.Stabilize()
	if findByTestID(f, "div", "catpick") == nil {
		t.Fatal("+ Category did not open the shared CategoryPicker")
	}
	findByTestID(f, "input", "catpick-search").Input("Repairs")
	f.Stabilize()
	findByTestID(f, "button", "catpick-create").Click()
	f.Stabilize()
	if updated.Category != "Repairs" {
		t.Fatalf("selected draft category = %q, want Repairs", updated.Category)
	}
}

func TestDraftReviewPaginatesAll1240RowsAndKeepsGlobalCategoryIndexIMPC(t *testing.T) {
	rows := make([]extract.Row, 1240)
	for i := range rows {
		date := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		rows[i] = extract.Row{Date: date, Description: "row-" + strconv.Itoa(i), Amount: "-1.00"}
	}
	// This is an identical repeat in the source file, not a match against the
	// existing ledger. IM-C-7 keeps every source occurrence; pagination still
	// needs to count and render the repeat outside the visible page.
	rows[1239] = rows[10]
	start, end, page, pageCount := draftReviewWindow(len(rows), 3)
	if start != 200 || end != 300 || page != 3 || pageCount != 13 || end-start > draftReviewPageSize {
		t.Fatalf("page 3 window = (%d,%d), page %d of %d", start, end, page, pageCount)
	}
	rows = importdraft.New(rows).Rows

	app := newR3EEditApp(t)
	if err := app.PutCategory(domain.Category{ID: "cat-home", Name: "Home", Kind: domain.KindExpense}); err != nil {
		t.Fatal(err)
	}
	var updatedIndex int = -1
	var updated extract.Row
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(draftReviewFixture, draftReviewTestProps{
		rows: rows, page: page, start: start, end: end,
		onUpdate: func(i int, row extract.Row) { updatedIndex, updated = i, row },
	}))
	if !strings.Contains(f.Text(), "Rows 201–300 of 1240") || !strings.Contains(f.Text(), uistate.T("impc.importAllRows", 1240)) || !strings.Contains(f.Text(), "Possible repeated purchases in this draft: 1") {
		t.Fatalf("full-draft counts or page range missing: %q", f.Text())
	}
	if got := strings.Count(f.Text(), "row-"); got != draftReviewPageSize {
		t.Fatalf("rendered %d row labels, want at most %d", got, draftReviewPageSize)
	}
	if !strings.Contains(f.Text(), "row-200") || strings.Contains(f.Text(), "row-199") || strings.Contains(f.Text(), "row-300") {
		t.Fatalf("rendered page does not contain exactly the page 3 boundary rows: %q", f.Text())
	}
	var addButton *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if strings.Contains(button.Text(), "Category") {
			addButton = button
			break
		}
	}
	if addButton == nil {
		t.Fatal("page 3 row did not render its category picker")
	}
	addButton.Click()
	f.Stabilize()
	findByTestID(f, "input", "catpick-search").Input("Home")
	f.Stabilize()
	var homeButton *render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Text() == "Home" {
			homeButton = button
			break
		}
	}
	if homeButton == nil {
		t.Fatalf("category picker did not offer Home: %q", f.Text())
	}
	homeButton.Click()
	f.Stabilize()
	if updatedIndex != 200 || updated.Category != "Home" {
		t.Fatalf("page 3 category update = index %d, row %#v; want index 200 and Home", updatedIndex, updated)
	}
	result, err := app.ImportReviewedDocumentRows(domain.DocImage, app.Accounts()[0].ID, rows)
	if err != nil {
		t.Fatalf("import 1240-row draft: %v", err)
	}
	if result.Imported != 1240 || result.Skipped != 0 {
		t.Fatalf("1240-row import result = imported %d, skipped %d, want all 1240 distinct occurrences imported", result.Imported, result.Skipped)
	}
}

type draftReviewTestProps struct {
	rows     []extract.Row
	page     int
	start    int
	end      int
	onUpdate func(int, extract.Row)
}

func draftReviewFixture(props draftReviewTestProps) ui.Node {
	noop := ui.UseEvent(func(ui.Event) {})
	batch, err := appstate.Default.Stage(domain.DocCSV, props.rows, "acct-1")
	if err != nil {
		panic(err)
	}
	summary := importdraft.Summarize(batch.Batch)
	return ui.CreateElement(paginatedDraftReview, draftReviewProps{
		Rows: props.rows, VisibleRows: props.rows[props.start:props.end], Start: props.start,
		Page: props.page, PageCount: 13, ReviewCur: "USD", ImportAcctID: "acct-1", Summary: &summary,
		ClearDraft: noop, OnPage: func(int) {}, OnAcctChange: noop,
		OnReceiptTotal: noop, OnReceiptMerchant: noop, OnImportDraft: noop, OnImportReceipt: noop,
		OnRemoveDraft: func(int) {}, OnUpdateDraft: props.onUpdate,
	})
}
