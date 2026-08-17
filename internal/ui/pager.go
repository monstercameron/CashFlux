// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/pagination"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	sh "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// PagerProps configures the shared, standardized list pager (mirrors top + bottom):
// a "from–to of total" range, a rows-per-page control, prev/next, and a jump-to-page box.
type PagerProps struct {
	// Page is the 1-based current page. Total is the full (unpaged) item count. PageSize<=0
	// (or AllPageSize) means "all on one page".
	Page, Total, PageSize int
	// PageSizes are the rows-per-page choices; an "All" button is appended automatically.
	PageSizes []int
	// OnPage jumps to a page (the component clamps to [1, totalPages]). OnPageSize changes
	// the rows-per-page. Both required.
	OnPage     func(int)
	OnPageSize func(int)
	// Top renders the pager above the list (adds .std-pager-top); AnchorID (optional) is an
	// element the nav scrolls back into view after a page change. IDPrefix scopes the
	// data-testids (e.g. "txn", "todo"); defaults to "pager".
	Top      bool
	AnchorID string
	IDPrefix string
}

// Pager is the app-standard paginator: one control language across every paged list. Render
// it both above and below a list (Top:true / Top:false) for mirrored controls. It shows the
// range, rows-per-page buttons, prev/next, and an editable "Page N of M" jump box.
func Pager(props PagerProps) uic.Node { return uic.CreateElement(pager, props) }

func pager(props PagerProps) uic.Node {
	prefix := props.IDPrefix
	if prefix == "" {
		prefix = "pager"
	}
	totalPages := pagination.TotalPages(props.Total, props.PageSize)
	from, to := pagination.Window(props.Page, props.Total, props.PageSize)

	onPrev := uic.UseEvent(func(e uic.Event) {
		e.PreventDefault()
		props.OnPage(props.Page - 1)
		scrollAnchorIntoView(props.AnchorID)
	})
	onNext := uic.UseEvent(func(e uic.Event) {
		e.PreventDefault()
		props.OnPage(props.Page + 1)
		scrollAnchorIntoView(props.AnchorID)
	})
	// Jump: keep the typed text in LOCAL state and commit it on Enter or blur.
	//
	// C627: this used to be a controlled input reading e.GetValue() from a single
	// OnChange. That is the same one-render-late read the rows-per-page control
	// already avoids by using buttons ("a controlled select mis-reads its value one
	// render late in this framework") — so the handler saw the OLD page, called
	// OnPage with it, and nothing moved, while the DOM kept the number the user had
	// typed. The control then reported a page that was not the rendered page, which
	// is worse than not navigating: the ledger looked like page 132.
	//
	// Local state fed by OnInput is read back reliably, so the commit acts on what
	// was actually typed. Committing on Enter/blur (not per keystroke) keeps "12"
	// from paging to 1 on its way to 12.
	jump := uic.UseState("")
	onJumpInput := uic.UseEvent(func(v string) { jump.Set(v) })
	// commitJump validates, clamps and navigates; an unparseable entry resets the
	// box to the page actually being shown rather than leaving a number that never
	// took effect sitting in the field.
	commitJump := func() {
		raw := strings.TrimSpace(jump.Get())
		if raw == "" {
			return
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			jump.Set("")
			return
		}
		if n < 1 {
			n = 1
		}
		if n > totalPages {
			n = totalPages
		}
		jump.Set("") // fall back to showing props.Page, which is now the real page
		if n == props.Page {
			return
		}
		props.OnPage(n)
		scrollAnchorIntoView(props.AnchorID)
	}
	onJumpCommit := uic.UseEvent(func(uic.Event) { commitJump() })
	onJumpKey := uic.UseEvent(func(e uic.KeyboardEvent) {
		if e.GetKey() == "Enter" {
			e.PreventDefault()
			commitJump()
		}
	})
	jumpValue := jump.Get()
	if jumpValue == "" {
		jumpValue = strconv.Itoa(props.Page)
	}

	// Rows-per-page choices (buttons — a controlled select mis-reads its value one render
	// late in this framework, per the DataTable note).
	sizeBtns := make([]any, 0, len(props.PageSizes)+2)
	sizeBtns = append(sizeBtns, sh.Span(css.Class("std-pager-size-label"), uistate.T("ui.table.rowsPerPage")))
	for _, s := range props.PageSizes {
		sizeBtns = append(sizeBtns, uic.CreateElement(pagerSizeBtn, pagerSizeBtnProps{
			Size: s, Label: strconv.Itoa(s), Active: props.PageSize == s, OnPick: props.OnPageSize,
		}))
	}
	sizeBtns = append(sizeBtns, uic.CreateElement(pagerSizeBtn, pagerSizeBtnProps{
		Size: AllPageSize, Label: uistate.T("ui.table.all"), Active: props.PageSize <= 0, OnPick: props.OnPageSize,
	}))

	prevArgs := []any{css.Class("std-page-btn"), sh.Type("button"), sh.Attr("data-testid", prefix+"-prev"),
		sh.Attr("aria-label", uistate.T("ui.table.prevPage")), sh.OnClick(onPrev),
		Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4)), sh.Span(uistate.T("ui.table.prev"))}
	if props.Page <= 1 {
		prevArgs = append(prevArgs, sh.Attr("disabled", "disabled"))
	}
	nextArgs := []any{css.Class("std-page-btn"), sh.Type("button"), sh.Attr("data-testid", prefix+"-next"),
		sh.Attr("aria-label", uistate.T("ui.table.nextPage")), sh.OnClick(onNext),
		sh.Span(uistate.T("ui.table.next")), Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))}
	if props.Page >= totalPages {
		nextArgs = append(nextArgs, sh.Attr("disabled", "disabled"))
	}

	cls := "std-pager"
	if props.Top {
		cls += " std-pager-top"
	}
	group := append([]any{css.Class("std-pager-sizes"), sh.Attr("role", "group"), sh.Attr("aria-label", uistate.T("ui.table.rowsPerPage"))}, sizeBtns...)

	return sh.Div(css.Class(cls),
		sh.Div(css.Class("std-pager-info"),
			sh.Span(css.Class("std-pager-range"), sh.Attr("data-testid", prefix+"-range"),
				strconv.Itoa(from)+"–"+strconv.Itoa(to)+" "+uistate.T("ui.table.of")+" "+strconv.Itoa(props.Total)),
			sh.Div(group...),
		),
		sh.Div(css.Class("std-pager-nav"),
			sh.Button(prevArgs...),
			sh.Label(css.Class("std-pager-jump"),
				sh.Span(uistate.T("ui.table.pageWord")),
				// The field shows the typed draft while one is in flight, and the REAL
				// page the rest of the time — so it can never sit there displaying a
				// page the ledger is not on (C627).
				sh.Input(css.Class("std-pager-jump-input"), sh.Type("number"), sh.Attr("data-testid", prefix+"-jump"),
					sh.Attr("aria-label", uistate.T("ui.table.jumpAria")), sh.Attr("min", "1"), sh.Attr("max", strconv.Itoa(totalPages)),
					FieldValue(jumpValue), sh.OnInput(onJumpInput), sh.OnChange(onJumpCommit),
					sh.OnBlur(onJumpCommit), sh.OnKeyDown(onJumpKey)),
				sh.Span(css.Class("std-pager-jump-total"), uistate.T("ui.table.of")+" "+strconv.Itoa(totalPages)),
			),
			sh.Button(nextArgs...),
		),
	)
}
