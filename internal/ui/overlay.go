// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

// OverlayKind selects the placement and accessibility behavior for an overlay.
type OverlayKind string

const (
	// OverlayModal renders a centered modal dialog with a backdrop and focus management.
	OverlayModal OverlayKind = "modal"
	// OverlaySheet renders a right-side modal sheet anchored to the viewport.
	OverlaySheet OverlayKind = "sheet"
	// OverlayPopover renders a non-modal surface positioned by its anchor.
	OverlayPopover OverlayKind = "popover"
	// OverlayBottomBar renders an action region anchored to the viewport bottom.
	OverlayBottomBar OverlayKind = "bottom-bar"
)

// OverlayProps configures a portal-backed Overlay. Title or Label supplies its accessible name.
type OverlayProps struct {
	Open     bool
	Kind     OverlayKind
	Title    string
	Label    string
	AnchorID string
	OnClose  func()
	Body     uic.Node
	Footer   uic.Node
	TestID   string
}

// Overlay renders modal, sheet, popover, and bottom-bar surfaces through a portal to body.
// Modal and Sheet trap focus, close on Escape, and restore focus when dismissed. Popover and
// BottomBar remain non-modal. Callers control Open and provide OnClose; body and footer are slots.
func Overlay(props OverlayProps) uic.Node {
	return uic.CreateElement(overlayView, props)
}

// Sheet renders props as a right-side viewport sheet, using the shared Overlay contract.
func Sheet(props OverlayProps) uic.Node {
	props.Kind = OverlaySheet
	return Overlay(props)
}

// BottomBar renders props as an action region anchored to the viewport bottom.
func BottomBar(props OverlayProps) uic.Node {
	props.Kind = OverlayBottomBar
	return Overlay(props)
}

func overlayView(props OverlayProps) uic.Node {
	id := uic.UseId()
	labelID := id + "-label"
	kind := props.Kind
	if kind == "" {
		kind = OverlayModal
	}
	role := "dialog"
	modal := true
	trapFocus := true
	backdrop := true
	closeOutside := true
	closeEscape := true
	restoreFocus := true
	baseZ := 3000 // Existing --z-modal token; GWC adds stack depth for nested surfaces.
	positioning := "viewport"
	surfaceClass := "cf-overlay__surface cf-overlay__surface--" + string(kind)
	surfaceClass += " " + string(css.New(overlaySurfaceRules...))
	backdropClass := "cf-overlay__backdrop " + string(css.New(overlayBackdropRules...))
	switch kind {
	case OverlayPopover:
		role, modal, trapFocus, backdrop, closeOutside = "region", false, false, false, true
		closeEscape = true
		baseZ = 2000 // Existing --z-popover token.
		positioning = "anchor"
	case OverlayBottomBar:
		role, modal, trapFocus, backdrop, closeOutside = "region", false, false, false, false
		closeEscape, restoreFocus = false, false
		baseZ = 20 // Existing --z-sticky token.
	case OverlaySheet:
		baseZ = 3000 // Existing --z-modal token.
	case OverlayModal:
		baseZ = 3000 // Existing --z-modal token; the separate confirm host uses --z-dialog.
	}
	if kind == OverlayPopover {
		surfaceClass += " " + string(css.New(overlayPopoverRules...))
	}
	if kind == OverlayBottomBar {
		surfaceClass += " " + string(css.New(overlayBottomBarRules...))
	}
	if kind == OverlaySheet {
		surfaceClass += " " + string(css.New(overlaySheetRules...))
	}
	if kind == OverlayModal {
		surfaceClass += " " + string(css.New(overlayModalRules...))
	}

	name := props.Title
	if name == "" {
		name = props.Label
	}
	var heading uic.Node
	if name != "" {
		headingClass := "cf-overlay__title " + string(css.New(overlayTitleRules...))
		if props.Title == "" {
			headingClass += " " + string(css.New(overlayVisuallyHiddenRules...))
		}
		heading = H2(ID(labelID), css.Class(headingClass), name)
	}
	content := Div(
		css.Class("cf-overlay__content "+string(css.New(overlayContentRules...))),
		Attr("data-testid", props.TestID),
		heading,
		Div(css.Class("cf-overlay__body "+string(css.New(overlayBodyRules...))), props.Body),
		If(props.Footer != nil, Div(css.Class("cf-overlay__footer "+string(css.New(overlayFooterRules...))), props.Footer)),
	)
	anchorSelector := ""
	if props.AnchorID != "" {
		anchorSelector = "#" + props.AnchorID
	}
	return uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open:                props.Open,
		Target:              uic.PortalTarget{Selector: "body"},
		Kind:                overlayGWCKind(kind),
		Role:                role,
		LabelledBy:          labelIDIfNamed(name, labelID),
		Modal:               modal,
		TrapFocus:           trapFocus,
		RestoreFocus:        restoreFocus,
		CloseOnEscape:       closeEscape,
		CloseOnOutsideClick: closeOutside,
		Backdrop:            backdrop,
		LockScroll:          modal,
		BackgroundInert:     modal,
		BaseZIndex:          baseZ,
		AnchorSelector:      anchorSelector,
		Positioning:         positioning,
		BackdropClass:       backdropClass,
		SurfaceClass:        surfaceClass,
		Child:               content,
		OnDismiss:           props.OnClose,
	})
}

