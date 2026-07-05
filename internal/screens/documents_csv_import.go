// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// csvImportCardProps carries the state and handlers the CSV import card needs.
type csvImportCardProps struct {
	Accounts     []domain.Account
	ImportAcctID string
	Msg          string

	// DupWarn is non-empty when a pre-import preview detected duplicate rows.
	// It holds the human-readable warning string. When set, OnConfirmCSV
	// replaces the primary import action so the user can proceed knowingly.
	DupWarn string

	OnChooseFile ui.Handler
	OnAcctChange ui.Handler
	OnCsvInput   ui.Handler
	// OnImportCSV runs the preview first; if duplicates are found it sets
	// DupWarn and waits for OnConfirmCSV.
	OnImportCSV ui.Handler
	// OnConfirmCSV commits the import after the user acknowledges the warning.
	OnConfirmCSV ui.Handler
}

// CsvImportCard renders the CSV import section: file picker, account selector,
// paste textarea, and import button. Also shows the last status message.
func CsvImportCard(props csvImportCardProps) ui.Node {
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("documents.csvTitle"),
		Body: Fragment(
			// C9: local-first framing — make the no-bank-login trade-off explicit up front
			// so it reads as a privacy benefit rather than a missing feature.
			P(css.Class("muted", tw.Text12), Attr("data-testid", "local-first-note"),
				uistate.T("documents.localFirstNote")),
			P(css.Class("muted"), uistate.T("documents.csvDesc")),
			// C19: collapsible "how to get your bank's CSV" guidance — most users don't
			// know their bank exports one. Closed by default so it doesn't add noise.
			Details(css.Class("csv-help"), Attr("data-testid", "csv-bank-help"),
				Summary(uistate.T("documents.bankCsvHelpTitle")),
				P(css.Class("muted", tw.Mt1), uistate.T("documents.bankCsvHelpBody")),
			),
			// File picker: the primary path for real .csv files (C60).
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter, tw.Mt1),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "csv-file-picker"),
					OnClick(props.OnChooseFile), uistate.T("documents.chooseCsvFile")),
				Span(css.Class("muted"), uistate.T("documents.csvFileOrPaste")),
			),
			Form(OnSubmit(props.OnImportCSV),
				// Account selector + Import button appear above the textarea so they are
				// always visible without scrolling on short viewports (L44).
				Div(css.Class("form-grid"),
					Style(map[string]string{"margin-bottom": "0.5rem", "margin-top": "0.5rem"}),
					csvAcctSelect(props.Accounts, props.ImportAcctID, props.OnAcctChange),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.import")),
				),
				Textarea(css.Class("field field-wide"), Attr("rows", "6"),
					Placeholder("date,payee,amount,account\n2026-06-01,Salary,4200.00,Checking\n2026-06-02,Groceries,-86.40,Checking"),
					OnInput(props.OnCsvInput),
				),
			),
			// C88: pre-import duplicate warning — shown when the preview step detects
			// that some incoming rows match existing transactions. The user can confirm
			// ("Import anyway") to proceed with the existing skip-duplicate behavior, or
			// simply paste different data to start fresh.
			If(props.DupWarn != "",
				Div(css.Class("notice notice-warn", tw.Mt2), Attr("role", "alert"),
					Attr("data-testid", "csv-dup-warn"),
					Span(props.DupWarn),
					Button(css.Class("btn btn-sm"), Style(map[string]string{"margin-left": "0.5rem"}), Type("button"),
						Attr("data-testid", "csv-dup-confirm"),
						OnClick(props.OnConfirmCSV),
						uistate.T("documents.dupWarnConfirm")),
				),
			),
			If(props.Msg != "", P(css.Class("muted"), Attr("data-testid", "csv-import-msg"), props.Msg)),
		),
	})
}
