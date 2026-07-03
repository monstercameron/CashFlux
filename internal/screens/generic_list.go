// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/widgetcatalog"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/ui"
)

// genericListProps configures the generic list body for a Studio-designed list widget.
type genericListProps struct {
	Spec  domain.WidgetSpec
	Frame domain.Frame
	Base  string
}

// genericListWidget renders a Studio-designed list with the display behaviour the
// author chose (stored on the spec, pure hydration): "cap" shows the top N rows;
// "scroll" shows everything in a scroll area inside the tile; "page" pages through N
// at a time with a pager. When the author enabled it, a "view all" link navigates to
// the source's full-data screen. Owns its page-index state.
func genericListWidget(p genericListProps) ui.Node {
	page := ui.UseState(0)
	// Reset to the first page whenever the underlying source changes, so a stale page
	// index never carries across collections.
	srcKey := ""
	if p.Spec.Pipeline != nil {
		srcKey = p.Spec.Pipeline.Source.Collection
	}
	ui.UseEffect(func() func() { page.Set(0); return nil }, srcKey)
	fr := p.Frame
	display := p.Spec.Settings["display"]
	if display == "" {
		display = "cap"
	}
	count := atoiOr(p.Spec.Settings["count"], 6)
	if count <= 0 {
		count = 6
	}
	viewAll := p.Spec.Settings["viewall"] == "true"

	if fr.Rows == 0 {
		return Div(css.Class(tw.Flex, tw.FlexCol, tw.MinH0),
			P(css.Class("empty t-body", tw.TextDim), uistate.T("dashboard.noDataYet")))
	}

	labelCol, hasLabel := frameLabelCol(fr)
	valCol, _ := frameValueCol(fr)

	start, end := 0, fr.Rows
	hasPager := false
	var pager ui.Node = Fragment()
	switch display {
	case "page":
		total := (fr.Rows + count - 1) / count
		if total < 1 {
			total = 1
		}
		cur := page.Get()
		if cur > total-1 {
			cur = total - 1
		}
		if cur < 0 {
			cur = 0
		}
		start = cur * count
		end = min(start+count, fr.Rows)
		pager = ui.CreateElement(listPager, listPagerProps{Page: cur, Total: total, OnPage: page.Set})
		hasPager = true
	case "scroll":
		// show all rows; the container scrolls
	default: // cap
		if count < fr.Rows {
			end = count
		}
	}

	rowNodes := make([]ui.Node, 0, end-start)
	for i := start; i < end; i++ {
		label := fmt.Sprintf("%d", i+1)
		if hasLabel {
			label = labelCol.Str(i)
		}
		rowNodes = append(rowNodes, Div(css.Class("t-body", tw.Flex, tw.JustifyBetween, tw.Py25, tw.BorderB, tw.BorderLine70),
			Span(css.Class(tw.TextDim), label),
			Span(css.Class("fig", tw.FontDisplay), dataViewValue(valCol, i, p.Base)),
		))
	}

	// Rows take the available height and clip/scroll; the pager + link live in a
	// fixed footer that is ALWAYS visible (never pushed out by overflowing rows).
	listBody := Div(css.Class("studio-list-body"), rowNodes)

	hasLink := false
	var link ui.Node = Fragment()
	if viewAll && p.Spec.Pipeline != nil {
		if route, lbl := widgetcatalog.CollectionRoute(p.Spec.Pipeline.Source.Collection); route != "" {
			link = ui.CreateElement(listViewAllLink, listViewAllLinkProps{Route: route, Label: lbl})
			hasLink = true
		}
	}

	var footer ui.Node = Fragment()
	if hasPager || hasLink {
		footer = Div(css.Class("studio-list-footer"), pager, link)
	}
	return Div(css.Class("studio-list-root"), listBody, footer)
}

// widgetDisplay returns a list tile's overflow behavior ("cap" | "scroll" |
// "page") from its settings, falling back to def for an unset/unknown value.
func widgetDisplay(spec domain.WidgetSpec, def string) string {
	switch spec.Settings["display"] {
	case "cap", "scroll", "page":
		return spec.Settings["display"]
	}
	return def
}

// pagedListProps configures pagedList.
type pagedListProps struct {
	Rows     []ui.Node // pre-built row nodes (whole list; pagedList shows one page)
	PageSize int
	AsTable  bool // wrap each page's rows in <table><tbody> (table rows) vs a plain block
}

// pagedList shows a fixed-size page of pre-built row nodes with a prev/next pager
// footer, owning its page-index state. The dashboard list tiles whose display
// setting is "page" render their rows through it, so a long list (e.g. recent
// transactions) is browsable in-tile without scrolling or clipping.
func pagedList(p pagedListProps) ui.Node {
	page := ui.UseState(0)
	size := p.PageSize
	if size < 1 {
		size = 6
	}
	total := (len(p.Rows) + size - 1) / size
	if total < 1 {
		total = 1
	}
	cur := page.Get()
	if cur > total-1 {
		cur = total - 1
	}
	if cur < 0 {
		cur = 0
	}
	start := cur * size
	end := min(start+size, len(p.Rows))
	pageRows := p.Rows[start:end]
	var content ui.Node
	if p.AsTable {
		content = Table(css.Class("t-body", tw.WFull), Tbody(pageRows))
	} else {
		content = Div(pageRows)
	}
	return Div(css.Class("dash-paged"),
		Div(css.Class("dash-paged-body"), content),
		ui.CreateElement(listPager, listPagerProps{Page: cur, Total: total, OnPage: page.Set}),
	)
}

type listPagerProps struct {
	Page, Total int
	OnPage      func(int)
}

// listPager renders the prev/next pager for a paged list (own hooks), with the
// boundary buttons visibly disabled at the first/last page.
func listPager(p listPagerProps) ui.Node {
	prev := ui.UseEvent(func() {
		if p.Page > 0 {
			p.OnPage(p.Page - 1)
		}
	})
	next := ui.UseEvent(func() {
		if p.Page < p.Total-1 {
			p.OnPage(p.Page + 1)
		}
	})
	atStart, atEnd := p.Page <= 0, p.Page >= p.Total-1
	prevCls, nextCls := "btn-icon", "btn-icon"
	if atStart {
		prevCls += " is-disabled"
	}
	if atEnd {
		nextCls += " is-disabled"
	}
	return Div(css.Class("studio-list-pager"),
		Button(css.Class(prevCls), Type("button"), Attr("aria-label", uistate.T("pager.previous")), Attr("aria-disabled", boolStr(atStart)), OnClick(prev), uiw.Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4))),
		Span(css.Class("t-caption", tw.TextDim), fmt.Sprintf("%d of %d", p.Page+1, p.Total)),
		Button(css.Class(nextCls), Type("button"), Attr("aria-label", uistate.T("pager.next")), Attr("aria-disabled", boolStr(atEnd)), OnClick(next), uiw.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))),
	)
}

type listViewAllLinkProps struct {
	Route, Label string
}

// listViewAllLink navigates to the source's full-data screen (own nav hook).
func listViewAllLink(p listViewAllLinkProps) ui.Node {
	nav := router.UseNavigate()
	open := ui.UseEvent(func() { nav.Navigate(uistate.RoutePath(p.Route)) })
	return Button(css.Class("btn-link studio-list-link"), Type("button"), OnClick(open), p.Label+" →")
}