func labelIDIfNamed(name, id string) string {
	if name == "" {
		return ""
	}
	return id
}

func overlayGWCKind(kind OverlayKind) uic.OverlayKind {
	switch kind {
	case OverlayPopover:
		return uic.OverlayKindPopover
	case OverlaySheet, OverlayModal:
		return uic.OverlayKindDialog
	default:
		return uic.OverlayKindCustom
	}
}

var overlaySurfaceRules = []css.Rule{
	css.Raw("box-sizing", "border-box"),
	css.Raw("color", "var(--text)"),
	css.Raw("background", "var(--bg-elev)"),
	css.Raw("border", "1px solid var(--border)"),
	css.Raw("border-radius", "var(--radius-lg)"),
	css.Raw("box-shadow", "var(--wonder-shadow)"),
	css.Raw("display", "flex"),
	css.Raw("flex-direction", "column"),
	css.Raw("max-height", "100dvh"),
	css.Raw("overflow", "hidden"),
}

var overlayBackdropRules = []css.Rule{
	css.Raw("position", "fixed"),
	css.Raw("inset", "0"),
	css.Raw("background", "color-mix(in srgb, var(--bg) 48%, transparent)"),
}

var overlayModalRules = []css.Rule{
	css.Raw("position", "fixed"),
	css.Raw("top", "50%"),
	css.Raw("left", "50%"),
	css.Raw("transform", "translate(-50%, -50%)"),
	css.Raw("width", "min(90vw, 44rem)"),
	css.Raw("max-height", "min(90dvh, 56rem)"),
}

var overlaySheetRules = []css.Rule{
	css.Raw("position", "fixed"),
	css.Raw("inset-block", "0"),
	css.Raw("inset-inline-end", "0"),
	css.Raw("width", "min(100vw, 23.75rem)"),
	css.Raw("max-height", "100dvh"),
	css.Raw("border-radius", "var(--radius-lg) 0 0 var(--radius-lg)"),
}

var overlayPopoverRules = []css.Rule{
	css.Raw("position", "fixed"),
	css.Raw("max-width", "min(90vw, 24rem)"),
}

var overlayBottomBarRules = []css.Rule{
	css.Raw("position", "fixed"),
	css.Raw("inset-inline", "var(--space-4)"),
	css.Raw("inset-block-end", "var(--space-4)"),
	css.Raw("max-width", "calc(100vw - 2 * var(--space-4))"),
	css.Raw("margin-inline", "auto"),
}

var overlayContentRules = []css.Rule{
	css.Raw("display", "flex"),
	css.Raw("flex-direction", "column"),
	css.Raw("min-height", "0"),
	css.Raw("height", "100%"),
}

var overlayTitleRules = []css.Rule{
	css.Raw("flex", "none"),
	css.Raw("margin", "0"),
	css.Raw("padding", "var(--space-4) var(--space-5)"),
	css.Raw("border-bottom", "1px solid var(--border)"),
	css.Raw("font", "inherit"),
	css.Raw("font-size", "1.125rem"),
	css.Raw("font-weight", "600"),
}

var overlayBodyRules = []css.Rule{
	css.Raw("flex", "1 1 auto"),
	css.Raw("min-height", "0"),
	css.Raw("overflow", "auto"),
	css.Raw("padding", "var(--space-5)"),
}

var overlayFooterRules = []css.Rule{
	css.Raw("flex", "none"),
	css.Raw("display", "flex"),
	css.Raw("align-items", "center"),
	css.Raw("justify-content", "flex-end"),
	css.Raw("gap", "var(--space-2)"),
	css.Raw("padding", "var(--space-4) var(--space-5)"),
	css.Raw("border-top", "1px solid var(--border)"),
}

var overlayVisuallyHiddenRules = []css.Rule{
	css.Raw("position", "absolute"),
	css.Raw("width", "1px"),
	css.Raw("height", "1px"),
	css.Raw("padding", "0"),
	css.Raw("margin", "-1px"),
	css.Raw("overflow", "hidden"),
	css.Raw("clip", "rect(0, 0, 0, 0)"),
	css.Raw("white-space", "nowrap"),
	css.Raw("border", "0"),
}
