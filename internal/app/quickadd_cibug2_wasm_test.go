// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestQuickAddHostOpensSharedModalCIBUG2(t *testing.T) {
	app, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = app
	uistate.SetQuickAdd(false)
	t.Cleanup(func() {
		appstate.Default = previous
		uistate.SetQuickAdd(false)
		_ = app.Store().Close()
	})

	f := render.New(t)
	defer f.Cleanup()
	renderHost := func() {
		f.Render(Body(ID("cibug2-body"), ui.CreateElement(QuickAddHost)))
	}
	renderHost()
	if len(f.BuildOverlaySurfaces()) != 0 {
		t.Fatal("Quick Add overlay should be absent while closed")
	}

	// Dashboard and ledger Add transaction controls both open this shared host
	// through the quick-add state atom.
	uistate.SetQuickAdd(true)
	renderHost()
	if f.ByRole("dialog", uistate.T("quickAdd.title")) == nil {
		t.Fatal("opening Quick Add did not render its named modal dialog")
	}
	if findAppTestID(f, "div", "txn-add-dialog") == nil {
		t.Fatal("opening Quick Add did not render the shared overlay surface")
	}
	surfaces := f.BuildOverlaySurfaces()
	if len(surfaces) != 1 || !surfaces[0].IsModal || surfaces[0].TrapFocusOwner == false {
		t.Fatalf("Quick Add overlay state = %#v, want one modal with focus trapping", surfaces)
	}
	if findAppTestID(f, "input", "txn-add-amount") == nil {
		t.Fatal("Quick Add modal opened without its Amount field")
	}
}
