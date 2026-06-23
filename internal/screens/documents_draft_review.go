//go:build js && wasm

package screens

import (
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

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

	// Build per-row components, flagging duplicates (G14 §4).
	items := make([]ui.Node, 0, len(rows))
	for i, r := range rows {
		dup := false
		if props.SeenSigs != nil {
			dup = props.SeenSigs[r.Signature()]
		}
		items = append(items, ui.CreateElement(DraftRow, draftRowProps{
			Index:       i,
			Row:         r,
			Currency:    props.ReviewCur,
			Categories:  props.Categories,
			IsDuplicate: dup,
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
			remainderLine = P(css.Class("muted"), "Lines add up to the total — ready to import as one transaction.")
		case residErr != nil:
			remainderLine = P(css.Class("err"), Attr("role", "alert"), "Check the amounts — one couldn't be read as a number.")
		default:
			off := resid
			if off < 0 {
				off = -off
			}
			remainderLine = P(css.Class("err"), Attr("role", "alert"),
				"Lines are off from the total by "+fmtMoney(money.New(off, recCur))+" — adjust the lines or the total to import.")
		}

		importBtn := []any{css.Class("btn btn-primary"), Type("submit")}
		if !reconciled {
			importBtn = append(importBtn, Attr("disabled", "disabled"))
		}
		importBtn = append(importBtn, "Import receipt")

		footer = Div(
			Div(css.Class("form-grid"),
				Input(css.Class("field"), Type("text"),
					Attr("aria-label", "Store name (optional)"),
					Placeholder("Store name (optional)"),
					Value(props.ReceiptMerchant),
					OnInput(props.OnReceiptMerchant),
				),
				Input(css.Class("field"), Type("text"),
					Attr("aria-label", "Receipt total"),
					Placeholder("Receipt total"),
					Value(props.ReceiptTotal),
					OnInput(props.OnReceiptTotal),
				),
			),
			remainderLine,
			Form(css.Class("form-grid"), OnSubmit(props.OnImportReceipt),
				Select(css.Class("field"), Attr("aria-label", "Import into account"),
					OnChange(props.OnAcctChange), acctOptions),
				Button(importBtn...),
			),
		)
	} else {
		footer = Form(css.Class("form-grid"), OnSubmit(props.OnImportDraft),
			Select(css.Class("field"), Attr("aria-label", "Import into account"), OnChange(props.OnAcctChange), acctOptions),
			Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.importThese")),
		)
	}

	// G14 §1 / §7: count how many rows are already-imported duplicates so the
	// banner can show an actionable summary.
	dupCount := 0
	for _, r := range rows {
		if props.SeenSigs != nil && props.SeenSigs[r.Signature()] {
			dupCount++
		}
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
					Span(plural(dupCount, "row")+" of "+plural(len(rows), "row")+" already imported — will be skipped."),
				),
			),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter, tw.Mt1),
				Button(css.Class("btn btn-sm"), Type("button"), OnClick(props.ClearDraft),
					"Start over"),
			),
			P(css.Class("muted"), uistate.T("documents.reviewDesc")),
			props.Toggle,
			Div(css.Class("rows"), items),
			footer,
		),
	})
}
