// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/txnform"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/router"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

// merchantContextPanel renders the TX6 merchant "story" inside the edit modal:
// typical amount, this charge vs typical, visits this week/month, this month vs a
// typical month, and a tiny sparkline of recent charges. It is read-only and
// quiet, and returns an empty fragment for transfers or one-off merchants (fewer
// than merchantstats.MinCharges charges), so it never nags. All amounts are
// converted to the household base currency via the FX table so a multi-currency
// history reads as one figure.
func merchantContextPanel(app *appstate.App, txn domain.Transaction) ui.Node {
	view := txnform.MerchantContext(txn, app.Transactions(), app.PayeeResolver(), app.Settings().BaseCurrency, app.Settings().FXRates, time.Now(), uistate.LoadPrefs().WeekStartWeekday())
	if !view.Show {
		return Fragment()
	}
	return Div(css.Class("card", tw.P3, tw.FlexCol, tw.Gap15), Attr("data-testid", "merchant-context-panel"),
		merchantStoryNodes(view.Stats, view.Merchant, view.Base, view.ThisMagnitude, view.HasThis))
}

// sparklineSVG draws a tiny polyline of a series with no chart library — a bare
// SVG scaled to its min/max. A series of one point (or all-equal values) draws a
// flat baseline. alt is the accessible name — every call site says what ITS
// sparkline shows (QA CF-30: the report's net-worth shape used to announce
// "Recent charges at this merchant").
func sparklineSVG(series []int64, alt string) ui.Node {
	points := txnform.SparklinePoints(series)
	if points == "" {
		return Fragment()
	}

	return Svg(
		Attr("viewBox", "0 0 120 24"),
		Attr("width", "120"), Attr("height", "24"),
		Attr("role", "img"), Attr("aria-label", alt),
		Attr("preserveAspectRatio", "none"),
		Polyline(Attr("points", points),
			Attr("fill", "none"), Attr("stroke", "var(--accent)"), Attr("stroke-width", "1.5")),
	)
}

// receiptThumbnails renders the transaction's attached receipts as small
// clickable thumbnails (TX5). Each thumbnail is its own component so its click
// handler is loop-hook-safe. Bytes are resolved from the live artifact set.
func receiptThumbnails(app *appstate.App, txn domain.Transaction) ui.Node {
	var thumbs []ui.Node
	for _, thumb := range txnform.Thumbnails(txn.Attachments, app.Artifacts()) {
		thumbs = append(thumbs, ui.CreateElement(receiptThumb, receiptThumbProps{Ref: thumb.Ref, DataURL: thumb.DataURL}))
	}

	return Div(css.Class(tw.Flex, tw.Gap2), Attr("data-testid", "txn-receipt-thumbs"), thumbs)
}

// receiptThumbProps configures one receipt thumbnail.
type receiptThumbProps struct {
	Ref     domain.AttachmentRef
	DataURL string // empty when the artifact bytes aren't available
}

// receiptThumb is a single receipt thumbnail: an image button that opens the
// shared receipt preview overlay (the same full-view the table's paperclip uses)
// on click. Owning its own OnClick hook keeps it safe inside the attachments loop.
func receiptThumb(props receiptThumbProps) ui.Node {
	preview := uistate.UseTxnPreview()
	ref := props.Ref
	open := ui.UseEvent(Prevent(func() { preview.Set(ref) }))
	label := uistate.T("merchantPanel.openReceipt", firstNonEmpty(ref.Name, ref.ArtifactID))
	inner := Span(css.Class("muted", tw.P1), "📎")
	if props.DataURL != "" {
		inner = Img(Attr("src", props.DataURL), Attr("alt", label),
			css.Class(tw.W10, tw.H10), Attr("style", "object-fit:cover;border-radius:4px;"))
	}
	return Button(css.Class("btn btn-icon"), Type("button"), Attr("data-testid", "txn-receipt-thumb"),
		Attr("aria-label", label), Title(label), OnClick(open), inner)
}

// TransactionEditFormProps configures the TransactionEditForm component.
type TransactionEditFormProps struct {
	// App supplies the container-owned data source; nil uses the shell default.
	App *appstate.App
	// TxnID is the id of the transaction being edited; looked up live from appstate.
	TxnID string
	// OnDone is called after a successful save or delete (and on Cancel) so the
	// caller (TxnEditHost) can close the modal. On a validation error the form
	// stays open and OnDone is not called.
	OnDone func()
}

// TransactionEditForm is the standalone edit-a-transaction form used by the
// TxnEditHost modal (the description button and pencil open this editor). It looks the transaction up by id, seeds its fields, validates on
// save, and persists via appstate. Mirrors the inline edit controls in
// transactions_row.go and the add-form submit/error pattern.
func TransactionEditForm(props TransactionEditFormProps) ui.Node {
	if props.App == nil {
		props.App = appstate.Default
	}
	if props.App == nil {
		return P(css.Class("empty"), uistate.T("common.notReady"))
	}
	return ui.CreateElement(transactionEditForm, props)
}

type txnEditState struct {
	app                                                                 *appstate.App
	props                                                               TransactionEditFormProps
	txn                                                                 domain.Transaction
	found                                                               bool
	categories                                                          []domain.Category
	members                                                             []domain.Member
	merchantPanel                                                       ui.Node
	descS, payeeS, amountS, directionS                                  ui.State[string]
	catS, dateS, tagsS, noteS                                           ui.State[string]
	transferToS, accountS, errMsg                                       ui.State[string]
	memberS                                                             ui.State[[]string]
	categoryPickerOpen, clearedS, excludeS, debtPayS, saving, splitOpen ui.State[bool]
	saveLease                                                           *txnFormLease
	simState                                                            ui.State[[]similartxns.Candidate]
	simTarget                                                           ui.State[domain.Transaction]
}

