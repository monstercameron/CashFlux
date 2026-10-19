// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type importControllerFixtureProps struct {
	capture func(*documentsController, state.Atom[*uistate.DialogRequest])
}

func importControllerFixture(p importControllerFixtureProps) ui.Node {
	dialog := uistate.UseDialog()
	uistate.CaptureDialog(dialog)
	c := useDocumentsController()
	p.capture(c, dialog)
	return c.renderPanel()
}

func mountImportController(t *testing.T) (*render.Fixture, func() *documentsController, func() *uistate.DialogRequest) {
	t.Helper()
	app := newR3EEditApp(t)
	settings := app.Settings()
	settings.OpenAIKey = "fake-test-key"
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	var current *documentsController
	var dialog state.Atom[*uistate.DialogRequest]
	f.Render(ui.CreateElement(importControllerFixture, importControllerFixtureProps{capture: func(c *documentsController, d state.Atom[*uistate.DialogRequest]) { current = c; dialog = d }}))
	f.Stabilize()
	current.model.Reset()
	current.syncDraft()
	f.Stabilize()
	return f, func() *documentsController { return current }, func() *uistate.DialogRequest { return dialog.Get() }
}

func TestImportStatusVisibleInEveryStageIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	for _, source := range []string{"", "csv", "stmt", "pdf", "receipt", "amazon"} {
		c := current()
		c.source.Set(source)
		c.msg.Set("visible failure")
		f.Stabilize()
		if findByTestID(f, "div", "import-status") == nil || !strings.Contains(f.Text(), "visible failure") {
			t.Fatalf("source %q lost status", source)
		}
	}
	c := current()
	c.appendRows([]extract.Row{{Date: "2026-03-01", Description: "Shop", Amount: "-2.00"}})
	f.Stabilize()
	if findByTestID(f, "div", "import-status") == nil || !strings.Contains(f.Text(), "visible failure") {
		t.Fatal("review lost status")
	}
}

func TestImportFooterRequiresVisibleCSVPreflightIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.chooseSource("csv")
	c.csvText.Set("date,desc,amount\n2026-03-01,Shop,-2.00")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if len(c.app.Transactions()) != 0 || findByTestID(f, "div", "csv-preflight") == nil {
		t.Fatal("footer bypassed preview")
	}
	if got := f.AllByTag("textarea")[0].Attr("data-cf-value"); !strings.Contains(got, "Shop") {
		t.Fatalf("paste hidden: %q", got)
	}
	c = current()
	c.chooseSource("")
	c.csvText.Set("date,desc,amount\n2026-03-01,Unseen,-2.00")
	f.Stabilize()
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if len(c.app.Transactions()) != 0 {
		t.Fatal("unseen CSV imported")
	}
}

func TestImportStaleCategoryReplyAndCancelIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.settings.OpenAIKey = "fake-key"
	c.job.consented = true
	c.appendRows([]extract.Row{{ID: "a", Date: "2026-03-01", Description: "Shop", Amount: "-2.00"}})
	f.Stabilize()
	c = current()
	c.settings.OpenAIKey = "fake-key"
	c.job.consented = true
	var reply func(string, ai.Usage)
	cancelled := false
	applied := false
	c.startAI(func(result func(string, ai.Usage), _ func(string)) func() {
		reply = result
		return func() { cancelled = true }
	}, func(string) { applied = true })
	c.model.Remove("a")
	reply(`{"items":[{"i":0,"category":"Groceries"}]}`, ai.Usage{TotalTokens: 3})
	if c.app.AISpendThisMonth().Tokens != 3 {
		t.Fatal("completed call missing from spend meter")
	}
	if applied || len(c.model.Rows) != 0 {
		t.Fatal("stale reply changed removed row")
	}
	c.cancelAI()
	if !cancelled {
		t.Fatal("transport not cancelled")
	}
}

func TestImportSmartPlusRequiresConsentBeforeFakeSendIMB(t *testing.T) {
	f, current, dialog := mountImportController(t)
	c := current()
	c.settings.OpenAIKey = "fake-key"
	sends := 0
	send := func(result func(string, ai.Usage), _ func(string)) func() {
		sends++
		result("ok", ai.Usage{})
		return func() {}
	}
	c.startAI(send, func(string) {})
	if sends != 0 || dialog() == nil || !strings.Contains(dialog().Message, "OpenAI") {
		t.Fatal("send before consent or no disclosure")
	}
	dialog().OnResult(false, "")
	f.Stabilize()
	if sends != 0 {
		t.Fatal("declined consent sent data")
	}
	c = current()
	c.settings.OpenAIKey = "fake-key"
	c.startAI(send, func(string) {})
	dialog().OnResult(true, "")
	if sends != 1 {
		t.Fatalf("fake sends = %d", sends)
	}
}

func TestImportCheckpointCancelPreventsCommitIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	var finish func(error)
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { finish = done })
	defer restore()
	called := false
	c.checkpointRun("test checkpoint", func(string) { called = true })
	if finish == nil {
		t.Fatal("checkpoint not started")
	}
	CancelImportPanel()
	f.Stabilize()
	finish(nil)
	if called {
		t.Fatal("commit after cancel")
	}
}

func TestImportWaitsForDurableSaveAndRetryNeverReimportsIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.chooseSource("csv")
	c.buildCSVPreflight([]byte("date,desc,amount\n2026-03-01,Shop,-2.00"))
	f.Stabilize()
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { done(nil) })
	defer restore()
	var save func(appstate.DurableResult)
	c = current()
	if info := c.csvPreflightS.Get(); info == nil || info.Batch.Counts.New != 1 {
		t.Fatalf("fixture not writable: %+v", info)
	}
	c.app.SetDurableWriter(func(_ uint64, done func(appstate.DurableResult)) { save = done })
	c.commitCSVImport(nil)
	if save == nil || !c.busy() || strings.Contains(c.msg.Get(), "Import result:") {
		t.Fatal("acknowledged before durable completion")
	}
	revision := c.app.Rev()
	save(appstate.DurableResult{Revision: revision, Err: errors.New("disk full")})
	f.Stabilize()
	c = current()
	if c.job.pending == nil || c.csvPreflightS.Get() == nil {
		t.Fatal("failed durable save lost pending import")
	}
	c.job.pending()
	save(appstate.DurableResult{Revision: c.app.Rev()})
	f.Stabilize()
	if len(c.app.Transactions()) != 1 || !strings.Contains(current().msg.Get(), uistate.T("impc.importSummary", 1, 0, 0, 0)) {
		t.Fatalf("retry: txns=%d summary=%q busy=%v pending=%v", len(c.app.Transactions()), current().msg.Get(), current().busy(), current().job.pending != nil)
	}
}

func TestImportEditorFollowsIdentityAfterRowRemovalIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.appendRows([]extract.Row{
		{ID: "first", Date: "2026-03-01", Description: "First", Amount: "-1.00"},
		{ID: "editing", Date: "2026-03-02", Description: "Editing", Amount: "-2.00"},
		{ID: "last", Date: "2026-03-03", Description: "Last", Amount: "-3.00"},
	})
	f.Stabilize()
	var editButtons []*render.QueryNode
	for _, button := range f.AllByTag("button") {
		if button.Attr("aria-label") == uistate.T("documents.editRow") {
			editButtons = append(editButtons, button)
		}
	}
	if len(editButtons) != 3 {
		t.Fatal("missing row editors")
	}
	editButtons[1].Click()
	f.Stabilize()
	var description *render.QueryNode
	for _, input := range f.AllByTag("input") {
		if input.Attr("placeholder") == uistate.T("documents.descPlaceholder") {
			description = input
			break
		}
	}
	if description == nil {
		t.Fatal("editor not opened")
	}
	description.Input("Changed second row")
	current().removeDraft(0)
	f.Stabilize()
	for _, form := range f.AllByTag("form") {
		if form.Attr("class") == "draft-edit-grid" {
			form.Submit()
			break
		}
	}
	f.Stabilize()
	rows := current().model.Rows
	if len(rows) != 2 || rows[0].ID != "editing" || rows[0].Description != "Changed second row" || rows[1].Description != "Last" {
		t.Fatalf("editor moved identities: %+v", rows)
	}
}

func TestImportFailedRowsRemainEditableAndSourcesAppendIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.chooseSource("stmt")
	c.appendRows([]extract.Row{{ID: "failed", Date: "bad", Description: "Keep this", Amount: "-1.00"}})
	f.Stabilize()
	c = current()
	c.chooseSource("receipt")
	c.appendRows([]extract.Row{{Date: "2026-03-01", Description: "New receipt", Amount: "2.00"}})
	f.Stabilize()
	c = current()
	if len(c.model.Rows) != 2 || c.model.Rows[0].Description != "Keep this" {
		t.Fatal("adding a source replaced the draft")
	}
	c.finishImport(appstate.CSVImportResult{Outcomes: []appstate.ImportRowOutcome{{ID: "failed", Status: "failed", Reason: "Correct the date"}}}, false)
	f.Stabilize()
	if len(current().model.Rows) != 2 || !strings.Contains(f.Text(), "Correct the date") {
		t.Fatal("failed row or reason lost")
	}
}

