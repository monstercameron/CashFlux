// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestSOLP1ClosedRowDoesNotBuildOrSubscribePicker(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	rows, pickers := 0, 0
	txnPerfObserve = func(stage string, _ time.Duration) {
		if stage == "row" {
			rows++
		}
		if stage == "category-picker" {
			pickers++
		}
	}
	defer func() { txnPerfObserve = nil }()
	p := r4cRow("solp1-row")
	chosen := ""
	p.OnChooseCategory = func(id string) { chosen = id }
	root := func() ui.Node {
		_ = uistate.UseDataRevision().Get()
		return Table(Tbody(ui.CreateElement(txnFrameRow, &p)))
	}
	f.Render(ui.CreateElement(root))
	uistate.BumpDataRevision()
	f.Render(ui.CreateElement(root))
	if rows != 1 || pickers != 0 {
		t.Fatalf("unchanged row renders=%d picker bodies=%d, want 1/0", rows, pickers)
	}
	choose := byTestID(f, "button", "txn-category-choose")
	if choose == nil {
		t.Fatal("missing category control")
	}
	choose.Click()
	if chosen != p.ID || pickers != 0 {
		t.Fatalf("row choose emitted %q and built %d pickers", chosen, pickers)
	}
}

func TestSOLP1DerivedMemosInvalidateWithoutUIRevisionBump(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	app := appstate.Default
	oldGroups, oldSuggestions := txnGroupMembersMemoState, txnCatSuggestCache
	defer func() { txnGroupMembersMemoState, txnCatSuggestCache = oldGroups, oldSuggestions }()
	group := domain.TxnLink{Kind: domain.TxnLinkOrderGroup, TxnIDs: []string{"countline-expense", "countline-income"}}
	groups := map[string]domain.TxnLink{"countline-expense": group}
	before := txnGroupMembersMemo(app, 42, groups)
	inputs := txnCatSuggestInputsFor(app, 42)
	row, _ := app.TransactionByID("countline-expense")
	row.Desc = "Updated by a background store write"
	if err := app.Store().PutTransaction(row); err != nil {
		t.Fatal(err)
	}
	after := txnGroupMembersMemo(app, 42, groups)
	updatedInputs := txnCatSuggestInputsFor(app, 42)
	if after[row.ID].Desc != row.Desc || updatedInputs.byID[row.ID].Desc != row.Desc {
		t.Fatal("store revision change retained stale derived data")
	}
	if before[row.ID].Desc == row.Desc || inputs.byID[row.ID].Desc == row.Desc {
		t.Fatal("memo refresh mutated a retained prior snapshot")
	}
}

func TestSOLP1ScopeEffectClearsSelectionButSortDoesNot(t *testing.T) {
	setupTxnCountCashflowFixture(t)
	f := render.New(t)
	defer f.Cleanup()
	var filter state.Atom[uistate.TxFilter]
	var selection state.Atom[map[string]bool]
	var anchor state.Atom[string]
	root := func() ui.Node {
		filter, selection, anchor = uistate.UseTxFilter(), uistate.UseTxnSelection(), uistate.UseTxnSelAnchor()
		return ui.CreateElement(Transactions)
	}
	f.Render(ui.CreateElement(root))
	previous := filter.Get()
	defer func() { filter.Set(previous) }()
	selection.Set(map[string]bool{"countline-expense": true})
	anchor.Set("countline-expense")
	next := filter.Get()
	next.Sort = "amount"
	filter.Set(next)
	f.Render(ui.CreateElement(root))
	if len(selection.Get()) != 1 {
		t.Fatal("sort cleared a selection in the same result set")
	}
	next.Text = "different scope"
	filter.Set(next)
	f.Render(ui.CreateElement(root))
	if len(selection.Get()) != 0 || anchor.Get() != "" {
		t.Fatal("scope effect retained hidden selected rows or their anchor")
	}
}

func TestSOLP1FlagSettingsDoNotRunUnrelatedEnginesOrMutatePreferences(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			original := smart.Settings{}.SetEnabled(spikeFlagCode, enabled).SetEnabled("SMART-T2", true)
			flags := txnFlagEngineSettings(original)
			for _, feature := range smart.Catalog() {
				want := feature.Code == spikeFlagCode && enabled
				if flags.IsEnabled(feature.Code) != want {
					t.Errorf("flag setting %s = %v, want %v", feature.Code, flags.IsEnabled(feature.Code), want)
				}
			}
			if !original.IsEnabled("SMART-T2") || original.IsEnabled(spikeFlagCode) != enabled {
				t.Fatal("row adapter mutated stored settings")
			}
		})
	}
}
