// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

func (c *documentsController) busy() bool { return c.checkpointSaving.Get() }

func (c *documentsController) syncDraft() {
	c.draftMetadata.Set(c.model.Metadata())
	setDocumentsDraft(c.draft, c.stage, c.reviewPage, c.model.Rows)
	c.model.Stage = c.stage.Get()
}

func (c *documentsController) appendRows(rows []extract.Row) {
	if c.busy() {
		return
	}
	if len(rows)+len(c.model.Rows) > maxImportRows {
		c.msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
		return
	}
	c.cancelAI()
	c.model.Append(c.source.Get(), rows)
	c.syncDraft()
}

func (c *documentsController) confirmReplace(rows []extract.Row) {
	if c.busy() {
		return
	}
	request := c.model.Begin()
	uistate.ConfirmModal(uistate.T("imb.replaceConfirm"), true, func(ok bool) {
		if !c.model.Accept(request) || !ok || c.busy() {
			return
		}
		c.model.Reset()
		c.appendRows(rows)
	})
}

func (c *documentsController) chooseSource(source string) {
	if c.busy() {
		return
	}
	c.cancelAI()
	c.source.Set(source)
}

func (c *documentsController) invalidateCSV() {
	c.pendingCSV.Set(nil)
	c.csvPreflightS.Set(nil)
}

func (c *documentsController) documentKind() domain.DocumentKind {
	return domain.DocumentKind(c.model.Kind())
}

func (c *documentsController) commitStaged(batch appstate.StagedImport, csv bool) {
	if c.busy() || c.aiLoading.Get() || len(batch.Rows) == 0 {
		return
	}
	c.checkpointRun(uistate.T("ckpt.beforeDocImport", importCount(len(batch.Rows), "row", "rows")), func(cpID string) {
		c.app.CommitDurable(batch, id.New(), cpID, func(result appstate.CSVImportResult, err error) {
			if c.job.closed {
				return
			}
			if err != nil {
				if result.DocumentID != "" {
					c.awaitImport(result, csv)
					c.msg.Set(uistate.T("imb.saveFailed"))
					return
				}
				c.checkpointSaving.Set(false)
				if !csv {
					outcomes := make([]importdraft.Outcome, 0, len(result.Outcomes))
					for _, o := range result.Outcomes {
						outcomes = append(outcomes, importdraft.Outcome{ID: o.ID, Status: o.Status, Reason: o.Reason})
					}
					c.model.RetainFailures(outcomes)
					c.syncDraft()
				}
				c.msg.Set(uistate.T("imb.commitFailed"))
				return
			}
			c.checkpointSaving.Set(false)
			c.finishImport(result, csv)
		})
	})
}

func (c *documentsController) awaitImport(result appstate.CSVImportResult, csv bool) {
	// Retain the exact SQL outcome while retrying persistence: Retry must never
	// execute Commit again or turn an unsaved write into a second import.
	c.job.pending = func() {
		c.app.RequestTransactionsDurable(importedOutcomeIDs(result.Outcomes), func(saved appstate.DurableResult) {
			if c.job.closed {
				return
			}
			if saved.Err != nil {
				c.msg.Set(uistate.T("imb.saveFailed"))
				return
			}
			c.job.pending = nil
			c.checkpointSaving.Set(false)
			c.finishImport(result, csv)
		})
	}
}

func (c *documentsController) finishImport(result appstate.CSVImportResult, csv bool) {
	if csv {
		if len(result.Failed) == 0 {
			c.csvText.Set("")
		}
		c.invalidateCSV()
		if len(result.Failed) > 0 {
			c.keepCSVFailures(result)
		}
	} else {
		outcomes := make([]importdraft.Outcome, 0, len(result.Outcomes))
		for _, outcome := range result.Outcomes {
			outcomes = append(outcomes, importdraft.Outcome{ID: outcome.ID, Status: outcome.Status, Reason: outcome.Reason})
		}
		c.model.RetainFailures(outcomes)
		c.syncDraft()
	}
	c.aiErr.Set("")
	c.msg.Set(uistate.T("impc.importSummary", result.Imported, result.Duplicates, len(result.Failed), len(result.PossibleDuplicates)))
	if result.Imported > 0 {
		uistate.PostDurable(uistate.T("documents.importUndo", importCount(result.Imported, "transaction", "transactions")), true)
	}
	c.savePDFCopy()
	c.rev.Set(c.rev.Get() + 1)
}

