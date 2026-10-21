// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/css"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestNetWorthUnavailableAccountWindowDoesNotRenderAsZero(t *testing.T) {
	previousCSS := css.SetSink(reportTestCSSSink{})
	defer css.SetSink(previousCSS)
	f := render.New(t, render.WithQueuedScheduler())
	defer f.Cleanup()
	f.Render(ui.CreateElement(func() ui.Node {
		return nwsWindowMoveCell(nwsAcctRow{WindowKnown: false}, "USD")
	}))
	f.Stabilize()
	text := f.Text()
	if !strings.Contains(text, "—") || strings.Contains(text, "$0.00") {
		t.Fatalf("unavailable account movement rendered as a numeric zero: %q", text)
	}
}
