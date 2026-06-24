// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// importHistoryListProps carries the data and callbacks the ImportHistoryList needs.
type importHistoryListProps struct {
	Docs     []domain.Document
	Accounts []domain.Account
	OnDelete func(string)
}

// ImportHistoryList renders the import-history card: every recorded document,
// newest first, with per-row delete buttons.
func ImportHistoryList(props importHistoryListProps) ui.Node {
	docs := props.Docs
	accounts := props.Accounts

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("documents.historyTitle"),
		Body: IfElse(len(docs) == 0,
			P(css.Class("empty"), uistate.T("documents.historyEmpty")),
			Div(css.Class("rows"), MapKeyed(docs,
				func(d domain.Document) any { return d.ID },
				func(d domain.Document) ui.Node {
					name := ""
					if a, ok := domain.AccountByID(accounts, d.AccountID); ok {
						name = a.Name
					}
					return ui.CreateElement(DocHistoryRow, docHistoryRowProps{
						Doc:         d,
						AccountName: name,
						OnDelete:    props.OnDelete,
					})
				},
			)),
		),
	})
}
