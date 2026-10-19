// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func importStatus(c *documentsController) ui.Node {
	cancel := ui.UseEvent(func() { c.cancelAI(); c.msg.Set(uistate.T("imb.cancelled")) })
	retry := ui.UseEvent(func() {
		if c.job.pending != nil {
			c.job.pending()
		}
	})
	settings := ui.UseEvent(func() { uistate.OpenGlobalSettingsAt("ai") })
	text := importdraft.StatusText(c.msg.Get(), c.aiErr.Get(), c.aiLoading.Get(), c.busy(), c.job.pending != nil, uistate.T)
	return Div(Attr("data-testid", "import-status"), Attr("role", "status"), Attr("aria-live", "polite"),
		P(css.Class("muted"), text),
		If(c.needsKey.Get(), Div(css.Class("notice notice-warn"), Span(uistate.T("documents.needKey")), Button(css.Class("btn btn-sm"), Type("button"), OnClick(settings), uistate.T("documents.goToSettings")))),
		If(c.aiLoading.Get(), Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "import-cancel-ai"), OnClick(cancel), uistate.T("imb.cancelAI"))),
		If(c.job.pending != nil, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "import-retry-save"), OnClick(retry), uistate.T("imb.retrySave"))),
	)
}

func (c *documentsController) renderDisclosure() ui.Node {
	return P(css.Class("muted"), Attr("data-testid", "import-ai-disclosure"), c.disclosure())
}

func (c *documentsController) renderHistory() ui.Node {
	docs := ui.UseMemo(c.app.Documents, c.app, c.app.Rev())
	txns := ui.UseMemo(c.app.Transactions, c.app, c.app.Rev())
	return importHistorySection(docs, c.accounts, txns, c.rollbackImport, c.goActivity)
}

func (c *documentsController) renderImpact() ui.Node {
	return ui.UseMemo(func() ui.Node {
		if c.receiptMode.Get() || c.summary.Overflow {
			return Fragment()
		}
		preview := importdraft.BatchPreview(c.batch.Batch, c.accounts, c.app.Transactions(), c.importAcct.Get())
		if !preview.HasBalance {
			return Fragment()
		}
		account := preview.Account
		return Div(css.Class("notice"), Attr("data-testid", "review-balance-impact"),
			P(uistate.T("documents.preflightBalance", account.Name, fmtMoney(money.New(preview.Before, account.Currency)), fmtMoney(money.New(preview.After, account.Currency)), fmtMoney(money.New(preview.Net, account.Currency)))),
			If(preview.Jump, Div(css.Class("notice notice-warn"), Attr("role", "alert"), Attr("data-testid", "review-jump-warn"), uistate.T("documents.preflightJumpWarn"))),
		)
	}, c.app, c.app.Rev(), c.model.Revision, len(c.batch.Rows), c.batch.Rows, c.importAcct.Get(), c.receiptMode.Get())
}

func (c *documentsController) savePDFCopy() {
	if !c.pdfSaveCopy.Get() || len(c.pdfBytes.Get()) == 0 {
		return
	}
	pdf := domain.Artifact{ID: id.New(), Name: firstNonEmpty(c.pdfName.Get(), "statement.pdf"), Kind: artifacts.KindPDF, MIME: "application/pdf", Bytes: c.pdfBytes.Get(), CreatedAt: time.Now()}
	pdf.Size = artifacts.Size(pdf)
	go func() {
		stored, err := c.app.StoreBlobForArtifact(pdf)
		if err == nil {
			err = c.app.PutArtifact(stored)
		}
		if err != nil {
			uistate.PostNotice(uistate.T("imb.artifactFailed"), true)
			return
		}
		c.app.RefreshBlobUsage()
		uistate.RequestPersist()
	}()
}
