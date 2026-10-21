// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"strconv"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/screens"
	"github.com/monstercameron/CashFlux/internal/txnform"
	"github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// QuickAddHost keeps the controlled modal mounted so closing it restores focus.
func QuickAddHost() uic.Node { return uic.CreateElement(quickAddHost, appstate.Default) }

func quickAddHost(app *appstate.App) uic.Node {
	open := uistate.UseQuickAdd()
	if app == nil {
		return Fragment()
	}
	return uic.CreateElement(quickAddForm, quickAddFormProps{App: app, Open: open.Get()})
}

type quickAddFormProps struct {
	App  *appstate.App
	Open bool
}

func quickAddForm(props quickAddFormProps) uic.Node {
	open := uistate.UseQuickAdd()
	seedKind := uistate.UseQuickAddKind()
	transferOpen := uistate.UseAcctTransferOpen()
	_ = uistate.UseDataRevision().Get()
	active := uistate.UseActiveMember().Get()
	draft := uic.UseState(txnform.NewQuickDraft(active))
	attempted := uic.UseState(false)
	focus := uic.UseState("")
	categoryOpen := uic.UseState(false)
	draftID := uic.UseRef("")
	source := txnform.QuickSource{Accounts: props.App.Accounts(), Transactions: props.App.Transactions(),
		Categories: props.App.Categories(), Members: props.App.Members(), Rules: props.App.Rules(),
		MemberLens: uistate.UseMemberLens(), ActiveMember: active, DefaultAccount: props.App.DefaultAccountID(),
		Now: time.Now(), LearnTally: uistate.LoadLearnTally(), LearnThreshold: uistate.LoadLearnThreshold(), TemplateCount: len(uistate.TxnTemplates())}
	view := txnform.QuickPreview(draft.Get(), source, attempted.Get())
	reset := func() {
		draft.Set(txnform.NewQuickDraft(active))
		attempted.Set(false)
		focus.Set("")
		categoryOpen.Set(false)
		draftID.Set("")
	}
	closePanel := func() { reset(); open.Set(false) }
	uic.UseEffect(func() func() {
		if field := focus.Get(); field != "" {
			focusQuickAddField(field)
		}
		return nil
	}, focus.Get())
	uic.UseEffect(func() func() {
		if open.Get() {
			if kind := uistate.TakeQuickAddKind(); kind != "" {
				draft.Update(func(d txnform.QuickDraft) txnform.QuickDraft {
					return txnform.QuickSetDirection(d, kind, source.Categories)
				})
			}
		}
		return nil
	}, "quickadd-seed:"+strconv.FormatBool(open.Get())+":"+seedKind.Get())
	save, saving := useQuickAddSave(quickAddSaveProps{App: props.App, Open: props.Open, Draft: draft.Get, Source: source,
		AccountID: view.AccountID, Date: view.Date, DraftID: draftID, Invalid: func(field string) { attempted.Set(true); focus.Set(field) }})
	fields := quickAddFieldsProps{Draft: draft.Get(), View: view, Source: source, Change: draft.Update, Focus: focus.Set,
		Close: closePanel, Transfer: func() { transferOpen.Set(true) }, CategoryOpen: categoryOpen.Get(), SetCategoryOpen: categoryOpen.Set, App: props.App}
	footer := uic.CreateElement(quickAddFooter, quickAddFooterProps{Saving: saving, Close: closePanel,
		Save: func() { save(closePanel) }, Another: func() { save(reset) }})
	return ui.Overlay(ui.OverlayProps{Open: props.Open, Kind: ui.OverlayModal, Title: uistate.T("quickAdd.title"),
		TestID: "txn-add-dialog", Body: uic.CreateElement(quickAddBody, fields), Footer: footer, OnClose: closePanel})
}

type quickAddSaveProps struct {
	App             *appstate.App
	Open            bool
	Draft           func() txnform.QuickDraft
	Source          txnform.QuickSource
	AccountID, Date string
	DraftID         uic.Ref[string]
	Invalid         func(string)
}

