// SPDX-License-Identifier: MIT

package appstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/artifactref"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/validate"
)

// ErrAccountNotFound is returned by the account-document operations when the target
// account does not exist.
var ErrAccountNotFound = errors.New("appstate: account not found")

// ErrAccountDocumentNotFound is returned by RemoveAccountDocument when the account
// holds no document referencing the given artifact.
var ErrAccountDocumentNotFound = errors.New("appstate: account document not found")

// ErrInvalidDocExpiry is returned for a non-empty renewal date that is not a
// YYYY-MM-DD calendar date.
var ErrInvalidDocExpiry = errors.New("appstate: renewal date is not a valid date")

// AccountDocumentInput describes one file to attach to an account: the file as the
// picker delivered it, plus the label and optional renewal date the person typed.
type AccountDocumentInput struct {
	Name      string // file name; names the stored artifact
	MIME      string // media type; an image/* type files the artifact as an image
	Data      []byte // file bytes
	Label     string // optional display label
	ExpiryISO string // optional renewal date, YYYY-MM-DD; blank means none
}

// ParseDocExpiry parses the optional renewal date typed on the attach form. A blank
// string yields the zero time and no error; a non-blank string that is not a
// YYYY-MM-DD date yields ErrInvalidDocExpiry.
func ParseDocExpiry(expiryISO string) (time.Time, error) {
	exp := strings.TrimSpace(expiryISO)
	if exp == "" {
		return time.Time{}, nil
	}
	d, err := dateutil.ParseDate(exp)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %v", ErrInvalidDocExpiry, err)
	}
	return d, nil
}

// MaxAccountDocumentBytes is the largest file AttachAccountDocument accepts.
const MaxAccountDocumentBytes = 20 << 20

// ErrUnsupportedDocType is returned when a document could run script when opened, such as SVG,
// HTML, XML or JavaScript, judged by its declared media type, its file name and its first bytes.
// Such files are refused at upload.
var ErrUnsupportedDocType = errors.New("appstate: document type is not allowed")

// ErrDocumentEmpty is returned when an attached file has no bytes.
var ErrDocumentEmpty = errors.New("appstate: document is empty")

// ErrDocumentTooLarge is returned when an attached file is larger than MaxAccountDocumentBytes.
var ErrDocumentTooLarge = errors.New("appstate: document is too large")

// scriptCapableSubtype reports whether a media subtype is markup or script that a browser would
// run: HTML, XML and its +xml/+html derivatives (SVG, XHTML, XSLT), XSL, and JavaScript.
// Office formats such as the vnd.openxmlformats types are not markup and pass.
func scriptCapableSubtype(sub string) bool {
	switch sub {
	case "html", "xml", "xsl", "xslt", "svg":
		return true
	}
	return strings.HasSuffix(sub, "+xml") || strings.HasSuffix(sub, "+html") ||
		strings.Contains(sub, "javascript") || strings.Contains(sub, "ecmascript")
}

// refusedDocExtensions are file extensions of script-capable formats.
var refusedDocExtensions = map[string]bool{
	".svg": true, ".svgz": true, ".html": true, ".htm": true, ".xhtml": true, ".xhtm": true, ".xht": true,
	".shtml": true, ".shtm": true, ".mht": true, ".mhtml": true,
	".xml": true, ".xsl": true, ".xslt": true, ".js": true, ".mjs": true, ".hta": true,
}

// imageDocSubtypes are the raster image subtypes filed as images.
var imageDocSubtypes = map[string]bool{
	"png": true, "jpeg": true, "jpg": true, "gif": true, "webp": true, "heic": true,
	"heif": true, "avif": true, "bmp": true, "tiff": true,
}

// refusedDocSniff are the leading markup forms that make a file script-capable whatever it claims.
var refusedDocSniff = []string{"<svg", "<script", "<html", "<!doctype", "<?xml", "<iframe", "<body", "<head"}

// embeddedMarkup are markup forms that make a text file script-capable wherever they appear in
// its first docSniffBytes bytes.
var embeddedMarkup = []string{"<svg", "<script", "<html", "<!doctype html", "<iframe"}

// docSniffBytes is how much of a file classifyDocument reads for markup.
const docSniffBytes = 64 << 10

