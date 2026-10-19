// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

type transactionRowProps struct {
	Txn         domain.Transaction
	Account     string
	Category    string
	Categories  []domain.Category // for the edit-mode category picker
	Members     []domain.Member   // for the edit-mode "Who" picker (may be empty)
	Selected    bool
	ShowTags    bool // whether the Tags column is present this render (G2 §6)
	OnDelete    func(string)
	OnDuplicate func(domain.Transaction)
	OnSave      func(orig domain.Transaction, desc, payee, amount, categoryID, date, memberID, tags string) bool
	// OnSaveFull persists the complete edited record, including fields added to
	// the shared form. It is optional while older screen hosts are migrated.
	OnSaveFull      func(orig, updated domain.Transaction) bool
	OnToggleSelect  func(id string, shift bool)
	OnToggleCleared func(domain.Transaction)
	// OnCreateRule navigates to the Rules screen with the add-form prefilled from
	// this transaction's payee/description and category.
	OnCreateRule func(domain.Transaction)
	// OnAttach uploads a receipt image and links it to this transaction; OnViewReceipt
	// opens a preview of an attached receipt (L29).
	OnAttach      func(domain.Transaction)
	OnViewReceipt func(domain.AttachmentRef)
	// OnSaveSplits (C58) persists a category breakdown set in the inline split
	// editor; the screen wires it to PutTransaction. May be nil (no split UI).
	OnSaveSplits func(domain.Transaction)
	// Smart badge inputs: SmartSettings + byEntity index from the page's insight run.
	// The badge key is the transaction ID (Action.RelatedID set by transaction engines).
	SmartSettings smart.Settings
	SmartByEntity map[string][]smart.Insight
	// TrendMerchant is the resolved merchant name when this row's merchant has enough
	// charge history to show the spending-trend chip (TX6b); "" hides the chip. The
	// caller precomputes eligibility once per page (merchantChargeCounts) so the row
	// never scans the ledger itself.
	TrendMerchant string
	// Transaction-link overlays (XC1/XC2). GroupSize > 0 marks this row as a
	// member of an N-charge order group; GroupTotal is the group's summed amount.
	// IsRefund marks the row as the refund side of a refund pair; IsRefunded marks
	// it as an original purchase that has a paired refund. The On* callbacks drive
	// the row's link affordances (nil = affordance hidden).
	GroupSize    int
	GroupTotal   money.Money
	IsRefund     bool
	IsRefunded   bool
	OnPairRefund func(domain.Transaction)
	OnUngroup    func(domain.Transaction)
	OnUnpair     func(domain.Transaction)
}