func useTxnEditState(props TransactionEditFormProps) txnEditState {
	app := props.App
	txn, found := app.TransactionByID(props.TxnID)
	draft := txnform.Seed(txn, found)
	s := txnEditState{app: app, props: props, txn: txn, found: found, categories: app.Categories(), members: app.Members()}
	s.merchantPanel = ui.UseMemo(func() ui.Node { return merchantContextPanel(app, txn) }, app, app.Rev(), txn.ID)
	s.descS = ui.UseState(draft.Desc)
	s.payeeS = ui.UseState(draft.Payee)
	s.amountS = ui.UseState(draft.Amount)
	s.directionS = ui.UseState(draft.Direction)
	s.catS = ui.UseState(draft.CategoryID)
	s.dateS = ui.UseState(draft.Date)
	s.memberS = ui.UseState(draft.Owners)
	s.tagsS = ui.UseState(draft.Tags)
	s.clearedS = ui.UseState(draft.Cleared)
	s.noteS = ui.UseState(draft.Note)
	s.excludeS = ui.UseState(draft.Exclude)
	s.transferToS = ui.UseState(draft.TransferAccountID)
	s.debtPayS = ui.UseState(draft.DebtPayment)
	s.accountS = ui.UseState(draft.AccountID)
	s.errMsg = ui.UseState("")
	s.saving = ui.UseState(false)
	s.categoryPickerOpen = ui.UseState(false)
	s.splitOpen = ui.UseState(false)
	s.saveLease = useTxnFormLease(props.TxnID)
	ui.UseEffect(func() func() { s.saving.Set(false); return nil }, props.TxnID)
	s.simState = ui.UseState[[]similartxns.Candidate](nil)
	s.simTarget = ui.UseState(domain.Transaction{})
	return s
}

func (s txnEditState) draft() txnform.Draft {
	return txnform.Draft{Desc: s.descS.Get(), Payee: s.payeeS.Get(), Amount: s.amountS.Get(), Direction: s.directionS.Get(),
		CategoryID: s.catS.Get(), Date: s.dateS.Get(), Owners: s.memberS.Get(), Tags: s.tagsS.Get(), Cleared: s.clearedS.Get(),
		Note: s.noteS.Get(), Exclude: s.excludeS.Get(), TransferAccountID: s.transferToS.Get(), DebtPayment: s.debtPayS.Get(), AccountID: s.accountS.Get()}
}

func transactionEditForm(props TransactionEditFormProps) ui.Node {
	s := useTxnEditState(props)
	return s.render()
}

func (s txnEditState) render() ui.Node {
	prefs := uistate.UsePrefs().Get()
	save := ui.UseEvent(Prevent(s.save))
	cancel := ui.UseEvent(Prevent(s.done))
	applySimilar := ui.UseEvent(s.applySimilar)
	alwaysRule := ui.UseEvent(s.alwaysRule)
	dismissSimilar := ui.UseEvent(s.dismissSimilar)
	amountError, dateError := txnform.FieldErrors(s.errMsg.Get(), uistate.T("transactions.positiveAmount"), uistate.T("transactions.amountNotANumber"), uistate.T("transactions.invalidDate"))
	amountField := ui.CreateElement(txnEditAmount, txnEditInputProps{Value: s.amountS.Get(), Error: amountError, OnInput: s.amountS.Set})
	directionField := ui.CreateElement(txnEditDirection, txnEditDirectionProps{Value: s.directionS.Get(), OnSelect: s.directionS.Set})
	payeeField := ui.CreateElement(txnEditPayee, txnEditInputProps{Value: s.payeeS.Get(), OnInput: s.payeeS.Set})
	dateField := ui.CreateElement(txnEditDate, txnEditInputProps{Value: s.dateS.Get(), Error: dateError, OnInput: s.dateS.Set})
	categoryPicker := s.categoryNode()
	accountControl := ui.CreateElement(txnEditAccount, txnEditAccountProps{App: s.app, Txn: s.txn, Selected: s.accountS.Get(), OnSelect: s.accountS.Set})
	more := s.moreNodes()
	rowActions := s.toolsNode()
	attachments := ui.CreateElement(txnEditAttachments, txnEditAttachmentsProps{App: s.app, Txn: s.txn})
	splits := ui.CreateElement(txnEditSplits, txnEditSplitsProps{Txn: s.txn, Categories: s.categories, Members: s.members, Open: s.splitOpen.Get(), Pending: s.saving.Get(), OnSave: s.saveSplits})
	cands := s.simState.Get()
	var pendingInert any
	if s.saving.Get() {
		pendingInert = Attr("inert", "")
	}
	heading, subtitle := txnform.Heading(s.txn)
	form := Form(css.Class("txn-form-root", "txn-edit"), pendingInert, Attr("id", "txn-edit-form"), Attr("data-testid", "txn-edit-form"), Attr("aria-busy", strconv.FormatBool(s.saving.Get())), OnSubmit(save),
		If(s.saving.Get(), P(Attr("role", "status"), Attr("data-testid", "txn-save-pending"), uistate.T("common.saving"))),
		Div(css.Class("txn-form-record"), Attr("data-testid", "txn-edit-record"),
			Div(css.Class("txn-form-record-meta"),
				Span(css.Class("txn-form-record-name"), heading),
				If(subtitle != "", Span(css.Class("txn-form-help", "txn-form-record-subtitle"), subtitle)),
				Span(css.Class("txn-form-help"), prefs.FormatDate(s.txn.Date)))),
		TxnSharedForm(TxnFormModel{Mode: TxnFormEdit, Amount: amountField, Direction: directionField, Payee: payeeField,
			Category: categoryPicker, Date: dateField, Account: accountControl, More: more, MoreID: "txn-edit-more-" + s.txn.ID,
			EditTools: rowActions, EditSummary: Div(css.Class("txn-edit-summary"), splits, attachments)}),
		If(s.errMsg.Get() != "" && amountError == "" && dateError == "", Div(css.Class("form-err-sticky"), errText("txn-edit-err", s.errMsg.Get()))),
	)
	return IfElse(!s.found,
		ui.CreateElement(txnEditMissing, txnEditMissingProps{Cancel: cancel}),
		IfElse(len(cands) > 0,
			ui.CreateElement(txnSimilarOffer, txnSimilarOfferProps{App: s.app, Candidates: cands,
				Apply: applySimilar, AlwaysRule: alwaysRule, Dismiss: dismissSimilar}),
			form))
}