// sniffText returns the lower-cased leading text of data for markup checks: a UTF-8 or UTF-16
// byte-order mark is removed (UTF-16 is reduced to its ASCII characters), then leading space. The
// bool is false when the bytes are binary.
func sniffText(data []byte) (string, bool) {
	if len(data) > docSniffBytes {
		data = data[:docSniffBytes]
	}
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE:
		data = utf16ASCII(data[2:], 1)
	case len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF:
		data = utf16ASCII(data[2:], 0)
	default:
		data = []byte(strings.TrimPrefix(string(data), "\xef\xbb\xbf"))
	}
	if slices.Contains(data[:min(len(data), 512)], 0) {
		return "", false
	}
	return strings.ToLower(strings.TrimLeft(string(data), " \t\r\n\f\v")), true
}

// utf16ASCII keeps the low byte of each UTF-16 code unit whose other byte is zero, which is enough
// to read ASCII markup; the high byte sits at offset hi of each pair.
func utf16ASCII(data []byte, hi int) []byte {
	out := make([]byte, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if data[i+hi] == 0 {
			out = append(out, data[i+1-hi])
		} else {
			out = append(out, '?')
		}
	}
	return out
}

// looksLikeRaster reports whether data starts with the magic bytes of a raster image format: PNG,
// JPEG, GIF, WebP, BMP, TIFF, or the HEIF family.
func looksLikeRaster(data []byte) bool {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")),
		bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}),
		bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")),
		bytes.HasPrefix(data, []byte("BM")),
		bytes.HasPrefix(data, []byte("II*\x00")), bytes.HasPrefix(data, []byte("MM\x00*")):
		return true
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return true
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		return true
	}
	return false
}

