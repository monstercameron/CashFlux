// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"strings"
	"syscall/js"
	"testing"

	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

func mountSegmentedControl(t *testing.T) *render.Fixture {
	t.Helper()
	f := render.New(t)
	f.Render(uic.CreateElement(segmented, SegmentedProps{
		Options:  []SegOption{{Value: "month", Label: "Month"}, {Value: "year", Label: "Year"}},
		Selected: "year",
	}))
	return f
}

func withSegmentedDocument(t *testing.T, getElementByID func(js.Value, []js.Value) any) {
	t.Helper()
	previousDocument := js.Global().Get("document")
	getElementByIDFunc := js.FuncOf(getElementByID)
	document := js.Global().Get("Object").New()
	document.Set("getElementById", getElementByIDFunc)
	js.Global().Set("document", document)
	t.Cleanup(func() {
		js.Global().Set("document", previousDocument)
		getElementByIDFunc.Release()
	})
}

func TestSegmentedEffectSkipsGroupWithoutQuerySelector(t *testing.T) {
	group := js.Global().Get("Object").New()
	withSegmentedDocument(t, func(js.Value, []js.Value) any { return group })
	f := mountSegmentedControl(t)

	for _, diagnostic := range f.BuildDiagnostics() {
		if strings.Contains(strings.ToLower(diagnostic.Message), "effect panic") {
			t.Fatalf("segmented effect reported a panic: %+v", diagnostic)
		}
	}
	for _, log := range f.BuildLogs() {
		if strings.Contains(strings.ToLower(log.Message), "effect panic") {
			t.Fatalf("segmented effect logged a panic: %+v", log)
		}
	}
}

func TestSegmentedEffectPositionsPillWhenDOMSupportsIt(t *testing.T) {
	styles := make(map[string]string)
	setProperty := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 2 {
			styles[args[0].String()] = args[1].String()
		}
		return nil
	})
	style := js.Global().Get("Object").New()
	style.Set("setProperty", setProperty)
	pill := js.Global().Get("Object").New()
	pill.Set("style", style)
	active := js.Global().Get("Object").New()
	active.Set("offsetLeft", 37)
	active.Set("offsetWidth", 81)
	querySelector := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return js.Null()
		}
		switch args[0].String() {
		case ".seg-pill":
			return pill
		case ".seg-btn.active":
			return active
		default:
			return js.Null()
		}
	})
	group := js.Global().Get("Object").New()
	group.Set("querySelector", querySelector)
	withSegmentedDocument(t, func(js.Value, []js.Value) any { return group })
	t.Cleanup(func() {
		querySelector.Release()
		setProperty.Release()
	})

	f := mountSegmentedControl(t)
	if got, want := styles["opacity"], "1"; got != want {
		t.Errorf("pill opacity = %q, want %q", got, want)
	}
	if got, want := styles["transform"], fmt.Sprintf("translateX(%gpx)", 37.0); got != want {
		t.Errorf("pill transform = %q, want %q", got, want)
	}
	if got, want := styles["width"], fmt.Sprintf("%gpx", 81.0); got != want {
		t.Errorf("pill width = %q, want %q", got, want)
	}
	for _, diagnostic := range f.BuildDiagnostics() {
		if strings.Contains(strings.ToLower(diagnostic.Message), "effect panic") {
			t.Errorf("segmented effect reported a panic in a supported DOM: %+v", diagnostic)
		}
	}
}