func (s txnEditState) saveSplits(updated domain.Transaction) {
	if s.saving.Get() {
		return
	}
	live, ok := s.app.TransactionByID(updated.ID)
	if !ok {
		return
	}
	live, labelKey := txnform.SplitUpdate(live, updated)
	isCurrent := s.saveLease.check()
	s.saving.Set(true)
	label := uistate.T(labelKey)
	_ = CommitUndoable(s.app, label, []string{live.ID}, func() error { return s.app.PutTransactionsAtomic([]domain.Transaction{live}) }, func(err error) {
		if !isCurrent() {
			return
		}
		s.saving.Set(false)
		if err != nil {
			s.errMsg.Set(txnEditSaveError(err))
		}
	})
}

func (s txnEditState) save() {
	if s.saving.Get() {
		return
	}
	plan := txnform.Build(s.txn, s.draft(), s.app.Accounts(), s.app.Transactions(), time.Now())
	if plan.Err != nil {
		if plan.Failure.Key == "" {
			s.errMsg.Set(txnEditSaveError(plan.Err))
		} else {
			s.errMsg.Set(txnEditMessage(plan.Failure))
		}
		return
	}
	t := plan.Transaction
	performSave := func(removePartner bool) { s.performSave(t, plan.SyncDirectionOut, removePartner) }
	if plan.UnclassifiesTransfer {
		if partnerID, confirm := txnform.PartnerRemoval(s.txn, t, s.app.ExpandTransferPairs([]string{s.txn.ID})); confirm {
			partner, ok := s.app.TransactionByID(partnerID)
			if !ok {
				return
			}
			prompt := txnEditMessage(txnform.PartnerPrompt(partner, s.app.Accounts()))
			uistate.ConfirmModalLabeled(prompt, uistate.T("uxt4.delete"), true, func(ok bool) {
				if ok {
					performSave(true)
				}
			})
			return
		}

	}
	performSave(false)
}

func (s txnEditState) performSave(t domain.Transaction, syncDirectionOut, removePartner bool) {
	isCurrent := s.saveLease.check()
	s.saving.Set(true)
	ids := s.app.ExpandTransferPairs([]string{t.ID})
	_ = CommitUndoable(s.app, uistate.T("toast.txnUpdated"), ids, func() error {
		var err error
		if removePartner {
			err = s.app.EditTransactionRemovingPartner(s.txn, t)
		} else {
			err = s.app.EditTransaction(s.txn, t)
		}
		if err != nil {
			return err
		}
		if t.CategoryID != "" {
			learn := firstNonEmpty(t.Payee, t.Desc)
			uistate.IncrementLearnTally(learn, t.CategoryID)
		}
		return nil
	}, func(err error) {
		if !isCurrent() {
			if err != nil {
				uistate.PostNotice(uistate.T("txint.transactionNotSaved"), true)
			}
			return
		}
		s.saving.Set(false)
		if err != nil {
			s.errMsg.Set(txnEditSaveError(err))
			return
		}

		s.afterSave(t, syncDirectionOut)
	})
}
func (s txnEditState) afterSave(t domain.Transaction, syncDirectionOut bool) {

	// C547: now that the write has landed, bring the control into agreement
	// with what was stored. Doing this earlier re-rendered the form mid-save
	// and lost the write.
	if syncDirectionOut {
		s.directionS.Set(txnDirOut)
	}

	// TX1 learning flow: when the user renamed the payee, quietly offer to always
	// show the original (raw) name as the new one — creating a view-layer alias so
	// every transaction with that raw name reads cleanly (the raw stays on the transaction).
	origPayee, newPayee, learnAlias := txnform.LearnAlias(s.txn.Payee, s.payeeS.Get(), s.app.PayeeResolver().HasLearned(s.txn.Payee))
	if learnAlias {
		uistate.ConfirmModal(uistate.T("payeealias.learnPrompt", origPayee, newPayee), false, func(ok bool) {
			if !ok {
				return
			}
			if err := s.app.PutPayeeAlias(domain.PayeeAlias{RawPayee: origPayee, Display: newPayee}); err != nil {
				uistate.PostNotice(txnEditSaveError(err), true)
				return
			}
			uistate.BumpDataRevision()
			uistate.PostNotice(uistate.T("payeealias.learned", origPayee, newPayee), false)
		})
	}

	// TX7: when the category changed to a real category, proactively find similar
	// transactions (alias/payee match via TX1, else the rules matcher) that carry a
	// different or no category, and offer to recategorize them too. The offer keeps
	// the modal open (inline preview + Apply); already-categorized rows are listed
	// but never overwritten without the click.
	if txnform.CategoryChanged(s.txn, t) {
		cands := similartxns.Find(t, s.app.Transactions(), t.CategoryID, s.app.PayeeResolver(), s.app.Rules())
		if len(cands) > 0 {
			s.simTarget.Set(t)
			s.simState.Set(cands)
			return // hold the modal open to show the offer
		}
	}

	if s.props.OnDone != nil {
		s.props.OnDone()
	}
}

func (s txnEditState) applySimilar() {
	writes, ids := txnform.SimilarWrites(s.simState.Get(), s.app.Transactions(), s.simTarget.Get().CategoryID)
	if len(writes) == 0 {
		return
	}
	isCurrent := s.saveLease.check()
	s.saving.Set(true)
	_ = CommitUndoable(s.app, uistate.T("similartxns.applied", len(writes)), ids, func() error { return s.app.PutTransactionsAtomic(writes) }, func(err error) {
		if !isCurrent() {
			return
		}
		s.saving.Set(false)
		if err != nil {
			s.errMsg.Set(txnEditSaveError(err))
			return
		}
		s.simState.Set(nil)
		if s.props.OnDone != nil {
			s.props.OnDone()
		}
	})
}

func (s txnEditState) attach() {
	isCurrent := s.saveLease.check()
	pickFile("image/*", func(name, mime string, data []byte) {
		if !isCurrent() {
			return
		}
		art := txnform.ReceiptArtifact(id.New(), name, mime, data, time.Now())
		s.saving.Set(true)
		_ = CommitUndoable(s.app, uistate.T("transactions.attachReceiptTitle"), []string{s.props.TxnID}, func() error {
			return s.app.AttachTransactionArtifact(s.props.TxnID, art)
		}, func(err error) {
			if !isCurrent() {
				return
			}
			s.saving.Set(false)
			if err != nil {
				s.errMsg.Set(txnEditSaveError(err))
			}
		})
	})
}