// classifyDocument parses the declared media type, then returns the media type to store (lower
// case, without parameters) and the artifact kind. It returns ErrUnsupportedDocType when the media
// type, the file name or the leading bytes mark the file as script-capable, when a declared image
// is not a raster image type, or when a declared image does not start with raster image bytes.
func classifyDocument(name, declared string, data []byte) (mediaType, kind string, err error) {
	kind = artifacts.KindDocument
	if declared = strings.TrimSpace(declared); declared != "" {
		parsed, _, perr := mime.ParseMediaType(declared)
		if perr != nil {
			return "", "", fmt.Errorf("%w: %q", ErrUnsupportedDocType, declared)
		}
		mediaType = strings.ToLower(parsed)
		top, sub, _ := strings.Cut(mediaType, "/")
		if scriptCapableSubtype(sub) {
			return "", "", fmt.Errorf("%w: %s", ErrUnsupportedDocType, mediaType)
		}
		if top == "image" {
			if !imageDocSubtypes[strings.TrimPrefix(sub, "x-")] {
				return "", "", fmt.Errorf("%w: %s", ErrUnsupportedDocType, mediaType)
			}
			if !looksLikeRaster(data) {
				return "", "", fmt.Errorf("%w: the file is not a %s image", ErrUnsupportedDocType, mediaType)
			}
			kind = artifacts.KindImage
		}
	}
	trimmed := strings.TrimRight(strings.TrimSpace(name), ". ")
	if strings.HasSuffix(trimmed, "/") || strings.HasSuffix(trimmed, `\`) {
		return "", "", fmt.Errorf("%w: the file name ends in a path separator", ErrUnsupportedDocType)
	}
	ext := strings.ToLower(filepath.Ext(trimmed))
	if refusedDocExtensions[ext] {
		return "", "", fmt.Errorf("%w: %s files", ErrUnsupportedDocType, ext)
	}
	if lead, text := sniffText(data); text {
		for _, marker := range refusedDocSniff {
			if strings.HasPrefix(lead, marker) {
				return "", "", fmt.Errorf("%w: the file contents are markup", ErrUnsupportedDocType)
			}
		}
		for _, marker := range embeddedMarkup {
			if strings.Contains(lead, marker) {
				return "", "", fmt.Errorf("%w: the file contents are markup", ErrUnsupportedDocType)
			}
		}
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	if sniffed := strings.ToLower(http.DetectContentType(head)); strings.HasPrefix(sniffed, "text/html") || strings.HasPrefix(sniffed, "text/xml") {
		return "", "", fmt.Errorf("%w: the file contents are markup", ErrUnsupportedDocType)
	}
	return mediaType, kind, nil
}

// AttachAccountDocument stores the file as an Artifact and files a reference to it on
// the account in one commit, then reconciles the renewal-reminder tasks so a dated attach
// schedules its nudge straight away. The file must be non-empty, at most
// MaxAccountDocumentBytes, and not script-capable (see classifyDocument); the stored media
// type is the declared one, lower-cased and without parameters, and an image type files the
// artifact as an image. The account and the renewal date are checked before anything is
// written, and the artifact and the reference land together or not at all, so a failed or
// interrupted attach leaves no orphaned blob. When a blob store is wired in, the file bytes go
// there (StoreBlobForArtifact) and the artifact row keeps only its metadata; without one the
// bytes stay on the row. The returned error wraps ErrAccountNotFound,
// ErrInvalidDocExpiry, ErrUnsupportedDocType, ErrDocumentEmpty or ErrDocumentTooLarge for
// those causes.
func (a *App) AttachAccountDocument(accountID string, in AccountDocumentInput) error {
	if len(in.Data) == 0 {
		return ErrDocumentEmpty
	}
	if len(in.Data) > MaxAccountDocumentBytes {
		return ErrDocumentTooLarge
	}
	mediaType, kind, err := classifyDocument(in.Name, in.MIME, in.Data)
	if err != nil {
		return err
	}
	if err := a.roleGuard(); err != nil {
		return err
	}
	ac, ok, err := a.findAccountChecked(accountID)
	if err != nil {
		return fmt.Errorf("attach account document: %w", err)
	}
	if !ok {
		return fmt.Errorf("attach account document: %w: %q", ErrAccountNotFound, accountID)
	}
	expiresAt, err := ParseDocExpiry(in.ExpiryISO)
	if err != nil {
		return fmt.Errorf("attach account document: %w", err)
	}
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("attach account document: the file needs a name")
	}
	now := a.clock()
	art := domain.Artifact{ID: id.New(), Name: in.Name, Kind: kind, MIME: mediaType, Bytes: in.Data, Size: len(in.Data), CreatedAt: now}
	ref := domain.AccountDocRef{ArtifactID: art.ID, Label: strings.TrimSpace(in.Label), AttachedAt: now, ExpiresAt: expiresAt}
	updated := ac
	updated.DocRefs = append(append([]domain.AccountDocRef(nil), ac.DocRefs...), ref)
	stored, err := a.StoreBlobForArtifact(art)
	if err != nil {
		return fmt.Errorf("attach account document: %w", err)
	}
	blobStored := len(stored.Bytes) == 0
	artifact, err := json.Marshal(stored)
	if err != nil {
		a.discardBlob(art.ID, blobStored)
		return fmt.Errorf("attach account document: encode file: %w", err)
	}
	changes := []store.RecordChange{{Table: "artifacts", ID: art.ID, After: artifact}}
	if err := a.commitAccountDocRefs(ac, updated, changes); err != nil {
		a.discardBlob(art.ID, blobStored)
		return fmt.Errorf("attach account document: %w", err)
	}
	a.log.Info("artifact saved", "id", art.ID, "kind", art.Kind, "size", art.Size)
	a.reconcileDocExpiry(now)
	return nil
}

// RemoveAccountDocument drops the reference to one document (by artifact id) from an
// account and reconciles the renewal-reminder tasks, so the document's reminder, if
// any, resolves itself. The stored file is deleted in the same commit when no other
// account document or transaction attachment still points at it, and its blob leaves the blob
// store once that commit succeeds; a failed blob delete is logged, not returned. The returned error
// wraps ErrAccountNotFound or ErrAccountDocumentNotFound for those causes.
func (a *App) RemoveAccountDocument(accountID, artifactID string) error {
	if err := a.roleGuard(); err != nil {
		return err
	}
	ac, ok, err := a.findAccountChecked(accountID)
	if err != nil {
		return fmt.Errorf("remove account document: %w", err)
	}
	if !ok {
		return fmt.Errorf("remove account document: %w: %q", ErrAccountNotFound, accountID)
	}
	kept := make([]domain.AccountDocRef, 0, len(ac.DocRefs))
	for _, d := range ac.DocRefs {
		if d.ArtifactID != artifactID {
			kept = append(kept, d)
		}
	}
	if len(kept) == len(ac.DocRefs) {
		return fmt.Errorf("remove account document: %w: %q", ErrAccountDocumentNotFound, artifactID)
	}
	updated := ac
	updated.DocRefs = kept
	var changes []store.RecordChange
	referenced, err := a.artifactStillReferenced(artifactID, updated)
	if err != nil {
		return fmt.Errorf("remove account document: %w", err)
	}
	if !referenced {
		changes = append(changes, store.RecordChange{Table: "artifacts", ID: artifactID})
	}
	if err := a.commitAccountDocRefs(ac, updated, changes); err != nil {
		return fmt.Errorf("remove account document: %w", err)
	}
	if !referenced && a.blobs != nil {
		if err := a.blobs.Delete(artifactID); err != nil {
			a.log.Warn("delete removed document blob", "id", artifactID, "err", err)
		}
	}
	a.reconcileDocExpiry(a.clock())
	return nil
}

// findAccountChecked looks up an account by ID. A storage failure is returned as an error,
// never reported as a missing account.
func (a *App) findAccountChecked(accountID string) (domain.Account, bool, error) {
	accounts, err := a.AccountsChecked()
	if err != nil {
		return domain.Account{}, false, err
	}
	for _, ac := range accounts {
		if ac.ID == accountID {
			return ac, true, nil
		}
	}
	return domain.Account{}, false, nil
}

// artifactStillReferenced reports whether the artifact is still pointed at by anything, using
// artifactref.Referenced (account documents, transaction attachments, custom-page widget bindings
// and goal vision images) with updated standing in for its stored account copy. A failed read of
// any of those sources is returned as an error: a partial read must not make a bound artifact
// look unreferenced.
func (a *App) artifactStillReferenced(artifactID string, updated domain.Account) (bool, error) {
	accounts, err := a.AccountsChecked()
	if err != nil {
		return false, err
	}
	txns, err := a.TransactionsChecked()
	if err != nil {
		return false, err
	}
	goals, err := a.GoalsChecked()
	if err != nil {
		return false, err
	}
	pages, err := a.store.ListCustomPages()
	if err != nil {
		return false, fmt.Errorf("read custom pages: %w", err)
	}
	accounts = slices.Clone(accounts)
	for i := range accounts {
		if accounts[i].ID == updated.ID {
			accounts[i] = updated
		}
	}
	return artifactref.Referenced(txns, pages, goals, accounts)[artifactID], nil
}

// discardBlob removes the blob stored for an artifact whose row was never committed. A failure
// is logged; the orphan is only wasted space.
func (a *App) discardBlob(artifactID string, stored bool) {
	if !stored || a.blobs == nil {
		return
	}
	if err := a.blobs.Delete(artifactID); err != nil {
		a.log.Warn("discard uncommitted document blob", "id", artifactID, "err", err)
	}
}

// commitAccountDocRefs validates the account with its new document references and writes it
// together with extra (artifact adds and removals) in one commit.
func (a *App) commitAccountDocRefs(before, after domain.Account, extra []store.RecordChange) error {
	if is := validate.ValidateAccount(after); !is.OK() {
		return fmt.Errorf("save account: %w", is)
	}
	if err := a.validateCustom("account", after.Custom); err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	beforeData, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("encode account: %w", err)
	}
	afterData, err := json.Marshal(after)
	if err != nil {
		return fmt.Errorf("encode updated account: %w", err)
	}
	changes := append(append([]store.RecordChange(nil), extra...),
		store.RecordChange{Table: "accounts", ID: after.ID, Before: beforeData, After: afterData})
	if err := a.store.ReplayChanges(changes); err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	a.log.Info("account documents saved", "id", after.ID, "documents", len(after.DocRefs))
	return nil
}

// reconcileDocExpiry brings the renewal-reminder tasks in line with the current
// document set. The document change it follows has already been saved, so a failure
// here is logged and the next document change reconciles again.
func (a *App) reconcileDocExpiry(now time.Time) {
	if _, _, err := a.ReconcileDocExpiryTasks(now, docexpiry.DefaultLeadDays); err != nil {
		a.log.Warn("reconcile document renewal reminders", "err", err)
	}
}
