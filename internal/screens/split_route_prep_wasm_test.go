// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestSplitRouteStopsPreviewForInvalidAmount(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	t.Cleanup(func() { appstate.Default = previous; _ = app.Store().Close() })
	for _, member := range []domain.Member{{ID: "split-a", Name: "A"}, {ID: "split-b", Name: "B"}} {
		if err := app.PutMember(member); err != nil {
			t.Fatal(err)
		}
	}
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(Split))
	f.Stabilize()
	byTestID(f, "button", "split-select-all").Click()
	byTestID(f, "input", "split-amount").Input("not-money")
	f.Stabilize()
	if !strings.Contains(f.Text(), "Check the amount and weights before saving.") {
		t.Fatalf("invalid input has no visible explanation: %s", f.Text())
	}
	if strings.Contains(f.Text(), "split among") {
		t.Fatalf("invalid input still shows a share summary: %s", f.Text())
	}
	byTestID(f, "input", "split-amount").Input("10.00")
	f.Stabilize()
	if !strings.Contains(f.Text(), "$5.00 each") && !strings.Contains(f.Text(), "5.00 each") {
		t.Fatalf("valid amount did not restore the even share preview: %s", f.Text())
	}
}
