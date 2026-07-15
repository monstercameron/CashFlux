// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// TransactionEditFormProps configures the TransactionEditForm component.
type TransactionEditFormProps struct {
	// TxnID is the id of the transaction being edited; looked up live from appstate.
	TxnID string
	// OnDone is called after a successful save or delete (and on Cancel) so the
	// caller (TxnEditHost) can close the modal. On a validation error the form
	// stays open and OnDone is not called.
	OnDone func()
}

// TransactionEditForm is the standalone edit-a-transaction form used by the
// TxnEditHost modal (the widgetized transactions page drills into it on row
// click). It looks the transaction up by id, seeds its fields, validates on
// save, and persists via appstate. Mirrors the inline edit controls in
// transactions_row.go and the add-form submit/error pattern.
func TransactionEditForm(props TransactionEditFormProps) ui.Node {
	return ui.CreateElement(transactionEditForm, props)
}

func transactionEditForm(props TransactionEditFormProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}

	// Look the transaction up live so an edit always works against current data.
	var txn domain.Transaction
	found := false
	for _, t := range app.Transactions() {
		if t.ID == props.TxnID {
			txn, found = t, true
			break
		}
	}

	categories := app.Categories()
	members := app.Members()

	// Seed display values from the transaction (amount shown in major units,
	// absolute, with the sign preserved separately on save).
	amountMajor := ""
	dateISO := ""
	if found {
		amountMajor = money.FormatMinor(txnfilter.AbsAmount(txn), currency.Decimals(txn.Amount.Currency))
		dateISO = dateutil.FormatDate(txn.Date)
	}

	descS := ui.UseState(txn.Desc)
	payeeS := ui.UseState(txn.Payee)
	amountS := ui.UseState(amountMajor)
	catS := ui.UseState(txn.CategoryID)
	dateS := ui.UseState(dateISO)
	memberS := ui.UseState(txn.MemberID)
	tagsS := ui.UseState(strings.Join(txn.Tags, ", "))
	clearedS := ui.UseState(txn.Cleared)
	errMsg := ui.UseState("")
	// C58: the split-into-categories editor, revealed by a toggle inside this modal
	// so the breakdown is visible and editable without leaving the edit form.
	splitOpen := ui.UseState(false)
	toggleSplit := ui.UseEvent(func() { splitOpen.Set(!splitOpen.Get()) })
	// saveSplits persists a breakdown set in the embedded SplitEditor. It writes the
	// stored transaction directly (the editor validated Σ=amount), so it is
	// independent of this form's unsaved field drafts.
	saveSplits := func(updated domain.Transaction) {
		if err := app.PutTransaction(updated); err != nil {
			errMsg.Set(err.Error())
			return
		}
		uistate.BumpDataRevision()
		if updated.HasSplits() {
			uistate.PostNotice(uistate.T("splitEditor.saved"), false)
		} else {
			uistate.PostNotice(uistate.T("splitEditor.cleared"), false)
		}
	}

	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onPayee := ui.UseEvent(func(v string) { payeeS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onTags := ui.UseEvent(func(v string) { tagsS.Set(v) })
	onCleared := ui.UseEvent(func(e ui.Event) { clearedS.Set(e.IsChecked()) })

	save := ui.UseEvent(Prevent(func() {
		t := txn
		amt, err := money.ParseMinor(strings.TrimSpace(amountS.Get()), currency.Decimals(t.Amount.Currency))
		if err != nil || amt <= 0 {
			errMsg.Set(uistate.T("transactions.positiveAmount"))
			return
		}
		if t.Amount.IsNegative() {
			amt = -amt // preserve the original income/expense sign
		}
		date, derr := dateutil.ParseDate(strings.TrimSpace(dateS.Get()))
		if derr != nil {
			errMsg.Set(uistate.T("transactions.invalidDate"))
			return
		}
		t.Desc = strings.TrimSpace(descS.Get())
		t.Payee = strings.TrimSpace(payeeS.Get())
		t.Amount = money.New(amt, t.Amount.Currency)
		t.CategoryID = catS.Get()
		t.Date = date
		if memberS.Get() != "" {
			t.MemberID = memberS.Get()
		}
		t.Tags = textutil.CommaFields(tagsS.Get())
		t.Cleared = clearedS.Get()
		// C58: an amount edit must not silently desync an existing category breakdown —
		// budgets attribute per split line, so a mismatched total would misreport. Block
		// and point at the split section below instead of quietly dropping the split.
		if t.HasSplits() && !t.SplitsReconcile() {
			errMsg.Set(uistate.T("transactions.splitAmountMismatch"))
			return
		}
		if err := app.PutTransaction(t); err != nil {
			errMsg.Set(err.Error())
			return
		}
		// C33-style learn tally: record the payee→category correction on an explicit set.
		if t.CategoryID != "" {
			learn := t.Payee
			if learn == "" {
				learn = t.Desc
			}
			uistate.IncrementLearnTally(learn, t.CategoryID)
		}
		uistate.BumpDataRevision()
		uistate.PostNotice(uistate.T("toast.txnUpdated"), false)
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	del := ui.UseEvent(Prevent(func() {
		id := props.TxnID
		uistate.ConfirmModal(uistate.T("transactions.deleteConfirm", txn.Desc), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteTransaction(id); err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			if props.OnDone != nil {
				props.OnDone()
			}
		})
	}))

	cancel := ui.UseEvent(Prevent(func() {
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	// Attach a receipt image (L29): upload it as an Artifact, link it to the live
	// transaction, and bump the shared data revision so both this modal and the
	// table (whose paperclip opens the preview) reflect it. Operates on the stored
	// transaction, so it persists independently of unsaved field edits in the form.
	attach := ui.UseEvent(Prevent(func() {
		pickFile("image/*", func(name, mime string, data []byte) {
			art := domain.Artifact{ID: id.New(), Name: name, Kind: "image", MIME: mime, Bytes: data, Size: len(data), CreatedAt: time.Now()}
			if err := app.PutArtifact(art); err != nil {
				errMsg.Set(err.Error())
				return
			}
			cur := txn
			cur.Attachments = append(cur.Attachments, domain.AttachmentRef{ArtifactID: art.ID, Name: name, Kind: "image", MIME: mime})
			if err := app.PutTransaction(cur); err != nil {
				errMsg.Set(err.Error())
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("transactions.attachReceiptTitle"), false)
		})
	}))

	if !found {
		return Div(css.Class("form-grid"),
			P(css.Class("empty"), uistate.T("txnwidget.notFound")),
			Button(css.Class("btn"), Type("button"), OnClick(cancel), uistate.T("action.cancel")),
		)
	}

	catOpts := uiw.OptionsFrom(
		categories,
		func(c domain.Category) string { return c.ID },
		func(c domain.Category) string { return c.Name },
		catS.Get(),
	)
	catOpts = append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}}, catOpts...)

	memberOpts := uiw.OptionsFrom(
		members,
		func(m domain.Member) string { return m.ID },
		func(m domain.Member) string { return m.Name },
		memberS.Get(),
	)

	// The current breakdown, listed read-only so a split is visible the moment the
	// modal opens; the full editor is one tap away on the toggle. Category id → name
	// resolves against the live category list (unknown/empty ids read as
	// uncategorized). Static text only, so a plain loop is loop-hook-safe.
	catNameByID := make(map[string]string, len(categories))
	for _, c := range categories {
		catNameByID[c.ID] = c.Name
	}
	var splitLines []ui.Node
	for _, s := range txn.Splits {
		n := catNameByID[s.CategoryID]
		if n == "" {
			n = uistate.T("transactions.uncategorized")
		}
		splitLines = append(splitLines, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class("badge badge-split"), "⑂"),
			Span(n),
			Span(css.Class("muted"), fmtMoney(s.Amount)),
		))
	}

	return Form(css.Class("form-grid txn-edit"), Attr("id", "txn-edit-form"), Attr("data-testid", "txn-edit-form"), OnSubmit(save),
		labeledField(uistate.T("transactions.descPlaceholder"),
			Input(css.Class("field"), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), Value(descS.Get()), OnInput(onDesc))),
		labeledField(uistate.T("transactions.payeeLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.payeeLabel")), Placeholder(uistate.T("transactions.payeeLabel")), Value(payeeS.Get()), OnInput(onPayee))),
		labeledField(uistate.T("transactions.amountPlaceholder"),
			Input(css.Class("field"), Type("number"), Step("0.01"), Placeholder(uistate.T("transactions.amountPlaceholder")), Value(amountS.Get()), OnInput(onAmount))),
		uiw.FormField(uistate.T("transactions.categoryLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   catOpts,
				Selected:  catS.Get(),
				AriaLabel: uistate.T("transactions.categoryLabel"),
				OnChange:  func(v string) { catS.Set(v) },
			})),
		labeledField(uistate.T("transactions.dateLabel"),
			Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), Value(dateS.Get()), OnInput(onDate))),
		labeledField(uistate.T("transactions.tagsLabel"),
			Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("transactions.tagsLabel")), Placeholder(uistate.T("transactions.tagsPlaceholder")), Value(tagsS.Get()), OnInput(onTags))),
		If(len(members) > 1, uiw.FormField(uistate.T("transactions.whoLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   memberOpts,
				Selected:  memberS.Get(),
				AriaLabel: uistate.T("transactions.whoLabel"),
				OnChange:  func(v string) { memberS.Set(v) },
			}))),
		Label(css.Class("txn-check"),
			Input(Type("checkbox"), Attr("aria-label", uistate.T("txnwidget.clearedLabel")), CheckedIf(clearedS.Get()), OnChange(onCleared)),
			Span(uistate.T("txnwidget.clearedLabel"))),
		// Receipts: attach a new image; the count of existing receipts is shown so the
		// user can confirm it took (viewing opens from the table's row paperclip).
		Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-edit-attach"), OnClick(attach), uistate.T("transactions.attachReceiptTitle")),
			If(len(txn.Attachments) > 0, Span(css.Class("muted"), receiptCountLabel(len(txn.Attachments)))),
		),
		// C58: split-into-categories, right in the modal — the current breakdown is
		// always visible; the toggle reveals the full editor (kept as type=button
		// children so nothing here submits the form).
		Div(Attr("data-testid", "txn-edit-splits"),
			If(len(splitLines) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), splitLines)),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-edit-split-toggle"),
				Attr("aria-expanded", ariaBool(splitOpen.Get())), OnClick(toggleSplit),
				uistate.T(splitToggleKey(splitOpen.Get(), txn.HasSplits()))),
			If(splitOpen.Get(), ui.CreateElement(SplitEditor, splitEditorProps{
				Txn: txn, Categories: categories, OnSave: saveSplits,
			})),
		),
		errText("txn-edit-err", errMsg.Get()),
		// Delete stays in the body (left-aligned, apart from the panel footer's
		// Cancel/Save): the FlipPanel's pinned .set-foot owns Cancel + Save now (via
		// FormID → native submit), fixing the old floating mid-panel button row.
		Div(css.Class(tw.Flex, tw.Mt2, tw.Mb2),
			Button(css.Class("btn btn-del"), Type("button"), Attr("data-testid", "txn-edit-delete"), OnClick(del), uistate.T("action.delete")),
		),
	)
}
