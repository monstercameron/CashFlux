// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/css"
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/interop"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

var (
	pickerTriggerStyles = css.New(tw.InlineFlex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Px3, tw.Py2, tw.Rounded, tw.BgBase, tw.TextFg, css.Raw("box-sizing", "border-box"), css.Raw("border", "1px solid var(--border)"))
	pickerSearchStyles  = css.New(tw.WFull, tw.Px3, tw.Py2, tw.Rounded)
	pickerListStyles    = css.New(tw.MaxH55vh, tw.OverflowYAuto, tw.Py1)
	pickerOptionStyles  = css.New(tw.Flex, tw.WFull, tw.ItemsCenter, tw.Gap2, tw.Rounded, tw.Px2, tw.Py1, tw.TextLeft)
	secondaryTextStyles = css.New(tw.TextDim, css.FontSize(css.Rem(0.75)))
)

// maxPopoverHeight is the tallest a picker panel may grow, in pixels. Longer content scrolls
// inside the panel.
const maxPopoverHeight = 420

// pickerPosition is where a picker panel sits, in CSS pixels. Top and Left are the panel's
// top-left corner, MaxHeight caps its height, and Width is the trigger's width.
type pickerPosition struct {
	Top, Left, MaxHeight, Width int
}

// pickerPopoverRules returns the fixed-position styles for a picker panel at position.
func pickerPopoverRules(position pickerPosition) []css.Rule {
	return []css.Rule{css.Position.Fixed, css.Top(css.Px(position.Top)), css.Left(css.Px(position.Left)), css.Raw("max-height", strconv.Itoa(position.MaxHeight)+"px"), css.Raw("overflow-y", "auto"), tw.MinW150, tw.RoundedXl, tw.BgBase, tw.P3, tw.ShadowLg}
}

// pickerViewportPosition places a panel beside a trigger whose top and bottom edges are given.
// The panel opens below the trigger, unless there is too little room there and more room above.
// Its height is capped at the space it gets.
func pickerViewportPosition(top, bottom, panelHeight, viewportHeight float64) pickerPosition {
	const margin = 16.0
	if viewportHeight < 0 {
		viewportHeight = 0
	}
	panelHeight = math.Min(panelHeight, maxPopoverHeight)
	spaceBelow := math.Max(0, viewportHeight-bottom-margin)
	spaceAbove := math.Max(0, top-margin)
	openAbove := spaceBelow < panelHeight && spaceAbove > spaceBelow
	available := spaceBelow
	popoverTop := bottom
	if openAbove {
		available = spaceAbove
	}
	maxHeight := int(math.Round(math.Min(maxPopoverHeight, available)))
	if openAbove {
		popoverTop = top - math.Min(panelHeight, float64(maxHeight))
		if popoverTop < margin {
			popoverTop = margin
		}
	}
	return pickerPosition{Top: int(math.Round(popoverTop)), MaxHeight: maxHeight}
}

// pickerPanelPlacement measures the trigger and the open panel, found by element id, and returns
// where the panel sits beside the trigger. The panel's height is the taller of its box and its
// content, so a scrolling panel is measured at full height. ok is false when the trigger is missing.
func pickerPanelPlacement(triggerID, panelID string) (placement pickerPosition, ok bool) {
	trigger := domNode(triggerID)
	if !trigger.Truthy() {
		return pickerPosition{}, false
	}
	bounds := domValue(trigger, "getBoundingClientRect")
	panelHeight := float64(maxPopoverHeight)
	if panel := domNode(panelID); panel.Truthy() {
		measuredHeight := domValue(panel, "getBoundingClientRect").Get("height").Float()
		if contentHeight := panel.Get("scrollHeight"); contentHeight.Present() && contentHeight.Float() > measuredHeight {
			measuredHeight = contentHeight.Float()
		}
		if measuredHeight > 0 {
			panelHeight = measuredHeight
		}
	}
	viewportHeight := domDocument().Get("defaultView").Get("innerHeight").Float()
	placement = pickerViewportPosition(bounds.Get("top").Float(), bounds.Get("bottom").Float(), panelHeight, viewportHeight)
	placement.Left = int(math.Round(bounds.Get("left").Float()))
	placement.Width = int(math.Round(bounds.Get("width").Float()))
	return placement, true
}

