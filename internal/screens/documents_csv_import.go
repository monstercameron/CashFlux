// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"strings"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// csvImportCardProps carries the state and handlers the CSV import card needs.
type csvImportCardProps struct {
	Accounts     []domain.Account
	ImportAcctID string

	CSVText      string
	OnChooseFile ui.Handler
	OnAcctChange ui.Handler
	OnCsvInput   ui.Handler
	// OnImportCSV runs the preview first; if duplicates are found it sets
	// DupWarn and waits for OnConfirmCSV.
	OnImportCSV ui.Handler
	// Preflight is the staged import's pre-commit preview card (#57): counts,
	// balance impact + jump warning, duplicate reasons, transfer pairs, and
	// the Import now / Cancel actions. Nil when nothing is staged.
	Preflight ui.Node
}

// CsvImportCard renders the CSV import inputs: a file picker, an account selector, a
// paste textarea + Import button, a collapsible "where do I get a CSV?" helper, and the
// duplicate warning / status line. The form's title + description live in the shared
// importFormHeader, so this returns just the input body.
func CsvImportCard(props csvImportCardProps) ui.Node {
	return Div(css.Class(importStack),
		// File picker: the primary path for real .csv files (C60).
		Div(css.Class(importDropZone),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "csv-file-picker"),
				OnClick(props.OnChooseFile), uistate.T("documents.chooseCsvFile")),
			Span(css.Class("muted", tw.Text13), uistate.T("documents.csvFileOrPaste")),
		),
		Form(css.Class(importStack), OnSubmit(props.OnImportCSV),
			Textarea(css.Class("field field-wide"), Attr("rows", "6"),
				Placeholder(uistate.T("imb.csvSample")), Value(props.CSVText), uiw.FieldValue(props.CSVText),
				OnInput(props.OnCsvInput), Attr("aria-label", uistate.T("documents.typeCsvTitle")),
			),
			Div(css.Class(importActionRow), csvAcctSelect(props.Accounts, props.ImportAcctID, props.OnAcctChange),
				Button(css.Class("btn btn-primary"), Type("submit"), Disabled(strings.TrimSpace(props.CSVText) == ""), uistate.T("importux.previewCSV"))),
		),
		// Terse column hint, then details tucked into a collapsible so they don't crowd
		// the input.
		P(css.Class("muted", tw.Text12), Attr("data-testid", "local-first-note"),
			uistate.T("importux.readLocal")),
		Details(css.Class("csv-help"), Attr("data-testid", "csv-bank-help"),
			Summary(uistate.T("documents.bankCsvHelpTitle")),
			P(css.Class("muted", tw.Mt1), uistate.T("documents.bankCsvHelpBody")),
			P(css.Class("muted", tw.Mt1), uistate.T("documents.localFirstNote")),
		),
		// #57: the staged pre-commit preview (replaces the old dup-only warning
		// as the primary two-step; DupWarn is kept for any legacy setters).
		props.Preflight,
	)
}
