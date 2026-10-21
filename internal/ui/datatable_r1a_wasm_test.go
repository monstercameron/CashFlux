// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestDataTableRendersTopPagerOnlyWhenRequested(t *testing.T) {
	for _, tt := range []struct {
		name    string
		top     bool
		wantTop bool
	}{
		{name: "default hidden", wantTop: false},
		{name: "opted in", top: true, wantTop: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := render.New(t)
			f.Render(ui.CreateElement(dataTable, DataTableProps{
				Columns: []Column{{Label: "Date"}}, Body: nil,
				Page: 1, Total: 51, PageSize: 25, PageSizes: []int{25, 50},
				OnPage: func(int) {}, OnPageSize: func(int) {}, TopPager: tt.top,
			}))
			found := false
			for _, div := range f.AllByTag("div") {
				if strings.Contains(div.Attr("class"), "std-pager-top") {
					found = true
					break
				}
			}
			if found != tt.wantTop {
				t.Fatalf("top pager rendered = %t, want %t", found, tt.wantTop)
			}
		})
	}
}
