// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
)

func TestImportDesignSuccessToastWaitsForDurability(t *testing.T) {
	fakeImportFetch(t)
	var notice state.Atom[uistate.Notice]
	f, current, _ := mountImportUXWithNotice(t, func(value state.Atom[uistate.Notice]) { notice = value })
	notice.Set(uistate.Notice{})
	current().chooseSource("stmt")
	current().appendRows([]extract.Row{{Date: "2026-03-01", Description: "Purchase", Amount: "-1.25"}})
	f.Stabilize()
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { done(nil) })
	defer restore()
	var complete func(appstate.DurableResult)
	current().app.SetDurableWriter(func(_ uint64, done func(appstate.DurableResult)) { complete = done })
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if complete == nil || notice.Get().Text != "" {
		t.Fatal("success was acknowledged before durable completion")
	}
	complete(appstate.DurableResult{Revision: current().app.Rev()})
	f.Stabilize()
	if notice.Get().Text != uistate.T("importux.importedOne", 1) || !notice.Get().Durable {
		t.Fatalf("unexpected success toast: %+v", notice.Get())
	}
}

func TestImportDesignStatementHeaderAndColumns(t *testing.T) {
	fakeImportFetch(t)
	f, current, _ := mountImportUX(t)
	current().chooseSource("stmt")
	current().appendRows([]extract.Row{
		{Date: "2026-03-03", Description: "Paycheck", Amount: "100.00"},
		{Date: "2026-03-01", Description: "Groceries", Amount: "-25.50"},
		{Date: "not-a-date", Description: "Needs a date", Amount: "-4.00"},
	})
	f.Stabilize()
	header := findByTestID(f, "div", "import-account-placement")
	if header == nil {
		t.Fatal("statement header missing")
	}
	for _, text := range []string{uistate.T("importux.importInto"), "3 transactions", "Mar 1, 2026 to Mar 3, 2026", "$100.00", "$25.50"} {
		if !strings.Contains(header.Text(), text) {
			t.Fatalf("statement header missing %q: %s", text, header.Text())
		}
	}
	table := findByTestID(f, "div", "draft-review-page")
	if table == nil || table.Attr("role") != "table" {
		t.Fatal("review must expose a statement table")
	}
	columns := []string{}
	for _, row := range table.Children() {
		if row.Attr("role") == "row" {
			for _, cell := range row.Children() {
				if cell.Attr("role") == "columnheader" {
					columns = append(columns, cell.Text())
				}
			}
			break
		}
	}
	if len(columns) != 5 || columns[0] != uistate.T("importux.date") || columns[1] != uistate.T("importux.description") || columns[2] != uistate.T("importux.category") || columns[3] != uistate.T("importux.amount") {
		t.Fatalf("statement column order = %v", columns)
	}
	if !strings.Contains(table.Text(), "Needs a date") || findByTestID(f, "p", "draft-row-error") == nil || !isDisabled(findByTestID(f, "button", "flip-save")) {
		t.Fatal("invalid row must remain visible with its reason and block import")
	}
	smartImportAccountPicker(t, f).Change("acct-2")
	f.Stabilize()
	if current().importAcct.Get() != "acct-2" {
		t.Fatal("dominant account selector cannot change placement")
	}
	current().removeDraft(2)
	f.Stabilize()
	if isDisabled(findByTestID(f, "button", "flip-save")) || !strings.Contains(findByTestID(f, "div", "import-statement-totals").Text(), "$25.50") {
		t.Fatal("statement facts/footer did not update after removing invalid row")
	}
}

func TestImportDesignInputPrecedesActions(t *testing.T) {
	for _, source := range []string{"stmt", "amazon", "csv"} {
		t.Run(source, func(t *testing.T) {
			fakeImportFetch(t)
			f, _, _ := mountImportUX(t)
			findByTestID(f, "button", "import-type-"+source).Click()
			f.Stabilize()
			progress := findByTestID(f, "nav", "import-progress")
			if progress == nil || !strings.Contains(progress.Text(), uistate.T("importux.source")) || !strings.Contains(progress.Text(), uistate.T("importux.read")) || !strings.Contains(progress.Text(), uistate.T("importux.review")) {
				t.Fatal("three-step progress is missing")
			}
			inputs := f.AllByTag("textarea")
			if len(inputs) != 1 || inputs[0].Attr("aria-label") == "" {
				t.Fatal("paste input must be labelled")
			}
			// Walk the form in document order. The paste field must precede the
			// action that consumes it, including the empty-input disabled state.
			for _, form := range f.AllByTag("form") {
				if form.Attr("id") == ImportModalFormID {
					continue
				}
				if source == "amazon" && form.Attr("data-testid") != "order-paste-form" {
					continue
				}
				children := form.Children()
				if len(children) < 2 || children[0].Tag() != "textarea" {
					t.Fatal("paste must precede the action row")
				}
				for _, action := range children[1].Children() {
					if action.Tag() == "button" && !isDisabled(action) {
						t.Fatal("empty paste action is enabled")
					}
				}
				break
			}
		})
	}
}

func TestImportDesignRemoveSelectedFile(t *testing.T) {
	for _, source := range []string{"pdf", "receipt"} {
		t.Run(source, func(t *testing.T) {
			fakeImportFetch(t)
			f, current, _ := mountImportUX(t)
			current().chooseSource(source)
			if source == "pdf" {
				current().pdfData.Set("data:application/pdf;base64,JVBERi0=")
				current().pdfName.Set("statement.pdf")
				current().pdfBytes.Set([]byte("%PDF-"))
			} else {
				current().imageURL.Set("data:image/png;base64,aW1hZ2U=")
				current().imageDraftAtom.Set(current().imageURL.Get())
				current().imageName.Set("receipt.png")
				current().imageSize.Set(5)
			}
			f.Stabilize()
			removeID, actionID := "statementimport-remove", "statementimport-run"
			if source == "receipt" {
				removeID, actionID = "import-remove-image", "import-read-image"
			}
			remove := findByTestID(f, "button", removeID)
			if remove == nil {
				t.Fatal("selected file cannot be removed")
			}
			remove.Click()
			f.Stabilize()
			if current().pdfData.Get() != "" || current().imageURL.Get() != "" || current().imageDraftAtom.Get() != "" || !isDisabled(findByTestID(f, "button", actionID)) {
				t.Fatal("removing file retained input or enabled Read")
			}
		})
	}
}
