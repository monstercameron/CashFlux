// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestUseAfterSettleTimerDoesNotRunAfterUnmount(t *testing.T) {
	called := make(chan struct{}, 1)
	cancel := afterSettleTimer(10*time.Millisecond, func() { called <- struct{}{} })
	cancel()
	time.Sleep(25 * time.Millisecond)
	select {
	case <-called:
		t.Fatal("settle callback ran after cleanup")
	default:
	}
}

func TestUseAfterSettleTimerRendersOnBrowserLoop(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(ui.CreateElement(func(_ struct{}) ui.Node {
		if useAfterSettle("timer-regression") {
			return Div("ready")
		}
		return Div("waiting")
	}, struct{}{}))
	if got := f.Text(); got != "waiting" {
		t.Fatalf("initial text = %q, want waiting", got)
	}
	deadline := time.Now().Add(time.Second)
	for f.Text() != "ready" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		f.Stabilize()
	}
	if got := f.Text(); got != "ready" {
		t.Fatalf("settled text = %q, want ready", got)
	}
}
