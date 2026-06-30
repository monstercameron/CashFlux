// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// AllPageSize is the page-size value the pager emits for "show all". Callers map
// it to their own "all" sentinel (e.g. txnfilter.PageSizeAll).
const AllPageSize = -1

// Column describes one DataTable header. A non-empty SortKey makes the column a
// click-to-sort header; Class is applied to the <th> (e.g. for right-alignment);
// Head, when set, replaces the plain label (e.g. a select-all checkbox).
type Column struct {
	Label   string
	SortKey string
	Class   string
	Head    ui.Node
}

// DataTableProps configures a reusable sortable, paginated data table. The caller
// renders the body rows (so each screen keeps its own per-row cells/controls);
// this component owns the chrome: the semantic table, the sortable column headers
// (with aria-sort + caret), and the pagination footer.
type DataTableProps struct {
	Class   string   // extra class on the <table> (e.g. "txn-table")
	Columns []Column // header columns, in order
	Body    any      // the <tr> rows for the current page (e.g. a MapKeyed result)

	// StickyHead pins the header row to the top of the scroll container while the
	// body scrolls beneath it, so the column headers stay visible on a long table.
	// It adds the "dt-sticky" class, which the stylesheet turns into a sticky,
	// opaque thead (position:sticky; top:0). Opt-in per table.
	StickyHead bool

	// Virtual, when non-nil, renders the body as a windowed (virtualized) list: only
	// the rows near the viewport are materialized, with spacer rows preserving the full
	// scroll height, so a list of thousands of rows stays smooth. Body is ignored in
	// this mode — rows come from Virtual.RowAt. See datatable_virtual.go.
	Virtual *VirtualSpec

	// Sort state. Sort is the active SortKey; Dir is "asc" or "desc".
	Sort, Dir string
	OnSort    func(sortKey string)

	// SortSpinner makes the table manage a sort-in-progress indicator itself: clicking a
	// sortable header shows a spinner on that column (and marks the table aria-busy)
	// while the re-sort runs, then clears when the new Sort/Dir arrive. The re-sort
	// (the OnSort call) is deferred one macrotask so the spinner paints first — useful
	// when sorting a large list is momentarily expensive. Opt-in; the caller passes a
	// plain OnSort and the table owns the indicator. No per-caller state machine needed.
	SortSpinner bool

	// Pagination (optional — omit OnPage to render no footer). Page is 1-based;
	// Total is the full (unpaged) row count; PageSize<=0 means "all".
	Page, Total, PageSize int
	PageSizes             []int
	OnPage                func(page int)
	OnPageSize            func(size int)

	// TopPager also renders the pager (page nav + rows-per-page) ABOVE the table, not
	// only below it — so on a long list the rows-per-page control is reachable without
	// scrolling all the way to the bottom. No effect when OnPage is nil.
	TopPager bool
}

// DataTable renders the table chrome around the caller-rendered Body rows. It is a
// thin wrapper over the dataTable component so the table's own hooks (the managed
// sort spinner) stay isolated from the caller's hook order — and so the table can be
// rendered conditionally (e.g. only when there are rows) like any component.
func DataTable(props DataTableProps) ui.Node {
	return ui.CreateElement(dataTable, props)
}

// dataTable is the table component: the header row, the body (a windowed virtual list
// or the caller's rows), an optional top + bottom pager, and the managed sort spinner.
func dataTable(props DataTableProps) ui.Node {
	// Managed sort spinner (SortSpinner): clicking a sortable header shows a spinner on
	// that column while the re-sort runs, then clears when the new Sort/Dir props arrive.
	// The OnSort call is deferred one macrotask so the spinner paints before the (maybe
	// heavy) re-sort. The hooks run unconditionally so the hook order is stable; they
	// stay inert unless SortSpinner is set. This is what makes the spinner a standard
	// table config rather than a per-screen state machine.
	sorting := ui.UseState("")
	ui.UseEffect(func() func() {
		if sorting.Get() != "" {
			sorting.Set("")
		}
		return nil
	}, props.Sort+"|"+props.Dir)
	sortingKey := ""
	onSort := props.OnSort
	if props.SortSpinner && props.OnSort != nil {
		sortingKey = sorting.Get()
		raw := props.OnSort
		onSort = func(key string) {
			if sorting.Get() != "" {
				return // a sort is already in flight; ignore re-entrant clicks
			}
			sorting.Set(key)
			deferMacrotask(func() { raw(key) })
		}
	}

	headers := make([]any, 0, len(props.Columns))
	for _, c := range props.Columns {
		headers = append(headers, ui.CreateElement(dtHeader, dtHeaderProps{
			Col: c, Sort: props.Sort, Dir: props.Dir, OnSort: onSort,
			Sorting: sortingKey != "" && sortingKey == c.SortKey,
		}))
	}
	cls := "data-table"
	if props.Class != "" {
		cls += " " + props.Class
	}
	if props.StickyHead {
		cls += " dt-sticky"
	}

	// The body is either a windowed virtual list (huge datasets) or the caller's
	// pre-rendered rows.
	var body ui.Node
	if props.Virtual != nil {
		vs := *props.Virtual
		if vs.ColSpan == 0 {
			vs.ColSpan = len(props.Columns)
		}
		body = ui.CreateElement(dtVirtualBody, vs)
	} else {
		body = Tbody(props.Body)
	}

	tableArgs := []any{ClassStr(cls), Thead(Tr(headers...)), body}
	if sortingKey != "" {
		tableArgs = append(tableArgs, Attr("aria-busy", "true"))
	}
	table := Table(tableArgs...)
	if props.OnPage == nil {
		return table
	}
	pager := func(top bool) ui.Node {
		return ui.CreateElement(dtPager, dtPagerProps{
			Page: props.Page, Total: props.Total, PageSize: props.PageSize,
			PageSizes: props.PageSizes, OnPage: props.OnPage, OnPageSize: props.OnPageSize,
			Top: top,
		})
	}
	if props.TopPager {
		return Div(pager(true), table, pager(false))
	}
	return Div(table, pager(false))
}

