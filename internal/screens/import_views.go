// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/importmap"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func (c *documentsController) renderPanel() ui.Node {
	return Div(
		Form(Attr("id", ImportModalFormID), OnSubmit(c.saveModalImport)),
		ui.CreateElement(importStatus, c),
		If(!c.inReview, ui.CreateElement(importInputStage, c)),
		If(c.inReview, ui.CreateElement(importReviewStage, c)),
	)
}

func importInputStage(c *documentsController) ui.Node {
	return Fragment(
		If(len(c.rows) > 0, Div(css.Class(tw.Mb2, tw.Flex, tw.JustifyEnd), Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "import-review-btn"), OnClick(c.goReview), uistate.T("documents.reviewStage")))),
		IfElse(c.source.Get() == "", ui.CreateElement(importTypePicker, importTypePickerProps{OnCSV: c.onPickCSV, OnStatement: c.onPickStmt, OnOrder: c.onPickAmazon, OnPDF: c.onPickPDF, OnReceipt: c.onPickReceipt}), ui.CreateElement(importSourceForm, c)),
		c.renderHistory(),
	)
}

func importSourceForm(c *documentsController) ui.Node {
	return Div(css.Class("doc-form"), Attr("data-testid", "import-source-form"),
		Div(css.Class("doc-form-head"), Button(css.Class("btn"), Type("button"), Attr("data-testid", "import-back-types"), OnClick(c.backToTypes), uistate.T("documents.backToTypes"))),
		If(c.source.Get() == "csv", ui.CreateElement(importCSVSource, c)),
		If(c.source.Get() == "stmt", ui.CreateElement(importStatementSource, c)),
		If(c.source.Get() == "amazon", ui.CreateElement(importOrderSource, c)),
		If(c.source.Get() == "pdf", ui.CreateElement(importPDFSource, c)),
		If(c.source.Get() == "receipt", ui.CreateElement(importReceiptSource, c)),
		If(c.source.Get() == "stmt", ui.CreateElement(importStatementOptions, c)),
	)
}

func importCSVSource(c *documentsController) ui.Node {
	return Fragment(
		importFormHeader(false, uiw.Icon(icon.Upload, css.Class(tw.W5, tw.H5)),
			uistate.T("documents.typeCsvTitle"), uistate.T("documents.csvFormDesc")),
		CsvImportCard(csvImportCardProps{
			Accounts:     c.accounts,
			ImportAcctID: c.importAcct.Get(),
			CSVText:      c.csvText.Get(),

			OnChooseFile: c.chooseCsvFile,
			OnAcctChange: c.onAcct,
			OnCsvInput:   c.onCsv,
			OnImportCSV:  c.importCSV,

			Preflight: csvPreflightCard(c.csvPreflightS.Get(), c.confirmCSV, c.cancelCSVPreflight),
		}),
	)
}

func importStatementSource(c *documentsController) ui.Node {
	return Fragment(
		importFormHeader(false, uiw.Icon(icon.Landmark, css.Class(tw.W5, tw.H5)),
			uistate.T("documents.typeStmtTitle"), uistate.T("documents.stmtFormDesc")),
		Div(css.Class("doc-form-body"),
			Form(OnSubmit(c.parseStatement),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2), Style(map[string]string{"margin-bottom": "0.5rem"}),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.stmtParse")),
					Button(css.Class("btn btn-plus", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "extract-ai-btn"),
						OnClick(c.extractWithAI), Disabled(c.aiLoading.Get() || c.busy()),
						uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("documents.extractAI")),
						// R24: the control says it needs a key BEFORE it is pressed. The
						// plain "Parse" button beside it is the no-key sibling and works
						// regardless, which is what makes this a label rather than a wall.
						KeyGateMark(c.canSend("stmt"))),
				),
				Textarea(css.Class("field field-wide"), Attr("rows", "4"),
					Placeholder(uistate.T("documents.stmtSample")),
					OnInput(c.onStmt), Value(c.stmtText.Get()), uiw.FieldValue(c.stmtText.Get()),
				),
			),
			c.renderDisclosure(),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				P(css.Class("muted", tw.Text12), uistate.T("documents.cadenceDesc")),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "cadence-reminder-btn"),
					OnClick(c.createCadenceReminder), uistate.T("documents.cadenceBtn")),
				If(c.cadenceMsg.Get() != "",
					Span(css.Class("text-up", tw.Text12), Attr("role", "status"),
						Attr("data-testid", "cadence-reminder-msg"), c.cadenceMsg.Get())),
			),
		),
	)
}

