// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/docexpiry"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// accounts_docs.go is the AC8 account-documents drawer (statements, contracts,
// titles, payoff letters) plus AC17's expiry-reminder date on attach. A document
// references a stored domain.Artifact (the same blob store receipts and goal images
// use) via domain.AccountDocRef; the drawer only ever holds the metadata + join, the
// bytes live once in the artifact store.

// attachAccountDoc opens the file picker (PDF or image), stores the chosen file as an
// Artifact, and appends a new AccountDocRef to the account (label + optional expiry).
// It operates on the stored account so it survives independently of unsaved edits in
// the row's other editors, then reconciles AC17's renewal-reminder tasks so a dated
// attach immediately schedules (or the removal of a date clears) its nudge.
func attachAccountDoc(accountID, label, expiryISO string, onErr func(string)) {
	app := appstate.Default
	if app == nil {
		return
	}
	pickFile("application/pdf,image/*", func(name, mime string, data []byte) {
		kind := "document"
		if strings.HasPrefix(mime, "image/") {
			kind = "image"
		}
		art := domain.Artifact{ID: id.New(), Name: name, Kind: kind, MIME: mime, Bytes: data, Size: len(data), CreatedAt: time.Now()}
		if err := app.PutArtifact(art); err != nil {
			if onErr != nil {
				onErr(err.Error())
			}
			return
		}
		ref := domain.AccountDocRef{ArtifactID: art.ID, Label: strings.TrimSpace(label), AttachedAt: time.Now()}
		if exp := strings.TrimSpace(expiryISO); exp != "" {
			if d, err := dateutil.ParseDate(exp); err == nil {
				ref.ExpiresAt = d
			}
		}
		for _, ac := range app.Accounts() {
			if ac.ID != accountID {
				continue
			}
			ac.DocRefs = append(append([]domain.AccountDocRef(nil), ac.DocRefs...), ref)
			if err := app.PutAccount(ac); err != nil {
				if onErr != nil {
					onErr(err.Error())
				}
				return
			}
			break
		}
		reconcileDocExpiry(app)
		uistate.BumpDataRevision()
		uistate.RequestPersist()
		uistate.PostNotice(uistate.T("accounts.documentAttached"), false)
	})
}

// removeAccountDoc deletes one document reference (by artifact id) from an account
// and reconciles the AC17 renewal-reminder tasks so a cleared document's nudge (if
// any) resolves itself. The underlying artifact bytes are left to the blob GC.
func removeAccountDoc(accountID, artifactID string) {
	app := appstate.Default
	if app == nil {
		return
	}
	for _, ac := range app.Accounts() {
		if ac.ID != accountID {
			continue
		}
		var kept []domain.AccountDocRef
		for _, d := range ac.DocRefs {
			if d.ArtifactID != artifactID {
				kept = append(kept, d)
			}
		}
		ac.DocRefs = kept
		if err := app.PutAccount(ac); err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		break
	}
	reconcileDocExpiry(app)
	uistate.BumpDataRevision()
	uistate.RequestPersist()
	uistate.PostNotice(uistate.T("accounts.documentRemoved"), false)
}

// reconcileDocExpiry brings the AC17 renewal-reminder tasks in line with the current
// document set. Safe to call on every document mutation — it never spawns duplicates.
func reconcileDocExpiry(app *appstate.App) {
	if app == nil {
		return
	}
	if _, _, err := app.ReconcileDocExpiryTasks(time.Now(), docexpiry.DefaultLeadDays); err != nil {
		uistate.PostNotice(err.Error(), true)
	}
}

// accountDocsDrawerProps configures the per-account documents disclosure.
type accountDocsDrawerProps struct {
	Account domain.Account
}