// useQuickAddSave owns browser persistence and lifecycle hooks, never draft decisions.
func useQuickAddSave(p quickAddSaveProps) (func(func()), bool) {
	saving := uic.UseState(false)
	generation := uic.UseRef(uint64(0))
	notice := uistate.UseNotice()
	uic.UseEffect(func() func() {
		generation.Set(generation.Get() + 1)
		saving.Set(false)
		return func() { generation.Set(generation.Get() + 1) }
	}, p.Open)
	return func(after func()) {
		if saving.Get() {
			return
		}
		d := p.Draft()
		if field, _ := txnform.QuickInvalidField(p.Source.Accounts, p.AccountID, d.Amount, d.Direction, d.Payee, d.Desc); field != "" {
			p.Invalid(field)
			return
		}
		if p.DraftID.Get() == "" {
			p.DraftID.Set(id.New())
		}
		d.Date = p.Date
		t, err := txnform.BuildQuick(d, p.Source.Accounts, p.AccountID, p.Source.ActiveMember, p.DraftID.Get(), time.Now())
		if err != nil {
			return
		}
		t = p.App.AutoCategorizeTransaction(t)
		current := generation.Get()
		saving.Set(true)
		_ = screens.CommitUndoable(p.App, uistate.T("quickAdd.added"), []string{t.ID}, func() error {
			if err := p.App.PutTransaction(t); err != nil {
				return err
			}
			if t.CategoryID != "" {
				uistate.IncrementLearnTally(txnform.RulePhrase(t), t.CategoryID)
			}
			return nil
		}, func(err error) {
			if generation.Get() != current {
				return
			}
			saving.Set(false)
			if err != nil {
				notice.Set(notice.Get().With(uistate.T("txint.transactionNotSaved"), true))
				return
			}
			if after != nil {
				after()
			}
		})
	}, saving.Get()
}

type quickAddFieldsProps struct {
	Draft           txnform.QuickDraft
	View            txnform.QuickView
	Source          txnform.QuickSource
	App             *appstate.App
	Change          func(func(txnform.QuickDraft) txnform.QuickDraft)
	Focus           func(string)
	Close, Transfer func()
	CategoryOpen    bool
	SetCategoryOpen func(bool)
}

func quickAddBody(p quickAddFieldsProps) uic.Node {
	return Div(css.Class("txn-add-layout"), screens.TxnSharedForm(screens.TxnFormModel{
		Mode: screens.TxnFormAdd, Amount: uic.CreateElement(quickAddAmount, p), Direction: uic.CreateElement(quickAddDirection, p),
		Payee: uic.CreateElement(quickAddPayee, p), Category: uic.CreateElement(quickAddCategory, p), Date: uic.CreateElement(quickAddDate, p), Account: uic.CreateElement(quickAddAccount, p),
		Sentence: uic.CreateElement(quickAddSentence, p), More: []uic.Node{uic.CreateElement(quickAddDescription, p),
			screens.SmartFieldAssist(uistate.LoadSmartSettings(), "qa-desc", p.View.DescriptionSuggestion, func() {
				p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Desc = p.View.CleanDescription; return d })
			}), uic.CreateElement(quickAddReview, p), uic.CreateElement(quickAddDetails, p)},
		AddQuickFill: uic.CreateElement(quickAddPayeeDatalist, p.View.Payees), AddTemplates: uic.CreateElement(quickAddTemplates, p), MoreID: "txn-add-more"}))
}

func quickAddSentence(p quickAddFieldsProps) uic.Node {
	return Div(css.Class("qa-sentence"), Label(Attr("for", "sm15-input"), uistate.T("txu3.sentenceLabel")),
		uic.CreateElement(screens.TxnDraftHint, screens.TxnDraftHintProps{OnApply: func(fill screens.TxnDraftFill) {
			next := txnform.QuickApplyFill(p.Draft, fill, p.Source.Categories)
			p.Change(func(d txnform.QuickDraft) txnform.QuickDraft {
				return txnform.QuickApplyFill(d, fill, p.Source.Categories)
			})
			p.Focus(txnform.QuickNextFocus(p.Source.Accounts, p.View.AccountID, next.Amount, next.Payee, next.Desc, fill))
		}}))
}

func quickAddAmount(p quickAddFieldsProps) uic.Node {
	amountAttrs := []any{css.Class("field"), Type("text"), Attr("inputmode", "decimal"), Attr("id", "txn-add-amount"), Attr("data-testid", "txn-add-amount"), Attr("autofocus", ""), Attr("aria-label", uistate.T("quickAdd.amount")), Attr("aria-required", "true"), Placeholder(uistate.T("quickAdd.amount")), ui.FieldValue(p.Draft.Amount)}
	if p.View.AmountError {
		amountAttrs = append(amountAttrs, Attr("aria-invalid", "true"), Attr("aria-describedby", "txn-add-amount-error"))
	}
	onAmount := func(v string) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft {
			return txnform.QuickInputAmount(d, v, p.Source.Categories)
		})
	}
	onAmountBlur := func(e uic.Event) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { return txnform.QuickBlurAmount(d, e.GetValue()) })
	}
	amountNote := Fragment()
	if p.View.AmountNote != "" {
		amountNote = Div(css.Class("qa-amount-note"), Attr("data-testid", "txn-add-amount-note"), Attr("role", "status"), Attr("aria-live", "polite"), uistate.T("quickAdd.minusMeansOut", p.View.AmountNote))
	}

	return Div(
		ui.Field(ui.FieldProps{ID: "txn-add-amount", Label: uistate.T("quickAdd.amount"),
			Error: uistate.T(p.View.AmountErrorKey), Required: true,
			Control: func(a ui.FieldA11y) uic.Node {
				return Input(append(amountAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onAmount), OnBlur(onAmountBlur))...)
			}}), amountNote)
}

