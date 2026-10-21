// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/state"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func importUXFixture(p importControllerFixtureProps) ui.Node {
	notice := uistate.UseNotice()
	if p.captureNotice != nil {
		p.captureNotice(notice)
	}
	dialog := uistate.UseDialog()
	uistate.CaptureDialog(dialog)
	c := useDocumentsController()
	p.capture(c, dialog)
	return c.renderFrame()
}

func mountImportUX(t *testing.T) (*render.Fixture, func() *documentsController, func() *uistate.DialogRequest) {
	return mountImportUXWithNotice(t, nil)
}

func mountImportUXWithNotice(t *testing.T, captureNotice func(state.Atom[uistate.Notice])) (*render.Fixture, func() *documentsController, func() *uistate.DialogRequest) {
	t.Helper()
	app := newR3EEditApp(t)
	settings := app.Settings()
	settings.OpenAIKey = "fake-import-ux-key"
	if err := app.PutSettings(settings); err != nil {
		t.Fatal(err)
	}
	for i := range 7 {
		if err := app.PutDocument(domain.Document{ID: fmt.Sprintf("ux-doc-%d", i), Kind: domain.DocCSV, UploadedAt: time.Date(2026, 3, i+1, 0, 0, 0, 0, time.UTC), Status: domain.DocImported}); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	var current *documentsController
	var dialog state.Atom[*uistate.DialogRequest]
	f.Render(uiw.FlipPanel(uiw.FlipPanelProps{
		Title: uistate.T("transactions.importBtn"), Width: "900px", Height: "660px", NoFooter: true, FlushBody: true,
		Back: ui.CreateElement(importUXFixture, importControllerFixtureProps{capture: func(c *documentsController, d state.Atom[*uistate.DialogRequest]) { current, dialog = c, d }, captureNotice: captureNotice}),
	}))
	f.Stabilize()
	current.model.Reset()
	current.syncDraft()
	f.Stabilize()
	return f, func() *documentsController { return current }, func() *uistate.DialogRequest { return dialog.Get() }
}

func TestImportUXSourceStructure(t *testing.T) {
	for _, source := range []string{"csv", "stmt", "pdf", "receipt", "amazon"} {
		t.Run(source, func(t *testing.T) {
			fakeImportFetch(t)
			f, current, _ := mountImportUX(t)
			history := findByTestID(f, "details", "import-history")
			if history == nil || history.Attr("open") != "" {
				t.Fatal("picker history must be collapsed")
			}
			count := 0
			for _, row := range f.AllByTag("div") {
				if row.Attr("data-testid") == "import-history-row" {
					count++
				}
			}
			if count != 5 {
				t.Fatalf("history rows = %d, want 5", count)
			}
			findByTestID(f, "button", "import-type-"+source).Click()
			f.Stabilize()
			if findByTestID(f, "details", "import-history") != nil || findByTestID(f, "a", "import-history-activity") != nil || strings.Contains(f.Text(), uistate.T("imb.ready")) {
				t.Fatal("source step contains history or an empty-draft note")
			}
			form := findByTestID(f, "div", "import-source-form")
			back := findByTestID(f, "button", "import-back-types")
			card := findByTestID(f, "div", "import-source-card")
			if form == nil || back == nil || card == nil || len(form.Children()) < 2 || form.Children()[0].NodeID() != back.NodeID() || form.Children()[1].NodeID() != card.NodeID() {
				t.Fatal("back link must precede the one source card")
			}
			if findByTestID(f, "button", "flip-save") != nil {
				t.Fatal("source footer must not show Done or an import submit")
			}
			if source == "pdf" || source == "stmt" || source == "receipt" {
				disclosure := findByTestID(f, "details", "import-ai-details")
				if disclosure == nil || disclosure.Attr("open") != "" || !strings.Contains(disclosure.Text(), current().aiModel) || !strings.Contains(disclosure.Text(), "OpenAI") {
					t.Fatal("AI details must be collapsed and name the model/destination")
				}
			}
			if source == "pdf" {
				run := findByTestID(f, "button", "statementimport-run")
				if !isDisabled(run) || !renderContainsNode(findByTestID(f, "div", "statementimport-dropzone"), findByTestID(f, "button", "statementimport-choose")) {
					t.Fatal("empty PDF must disable Read; chooser must be inside the file area")
				}
				c := current()
				c.pdfData.Set("data:application/pdf;base64,JVBERi0=")
				c.pdfBytes.Set(make([]byte, 1536))
				c.pdfName.Set("SCCU statement.pdf")
				f.Stabilize()
				if !strings.Contains(f.Text(), "SCCU statement.pdf") || !strings.Contains(f.Text(), "1.5 KB") || findByTestID(f, "button", "statementimport-run").Property("disabled") == true {
					t.Fatalf("selected PDF name/size or enabled Read is missing: %s", f.Text())
				}
			}
			back = findByTestID(f, "button", "import-back-types")
			back.Click()
			f.Stabilize()
			if findByTestID(f, "div", "import-type-picker") == nil {
				t.Fatal("back link did not return to import types")
			}
		})
	}
}

func TestImportUXReviewAutoShownAndFooter(t *testing.T) {
	calls := fakeImportFetch(t)
	f, current, dialog := mountImportUX(t)
	findByTestID(f, "button", "import-type-pdf").Click()
	f.Stabilize()
	current().pdfData.Set("data:application/pdf;base64,JVBERi0=")
	f.Stabilize()
	findByTestID(f, "button", "statementimport-run").Click()
	if dialog() == nil || calls.Length() != 0 {
		t.Fatal("consent must precede the fake extraction")
	}
	dialog().OnResult(true, "")
	f.Stabilize()
	run := findByTestID(f, "button", "statementimport-run")
	if !isDisabled(run) || run.Attr("aria-busy") != "true" ||
		findByTestID(f, "span", "statementimport-spinner") == nil ||
		!strings.Contains(run.Text(), uistate.T("statementimport.reading")) {
		t.Fatal("read action must be disabled and show an accessible spinner while processing")
	}
	deliverImportExtraction(t, calls, 0)
	f.Stabilize()
	// Fetch resolution and the review render use separate scheduler turns. Wait
	// for the visible result rather than assuming the transport's short drain
	// delay also completed the component transition on a busy test machine.
	for deadline := time.Now().Add(3 * time.Second); findByTestID(f, "div", "draft-review-page") == nil && time.Now().Before(deadline); {
		time.Sleep(5 * time.Millisecond)
		f.Stabilize()
	}
	if findByTestID(f, "div", "import-source-form") != nil || findByTestID(f, "div", "draft-review-page") == nil || !current().inReview {
		t.Fatal("successful read did not replace source with review")
	}
	order := []string{}
	var walk func(*render.QueryNode)
	walk = func(n *render.QueryNode) {
		if id := n.Attr("data-testid"); id == "import-account-placement" || id == "draft-review-page" {
			order = append(order, id)
		}
		for _, child := range n.Children() {
			walk(child)
		}
	}
	walk(findByTestID(f, "div", "import-panel"))
	if len(order) != 2 || order[0] != "import-account-placement" || order[1] != "draft-review-page" {
		t.Fatalf("account placement must precede review rows: %v", order)
	}
	if findByTestID(f, "button", "draft-import-all") != nil {
		t.Fatal("review has two commit actions")
	}
	save := findByTestID(f, "button", "flip-save")
	if save == nil || !isDisabled(save) || save.Text() != uistate.T("importux.importMany", 0) {
		t.Fatal("review footer must wait for an account selection")
	}
	smartImportAccountPicker(t, f).Change("acct-2")
	f.Stabilize()
	save = findByTestID(f, "button", "flip-save")
	if isDisabled(save) || save.Text() != uistate.T("importux.importMany", 3) || save.Attr("form") != ImportModalFormID {
		t.Fatal("review footer must name and submit the three ready transactions")
	}
	smartImportCommitAndCheck(t, f, current, 3)
}

func TestImportUXAddSourceNoteAndInvalidFooter(t *testing.T) {
	fakeImportFetch(t)
	f, current, _ := mountImportUX(t)
	current().chooseSource("stmt")
	current().appendRows([]extract.Row{{Date: "bad-date", Description: "Check me", Amount: "-1.00"}})
	f.Stabilize()
	if !isDisabled(findByTestID(f, "button", "flip-save")) {
		t.Fatal("invalid rows must disable the pinned import")
	}
	findByTestID(f, "button", "import-back-btn").Click()
	f.Stabilize()
	findByTestID(f, "button", "import-type-pdf").Click()
	f.Stabilize()
	if findByTestID(f, "p", "import-draft-note") == nil {
		t.Fatal("adding a source to existing rows needs the append note")
	}
}
