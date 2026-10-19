// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestQuickAddTV8FLayoutHasSharedDialogWidthHook(t *testing.T) {
	state, err := appstate.New(&bytes.Buffer{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.PutAccount(domain.Account{
		ID: "tv8f-checking", Name: "Checking", Currency: "USD", Type: domain.TypeChecking,
		Class: domain.ClassAsset, OwnerID: domain.GroupOwnerID, Scope: domain.ScopeShared,
	}); err != nil {
		t.Fatal(err)
	}
	previous := appstate.Default
	appstate.Default = state
	t.Cleanup(func() { appstate.Default = previous; uistate.SetQuickAdd(false) })

	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ID("tv8f-add-body"), ui.CreateElement(QuickAddHost)))
	uistate.SetQuickAdd(true)
	f.Render(Body(ID("tv8f-add-body"), ui.CreateElement(QuickAddHost)))
	for _, node := range f.AllByTag("div") {
		if strings.Contains(node.Attr("class"), "txn-add-layout") {
			return
		}
	}
	t.Fatal("Quick-Add body is missing the marker used to match its width to Edit")
}