func quickAddPayee(p quickAddFieldsProps) uic.Node {
	payeeAttrs := []any{css.Class("field"), Type("text"), Attr("id", "txn-add-payee"), Attr("data-testid", "txn-add-payee"), Attr("list", "qa-payees"), Attr("aria-label", uistate.T("quickAdd.payee")), Placeholder(uistate.T("quickAdd.payeePlaceholder")), ui.FieldValue(p.Draft.Payee)}
	if p.View.PayeeError {
		payeeAttrs = append(payeeAttrs, Attr("aria-invalid", "true"), Attr("aria-describedby", "txn-add-payee-error"))
	}
	onPayee := func(v string) { p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Payee = v; return d }) }

	return ui.Field(ui.FieldProps{ID: "txn-add-payee", Label: uistate.T("quickAdd.payee"),
		Error: uistate.T(p.View.PayeeErrorKey),
		Control: func(a ui.FieldA11y) uic.Node {
			return Input(append(payeeAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onPayee))...)
		}})
}

func quickAddDate(p quickAddFieldsProps) uic.Node {
	onDate := func(v string) { p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Date = v; return d }) }

	return ui.Field(ui.FieldProps{ID: "txn-add-date", Label: uistate.T("quickAdd.date"),
		Control: func(a ui.FieldA11y) uic.Node {
			return Input(css.Class("field"), ID(a.ID), Type("date"), Attr("data-testid", "txn-add-date"),
				Attr("aria-label", a.Label), OnInput(onDate), ui.FieldValue(p.View.Date))
		}})
}

func quickAddAccount(p quickAddFieldsProps) uic.Node {
	accountAttrs := []any{css.Class("field"), Attr("id", "txn-add-account"), Attr("data-testid", "txn-add-account"), Attr("aria-label", uistate.T("quickAdd.account")), Attr("aria-required", "true")}
	if p.View.AccountError {
		accountAttrs = append(accountAttrs, Attr("aria-invalid", "true"), Attr("aria-describedby", "txn-add-account-error"))
	}
	acctOpts := make([]uic.Node, 0, len(p.View.Accounts))
	for _, o := range p.View.Accounts {
		acctOpts = append(acctOpts, Option(Value(o.Value), SelectedIf(p.View.AccountID == o.Value), o.Label))
	}
	onAcct := func(v uic.Event) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.AccountID = v.GetValue(); return d })
	}

	return ui.Field(ui.FieldProps{ID: "txn-add-account", Label: uistate.T("quickAdd.account"),
		Error: uistate.T(p.View.AccountErrorKey), Required: true,
		Control: func(a ui.FieldA11y) uic.Node {
			return Select(append(append(accountAttrs, Attr("aria-describedby", a.DescribedBy), OnChange(onAcct)), acctOpts)...)
		}})
}

func quickAddDirection(p quickAddFieldsProps) uic.Node {
	return screens.TxnDirectionToggle(screens.TxnDirectionToggleProps{
		Label: uistate.T("quickAdd.kind"), Value: p.Draft.Direction,
		Options: []screens.TxnDirectionOption{
			{Value: "Expense", Label: uistate.T("quickAdd.expense"), TestID: "txn-add-direction-expense"},
			{Value: "Income", Label: uistate.T("quickAdd.income"), TestID: "txn-add-direction-income"},
			{Value: "Transfer", Label: uistate.T("quickAdd.transfer"), TestID: "txn-add-direction-transfer"},
		},
		OnSelect: func(v string) {
			if _, transfer := txnform.QuickChooseDirection(p.Draft, v, p.Source.Categories); transfer {
				p.Close()
				p.Transfer()
				return
			}
			p.Change(func(d txnform.QuickDraft) txnform.QuickDraft {
				next, _ := txnform.QuickChooseDirection(d, v, p.Source.Categories)
				return next
			})
		},
	})
}

