// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// dupeGroupProps is the props bag passed to a single duplicate-group card.
// Each group gets its own component so that per-row hooks occupy stable
// positions — never called inside the variable-length outer loop.
type dupeGroupProps struct {
	Group    dedupe.Group
	Txns     map[string]domain.Transaction // keyed by ID, for full field access
	AccByID  map[string]domain.Account
	BaseCur  string
	OnDelete func(id string)
	OnMerge  func(g dedupe.Group) // C87: merge-group action
}

// dupeGroup renders one card for a set of likely-duplicate transactions.
// The first transaction is treated as the one to keep (labelled "Keep");
// the remaining duplicates each get a "Delete duplicate" button.
// Because this is its own component (called via ui.CreateElement from the outer
// MapKeyed), UseEvent calls inside it are at unconditional, stable positions.
func dupeGroup(props dupeGroupProps) ui.Node {
	g := props.Group

	// C87: merge event — UseEvent at a stable, unconditional position in this component.
	merge := ui.UseEvent(func() {
		others := len(g.IDs) - 1
		msg := fmt.Sprintf(uistate.T("duplicates.mergeConfirm"), others)
		uistate.ConfirmModal(msg, true, func(ok bool) {
			if ok && props.OnMerge != nil {
				props.OnMerge(g)
			}
		})
	})

	// Format the shared amount for the group header.
	dec := currency.Decimals(g.Currency)
	sym := currency.Symbol(g.Currency)
	absAmt := g.Amount
	if absAmt < 0 {
		absAmt = -absAmt
	}
	amtStr := sym + fmtMinorAmount(absAmt, dec)
	sign := "+"
	if g.Amount < 0 {
		sign = "-"
	}
	amtDisplay := sign + amtStr
	amtClass := "text-up"
	if g.Amount < 0 {
		amtClass = "text-down"
	}

	// Group header: payee · date · amount.
	header := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap4),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), g.Description),
			Span(css.Class("t-caption", tw.TextDim), g.Date),
		),
		Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)+" "+tw.ColorClass(amtClass)), amtDisplay),
	)

	// Badge: number of entries in this group.
	badge := Span(css.Class("badge"),
		fmt.Sprintf(uistate.T("duplicates.groupCount"), len(g.IDs)),
	)

	titleRow := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb3),
		badge,
		Div(css.Class(tw.Flex1)),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.keepNote")),
		Button(
			css.Class("btn-sm"),
			Attr("type", "button"),
			Attr("aria-label", uistate.T("duplicates.mergeAria")),
			OnClick(merge),
			uistate.T("duplicates.mergeBtn"),
		),
	)

	// Per-transaction rows — each is its own component (dupeRow) so that
	// UseEvent hooks are not inside a variable-length loop body.
	rows := MapKeyed(
		g.IDs,
		func(id string) any { return id },
		func(id string) ui.Node {
			t, _ := props.Txns[id]
			accName := ""
			if a, ok := props.AccByID[t.AccountID]; ok {
				accName = a.Name
			}
			isFirst := len(g.IDs) > 0 && id == g.IDs[0]
			return ui.CreateElement(dupeRow, dupeRowProps{
				TxnID:    id,
				Date:     g.Date,
				AccName:  accName,
				IsFirst:  isFirst,
				OnDelete: props.OnDelete,
			})
		},
	)

	return uiw.Card(uiw.CardProps{
		Body: Div(
			titleRow,
			header,
			Div(css.Class(tw.Mt3, tw.Flex, tw.FlexCol, tw.Gap2), rows),
		),
	})
}

// dupeRowProps is the props bag for a single transaction entry within a group.
type dupeRowProps struct {
	TxnID    string
	Date     string
	AccName  string
	IsFirst  bool // first = "keep" row; others = deletable duplicates
	OnDelete func(id string)
}