type dtHeaderProps struct {
	Col       Column
	Sort, Dir string
	OnSort    func(string)
	Sorting   bool // this column is being re-sorted — show a spinner, not the caret
}

// dtHeader renders one column header — a plain <th> or a sortable header button.
func dtHeader(props dtHeaderProps) ui.Node {
	c := props.Col
	args := []any{Attr("scope", "col")}
	if c.Class != "" {
		args = append(args, ClassStr(c.Class))
	}
	if c.SortKey == "" {
		if c.Head != nil {
			args = append(args, c.Head)
		} else {
			args = append(args, c.Label)
		}
		return Th(args...)
	}
	ariaSort := "none"
	var caretIcon ui.Node
	if props.Sort == c.SortKey {
		if props.Dir == "asc" {
			ariaSort = "ascending"
			caretIcon = Icon(icon.ArrowUp, css.Class(tw.W4, tw.H4, tw.ShrinkO))
		} else {
			ariaSort = "descending"
			caretIcon = Icon(icon.ArrowDown, css.Class(tw.W4, tw.H4, tw.ShrinkO))
		}
	}
	// While this column's sort is in flight, swap the caret for a spinner so the user
	// sees the sort is working (the sticky header keeps it in view on a long list).
	if props.Sorting {
		caretIcon = Span(css.Class("dt-spin dt-spin-sm"), Attr("aria-hidden", "true"))
	}
	key := c.SortKey
	on := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnSort(key) })
	args = append(args, Attr("aria-sort", ariaSort),
		Button(css.Class("th-sort"), Type("button"), OnClick(on), c.Label, caretIcon))
	return Th(args...)
}

type dtPagerProps struct {
	Page, Total, PageSize int
	PageSizes             []int
	OnPage                func(int)
	OnPageSize            func(int)
	Top                   bool // rendered above the table (adds the data-pager-top class)
}

type pagerSizeBtnProps struct {
	Size   int // the page-size value (AllPageSize for "All")
	Label  string
	Active bool
	OnPick func(int)
}

// pagerSizeBtn is one rows-per-page choice, rendered as a button so its click
// handler carries the exact value directly. This replaced a controlled <select>
// whose change handler couldn't reliably read the chosen value (the framework's
// GetValue lagged a render), so picking "All" silently kept the old size (L78-T2).
// Its own component so the click hook stays at a stable call-site (On*-in-loop rule).
func pagerSizeBtn(props pagerSizeBtnProps) ui.Node {
	onClick := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnPick(props.Size) })
	cls := "btn pager-size"
	if props.Active {
		cls += " active"
	}
	args := []any{css.Class(cls), Type("button"), OnClick(onClick)}
	if props.Active {
		args = append(args, Attr("aria-pressed", "true"))
	} else {
		args = append(args, Attr("aria-pressed", "false"))
	}
	args = append(args, props.Label)
	return Button(args...)
}

// dtPager renders the prev/next + "from-to of total" + rows-per-page footer.
func dtPager(props dtPagerProps) ui.Node {
	onPrev := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnPage(props.Page - 1) })
	onNext := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnPage(props.Page + 1) })

	from, to := pagination.Window(props.Page, props.Total, props.PageSize)
	totalPages := pagination.TotalPages(props.Total, props.PageSize)

	sizeBtns := make([]any, 0, len(props.PageSizes)+1)
	for _, s := range props.PageSizes {
		sizeBtns = append(sizeBtns, ui.CreateElement(pagerSizeBtn, pagerSizeBtnProps{
			Size: s, Label: strconv.Itoa(s), Active: props.PageSize == s, OnPick: props.OnPageSize,
		}))
	}
	sizeBtns = append(sizeBtns, ui.CreateElement(pagerSizeBtn, pagerSizeBtnProps{
		Size: AllPageSize, Label: "All", Active: props.PageSize < 0, OnPick: props.OnPageSize,
	}))

	prevArgs := []any{css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("ui.table.prevPage")), OnClick(onPrev)}
	if props.Page <= 1 {
		prevArgs = append(prevArgs, Attr("disabled", "disabled"))
	}
	prevArgs = append(prevArgs, "Prev")
	nextArgs := []any{css.Class("btn"), Type("button"), Attr("aria-label", uistate.T("ui.table.nextPage")), OnClick(onNext)}
	if props.Page >= totalPages {
		nextArgs = append(nextArgs, Attr("disabled", "disabled"))
	}
	nextArgs = append(nextArgs, "Next")

	pos := strconv.Itoa(from) + "–" + strconv.Itoa(to) + " of " + strconv.Itoa(props.Total)
	groupArgs := []any{css.Class("pager-sizes"), Attr("role", "group"), Attr("aria-label", uistate.T("ui.table.rowsPerPage"))}
	groupArgs = append(groupArgs, sizeBtns...)
	pagerCls := "data-pager"
	if props.Top {
		pagerCls += " data-pager-top"
	}
	return Div(css.Class(pagerCls),
		Button(prevArgs...),
		Span(css.Class("muted data-pos"), pos),
		Button(nextArgs...),
		Span(css.Class("muted data-pager-label"), "Rows per page"),
		Div(groupArgs...),
	)
}
