// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"testing"

	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

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

func mountOverlayTestRoot(t *testing.T, f *render.Fixture, overlay uic.Node) {
	t.Helper()
	f.Render(Body(ID("overlay-test-body"), Div(ID("caller-tile"), overlay)))
}
