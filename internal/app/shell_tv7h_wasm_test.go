// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"
	"testing"

	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

func TestTV7HSampleChipKeepsOneStableTopbarSlotAcrossRouteChanges(t *testing.T) {
	uistate.SetSampleActive(true)
	t.Cleanup(func() { uistate.SetSampleActive(false) })
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	for _, periodAware := range []bool{true, false, true, false, true} {
		f.Render(uic.CreateElement(tv7hTopbarSlotTest, shellResolutionSlotProps{PeriodAware: periodAware}))
		f.Stabilize()
		count := 0
		for _, node := range f.AllByTag("div") {
			if node.Attr("data-testid") == "sample-data-banner" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("sample chip slot count after periodAware=%t: %d, want one", periodAware, count)
		}
	}
}

func tv7hTopbarSlotTest(props shellResolutionSlotProps) uic.Node {
	return Div(
		uic.CreateElement(shellResolutionSlot, props),
		uic.CreateElement(SampleDataBanner, nil),
	)
}

func TestTV7HThemeToggleLabelTracksAppliedTheme(t *testing.T) {
	var atom interface{ Set(prefs.Prefs) }
	f := render.New(t)
	t.Cleanup(f.Cleanup)
	installTV7HThemeDocument(t)
	f.Render(uic.CreateElement(func() uic.Node {
		p := uistate.UsePrefs()
		atom = p
		return uic.CreateElement(ThemeToggle, nil)
	}, nil))
	f.Stabilize()

	initial := prefs.Default()
	initial.Theme = prefs.ThemeDark
	atom.Set(initial)
	f.Stabilize()
	button := tv7hThemeButton(t, f)
	if got, want := button.Attr("aria-label"), uistate.T("tv7h.themeLabel", uistate.T("settings.themeDark")); got != want {
		t.Fatalf("initial theme label = %q, want %q", got, want)
	}

	button.Click()
	f.Stabilize()
	button = tv7hThemeButton(t, f)
	want := uistate.T("tv7h.themeLabel", uistate.T("settings.themeLight"))
	if got := button.Attr("aria-label"); got != want {
		t.Fatalf("theme label after applying light = %q, want %q", got, want)
	}
	if got := button.Attr("data-theme-current"); got != string(prefs.ThemeLight) {
		t.Fatalf("theme toggle state = %q, want %q", got, prefs.ThemeLight)
	}
}

func installTV7HThemeDocument(t *testing.T) {
	t.Helper()
	old := js.Global().Get("document")
	noop := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	getElementByID := js.FuncOf(func(js.Value, []js.Value) any { return js.Null() })
	createElement := js.FuncOf(func(js.Value, []js.Value) any {
		element := js.Global().Get("Object").New()
		element.Set("setAttribute", noop)
		return element
	})
	style := js.Global().Get("Object").New()
	style.Set("setProperty", noop)
	root := js.Global().Get("Object").New()
	root.Set("style", style)
	root.Set("setAttribute", noop)
	root.Set("removeAttribute", noop)
	doc := js.Global().Get("Object").New()
	head := js.Global().Get("Object").New()
	head.Set("appendChild", noop)
	doc.Set("getElementById", getElementByID)
	doc.Set("createElement", createElement)
	doc.Set("head", head)
	doc.Set("documentElement", root)
	js.Global().Set("document", doc)
	t.Cleanup(func() {
		js.Global().Set("document", old)
		noop.Release()
		getElementByID.Release()
		createElement.Release()
	})
}

func tv7hThemeButton(t *testing.T, f *render.Fixture) *render.QueryNode {
	t.Helper()
	for _, node := range f.AllByTag("button") {
		if node.Attr("data-testid") == "theme-toggle" {
			return node
		}
	}
	t.Fatal("theme toggle is missing")
	return nil
}
