// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// utf16 encodes ASCII text as UTF-16 with a byte-order mark.
func utf16Bytes(text string, littleEndian bool) []byte {
	out := []byte{0xFF, 0xFE}
	if !littleEndian {
		out = []byte{0xFE, 0xFF}
	}
	for _, r := range []byte(text) {
		if littleEndian {
			out = append(out, r, 0)
		} else {
			out = append(out, 0, r)
		}
	}
	return out
}

func TestClassifyDocumentRefusesMarkupHiddenBySniffingTricks(t *testing.T) {
	pad := strings.Repeat(" ", 600)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`
	refused := []struct {
		name, file, mime string
		data             []byte
	}{
		{"markup after 512 bytes of whitespace", "a.pdf", "application/pdf", []byte(pad + svg)},
		{"markup after a long comment", "a.txt", "", []byte("<!-- " + strings.Repeat("x", 2000) + " --><svg/>")},
		{"markup after a byte-order mark and blanks", "a.pdf", "application/pdf", []byte("\xef\xbb\xbf \r\n" + svg)},
		{"utf-16 little endian markup", "a.pdf", "application/pdf", utf16Bytes(svg, true)},
		{"utf-16 big endian markup", "a.pdf", "application/pdf", utf16Bytes(svg, false)},
		{"script deep inside a text file", "a.txt", "text/plain", []byte(strings.Repeat("hello\n", 500) + "<script>alert(1)</script>")},
		{"doctype with blanks first", "a.txt", "", []byte("\t\n<!DOCTYPE html><title>x</title>")},
		{"xml prolog", "a.txt", "", []byte("<?xml version='1.0'?><svg/>")},
		{"a name ending in a slash", "evil.svg/", "application/pdf", []byte("%PDF")},
		{"a name ending in a backslash", `evil.html\`, "application/pdf", []byte("%PDF")},
		{".shtml", "page.shtml", "application/pdf", []byte("%PDF")},
		{".xhtm", "page.xhtm", "application/pdf", []byte("%PDF")},
		{".mht", "page.mht", "application/pdf", []byte("%PDF")},
		{".mhtml", "page.MHTML", "application/pdf", []byte("%PDF")},
		{"a png with text bytes", "a.png", "image/png", []byte("not a png")},
		{"a jpeg with pdf bytes", "a.jpg", "image/jpeg", []byte("%PDF-1.4")},
		{"a gif with html bytes", "a.gif", "image/gif", []byte("<html></html>")},
		{"a webp without its signature", "a.webp", "image/webp", []byte("RIFF\x00\x00\x00\x00WAVE")},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := classifyDocument(tt.file, tt.mime, tt.data); !errors.Is(err, ErrUnsupportedDocType) {
				t.Fatalf("classifyDocument = %v, want ErrUnsupportedDocType", err)
			}
		})
	}
}

func TestClassifyDocumentAcceptsRasterImagesAndPlainDocuments(t *testing.T) {
	accepted := []struct {
		name, file, mime, wantKind string
		data                       []byte
	}{
		{"png", "a.png", "image/png", "image", pngTestBytes},
		{"jpeg", "a.jpg", "image/jpeg", "image", jpegTestBytes},
		{"gif", "a.gif", "image/gif", "image", []byte("GIF89a\x01\x00")},
		{"webp", "a.webp", "image/webp", "image", []byte("RIFF\x10\x00\x00\x00WEBPVP8 ")},
		{"bmp", "a.bmp", "image/bmp", "image", []byte("BM\x36\x00\x00\x00")},
		{"heic", "a.heic", "image/heic", "image", []byte("\x00\x00\x00\x18ftypheic")},
		{"pdf", "a.pdf", "application/pdf", "document", []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3")},
		{"plain text", "notes.txt", "text/plain", "document", []byte("Account number ends 4412. Call 555-0100.")},
		{"a zip based office file", "a.docx", "", "document", []byte("PK\x03\x04\x14\x00\x00\x00")},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			_, kind, err := classifyDocument(tt.file, tt.mime, tt.data)
			if err != nil || kind != tt.wantKind {
				t.Fatalf("classifyDocument = kind %q, %v; want %q", kind, err, tt.wantKind)
			}
		})
	}
}

func TestAttachAccountDocumentWritesTheFileAndReferenceInOneCommit(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	ac := docTestAccount(t, app)
	rev := app.Store().Rev()
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "policy.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.4")}); err != nil {
		t.Fatal(err)
	}
	if got := app.Store().Rev() - rev; got != 1 {
		t.Fatalf("attach advanced the store revision by %d, want one commit", got)
	}
	// A failure leaves neither the file nor the reference.
	broken := storedAccount(app, ac.ID)
	broken.Name = ""
	if err := app.Store().PutAccount(broken); err != nil {
		t.Skipf("store refuses an unnamed account: %v", err)
	}
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "two.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.4")}); err == nil {
		t.Fatal("attach succeeded for an account that fails validation")
	}
	if n := len(app.Artifacts()); n != 1 {
		t.Fatalf("%d artifacts after a failed attach, want only the first", n)
	}
	if n := len(storedAccount(app, ac.ID).DocRefs); n != 1 {
		t.Fatalf("%d references after a failed attach, want 1", n)
	}
}

func TestRemoveAccountDocumentKeepsAFileAnotherAccountStillUses(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	first := docTestAccount(t, app)
	other := refsAccount("other")
	if err := app.PutAccount(other); err != nil {
		t.Fatal(err)
	}
	if err := app.AttachAccountDocument(first.ID, AccountDocumentInput{Name: "shared.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.4")}); err != nil {
		t.Fatal(err)
	}
	artID := app.Artifacts()[0].ID
	other = storedAccount(app, "other")
	other.DocRefs = []domain.AccountDocRef{{ArtifactID: artID, Label: "same file"}}
	if err := app.PutAccount(other); err != nil {
		t.Fatal(err)
	}
	if err := app.RemoveAccountDocument(first.ID, artID); err != nil {
		t.Fatal(err)
	}
	if len(app.Artifacts()) != 1 {
		t.Fatal("the file was deleted while another account still references it")
	}
	if err := app.RemoveAccountDocument("other", artID); err != nil {
		t.Fatal(err)
	}
	if len(app.Artifacts()) != 0 {
		t.Fatal("the last reference was removed but the file stayed")
	}
}

func TestRemoveAccountDocumentKeepsAFileATransactionAttachmentUses(t *testing.T) {
	app := newTestAppAt(t, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC))
	ac := docTestAccount(t, app)
	if err := app.AttachAccountDocument(ac.ID, AccountDocumentInput{Name: "receipt.pdf", MIME: "application/pdf", Data: bytes.Repeat([]byte("%PDF"), 3)}); err != nil {
		t.Fatal(err)
	}
	artID := app.Artifacts()[0].ID
	row := domain.Transaction{ID: "t1", AccountID: ac.ID, Desc: "Receipt", Amount: ac.OpeningBalance, Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Attachments: []domain.AttachmentRef{{ArtifactID: artID, Name: "receipt.pdf"}}}
	row.Amount.Currency = "USD"
	row.Amount.Amount = -100
	if err := app.PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	if err := app.RemoveAccountDocument(ac.ID, artID); err != nil {
		t.Fatal(err)
	}
	if len(app.Artifacts()) != 1 {
		t.Fatal("the file was deleted although a transaction attachment points at it")
	}
}