// usePickerPlacement keeps an open panel beside its trigger. It measures on open, then again
// when the page scrolls or resizes. Scroll events arrive in bursts, so each burst is coalesced
// into one animation frame. A placement equal to the last one is not handed on, so the picker
// only re-renders when the panel really moves.
func usePickerPlacement(open bool, id string, setPosition func(pickerPosition)) {
	last := uic.UseRef(pickerPosition{})
	placed := uic.UseRef(false)
	queued := uic.UseRef(false)
	cancelFrame := uic.UseRef(func() {})
	place := func() {
		placement, ok := pickerPanelPlacement(id, id+"-surface")
		if !ok || (placed.Get() && placement == last.Get()) {
			return
		}
		last.Set(placement)
		placed.Set(true)
		setPosition(placement)
	}
	// Measure on open, before paint, so the panel never shows at a stale spot. Each open starts
	// with placed reset, so its first measurement is always handed on.
	uic.UseLayoutEffect(func() func() {
		if open {
			placed.Set(false)
			place()
		}
		return nil
	}, open, id)
	// onMove queues at most one placement per animation frame.
	onMove := func(uic.Event) {
		if !open || queued.Get() {
			return
		}
		queued.Set(true)
		cancelFrame.Set(interop.RequestAnimationFrame(func(float64) {
			queued.Set(false)
			place()
		}))
	}
	uic.UseWindowEvent("resize", onMove, open, id)
	uic.UseWindowEvent("scroll", onMove, open, id)
	// A frame still queued when the panel closes is cancelled, so it cannot re-place a closed panel.
	uic.UseLayoutEffect(func() func() {
		return func() {
			if queued.Get() {
				cancelFrame.Get()()
				queued.Set(false)
			}
		}
	}, open, id)
}

// multiSheetSurfaceStyle places the multi-select sheet beside its trigger. The sheet is as wide
// as the trigger, but at least 200px. Only the placement is inline. The sheet's colours and shape
// come from the shared styles.
func multiSheetSurfaceStyle(place pickerPosition) map[string]string {
	return map[string]string{
		"position":   "fixed",
		"top":        strconv.Itoa(place.Top) + "px",
		"left":       strconv.Itoa(place.Left) + "px",
		"width":      strconv.Itoa(max(place.Width, 200)) + "px",
		"max-height": strconv.Itoa(place.MaxHeight) + "px",
	}
}

// pickerSurfaceHasFocus reports whether keyboard focus is inside the element with the given id.
// It reads the document's active element, so it needs no event target.
func pickerSurfaceHasFocus(surfaceID string) bool {
	surface := domNode(surfaceID)
	active := domDocument().Get("activeElement")
	return surface.Truthy() && active.Truthy() && domValue(surface, "contains", active).Bool()
}

// PickerProps configures a Picker.
type PickerProps struct {
	// ID names the trigger. The list, the search field and the panel derive their ids from it.
	// Label is the accessible name of the trigger and of the list. Placeholder is the trigger's
	// text when SelectedLabel is empty, and the hint in the search field. SelectedLabel is the
	// trigger's text once a value is chosen.
	ID, Label, Placeholder, SelectedLabel string
	Options                               []PickerOption
	// Searchable adds a search field, and lets a typed letter open the picker with that letter as
	// the search.
	Searchable bool
	// OnCommit receives the value of the option the reader chooses.
	OnCommit func(value string)
	// OnCreate, when set, offers to create a new item named by the query when nothing matches it.
	OnCreate func(query string)
	// TestID is the data-testid of the trigger. Empty adds none.
	TestID string
}

