// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestCSVPreflightReportsFailedRowsAndCountsOnlyWritableRowsTC3B7(t *testing.T) {
	info := &csvPreflightInfo{
		Total:    2,
		Batch:    staging.Stage(nil, nil, "checking", "USD"),
		Failures: []store.CSVRowError{{Line: 3, Reason: "invalid amount"}},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(csvPreflightCard(info, ui.Handler{}, ui.Handler{}))
	if !strings.Contains(f.Text(), "0 new transactions") || !strings.Contains(f.Text(), "1 row needs attention") || !strings.Contains(f.Text(), "line 3") {
		t.Fatalf("preflight hid failed rows or counted them as new: %q", f.Text())
	}
}
