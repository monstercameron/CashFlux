// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

type transactionRowProps struct {
	Txn             domain.Transaction
	Account         string
	Category        string
	Categories      []domain.Category // for the edit-mode category picker
	Members         []domain.Member   // for the edit-mode "Who" picker (may be empty)
	Selected        bool
	ShowTags        bool // whether the Tags column is present this render (G2 §6)
	OnDelete        func(string)
	OnDuplicate     func(domain.Transaction)
	OnSave          func(orig domain.Transaction, desc, amount, categoryID, date, memberID string)
	OnToggleSelect  func(string)
	OnToggleCleared func(domain.Transaction)
	// OnCreateRule navigates to the Rules screen with the add-form prefilled from
	// this transaction's payee/description and category.
	OnCreateRule func(domain.Transaction)
	// OnAttach uploads a receipt image and links it to this transaction; OnViewReceipt
	// opens a preview of an attached receipt (L29).
	OnAttach      func(domain.Transaction)
	OnViewReceipt func(domain.AttachmentRef)
}

// TransactionRow is a per-transaction row. Income/expense rows can be edited
// inline (description, amount, category, date); transfers cannot. All hooks are
// declared unconditionally so the edit toggle never reorders them.
func TransactionRow(props transactionRowProps) ui.Node {
	t := props.Txn
	amountMajor := money.FormatMinor(txnfilter.AbsAmount(t), currency.Decimals(t.Amount.Currency))
	dateISO := dateutil.FormatDate(t.Date)

	del := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("transactions.deleteConfirm", t.Desc), true, func(ok bool) {
			if ok {
				props.OnDelete(t.ID)
			}
		})
	}))
	dup := ui.UseEvent(Prevent(func() { props.OnDuplicate(t) }))
	sel := ui.UseEvent(Prevent(func() { props.OnToggleSelect(t.ID) }))
	clr := ui.UseEvent(Prevent(func() { props.OnToggleCleared(t) }))
	createRule := ui.UseEvent(Prevent(func() {
		if props.OnCreateRule != nil {
			props.OnCreateRule(t)
		}
	}))
	attach := ui.UseEvent(Prevent(func() {
		if props.OnAttach != nil {
			props.OnAttach(t)
		}
	}))
	viewReceipt := ui.UseEvent(Prevent(func() {
		if props.OnViewReceipt != nil && len(t.Attachments) > 0 {
			props.OnViewReceipt(t.Attachments[0])
		}
	}))
	pr := uistate.UsePrefs().Get()
	// Resolve the default member for this row: the transaction's own MemberID if
	// set, otherwise the account owner via MemberForNewTransaction.
	defaultRowMember := t.MemberID
	editing := ui.UseState(false)
	descS := ui.UseState(t.Desc)
	amountS := ui.UseState(amountMajor)
	catS := ui.UseState(t.CategoryID)
	dateS := ui.UseState(dateISO)
	memberS := ui.UseState(defaultRowMember)
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	// onCat and onMember hooks are preserved as anonymous stubs so the hook slot
	// order is stable; the actual onChange is wired through uiw.SelectInput below.
	ui.UseEvent(func(e ui.Event) {})
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	ui.UseEvent(func(e ui.Event) {})
	startEdit := ui.UseEvent(Prevent(func() {
		descS.Set(t.Desc)
		amountS.Set(amountMajor)
		catS.Set(t.CategoryID)
		dateS.Set(dateISO)
		memberS.Set(defaultRowMember)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnSave(t, descS.Get(), amountS.Get(), catS.Get(), dateS.Get(), memberS.Get())
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID("txn-edit-" + t.ID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		catOpts := uiw.OptionsFrom(
			props.Categories,
			func(c domain.Category) string { return c.ID },
			func(c domain.Category) string { return c.Name },
			catS.Get(),
		)
		catOpts = append([]uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}}, catOpts...)

		memberOpts := uiw.OptionsFrom(
			props.Members,
			func(m domain.Member) string { return m.ID },
			func(m domain.Member) string { return m.Name },
			memberS.Get(),
		)

		editColspan := "8"
		if props.ShowTags {
			editColspan = "9"
		}
		// GM2-4: inline-edit had 0 labeled-field wrappers (description + amount were
		// placeholder-only, invisible to screen readers once text is entered). Wrap
		// each in labeledField() to match the entity add-form pattern.
		return Tr(css.Class("row-edit"),
			Td(Attr("colspan", editColspan),
				Form(css.Class("form-grid"), OnSubmit(saveEdit),
					labeledField(uistate.T("transactions.descPlaceholder"),
						Input(css.Class("field"), Attr("id", "txn-edit-"+t.ID), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), Value(descS.Get()), OnInput(onDesc))),
					labeledField(uistate.T("transactions.amountPlaceholder"),
						Input(css.Class("field"), Type("number"), Placeholder(uistate.T("transactions.amountPlaceholder")), Value(amountS.Get()), Step("0.01"), OnInput(onAmount))),
					uiw.FormField(uistate.T("transactions.categoryLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   catOpts,
							Selected:  catS.Get(),
							AriaLabel: uistate.T("transactions.categoryLabel"),
							OnChange:  func(v string) { catS.Set(v) },
						})),
					Input(css.Class("field"), Type("date"), Attr("aria-label", uistate.T("transactions.dateLabel")), Value(dateS.Get()), OnInput(onDate)),
					If(len(props.Members) > 1, uiw.FormField(uistate.T("transactions.whoLabel"),
						uiw.SelectInput(uiw.SelectInputProps{
							Options:   memberOpts,
							Selected:  memberS.Get(),
							AriaLabel: uistate.T("transactions.whoLabel"),
							TestID:    "txn-who-edit",
							OnChange:  func(v string) { memberS.Set(v) },
						}))),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
					Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
				),
			),
		)
	}

	cat := props.Category
	switch {
	case props.Txn.IsTransfer():
		cat = uistate.T("transactions.transfer")
	case cat == "":
		cat = uistate.T("transactions.uncategorized")
	}
	tagsText := ""
	if len(props.Txn.Tags) > 0 {
		tagsText = "#" + strings.Join(props.Txn.Tags, " #")
	}

	selectGlyph := "☐"
	if props.Selected {
		selectGlyph = "☑"
	}
	rowClass := "row"
	if props.Selected {
		rowClass += " selected"
	}
	if t.Cleared {
		rowClass += " cleared"
	}
	// Cleared state reads as a distinct icon button (G2 §5): a green ✓ when cleared
	// (click to unclear), a dim ○ when not (click to clear) — so Nadia can tell
	// reconciled rows apart at a glance instead of every cell saying "Mark cleared".
	clearedGlyph, clearedTitle := "○", uistate.T("transactions.markCleared")
	clearedCls := "clr-toggle"
	if t.Cleared {
		clearedGlyph, clearedTitle = "✓", uistate.T("transactions.clearedCheck")
		clearedCls = "clr-toggle is-cleared"
	}
	// Tags collapse to an inline #chip on the Description cell when the column is
	// hidden, so a tagged row still shows its tags (G2 §6).
	var descTags ui.Node = Fragment()
	if !props.ShowTags && tagsText != "" {
		descTags = Span(css.Class("td-tags-inline"), " "+tagsText)
	}
	return Tr(ClassStr(rowClass), Attr("data-id", props.Txn.ID),
		Td(css.Class("td-select"), Button(css.Class("check"), Type("button"), Title(uistate.T("transactions.selectTitle")), OnClick(sel), selectGlyph)),
		Td(css.Class("td-date fig"), pr.FormatDate(props.Txn.Date)),
		Td(ClassStr("td-amount fig "+amountClass(props.Txn.Amount)), fmtMoney(props.Txn.Amount)),
		Td(css.Class("row-desc"), Span(props.Txn.Desc), descTags),
		Td(css.Class("td-cat"), cat),
		Td(css.Class("td-acct"), props.Account),
		If(props.ShowTags, Td(css.Class("td-tags"), tagsText)),
		Td(css.Class("td-cleared"), Button(ClassStr(clearedCls), Type("button"), Title(clearedTitle), Attr("aria-pressed", ariaBool(t.Cleared)), Attr("aria-label", clearedTitle), OnClick(clr), clearedGlyph)),
		Td(css.Class("td-actions"),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon"), Type("button"), Attr("aria-label", uistate.T("transactions.editTitle")), Title(uistate.T("transactions.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon"), Type("button"), Attr("aria-label", uistate.T("transactions.duplicateTitle")), Title(uistate.T("transactions.duplicateTitle")), OnClick(dup), uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon"), Type("button"), Attr("aria-label", uistate.T("transactions.createRuleTitle")), Title(uistate.T("transactions.createRuleTitle")), Attr("data-testid", "txn-create-rule"), OnClick(createRule), uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon"), Type("button"), Attr("aria-label", uistate.T("transactions.attachReceiptTitle")), Title(uistate.T("transactions.attachReceiptTitle")), Attr("data-testid", "txn-attach"), OnClick(attach), uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(len(props.Txn.Attachments) > 0, Button(css.Class("btn btn-icon", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("aria-label", receiptCountLabel(len(props.Txn.Attachments))), Title(receiptCountLabel(len(props.Txn.Attachments))), Attr("data-testid", "txn-attach-marker"), OnClick(viewReceipt), uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(strconv.Itoa(len(props.Txn.Attachments))))),
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("transactions.deleteTitle")), Title(uistate.T("transactions.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
}

// receiptCountLabel is the plain-English label for a transaction's attached
// receipts, e.g. "1 receipt attached" / "3 receipts attached" (L29).
func receiptCountLabel(n int) string {
	if n == 1 {
		return uistate.T("transactions.receiptAttached", n)
	}
	return uistate.T("transactions.receiptsAttached", n)
}