func (c *documentsController) keepCSVFailures(result appstate.CSVImportResult) {
	outcomes := make([]importdraft.Outcome, 0, len(result.Outcomes))
	lines := map[string]int{}
	for _, o := range result.Outcomes {
		outcomes = append(outcomes, importdraft.Outcome{ID: o.ID, Status: o.Status, Reason: o.Reason})
		lines[o.ID] = o.Line
	}
	rows := importdraft.CSVFailureRows(c.csvText.Get(), outcomes, lines)
	c.model.Append("csv", rows)
	for _, o := range outcomes {
		if o.Status == "failed" {
			c.model.Errors[o.ID] = o.Reason
		}
	}
	c.syncDraft()
}

func (c *documentsController) commitReceipt(txnID string) {
	if len(c.model.Errors) > 0 {
		c.msg.Set(uistate.T("imb.fixRows"))
		return
	}
	if c.busy() || c.aiLoading.Get() || c.stage.Get() != "review" || len(c.model.Rows) == 0 {
		return
	}
	receipt := importdraft.Receipt(c.model.Rows, c.receiptMerchant.Get(), c.receiptTotal.Get())
	if residual, err := receipt.Residual(c.recDec); err != nil || residual != 0 {
		c.msg.Set(uistate.T("documents.linesUnreadable"))
		return
	}
	accountID := c.importAcct.Get()
	rows := append([]extract.Row(nil), c.model.Rows...)
	c.checkpointRun(uistate.T("ckpt.beforeReceiptImport"), func(cpID string) {
		var tx domain.Transaction
		var err error
		if txnID == "" {
			tx, err = c.app.ImportReceipt(receipt, accountID, importdraft.ReceiptDate(rows, time.Now()))
		} else {
			tx, err = c.app.AttachReceiptToTransaction(txnID, receipt)
		}
		if err != nil {
			c.checkpointSaving.Set(false)
			c.msg.Set(uistate.T("imb.receiptFailed"))
			return
		}
		if err = recordDocumentAs(c.app, id.New(), domain.DocImage, accountID, rows, 1, 0, 0, 0, cpID); err != nil {
			c.job.receiptWarning = uistate.T("imb.historyFailed")
			c.msg.Set(c.job.receiptWarning)
		}
		learnReceiptChoices(receipt.Merchant, tx.Splits)
		c.awaitReceipt(tx, txnID != "")
	})
}

func (c *documentsController) awaitReceipt(tx domain.Transaction, attached bool) {
	c.job.pending = func() {
		c.app.RequestTransactionsDurable([]string{tx.ID}, func(saved appstate.DurableResult) {
			if c.job.closed {
				return
			}
			if saved.Err != nil {
				c.msg.Set(uistate.T("imb.saveFailed"))
				return
			}
			c.job.pending = nil
			c.checkpointSaving.Set(false)
			c.model.Reset()
			c.syncDraft()
			c.imageURL.Set("")
			c.imageDraftAtom.Set("")
			c.receiptTotal.Set("")
			c.receiptMerchant.Set("")
			c.receiptMode.Set(false)
			summary := uistate.T("tc4n6.receiptImported", importCount(len(tx.Splits), "category", "categories"))
			if attached {
				summary = uistate.T("documents.receiptAttached", tx.Desc, importCount(len(tx.Splits), "category", "categories"))
			}
			summary += " " + c.job.receiptWarning
			c.msg.Set(summary)
			uistate.PostDurable(summary, true)
			c.rev.Set(c.rev.Get() + 1)
		})
	}
	c.job.pending()
}

func importFriendlyError(message string) string { return importdraft.FriendlyError(message, uistate.T) }

func importedOutcomeIDs(outcomes []appstate.ImportRowOutcome) []string {
	return importdraft.ImportedTransactionIDs(outcomes, func(outcome appstate.ImportRowOutcome) (string, string) { return outcome.Status, outcome.TransactionID })
}
