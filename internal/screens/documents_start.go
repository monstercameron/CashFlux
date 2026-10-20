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
	return Div(css.Class(importSourceGrid), Attr("data-testid", "import-type-picker"),
		docTypeTile("import-type-pdf", props.OnPDF, uiw.Icon(icon.FileText, css.Class(tw.W5, tw.H5)), uistate.T("importux.sourcePDF"), uistate.T("importux.sourcePDFDesc"), true),
		docTypeTile("import-type-csv", props.OnCSV, uiw.Icon(icon.Upload, css.Class(tw.W5, tw.H5)), uistate.T("documents.typeCsvTitle"), uistate.T("importux.sourceCSVDesc"), false),
		docTypeTile("import-type-amazon", props.OnOrder, uiw.Icon(icon.Box, css.Class(tw.W5, tw.H5)), uistate.T("orderimport.cardTitle"), uistate.T("importux.sourceOrderDesc"), false),
		docTypeTile("import-type-receipt", props.OnReceipt, uiw.Icon(icon.Receipt, css.Class(tw.W5, tw.H5)), uistate.T("documents.typeReceiptTitle"), uistate.T("importux.sourceReceiptDesc"), true),
		docTypeTile("import-type-stmt", props.OnStatement, uiw.Icon(icon.Landmark, css.Class(tw.W5, tw.H5)), uistate.T("importux.sourceStatement"), uistate.T("importux.sourceStatementDesc"), false),
	)
}

// docTypeTile renders a source tile without registering per-item hooks in the picker.
func docTypeTile(testid string, onClick ui.Handler, ic ui.Node, title, desc string, smartPlus bool) ui.Node {
	tier := "documents.smartBranch"
	if smartPlus {
		tier = "documents.smartPlusBranch"
	}
	return Button(css.Class(importSourceTile), Type("button"), Attr("data-testid", testid), OnClick(onClick),
		ic,
		Div(css.Class(importStack), Style(map[string]string{"gap": "8px"}),
			Div(Style(map[string]string{"display": "flex", "align-items": "baseline", "gap": "8px", "flex-wrap": "wrap"}),
				Span(Style(map[string]string{"font-weight": "600"}), title), Span(css.Class(importTier), uistate.T(tier))),
			Span(css.Class(importMuted), desc)),
	)
}

// importFormHeader labels a source form with its Smart or Smart+ identity.
func importFormHeader(plus bool, ic ui.Node, title, desc string) ui.Node {
	tierLabel := uistate.T("documents.smartBranch")
	if plus {
		tierLabel = uistate.T("documents.smartPlusBranch")
	}
	return Div(css.Class(importStack), Style(map[string]string{"gap": "8px"}),
		Div(Style(map[string]string{"display": "flex", "align-items": "center", "gap": "8px"}), ic,
			H3(Style(map[string]string{"margin": "0", "font-size": "18px", "font-weight": "600"}), title), Span(css.Class(importTier), tierLabel)),
		P(css.Class(importMuted), desc))
}
