// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/css"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

type pickerCSSCapture struct{ text string }

func (capture *pickerCSSCapture) Emit(_, text string) { capture.text += text }

func TestPickerTriggerHasVisibleInputBorder(t *testing.T) {
	f := render.New(t)
	renderPickerForTest(f, PickerProps{ID: "category-picker", Label: "Category", Placeholder: "Choose a category"})
	style := f.ByID("category-picker").Attr("style")
	if !strings.Contains(style, "border: 1px solid var(--border)") {
		t.Fatalf("picker trigger style %q does not include the visible input border", style)
	}
}

func TestPickerPopoverCSSClampsHeightAndScrolls(t *testing.T) {
	capture := &pickerCSSCapture{}
	previousSink := css.SetSink(capture)
	t.Cleanup(func() { css.SetSink(previousSink) })
	css.New(pickerPopoverRules(pickerPosition{Top: 713, Left: 19, MaxHeight: 419})...)
	style := capture.text
	for _, want := range []string{"max-height:419px", "overflow-y:auto"} {
		if !strings.Contains(style, want) {
			t.Errorf("picker popover CSS is missing %q", want)
		}
	}
}

func TestPickerViewportPositionKeepsPopoverInsideViewport(t *testing.T) {
	tests := []struct {
		name                    string
		top, bottom, height, vh float64
		wantTop, wantMaxHeight  int
	}{
		{name: "fits below", top: 100, bottom: 140, height: 260, vh: 900, wantTop: 140, wantMaxHeight: 420},
		{name: "flips and caps tall list", top: 800, bottom: 840, height: 556, vh: 900, wantTop: 380, wantMaxHeight: 420},
		{name: "clamps to short space below", top: 40, bottom: 80, height: 556, vh: 300, wantTop: 80, wantMaxHeight: 204},
		{name: "uses larger side and clamps above", top: 250, bottom: 290, height: 556, vh: 500, wantTop: 16, wantMaxHeight: 234},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickerViewportPosition(tt.top, tt.bottom, tt.height, tt.vh)
			if got.Top != tt.wantTop || got.MaxHeight != tt.wantMaxHeight {
				t.Fatalf("pickerViewportPosition() = %+v, want top=%d maxHeight=%d", got, tt.wantTop, tt.wantMaxHeight)
			}
			if got.Top < 16 || got.Top+got.MaxHeight > int(tt.vh)-16 {
				t.Fatalf("popover bounds top=%d height=%d exceed viewport height %.0f", got.Top, got.MaxHeight, tt.vh)
			}
		})
	}
}