// Picker renders a button that opens a listbox in a portal. Enter, Space or ArrowDown on the
// trigger opens it. When the picker is searchable, a typed letter opens it too. Focus then moves
// to the search field, or to the list when the picker is not searchable. Arrow keys move the
// active option. Enter or a click commits the value. Escape closes the picker without applying a
// value, and returns focus to the trigger.
func Picker(props PickerProps) uic.Node {
	return uic.CreateElement(pickerComponent, pickerComponentProps{Props: props, PortalTarget: "body"})
}

type pickerComponentProps struct {
	Props        PickerProps
	PortalTarget string
}

func pickerComponent(componentProps pickerComponentProps) uic.Node {
	props := componentProps.Props
	open := uic.UseState(false)
	query := uic.UseState("")
	active := uic.UseState(-1)
	position := uic.UseState(pickerPosition{MaxHeight: maxPopoverHeight})
	filtered := FilterPickerOptions(props.Options, query.Get())
	// If the stored index is out of range or on a disabled option, use the first enabled one.
	activeIndex := active.Get()
	if activeIndex < 0 || activeIndex >= len(filtered) || filtered[activeIndex].Disabled {
		activeIndex = PickerActiveIndex(filtered)
	}
	listID := props.ID + "-listbox"
	searchID := props.ID + "-search"
	activeID := ""
	if activeIndex >= 0 {
		activeID = PickerOptionID(props.ID, activeIndex)
	}

	openPicker := func() {
		query.Set("")
		active.Set(PickerActiveIndex(FilterPickerOptions(props.Options, "")))
		open.Set(true)
	}
	closePicker := func() {
		open.Set(false)
		query.Set("")
		focusPickerID(props.ID)
	}
	// chooseOption commits a value, closes the picker and returns focus to the trigger.
	chooseOption := func(value string) {
		if props.OnCommit != nil {
			props.OnCommit(value)
		}
		open.Set(false)
		focusPickerID(props.ID)
	}
	onOpen := uic.UseEvent(openPicker)
	onTriggerKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		if event.GetKey() == "ArrowDown" || event.GetKey() == "Enter" || event.GetKey() == " " {
			event.PreventDefault()
			openPicker()
		}
		if props.Searchable && len(event.GetKey()) == 1 && event.GetKey() != " " {
			query.Set(event.GetKey())
			active.Set(PickerActiveIndex(FilterPickerOptions(props.Options, event.GetKey())))
			open.Set(true)
		}
	})
	onQuery := uic.UseEvent(func(event uic.Event) {
		query.Set(event.GetValue())
		active.Set(PickerActiveIndex(FilterPickerOptions(props.Options, event.GetValue())))
	})
	onKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		switch event.GetKey() {
		case "ArrowDown", "ArrowUp":
			event.PreventDefault()
			delta := 1
			if event.GetKey() == "ArrowUp" {
				delta = -1
			}
			active.Set(PickerNextIndex(filtered, activeIndex, delta))
		case "Enter":
			event.PreventDefault()
			if activeIndex >= 0 && activeIndex < len(filtered) {
				chooseOption(filtered[activeIndex].Value)
			}
		case "Escape":
			event.PreventDefault()
			event.StopPropagation()
			closePicker()
		}
	})
	onOpenOption := OnClick(onOpen)
	onTriggerKeyOption := OnKeyDown(onTriggerKey)
	onQueryOption := OnInput(onQuery)
	onKeyOption := OnKeyDown(onKey)
	firstFocus := listID
	if props.Searchable {
		firstFocus = searchID
	}
	uic.UseEffect(func() func() {
		if open.Get() {
			focusPickerID(firstFocus)
		}
		return nil
	}, open.Get())
	usePickerPlacement(open.Get(), props.ID, position.Set)

	triggerLabel := props.SelectedLabel
	if triggerLabel == "" {
		triggerLabel = props.Placeholder
	}
	triggerArgs := []any{
		css.Class("cf-picker-trigger", pickerTriggerStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", props.Label), Attr("style", "border: 1px solid var(--border); box-sizing: border-box"),
		Attr("aria-haspopup", "listbox"), Attr("aria-expanded", boolAttr(open.Get())),
		Attr("aria-controls", listID), onOpenOption, onTriggerKeyOption,
	}
	if props.TestID != "" {
		triggerArgs = append(triggerArgs, Attr("data-testid", props.TestID))
	}
	trigger := Button(append(triggerArgs, triggerLabel)...)
	positionRules := pickerPopoverRules(position.Get())
	panelClass := "cf-picker-popover " + string(css.New(positionRules...))
	var panel uic.Node = Fragment()
	if open.Get() {
		listArgs := []any{Attr("id", listID), Attr("role", "listbox"), Attr("tabindex", "0"), Attr("aria-label", props.Label), Attr("aria-activedescendant", activeID), css.Class("cf-picker-list", pickerListStyles), onKeyOption}
		for index, option := range filtered {
			listArgs = append(listArgs, uic.CreateElement(pickerOptionRow, pickerOptionRowProps{
				ID: PickerOptionID(props.ID, index), Option: option, Active: index == activeIndex, Depth: option.Depth,
				OnChoose: chooseOption,
			}))
		}
		var search uic.Node = Fragment()
		if props.Searchable {
			search = Input(css.Class("cf-picker-search", pickerSearchStyles),
				Attr("id", searchID), Attr("type", "search"), Attr("role", "combobox"), Attr("aria-label", uistate.T("picker.search", props.Label)),
				Attr("aria-controls", listID), Attr("aria-expanded", "true"), Attr("aria-autocomplete", "list"),
				Attr("aria-activedescendant", activeID), Placeholder(props.Placeholder), Value(query.Get()), onQueryOption, onKeyOption)
		}
		panelChildren := []any{search, Div(listArgs...)}
		trimmedQuery := strings.TrimSpace(query.Get())
		if props.OnCreate != nil && trimmedQuery != "" && len(filtered) == 0 {
			panelChildren = append(panelChildren, uic.CreateElement(pickerCreateRow, pickerCreateRowProps{
				ID: props.ID + "-create", Query: trimmedQuery, OnCreate: props.OnCreate,
				OnChoose: func() { open.Set(false); focusPickerID(props.ID) },
			}))
		}
		panel = Fragment(panelChildren...)
	}
	// The panel is an accessible overlay, portalled to the body above any dialog it sits in. The
	// overlay handles Escape and outside presses itself, so a press on an option does not dismiss
	// the dialog behind it.
	popup := uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open: open.Get(), Target: uic.PortalTarget{Selector: componentProps.PortalTarget},
		SurfaceID: props.ID + "-surface", Kind: uic.OverlayKindPopover, Role: "region", LabelledBy: props.ID,
		CloseOnEscape: true, CloseOnOutsideClick: true, RestoreFocus: true,
		InitialFocusSelector: "#" + firstFocus, BaseZIndex: 3501,
		SurfaceClass: panelClass, Child: panel, OnDismiss: closePicker,
	})
	return Div(css.Class("cf-picker", css.New(tw.Relative)), trigger, popup)
}

