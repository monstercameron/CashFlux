// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"log/slog"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// This file holds the account-documents drawer (statements, contracts, titles and
// payoff letters) and the expiry-reminder date taken on attach. A document references a
// stored domain.Artifact, the same blob store receipts and goal images use, through
// domain.AccountDocRef. The drawer holds only the metadata and the join; the bytes live
// once in the artifact store.

// docErrorMessage turns an account-document error into the sentence shown to the person. An error
// it does not know is logged, and the person gets a general sentence, never the internal text.
func docErrorMessage(err error) string {
	switch {
	case errors.Is(err, appstate.ErrReadOnly):
		return uistate.T("accounts.writeReadOnly")
	case errors.Is(err, appstate.ErrInvalidDocExpiry):
		return uistate.T("accounts.documentExpiryInvalid")
	case errors.Is(err, appstate.ErrAccountNotFound):
		return uistate.T("accounts.documentAccountMissing")
	case errors.Is(err, appstate.ErrAccountDocumentNotFound):
		return uistate.T("accounts.documentMissing")
	case errors.Is(err, appstate.ErrUnsupportedDocType):
		return uistate.T("accounts.documentTypeUnsupported")
	case errors.Is(err, appstate.ErrDocumentEmpty):
		return uistate.T("accounts.documentEmpty")
	case errors.Is(err, appstate.ErrDocumentTooLarge):
		return uistate.T("accounts.documentTooLarge", appstate.MaxAccountDocumentBytes>>20)
	}
	slog.Error("account document", "error", err)
	return uistate.T("accounts.documentFailed")
}

// attachPickGuard is how long a second attach click is ignored after the file picker opens. The
// picker reports nothing when it is cancelled, so the guard is a short window rather than a flag
// that would have to be cleared by an event that may never come.
const attachPickGuard = time.Second

// lastAttachPick is when the file picker was last opened for an attachment.
var lastAttachPick time.Time

// attachAccountDoc checks the renewal date, opens the file picker (PDF or image), and
// hands the chosen file with the label and expiry to appstate, which stores it and files
// it on the account. onDone runs after a successful attach so the caller can clear its
// inputs; a failure posts an error notice and leaves them untouched.
func attachAccountDoc(accountID, label, expiryISO string, onDone func()) {
	app := appstate.Default
	if app == nil {
		return
	}
	if _, err := appstate.ParseDocExpiry(expiryISO); err != nil {
		uistate.PostNotice(docErrorMessage(err), true)
		return
	}
	now := time.Now()
	if now.Sub(lastAttachPick) < attachPickGuard {
		return
	}
	lastAttachPick = now
	pickFile("application/pdf,image/*", func(name, mime string, data []byte) {
		in := appstate.AccountDocumentInput{Name: name, MIME: mime, Data: data, Label: label, ExpiryISO: expiryISO}
		if err := app.AttachAccountDocument(accountID, in); err != nil {
			uistate.PostNotice(docErrorMessage(err), true)
			return
		}
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("accounts.documentAttached"), false)
		if onDone != nil {
			onDone()
		}
	})
}

// removeAccountDoc deletes one document reference (by artifact id) from an account.
func removeAccountDoc(accountID, artifactID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	if err := app.RemoveAccountDocument(accountID, artifactID); err != nil {
		uistate.PostNotice(docErrorMessage(err), true)
		return
	}
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	uistate.PostNotice(uistate.T("accounts.documentRemoved"), false)
}

// docBlobRevokeDelay is how long an opened document's object URL stays valid; the
// new tab has loaded it well within this window.
const docBlobRevokeDelay = time.Minute

// inlineMIME lists the media types a browser may render in a tab. Anything else is downloaded
// instead, so a stored file cannot run script on the app's own origin.
var inlineMIME = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
}

