// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTXU5UserColumnRequiresEveryoneAndMultipleMembers(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		memberScope string
		members     int
		want        bool
	}{
		{name: "everyone in household", enabled: true, members: 2, want: true},
		{name: "disabled in settings", enabled: false, members: 2},
		{name: "single member household", enabled: true, members: 1},
		{name: "member scoped", enabled: true, memberScope: "member-1", members: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnUserColumnVisible(tt.enabled, tt.memberScope, tt.members); got != tt.want {
				t.Fatalf("visible = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestTXU5PageRangeIsPlainText(t *testing.T) {
	tests := []struct {
		name, want        string
		page, size, total int
	}{
		{name: "first page", page: 1, size: 25, total: 3292, want: "1–25 of 3,292"},
		{name: "last page", page: 132, size: 25, total: 3292, want: "3,276–3,292 of 3,292"},
		{name: "all rows", page: 1, size: -1, total: 8, want: "1–8 of 8"},
		{name: "empty", page: 1, size: 25, total: 0, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnPageRange(tt.page, tt.size, tt.total); got != tt.want {
				t.Fatalf("range = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTXU5MemberInitialsKeepAccessibleNameSource(t *testing.T) {
	for name, want := range map[string]string{
		"  Marcus  Reed ": "MR",
		"Priya":           "P",
		"":                "—",
	} {
		if got := txnMemberInitials(name); got != want {
			t.Errorf("initials for %q = %q, want %q", name, got, want)
		}
	}
}

func TestTXU5TransactionRowColumnOrderAndMemberName(t *testing.T) {
	f := render.New(t)
	props := &txnFrameRowProps{
		ID: "row-1", Date: "Oct 18, 2026", Desc: "Corner store", Amount: "$12.00",
		Category: "Groceries", Account: "Checking", Member: "Marcus Reed",
		Vis:    uistate.TxnCols{Category: true, Account: true, User: true, Amount: true, Status: true},
		OnOpen: func(string) {}, OnToggleSelect: func(string, bool) {},
	}
	f.Render(ui.CreateElement(txnFrameRow, props))
	cells := f.AllByTag("td")
	var order []string
	for _, cell := range cells {
		order = append(order, cell.Attr("class"))
	}
	want := []string{"", "", "row-desc-cell", "td-cat", "td-acct", "td-user", "td-amount", "td-actions"}
	if len(order) != len(want) {
		t.Fatalf("cell classes = %#v, want %#v", order, want)
	}
	for i := range want {
		if !strings.HasPrefix(order[i], want[i]) {
			t.Fatalf("cell %d class = %q, want %q (all: %#v)", i, order[i], want[i], order)
		}
	}
	if got := cells[5].Attr("aria-label"); got != "Member: Marcus Reed" {
		t.Errorf("member accessible name = %q, want full name", got)
	}
	if !strings.Contains(f.Text(), "MR") {
		t.Errorf("row text %q does not show member initials", f.Text())
	}
}

func TestTXU5CountLineOmitsPageRange(t *testing.T) {
	f := render.New(t)
	f.Render(txnCountLine(txnCountLineProps{
		Shown: 25, Total: 3292, Range: txnPageRange(1, 25, 3292),
	}))
	if got := f.Text(); strings.Contains(got, "1–25 of 3,292") {
		t.Fatalf("count line = %q, want page range omitted per R2A B4", got)
	}
}