type pickerOptionRowProps struct {
	ID       string
	Option   PickerOption
	Active   bool
	Depth    int
	OnChoose func(string)
}

// pickerOptionRow renders one option. The option is wrapped once per level of depth, so it
// indents with its place in the tree. Indentation is capped at 24 levels.
func pickerOptionRow(props pickerOptionRowProps) uic.Node {
	option := Button(css.Class("cf-picker-option", pickerOptionStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("role", "option"), Attr("aria-selected", boolAttr(props.Active)),
		Attr("aria-disabled", boolAttr(props.Option.Disabled)), Attr("tabindex", "-1"),
		OnClick(func() {
			if !props.Option.Disabled {
				props.OnChoose(props.Option.Value)
			}
		}),
		Attr("data-depth", props.Depth), props.Option.Label, If(props.Option.Secondary != "", Span(css.Class("cf-picker-secondary", secondaryTextStyles), props.Option.Secondary)),
	)
	for range min(props.Depth, 24) {
		option = Div(css.Class("cf-picker-indent", css.New(tw.Ml1)), option)
	}
	return option
}

type pickerCreateRowProps struct {
	ID, Query string
	OnCreate  func(string)
	OnChoose  func()
}

// pickerCreateRow offers to create the typed query as a new item. Choosing it runs OnCreate,
// then OnChoose.
func pickerCreateRow(props pickerCreateRowProps) uic.Node {
	return Button(css.Class("cf-picker-create", css.New(tw.Flex, tw.WFull, tw.ItemsCenter, tw.Gap2, tw.Rounded, tw.Px2, tw.Py2, tw.TextLeft, tw.TextFg)),
		Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", uistate.T("picker.createAria", props.Query)),
		OnClick(func() {
			props.OnCreate(props.Query)
			props.OnChoose()
		}),
		uistate.T("picker.create", props.Query),
	)
}

// MultiPickerProps configures a MultiPicker.
type MultiPickerProps struct {
	// ID names the trigger. The sheet, its title and its options derive their ids from it.
	ID string
	// Label is the accessible name of the trigger and the heading of the sheet.
	Label string
	// Placeholder is the trigger's text, usually a summary of what is chosen.
	Placeholder string
	// Options are the choices, in order.
	Options []PickerOption
	// Selected holds the values currently chosen.
	Selected []string
	// OnToggle receives the value of each option the reader clicks.
	OnToggle func(value string)
	// TestID names the trigger for tests. Empty adds no test id.
	TestID string
}

// MultiPicker is a field that opens a sheet of choices beside it. Choosing an option toggles it
// through OnToggle, and the sheet stays open so several options can be chosen in one visit. The
// sheet closes on Escape, on a press outside it, or when focus moves out of it. Escape also
// returns focus to the field.
func MultiPicker(props MultiPickerProps) uic.Node {
	return uic.CreateElement(multiPickerComponent, multiPickerComponentProps{Props: props, PortalTarget: "body"})
}

type multiPickerComponentProps struct {
	Props        MultiPickerProps
	PortalTarget string
}

func multiPickerComponent(componentProps multiPickerComponentProps) uic.Node {
	props := componentProps.Props
	open := uic.UseState(false)
	active := uic.UseState(0)
	place := uic.UseState(pickerPosition{MaxHeight: maxPopoverHeight})
	usePickerPlacement(open.Get(), props.ID, place.Set)
	listID := props.ID + "-listbox"
	surfaceID := props.ID + "-surface"

	onOpen := uic.UseEvent(func(uic.Event) {
		active.Set(0)
		open.Set(true)
	})
	// ArrowDown and ArrowUp on the trigger open the sheet at its first option.
	onTriggerKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		if key := event.GetKey(); key == "ArrowDown" || key == "ArrowUp" {
			event.PreventDefault()
			active.Set(0)
			open.Set(true)
		}
	})
	dismiss := func() { open.Set(false) }
	// leave closes the sheet when focus moves out of it. The check runs in the next animation
	// frame, after the browser has moved focus to its new owner, so moving between options inside
	// the sheet counts as staying in it.
	leave := func() {
		interop.RequestAnimationFrame(func(float64) {
			if !pickerSurfaceHasFocus(surfaceID) {
				dismiss()
			}
		})
	}

	trigger := []any{
		css.Class("cf-multi-trigger"), Type("button"),
		Attr("id", props.ID), Attr("aria-label", props.Label), Attr("aria-haspopup", "listbox"),
		Attr("aria-expanded", boolAttr(open.Get())), Attr("aria-controls", listID),
		OnClick(onOpen), OnKeyDown(onTriggerKey), props.Placeholder,
	}
	if props.TestID != "" {
		trigger = append(trigger, Attr("data-testid", props.TestID))
	}

	var panel uic.Node = Fragment()
	initialFocus := ""
	if open.Get() {
		options := FilterPickerOptions(props.Options, "")
		if len(options) > 0 {
			initialFocus = "#" + props.ID + "-option-0"
		}
		listArgs := []any{
			css.Class("cf-multi-list"), Attr("id", listID), Attr("role", "listbox"),
			Attr("aria-label", props.Label), Attr("aria-multiselectable", "true"),
			Attr("aria-activedescendant", props.ID+"-option-"+strconv.Itoa(active.Get())),
		}
		for index, option := range options {
			listArgs = append(listArgs, uic.CreateElement(multiPickerOptionRow, multiPickerOptionRowProps{
				ID:       props.ID + "-option-" + strconv.Itoa(index),
				Option:   option,
				Selected: slices.Contains(props.Selected, option.Value),
				OnToggle: props.OnToggle,
				OnLeave:  leave,
				OnMove: func(delta int) {
					if next := PickerNextIndex(options, index, delta); next >= 0 {
						active.Set(next)
						focusPickerID(props.ID + "-option-" + strconv.Itoa(next))
					}
				},
			}))
		}
		panel = Div(css.Class("cf-multi-sheet"),
			Div(css.Class("cf-multi-title"), Attr("id", props.ID+"-title"), props.Label),
			Div(listArgs...),
		)
	}

	// The sheet is an overlay in the body, so it stacks above any dialog it sits in.
	overlay := uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open: open.Get(), Target: uic.PortalTarget{Selector: componentProps.PortalTarget},
		SurfaceID: surfaceID, SurfaceClass: "cf-multi-surface", SurfaceStyle: multiSheetSurfaceStyle(place.Get()),
		Kind: uic.OverlayKindPopover, Role: "dialog", LabelledBy: props.ID + "-title",
		RestoreFocus: true, CloseOnEscape: true, CloseOnOutsideClick: true,
		InitialFocusSelector: initialFocus, Child: panel, OnDismiss: dismiss,
	})
	return Fragment(Button(trigger...), overlay)
}