func importOrderSource(c *documentsController) ui.Node {
	return Fragment(
		importFormHeader(false, uiw.Icon(icon.Box, css.Class(tw.W5, tw.H5)),
			uistate.T("orderimport.cardTitle"), uistate.T("orderimport.cardDesc")),
		ui.CreateElement(OrderImportCard, orderImportCardProps{}),
	)
}

func importReceiptSource(c *documentsController) ui.Node {
	return Fragment(
		importFormHeader(true, uiw.Icon(icon.Receipt, css.Class(tw.W5, tw.H5)),
			uistate.T("documents.typeReceiptTitle"), uistate.T("documents.receiptFormDesc")),
		ImageImportCard(imageImportCardProps{
			ImageURL:  c.imageURL.Get(),
			AILoading: c.aiLoading.Get(),

			OnChoose:   c.chooseImage,
			OnReadAI:   c.readAI,
			Disclosure: c.disclosure(),
		}),
	)
}

func importPDFSource(c *documentsController) ui.Node {
	return Fragment(c.pdfCard, c.renderDisclosure())
}

func importStatementOptions(c *documentsController) ui.Node {
	return Fragment(
		If(c.wizardVisible.Get(), Fragment(
			wizardCard(
				c.wizardHeader.Get(),
				c.wizardDate.Get(), c.wizardDescCol.Get(), c.wizardAmount.Get(),
				c.wizardDebit.Get(), c.wizardCredit.Get(),
				c.onWizardDate, c.onWizardDesc, c.onWizardAmount, c.onWizardDebit, c.onWizardCredit,
				c.applyWizard,
				c.profileName.Get(), c.onProfileName, c.saveProfile,
			),
		)),
		If(c.wizardVisible.Get() || c.statementFormatOptionsVisible.Get(), ui.CreateElement(importFormatOptions, c)),
		If(c.wizardVisible.Get() || c.showProfiles.Get(), ui.CreateElement(importSavedProfiles, c)),
	)
}
func importFormatOptions(c *documentsController) ui.Node {
	return statementFormatControls(c.dateOrderChoice.Get(), c.numberFormatChoice.Get(), c.statementAmbiguousDates.Get(), c.statementAmbiguousNumbers.Get(), c.statementInvalidRows.Get(), c.onDateOrderChoice, c.onNumberFormatChoice, c.applyStatementFormats)
}
func importSavedProfiles(c *documentsController) ui.Node {
	return savedProfilesCard(c.app, c.accounts, c.importAcct.Get(), c.showProfiles.Get(), func() { c.showProfiles.Set(!c.showProfiles.Get()) }, c.applySavedProfile, func(id string) {
		if err := c.app.DeleteImportProfile(id, activeImportProfileWorkspaceID()); err != nil {
			c.msg.Set(uistate.T("imb.profileDeleteFailed"))
			return
		}
		c.rev.Set(c.rev.Get() + 1)
	})
}

func (c *documentsController) applySavedProfile(sp importmap.SavedProfile) {
	if c.busy() || c.aiLoading.Get() {
		return
	}
	c.cancelAI()

	c.dateOrderChoice.Set(string(sp.Profile.DateOrder))
	// Apply a saved profile to the current raw rows.
	flat := importdraft.FlattenRecords(c.wizardRawRows.Get())
	if len(flat) == 0 {
		c.msg.Set(uistate.T("tc4n6.profilePasteFirst"))
		return
	}
	dec := currency.Decimals(reviewCurrencyFor(c.app, c.accounts, c.importAcct.Get()))
	sp.Profile.Decimals = dec
	rows2, rowErrors, rowWarnings := importdraft.ApplyProfile(sp.Profile, c.wizardRawRows.Get())
	if len(rows2) == 0 {
		c.msg.Set(uistate.T("tc4n6.profileNoRows"))
		return
	}
	c.wizardVisible.Set(false)
	c.setDraft(rows2)
	c.msg.Set(importdraft.ParsedMessage(len(rows2), rowErrors, rowWarnings, uistate.T))
	c.rev.Set(c.rev.Get() + 1)

}