func quickAddCategory(p quickAddFieldsProps) uic.Node {
	catOpts := []uic.Node{Option(Value(""), SelectedIf(p.Draft.CategoryID == ""), uistate.T("quickAdd.noCategory"))}
	for _, o := range p.View.Categories {
		catOpts = append(catOpts, Option(Value(o.Value), SelectedIf(p.Draft.CategoryID == o.Value), o.Label))
	}
	setCategory := func(category string) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.CategoryID = category; return d })
	}
	onCat := uic.UseEvent(func(e uic.Event) { setCategory(e.GetValue()) })
	toggleNewCategory := uic.UseEvent(Prevent(func() { p.SetCategoryOpen(true) }))
	catAssist := screens.SmartFieldAssist(uistate.LoadSmartSettings(), "qa-cat", p.View.CategorySuggestion, func() { setCategory(p.View.RuleCategoryID) })
	learnedCatAssist := screens.SmartFieldAssist(uistate.LoadSmartSettings(), "qa-learned-cat", p.View.LearnedCategoryName, func() { setCategory(p.View.LearnedCategoryID) })
	budgetImpact := screens.QuickAddBudgetImpact(p.App, p.Draft.CategoryID, p.View.ImpactMinor)

	return Div(css.Class("labeled-field", "txn-edit-category-field"),
		Div(css.Class("txn-edit-category-label"), Span(css.Class("t-caption", tw.TextDim), uistate.T("quickAdd.category"))),
		Div(css.Class("txn-cat-picker"),
			Div(css.Class("txn-cat-row"), Select(css.Class("field"), Attr("data-testid", "txn-add-category"), Attr("aria-label", uistate.T("quickAdd.category")), OnChange(onCat), catOpts),
				screens.TxnNewCategoryAction("txn-add-newcat-toggle", p.CategoryOpen, toggleNewCategory)),
			catAssist, learnedCatAssist, budgetImpact,
			If(p.CategoryOpen, uic.CreateElement(screens.CategoryPicker, screens.CategoryPickerProps{
				Kind:     p.View.CategoryKind,
				Selected: p.Draft.CategoryID,
				OnPick:   func(categoryID string) { setCategory(categoryID); p.SetCategoryOpen(false) },
				OnClose:  func() { p.SetCategoryOpen(false) },
			})),
		),
	)
}

func quickAddDescription(p quickAddFieldsProps) uic.Node {
	descAttrs := []any{css.Class("field"), Type("text"), Attr("id", "txn-add-desc"), Attr("data-testid", "txn-add-desc"), Attr("aria-label", uistate.T("quickAdd.description")), Attr("aria-required", "false"), Placeholder(uistate.T("quickAdd.descPlaceholder")), ui.FieldValue(p.Draft.Desc)}
	onDesc := func(v string) { p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Desc = v; return d }) }

	return ui.Field(ui.FieldProps{ID: "txn-add-desc", Label: uistate.T("quickAdd.description"), Hint: uistate.T("sol1.bankDescription"),
		Control: func(a ui.FieldA11y) uic.Node {
			return Input(append(descAttrs, Attr("aria-describedby", a.DescribedBy), OnInput(onDesc))...)
		}})
}

func quickAddReview(p quickAddFieldsProps) uic.Node {
	onReviewed := uic.UseEvent(func(e uic.Event) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.NeedsReview = e.IsChecked(); return d })
	})
	needsReviewArgs := []any{Type("checkbox"), Attr("data-testid", "txn-add-needs-review"), OnChange(onReviewed), Checked(p.Draft.NeedsReview)}
	return Div(css.Class("txn-form-check-field"),
		Label(css.Class("txn-form-check"),
			Input(needsReviewArgs...),
			uistate.T("txu3.needsReview")),
		Span(css.Class("txn-form-help"),
			uistate.T("txu3.needsReviewHelp")))
}

