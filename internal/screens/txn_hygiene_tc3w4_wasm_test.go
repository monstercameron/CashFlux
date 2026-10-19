// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTC3W4RowInfoWithoutActionsIsVisibleTextNotNoOpButton(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(txnRowInfo, txnRowInfoProps{Text: "Category added automatically"}))
	if byTestID(f, "button", "txn-row-info") != nil {
		t.Fatal("informational text rendered as a no-op button")
	}
	if byTestID(f, "span", "txn-row-info-text") == nil {
		t.Fatal("row information is not visible when no action is available")
	}
}

func TestTC3W4RowInfoActionsUseDismissiblePopover(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Body(ui.CreateElement(txnRowInfo, txnRowInfoProps{
		Text: "1 receipt. 1 open follow-up", HasReceipt: true, HasFollowUps: true,
	})))
	trigger := byTestID(f, "button", "txn-row-info")
	if trigger == nil || trigger.Attr("aria-haspopup") != "true" {
		t.Fatal("combined receipt and follow-up actions have no accessible disclosure trigger")
	}
	trigger.Click()
	f.Stabilize()
	if got := len(f.BuildOverlaySurfaces()); got != 1 {
		t.Fatalf("combined actions did not open the overlay popover: surfaces=%d expanded=%q text=%q", got, trigger.Attr("aria-expanded"), f.Text())
	}
	if f.BuildOverlayEscapeSurfaceID() == "" {
		t.Fatal("open row-info popover did not register Escape dismissal")
	}
}

func TestTC1W4AfterSettleTimerHonorsUnmountCleanup(t *testing.T) {
	called := false
	cancel := afterSettleTimer(25*time.Millisecond, func() { called = true })
	cancel()
	time.Sleep(40 * time.Millisecond)
	if called {
		t.Fatal("cancelled settle timer updated state after cleanup")
	}
}

func TestTC3W4SaveStatusUsesCatalogCopy(t *testing.T) {
	if got := txnSaveStatusText(txnSavePending); got != "Saving..." {
		t.Fatalf("pending copy = %q", got)
	}
	if got := txnSaveStatusText(txnSaveFailed); got != "Not saved" {
		t.Fatalf("failed copy = %q", got)
	}
	if got := txnSaveStatusText(txnSaveIdle); got != "" {
		t.Fatalf("idle copy = %q, want empty", got)
	}
}

func TestTC3W4SaveStatusRegistryPrunesUnmountedRows(t *testing.T) {
	a := sol1EditApp(t)
	previous := appstate.Default
	appstate.Default = a
	t.Cleanup(func() { appstate.Default = previous })
	id := "tc3w4-pruned-status"
	f := render.New(t)
	defer f.Cleanup()
	root := func() ui.Node {
		useTxnSaveStatus(id)
		return Div()
	}
	f.Render(ui.CreateElement(root))
	if _, ok := txnSaveStatuses[id]; !ok {
		t.Fatal("mounted row has no save status")
	}
	f.Rerender(Div())
	if _, ok := txnSaveStatuses[id]; ok {
		t.Fatal("unmounted row retained its save status entry")
	}
}

func TestTC3W4FlashKeyIsCompactAndTracksTransactionChanges(t *testing.T) {
	transaction := domain.Transaction{ID: "row-1", Desc: "Coffee"}
	first := txnRowFlashKey(transaction)
	if len(first) != len(transaction.ID)+1+64 {
		t.Fatalf("flash key length = %d, want compact id plus SHA-256 digest", len(first))
	}
	if again := txnRowFlashKey(transaction); again != first {
		t.Fatalf("unchanged transaction key changed: %q != %q", again, first)
	}
	transaction.Desc = "Tea"
	if changed := txnRowFlashKey(transaction); changed == first {
		t.Fatal("edited transaction reused its previous flash key")
	}
}
