// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// TestAccountDocsDrawerOpensToTheDocumentRowsAndAttachForm checks the documents disclosure: closed it
// renders only its toggle, and opening it shows each filed document as a row with Open and Remove,
// plus the attach form.
func TestAccountDocsDrawerOpensToTheDocumentRowsAndAttachForm(t *testing.T) {
	acct := checkingFixture("checking", "Checking", 10000)
	acct.DocRefs = []domain.AccountDocRef{{ArtifactID: "art-1", Label: "March statement", AttachedAt: time.Now()}}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(accountDocsDrawer, accountDocsDrawerProps{Account: acct}))
	f.Stabilize()

	toggle := byTestID(f, "button", "acct-docs-toggle-checking")
	if toggle == nil || toggle.Attr("aria-expanded") != "false" {
		t.Fatalf("documents toggle missing or open at first: %v", toggle)
	}
	if !strings.Contains(toggle.Text(), "(1)") {
		t.Fatalf("toggle does not state the document count: %q", toggle.Text())
	}
	if byTestID(f, "div", "acct-docs-drawer-checking") != nil {
		t.Fatal("closed drawer rendered its body")
	}

	toggle.Click()
	f.Stabilize()
	if toggle = byTestID(f, "button", "acct-docs-toggle-checking"); toggle.Attr("aria-expanded") != "true" {
		t.Fatal("toggle did not report the drawer as open")
	}
	if byTestID(f, "div", "acct-doc-row-art-1") == nil {
		t.Fatalf("open drawer lacks the document row: %s", f.Text())
	}
	if byTestID(f, "button", "acct-doc-remove-art-1") == nil {
		t.Fatal("document row lacks its Remove action")
	}
	if byTestID(f, "button", "acct-doc-open-art-1") != nil {
		t.Fatal("document whose file is missing offers Open")
	}
	if byTestID(f, "input", "acct-doc-label-checking") == nil || byTestID(f, "button", "acct-doc-attach-btn-checking") == nil {
		t.Fatal("open drawer lacks the attach form")
	}
}

// TestAccountDocsDrawerWithoutDocumentsShowsTheEmptyNotice checks the empty drawer: a notice in place
// of the list, with the attach form still reachable.
func TestAccountDocsDrawerWithoutDocumentsShowsTheEmptyNotice(t *testing.T) {
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(accountDocsDrawer, accountDocsDrawerProps{Account: checkingFixture("checking", "Checking", 0)}))
	f.Stabilize()
	byTestID(f, "button", "acct-docs-toggle-checking").Click()
	f.Stabilize()
	if calloutCount(f) == 0 {
		t.Fatalf("empty drawer has no notice: %s", f.Text())
	}
	if byTestID(f, "button", "acct-doc-attach-btn-checking") == nil {
		t.Fatal("empty drawer lacks the attach form")
	}
}

// TestInstitutionsManagerListsInstitutionsAndOpensTheEditForm checks the directory: each institution
// is a row with its Edit action, and Edit swaps in the edit form for that institution.
func TestInstitutionsManagerListsInstitutionsAndOpensTheEditForm(t *testing.T) {
	app := primaryAccountsGoalsApp(t)
	if err := app.PutInstitution(domain.Institution{ID: "inst-1", Name: "First Bank", SupportPhone: "555-0100"}); err != nil {
		t.Fatal(err)
	}
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(InstitutionsManagerForm, InstitutionsManagerFormProps{}))
	f.Stabilize()

	row := byTestID(f, "div", "institution-row-inst-1")
	if row == nil || !strings.Contains(row.Text(), "First Bank") || !strings.Contains(row.Text(), "555-0100") {
		t.Fatalf("institution row missing or incomplete: %v", row)
	}
	edit := byTestID(f, "button", "institution-edit-inst-1")
	if edit == nil || !strings.Contains(edit.Attr("aria-label"), "First Bank") {
		t.Fatalf("edit action missing or unnamed: %v", edit)
	}
	edit.Click()
	f.Stabilize()
	if byTestID(f, "form", "institution-edit-form") == nil {
		t.Fatalf("Edit did not open the edit form: %s", f.Text())
	}
	if byTestID(f, "input", "institution-name") == nil {
		t.Fatal("edit form lacks the name field")
	}
	if byTestID(f, "button", "institution-delete") == nil {
		t.Fatal("edit form for an existing institution lacks Delete")
	}
}

// TestInstitutionsManagerWithNoInstitutionsShowsTheEmptyNotice checks the empty directory.
func TestInstitutionsManagerWithNoInstitutionsShowsTheEmptyNotice(t *testing.T) {
	primaryAccountsGoalsApp(t)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(InstitutionsManagerForm, InstitutionsManagerFormProps{}))
	f.Stabilize()
	if byTestID(f, "button", "institution-add") == nil {
		t.Fatal("empty directory lacks Add")
	}
	if calloutCount(f) < 2 {
		t.Fatalf("empty directory lacks its hint and empty notices: %s", f.Text())
	}
}

// calloutCount counts the Callout notices in the rendered page.
func calloutCount(f *render.Fixture) int {
	n := 0
	for _, d := range f.AllByTag("div") {
		if strings.Contains(" "+d.Attr("class")+" ", " callout ") {
			n++
		}
	}
	return n
}
