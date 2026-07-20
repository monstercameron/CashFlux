// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/attribution"
	"github.com/monstercameron/CashFlux/internal/balancesheet"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// The DETAIL view: the full balance sheet as a numbered document, following the
// Reports full-report conventions (numbered sections, a chip index that jumps
// between them) because these sections ARE a sequence a reader walks — where
// you stand, what changed, what you own, what you owe, the history, the health.
//
// Every figure here comes from the SAME nwsView the Glance view reads, which is
// the mechanical reason the two views cannot disagree. Nothing is capped: where
// Glance summarizes, Detail enumerates.

// nwsDetailSections are the numbered sections, in reading order. Kept as data
// so the index and the document can never fall out of step.
var nwsDetailSections = []struct{ ID, Num, TitleKey string }{
	{"nws-00", "00", "nws.secStand"},
	{"nws-01", "01", "nws.secChanged"},
	{"nws-02", "02", "nws.secOwn"},
	{"nws-03", "03", "nws.secOwe"},
	{"nws-04", "04", "nws.secHistory"},
	{"nws-05", "05", "nws.secHealth"},
}

// nwsIndex is the sticky chip index. Items are buttons that scroll their
// section into view rather than hash anchors, which would push a fragment URL
// through the SPA router.
func nwsIndex() ui.Node {
	scrollTo := func(id string) func() {
		return func() {
			if el := js.Global().Get("document").Call("getElementById", id); el.Truthy() {
				el.Call("scrollIntoView", map[string]any{"behavior": "smooth", "block": "start"})
			}
		}
	}
	item := func(id, num, key string) ui.Node {
		label := uistate.T(key)
		return Button(css.Class("nws-idx"), Type("button"),
			Attr("data-testid", "nws-idx-"+num), Attr("data-section", id),
			Title(label), OnClick(scrollTo(id)),
			Span(css.Class("nws-idx-num"), num),
			Span(label),
		)
	}
	// Written out rather than looped: On* prop options register hooks, which must
	// sit at stable render positions (CLAUDE.md, "CRITICAL gotchas").
	s := nwsDetailSections
	return Nav(css.Class("nws-index"), Attr("data-testid", "nws-index"),
		Attr("aria-label", uistate.T("nws.indexAria")),
		item(s[0].ID, s[0].Num, s[0].TitleKey),
		item(s[1].ID, s[1].Num, s[1].TitleKey),
		item(s[2].ID, s[2].Num, s[2].TitleKey),
		item(s[3].ID, s[3].Num, s[3].TitleKey),
		item(s[4].ID, s[4].Num, s[4].TitleKey),
		item(s[5].ID, s[5].Num, s[5].TitleKey),
	)
}

// nwsDetailSection is one numbered document section.
func nwsDetailSection(id, num, title, note string, action, body ui.Node) ui.Node {
	if action == nil {
		action = Fragment()
	}
	args := []any{css.Class("nws-section"), Attr("id", id), Attr("data-testid", id),
		Div(css.Class("nws-sec-head"),
			Div(css.Class("nws-dsec-head"),
				Span(css.Class("nws-dsec-num"), num),
				H2(css.Class("nws-sec-title"), title),
			),
			action,
		),
	}
	if note != "" {
		args = append(args, P(css.Class("nws-sec-note"), note))
	}
	args = append(args, body)
	return Div(args...)
}

// nwsMoneyCell renders one right-aligned figure cell.
func nwsMoneyCell(minor int64, base string) ui.Node {
	return Td(css.Class("nws-num"), fmtMoney(money.New(minor, base)))
}

// nwsSignedCell renders a signed movement, toned only when it is a gain — a
// negative movement is stated plainly rather than alarmed about.
func nwsSignedCell(minor int64, base string) ui.Node {
	sign := "+"
	if minor < 0 {
		sign = "−"
	}
	cls := "nws-num"
	if minor > 0 {
		cls += " " + tw.ColorClass("text-up")
	}
	return Td(ClassStr(cls), sign+fmtMoney(money.New(absMinor(minor), base)))
}

// nwsStandSection (00) states the balance sheet as three figures and nothing else.
func nwsStandSection(v nwsView) ui.Node {
	p := v.Latest()
	body := Div(css.Class("nws-scroll"),
		Table(css.Class("nws-table"), Attr("data-testid", "nws-stand-table"),
			Tbody(
				Tr(Td(uistate.T("accounts.assets")), nwsMoneyCell(p.AssetsMinor, v.Base)),
				Tr(Td(uistate.T("dashboard.liabilities")), nwsMoneyCell(p.LiabilitiesMinor, v.Base)),
				Tr(css.Class("nws-total"),
					Td(uistate.T("dashboard.netWorth")),
					Td(css.Class("nws-num"), Attr("data-testid", "nws-detail-net"),
						fmtMoney(money.New(p.NetMinor, v.Base))),
				),
			),
		),
	)
	return nwsDetailSection("nws-00", "00", uistate.T("nws.secStand"), uistate.T("nws.secStandNote"), nil, body)
}