// TransactionRow is a per-transaction row. Income/expense rows can be edited
// inline (description, amount, category, date); transfers cannot. All hooks are
// declared unconditionally so the edit toggle never reorders them.
func TransactionRow(props transactionRowProps) ui.Node {
	t := props.Txn
	amountMajor := money.FormatMinor(txnfilter.AbsAmount(t), currency.Decimals(t.Amount.Currency))
	dateISO := dateutil.FormatDate(t.Date)

	del := ui.UseEvent(Prevent(func() {
		// Capture which row holds focus now, before the confirm modal opens, so focus
		// can be restored to the next row after the delete (§6.7).
		captureRowDeleteFocus(".txn-table tbody", "tr.row")
		// C70: a transfer is two linked legs (money out + money in); deleting one removes
		// both. Warn explicitly so the user isn't surprised that the paired leg vanished.
		msg := uistate.T("transactions.deleteConfirm", t.Desc)
		if t.IsTransfer() {
			msg = uistate.T("transactions.deleteTransferConfirm", t.Desc)
		}
		uistate.ConfirmModal(msg, true, func(ok bool) {
			if ok {
				props.OnDelete(t.ID)
			}
		})
	}))
	dup := ui.UseEvent(Prevent(func() { props.OnDuplicate(t) }))
	sel := ui.UseEvent(func(e ui.Event) { // C62: pass shift for range selection
		shift := e.JSValue().Get("shiftKey").Bool()
		props.OnToggleSelect(t.ID, shift)
	})
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
	pairRefund := ui.UseEvent(Prevent(func() {
		if props.OnPairRefund != nil {
			props.OnPairRefund(t)
		}
	}))
	ungroup := ui.UseEvent(Prevent(func() {
		if props.OnUngroup != nil {
			props.OnUngroup(t)
		}
	}))
	unpair := ui.UseEvent(Prevent(func() {
		if props.OnUnpair != nil {
			props.OnUnpair(t)
		}
	}))
	pr := uistate.UsePrefs().Get()
	// Resolve the default member for this row: its first owner if
	// set, otherwise the account owner via MemberForNewTransaction.
	defaultRowMember := t.PrimaryOwner() // TODO(WHO2): the editor picker still accepts one owner.
	editing := ui.UseState(false)
	descS := ui.UseState(t.Desc)
	payeeS := ui.UseState(t.Payee) // C60: editable payee in inline edit
	amountS := ui.UseState(amountMajor)
	catS := ui.UseState(t.CategoryID)
	dateS := ui.UseState(dateISO)
	memberS := ui.UseState(defaultRowMember)
	tagsS := ui.UseState(strings.Join(t.Tags, ", ")) // C48: editable tags (comma-separated)
	directionS := ui.UseState(txnDirection(t))
	clearedS := ui.UseState(t.Cleared)
	excludeS := ui.UseState(t.ExcludeFromReports)
	noteS := ui.UseState(transactionNote(t))
	splitOpen := ui.UseState(false) // C58: reveal the split-into-categories editor
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onPayee := ui.UseEvent(func(v string) { payeeS.Set(v) }) // C60
	onAmount := ui.UseEvent(func(v string) { amountS.Set(v) })
	onTags := ui.UseEvent(func(v string) { tagsS.Set(v) })
	onNote := ui.UseEvent(func(v string) { noteS.Set(v) })
	onCleared := ui.UseEvent(func(e ui.Event) { clearedS.Set(e.JSValue().Get("target").Get("checked").Bool()) })
	onExclude := ui.UseEvent(func(e ui.Event) { excludeS.Set(e.JSValue().Get("target").Get("checked").Bool()) })
	setExpense := ui.UseEvent(Prevent(func() { directionS.Set("expense") }))
	setIncome := ui.UseEvent(Prevent(func() { directionS.Set("income") }))
	applySuggestion := ui.UseEvent(Prevent(func() {
		candidate := t
		candidate.Amount = money.New(signedEditAmount(amountS.Get(), directionS.Get(), t.Amount.Currency), t.Amount.Currency)
		if s, ok := reviewSuggestion(appstate.Default, candidate); ok {
			catS.Set(s.CategoryID)
		}
	}))
	toggleSplit := ui.UseEvent(Prevent(func() { splitOpen.Set(!splitOpen.Get()) })) // C58
	// onCat and onMember hooks are preserved as anonymous stubs so the hook slot
	// order is stable; the actual onChange is wired through uiw.SelectInput below.
	ui.UseEvent(func(e ui.Event) {})
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	ui.UseEvent(func(e ui.Event) {})
	startEdit := ui.UseEvent(Prevent(func() {
		descS.Set(t.Desc)
		payeeS.Set(t.Payee)
		amountS.Set(amountMajor)
		catS.Set(t.CategoryID)
		dateS.Set(dateISO)
		memberS.Set(defaultRowMember)
		tagsS.Set(strings.Join(t.Tags, ", "))
		directionS.Set(txnDirection(t))
		clearedS.Set(t.Cleared)
		excludeS.Set(t.ExcludeFromReports)
		noteS.Set(transactionNote(t))
		splitOpen.Set(false)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		// C59: only close the editor when OnSave signals success (returns true);
		// a validation failure (e.g. amount = 0) keeps the form open so the user
		// can correct the value.
		saved := false
		if props.OnSaveFull != nil {
			updated := t
			updated.Desc, updated.Payee = descS.Get(), payeeS.Get()
			updated.CategoryID, updated.Date = catS.Get(), t.Date
			updated.SetOwners([]string{memberS.Get()}) // TODO(WHO2): inline picker still assigns one owner.
			updated.Tags = splitTags(tagsS.Get())
			updated.Cleared, updated.ExcludeFromReports = clearedS.Get(), excludeS.Get()
			updated.Custom = cloneTransactionCustom(t.Custom)
			if strings.TrimSpace(noteS.Get()) == "" {
				delete(updated.Custom, "note")
			} else {
				updated.Custom["note"] = strings.TrimSpace(noteS.Get())
			}
			updated.Amount = money.New(signedEditAmount(amountS.Get(), directionS.Get(), t.Amount.Currency), t.Amount.Currency)
			if d, err := dateutil.ParseDate(strings.TrimSpace(dateS.Get())); err == nil {
				updated.Date = d
			}
			saved = props.OnSaveFull(t, updated)
		} else if directionS.Get() == txnDirection(t) && clearedS.Get() == t.Cleared && excludeS.Get() == t.ExcludeFromReports && noteS.Get() == transactionNote(t) {
			saved = props.OnSave(t, descS.Get(), payeeS.Get(), amountS.Get(), catS.Get(), dateS.Get(), memberS.Get(), tagsS.Get())
		} else {
			uistate.PostNotice(uistate.T("r2e.fullEditUnavailable"), true)
		}
		if saved {
			editing.Set(false)
		}
	}))
	onEditKeyDown := ui.UseEvent(func(e ui.KeyboardEvent) {
		if e.GetKey() == "Escape" {
			e.PreventDefault()
			editing.Set(false)
		}
	})

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
		candidate := t
		candidate.Amount = money.New(signedEditAmount(amountS.Get(), directionS.Get(), t.Amount.Currency), t.Amount.Currency)
		suggested, hasSuggestion := reviewSuggestion(appstate.Default, candidate)
		suggestionName := ""
		if hasSuggestion {
			if c, ok := categoryByID(props.Categories, suggested.CategoryID); ok {
				suggestionName = c.Name
			} else {
				hasSuggestion = false
			}
		}
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
		primary := []ui.Node{
			labeledField(uistate.T("transactions.amountPlaceholder"), Input(css.Class("field"), Type("number"), Step("0.01"), OnInput(onAmount), uiw.FieldValue(amountS.Get()))),
			uiw.FormField(uistate.T("quickAdd.kind"), Div(css.Class("btn-group"),
				Button(ClassStr("btn"), Type("button"), Attr("aria-label", uistate.T("quickAdd.expense")), Attr("aria-pressed", ariaBool(directionS.Get() == "expense")), OnClick(setExpense), uistate.T("quickAdd.expense")),
				Button(ClassStr("btn"), Type("button"), Attr("aria-label", uistate.T("quickAdd.income")), Attr("aria-pressed", ariaBool(directionS.Get() == "income")), OnClick(setIncome), uistate.T("quickAdd.income")),
			)),
			labeledField(uistate.T("transactions.payeeLabel"), Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-edit-payee"), Placeholder(uistate.T("transactions.payeeLabel")), OnInput(onPayee), uiw.FieldValue(payeeS.Get()))),
			uiw.FormField(uistate.T("transactions.categoryLabel"), Fragment(
				If(hasSuggestion && suggested.CategoryID != catS.Get(), Button(css.Class("txn-catsuggest"), Type("button"), Attr("data-testid", "txn-edit-category-suggestion"), OnClick(applySuggestion), uistate.T("r2e.useSuggestion", suggestionName))),
				uiw.SelectInput(uiw.SelectInputProps{Options: catOpts, Selected: catS.Get(), AriaLabel: uistate.T("transactions.categoryLabel"), OnChange: func(v string) { catS.Set(v) }}),
			)),
			labeledField(uistate.T("transactions.dateLabel"), Input(css.Class("field"), Type("date"), OnInput(onDate), uiw.FieldValue(dateS.Get()))),
			labeledField(uistate.T("quickAdd.account"), Input(css.Class("field"), Type("text"), Attr("readonly", "readonly"), Attr("aria-readonly", "true"), uiw.FieldValue(props.Account))),
		}
		more := []ui.Node{
			labeledField(uistate.T("transactions.descPlaceholder"), Input(css.Class("field"), Attr("id", "txn-edit-"+t.ID), Type("text"), Placeholder(uistate.T("transactions.descPlaceholder")), OnInput(onDesc), uiw.FieldValue(descS.Get()))),
			labeledField(uistate.T("transactions.tagsLabel"), Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-edit-tags"), Placeholder(uistate.T("transactions.tagsPlaceholder")), OnInput(onTags), uiw.FieldValue(tagsS.Get()))),
			If(len(props.Members) > 1, uiw.FormField(uistate.T("transactions.whoLabel"), uiw.SelectInput(uiw.SelectInputProps{Options: memberOpts, Selected: memberS.Get(), AriaLabel: uistate.T("transactions.whoLabel"), TestID: "txn-who-edit", OnChange: func(v string) { memberS.Set(v) }}))),
			labeledField(uistate.T("quickAdd.note"), Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-edit-note"), OnInput(onNote), uiw.FieldValue(noteS.Get()))),
			Label(Input(Type("checkbox"), Checked(clearedS.Get()), OnChange(onCleared)), " "+uistate.T("quickAdd.cleared")),
			Label(Input(Type("checkbox"), Checked(excludeS.Get()), OnChange(onExclude)), " "+uistate.T("quickAdd.exclude")),
		}
		entries := make([]ui.Node, 0, len(t.Splits)+len(t.Attachments))
		for _, split := range t.Splits {
			name := catname.LabelByID(props.Categories, split.CategoryID)
			entries = append(entries, Div(Attr("data-testid", "txn-edit-split-entry"), name+" · "+fmtMoney(split.Amount)))
		}
		if len(t.Attachments) > 0 {
			for _, receipt := range t.Attachments {
				entries = append(entries, Div(Attr("data-testid", "txn-edit-receipt-entry"), receipt.Name))
			}
		}
		return Tr(css.Class("row-edit"), Td(Attr("colspan", editColspan),
			Form(OnSubmit(saveEdit), OnKeyDown(onEditKeyDown),
				Div(css.Class("txn-bank-text"), uistate.T("r2e.bankText", t.Desc)),
				TxnSharedForm(TxnSharedFormProps{Primary: primary, More: more, AfterMore: entries, MoreID: "txn-edit-more-" + t.ID}),
				If(props.OnSaveSplits != nil, Fragment(
					Button(css.Class("btn", "btn-sm"), Type("button"), Attr("data-testid", "txn-split-toggle"), OnClick(toggleSplit), uistate.T(splitToggleKey(splitOpen.Get(), t.HasSplits()))),
					If(splitOpen.Get(), ui.CreateElement(SplitEditor, splitEditorProps{Txn: t, Categories: props.Categories, Members: props.Members, OnSave: props.OnSaveSplits})),
				)),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
			)),
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
	// XC1/XC2 link badges shown in the Description cell.
	var linkBadge ui.Node = Fragment()
	switch {
	case props.GroupSize > 1:
		title := uistate.T("txnlinks.groupBadgeTitle", props.GroupSize, fmtMoney(props.GroupTotal))
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-group-badge"), Attr("title", title),
			"◱ "+uistate.T("txnlinks.groupBadge", props.GroupSize))
	case props.IsRefund:
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-refund-badge"), Attr("title", uistate.T("txnlinks.refundBadge")),
			"↩ "+uistate.T("txnlinks.refundBadge"))
	case props.IsRefunded:
		linkBadge = Span(css.Class("badge"), Attr("data-testid", "txn-refunded-badge"), Attr("title", uistate.T("txnlinks.refundedBadge")),
			"↩ "+uistate.T("txnlinks.refundedBadge"))
	}

	// A grouped row reads as a physical grouping: a quiet accent tie-line on the
	// left with a small indent (the one signature moment; everything else stays
	// calm). Members share the same left rail so the eye ties them together.
	rowArgs := []any{ClassStr(rowClass), Attr("data-id", props.Txn.ID)}
	if props.GroupSize > 1 {
		rowArgs = append(rowArgs, Style(map[string]string{
			"box-shadow":   "inset 3px 0 0 0 var(--accent)",
			"padding-left": "0.25rem",
		}))
	}
	amountText, amountTone := fmtLedgerAmount(props.Txn.Amount, props.Txn.IsTransfer())
	rowArgs = append(rowArgs,
		// C65: the row-select control was a bare glyph button — no accessible name, no
		// pressed state, no row context. Give it an aria-label naming the row and an
		// aria-pressed reflecting selection so screen-reader users know what they're
		// toggling and its current state. The glyph itself is decorative.
		Td(css.Class("td-select"), Button(css.Class("check"), Type("button"),
			Title(uistate.T("transactions.selectTitle")),
			Attr("aria-label", uistate.T("transactions.selectRow", rowSelectName(props.Txn))),
			Attr("aria-pressed", ariaBool(props.Selected)),
			OnClick(sel), Span(Attr("aria-hidden", "true"), selectGlyph))),
		Td(css.Class("td-date fig"), pr.FormatDate(props.Txn.Date)),
		Td(ClassStr("td-amount fig "+tw.ColorClass(amountTone)), amountText),
		Td(css.Class("row-desc"), Span(Attr("title", props.Txn.Desc), Attr("aria-label", props.Txn.Desc), props.Txn.Desc), descTags,
			// C58-ui: show a small "Split" chip when the transaction has a category
			// breakdown, so the user can see at a glance which rows are split without
			// having to open the inline editor.
			If(t.HasSplits(), Span(css.Class("badge badge-split"), Attr("data-testid", "txn-split-badge"), Attr("title", uistate.T("transactions.splitBadgeTitle")), "⑂ "+uistate.T("transactions.splitBadge"))),
			smartBadgeFor(props.SmartSettings, props.SmartByEntity, t.ID), linkBadge,
			// TX6b: a spending-trend chip when this merchant has history — opens the
			// merchant story (sparkline + this-vs-typical) that used to hide in the editor.
			If(props.TrendMerchant != "", ui.CreateElement(merchantTrendChip, merchantTrendChipProps{
				Merchant: props.TrendMerchant, TxnID: t.ID, Amount: props.Txn.Amount,
			}))),
		// Title mirrors the text: long names ("Education & Loans") ellipsize in
		// their fixed columns with no way to read the rest (UI/UX task #37).
		Td(css.Class("td-cat"), Title(cat), Attr("aria-label", cat), cat),
		Td(css.Class("td-acct"), Title(props.Account), props.Account),
		If(props.ShowTags, Td(css.Class("td-tags"), tagsText)),
	)
	rowArgs = append(rowArgs,
		Td(css.Class("td-cleared"), Button(ClassStr(clearedCls), Type("button"), Title(clearedTitle), Attr("aria-pressed", ariaBool(t.Cleared)), Attr("aria-label", clearedTitle), OnClick(clr), clearedGlyph)),
		Td(css.Class("td-actions"),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon"), Type("button"), Attr("aria-label", uistate.T("transactions.editTitle")), Title(uistate.T("transactions.editTitle")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("transactions.duplicateTitle")), Title(uistate.T("transactions.duplicateTitle")), OnClick(dup), uiw.Icon(icon.Copy, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("transactions.createRuleTitle")), Title(uistate.T("transactions.createRuleTitle")), Attr("data-testid", "txn-create-rule"), OnClick(createRule), uiw.Icon(icon.Filter, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(!props.Txn.IsTransfer(), Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("transactions.attachReceiptTitle")), Title(uistate.T("transactions.attachReceiptTitle")), Attr("data-testid", "txn-attach"), OnClick(attach), uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			If(len(props.Txn.Attachments) > 0, Button(css.Class("btn btn-icon", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("aria-label", receiptCountLabel(len(props.Txn.Attachments))), Title(receiptCountLabel(len(props.Txn.Attachments))), Attr("data-testid", "txn-attach-marker"), OnClick(viewReceipt), uiw.Icon(icon.Paperclip, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(strconv.Itoa(len(props.Txn.Attachments))))),
			// XC2: pair a positive (money-in) transaction as the refund of a purchase.
			If(props.OnPairRefund != nil && props.Txn.IsIncome() && !props.IsRefund,
				Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("txnlinks.pairAction")), Title(uistate.T("txnlinks.pairAction")), Attr("data-testid", "txn-pair-refund"), OnClick(pairRefund), uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			// XC2: remove an existing refund pairing (shown on either side of the pair).
			If(props.OnUnpair != nil && (props.IsRefund || props.IsRefunded),
				Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("txnlinks.unpairAction")), Title(uistate.T("txnlinks.unpairAction")), Attr("data-testid", "txn-unpair"), OnClick(unpair), uiw.Icon(icon.Repeat, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			// XC1: ungroup an order-group member (releases the group; keeps the atoms).
			If(props.OnUngroup != nil && props.GroupSize > 1,
				Button(css.Class("btn btn-icon tx-2nd"), Type("button"), Attr("aria-label", uistate.T("txnlinks.ungroupAction")), Title(uistate.T("txnlinks.ungroupAction")), Attr("data-testid", "txn-ungroup"), OnClick(ungroup), uiw.Icon(icon.Box, css.Class(tw.ShrinkO, tw.W4, tw.H4)))),
			Button(css.Class("btn-del tx-2nd"), Type("button"), Attr("aria-label", uistate.T("transactions.deleteTitle")), Title(uistate.T("transactions.deleteTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
	)
	return Tr(rowArgs...)
}

// receiptCountLabel is the plain-English label for a transaction's attached
// receipts, e.g. "1 receipt attached" / "3 receipts attached" (L29).
func receiptCountLabel(n int) string {
	if n == 1 {
		return uistate.T("transactions.receiptAttached", n)
	}
	return uistate.T("transactions.receiptsAttached", n)
}

// rowSelectName builds a concise human label for a transaction's select control
// (C65): prefer the payee, fall back to the description, then the amount, so the
// accessible name names the actual row instead of an opaque glyph.
func rowSelectName(t domain.Transaction) string {
	if s := strings.TrimSpace(t.Payee); s != "" {
		return s
	}
	if s := strings.TrimSpace(t.Desc); s != "" {
		return s
	}
	return fmtMoney(t.Amount)
}

func txnDirection(t domain.Transaction) string {
	if t.Amount.IsPositive() {
		return "income"
	}
	return "expense"
}

func signedEditAmount(raw, direction, currencyCode string) int64 {
	amount, err := money.ParseMinor(strings.TrimSpace(raw), currency.Decimals(currencyCode))
	if err != nil {
		return 0
	}
	if amount < 0 {
		amount = -amount
	}
	if direction == "income" {
		return amount
	}
	return -amount
}

func splitTags(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' })
	seen := make(map[string]bool, len(parts))
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		key := strings.ToLower(tag)
		if tag != "" && !seen[key] {
			seen[key] = true
			tags = append(tags, tag)
		}
	}
	return tags
}

func transactionNote(t domain.Transaction) string {
	if t.Custom == nil {
		return ""
	}
	if note, ok := t.Custom["note"].(string); ok {
		return note
	}
	return ""
}

func cloneTransactionCustom(custom map[string]any) map[string]any {
	copy := make(map[string]any, len(custom)+1)
	for key, value := range custom {
		copy[key] = value
	}
	return copy
}
