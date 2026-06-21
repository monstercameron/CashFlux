//go:build js && wasm

package ui

import (
	"strconv"

	"github.com/monstercameron/CashFlux/internal/pagination"
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

	// Sort state. Sort is the active SortKey; Dir is "asc" or "desc".
	Sort, Dir string
	OnSort    func(sortKey string)

	// Pagination (optional — omit OnPage to render no footer). Page is 1-based;
	// Total is the full (unpaged) row count; PageSize<=0 means "all".
	Page, Total, PageSize int
	PageSizes             []int
	OnPage                func(page int)
	OnPageSize            func(size int)
}

// DataTable renders the table chrome around the caller-rendered Body rows.
func DataTable(props DataTableProps) ui.Node {
	headers := make([]any, 0, len(props.Columns))
	for _, c := range props.Columns {
		headers = append(headers, ui.CreateElement(dtHeader, dtHeaderProps{Col: c, Sort: props.Sort, Dir: props.Dir, OnSort: props.OnSort}))
	}
	cls := "data-table"
	if props.Class != "" {
		cls += " " + props.Class
	}
	table := Table(ClassStr(cls), Thead(Tr(headers...)), Tbody(props.Body))
	if props.OnPage == nil {
		return table
	}
	return Div(table, ui.CreateElement(dtPager, dtPagerProps{
		Page: props.Page, Total: props.Total, PageSize: props.PageSize,
		PageSizes: props.PageSizes, OnPage: props.OnPage, OnPageSize: props.OnPageSize,
	}))
}

type dtHeaderProps struct {
	Col       Column
	Sort, Dir string
	OnSort    func(string)
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
	ariaSort, caret := "none", ""
	if props.Sort == c.SortKey {
		if props.Dir == "asc" {
			ariaSort, caret = "ascending", " ▲"
		} else {
			ariaSort, caret = "descending", " ▼"
		}
	}
	key := c.SortKey
	on := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnSort(key) })
	args = append(args, Attr("aria-sort", ariaSort),
		Button(css.Class("th-sort"), Type("button"), OnClick(on), c.Label+caret))
	return Th(args...)
}

type dtPagerProps struct {
	Page, Total, PageSize int
	PageSizes             []int
	OnPage                func(int)
	OnPageSize            func(int)
}

// dtPager renders the prev/next + "from-to of total" + rows-per-page footer.
func dtPager(props dtPagerProps) ui.Node {
	onPrev := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnPage(props.Page - 1) })
	onNext := ui.UseEvent(func(e ui.Event) { e.PreventDefault(); props.OnPage(props.Page + 1) })
	onSize := ui.UseEvent(func(e ui.Event) {
		if n, err := strconv.Atoi(e.GetValue()); err == nil {
			props.OnPageSize(n)
		}
	})

	from, to := pagination.Window(props.Page, props.Total, props.PageSize)
	totalPages := pagination.TotalPages(props.Total, props.PageSize)

	sizeOpts := make([]ui.Node, 0, len(props.PageSizes)+1)
	for _, s := range props.PageSizes {
		sizeOpts = append(sizeOpts, Option(Value(strconv.Itoa(s)), SelectedIf(props.PageSize == s), strconv.Itoa(s)))
	}
	sizeOpts = append(sizeOpts, Option(Value(strconv.Itoa(AllPageSize)), SelectedIf(props.PageSize < 0), "All"))

	prevArgs := []any{css.Class("btn"), Type("button"), Attr("aria-label", "Previous page"), OnClick(onPrev)}
	if props.Page <= 1 {
		prevArgs = append(prevArgs, Attr("disabled", "disabled"))
	}
	prevArgs = append(prevArgs, "Prev")
	nextArgs := []any{css.Class("btn"), Type("button"), Attr("aria-label", "Next page"), OnClick(onNext)}
	if props.Page >= totalPages {
		nextArgs = append(nextArgs, Attr("disabled", "disabled"))
	}
	nextArgs = append(nextArgs, "Next")

	pos := strconv.Itoa(from) + "–" + strconv.Itoa(to) + " of " + strconv.Itoa(props.Total)
	return Div(css.Class("data-pager"),
		Button(prevArgs...),
		Span(css.Class("muted data-pos"), pos),
		Button(nextArgs...),
		Span(css.Class("muted data-pager-label"), "Rows per page"),
		Select(css.Class("field"), Attr("aria-label", "Rows per page"), OnChange(onSize), sizeOpts),
	)
}
