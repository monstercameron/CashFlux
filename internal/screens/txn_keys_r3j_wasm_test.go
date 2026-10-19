// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestR3JUnmountedReviewActionsFallThroughToLedger(t *testing.T) {
	calledReview, calledLedger := false, false
	actions := reviewKeyActions{Next: func() { calledReview = true }}
	got := dispatchReviewKey("j", actions, false, func(key string) bool {
		calledLedger = key == "j"
		return calledLedger
	})
	if !got || !calledLedger || calledReview {
		t.Fatalf("closed Review dispatch = (handled %t, ledger %t, review %t), want (true, true, false)", got, calledLedger, calledReview)
	}
}

func TestR3JMountedReviewOwnsItsActionsAndFallsBackForOtherKeys(t *testing.T) {
	calledReview, calledLedger := false, false
	actions := reviewKeyActions{Next: func() { calledReview = true }}
	fallback := func(string) bool { calledLedger = true; return true }
	if !dispatchReviewKey("j", actions, true, fallback) || !calledReview || calledLedger {
		t.Fatal("mounted Review did not own its j action exclusively")
	}
	calledReview, calledLedger = false, false
	if !dispatchReviewKey("x", actions, true, fallback) || calledReview || !calledLedger {
		t.Fatal("mounted Review did not offer an unmapped key to the ledger")
	}
}

func TestR3JFirstLedgerNavigationAndWrapping(t *testing.T) {
	tests := []struct {
		name       string
		idx, count int
		key        string
		hasFocus   bool
		want       int
	}{
		{name: "first j starts at first row", idx: -1, count: 3, key: "j", want: 0},
		{name: "first k starts at last row", idx: -1, count: 3, key: "k", want: 2},
		{name: "j advances", idx: 0, count: 3, key: "j", hasFocus: true, want: 1},
		{name: "k moves backward", idx: 2, count: 3, key: "k", hasFocus: true, want: 1},
		{name: "j wraps", idx: 2, count: 3, key: "j", hasFocus: true, want: 0},
		{name: "k wraps", idx: 0, count: 3, key: "k", hasFocus: true, want: 2},
		{name: "empty ledger", idx: -1, count: 0, key: "j", want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnNextRowIndex(tt.idx, tt.count, tt.key, tt.hasFocus); got != tt.want {
				t.Fatalf("txnNextRowIndex(%d, %d, %q, %t) = %d, want %d", tt.idx, tt.count, tt.key, tt.hasFocus, got, tt.want)
			}
		})
	}
}

func TestR3JEditShortcutUsesLiveRowID(t *testing.T) {
	for _, tt := range []struct {
		testID string
		wantID string
		wantOK bool
	}{
		{testID: "txn-row-r3j-1", wantID: "r3j-1", wantOK: true},
		{testID: "txn-rowedit", wantOK: false},
		{testID: "other-row-r3j-1", wantOK: false},
		{testID: "txn-row-", wantOK: false},
	} {
		gotID, gotOK := txnRowID(tt.testID)
		if gotID != tt.wantID || gotOK != tt.wantOK {
			t.Errorf("txnRowID(%q) = (%q, %t), want (%q, %t)", tt.testID, gotID, gotOK, tt.wantID, tt.wantOK)
		}
	}
}

func TestTxnHelpQuestionMarkClosesOnlyItsOpenGuide(t *testing.T) {
	for _, tt := range []struct {
		key       string
		guideOpen bool
		want      bool
	}{
		{key: "?", guideOpen: true, want: true},
		{key: "?", guideOpen: false, want: false},
		{key: "Escape", guideOpen: true, want: false},
		{key: "j", guideOpen: true, want: false},
	} {
		if got := txnHelpKeyCloses(tt.key, tt.guideOpen); got != tt.want {
			t.Errorf("txnHelpKeyCloses(%q, %t) = %t, want %t", tt.key, tt.guideOpen, got, tt.want)
		}
	}
}

func TestTxnKeyHelpPanelRendersAndReleasesItsListenersAcrossOpens(t *testing.T) {
	f := render.New(t)
	host := ui.CreateElement(TxnKeyHelpHost, struct{}{})
	f.Render(host)
	if got := txnKeyHelpListenerCountForTest(); got != 0 {
		t.Fatalf("closed guide listener count = %d, want 0", got)
	}

	for openNumber := 1; openNumber <= 2; openNumber++ {
		SetTxnKeyHelpOpen(true)
		f.Render(host)
		text := f.Text()
		if !strings.Contains(text, "Search transactions") || !strings.Contains(text, "Close") {
			t.Fatalf("open %d did not render the shortcut list and close action: %q", openNumber, text)
		}
		if dialog := f.ByRole("dialog", "Keyboard shortcuts"); !dialog.Exists() {
			t.Fatalf("open %d did not render the guide in the standard dialog component", openNumber)
		}
		if close := f.ByRole("button", "Close"); !close.Exists() {
			t.Fatalf("open %d did not render the standard dialog Close button", openNumber)
		}
		if got := txnKeyHelpListenerCountForTest(); got != 2 {
			t.Fatalf("open %d panel listener count = %d, want 2", openNumber, got)
		}

		SetTxnKeyHelpOpen(false)
		f.Render(host)
		if got := txnKeyHelpListenerCountForTest(); got != 0 {
			t.Fatalf("close %d panel listener count = %d, want 0", openNumber, got)
		}
	}
	if got := activeTxnFocusCallbacks.Load(); got != 0 {
		t.Fatalf("active deferred-focus callbacks after guide cycles = %d, want 0", got)
	}
}