func (s txnEditState) toolActions() (func(), func(), func(), func(), func()) {
	showHistory := func() { uistate.SetTxnHistory(s.txn.ID) }
	showCategorize := func() { uistate.SetCatSuggest(s.txn.ID) }
	showPayeeClean := func() { uistate.SetPayeeClean(s.txn.ID) }
	createRuleAction := func() {
		phrase := txnform.Phrase(s.txn)
		uistate.SetRuleDraft(phrase, s.txn.CategoryID)
		if s.props.OnDone != nil {
			s.props.OnDone()
		}
		router.Navigate(uistate.RoutePath("/rules"))
	}
	followUpAction := func() {
		merchant := txnform.Phrase(s.txn)
		uistate.SetTaskAddSeed(uistate.TaskAddSeed{
			Title:    uistate.T("transactions.followUpTaskTitle", merchant),
			LinkType: string(domain.RelatedTransaction),
			LinkID:   s.txn.ID,
		})
		uistate.SetAddTarget("task")
	}
	return showHistory, showCategorize, showPayeeClean, createRuleAction, followUpAction
}

func (s txnEditState) done() {
	if s.props.OnDone != nil {
		s.props.OnDone()
	}
}
func (s txnEditState) alwaysRule() {
	target := s.simTarget.Get()
	// This action historically falls back after trimming the payee.
	uistate.SetRuleDraft(txnform.RulePhrase(target), target.CategoryID)
	s.simState.Set(nil)
	s.done()
	router.Navigate(uistate.RoutePath("/rules"))
}
func (s txnEditState) dismissSimilar() { s.simState.Set(nil); s.done() }

type txnEditInputProps struct {
	Value, Error string
	OnInput      func(string)
}
type txnEditDirectionProps struct {
	Value    string
	OnSelect func(string)
}

func txnEditAmount(props txnEditInputProps) ui.Node {
	return uiw.Field(uiw.FieldProps{
		ID: "txn-edit-amount", Label: uistate.T("quickAdd.amount"),
		Error: props.Error, Required: true,
		Control: func(a uiw.FieldA11y) ui.Node {
			return Input(css.Class("field"), ID(a.ID), Type("text"), Attr("inputmode", "decimal"),
				Attr("data-testid", "txn-edit-amount"), Attr("aria-label", a.Label),
				Attr("aria-describedby", a.DescribedBy), Attr("aria-invalid", ariaBool(a.Invalid)), Attr("aria-required", "true"),
				OnInput(props.OnInput), uiw.FieldValue(props.Value))
		},
	})
}

func txnEditPayee(props txnEditInputProps) ui.Node {
	return uiw.Field(uiw.FieldProps{
		ID: "txn-edit-payee", Label: uistate.T("quickAdd.payee"),
		Control: func(a uiw.FieldA11y) ui.Node {
			return Input(css.Class("field"), ID(a.ID), Type("text"), Attr("data-testid", "txn-edit-payee"),
				Attr("aria-label", a.Label), Placeholder(uistate.T("quickAdd.payeePlaceholder")), OnInput(props.OnInput), uiw.FieldValue(props.Value))
		},
	})
}

func txnEditDate(props txnEditInputProps) ui.Node {
	return uiw.Field(uiw.FieldProps{
		ID: "txn-edit-date", Label: uistate.T("quickAdd.date"), Error: props.Error,
		Control: func(a uiw.FieldA11y) ui.Node {
			return Input(css.Class("field"), ID(a.ID), Type("date"), Attr("aria-label", a.Label),
				Attr("aria-describedby", a.DescribedBy), Attr("aria-invalid", ariaBool(a.Invalid)), OnInput(props.OnInput), uiw.FieldValue(props.Value))
		},
	})
}

func txnEditDirection(props txnEditDirectionProps) ui.Node {
	return TxnDirectionToggle(TxnDirectionToggleProps{
		Label: uistate.T("quickAdd.kind"), Value: props.Value,
		Options: []TxnDirectionOption{
			{Value: txnDirOut, Label: uistate.T("quickAdd.expense"), TestID: "txn-edit-direction-expense"},
			{Value: txnDirIn, Label: uistate.T("quickAdd.income"), TestID: "txn-edit-direction-income"},
			{Value: "Transfer", Label: uistate.T("quickAdd.transfer"), TestID: "txn-edit-direction-transfer", Title: uistate.T("r9f.transferEditUnavailable")},
		},
		OnSelect: func(value string) {
			if value == "Transfer" {
				uistate.PostNotice(uistate.T("r9f.transferEditUnavailable"), false)
				return
			}
			props.OnSelect(value)
		},
	})
}

type txnEditCategoryProps struct {
	App              *appstate.App
	Options          []uiw.SelectOption
	Kind             domain.CategoryKind
	Selected         string
	Open             bool
	Suggestion       ui.Node
	OnOpen, OnClose  func()
	OnSelect, OnPick func(string)
}

func txnEditCategory(props txnEditCategoryProps) ui.Node {
	selectGeneration := ui.UseState(0)
	onCategory := ui.UseEvent(func(e ui.Event) {
		if value := e.GetValue(); value == txnEditNewCategoryOption {
			// Selecting the command mutates the browser's live value even though
			// the category state did not change. Replace just the native select
			// so dismissing or reopening the picker retains the real category.
			selectGeneration.Set(selectGeneration.Get() + 1)
			props.OnOpen()
		} else {
			props.OnSelect(value)
		}
	})
	selectArgs := []any{css.Class("field"), Value(props.Selected), Attr("aria-label", uistate.T("quickAdd.category")),
		Attr("data-testid", "txn-edit-category"), OnChange(onCategory)}
	for _, option := range props.Options {
		selectArgs = append(selectArgs, Option(Value(option.Value), SelectedIf(option.Value == props.Selected), option.Label))
	}
	app := props.App
	if app == nil {
		app = appstate.Default
	}
	return Div(css.Class("labeled-field", "txn-edit-category-field"),
		Div(css.Class("txn-edit-category-label"),
			Span(css.Class("t-caption", tw.TextDim), uistate.T("quickAdd.category"))),
		Div(css.Class("txn-cat-picker"),
			Div(css.Class("txn-cat-row"),
				WithKey(Select(selectArgs...), selectGeneration.Get()),
			),
			props.Suggestion,
			If(props.Open && app != nil, ui.CreateElement(categoryPickerComponent, categoryPickerComponentProps{App: app, Props: CategoryPickerProps{
				Kind: props.Kind, Selected: props.Selected,
				OnPick:  func(categoryID string) { props.OnPick(categoryID) },
				OnClose: func() { props.OnClose() },
			}})),
		),
	)
}

