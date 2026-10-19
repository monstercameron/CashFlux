// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type csvImportContext struct {
	App           *appstate.App
	Accounts      []domain.Account
	ImportAccount ui.State[string]
	CSVText       ui.State[string]
	DuplicateWarn ui.State[string]
	PendingCSV    ui.State[[]byte]
	Preflight     ui.State[*csvPreflightInfo]
	Message       ui.State[string]
	ModalOpen     state.Atom[bool]
	Revision      state.Atom[int]
	CheckpointRun func(string, func(string))
}

// commitCSVImport checkpoints, imports, records, and summarizes one staged CSV file.
func (ctx csvImportContext) commitCSVImport(data []byte) {
	ctx.CheckpointRun(uistate.T("ckpt.beforeCSVImport"), func(cpID string) {
		var beforeMinor int64
		var balanceDecimals int
		hasBalance := false
		if account, ok := domain.AccountByID(ctx.App.Accounts(), ctx.ImportAccount.Get()); ok {
			balanceDecimals = currency.Decimals(account.Currency)
			if balance, err := ledger.Balance(account, ctx.App.Transactions()); err == nil {
				beforeMinor, hasBalance = balance.Amount, true
			}
		}

		documentID := id.New()
		result, err := ctx.App.ImportTransactionsCSV(data, ctx.ImportAccount.Get(), documentID)
		imported, skipped := result.Imported, result.Failed
		ctx.DuplicateWarn.Set("")
		ctx.PendingCSV.Set(nil)
		ctx.Preflight.Set(nil)
		if err != nil {
			friendly := strings.TrimPrefix(err.Error(), "store: ")
			ctx.Message.Set(uistate.T("documents.csvError", friendly))
			return
		}
		if imported > 0 || result.Duplicates > 0 || len(skipped) > 0 {
			recordDocumentAs(ctx.App, documentID, domain.DocCSV, ctx.ImportAccount.Get(), nil,
				imported, result.Duplicates+len(skipped), result.Duplicates, len(skipped), cpID)
		}
		summary := csvImportSummary(ctx.Accounts, ctx.ImportAccount.Get(), imported)
		if len(skipped) > 0 {
			summary += " " + uistate.T("documents.importedCsvSkipped", importCount(len(skipped), "row", "rows"))
			if detail := csvSkipDetail(skipped); detail != "" {
				summary += " " + detail
			}
		}
		if hasBalance && imported > 0 {
			if account, ok := domain.AccountByID(ctx.App.Accounts(), ctx.ImportAccount.Get()); ok {
				if after, err := ledger.Balance(account, ctx.App.Transactions()); err == nil {
					summary += " " + uistate.T("documents.balanceMoved",
						money.FormatMinor(beforeMinor, balanceDecimals), money.FormatMinor(after.Amount, balanceDecimals))
				}
			}
		}
		ctx.Message.Set(summary)
		if imported > 0 {
			postUndoStory(uistate.T("documents.importUndo", importCount(imported, "transaction", "transactions")))
		}
		ctx.CSVText.Set("")
		if ctx.ModalOpen.Get() {
			ctx.ModalOpen.Set(false)
		}
		ctx.Revision.Set(ctx.Revision.Get() + 1)
	})
}
