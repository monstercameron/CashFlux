// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strconv"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
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
	onNewCat := ui.UseEvent(func() { props.OnCategory(key, catSelectNew) })

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

	// Deliberately NOT memoized. Memoizing this subtree was measured and made no
	// difference (472ms vs 468ms per interaction) because the cost is not here —
	// and any memo key cheap enough to be worth it omitted the category list,
	// which would leave a newly created category missing from this select.
	return Div(css.Class(cls),
		Div(css.Class("rvs-grp-row"),
			// Named, not just testable: an icon-only checkbox in a list of 30
			// merchants is unusable without the merchant in its accessible name.
			Input(css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "review-pick-"+key),
				Attr("aria-label", uistate.T("review.pickAria", g.Merchant, len(g.Items))),
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
				Button(css.Class("rvs-newcat"), Type("button"),
					Attr("data-testid", "review-newcat-"+key),
					Attr("aria-label", uistate.T("catpick.newOption")),
					Attr("title", uistate.T("catpick.newOption")),
					OnClick(onNewCat), "+"),
				Span(css.Class("rvs-grp-total"), fmtMoney(money.Money{Amount: g.Total(), Currency: g.Items[0].Amount.Currency})),
				Button(css.Class("rvs-caret"), Type("button"), Attr("aria-expanded", boolAttr(props.Open)),
					Attr("aria-label", uistate.T("review.expandAria", g.Merchant)),
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
	Index reviewIndex
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
		// Resolve through the precomputed map: this was a nested scan of every
		// transaction per link member, on every render.
		members := make([]domain.Transaction, 0, len(l.TxnIDs))
		for _, id := range l.TxnIDs {
			if o, ok := props.Index.TxnByID[id]; ok {
				members = append(members, o)
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
	// the flow used to make silently. Detection runs ONCE when the index is built;
	// re-running it over the whole ledger per render was pure waste.
	if ids := props.Index.Dupes[t.ID]; len(ids) >= 2 {
		rows := make([]domain.Transaction, 0, len(ids))
		for _, id := range ids {
			if o, ok := props.Index.TxnByID[id]; ok {
				rows = append(rows, o)
			}
		}
		blocks = append(blocks, Div(css.Class("rvs-ctx is-warn"), Attr("data-testid", "review-dupe"),
			Div(css.Class("rvs-ctx-t"), Strong(uistate.T("review.dupeTitle", len(ids)))),
			Fragment(nodesToAny(mapTxnRows(rows, props.Prefs))...),
		))
	}

	// 4. Is this normal for the merchant?
	charges := make([]merchantstats.Charge, 0, len(props.Row.Group.Items))
	for _, o := range props.Row.Group.Items {
		charges = append(charges, merchantstats.Charge{Minor: absInt64(o.Amount.Amount), Date: o.Date})
	}
	if len(charges) >= 3 {
		st := merchantstats.Compute(charges, time.Now(), time.Sunday)
		// C599: no baseline, nothing to say. A delta measured against a zero typical
		// is the whole charge, and phrasing it reads as "all of this is unusual".
		if st.HasTypical() {
			// DeltaVsTypical compares magnitudes; this used to hand it the SIGNED
			// amount, so a $6.75 charge against a $6.90 typical came out as
			// (−675) − 690 = −1365 and the card said "$13.65 above". Wrong size,
			// wrong direction, and the heading asserted "bigger than usual" over a
			// charge that was fifteen cents smaller.
			delta := st.DeltaVsTypical(t.Amount.Amount)
			if delta != 0 {
				title, sub := "review.typicalTitleAbove", "review.typicalSubAbove"
				if delta < 0 {
					title, sub = "review.typicalTitleBelow", "review.typicalSubBelow"
				}
				blocks = append(blocks, Div(css.Class("rvs-ctx"), Attr("data-testid", "review-typical"),
					Div(css.Class("rvs-ctx-t"),
						Strong(uistate.T(title)),
						Span(css.Class("rvs-ctx-why"),
							uistate.T(sub,
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
type reviewScanStripProps struct {
	// Gaps is how many merchants the LOCAL sources could not answer — the only
	// thing a paid scan is worth running on.
	Gaps int
	// GapCharges is how many charges those merchants account for.
	GapCharges int
	// State is idle | scanning | done.
	State string
	// Filled/Skipped describe a finished scan.
	Filled, Skipped int
	// Err is a failed scan's message.
	Err string
	// HasProvider reports whether a key/proxy is configured.
	HasProvider bool
	OnScan      any
	OnUse       any
	CanUse      bool
}

// reviewScanStrip states the scope and cost of a SMART+ scan BEFORE the button
// (C504/C509), then reports what it actually did.
//
// It renders even with NO provider configured: a feature the user cannot see is
// a feature they do not have, and the strip is where the paid tier explains
// itself. The scan stays an explicit consent step — nothing leaves the device
// until it is clicked.
func reviewScanStrip(props reviewScanStripProps) ui.Node {
	// Rough, honest, stated before spending: ~120 tokens per charge at the cheap
	// model rate. An approximate number up front beats an exact one afterwards.
	batch := props.GapCharges
	if batch > smartCatScanCap {
		batch = smartCatScanCap
	}
	cost := "$" + strconv.FormatFloat(float64(batch)*0.0007, 'f', 3, 64)

	title, sub := uistate.T("review.scanTitle"), uistate.T("review.scanSub", smartCatScanCap, cost)
	var action ui.Node = Fragment()
	cls := "rvs-smart"

	switch {
	case !props.HasProvider:
		sub = uistate.T("review.scanNoKey")
		action = Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "review-scan-setup"),
			OnClick(props.OnScan), uistate.T("review.scanConnect"))
	case props.Gaps == 0:
		title = uistate.T("review.scanNoGaps")
		sub = uistate.T("review.scanNoGapsSub")
	case props.State == "scanning":
		title = uistate.T("review.scanning", batch)
		sub = uistate.T("review.scanningSub")
		cls += " is-busy"
	case props.State == "done":
		cls += " is-done"
		title = uistate.T("review.scanDone", props.Filled, props.Gaps)
		sub = uistate.T("review.scanDoneSub", props.Skipped, cost)
		if props.Err != "" {
			title, sub = uistate.T("review.scanFailed"), props.Err
			cls = "rvs-smart is-err"
		}
		// There must ALWAYS be a way forward from here. Previously the done state
		// could render with no button at all — a reply the parser rejected left
		// "filled 0" and nothing to click, so the only way to retry was closing
		// and reopening the whole surface. That reads as "it doesn't work".
		retry := Button(css.Class("btn btn-sm"), Type("button"),
			Attr("data-testid", "review-scan-again"), OnClick(props.OnScan),
			uistate.T("review.scanAgain"))
		if props.CanUse {
			action = Fragment(retry,
				Button(css.Class("btn btn-primary btn-sm"), Type("button"),
					Attr("data-testid", "review-scan-use"), OnClick(props.OnUse),
					uistate.T("review.useSuggested")))
		} else {
			action = retry
		}
	default:
		action = Button(css.Class("btn btn-primary btn-sm"), Type("button"),
			Attr("data-testid", "review-scan"), OnClick(props.OnScan),
			uistate.T("review.scanBtn", batch))
	}

	return Div(css.Class(cls), Attr("data-testid", "review-scan-strip"),
		Attr("data-state", props.State),
		Span(css.Class("rvs-smart-mark"), "S+"),
		Div(css.Class("rvs-smart-main"),
			Div(css.Class("rvs-smart-t"), Attr("data-testid", "review-scan-title"), title),
			Div(css.Class("rvs-smart-sub"), sub),
		),
		action,
	)
}