type txnEditSuggestion struct {
	Suggestion catsuggest.Suggestion
	Found      bool
}

func (s txnEditState) categoryNode() ui.Node {
	revision := uistate.UseDataRevision().Get()
	proposal := ui.UseMemo(func() txnEditSuggestion {
		defer txnPerfStart("edit-category-suggestion")()
		candidate := txnform.SuggestionTransaction(s.txn, s.amountS.Get(), s.directionS.Get())
		suggested, ok := reviewSuggestion(s.app, candidate)
		return txnEditSuggestion{Suggestion: suggested, Found: ok}
	}, s.app, s.app.Rev(), revision, s.txn.ID, s.amountS.Get(), s.directionS.Get())
	options, kind := txnform.Categories(s.categories, s.directionS.Get())
	catOpts := []uiw.SelectOption{{Value: "", Label: uistate.T("transactions.noCategory")}}
	for _, o := range options {
		catOpts = append(catOpts, uiw.SelectOption{Value: o.Value, Label: o.Label})
	}
	catOpts = append(catOpts, uiw.SelectOption{Value: txnEditNewCategoryOption, Label: uistate.T("transactions.newCategoryOption")})
	name, show := txnform.SuggestionName(s.categories, proposal.Suggestion.CategoryID, s.catS.Get(), proposal.Found)
	var suggestion ui.Node
	if show {
		suggestion = uiw.Pill(uiw.PillProps{
			Label: uistate.T("r5e.suggestedCategory", name), AriaLabel: uistate.T("sol1.useCategory", name), Tone: uiw.PillAccent,
			TestID: "txn-edit-category-suggestion", OnClick: func() { s.catS.Set(proposal.Suggestion.CategoryID) }})
	}
	return ui.CreateElement(txnEditCategory, txnEditCategoryProps{App: s.app, Options: catOpts, Kind: kind, Selected: s.catS.Get(), Open: s.categoryPickerOpen.Get(), Suggestion: suggestion,
		OnOpen: func() { s.categoryPickerOpen.Set(true); s.errMsg.Set("") }, OnClose: func() { s.categoryPickerOpen.Set(false) }, OnSelect: s.catS.Set,
		OnPick: func(id string) { s.catS.Set(id); s.categoryPickerOpen.Set(false) }})
}

type txnEditDetailsProps struct {
	Desc, Tags, Note                string
	Owners                          []string
	Cleared, Exclude, HasMembers    bool
	MemberOptions                   []uiw.PickerOption
	MemberColors                    map[string]string
	OnDesc, OnTags, OnNote, OnOwner func(string)
	OnCleared, OnExclude            func(bool)
}

func txnEditDetails(props txnEditDetailsProps) ui.Node {
	return Fragment(
		uiw.Field(uiw.FieldProps{ID: "txn-edit-description", Label: uistate.T("transactions.descPlaceholder"), Hint: uistate.T("sol1.bankDescription"),
			Control: func(a uiw.FieldA11y) ui.Node {
				return Input(css.Class("field"), ID(a.ID), Type("text"), Attr("data-testid", "txn-edit-description"),
					Attr("aria-label", a.Label), Attr("aria-describedby", a.DescribedBy), OnInput(props.OnDesc), uiw.FieldValue(props.Desc))
			}}),
		labeledField(uistate.T("transactions.tagsLabel"), Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-edit-tags"), Attr("aria-label", uistate.T("transactions.tagsLabel")),
			Placeholder(uistate.T("transactions.tagsPlaceholder")), OnInput(props.OnTags), uiw.FieldValue(props.Tags))),
		If(props.HasMembers, TxnWhoField(TxnWhoFieldProps{ID: "txn-edit-who", Owners: props.Owners, Options: props.MemberOptions, Colors: props.MemberColors, OnToggle: func(owner string) { props.OnOwner(owner) }})),
		uiw.FormField(uistate.T("transactions.noteLabel"), Textarea(css.Class("field"), Attr("rows", "2"), Attr("data-testid", "txn-edit-note"),
			Attr("aria-label", uistate.T("transactions.noteLabel")), Attr("placeholder", uistate.T("transactions.notePlaceholder")), OnInput(props.OnNote), props.Note)),
		Div(css.Class("txn-form-check-field"),
			Label(css.Class("txn-form-check"), Input(Type("checkbox"), Attr("data-testid", "txn-edit-cleared"), Attr("aria-label", uistate.T("txnwidget.clearedLabel")),
				Attr("aria-describedby", "txn-edit-cleared-hint"), CheckedIf(props.Cleared), OnChange(func(e ui.Event) { props.OnCleared(e.IsChecked()) })), Span(uistate.T("txnwidget.clearedLabel"))),
			Span(ID("txn-edit-cleared-hint"), css.Class("txn-form-help"), uistate.T("sol1.clearedHint"))),
		Div(css.Class("txn-exclude-field", tw.FlexCol, tw.Gap1),
			Label(css.Class("txn-check"), Input(Type("checkbox"), Attr("data-testid", "txn-edit-exclude"), Attr("aria-describedby", "txn-edit-exclude-hint"), Attr("aria-label", uistate.T("transactions.excludeLabel")), CheckedIf(props.Exclude), OnChange(func(e ui.Event) { props.OnExclude(e.IsChecked()) })), Span(uistate.T("transactions.excludeLabel"))),
			Span(ID("txn-edit-exclude-hint"), css.Class("txn-form-help"), uistate.T("transactions.excludeHint"))),
	)
}

