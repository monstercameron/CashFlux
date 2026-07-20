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

// nwsIndex is the sticky chip index: where you are in the document, and the way
// back out of it. Items are buttons that scroll their section into view rather
// than hash anchors, which would push a fragment URL through the SPA router.
//
// Detail is a long document, and a reader deep inside it needs three things the
// first version did not provide: to land on a section's TITLE rather than
// mid-chart behind the fixed header (fixed with scroll-margin-top on the
// sections), to see which section they are currently in (the scroll-spy below
// marks it .is-current), and to get back to the top or to Glance without
// hunting for a control that has scrolled away.
func nwsIndex(onGlance ui.Handler) ui.Node {
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
			Span(css.Class("nws-idx-label"), label),
		)
	}
	// Written out rather than looped: On* prop options register hooks, which must
	// sit at stable render positions (CLAUDE.md, "CRITICAL gotchas").
	sec := nwsDetailSections
	return Nav(css.Class("nws-index"), Attr("data-testid", "nws-index"),
		Attr("aria-label", uistate.T("nws.indexAria")),
		item(sec[0].ID, sec[0].Num, sec[0].TitleKey),
		item(sec[1].ID, sec[1].Num, sec[1].TitleKey),
		item(sec[2].ID, sec[2].Num, sec[2].TitleKey),
		item(sec[3].ID, sec[3].Num, sec[3].TitleKey),
		item(sec[4].ID, sec[4].Num, sec[4].TitleKey),
		item(sec[5].ID, sec[5].Num, sec[5].TitleKey),
		Span(css.Class("nws-idx-sep"), Attr("aria-hidden", "true")),
		Button(css.Class("nws-idx", "nws-idx-back"), Type("button"),
			Attr("data-testid", "nws-idx-top"), Title(uistate.T("nws.backTop")),
			OnClick(scrollTo("nws-00")),
			Span(Attr("aria-hidden", "true"), "↑ "), uistate.T("nws.backTop")),
		Button(css.Class("nws-idx", "nws-idx-back"), Type("button"),
			Attr("data-testid", "nws-idx-glance"), Title(uistate.T("nws.backGlanceTitle")),
			OnClick(onGlance), uistate.T("nws.backGlance")),
	)
}

// nwsScrollSpy marks the section the reader is currently inside, so a long
// document never loses its place. It toggles the class directly on the DOM
// rather than through state, so scrolling never re-renders the document. Detail
// only; Glance has no index.
func nwsScrollSpy(active bool) {
	ui.UseEffect(func() func() {
		if !active {
			return nil
		}
		doc := js.Global().Get("document")
		ioCtor := js.Global().Get("IntersectionObserver")
		if !doc.Truthy() || !ioCtor.Truthy() {
			return nil
		}
		var secs []js.Value
		for _, s := range nwsDetailSections {
			if el := doc.Call("getElementById", s.ID); el.Truthy() {
				secs = append(secs, el)
			}
		}
		if len(secs) == 0 {
			return nil
		}
		apply := func() {
			// Current = the last section whose heading has reached the band just
			// under the sticky index: the greatest top still above it.
			const band = 170.0
			cur := secs[0].Get("id").String()
			best := -1e18
			for _, el := range secs {
				top := el.Call("getBoundingClientRect").Get("top").Float()
				if top <= band && top > best {
					best, cur = top, el.Get("id").String()
				}
			}
			items := doc.Call("querySelectorAll", ".nws-idx[data-section]")
			for i := 0; i < items.Get("length").Int(); i++ {
				it := items.Call("item", i)
				on := it.Call("getAttribute", "data-section").String() == cur
				it.Get("classList").Call("toggle", "is-current", on)
			}
		}
		cb := js.FuncOf(func(js.Value, []js.Value) any { apply(); return nil })
		io := ioCtor.New(cb, map[string]any{"rootMargin": "-150px 0px -55% 0px", "threshold": 0})
		for _, el := range secs {
			io.Call("observe", el)
		}
		apply()
		return func() {
			io.Call("disconnect")
			cb.Release()
		}
	}, "nws-spy|"+boolStr(active))
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
		uistate.T("nws.secChangedNote", nwsWindowLabel(v.Months)), nwsBridgeExplain(), body)
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
		uistate.T("nws.secHistoryNote"), nwsSidesExplain(), body)
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
