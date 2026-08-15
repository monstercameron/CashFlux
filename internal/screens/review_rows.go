// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/merchantstats"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// reviewGroupRowProps drives one merchant row in bulk mode.
type reviewGroupRowProps struct {
	Row            reviewRow
	App            *appstate.App
	Selected       bool
	Open           bool
	Focused        bool
	CategoryID     string
	Manual         bool
	Prefs          prefs.Prefs
	OnToggleOpen   func(string)
	OnToggleSelect func(string)
	OnCategory     func(string, string)
}

// reviewGroupRow is one merchant group: its own component so the checkbox,
// caret and select hooks are registered at stable positions rather than inside
// the list loop (the framework's top gotcha).
func reviewGroupRow(props reviewGroupRowProps) ui.Node {
	key := props.Row.Group.Key
	toggleOpen := ui.UseEvent(func() { props.OnToggleOpen(key) })
	toggleSel := ui.UseEvent(func() { props.OnToggleSelect(key) })
	onCat := ui.UseEvent(func(e ui.Event) { props.OnCategory(key, e.GetValue()) })

	g := props.Row.Group
	kind := domain.KindExpense
	if len(g.Items) > 0 && g.Items[0].Amount.Amount > 0 {
		kind = domain.KindIncome
	}
	selArgs := []any{css.Class("rvs-cat"), Attr("data-testid", "review-cat-"+key),
		Attr("aria-label", uistate.T("review.categoryLabel")), OnChange(onCat)}
	if props.Manual {
		selArgs = append(selArgs, Attr("data-manual", "true"))
	}
	selArgs = append(selArgs, categorySelectNodes(props.App.Categories(), kind, props.CategoryID)...)

	cls := "rvs-grp"
	if props.Selected {
		cls += " is-sel"
	}
	if props.Open {
		cls += " is-open"
	}
	if props.Focused {
		cls += " is-focus"
	}
	countLabel := uistate.T("review.chargeCount", plural(len(g.Items), "charge"))

	// Only the first few charges render until the group is expanded: a 121-charge
	// merchant must not put 121 rows in the DOM to show a total (C510).
	shown := g.Items
	const preview = 6
	extra := 0
	if len(shown) > preview {
		extra = len(shown) - preview
		shown = shown[:preview]
	}

	return Div(css.Class(cls),
		Div(css.Class("rvs-grp-row"),
			Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "review-pick-"+key),
				CheckedIf(props.Selected), OnClick(toggleSel)),
			Div(css.Class("rvs-grp-main"),
				Div(css.Class("rvs-grp-name"),
					Strong(g.Merchant),
					Span(css.Class("rvs-grp-n"), countLabel),
				),
				Div(css.Class("rvs-grp-raw"), rawPayeeOf(g.Items[0])),
			),
			Div(css.Class("rvs-grp-right"),
				Span(css.Class("rvs-dot "+tierMod(props.Row.Tier))),
				Select(selArgs...),
				Span(css.Class("rvs-grp-total"), fmtMoney(money.Money{Amount: g.Total(), Currency: g.Items[0].Amount.Currency})),
				Button(css.Class("rvs-caret"), Type("button"), Attr("aria-expanded", boolAttr(props.Open)),
					Attr("data-testid", "review-expand-"+key), OnClick(toggleOpen), "▸"),
			),
		),
		If(props.Open, Div(css.Class("rvs-members"),
			MapKeyed(shown, func(t domain.Transaction) any { return t.ID }, func(t domain.Transaction) ui.Node {
				return Div(css.Class("rvs-mem"),
					Span(css.Class("rvs-mem-date"), props.Prefs.FormatDate(t.Date)),
					Span(css.Class("rvs-mem-desc"), rawPayeeOf(t)),
					Span(css.Class("rvs-mem-amt"), fmtMoney(t.Amount)),
				)
			}),
			If(extra > 0, P(css.Class("rvs-more"), uistate.T("review.showOther", extra))),
		)),
	)
}

// reviewContextBandProps drives the single-mode "what else this is tied to" band.
type reviewContextBandProps struct {
	App   *appstate.App
	Row   reviewRow
	Txn   domain.Transaction
	Prefs prefs.Prefs
}