func (s txnEditState) moreNodes() []ui.Node {
	options, colors := txnform.Members(s.members)
	memberOpts := make([]uiw.PickerOption, 0, len(options))
	for _, o := range options {
		memberOpts = append(memberOpts, uiw.PickerOption{Value: o.Value, Label: o.Label})
	}
	details := ui.CreateElement(txnEditDetails, txnEditDetailsProps{Desc: s.descS.Get(), Tags: s.tagsS.Get(), Note: s.noteS.Get(), Owners: s.memberS.Get(), Cleared: s.clearedS.Get(), Exclude: s.excludeS.Get(), HasMembers: len(s.members) > 0,
		MemberOptions: memberOpts, MemberColors: colors, OnDesc: s.descS.Set, OnTags: s.tagsS.Set, OnNote: s.noteS.Set,
		OnOwner: func(owner string) { s.memberS.Set(txnform.ToggleOwner(s.memberS.Get(), owner)) }, OnCleared: s.clearedS.Set, OnExclude: s.excludeS.Set})
	movement := ui.CreateElement(txnEditMovement, txnEditMovementProps{App: s.app, Txn: s.txn, Selected: s.transferToS.Get(), DebtPay: s.debtPayS.Get(),
		OnSelect: func(v string) {
			s.transferToS.Set(v)
			if !txnform.IsLiability(s.app.Accounts(), v) {
				s.debtPayS.Set(false)
			}
		}, OnDebtPay: s.debtPayS.Set})
	return []ui.Node{details, movement, s.merchantPanel}
}

type txnEditToolsProps struct {
	Transfer, SplitOpen, HasSplits                                                                     bool
	ShowHistory, ShowCategorize, ShowPayeeClean, CreateRuleAction, FollowUpAction, Attach, ToggleSplit func()
}

func txnEditTools(props txnEditToolsProps) ui.Node {
	return Div(css.Class("txn-edit-row-actions"), Attr("data-testid", "txn-edit-row-actions"),
		uiw.OverflowMenu(uiw.OverflowMenuProps{
			TriggerText:   uistate.T("r9f.tools"),
			TriggerLabel:  uistate.T("r9f.tools"),
			TriggerTestID: "txn-edit-more-actions",
			TriggerClass:  "btn btn-tool",
			Items: []uiw.OverflowMenuItem{
				{Label: uistate.T("txnhistory.menuAction"), Icon: icon.History, TestID: "txn-history-open", OnSelect: props.ShowHistory},
				{Label: uistate.T("r1b.catSuggest.menuAction"), Icon: icon.Tag, TestID: "txn-categorize-open", OnSelect: props.ShowCategorize, Hidden: props.Transfer},
				{Label: uistate.T("payeeClean.menuAction"), Icon: icon.Pencil, TestID: "txn-cleanname-open", OnSelect: props.ShowPayeeClean, Hidden: props.Transfer},
				{Label: uistate.T("transactions.createRule"), Icon: icon.Workflow, TestID: "txn-create-rule", OnSelect: props.CreateRuleAction, Hidden: props.Transfer},
				{Label: uistate.T("transactions.followUpTask"), Icon: icon.PlusCircle, TestID: "txn-followup-task", OnSelect: props.FollowUpAction},
				{Label: uistate.T(txnform.SplitKey(props.SplitOpen, props.HasSplits)), Icon: icon.PlusCircle, TestID: "txn-edit-split-toggle", OnSelect: props.ToggleSplit},
				{Label: uistate.T("transactions.attachReceiptTitle"), Icon: icon.PlusCircle, TestID: "txn-edit-attach", OnSelect: props.Attach},
			},
		}),
	)
}

func (s txnEditState) toolsNode() ui.Node {
	history, categorize, clean, rule, follow := s.toolActions()
	return ui.CreateElement(txnEditTools, txnEditToolsProps{Transfer: s.txn.IsTransfer(), SplitOpen: s.splitOpen.Get(), HasSplits: s.txn.HasSplits(),
		ShowHistory: history, ShowCategorize: categorize, ShowPayeeClean: clean, CreateRuleAction: rule, FollowUpAction: follow, Attach: s.attach,
		ToggleSplit: func() { s.splitOpen.Set(!s.splitOpen.Get()) }})
}

type txnEditAttachmentsProps struct {
	App *appstate.App
	Txn domain.Transaction
}

func txnEditAttachments(props txnEditAttachmentsProps) ui.Node {
	txn, app := props.Txn, props.App
	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
		If(len(txn.Attachments) > 0, Span(css.Class("muted"), uistate.T(txnform.ReceiptKey(len(txn.Attachments)), len(txn.Attachments)))),
		If(len(txn.Attachments) > 0, receiptThumbnails(app, txn)),
	)
}

type txnEditSplitsProps struct {
	Txn           domain.Transaction
	Categories    []domain.Category
	Members       []domain.Member
	Open, Pending bool
	OnSave        func(domain.Transaction)
}

func txnEditSplits(props txnEditSplitsProps) ui.Node {
	var splitLines []ui.Node
	for _, line := range txnform.Splits(props.Txn, props.Categories, props.Members) {
		name := line.Category
		if name == "" {
			name = uistate.T("transactions.uncategorized")
		}
		ownerTag := ""
		if line.Owner != "" {
			ownerTag = uistate.T("splitEditor.ownerFor", line.Owner)
		}
		splitLines = append(splitLines, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class("badge badge-split"), "⑂"), Span(name), If(ownerTag != "", Span(css.Class("muted"), ownerTag)),
			uiw.MoneyFig(uiw.MoneyFigProps{Money: line.Amount, Style: uiw.MoneyLedger})))
	}
	return Div(Attr("data-testid", "txn-edit-splits"),
		If(len(splitLines) > 0, Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), splitLines)),
		If(props.Open, ui.CreateElement(SplitEditor, splitEditorProps{Txn: props.Txn, Categories: props.Categories, Members: props.Members, OnSave: props.OnSave, Pending: props.Pending})))
}