type multiPickerOptionRowProps struct {
	ID       string
	Option   PickerOption
	Selected bool
	OnToggle func(string)
	OnMove   func(int)
	OnLeave  func()
}

func multiPickerOptionRow(props multiPickerOptionRowProps) uic.Node {
	onKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		delta := 0
		switch event.GetKey() {
		case "ArrowDown":
			delta = 1
		case "ArrowUp":
			delta = -1
		}
		if delta != 0 {
			event.PreventDefault()
			props.OnMove(delta)
		}
	})
	onClick := uic.UseEvent(func(uic.Event) {
		if !props.Option.Disabled && props.OnToggle != nil {
			props.OnToggle(props.Option.Value)
		}
	})
	onBlur := uic.UseEvent(func(uic.Event) {
		props.OnLeave()
	})
	return Button(css.Class("cf-multi-option"), Type("button"),
		Attr("id", props.ID), Attr("role", "option"),
		Attr("aria-selected", boolAttr(props.Selected)), Attr("aria-disabled", boolAttr(props.Option.Disabled)),
		OnKeyDown(onKey), OnClick(onClick), OnBlur(onBlur),
		props.Option.Label)
}

// focusPickerID moves focus to the element with the given id, if it is on the page.
func focusPickerID(id string) {
	if element := domNode(id); element.Truthy() {
		domValue(element, "focus")
	}
}

// boolAttr returns "true" or "false", the form ARIA attributes expect.
func boolAttr(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