// accountDocsDrawer renders the AC8 documents disclosure for one account row: a
// toggle naming how many documents are filed, and — expanded — the dated list (open /
// remove each) plus an "Attach a document" mini-form with an optional label and an
// optional AC17 expiry date. Its own component (its own hooks) so it slots into
// AccountRow at a stable position without disturbing the row's other toggles.
func accountDocsDrawer(props accountDocsDrawerProps) ui.Node {
	a := props.Account
	expanded := ui.UseState(false)
	toggle := ui.UseEvent(Prevent(func() { expanded.Set(!expanded.Get()) }))
	labelS := ui.UseState("")
	expiryS := ui.UseState("")
	onLabel := ui.UseEvent(func(v string) { labelS.Set(v) })
	onExpiry := ui.UseEvent(func(v string) { expiryS.Set(v) })
	attach := ui.UseEvent(Prevent(func() {
		attachAccountDoc(a.ID, labelS.Get(), expiryS.Get(), func(msg string) { uistate.PostNotice(msg, true) })
		labelS.Set("")
		expiryS.Set("")
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

	toggleBtn := Button(css.Class("btn-link", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
		Attr("data-testid", "acct-docs-toggle-"+a.ID), Attr("aria-expanded", ariaBool(expanded.Get())), OnClick(toggle),
		uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(toggleLabel))

	if !expanded.Get() {
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
		dataURL := ""
		if len(art.Bytes) > 0 {
			dataURL = artifacts.DataURL(art.MIME, art.Bytes)
		}
		meta := uistate.T("accounts.documentFiledOn", fmtShortDate(d.AttachedAt))
		if !d.ExpiresAt.IsZero() {
			meta += " · " + uistate.T("accounts.documentExpiresOn", fmtShortDate(d.ExpiresAt))
		}
		rows = append(rows, ui.CreateElement(accountDocRow, accountDocRowProps{
			AccountID: a.ID, ArtifactID: d.ArtifactID, Label: label, Meta: meta, DataURL: dataURL,
		}))
	}

	return Div(css.Class("acct-docs"), Attr("data-testid", "acct-docs-drawer-"+a.ID),
		toggleBtn,
		If(n == 0, P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0.35rem 0"}), uistate.T("accounts.documentsEmpty"))),
		If(n > 0, Div(css.Class("rows", tw.Mt1), rows)),
		Div(css.Class(tw.Mt2), Attr("data-testid", "acct-docs-attach-"+a.ID),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				Input(css.Class("field"), Type("text"), Attr("data-testid", "acct-doc-label-"+a.ID),
					Style(map[string]string{"flex": "1 1 12rem"}),
					Placeholder(uistate.T("accounts.documentLabelPh")), Attr("aria-label", uistate.T("accounts.documentLabelField")),
					Value(labelS.Get()), OnInput(onLabel)),
				Input(css.Class("field"), Type("date"), Attr("data-testid", "acct-doc-expiry-"+a.ID),
					Attr("aria-label", uistate.T("accounts.documentExpiryField")), Title(uistate.T("accounts.documentExpiryField")),
					Value(expiryS.Get()), OnInput(onExpiry)),
				Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "acct-doc-attach-btn-"+a.ID),
					OnClick(attach), uistate.T("accounts.attachDocument")),
			),
			P(css.Class("t-caption", tw.TextDim), Style(map[string]string{"margin": "0.25rem 0 0"}), uistate.T("accounts.documentExpiryHint")),
		),
	)
}

// accountDocRowProps carries the data + callback for one filed-document row.
type accountDocRowProps struct {
	AccountID  string
	ArtifactID string
	Label      string
	Meta       string
	DataURL    string // "" when the referenced artifact's bytes are missing
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
	var openLink ui.Node = Fragment()
	if props.DataURL != "" {
		openLink = A(css.Class("btn-link"), Attr("data-testid", "acct-doc-open-"+props.ArtifactID),
			Href(props.DataURL), Attr("target", "_blank"), Attr("rel", "noopener noreferrer"),
			uistate.T("accounts.documentOpen"))
	}
	return Div(css.Class("row"), Attr("data-testid", "acct-doc-row-"+props.ArtifactID),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), props.Label),
			Span(css.Class("row-meta"), props.Meta),
		),
		openLink,
		Button(css.Class("btn-link"), Type("button"), Attr("data-testid", "acct-doc-remove-"+props.ArtifactID),
			OnClick(remove), uistate.T("accounts.documentRemove")),
	)
}
