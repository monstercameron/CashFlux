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

func TestCSVPreflightShowsFailuresWhenNoRowsAreParseableTC4N6(t *testing.T) {
	info := &csvPreflightInfo{
		Total:    1,
		Batch:    staging.Stage(nil, nil, "usd", "USD"),
		Failures: []store.CSVRowError{{Line: 2, Reason: "amount is required"}},
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(csvPreflightCard(info, ui.Handler{}, ui.Handler{}))
	if !strings.Contains(f.Text(), "0 new transactions") || !strings.Contains(f.Text(), "1 row needs attention") || !strings.Contains(f.Text(), "amount is missing") {
		t.Fatalf("all-invalid file did not retain row failure details: %q", f.Text())
	}
}

func TestCSVSkipReasonIsLocalizedTC4N6(t *testing.T) {
	got := csvSkipDetail([]store.CSVRowError{{Line: 4, Reason: "amount is required"}})
	if !strings.Contains(got, "line 4") || !strings.Contains(got, "amount is missing") {
		t.Fatalf("unexpected CSV skip detail: %q", got)
	}
}