// reviewContextBand renders the linked-transaction context (C503): the other
// queued charges from this merchant, any order group or refund pair, duplicate
// candidates, and how this charge compares with the merchant's usual. All of it
// was already computed somewhere in the app and read nowhere in this flow.
func reviewContextBand(props reviewContextBandProps) ui.Node {
	app, t := props.App, props.Txn
	if app == nil || t.ID == "" {
		return Fragment()
	}
	var blocks []ui.Node

	// 1. Other queued charges from the same merchant — the LIST, not just a count.
	sibs := make([]domain.Transaction, 0)
	for _, o := range props.Row.Group.Items {
		if o.ID != t.ID {
			sibs = append(sibs, o)
		}
	}
	if len(sibs) > 0 {
		shown := sibs
		extra := 0
		if len(shown) > 5 {
			extra = len(shown) - 5
			shown = shown[:5]
		}
		var total int64
		for _, s := range sibs {
			total += s.Amount.Amount
		}
		blocks = append(blocks, Div(css.Class("rvs-ctx"), Attr("data-testid", "review-siblings"),
			Div(css.Class("rvs-ctx-t"),
				Strong(uistate.T("review.sibTitle", len(sibs))),
				Span(css.Class("rvs-ctx-why"), fmtMoney(money.Money{Amount: total, Currency: t.Amount.Currency})),
			),
			Fragment(nodesToAny(mapTxnRows(shown, props.Prefs))...),
			If(extra > 0, P(css.Class("rvs-more"), uistate.T("review.showOther", extra))),
		))
	}

	// 2. Linked transactions: order groups, refund pairs, bill matches.
	for _, l := range app.TxnLinks() {
		if !l.HasTxn(t.ID) || len(l.TxnIDs) < 2 {
			continue
		}
		members := make([]domain.Transaction, 0, len(l.TxnIDs))
		for _, id := range l.TxnIDs {
			for _, o := range app.Transactions() {
				if o.ID == id {
					members = append(members, o)
					break
				}
			}
		}
		if len(members) < 2 {
			continue
		}
		title := uistate.T("review.linkOrder", len(members))
		if l.Kind == domain.TxnLinkRefundPair {
			title = uistate.T("review.linkRefund")
		}
		blocks = append(blocks, Div(css.Class("rvs-ctx"), Attr("data-testid", "review-link"),
			Div(css.Class("rvs-ctx-t"), Strong(title)),
			Fragment(nodesToAny(mapTxnRows(members, props.Prefs))...),
		))
		break // one link band is enough context; more would bury the decision
	}

	// 3. Duplicate candidates — categorizing both halves of a dupe is a mistake
	// the flow used to make silently.
	for _, grp := range dedupe.FindDuplicates(app.Transactions()) {
		hit := false
		for _, d := range grp.IDs {
			if d == t.ID {
				hit = true
				break
			}
		}
		if !hit || len(grp.IDs) < 2 {
			continue
		}
		blocks = append(blocks, Div(css.Class("rvs-ctx is-warn"), Attr("data-testid", "review-dupe"),
			Div(css.Class("rvs-ctx-t"), Strong(uistate.T("review.dupeTitle", len(grp.IDs)))),
			Fragment(nodesToAny(mapTxnRows(dupeTxns(app, grp), props.Prefs))...),
		))
		break
	}

	// 4. Is this normal for the merchant?
	charges := make([]merchantstats.Charge, 0, len(props.Row.Group.Items))
	for _, o := range props.Row.Group.Items {
		charges = append(charges, merchantstats.Charge{Minor: absInt64(o.Amount.Amount), Date: o.Date})
	}
	if len(charges) >= 3 {
		st := merchantstats.Compute(charges, time.Now(), time.Sunday)
		if st.TypicalMinor != 0 {
			delta := st.DeltaVsTypical(t.Amount.Amount)
			if delta != 0 {
				blocks = append(blocks, Div(css.Class("rvs-ctx"), Attr("data-testid", "review-typical"),
					Div(css.Class("rvs-ctx-t"),
						Strong(uistate.T("review.typicalTitle")),
						Span(css.Class("rvs-ctx-why"),
							uistate.T("review.typicalSub",
								fmtMoney(money.Money{Amount: st.TypicalMinor, Currency: t.Amount.Currency}),
								fmtMoney(money.Money{Amount: absInt64(delta), Currency: t.Amount.Currency}))),
					),
				))
			}
		}
	}

	if len(blocks) == 0 {
		return Fragment()
	}
	return Div(css.Class("rvs-band"),
		Div(css.Class("rvs-band-head"), uistate.T("review.bandTitle")),
		Fragment(nodesToAny(blocks)...),
	)
}

func mapTxnRows(in []domain.Transaction, pr prefs.Prefs) []ui.Node {
	out := make([]ui.Node, 0, len(in))
	for _, t := range in {
		out = append(out, Div(css.Class("rvs-link-row"),
			Span(pr.FormatDate(t.Date)),
			Span(css.Class("rvs-link-amt"), fmtMoney(t.Amount)),
			Span(css.Class("rvs-link-desc"), truncate(rawPayeeOf(t), 32)),
		))
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// reviewScanStripProps drives the SMART+ scan strip.
type reviewScanStripProps struct{ Queued int }

// reviewScanStrip states the scope and cost of a SMART+ scan BEFORE the button
// (C504/C509). The scan stays an explicit consent step: nothing leaves the
// device until it is clicked.
func reviewScanStrip(props reviewScanStripProps) ui.Node {
	app := appstate.Default
	pr := uistate.UsePrefs().Get()
	backendAI := pr.Normalize().BackendActive()
	hasProvider := app != nil && aiProviderConfigured(app, backendAI)
	openSmart := ui.UseEvent(func() { uistate.UseTxnSmartCatOpen().Set(true) })
	if !hasProvider {
		return Fragment()
	}
	batch := props.Queued
	if batch > smartCatScanCap {
		batch = smartCatScanCap
	}
	// Rough, honest, and stated before spending: ~120 tokens per charge at the
	// cheap-model rate. Better an approximate number up front than an exact one
	// after the money is gone.
	cost := "$" + strconv.FormatFloat(float64(batch)*0.0007, 'f', 3, 64)
	return Div(css.Class("rvs-smart"), Attr("data-testid", "review-scan-strip"),
		Span(css.Class("rvs-smart-mark"), "S+"),
		Div(css.Class("rvs-smart-main"),
			Div(css.Class("rvs-smart-t"), uistate.T("review.scanTitle")),
			Div(css.Class("rvs-smart-sub"), uistate.T("review.scanSub", smartCatScanCap, cost)),
		),
		Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-scan"), OnClick(openSmart),
			uistate.T("review.scanBtn", batch)),
	)
}

// dupeTxns resolves a duplicate group's ids back to transactions.
func dupeTxns(app *appstate.App, g dedupe.Group) []domain.Transaction {
	out := make([]domain.Transaction, 0, len(g.IDs))
	for _, id := range g.IDs {
		for _, t := range app.Transactions() {
			if t.ID == id {
				out = append(out, t)
				break
			}
		}
	}
	return out
}