// nwsChangedSection (01) carries the full bridge — every leg with what it
// contains and the residual stated outright — plus every account that moved.
func nwsChangedSection(v nwsView) ui.Node {
	b := v.Bridge
	rows := []any{}
	for _, k := range attribution.BridgeLegOrder {
		amt := b.Leg(k)
		if amt == 0 && k != attribution.LegResidual {
			continue
		}
		rows = append(rows, Tr(Attr("data-testid", "nws-leg-row"), Attr("data-leg", string(k)),
			Td(Div(nwsLegLabel(k)),
				Div(css.Class("nws-sec-note"), Style(map[string]string{"margin": "0"}), nwsLegExplain(k))),
			nwsSignedCell(amt, v.Base),
		))
	}
	legTable := Div(css.Class("nws-scroll"),
		Table(css.Class("nws-table"), Attr("data-testid", "nws-bridge-table"),
			Thead(Tr(Th(uistate.T("nws.colLeg")), Th(css.Class("nws-num"), uistate.T("nws.colEffect")))),
			Tbody(append([]any{
				Tr(Attr("data-testid", "nws-leg-row"), Attr("data-leg", "start"),
					Td(uistate.T("nws.legStart")), nwsMoneyCell(b.StartMinor, v.Base)),
			}, append(rows, Tr(css.Class("nws-total"), Attr("data-testid", "nws-leg-row"), Attr("data-leg", "end"),
				Td(uistate.T("nws.legEnd")),
				Td(css.Class("nws-num"), Attr("data-testid", "nws-detail-bridge-end"),
					fmtMoney(money.New(b.EndMinor, v.Base)))))...)...),
		),
	)

	movers := v.Movers()
	var moverBody ui.Node
	if len(movers) == 0 {
		moverBody = P(css.Class("empty"), uistate.T("nws.moversEmpty"))
	} else {
		mrows := make([]any, 0, len(movers))
		for _, m := range movers {
			mrows = append(mrows, Tr(Attr("data-testid", "nws-mover-row"),
				Td(m.Acct.Name),
				Td(selectorTypeLabel(m.Acct.Type)),
				nwsSignedCell(m.MoveMinor, v.Base),
			))
		}
		moverBody = Div(css.Class("nws-scroll"),
			Table(css.Class("nws-table"), Attr("data-testid", "nws-movers-table"),
				Thead(Tr(Th(uistate.T("nws.colAccount")), Th(uistate.T("nws.colKind")),
					Th(css.Class("nws-num"), uistate.T("nws.colMoved")))),
				Tbody(mrows...),
			),
		)
	}

	body := Fragment(
		nwsBridge(v),
		legTable,
		H3(css.Class("nws-sec-title"), Style(map[string]string{"margin": "1rem 0 0.4rem"}), uistate.T("nws.moversTitle")),
		P(css.Class("nws-sec-note"), uistate.T("nws.moversNote", len(movers))),
		moverBody,
	)
	return nwsDetailSection("nws-01", "01", uistate.T("nws.secChanged"),
		uistate.T("nws.secChangedNote", nwsWindowLabel(v.Months)), nil, body)
}

// nwsSideSection renders §02 / §03: a side's composition followed by EVERY
// account on that side. Shares are normalized WITHIN the side, so the largest
// holding sets the scale for its own side only.
func nwsSideSection(v nwsView, asset bool) ui.Node {
	p := v.Latest()
	var (
		order  []balancesheet.Bucket
		total  int64
		rows   []nwsAcctRow
		id     = "nws-03"
		num    = "03"
		titleK = "nws.secOwe"
		noteK  = "nws.secOweNote"
	)
	if asset {
		order, total, rows = balancesheet.AssetBuckets, p.AssetsMinor, v.Assets()
		id, num, titleK, noteK = "nws-02", "02", "nws.secOwn", "nws.secOwnNote"
	} else {
		order, total, rows = balancesheet.LiabilityBuckets, p.LiabilitiesMinor, v.Liabilities()
	}
	amounts := p.Liabilities
	if asset {
		amounts = p.Assets
	}

	compRows := []any{}
	for _, bkt := range order {
		amt := amounts[bkt]
		if amt == 0 {
			continue
		}
		share := int64(0)
		if total > 0 {
			share = amt * 100 / total
		}
		compRows = append(compRows, Tr(Attr("data-testid", "nws-comp-row"),
			Td(nwsBucketLabel(bkt)),
			Td(nwsShareBar(share, asset)),
			Td(css.Class("nws-num"), fmt.Sprintf("%d%%", share)),
			nwsMoneyCell(amt, v.Base),
		))
	}

	acctRows := []any{}
	for _, r := range rows {
		share := int64(0)
		if total > 0 {
			share = r.SideMinor * 100 / total
		}
		acctRows = append(acctRows, Tr(Attr("data-testid", "nw-acct-row"),
			Td(r.Acct.Name),
			Td(selectorTypeLabel(r.Acct.Type)),
			Td(nwsShareBar(share, asset)),
			nwsMoneyCell(r.SideMinor, v.Base),
			nwsSignedCell(r.MoveMinor, v.Base),
		))
	}

	var action ui.Node
	if asset {
		action = A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/accounts")),
			Attr("data-testid", "networth-drill"), uistate.T("reports.viewAccounts"))
	} else {
		action = A(css.Class("btn", "btn-sm"), Href(uistate.RoutePath("/debt")),
			Attr("data-testid", "nw-owe-drill"), uistate.T("nw.viewDebts"))
	}

	var body ui.Node
	if len(rows) == 0 {
		empty := P(css.Class("empty"), uistate.T("nw.ownEmpty"))
		if !asset {
			empty = P(css.Class("empty"), Attr("data-testid", "nw-debt-free"), uistate.T("nw.debtFree"))
		}
		body = empty
	} else {
		body = Fragment(
			Div(css.Class("nws-scroll"),
				Table(css.Class("nws-table"),
					Thead(Tr(Th(uistate.T("nws.colGroup")), Th(""), Th(css.Class("nws-num"), uistate.T("nws.colShare")),
						Th(css.Class("nws-num"), uistate.T("nws.colAmount")))),
					Tbody(compRows...),
				),
			),
			// Every account on this side, listed. No "+2 more".
			P(css.Class("nws-sec-note"), Style(map[string]string{"margin": "1rem 0 0.4rem"}),
				uistate.T("nws.sideAccounts", len(rows))),
			Div(css.Class("nws-scroll"),
				Table(css.Class("nws-table"),
					Thead(Tr(Th(uistate.T("nws.colAccount")), Th(uistate.T("nws.colKind")), Th(""),
						Th(css.Class("nws-num"), uistate.T("nws.colAmount")),
						Th(css.Class("nws-num"), uistate.T("nws.colMoved")))),
					Tbody(acctRows...),
				),
			),
		)
	}
	return nwsDetailSection(id, num, uistate.T(titleK),
		uistate.T(noteK, fmtMoney(money.New(total, v.Base))), action, body)
}