func TestTxnFocusCallbackReleasesAfterItsTimerRuns(t *testing.T) {
	if got := activeTxnFocusCallbacks.Load(); got != 0 {
		t.Fatalf("active deferred-focus callbacks at test start = %d, want 0", got)
	}
	querySelector := js.FuncOf(func(js.Value, []js.Value) any { return js.Null() })
	defer querySelector.Release()
	doc := js.Global().Get("Object").New()
	doc.Set("querySelector", querySelector)

	scheduleTxnFocus(doc, "#missing-category")
	if got := activeTxnFocusCallbacks.Load(); got != 1 {
		t.Fatalf("active deferred-focus callbacks before timer = %d, want 1", got)
	}
	time.Sleep(20 * time.Millisecond)
	if got := activeTxnFocusCallbacks.Load(); got != 0 {
		t.Fatalf("active deferred-focus callbacks after timer = %d, want 0", got)
	}
}

func TestUXT4RowShortcutsOpenCategoryPickerAndGuide(t *testing.T) {
	row := js.Global().Get("Object").New()
	row.Set("getAttribute", txnKeyTestFunc(t, func(_ js.Value, args []js.Value) any {
		if args[0].String() == "data-testid" {
			return "txn-row-uxt4-1"
		}
		return js.Null()
	}))
	target := js.Global().Get("Object").New()
	target.Set("tagName", "TR")
	target.Set("closest", txnKeyTestFunc(t, func(_ js.Value, args []js.Value) any {
		if args[0].String() == ".txn-table tbody tr.row" {
			return row
		}
		return js.Null()
	}))
	focused := false
	field := js.Global().Get("Object").New()
	field.Set("focus", txnKeyTestFunc(t, func(js.Value, []js.Value) any { focused = true; return nil }))
	doc := js.Global().Get("Object").New()
	doc.Set("querySelector", txnKeyTestFunc(t, func(_ js.Value, args []js.Value) any {
		if args[0].String() == "input[data-testid='catpick-search']" {
			return field
		}
		return js.Null()
	}))
	var categoryID string
	if !handleTxnTableShortcut("c", target, doc, func(id string) { categoryID = id }, nil) {
		t.Fatal("c on the row was not handled")
	}
	time.Sleep(20 * time.Millisecond)
	if categoryID != "uxt4-1" || !focused {
		t.Fatalf("category picker open = %q, search focused = %t; want uxt4-1 and focus", categoryID, focused)
	}
	helpOpened := false
	if !handleTxnTableShortcut("?", target, doc, nil, func() { helpOpened = true }) || !helpOpened {
		t.Fatal("? on the row did not open the keyboard guide")
	}
}

func TestUXT4RowShortcutsIgnoreControlsAndUnrelatedTargets(t *testing.T) {
	for _, tag := range []string{"BUTTON", "INPUT", "SELECT"} {
		target := js.Global().Get("Object").New()
		target.Set("tagName", tag)
		if handleTxnTableShortcut("c", target, js.Null(), func(string) { t.Errorf("category opened from %s", tag) }, nil) {
			t.Errorf("shortcut on %s should be ignored", tag)
		}
	}
	target := js.Global().Get("Object").New()
	target.Set("tagName", "BODY")
	target.Set("closest", txnKeyTestFunc(t, func(js.Value, []js.Value) any { return js.Null() }))
	if handleTxnTableShortcut("?", target, js.Null(), nil, func() { t.Error("help opened outside a transaction row") }) {
		t.Fatal("shortcut outside a transaction row should be ignored")
	}
}

func TestUXT4CategoryPickerRestoresOpenerWithoutScrolling(t *testing.T) {
	focused := false
	preventScroll := false
	opener := js.Global().Get("Object").New()
	opener.Set("isConnected", true)
	opener.Set("focus", txnKeyTestFunc(t, func(_ js.Value, args []js.Value) any {
		focused = true
		preventScroll = len(args) > 0 && args[0].Get("preventScroll").Bool()
		return nil
	}))
	focusTxnOpener(opener)
	if !focused || !preventScroll {
		t.Fatalf("picker opener restored = %t, preventScroll = %t; want true and true", focused, preventScroll)
	}
}

func txnKeyTestFunc(t *testing.T, fn func(js.Value, []js.Value) any) js.Func {
	t.Helper()
	owned := js.FuncOf(fn)
	t.Cleanup(owned.Release)
	return owned
}
