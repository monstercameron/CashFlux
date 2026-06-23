//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// csvImportCardProps carries the state and handlers the CSV import card needs.
type csvImportCardProps struct {
	Accounts     []domain.Account
	ImportAcctID string
	Msg          string

	OnChooseFile ui.Handler
	OnAcctChange ui.Handler
	OnCsvInput   ui.Handler
	OnImportCSV  ui.Handler
}

// CsvImportCard renders the CSV import section: file picker, account selector,
// paste textarea, and import button. Also shows the last status message.
func CsvImportCard(props csvImportCardProps) ui.Node {
	return Section(css.Class("card"),
		H2(css.Class("card-title"), uistate.T("documents.csvTitle")),
		P(css.Class("muted"), uistate.T("documents.csvDesc")),
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
		If(props.Msg != "", P(css.Class("muted"), props.Msg)),
	)
}
