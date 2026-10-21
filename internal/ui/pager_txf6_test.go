// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTXF6PageJumpAllowsInvalidDraftAndRestoresCurrentPage(t *testing.T) {
	page := 7
	f := render.New(t)
	f.Render(ui.CreateElement(pager, PagerProps{
		Page: page, Total: 200, PageSize: 25,
		PageSizes:  []int{25, 50, 100},
		OnPage:     func(next int) { page = next },
		OnPageSize: func(int) {},
		IDPrefix:   "txn",
	}))

	var box *render.QueryNode
	for _, input := range f.AllByTag("input") {
		if strings.Contains(input.Attr("class"), "std-pager-jump-input") {
			box = input
			break
		}
	}
	if box == nil {
		t.Fatal("page number input not found")
	}
	if got := box.Attr("type"); got != "text" {
		t.Fatalf("page number input type = %q, want text so invalid drafts remain visible until blur", got)
	}
	if got := box.Attr("inputmode"); got != "numeric" {
		t.Fatalf("page number input mode = %q, want numeric keyboard hint", got)
	}

	box.Input("abc")
	if got := box.Attr(fieldValueAttr); got != "abc" {
		t.Fatalf("page number draft = %q after typing invalid text, want abc", got)
	}
	box.Dispatch("onblur", render.Event{})
	if page != 7 {
		t.Fatalf("page after invalid draft blur = %d, want current page 7", page)
	}
	if got := box.Attr(fieldValueAttr); got != "7" {
		t.Fatalf("page number input after invalid draft blur = %q, want current page 7", got)
	}
}
