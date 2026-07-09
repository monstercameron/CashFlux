// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// draftSumItem renders one figure of the statement summary (a small uppercase
// label above a bold amount). tone "in"/"out" tints the amount with the app's
// money-positive/negative tokens; anything else leaves it neutral.
func draftSumItem(label, value, tone string) ui.Node {
	valCls := "draft-sum-val fig"
	switch tone {
	case "in":
		valCls += " amount-income"
	case "out":
		valCls += " amount-expense"
	}
	return Div(css.Class("draft-sum-item"),
		Span(css.Class("draft-sum-label"), label),
		Span(css.Class(valCls), value),
	)
}

// draftSumNet renders the net figure — the summary's emphasized close — tinted by
// its own sign and pushed to the right so it foots the amount column like a
// statement's bottom line.
func draftSumNet(netMinor int64, cur string) ui.Node {
	cls := "draft-sum-val draft-sum-net fig"
	switch {
	case netMinor > 0:
		cls += " amount-income"
	case netMinor < 0:
		cls += " amount-expense"
	}
	return Div(css.Class("draft-sum-item draft-sum-item-net"),
		Span(css.Class("draft-sum-label"), uistate.T("documents.sumNet")),
		Span(css.Class(cls), fmtMoney(money.New(netMinor, cur))),
	)
}

// draftReviewListProps carries the state and callbacks the draft-review list
// needs from the parent Documents() component.
type draftReviewListProps struct {
	Rows            []extract.Row
	Accounts        []domain.Account
	Categories      []domain.Category
	ReviewCur       string
	ImportAcctID    string
	ReceiptMode     bool
	ReceiptTotal    string
	ReceiptMerchant string
	RecBaseCur      string // base currency for receipt math

	// SeenSigs holds existing transaction signatures for the chosen account so
	// DraftReviewList can badge rows that would be skipped as duplicates (G14 §4).
	SeenSigs map[string]bool

	// ClearDraft clears all pending rows (G14 §1 / "Start over" action).
	ClearDraft ui.Handler

	// Toggle is the pre-built receipt-mode ToggleRow node (built in Documents()
	// so the OnChange handler has direct access to state setters).
	Toggle ui.Node

	OnAcctChange      ui.Handler
	OnReceiptTotal    ui.Handler
	OnReceiptMerchant ui.Handler
	OnImportDraft     ui.Handler
	OnImportReceipt   ui.Handler
	OnRemoveDraft     func(int)
	OnUpdateDraft     func(int, extract.Row)
}