type txnEditAccountProps struct {
	App      *appstate.App
	Txn      domain.Transaction
	Selected string
	OnSelect func(string)
}

func txnEditAccount(props txnEditAccountProps) ui.Node {
	return accountField(props.App, props.Txn, props.Selected, props.OnSelect)
}

type txnEditMovementProps struct {
	App       *appstate.App
	Txn       domain.Transaction
	Selected  string
	DebtPay   bool
	OnSelect  func(string)
	OnDebtPay func(bool)
}

func txnEditMovement(props txnEditMovementProps) ui.Node {
	onDebtPay := ui.UseEvent(func(e ui.Event) { props.OnDebtPay(e.IsChecked()) })
	return classifyField(props.App, props.Txn, props.Selected, props.DebtPay, props.OnSelect, onDebtPay)
}

type txnEditMissingProps struct{ Cancel ui.Handler }

func txnEditMissing(props txnEditMissingProps) ui.Node {
	return Div(css.Class("form-grid"),
		P(css.Class("empty"), uistate.T("txnwidget.notFound")),
		Button(css.Class("btn"), Type("button"), OnClick(props.Cancel), uistate.T("action.cancel")),
	)
}

type txnSimilarOfferProps struct {
	App        *appstate.App
	Candidates []similartxns.Candidate
	Apply      ui.Handler
	AlwaysRule ui.Handler
	Dismiss    ui.Handler
}

func txnSimilarOffer(props txnSimilarOfferProps) ui.Node {
	view := txnform.SimilarOffer(props.Candidates, props.App.PayeeResolver())
	var rows []ui.Node
	for _, row := range view.Rows {
		rows = append(rows, Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(css.Class(tw.Flex1), row.Name), Span(css.Class("muted"), row.Date), Span(css.Class("muted"), row.Amount),
			If(row.AlreadyCategorized, Span(css.Class("muted"), uistate.T("similartxns.hasCategory")))))
	}
	if view.Remaining > 0 {
		rows = append(rows, P(css.Class("muted"), uistate.T("similartxns.moreCount", view.Remaining)))
	}
	offer := txnEditMessage(view.Offer)

	// Keep the prompt, evidence, and answers in one stack; the actions wrap rather
	// than being squeezed into a third grid column.
	return Div(ClassStr("txn-recat-offer"), Attr("data-testid", "txn-recat-offer"), Attr("role", "status"),
		P(css.Class("t-body"), offer),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1, tw.Mb2), rows),
		Div(css.Class("txn-recat-actions", tw.Flex, tw.ItemsCenter, tw.Gap2),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "txn-recat-apply"), OnClick(props.Apply), uistate.T("similartxns.apply")),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-recat-rule"), OnClick(props.AlwaysRule), uistate.T("similartxns.alwaysDo")),
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "txn-recat-dismiss"), OnClick(props.Dismiss), uistate.T("similartxns.dismiss")),
		),
	)
}

// TxnEditDeleteFooter renders the named destructive action for the Edit panel footer.
func TxnEditDeleteFooter(props TransactionEditFormProps) ui.Node {
	if props.App == nil {
		props.App = appstate.Default
	}
	return ui.CreateElement(txnEditDeleteFooter, props)
}

func txnEditDeleteFooter(props TransactionEditFormProps) ui.Node {
	app := props.App
	var txn domain.Transaction
	found := false
	if app != nil {
		txn, found = app.TransactionByID(props.TxnID)
	}
	remove := ui.UseEvent(Prevent(func() {
		if !found {
			return
		}
		uistate.ConfirmModal(txnDeletePrompt(app, txn), true, func(confirmed bool) {
			if !confirmed {
				return
			}
			ids := app.ExpandTransferPairs([]string{props.TxnID})
			_ = CommitUndoable(app, uistate.T("toast.txnDeleted"), ids, func() error { _, err := app.DeleteTransactionsBulkExpanded(ids); return err }, func(err error) {
				if err != nil {
					uistate.PostNotice(uistate.T("sol1.deleteFailed"), true)
					return
				}
				if props.OnDone != nil {
					props.OnDone()
				}
			})
		})
	}))
	return ui.CreateElement(txnEditDeleteButton, txnEditDeleteButtonProps{Found: found, Remove: remove})
}

type txnEditDeleteButtonProps struct {
	Found  bool
	Remove ui.Handler
}

func txnEditDeleteButton(props txnEditDeleteButtonProps) ui.Node {
	if !props.Found {
		return Fragment()
	}
	return Button(css.Class("btn btn-ghost txn-edit-delete"), Type("button"), Attr("data-testid", "txn-edit-delete"), OnClick(props.Remove), uistate.T("r9f.deleteTransaction"))
}

// Transaction direction values for the edit form's sign control. They are the
// form's own vocabulary, not persisted — the stored truth is the amount's sign.
const (
	txnDirOut                = "out"
	txnDirIn                 = "in"
	txnEditNewCategoryOption = "__txn_edit_new_category__"
)

// accountField renders the "which account is this on" picker — the correction
// for a row that arrived on the wrong one.
//
// Every other fact about an imported row was already editable here; this was not,
// so the only fix for a mis-import was to delete the row and retype it, throwing
// away its links, splits, history and id.
//
// It previews rather than only validating on save, because the two consequences
// worth knowing about are both invisible from the picker: that a transfer's other
// leg comes along, and that moving a cleared row disturbs a reconciled balance. A
// refusal (a currency the account cannot hold) also shows here rather than after
// the button.
func accountField(app *appstate.App, txn domain.Transaction, selected string, onSelect func(string)) ui.Node {
	return ui.CreateElement(txnEditAccountView, txnEditAccountProps{App: app, Txn: txn, Selected: selected, OnSelect: onSelect})
}

