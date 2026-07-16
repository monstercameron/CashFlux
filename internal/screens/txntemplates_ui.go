// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// txntemplates_ui.go renders the transaction quick-template ("favourites") UI
// mounted inside the quick-add flip panel: a horizontal picker strip of chips that
// pre-fill the add form in one click, and a "Save as template" action that snapshots
// the current form as a reusable favourite. Because there is no bank sync, every
// transaction is entered by hand, so one-click pre-fill of a frequent entry is high
// value. The pure logic + persistence live in internal/txntemplate and internal/
// uistate; this file is a thin view over them.

// TxnTemplateDraft is the quick-add form's current field set, handed to
// saveAsTemplateButton so it can snapshot the in-progress transaction as a template.
// AmountMinor is the magnitude (non-negative); Direction carries the sign, mirroring
// the form's positive amount field + Expense/Income toggle.
type TxnTemplateDraft struct {
	Payee       string
	CategoryID  string
	AccountID   string
	AmountMinor int64
	Currency    string
	Direction   domain.TxnDirection
	Note        string
	Tags        []string
}

// TxnTemplatePicker renders the compact horizontal strip of saved-template chips.
// Clicking a chip calls onPick with its template so the caller can pre-fill the
// add form. When no templates exist it shows a subtle prompt instead of nothing,
// so the affordance is discoverable. Each chip is its OWN component (never an On*
// handler inside this variable-length loop).
func TxnTemplatePicker(onPick func(domain.TxnTemplate)) ui.Node {
	tmpls := uistate.TxnTemplates()
	if len(tmpls) == 0 {
		return Div(css.Class("txt-picker"), Attr("data-testid", "txn-tmpl-picker"),
			P(css.Class("txt-empty"), uistate.T("txnTemplates.empty")))
	}
	chips := make([]ui.Node, 0, len(tmpls))
	for _, t := range tmpls {
		chips = append(chips, ui.CreateElement(txnTemplateChip, txnTemplateChipProps{Tmpl: t, OnPick: onPick}))
	}
	return Div(css.Class("txt-picker"), Attr("data-testid", "txn-tmpl-picker"), chips)
}

// txnTemplateChipProps carries one template plus the pre-fill callback.
type txnTemplateChipProps struct {
	Tmpl   domain.TxnTemplate
	OnPick func(domain.TxnTemplate)
}

// txnTemplateChip is one favourite as a clickable pill (name + amount) with a small
// ✕ delete affordance revealed on hover. Its own component so the click/delete hooks
// stay at a stable position across the variable-length picker strip.
func txnTemplateChip(props txnTemplateChipProps) ui.Node {
	t := props.Tmpl

	pick := ui.UseEvent(func(ui.Event) {
		if props.OnPick != nil {
			props.OnPick(t)
		}
	})
	del := ui.UseEvent(Prevent(func() {
		uistate.ConfirmModal(uistate.T("txnTemplates.deleteConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			uistate.DeleteTxnTemplate(t.ID)
			uistate.PostNotice(uistate.T("txnTemplates.deleted"), false)
			uistate.BumpDataRevision()
		})
	}))

	label := t.Name
	if label == "" {
		label = t.Payee
	}
	amt := fmtMoney(money.New(t.SignedMinor(), t.Currency))

	return Button(css.Class("txt-chip"), Type("button"),
		Attr("data-testid", "txn-tmpl-chip-"+t.ID),
		Attr("title", uistate.T("txnTemplates.use")),
		OnClick(pick),
		Span(css.Class("txt-chip-name"), label),
		Span(css.Class("txt-chip-amt"), amt),
		Span(css.Class("txt-chip-del"), Attr("role", "button"),
			Attr("data-testid", "txn-tmpl-chip-del-"+t.ID),
			Attr("aria-label", uistate.T("txnTemplates.delete")),
			OnClick(del), "✕"),
	)
}

// SaveAsTemplateButton renders the "Save as template" action. It is disabled until
// the draft has a non-zero amount (there is nothing useful to save otherwise). On
// click it opens a small name prompt (seeded with the payee) and, on a non-empty
// name, persists the draft as a template and calls onSaved. Its own component so the
// click hook sits at a stable render position.
func SaveAsTemplateButton(draft TxnTemplateDraft, onSaved func()) ui.Node {
	save := ui.UseEvent(Prevent(func() {
		uistate.PromptModal(uistate.T("txnTemplates.namePrompt"), draft.Payee, func(name string) {
			if name == "" {
				return
			}
			uistate.SaveTxnTemplate(domain.TxnTemplate{
				Name:        name,
				Payee:       draft.Payee,
				CategoryID:  draft.CategoryID,
				AccountID:   draft.AccountID,
				AmountMinor: draft.AmountMinor,
				Currency:    draft.Currency,
				Direction:   draft.Direction,
				Note:        draft.Note,
				Tags:        draft.Tags,
			})
			uistate.PostNotice(uistate.T("txnTemplates.saved"), false)
			uistate.BumpDataRevision()
			if onSaved != nil {
				onSaved()
			}
		})
	}))

	args := []any{
		css.Class("txt-save-btn"), Type("button"),
		Attr("data-testid", "txn-tmpl-save"),
		OnClick(save),
		uistate.T("txnTemplates.save"),
	}
	if draft.AmountMinor == 0 {
		args = append(args, Attr("disabled", ""))
	}
	return Button(args...)
}