func importReviewStage(c *documentsController) ui.Node {
	return Fragment(
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb2),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "import-back-btn"),
				OnClick(c.goInput), uistate.T("documents.backToInput")),
			A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/artifacts")),
				Attr("data-testid", "import-artifacts-link"), OnClick(c.goArtifacts),
				uistate.T("documents.pastImportsLink"))),
		// C74 — Suggest categories button: shown when draft is non-empty.
		// Pass 1 (deterministic rules) runs free; Pass 2 (AI) only fires with a key.
		If(len(c.rows) > 0,
			Div(css.Class(tw.Mt2, tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "suggest-categories-btn"),
					OnClick(c.categorizeDraft), Disabled(c.aiLoading.Get() || c.busy()), uistate.T("imb.suggestLocal")),
				Button(css.Class("btn btn-plus"), Type("button"), Attr("data-testid", "suggest-categories-ai"), OnClick(c.categorizeAI), Disabled(c.aiLoading.Get() || c.busy()), uistate.T("imb.suggestAI"), KeyGateMark(c.canSend("categories"))),
				c.renderDisclosure(),
			),
		),
		If(c.source.Get() == "stmt" && c.statementRaw.Get() != "", ui.CreateElement(importFormatOptions, c)),
		// Review results sit below the import inputs that produce them (G14 §1): a
		// parsed statement / scanned receipt's draft rows no longer pop in *above* the
		// card the user just acted in.
		ui.CreateElement(paginatedDraftReview, draftReviewProps{
			Rows:              c.rows,
			VisibleRows:       c.visibleRows,
			Start:             c.reviewStart,
			Page:              c.currentReviewPage,
			PageCount:         c.reviewPageCount,
			Accounts:          c.accounts,
			Categories:        c.app.Categories(),
			ReviewCur:         c.reviewCur,
			ImportAcctID:      c.importAcct.Get(),
			SeenSigs:          c.seenSigs,
			ReceiptMode:       c.receiptMode.Get(),
			ReceiptTotal:      c.receiptTotal.Get(),
			ReceiptMerchant:   c.receiptMerchant.Get(),
			RecBaseCur:        c.recBaseCur,
			ClearDraft:        c.clearDraft,
			Toggle:            c.receiptToggle,
			OnPage:            func(page int) { c.reviewPage.Set(page) },
			OnAcctChange:      c.onAcct,
			OnReceiptTotal:    c.onReceiptTotal,
			OnReceiptMerchant: c.onReceiptMerchant,
			OnImportDraft:     c.importDraft,
			OnImportReceipt:   c.importReceipt,
			SavingCheckpoint:  c.busy() || c.aiLoading.Get(),
			Summary:           &c.summary, Errors: c.model.Errors,
			OnRemoveDraft: c.removeDraft,
			OnUpdateDraft: c.updateDraft,
		}),
		// #58: when the receipt's total matches an existing charge, lead with
		// the attach-to-existing offer — "Import" below stays the create-new path.
		If(c.receiptMode.Get() && len(c.receiptCands) > 0,
			receiptMatchOffer(c.receiptCands, c.attachReceipt)),
		// #57: pre-commit balance impact for the reviewed draft — where the
		// import account's balance lands if every row commits, with the
		// implausible-jump warning.
		c.renderImpact(),
		ui.CreateElement(SpendSummaryCard, spendSummaryCardProps{
			Rows:         c.rows,
			Accounts:     c.accounts,
			ImportAcctID: c.importAcct.Get(),
			BaseCurrency: c.app.Settings().BaseCurrency,
		}),
	)
}
