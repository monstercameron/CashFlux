// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"syscall/js"
	"testing"
	"time"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// CSS is a public contract here: the modal and sheet animate, respect both
// motion settings, and remain readable when blur is unsupported.
func TestOverlayMotionAndBackdropStyleContract(t *testing.T) {
	for _, want := range []string{
		".cf-overlay__surface--modal, .cf-overlay__surface--sheet",
		"var(--wonder-dur-slow, 280ms)",
		"cubic-bezier(0.16, 1, 0.3, 1)",
		"translate: 0 calc(var(--space-2) * var(--wonder-on, 1))",
		"scale: calc(1 - 0.02 * var(--wonder-on, 1))",
		"backdrop-filter: blur(4px)",
		"@supports not ((backdrop-filter:",
		"@media (prefers-reduced-motion: reduce)",
		"[data-wonder=\"off\"] .cf-overlay__surface--modal",
		"animation: none",
	} {
		if !strings.Contains(overlayMotionCSS, want) {
			t.Errorf("missing motion contract %q", want)
		}
	}
	for _, forbidden := range []string{"!important", "z-index", "flip-", "position: fixed"} {
		if strings.Contains(overlayMotionCSS, forbidden) {
			t.Errorf("unexpected motion CSS %q", forbidden)
		}
	}
}

func TestOverlayHasNamedModalRoleAndOwnsFocusContract(t *testing.T) {
	f := render.New(t)
	mountOverlayTestRoot(t, f, Overlay(OverlayProps{
		Open:  true,
		Kind:  OverlayModal,
		Title: "Edit transaction",
		Body:  Button(ID("overlay-action"), "Save"),
	}))

	if f.ByRole("dialog", "Edit transaction") == nil {
		t.Fatal("open modal is not exposed as a named dialog")
	}
	if got := f.BuildOverlayFocusSurfaceID(); got == "" {
		t.Fatal("open modal did not own the focus trap")
	}
	if got := f.BuildOverlayEscapeSurfaceID(); got == "" {
		t.Fatal("open modal did not register Escape dismissal")
	}
	if !f.BuildOverlayScrollLockActive() {
		t.Fatal("open modal did not lock background scrolling")
	}
	surfaces := f.BuildOverlaySurfaces()
	if len(surfaces) != 1 || surfaces[0].PortalTargetID != "overlay-test-body" {
		t.Fatalf("portal target = %#v, want the body outside the caller tile", surfaces)
	}
}

func TestOverlayDismissalCallsControlledClose(t *testing.T) {
	closed := 0
	f := render.New(t)
	mountOverlayTestRoot(t, f, Overlay(OverlayProps{
		Open:    true,
		Kind:    OverlayModal,
		Label:   "Confirm delete",
		Body:    Div("Delete this transaction?"),
		OnClose: func() { closed++ },
	}))
	if len(f.BuildOverlaySurfaces()) != 1 {
		t.Fatal("expected one active overlay surface")
	}
	if !f.HandleOverlayOutsideClick(f.BuildOverlaySurfaces()[0].SurfaceID) {
		t.Fatal("test harness could not dispatch backdrop dismissal")
	}
	if closed != 1 {
		t.Fatalf("close callback count = %d, want 1", closed)
	}
}

func TestSheetIsModalAndBottomBarIsNot(t *testing.T) {
	cases := []struct {
		name      string
		render    func(OverlayProps) uic.Node
		wantModal bool
		wantTrap  bool
	}{
		{name: "sheet", render: Sheet, wantModal: true, wantTrap: true},
		{name: "bottom bar", render: BottomBar},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			mountOverlayTestRoot(t, f, tc.render(OverlayProps{Open: true, Label: "Actions", Body: Button("Apply")}))
			surfaces := f.BuildOverlaySurfaces()
			if len(surfaces) != 1 {
				t.Fatalf("surface count = %d, want 1", len(surfaces))
			}
			if surfaces[0].IsModal != tc.wantModal {
				t.Errorf("modal = %t, want %t", surfaces[0].IsModal, tc.wantModal)
			}
			if surfaces[0].TrapFocusOwner != tc.wantTrap {
				t.Errorf("focus trap owner = %t, want %t", surfaces[0].TrapFocusOwner, tc.wantTrap)
			}
		})
	}
}

func TestPopoverUsesNonModalNamedRegion(t *testing.T) {
	f := render.New(t)
	mountOverlayTestRoot(t, f, Overlay(OverlayProps{
		Open:     true,
		Kind:     OverlayPopover,
		Label:    "More options",
		AnchorID: "more-trigger",
		Body:     Button("Archive"),
	}))
	if f.ByRole("region", "More options") == nil {
		t.Fatal("popover is not exposed as a named region")
	}
	if got := f.BuildOverlayFocusSurfaceID(); got != "" {
		t.Fatalf("non-modal popover unexpectedly owns focus trap %q", got)
	}
	if got := f.BuildOverlayEscapeSurfaceID(); got == "" {
		t.Fatal("open popover did not register Escape dismissal")
	}
}