// openAccountDocument opens a stored document. A type on the inline allowlist opens in a new tab;
// any other type downloads with its original name. The object URL is built here, when the link is
// activated, so no document is encoded while the list renders. Chrome blocks top-frame navigation
// to data URLs, so the bytes go through a Blob. A document whose file is missing, or a tab that the
// browser blocks, is reported in a notice.
func openAccountDocument(artifactID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, art := range app.Artifacts() {
		if art.ID != artifactID {
			continue
		}
		if len(art.Bytes) == 0 {
			break
		}
		mime := strings.ToLower(strings.TrimSpace(art.MIME))
		inline := inlineMIME[mime]
		if !inline {
			mime = "application/octet-stream"
		}
		buf := js.Global().Get("Uint8Array").New(len(art.Bytes))
		js.CopyBytesToJS(buf, art.Bytes)
		parts := js.Global().Get("Array").New()
		parts.Call("push", buf)
		blob := js.Global().Get("Blob").New(parts, map[string]any{"type": mime})
		urlAPI := js.Global().Get("URL")
		url := urlAPI.Call("createObjectURL", blob)
		time.AfterFunc(docBlobRevokeDelay, func() { urlAPI.Call("revokeObjectURL", url) })
		if !inline {
			downloadBlobURL(url, art.Name)
			return
		}
		win := js.Global().Call("open", url, "_blank")
		if win.IsNull() || win.IsUndefined() {
			urlAPI.Call("revokeObjectURL", url)
			uistate.PostNotice(uistate.T("accounts.documentPopupBlocked"), true)
			return
		}
		win.Set("opener", nil)
		return
	}
	uistate.PostNotice(uistate.T("accounts.documentFileMissing"), true)
}

// downloadBlobURL saves a blob URL under name through a transient anchor, so the browser downloads it
// rather than rendering it.
func downloadBlobURL(url js.Value, name string) {
	doc := js.Global().Get("document")
	link := doc.Call("createElement", "a")
	link.Set("href", url)
	link.Set("download", name)
	doc.Get("body").Call("appendChild", link)
	link.Call("click")
	doc.Get("body").Call("removeChild", link)
}

// accountDocsDrawerProps configures the per-account documents disclosure.
type accountDocsDrawerProps struct {
	Account domain.Account
}