func quickAddDetails(p quickAddFieldsProps) uic.Node {
	onTags := uic.UseEvent(func(v string) { p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Tags = v; return d }) })
	onNote := uic.UseEvent(func(v string) { p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Note = v; return d }) })
	onCleared := uic.UseEvent(func(v uic.Event) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Cleared = v.IsChecked(); return d })
	})
	onExclude := uic.UseEvent(func(v uic.Event) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft { d.Exclude = v.IsChecked(); return d })
	})
	memberOptions := make([]ui.PickerOption, 0, len(p.View.Members))
	for _, m := range p.View.Members {
		memberOptions = append(memberOptions, ui.PickerOption{Value: m.Value, Label: m.Label})
	}
	return Div(css.Class("txn-form-detail-group"),
		ui.FormField(uistate.T("quickAdd.tags"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-tags"), Attr("aria-label", uistate.T("quickAdd.tags")),
				Placeholder(uistate.T("quickAdd.tagsPh")), OnInput(onTags), ui.FieldValue(p.Draft.Tags))),
		If(p.View.ShowMembers, screens.TxnWhoField(screens.TxnWhoFieldProps{
			ID: "txn-add-who", Owners: p.View.Owners, Colors: p.View.Colors, Options: memberOptions,
			OnToggle: func(owner string) {
				p.Change(func(d txnform.QuickDraft) txnform.QuickDraft {
					return txnform.QuickToggleOwner(d, owner)
				})
			},
		})),
		ui.FormField(uistate.T("quickAdd.note"),
			Input(css.Class("field"), Type("text"), Attr("data-testid", "txn-add-note"), Attr("aria-label", uistate.T("quickAdd.note")),
				Placeholder(uistate.T("quickAdd.notePh")), OnInput(onNote), ui.FieldValue(p.Draft.Note))),
		Div(css.Class("txn-form-check-field"),
			Label(css.Class("txn-form-check"), Input(Type("checkbox"), Attr("data-testid", "txn-add-cleared"),
				Attr("aria-describedby", "txn-add-cleared-hint"), OnChange(onCleared), Checked(p.Draft.Cleared)), uistate.T("quickAdd.cleared")),
			Span(ID("txn-add-cleared-hint"), css.Class("txn-form-help"), uistate.T("sol1.clearedHint"))),
		Div(css.Class("txn-form-check-field"),
			Label(css.Class("txn-form-check"), Input(Type("checkbox"), Attr("data-testid", "txn-add-exclude"),
				Attr("aria-describedby", "txn-add-exclude-hint"), OnChange(onExclude), Checked(p.Draft.Exclude)), uistate.T("quickAdd.exclude")),
			Span(ID("txn-add-exclude-hint"), css.Class("txn-form-help"), uistate.T("transactions.excludeHint"))),
	)
}

func quickAddPayeeDatalist(payees []string) uic.Node {
	args := make([]any, 0, len(payees)+1)
	args = append(args, Attr("id", "qa-payees"))
	for _, payee := range payees {
		args = append(args, Option(Value(payee)))
	}
	return Datalist(args...)
}

func quickAddTemplates(p quickAddFieldsProps) uic.Node {
	onPick := func(t domain.TxnTemplate) {
		p.Change(func(d txnform.QuickDraft) txnform.QuickDraft {
			return txnform.QuickApplyTemplate(d, t, p.Source.Categories)
		})
	}
	save := screens.SaveAsTemplateButton(screens.TxnTemplateDraft{
		Payee: p.View.TemplatePayee, CategoryID: p.Draft.CategoryID, AccountID: p.View.AccountID,
		AmountMinor: p.View.TemplateMinor, Currency: p.View.Currency, Direction: p.View.TemplateDirection, Note: p.View.TemplateNote}, nil)
	return Div(css.Class("txt-zone"), If(p.View.ShowTemplates, screens.TxnTemplatePicker(onPick)), Div(css.Class("txt-save-row"), save))
}

type quickAddFooterProps struct {
	Saving               bool
	Close, Another, Save func()
}

func quickAddFooter(p quickAddFooterProps) uic.Node {
	return Div(css.Class("txn-form-footer"), Div(css.Class("txn-form-footer-actions"),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "txn-add-cancel"), OnClick(p.Close), uistate.T("action.cancel")),
		Button(css.Class("btn"), Type("button"), Disabled(p.Saving), Attr("data-testid", "txn-add-another"), OnClick(p.Another), uistate.T("quickAdd.saveAndAnother")),
		Button(css.Class("btn btn-primary"), Type("button"), Disabled(p.Saving), Attr("aria-busy", strconv.FormatBool(p.Saving)), Attr("id", "txn-add-save"), Attr("data-testid", "flip-save"), OnClick(p.Save), uistate.T("action.save"))))
}

// Compatibility aliases keep existing tests unchanged; all decisions live in txnform.
var (
	quickAddCategoryIsValid  = txnform.QuickCategoryValid
	resolveQuickAddAccountID = txnform.QuickAccountID
	quickAddInvalidFieldR4F  = txnform.QuickInvalidField
	quickAddDescriptionR4F   = txnform.QuickDescription
	quickAddNextFocusR4F     = txnform.QuickNextFocus
	quickAddMemberValue      = txnform.QuickMemberValue
	quickAddOwnerSelection   = txnform.QuickOwners
	quickAddReviewedValue    = txnform.QuickReviewed
)

func focusQuickAddField(id string) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	field := doc.Call("getElementById", id)
	if !field.IsNull() && !field.IsUndefined() {
		field.Call("focus")
	}
}
