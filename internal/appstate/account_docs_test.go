// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// pngTestBytes and jpegTestBytes start with the magic bytes of their formats, which is all the document check reads.
var (
	pngTestBytes  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpegTestBytes = []byte("\xff\xd8\xff\xe0\x00\x10JFIF")
)

func docTestAccount(t *testing.T, app *App) domain.Account {
	t.Helper()
	ac := domain.Account{ID: "docs-acct", Name: "Car", Currency: "USD", Class: domain.ClassAsset, Type: domain.TypeOther, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared}
	if err := app.PutAccount(ac); err != nil {
		t.Fatalf("put account: %v", err)
	}
	return ac
}

func TestAttachAccountDocument(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	ac := docTestAccount(t, app)
	in := AccountDocumentInput{Name: "policy.pdf", MIME: "application/pdf", Data: []byte("pdf"), Label: " Insurance ", ExpiryISO: "2026-04-01"}
	if err := app.AttachAccountDocument(ac.ID, in); err != nil {
		t.Fatalf("attach: %v", err)
	}
	var got domain.Account
	for _, a := range app.Accounts() {
		if a.ID == ac.ID {
			got = a
		}
	}
	if len(got.DocRefs) != 1 || got.DocRefs[0].Label != "Insurance" || got.DocRefs[0].ExpiresAt.IsZero() {
		t.Fatalf("doc refs = %+v", got.DocRefs)
	}
	if len(app.Artifacts()) != 1 || app.Artifacts()[0].Kind != "document" {
		t.Fatalf("artifacts = %+v", app.Artifacts())
	}
	if len(app.Tasks()) != 1 {
		t.Fatalf("a dated attach inside the lead window should schedule one reminder, got %d tasks", len(app.Tasks()))
	}
}

func TestAttachAccountDocumentMissingAccountLeavesNothing(t *testing.T) {
	app := newApp(t, false)
	err := app.AttachAccountDocument("nope", AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: []byte("x")})
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
	if n := len(app.Artifacts()); n != 0 {
		t.Fatalf("%d orphaned artifacts", n)
	}
}

func TestAttachAccountDocumentRejectsBadExpiry(t *testing.T) {
	app := newApp(t, false)
	ac := docTestAccount(t, app)
	err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.pdf", MIME: "application/pdf", Data: []byte("x"), ExpiryISO: "soon"})
	if !errors.Is(err, ErrInvalidDocExpiry) {
		t.Fatalf("err = %v, want ErrInvalidDocExpiry", err)
	}
	if n := len(app.Artifacts()); n != 0 {
		t.Fatalf("%d orphaned artifacts", n)
	}
}

func TestAttachAccountDocumentRejectsUnnamedFile(t *testing.T) {
	app := newApp(t, false)
	ac := docTestAccount(t, app)
	// A blank name fails the artifact write, so no reference is filed.
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: " ", MIME: "application/pdf", Data: []byte("x")}); err == nil {
		t.Fatal("expected an error for an unnamed file")
	}
	if n := len(app.Artifacts()); n != 0 {
		t.Fatalf("%d artifacts left behind", n)
	}
}

func TestRemoveAccountDocument(t *testing.T) {
	app := newApp(t, false)
	ac := docTestAccount(t, app)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.png", MIME: "image/png", Data: pngTestBytes}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if k := app.Artifacts()[0].Kind; k != "image" {
		t.Fatalf("kind = %q, want image", k)
	}
	artID := app.Artifacts()[0].ID
	if err := app.RemoveAccountDocument("nope", artID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("missing account: err = %v", err)
	}
	if err := app.RemoveAccountDocument(ac.ID, "other"); !errors.Is(err, ErrAccountDocumentNotFound) {
		t.Fatalf("missing doc: err = %v", err)
	}
	if err := app.RemoveAccountDocument(ac.ID, artID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	for _, a := range app.Accounts() {
		if a.ID == ac.ID && len(a.DocRefs) != 0 {
			t.Fatalf("doc refs left: %+v", a.DocRefs)
		}
	}
}

func TestParseDocExpiry(t *testing.T) {
	if d, err := ParseDocExpiry("  "); err != nil || !d.IsZero() {
		t.Fatalf("blank: %v %v", d, err)
	}
	if d, err := ParseDocExpiry("2027-01-02"); err != nil || d.Year() != 2027 {
		t.Fatalf("valid: %v %v", d, err)
	}
	if _, err := ParseDocExpiry("2027-13-40"); !errors.Is(err, ErrInvalidDocExpiry) {
		t.Fatalf("invalid: %v", err)
	}
}

func TestRemoveAccountDocumentReturnsStorageErrorAndKeepsArtifact(t *testing.T) {
	app := newApp(t, false)
	ac := docTestAccount(t, app)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "a.png", MIME: "image/png", Data: pngTestBytes}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	artID := app.Artifacts()[0].ID
	if err := app.Store().Close(); err != nil {
		t.Fatal(err)
	}
	// With every read failing, the artifact must not be judged unreferenced.
	if referenced, err := app.artifactStillReferenced(artID, ac); err == nil || referenced {
		t.Fatalf("artifactStillReferenced = (%v, %v), want (false, storage error)", referenced, err)
	}
	err := app.RemoveAccountDocument(ac.ID, artID)
	if err == nil {
		t.Fatal("RemoveAccountDocument succeeded although storage reads failed")
	}
	if errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("a storage failure was reported as a missing account: %v", err)
	}
}