// DraftReviewList renders the reviewed-rows panel: editable rows, account
// selector, receipt-mode toggle, and the appropriate import button/form.
// Returns nil when there are no rows (so Documents() renders nothing in that slot).
func DraftReviewList(props draftReviewListProps) ui.Node {
	rows := props.Rows
	if len(rows) == 0 {
		return nil
	}

	// Build per-row components, flagging duplicates (G14 §4). A row is a duplicate
	// if it matches an already-imported transaction (props.SeenSigs) OR an earlier
	// row in this same batch (C17 — within-batch repeats are skipped on import too,
	// so they should badge identically rather than look importable).
	items := make([]ui.Node, 0, len(rows))
	batchSeen := make(map[string]bool, len(rows))
	for i, r := range rows {
		sig := r.Signature()
		dup := false
		if props.SeenSigs != nil {
			dup = props.SeenSigs[sig]
		}
		if batchSeen[sig] {
			dup = true
		}
		batchSeen[sig] = true
		items = append(items, ui.CreateElement(DraftRow, draftRowProps{
			Index:       i,
			Row:         r,
			Currency:    props.ReviewCur,
			Categories:  props.Categories,
			IsDuplicate: dup,
			// Tint amounts by direction only for statements (signed money in/out);
			// receipt line items are all positive purchase prices, so coloring them
			// green as "income" would mislead.
			ColorBySign: !props.ReceiptMode,
			OnRemove:    props.OnRemoveDraft,
			OnUpdate:    props.OnUpdateDraft,
		}))
	}

	acctOptions := make([]ui.Node, 0, len(props.Accounts))
	for _, a := range props.Accounts {
		acctOptions = append(acctOptions, Option(Value(a.ID), SelectedIf(props.ImportAcctID == a.ID), a.Name))
	}

	// Receipt math uses the base currency (matches appstate.ImportReceipt).
	recCur := props.RecBaseCur
	if recCur == "" {
		recCur = "USD"
	}
	recDec := currency.Decimals(recCur)

	// Footer: either receipt import form or plain import form.
	var footer ui.Node
	if props.ReceiptMode {
		recLines := make([]extract.ReceiptLine, 0, len(rows))
		for _, r := range rows {
			recLines = append(recLines, extract.ReceiptLine{
				Description: r.Description,
				Category:    r.Category,
				Amount:      absAmount(r.Amount),
			})
		}
		resid, residErr := (extract.Receipt{
			Total: absAmount(props.ReceiptTotal),
			Lines: recLines,
		}).Residual(recDec)
		reconciled := residErr == nil && resid == 0

		var remainderLine ui.Node
		switch {
		case reconciled:
			remainderLine = P(css.Class("muted"), uistate.T("documents.linesReconciled"))
		case residErr != nil:
			remainderLine = P(css.Class("err"), Attr("role", "alert"), uistate.T("documents.linesUnreadable"))
		default:
			off := resid
			if off < 0 {
				off = -off
			}
			remainderLine = P(css.Class("err"), Attr("role", "alert"),
				uistate.T("documents.linesOffBy", fmtMoney(money.New(off, recCur))))
		}

		importBtn := []any{css.Class("btn btn-primary"), Type("submit")}
		if !reconciled {
			importBtn = append(importBtn, Attr("disabled", "disabled"))
		}
		importBtn = append(importBtn, uistate.T("documents.importReceipt"))

		footer = Div(
			Div(css.Class("form-grid"),
				Input(css.Class("field"), Type("text"),
					Attr("aria-label", uistate.T("documents.storeName")),
					Placeholder(uistate.T("documents.storeNamePh")),
					Value(props.ReceiptMerchant),
					OnInput(props.OnReceiptMerchant),
				),
				Input(css.Class("field"), Type("text"),
					Attr("aria-label", uistate.T("documents.receiptTotal")),
					Placeholder(uistate.T("documents.receiptTotal")),
					Value(props.ReceiptTotal),
					OnInput(props.OnReceiptTotal),
				),
			),
			remainderLine,
			Form(css.Class("form-grid"), OnSubmit(props.OnImportReceipt),
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.importAccount")),
					OnChange(props.OnAcctChange), acctOptions),
				Button(importBtn...),
			),
		)
	} else {
		// Plain (statement) mode: the account picker + Import live in the sticky
		// summary bar at the top (built below), so there is no separate bottom
		// footer — one primary action, always reachable.
		footer = Fragment()
		_ = acctOptions
	}

	// C12: a sticky header at the TOP of the review card so the account selector +
	// Import button are reachable without scrolling past a long list of rows. Shown
	// for the plain (statement) import — receipt mode keeps its single bottom form
	// (total/merchant/account belong together there). Above the action row sits a
	// statement-style money-in / money-out / net summary of what will actually be
	// imported (duplicates, which are skipped on import, are excluded from the tally).
	topBar := Fragment()
	if !props.ReceiptMode && len(rows) >= 1 {
		topOpts := make([]ui.Node, 0, len(props.Accounts))
		for _, a := range props.Accounts {
			topOpts = append(topOpts, Option(Value(a.ID), SelectedIf(props.ImportAcctID == a.ID), a.Name))
		}

		dec := currency.Decimals(props.ReviewCur)
		var inMinor, outMinor int64
		importable := 0
		sumSeen := make(map[string]bool, len(rows))
		for _, r := range rows {
			sig := r.Signature()
			if (props.SeenSigs != nil && props.SeenSigs[sig]) || sumSeen[sig] {
				sumSeen[sig] = true
				continue // skipped on import — leave it out of the tally
			}
			sumSeen[sig] = true
			importable++
			if m, err := money.ParseMinor(strings.TrimSpace(r.Amount), dec); err == nil {
				if m >= 0 {
					inMinor += m
				} else {
					outMinor += -m
				}
			}
		}
		netMinor := inMinor - outMinor

		var summary ui.Node = Fragment()
		if inMinor != 0 || outMinor != 0 {
			summary = Div(css.Class("draft-summary"),
				draftSumItem(uistate.T("documents.sumIn"), fmtMoney(money.New(inMinor, props.ReviewCur)), "in"),
				draftSumItem(uistate.T("documents.sumOut"), fmtMoney(money.New(outMinor, props.ReviewCur)), "out"),
				draftSumNet(netMinor, props.ReviewCur),
			)
		}

		importLabel := uistate.T("documents.importThese")
		if importable > 0 {
			importLabel = uistate.T("documents.importN", plural(importable, "transaction"))
		}
		topBar = Div(css.Class("draft-actionbar"),
			summary,
			Form(css.Class("draft-actionrow"), OnSubmit(props.OnImportDraft),
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.importAccount")), OnChange(props.OnAcctChange), topOpts),
				buttonWithDisabled(importable == 0,
					[]any{css.Class("btn btn-primary"), Type("submit")}, importLabel),
			),
		)
	}

	// G14 §1 / §7: count how many rows are duplicates (already-imported OR repeated
	// within this batch — C17) so the banner can show an actionable summary that
	// matches the per-row badges.
	dupCount := 0
	countSeen := make(map[string]bool, len(rows))
	for _, r := range rows {
		sig := r.Signature()
		if (props.SeenSigs != nil && props.SeenSigs[sig]) || countSeen[sig] {
			dupCount++
		}
		countSeen[sig] = true
	}

	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title: uistate.T("documents.reviewTitle", plural(len(rows), "transaction")),
		// G14 §1: colored left-border step indicator — signals this card is an
		// "action required" next step that materialised from the user's last action.
		ClassParts: []any{"card-step-active"},
		Body: Fragment(
			// G14 §1 / §7: contextual state banner — tells the user whether these
			// rows are freshly parsed or persisted from a prior session, and offers
			// a "Start over" escape hatch so they are never stuck with stale rows.
			If(dupCount > 0,
				Div(css.Class("notice notice-warn", tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
					Span(uistate.T("documents.rowsAlreadyImported", plural(dupCount, "row"), plural(len(rows), "row"))),
				),
			),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter, tw.Mt1),
				Button(css.Class("btn btn-sm"), Type("button"), OnClick(props.ClearDraft),
					uistate.T("documents.startOver")),
			),
			P(css.Class("muted"), uistate.T("documents.reviewDesc")),
			props.Toggle,
			topBar,
			Div(css.Class("rows draft-ledger"), items),
			footer,
		),
	})
}
