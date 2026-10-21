// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/txnscope"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestTC4N11UpcomingScopeUsesCalendarDayBounds(t *testing.T) {
	newYork := time.FixedZone("America/New_York", -4*60*60)
	cases := []struct {
		name string
		now  time.Time
		from time.Time
		to   time.Time
		want bool
	}{
		{
			name: "early local morning remains inside its date despite previous UTC day",
			now:  time.Date(2026, time.October, 20, 0, 30, 0, 0, newYork),
			from: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "late local evening remains inside its date despite next UTC day",
			now:  time.Date(2026, time.October, 20, 23, 30, 0, 0, newYork),
			from: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			want: true,
		},
		{
			name: "date after selected period is outside",
			now:  time.Date(2026, time.October, 21, 0, 0, 0, 0, newYork),
			from: time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC),
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope := txnscope.Scope{Kind: txnscope.Custom, From: tc.from, To: tc.to}
			if got := txnUpcomingScopeIncludes(scope, tc.now); got != tc.want {
				t.Fatalf("scope inclusion = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestTC4N11UpcomingPopoverEscapeClosesAndReturnsFocus(t *testing.T) {
	previous := js.Global().Get("document")
	focusedID := ""
	var callbacks []js.Func
	makeFunc := func(fn func(js.Value, []js.Value) any) js.Func {
		f := js.FuncOf(fn)
		callbacks = append(callbacks, f)
		return f
	}
	t.Cleanup(func() {
		js.Global().Set("document", previous)
		for _, f := range callbacks {
			f.Release()
		}
	})
	toggle := js.Global().Get("Object").New()
	toggle.Set("focus", makeFunc(func(js.Value, []js.Value) any { focusedID = "txn-upcoming-toggle"; return nil }))
	doc := js.Global().Get("Object").New()
	doc.Set("getElementById", makeFunc(func(_ js.Value, args []js.Value) any {
		if args[0].String() == "txn-upcoming-toggle" {
			return toggle
		}
		return js.Null()
	}))
	js.Global().Set("document", doc)

	f := render.New(t)
	defer f.Cleanup()
	closed := false
	f.Render(txnUpcomingDisclosure(true, "1 bill still to come", ui.Handler{}, ui.Text("Scheduled bill"), func() { closed = true }))
	popover := byTestID(f, "div", "txn-upcoming-popover")
	if popover == nil || popover.Attr("tabindex") != "-1" || popover.Attr("aria-modal") != "false" {
		t.Fatal("upcoming dialog has no programmatic, non-modal focus target")
	}
	popover.Dispatch("onkeydown", render.Event{Key: "Escape"})
	if !closed {
		t.Fatal("Escape did not close the upcoming popover")
	}
	if focusedID != "txn-upcoming-toggle" {
		t.Fatalf("Escape returned focus to %q, want trigger", focusedID)
	}
}

func TestTC4N11UpcomingPopoverMovesFocusWhenOpened(t *testing.T) {
	previous := js.Global().Get("document")
	focusedID := ""
	var callbacks []js.Func
	makeFunc := func(fn func(js.Value, []js.Value) any) js.Func {
		f := js.FuncOf(fn)
		callbacks = append(callbacks, f)
		return f
	}
	t.Cleanup(func() {
		js.Global().Set("document", previous)
		for _, f := range callbacks {
			f.Release()
		}
	})
	popover := js.Global().Get("Object").New()
	popover.Set("focus", makeFunc(func(js.Value, []js.Value) any { focusedID = "txn-upcoming-popover"; return nil }))
	doc := js.Global().Get("Object").New()
	doc.Set("getElementById", makeFunc(func(_ js.Value, args []js.Value) any {
		if args[0].String() == "txn-upcoming-popover" {
			return popover
		}
		return js.Null()
	}))
	js.Global().Set("document", doc)

	f := render.New(t)
	defer f.Cleanup()
	f.Render(txnUpcomingDisclosure(true, "1 bill still to come", ui.Handler{}, ui.Text("Scheduled bill"), func() {}))
	f.Stabilize()
	if focusedID != "txn-upcoming-popover" {
		t.Fatalf("opening moved focus to %q, want the dialog", focusedID)
	}
}