func TestImportAccountChangeInvalidatesCSVPreviewIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	c.chooseSource("csv")
	c.buildCSVPreflight([]byte("date,desc,amount\n2026-03-01,Shop,-2.00"))
	f.Stabilize()
	if current().csvPreflightS.Get() == nil {
		t.Fatal("missing initial preview")
	}
	selects := f.AllByTag("select")
	if len(selects) == 0 {
		t.Fatal("missing account selector")
	}
	selects[0].Change("acct-2")
	f.Stabilize()
	if current().csvPreflightS.Get() != nil {
		t.Fatal("account change kept stale preview")
	}
	current().commitCSVImport(nil)
	if len(current().app.Transactions()) != 0 {
		t.Fatal("stale account preview committed")
	}
	f.AllByTag("form")[0].Submit()
	f.Stabilize()
	if info := current().csvPreflightS.Get(); info == nil || info.Staged.AccountID != "acct-2" {
		t.Fatal("new account was not previewed")
	}
}

func TestImportTimeoutAndSingleFlightUseFakeTransportIMB(t *testing.T) {
	_, current, _ := mountImportController(t)
	c := current()
	c.job.consented = true
	sends, cancels := 0, 0
	send := func(func(string, ai.Usage), func(string)) func() { sends++; return func() { cancels++ } }
	c.startAI(send, func(string) {})
	c.startAI(send, func(string) {})
	if sends != 1 {
		t.Fatal("double submit started multiple sends")
	}
	// Drive the timeout callback directly, with no clock delay or real transport.
	c.cancelAI()
	request := c.model.Begin()
	c.aiLoading.Set(true)
	c.job.cancel = func() { cancels++ }
	c.timeoutAI(request)
	if c.aiLoading.Get() || c.aiErr.Get() != uistate.T("imb.timeout") || cancels != 2 {
		t.Fatal("timeout did not cancel and report")
	}
	c.timeoutAI(request)
	if cancels != 2 {
		t.Fatal("stale timeout affected the next operation")
	}
}

func TestImportDisclosureMatchesAvailableTransportIMB(t *testing.T) {
	_, current, _ := mountImportController(t)
	c := current()
	c.useBackendAI = true
	c.pr.ServerURL = "https://example.invalid"
	c.stage.Set("input")
	c.source.Set("receipt")
	if !strings.Contains(c.disclosure(), c.pr.ServerURL) {
		t.Fatal("proxy destination missing")
	}
	for _, source := range []string{"stmt", "pdf"} {
		c.source.Set(source)
		if strings.Contains(c.disclosure(), c.pr.ServerURL) {
			t.Fatal("direct request disclosed as proxy")
		}
	}
	c.stage.Set("review")
	c.source.Set("receipt")
	if strings.Contains(c.disclosure(), c.pr.ServerURL) {
		t.Fatal("category request disclosed as proxy")
	}
}

func TestImportReceiptAttachHasCheckpointHistoryAndDurableCompletionIMB(t *testing.T) {
	f, current, _ := mountImportController(t)
	c := current()
	batch, err := c.app.StageCSV([]byte("date,desc,amount\n2026-03-01,Shop,-8.00"), "acct-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.app.Commit(batch, "", ""); err != nil {
		t.Fatal(err)
	}
	target := c.app.Transactions()[0]
	c.chooseSource("receipt")
	c.appendRows([]extract.Row{
		{Date: "2026-03-01", Description: "Purchase", Amount: "10.00", Category: "Groceries"},
		{Date: "2026-03-01", Description: "Coupon", Amount: "-2.00", Category: "Groceries"},
	})
	c.receiptMode.Set(true)
	c.receiptTotal.Set("8.00")
	c.receiptMerchant.Set("Shop")
	f.Stabilize()
	c = current()
	c.app.SetDurableWriter(func(rev uint64, done func(appstate.DurableResult)) { done(appstate.DurableResult{Revision: rev}) })
	restore := uistate.SetCheckpointCommitForTest(func(_, _ map[string]string, done func(error)) { done(nil) })
	defer restore()
	c.commitReceipt(target.ID)
	f.Stabilize()
	if txns := c.app.Transactions(); len(txns) != 1 || len(txns[0].Splits) == 0 || txns[0].Amount.Amount != -800 {
		t.Fatal(txns)
	}
	if docs := c.app.Documents(); len(docs) != 1 || docs[0].CheckpointID == "" || docs[0].RowsLinked {
		t.Fatal("missing or misleading attach history", docs)
	}
	if current().busy() || len(current().model.Rows) != 0 {
		t.Fatal("durable attach did not complete")
	}
}
