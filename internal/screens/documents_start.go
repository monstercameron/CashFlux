// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// setDocumentsDraft keeps the persisted in-progress marker and visible wizard stage in sync.
func setDocumentsDraft(draft state.Atom[[]extract.Row], stage ui.State[string], reviewPage ui.State[int], rows []extract.Row) {
	if len(draft.Get()) == 0 && len(rows) > 0 {
		reviewPage.Set(1)
	}
	draft.Set(rows)
	if len(rows) > 0 {
		stage.Set("review")
		uistate.SetImportWIP(len(rows))
		return
	}
	stage.Set("input")
	uistate.ClearImportWIP()
}

// importTypePickerProps carries the stable, already-hooked callbacks for the start screen.
type importTypePickerProps struct {
	OnCSV, OnStatement, OnOrder, OnPDF, OnReceipt ui.Handler
}

// importTypePicker renders the Smart and Smart+ source choices.
func importTypePicker(props importTypePickerProps) ui.Node {
	return Div(css.Class("doc-picker"), Attr("data-testid", "import-type-picker"),
		Div(css.Class("doc-branch"),
			Div(css.Class("doc-branch-head"),
				Span(css.Class("doc-branch-title", "smart"), uistate.T("documents.smartBranch")),
				tierChip(false, uistate.T("documents.smartBranchTier")),
				Span(css.Class("doc-branch-sub"), uistate.T("documents.smartBranchDesc"))),
			Div(css.Class("doc-type-grid"),
				docTypeTile("import-type-csv", props.OnCSV, uiw.Icon(icon.Upload, css.Class(tw.W5, tw.H5)),
					uistate.T("documents.typeCsvTitle"), uistate.T("documents.typeCsvDesc"), false),
				docTypeTile("import-type-stmt", props.OnStatement, uiw.Icon(icon.Landmark, css.Class(tw.W5, tw.H5)),
					uistate.T("documents.typeStmtTitle"), uistate.T("documents.typeStmtDesc"), false),
				docTypeTile("import-type-amazon", props.OnOrder, uiw.Icon(icon.Box, css.Class(tw.W5, tw.H5)),
					uistate.T("orderimport.cardTitle"), uistate.T("orderimport.cardDesc"), false),
			),
		),
		Div(css.Class("doc-branch"),
			Div(css.Class("doc-branch-head"),
				Span(css.Class("doc-branch-title", "plus"), uistate.T("documents.smartPlusBranch")),
				tierChip(true, uistate.T("documents.smartPlusBranchTier")),
				Span(css.Class("doc-branch-sub"), uistate.T("documents.smartPlusBranchDesc"))),
			Div(css.Class("doc-type-grid"),
				docTypeTile("import-type-pdf", props.OnPDF, uiw.Icon(icon.FileText, css.Class(tw.W5, tw.H5)),
					uistate.T("documents.typePdfTitle"), uistate.T("documents.typePdfDesc"), true),
				docTypeTile("import-type-receipt", props.OnReceipt, uiw.Icon(icon.Receipt, css.Class(tw.W5, tw.H5)),
					uistate.T("documents.typeReceiptTitle"), uistate.T("documents.typeReceiptDesc"), true),
			),
		),
	)
}

// docTypeTile renders a source tile without registering per-item hooks in the picker.
func docTypeTile(testid string, onClick ui.Handler, ic ui.Node, title, desc string, smartPlus bool) ui.Node {
	tier := "smart"
	if smartPlus {
		tier = "smartplus"
	}
	return Button(css.Class("doc-type-tile", tier), Type("button"), Attr("data-testid", testid), OnClick(onClick),
		tierIcon(smartPlus, ic),
		Div(css.Class("dt-body"), Div(css.Class("dt-title"), title), Div(css.Class("dt-desc"), desc)),
	)
}

// importFormHeader labels a source form with its Smart or Smart+ identity.
func importFormHeader(plus bool, ic ui.Node, title, desc string) ui.Node {
	tierLabel := uistate.T("documents.smartBranch")
	if plus {
		tierLabel = uistate.T("documents.smartPlusBranch")
	}
	return Div(css.Class("doc-form-title"), tierIcon(plus, ic),
		Div(css.Class("doc-form-titletext"),
			H3(css.Class("doc-form-h"), Span(title), tierChip(plus, tierLabel)),
			P(css.Class("doc-form-desc"), desc),
		),
	)
}