// dupeRow is a single entry row inside a duplicate group card. It is its own
// component so that UseEvent is called at a stable, unconditional position
// (never inside an outer variable-length loop). The first entry is marked
// "Keep"; all others get a "Delete duplicate" button.
func dupeRow(props dupeRowProps) ui.Node {
	del := ui.UseEvent(func() {
		msg := uistate.T("duplicates.deleteConfirm")
		uistate.ConfirmModal(msg, true, func(ok bool) {
			if ok {
				props.OnDelete(props.TxnID)
			}
		})
	})

	rowClass := tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.Py2)

	if props.IsFirst {
		return Div(css.Class(rowClass, tw.BorderB),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
				Span(css.Class("t-caption", tw.TextDim), props.AccName),
				Span(css.Class("t-caption", tw.TextFaint), uistate.T("duplicates.keepLabel")),
			),
			Span(css.Class("badge"), uistate.T("duplicates.keepBadge")),
		)
	}

	return Div(css.Class(rowClass),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(css.Class("t-caption", tw.TextDim), props.AccName),
			Span(css.Class("t-caption", tw.TextFaint), props.Date),
		),
		Button(
			css.Class("btn-danger-sm"),
			Attr("type", "button"),
			Attr("aria-label", fmt.Sprintf(uistate.T("duplicates.deleteAria"), props.TxnID)),
			OnClick(del),
			uistate.T("duplicates.deleteBtn"),
		),
	)
}

// duplicatesPanelProps is the props bag for DuplicatesPanel. Currently empty
// — the panel reads all its data from appstate.Default — but typed so it can be
// embedded via ui.CreateElement and have its hook state isolated from parents.
type duplicatesPanelProps struct{}

// DuplicatesPanel is the registered component that groups duplicate transactions
// and lets the user delete or merge them. Extracted from DuplicatesScreen() so it
// can be embedded on /transactions without duplicating logic (FEATURE_MAP §5.3 /
// §5.7b). Per-row hook state (UseEvent) lives inside dupeRow; per-group layout
// lives inside dupeGroup. DuplicatesPanel itself holds no per-item hooks — only
// UseDataRevision() to react to data changes.
func DuplicatesPanel(props duplicatesPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()

	txns := app.Transactions()
	accounts := app.Accounts()

	accByID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}
	txnByID := make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		txnByID[t.ID] = t
	}

	groups := dedupe.FindDuplicates(txns)
	total := dedupe.Count(groups)

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	// Plain func passed down as a prop — no hook here.
	deleteTxn := func(id string) {
		if err := app.DeleteTransaction(id); err != nil {
			uistate.PostNotice(err.Error(), false)
			return
		}
		uistate.PostUndoable(uistate.T("duplicates.deleted"))
	}

	// C87: merge a duplicate group — keep the first entry (union tags/cleared),
	// delete the rest. No hook here; plain func passed as a prop.
	mergeTxns := func(g dedupe.Group) {
		if len(g.IDs) < 2 {
			return
		}
		survivorID := g.IDs[0]
		survivor, ok := txnByID[survivorID]
		if !ok {
			return
		}
		others := make([]domain.Transaction, 0, len(g.IDs)-1)
		for _, id := range g.IDs[1:] {
			if t, ok := txnByID[id]; ok {
				others = append(others, t)
			}
		}
		merged := dedupe.Merge(survivor, others)
		if err := app.PutTransaction(merged); err != nil {
			uistate.PostNotice(err.Error(), false)
			return
		}
		for _, t := range others {
			if err := app.DeleteTransaction(t.ID); err != nil {
				uistate.PostNotice(err.Error(), false)
				return
			}
		}
		uistate.PostUndoable(uistate.T("duplicates.merged"))
	}

	_ = base // reserved for future per-group currency display

	// Empty state.
	if len(groups) == 0 {
		return uiw.Card(uiw.CardProps{
			Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("duplicates.emptyTitle")),
				P(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.emptyBody")),
			),
		})
	}

	// Summary banner.
	summary := uiw.Card(uiw.CardProps{
		Body: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3),
			Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.Gap1),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)),
					fmt.Sprintf(uistate.T("duplicates.headline"), total, len(groups))),
				P(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.hint")),
			),
		),
	})

	// One card per duplicate group.
	groupCards := MapKeyed(
		groups,
		func(g dedupe.Group) any {
			if len(g.IDs) > 0 {
				return g.IDs[0]
			}
			return g.Date + "|" + g.Description
		},
		func(g dedupe.Group) ui.Node {
			return ui.CreateElement(dupeGroup, dupeGroupProps{
				Group:    g,
				Txns:     txnByID,
				AccByID:  accByID,
				BaseCur:  base,
				OnDelete: deleteTxn,
				OnMerge:  mergeTxns,
			})
		},
	)

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		summary,
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4), groupCards),
	)
}

// DuplicatesScreen is the /duplicates route — a thin shell that delegates
// entirely to DuplicatesPanel. Routes remain registered (pending rail regroup);
// logic lives in DuplicatesPanel so it can also be embedded on /transactions.
func DuplicatesScreen() ui.Node {
	return ui.CreateElement(DuplicatesPanel, duplicatesPanelProps{})
}
