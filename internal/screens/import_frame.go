// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// ImportPanelFrame includes the step-aware pinned footer. The shell host must
// use NoFooter and FlushBody so it supplies neither a second footer nor scrolling.
func ImportPanelFrame(_ struct{}) ui.Node {
	if appstate.Default == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return useDocumentsController().renderFrame()
}

func (c *documentsController) renderFrame() ui.Node {
	c.pinnedFooter = true
	return Div(Style(map[string]string{"display": "flex", "flex-direction": "column", "height": "100%", "min-height": "0"}), Attr("data-testid", "import-frame"),
		Div(Style(map[string]string{"padding": "16px 24px", "border-bottom": "1px solid var(--border)"}), importProgress(c.progressStep())),
		Div(Style(map[string]string{"flex": "1 1 auto", "min-height": "0", "overflow-y": "auto", "padding": "24px"}), c.renderPanel()),
		ui.CreateElement(importModalFooter, c),
	)
}

func importModalFooter(c *documentsController) ui.Node {
	cancel := ui.UseEvent(func() {
		if !c.busy() {
			CancelImportPanel()
			c.importModalOpen.Set(false)
		}
	})
	count := c.summary.Importable
	disabled := importdraft.StatementDisabled(c.summary, len(c.model.Errors), c.busy() || c.aiLoading.Get())
	if c.receiptMode.Get() {
		count = 1
		disabled = importdraft.ReconcileReceipt(c.rows, c.receiptTotal.Get(), c.recBaseCur, len(c.model.Errors), c.busy() || c.aiLoading.Get()).Disabled
	}
	key := "importux.importMany"
	if count == 1 {
		key = "importux.importOne"
	}
	label := uistate.T(key, count)
	if c.busy() {
		label = uistate.T("enc34.saving")
	}
	return Div(css.Class("set-foot"), Attr("data-testid", "import-footer"), Style(map[string]string{"justify-content": "space-between"}),
		Button(css.Class("btn btn-ghost"), Type("button"), Attr("data-testid", "import-cancel"), Disabled(c.busy()), OnClick(cancel), uistate.T("action.cancel")),
		If(c.inReview, Button(css.Class("btn btn-primary"), Type("submit"), Attr("form", ImportModalFormID), Attr("data-testid", "flip-save"), Disabled(disabled), label)),
	)
}