func txnEditAccountView(props txnEditAccountProps) ui.Node {
	app, txn, selected, onSelect := props.App, props.Txn, props.Selected, props.OnSelect
	view := ui.UseMemo(func() txnform.AccountView {
		defer txnPerfStart("edit-account-preview")()
		return txnform.AccountPreview(txn, selected, app.Accounts(), app.Transactions())
	}, app, app.Rev(), txn, selected)
	if !view.Show {
		return Fragment()
	}
	opts := make([]uiw.SelectOption, 0, len(view.Options))
	for _, o := range view.Options {
		opts = append(opts, uiw.SelectOption{Value: o.Value, Label: o.Label})
	}
	note := Fragment()
	if view.Refusal.Key != "" {
		note = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-account-note"), Attr("role", "status"), txnEditMessage(view.Refusal))
	} else if len(view.Lines) > 0 {
		lines := []ui.Node{Span(txnEditMessage(view.Lines[0]))}
		for _, line := range view.Lines[1:] {
			lines = append(lines, Span(" "+txnEditMessage(line)))
		}
		note = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-account-note"), Attr("role", "status"), lines)
	}

	return Div(css.Class("txn-account-field", tw.FlexCol, tw.Gap1), Attr("data-testid", "txn-edit-account-field"),
		uiw.FormField(uistate.T("quickAdd.account"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   opts,
				Selected:  selected,
				AriaLabel: uistate.T("quickAdd.account"),
				OnChange:  onSelect,
				TestID:    "txn-edit-account",
			})),
		note,
	)
}

// classifyField renders the movement classifier: the counterparty picker, the
// consequence of the current choice, and — for a card or loan — the claim that
// this row IS the payment.
//
// The component adapter owns its stable event hook; this renderer only maps
// the pure preview to the existing elements.
func classifyField(app *appstate.App, txn domain.Transaction, selected string, debtPay bool,
	onSelect func(string), onDebtPay ui.Handler) ui.Node {
	return ui.CreateElement(txnEditMovementView, txnEditMovementViewProps{App: app, Txn: txn, Selected: selected, DebtPay: debtPay, OnSelect: onSelect, OnDebtPay: onDebtPay})
}

type txnEditMovementViewProps struct {
	App       *appstate.App
	Txn       domain.Transaction
	Selected  string
	DebtPay   bool
	OnSelect  func(string)
	OnDebtPay ui.Handler
}

func txnEditMovementView(props txnEditMovementViewProps) ui.Node {
	app, txn, selected, debtPay := props.App, props.Txn, props.Selected, props.DebtPay
	onSelect, onDebtPay := props.OnSelect, props.OnDebtPay
	view := ui.UseMemo(func() txnform.MovementView {
		defer txnPerfStart("edit-movement-preview")()
		return txnform.MovementPreview(txn, selected, debtPay, app.Accounts(), app.Transactions())
	}, app, app.Rev(), txn, selected, debtPay)
	if !view.Show {
		return Fragment()
	}
	opts := []uiw.SelectOption{{Value: "", Label: uistate.T("transactions.classifyNone")}}
	for _, o := range view.Options {
		opts = append(opts, uiw.SelectOption{Value: o.Value, Label: o.Label})
	}
	pairNote := Fragment()
	if view.Pair.Key != "" {
		pairNote = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-pair"), Attr("role", "status"), txnEditMessage(view.Pair))
	}
	effect, debtClaim := Fragment(), Fragment()
	if view.Effect.Key != "" {
		effect = P(css.Class("muted", tw.Text12), Attr("data-testid", "txn-edit-classify-effect"), txnEditMessage(view.Effect))
	}
	if view.DebtAccount != "" {
		debtClaim = ui.CreateElement(classifyDebtClaim, classifyDebtClaimProps{Account: view.DebtAccount, Checked: debtPay, OnChange: onDebtPay})
	}

	// C673: a titled GROUP, not a lone field. "Other account this money moved to or
	// from" named a field and left the reader to work out what filling it in would
	// do — so it read as optional metadata sitting beside the category, which is
	// precisely how a row ends up filed under a category called "Transfer" while
	// still counting as spending. The heading names the action; the picker is the
	// question that action needs answered; the sign note is here because the minus
	// on this leg is not evidence of spending.
	return Div(css.Class("txn-classify-field", tw.FlexCol, tw.Gap1), Attr("data-testid", "txn-edit-classify-field"),
		Attr("role", "group"), Attr("aria-label", uistate.T("transactions.classifyGroup")),
		H4(css.Class("set-label"), Attr("data-testid", "txn-edit-classify-heading"),
			uistate.T("transactions.classifyGroup")),
		Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyGroupHint")),
		uiw.FormField(uistate.T("transactions.classifyLabel"),
			uiw.SelectInput(uiw.SelectInputProps{
				Options:   opts,
				Selected:  selected,
				AriaLabel: uistate.T("transactions.classifyLabel"),
				OnChange:  onSelect,
				TestID:    "txn-edit-classify",
			})),
		effect,
		pairNote,
		Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyHint")),
		// Shown only once the row IS a transfer: before that there is no leg to
		// explain, and the note would be answering a question nobody asked.
		If(view.ShowSign, Span(css.Class("muted", tw.Text12),
			Attr("data-testid", "txn-edit-classify-sign"),
			uistate.T("transactions.classifySignHint"))),
		debtClaim,
	)
}

type classifyDebtClaimProps struct {
	Account  string
	Checked  bool
	OnChange ui.Handler
}

func classifyDebtClaim(props classifyDebtClaimProps) ui.Node {
	return Div(css.Class(tw.FlexCol, tw.Gap1),
		Label(css.Class("txn-check"),
			Input(Type("checkbox"), Attr("data-testid", "txn-edit-classify-debt"),
				Attr("aria-label", uistate.T("transactions.classifyDebtLabel", props.Account)),
				CheckedIf(props.Checked), OnChange(props.OnChange)),
			Span(uistate.T("transactions.classifyDebtLabel", props.Account))),
		Span(css.Class("muted", tw.Text12), uistate.T("transactions.classifyDebtHint")))
}

func txnEditSaveError(err error) string {
	slog.Error("transaction editor operation failed", "error", err)
	return uistate.T("sol1.saveFailed")
}

// txnEditMessage localizes a prepared message at the rendering edge.
func txnEditMessage(message txnform.Message) string {
	if message.Key == "" {
		return ""
	}
	return uistate.T(message.Key, message.Args...)
}