// accountDocsDrawer renders the documents disclosure for one account row: a
// toggle naming how many documents are filed, and, expanded, the dated list (open /
// remove each) plus an "Attach a document" mini-form with an optional label and an
// optional expiry date. It is its own component with its own hooks, so it slots into
// AccountRow at a stable position without disturbing the row's other toggles.
func accountDocsDrawer(props accountDocsDrawerProps) ui.Node {
	a := props.Account
	expanded := ui.UseState(false)
	// The list stays mounted for its fade-out, so closing animates as well as opening.
	bodyMounted := uiw.UsePanelFade(expanded.Get())
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	labelS := ui.UseState("")
	expiryS := ui.UseState("")
	onLabel := ui.UseEvent(func(v string) { labelS.Set(v) })
	onExpiry := ui.UseEvent(func(v string) { expiryS.Set(v) })
	attach := ui.UseEvent(Prevent(func() {
		attachAccountDoc(a.ID, labelS.Get(), expiryS.Get(), func() {
			labelS.Set("")
			expiryS.Set("")
		})
	}))

	docs := domain.SortDocRefsByDate(a.DocRefs)
	n := len(docs)

	toggleLabel := uistate.T("accounts.documentsSection")
	if n > 0 {
		toggleLabel = uistate.T("accounts.documentsToggleShow", n)
	}
	if expanded.Get() {
		toggleLabel = uistate.T("accounts.documentsToggleHide")
	}

	toggleBtn := uiw.ToolButton(uiw.ToolButtonProps{Label: toggleLabel, Icon: icon.Paperclip, Open: expanded.Get(), TestID: "acct-docs-toggle-" + a.ID},
		AriaExpanded(expanded.Get()), OnClick(toggle))

	if !expanded.Get() && !bodyMounted {
		return Div(css.Class("acct-docs"), toggleBtn)
	}

	app := appstate.Default
	artByID := map[string]domain.Artifact{}
	if app != nil {
		for _, art := range app.Artifacts() {
			artByID[art.ID] = art
		}
	}

	rows := make([]ui.Node, 0, n)
	for _, d := range docs {
		art := artByID[d.ArtifactID]
		label := d.DisplayLabel(art.Name)
		meta := uistate.T("accounts.documentFiledOn", fmtShortDate(d.AttachedAt))
		if !d.ExpiresAt.IsZero() {
			meta = uistate.T("accounts.documentMeta", meta, uistate.T("accounts.documentExpiresOn", fmtShortDate(d.ExpiresAt)))
		}
		rows = append(rows, WithKey(ui.CreateElement(accountDocRow, accountDocRowProps{
			AccountID: a.ID, ArtifactID: d.ArtifactID, Label: label, Meta: meta, CanOpen: len(art.Bytes) > 0,
		}), d.ArtifactID))
	}

	return Div(css.Class("acct-docs"), Attr("data-testid", "acct-docs-drawer-"+a.ID),
		toggleBtn,
		Div(css.Class(uiw.FadeClass("acct-docs-body", expanded.Get())),
			If(n == 0, P(css.Class("t-caption", tw.TextDim, "acct-docs-empty"), uistate.T("accounts.documentsEmpty"))),
			If(n > 0, Div(css.Class("rows", tw.Mt1), rows)),
			Div(css.Class(tw.Mt2), Attr("data-testid", "acct-docs-attach-"+a.ID),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
					Input(css.Class("field", "acct-doc-label"), Type("text"), Attr("data-testid", "acct-doc-label-"+a.ID),
						Placeholder(uistate.T("accounts.documentLabelPh")), AriaLabel(uistate.T("accounts.documentLabelField")), OnInput(onLabel), uiw.FieldValue(labelS.Get())),
					Input(css.Class("field"), Type("date"), Attr("data-testid", "acct-doc-expiry-"+a.ID),
						AriaLabel(uistate.T("accounts.documentExpiryField")), Title(uistate.T("accounts.documentExpiryField")), OnInput(onExpiry), uiw.FieldValue(expiryS.Get())),
					Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "acct-doc-attach-btn-"+a.ID),
						OnClick(attach), uistate.T("accounts.attachDocument")),
				),
				P(css.Class("t-caption", tw.TextDim, "acct-docs-hint"), uistate.T("accounts.documentExpiryHint")),
			),
		),
	)
}

// accountDocRowProps carries the data + callback for one filed-document row.
type accountDocRowProps struct {
	AccountID  string
	ArtifactID string
	Label      string
	Meta       string
	CanOpen    bool // false when the referenced artifact's bytes are missing
}

// accountDocRow is one document line in the drawer: its label/date meta, an Open
// link (when the bytes resolved), and Remove. Its own component so the Remove click
// hook stays stable across the variable-length document list (never an On* handler
// inside a loop).
func accountDocRow(props accountDocRowProps) ui.Node {
	remove := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("accounts.documentRemoveConfirm"), true, func(ok bool) {
			if ok {
				removeAccountDoc(props.AccountID, props.ArtifactID)
			}
		})
	}))
	open := ui.UseEvent(Prevent(func() { openAccountDocument(props.ArtifactID) }))
	openLink := If(props.CanOpen, Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-doc-open-"+props.ArtifactID),
		AriaLabel(uistate.T("accounts.documentOpenAria", props.Label)), OnClick(open), uistate.T("accounts.documentOpen")))
	return Div(css.Class("row"), Attr("data-testid", "acct-doc-row-"+props.ArtifactID),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Label),
			Span(css.Class("row-meta"), props.Meta),
		),
		openLink,
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-doc-remove-"+props.ArtifactID),
			AriaLabel(uistate.T("accounts.documentRemoveAria", props.Label)), OnClick(remove), uistate.T("accounts.documentRemove")),
	)
}