// nwsShareBar draws a share within its own side.
func nwsShareBar(share int64, asset bool) ui.Node {
	cls := "nws-share-fill"
	if !asset {
		cls += " is-liability"
	}
	return Div(css.Class("nws-share"),
		Div(ClassStr(cls), Style(map[string]string{"width": fmt.Sprintf("%d%%", share)})))
}

// nwsHistorySection (04) is the window month by month: the mirrored chart
// again, then the figures behind it.
func nwsHistorySection(v nwsView) ui.Node {
	rows := make([]any, 0, len(v.Points))
	for i, p := range v.Points {
		label := ""
		if i < len(v.Labels) {
			label = v.Labels[i]
		}
		if label == "" {
			label = p.At.Format("Jan 2006")
		}
		rows = append(rows, Tr(Attr("data-testid", "nws-history-row"),
			Td(label),
			nwsMoneyCell(p.AssetsMinor, v.Base),
			nwsMoneyCell(p.LiabilitiesMinor, v.Base),
			nwsMoneyCell(p.NetMinor, v.Base),
		))
	}
	body := Fragment(
		nwsSides(v),
		Div(css.Class("nws-scroll"), Style(map[string]string{"margin-top": "0.9rem"}),
			Table(css.Class("nws-table"), Attr("data-testid", "nws-history-table"),
				Thead(Tr(Th(uistate.T("nws.colWhen")),
					Th(css.Class("nws-num"), uistate.T("accounts.assets")),
					Th(css.Class("nws-num"), uistate.T("dashboard.liabilities")),
					Th(css.Class("nws-num"), uistate.T("dashboard.netWorth")))),
				Tbody(rows...),
			),
		),
	)
	return nwsDetailSection("nws-04", "04", uistate.T("nws.secHistory"),
		uistate.T("nws.secHistoryNote"), nil, body)
}

// nwsHealthSection (05) restates every ratio with its definition alongside its
// reading, so the interpretation on the Glance view can be checked rather than
// merely trusted.
func nwsHealthSection(v nwsView) ui.Node {
	defs := Div(css.Class("nws-scroll"),
		Table(css.Class("nws-table"), Attr("data-testid", "nws-ratio-table"),
			Thead(Tr(Th(uistate.T("nws.colRatio")), Th(uistate.T("nws.colMeans")),
				Th(css.Class("nws-num"), uistate.T("nws.colNow")))),
			Tbody(
				Tr(Td(uistate.T("nws.ratioLiquid")), Td(uistate.T("nws.ratioLiquidDef")),
					Td(css.Class("nws-num"), nwsPctText(v.Health.LiquidShare))),
				Tr(Td(uistate.T("nws.ratioRunway")), Td(uistate.T("nws.ratioRunwayDef")),
					Td(css.Class("nws-num"), nwsRunwayText(v.Health))),
				Tr(Td(uistate.T("nws.ratioDebt")), Td(uistate.T("nws.ratioDebtDef")),
					Td(css.Class("nws-num"), nwsPctText(v.Health.DebtToAsset))),
			),
		),
	)
	return nwsDetailSection("nws-05", "05", uistate.T("nws.secHealth"),
		uistate.T("nws.secHealthNote"), nil, Fragment(nwsRatioCards(v), defs))
}