func TestOverlayClosedDoesNotRenderSurface(t *testing.T) {
	f := render.New(t)
	mountOverlayTestRoot(t, f, Sheet(OverlayProps{Label: "Filters", Body: Div("Hidden filters")}))
	if got := len(f.BuildOverlaySurfaces()); got != 0 {
		t.Fatalf("closed sheet surface count = %d, want 0", got)
	}
}

func TestOverlayTrapsTabClosesOnEscapeAndRestoresEitherOpener(t *testing.T) {
	for _, opener := range []string{"description", "pencil"} {
		t.Run(opener, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			dom := newOverlayFocusDOM(t, opener)
			closed := 0
			props := OverlayProps{Open: true, Title: "Edit transaction", Body: Button("Save"), OnClose: func() { closed++ }}
			mountOverlayTestRoot(t, f, Overlay(props))
			if !dom.document.Get("activeElement").Equal(dom.first) {
				t.Fatal("opening did not focus the first dialog control")
			}
			dom.document.Set("activeElement", dom.last)
			dom.key("Tab", false)
			if !dom.document.Get("activeElement").Equal(dom.first) {
				t.Fatal("Tab escaped the dialog")
			}
			dom.key("Tab", true)
			if !dom.document.Get("activeElement").Equal(dom.last) {
				t.Fatal("Shift+Tab escaped the dialog")
			}
			dom.key("Escape", false)
			if closed != 1 {
				t.Fatalf("Escape emitted %d dismissals", closed)
			}
			props.Open = false
			mountOverlayTestRoot(t, f, Overlay(props))
			select {
			case got := <-dom.restored:
				if got != opener {
					t.Fatalf("restored %q, want %q", got, opener)
				}
			case <-time.After(time.Second):
				t.Fatal("closing did not restore the opener")
			}
			if len(dom.listeners["keydown"]) != 0 {
				t.Fatal("closed dialog retained keyboard listeners")
			}
		})
	}
}

// overlayFocusDOM supplies just the DOM focus/event seam that testkit's render
// bridge does not model. Each test owns its nodes, listeners and JS callbacks.
type overlayFocusDOM struct {
	document, first, last js.Value
	listeners             map[string][]js.Value
	prevent               js.Func
	restored              chan string
}

func newOverlayFocusDOM(t *testing.T, opener string) *overlayFocusDOM {
	t.Helper()
	previous := js.Global().Get("document")
	d := &overlayFocusDOM{document: js.Global().Get("Object").New(), listeners: map[string][]js.Value{}, restored: make(chan string, 1)}
	t.Cleanup(func() { js.Global().Set("document", previous) })
	callback := func(fn func(js.Value, []js.Value) any) js.Func {
		owned := js.FuncOf(fn)
		t.Cleanup(owned.Release)
		return owned
	}
	object := func() js.Value { return js.Global().Get("Object").New() }
	container, trigger := object(), object()
	d.first, d.last = object(), object()
	for _, node := range []js.Value{container, d.first, d.last, trigger} {
		node := node
		node.Set("focus", callback(func(js.Value, []js.Value) any {
			d.document.Set("activeElement", node)
			if node.Equal(trigger) {
				d.restored <- opener
			}
			return nil
		}))
	}
	container.Set("contains", callback(func(_ js.Value, args []js.Value) any {
		return args[0].Equal(d.first) || args[0].Equal(d.last) || args[0].Equal(container)
	}))
	container.Set("querySelectorAll", callback(func(js.Value, []js.Value) any { return js.ValueOf([]any{d.first, d.last}) }))
	lookup := callback(func(_ js.Value, args []js.Value) any {
		if strings.Contains(args[0].String(), "app") {
			return js.Null()
		}
		return container
	})
	d.document.Set("getElementById", lookup)
	d.document.Set("querySelector", lookup)
	d.document.Set("addEventListener", callback(func(_ js.Value, args []js.Value) any {
		name := args[0].String()
		d.listeners[name] = append(d.listeners[name], args[1])
		return nil
	}))
	d.document.Set("removeEventListener", callback(func(_ js.Value, args []js.Value) any {
		name := args[0].String()
		for i, listener := range d.listeners[name] {
			if listener.Equal(args[1]) {
				d.listeners[name] = append(d.listeners[name][:i], d.listeners[name][i+1:]...)
				break
			}
		}
		return nil
	}))
	d.prevent = callback(func(js.Value, []js.Value) any { return nil })
	body, style := object(), object()
	style.Set("overflow", "")
	body.Set("style", style)
	d.document.Set("body", body)
	d.document.Set("activeElement", trigger)
	js.Global().Set("document", d.document)
	return d
}

func (d *overlayFocusDOM) key(key string, shift bool) {
	event := js.Global().Get("Object").New()
	event.Set("key", key)
	event.Set("shiftKey", shift)
	event.Set("preventDefault", d.prevent)
	event.Set("stopPropagation", d.prevent)
	for _, listener := range append([]js.Value(nil), d.listeners["keydown"]...) {
		listener.Invoke(event)
	}
}

func mountOverlayTestRoot(t *testing.T, f *render.Fixture, overlay uic.Node) {
	t.Helper()
	f.Render(Body(ID("overlay-test-body"), Div(ID("caller-tile"), overlay)))
}
