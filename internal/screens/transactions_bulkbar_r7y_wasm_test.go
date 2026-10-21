// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"io"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestR7YBulkBarWidgetUsesTransparentOverlayAndFloatingBarClass(t *testing.T) {
	app, err := appstate.New(io.Discard, false)
	if err != nil {
		t.Fatalf("create app state: %v", err)
	}
	f := render.New(t)
	f.Render(ui.CreateElement(txnBulkBarWidget, txnBulkBarProps{App: app}))

	for _, node := range f.AllByTag("div") {
		if node.Attr("data-widget") != "txn-bulkbar" {
			continue
		}
		style := txnBulkBarWidgetStyle()
		for key, want := range map[string]string{"background": "transparent", "border": "0", "box-shadow": "none"} {
			if got := style[key]; got != want {
				t.Errorf("bulk bar widget overlay style %s = %q, want %q", key, got, want)
			}
		}
		for _, child := range f.AllByTag("div") {
			if strings.Contains(child.Attr("class"), "txn-bulkbar-overlay") {
				return
			}
		}
		t.Fatal("bulk bar has no floating txn-bulkbar-overlay surface")
	}
	t.Fatal(`rendered bulk bar has no element with data-widget="txn-bulkbar"`)
}
