// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"
	"time"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func TestStreamingBubbleScrollRunsAfterCommitAndCancelsOnUnmount(t *testing.T) {
	stubBudgetRowBrowser(t)
	doc := js.Global().Get("document")
	previousGet := doc.Get("getElementById")
	scroller := js.Global().Get("Object").New()
	scroller.Set("scrollHeight", 321)
	scroller.Set("scrollTop", 0)
	reads := 0
	get := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].String() == "cf-chat-scroll" {
			reads++
			return scroller
		}
		return js.Null()
	})
	doc.Set("getElementById", get)
	t.Cleanup(func() { doc.Set("getElementById", previousGet); get.Release() })
	f := render.New(t)
	t.Cleanup(func() { f.Render(ui.Fragment()); f.Stabilize(); f.Cleanup() })
	f.Render(ui.CreateElement(StreamingBubble, streamingBubbleProps{Text: "First"}))
	f.Stabilize()
	f.Render(ui.Fragment())
	f.Stabilize()
	time.Sleep(25 * time.Millisecond)
	if reads != 0 {
		t.Fatal("removed streaming bubble retained a pending scroll")
	}
	f.Render(ui.CreateElement(StreamingBubble, streamingBubbleProps{Text: "Second"}))
	f.Stabilize()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) && (reads != 1 || scroller.Get("scrollTop").Int() != 321) {
		time.Sleep(time.Millisecond)
	}
	if reads != 1 || scroller.Get("scrollTop").Int() != 321 {
		t.Fatalf("mounted bubble did not scroll its own canvas: reads=%d top=%v", reads, scroller.Get("scrollTop"))
	}
	f.Render(ui.CreateElement(StreamingBubble, streamingBubbleProps{Text: "Updated"}))
	f.Stabilize()
	f.Render(ui.Fragment())
	f.Stabilize()
	time.Sleep(25 * time.Millisecond)
	if reads != 1 {
		t.Fatal("updated bubble's scroll survived unmount")
	}
}
