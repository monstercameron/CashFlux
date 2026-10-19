// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"math"
	"strconv"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v5/ui"
)

var (
	pickerTriggerStyles = css.New(tw.InlineFlex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Px3, tw.Py2, tw.Rounded, tw.BgBase, tw.TextFg, css.Raw("box-sizing", "border-box"), css.Raw("border", "1px solid var(--border)"))
	pickerSearchStyles  = css.New(tw.WFull, tw.Px3, tw.Py2, tw.Rounded)
	pickerListStyles    = css.New(tw.MaxH55vh, tw.OverflowYAuto, tw.Py1)
	pickerOptionStyles  = css.New(tw.Flex, tw.WFull, tw.ItemsCenter, tw.Gap2, tw.Rounded, tw.Px2, tw.Py1, tw.TextLeft)
	multiSheetStyles    = css.New(tw.WFull, tw.MaxW85, tw.RoundedXl, tw.BgBase, tw.P3, tw.ShadowLg)
	multiListStyles     = css.New(tw.MaxH55vh, tw.OverflowYAuto, tw.Flex, tw.FlexWrap, tw.Gap2)
	multiOptionStyles   = css.New(tw.InlineFlex, tw.ItemsCenter, tw.Gap2, tw.RoundedFull, tw.Px3, tw.Py2)
	secondaryTextStyles = css.New(tw.TextDim, css.FontSize(css.Rem(0.75)))
)

type pickerPosition struct {
	Top, Left, MaxHeight int
}

func pickerPopoverRules(position pickerPosition) []css.Rule {
	return []css.Rule{css.Position.Fixed, css.Top(css.Px(position.Top)), css.Left(css.Px(position.Left)), css.Raw("max-height", strconv.Itoa(position.MaxHeight)+"px"), css.Raw("overflow-y", "auto"), tw.MinW150, tw.RoundedXl, tw.BgBase, tw.P3, tw.ShadowLg}
}

func pickerViewportPosition(top, bottom, panelHeight, viewportHeight float64) pickerPosition {
	const margin = 16.0
	const maxPopoverHeight = 420.0
	if viewportHeight < 0 {
		viewportHeight = 0
	}
	if panelHeight < 0 {
		panelHeight = 0
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
	if maxHeight < 0 {
		maxHeight = 0
	}
	if openAbove {
		popoverTop = top - math.Min(panelHeight, float64(maxHeight))
		if popoverTop < margin {
			popoverTop = margin
		}
	}
	return pickerPosition{Top: int(math.Round(popoverTop)), MaxHeight: maxHeight}
}

// PickerProps configures a searchable, explicit-commit picker.
type PickerProps struct {
	ID, Label, Placeholder, SelectedLabel string
	Options                               []PickerOption
	Searchable                            bool
	OnCommit                              func(value string)
	OnCreate                              func(query string)
	TestID                                string
}

// Picker renders a button and a portalled listbox. The trigger opens on ArrowDown;
// focus enters the search field, arrows only change the active option, Enter/click
// commits once, and Escape closes without applying and returns focus to the trigger.
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
	position := uic.UseState(pickerPosition{MaxHeight: 420})
	filtered := FilterPickerOptions(props.Options, query.Get())
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
	onOpen := uic.UseEvent(func() { openPicker() })
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
				if props.OnCommit != nil {
					props.OnCommit(filtered[activeIndex].Value)
				}
				open.Set(false)
				focusPickerID(props.ID)
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
	uic.UseEffect(func() func() {
		if open.Get() {
			focusTarget := listID
			if props.Searchable {
				focusTarget = searchID
			}
			focusPickerID(focusTarget)
		}
		return nil
	}, open.Get())
	uic.UseEffect(func() func() {
		if !open.Get() {
			return nil
		}
		reposition := func() {
			document := js.Global().Get("document")
			if !document.Truthy() || document.Get("getElementById").Type() != js.TypeFunction {
				return
			}
			trigger := document.Call("getElementById", props.ID)
			if !trigger.Truthy() || trigger.Get("getBoundingClientRect").Type() != js.TypeFunction {
				return
			}
			bounds := trigger.Call("getBoundingClientRect")
			panelHeight := 420.0
			panel := document.Call("getElementById", props.ID+"-surface")
			if panel.Truthy() && panel.Get("getBoundingClientRect").Type() == js.TypeFunction {
				measuredHeight := panel.Call("getBoundingClientRect").Get("height").Float()
				contentHeight := panel.Get("scrollHeight")
				if contentHeight.Type() == js.TypeNumber && contentHeight.Float() > measuredHeight {
					measuredHeight = contentHeight.Float()
				}
				if measuredHeight > 0 {
					panelHeight = measuredHeight
				}
			}
			window := js.Global().Get("window")
			viewportHeight := window.Get("innerHeight").Float()
			placement := pickerViewportPosition(bounds.Get("top").Float(), bounds.Get("bottom").Float(), panelHeight, viewportHeight)
			placement.Left = int(math.Round(bounds.Get("left").Float()))
			position.Set(placement)
		}
		reposition()
		window := js.Global().Get("window")
		if !window.Truthy() {
			return nil
		}
		listener := js.FuncOf(func(js.Value, []js.Value) any { reposition(); return nil })
		window.Call("addEventListener", "resize", listener)
		window.Call("addEventListener", "scroll", listener, true)
		return func() {
			window.Call("removeEventListener", "resize", listener)
			window.Call("removeEventListener", "scroll", listener, true)
			listener.Release()
		}
	}, open.Get(), props.ID)

	triggerLabel := props.SelectedLabel
	if triggerLabel == "" {
		triggerLabel = props.Placeholder
	}
	trigger := Button(
		css.Class("cf-picker-trigger", pickerTriggerStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", props.Label), Attr("style", "border: 1px solid var(--border); box-sizing: border-box"),
		Attr("aria-haspopup", "listbox"), Attr("aria-expanded", boolAttr(open.Get())),
		Attr("aria-controls", listID), onOpenOption, onTriggerKeyOption, triggerLabel,
	)
	if props.TestID != "" {
		trigger = Button(
			css.Class("cf-picker-trigger", pickerTriggerStyles),
			Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", props.Label), Attr("style", "border: 1px solid var(--border); box-sizing: border-box"),
			Attr("aria-haspopup", "listbox"), Attr("aria-expanded", boolAttr(open.Get())),
			Attr("aria-controls", listID), Attr("data-testid", props.TestID), onOpenOption, onTriggerKeyOption, triggerLabel,
		)
	}
	var popup uic.Node = Fragment()
	positionRules := pickerPopoverRules(position.Get())
	panelClass := "cf-picker-popover " + string(css.New(positionRules...))
	var panel uic.Node = Fragment()
	if open.Get() {
		rows := make([]uic.Node, 0, len(filtered)+1)
		for index, option := range filtered {
			rows = append(rows, uic.CreateElement(pickerOptionRow, pickerOptionRowProps{
				ID: PickerOptionID(props.ID, index), Option: option, Active: index == activeIndex, Depth: option.Depth,
				OnChoose: func(value string) {
					if props.OnCommit != nil {
						props.OnCommit(value)
					}
					open.Set(false)
					focusPickerID(props.ID)
				},
			}))
		}
		var listbox uic.Node
		var search uic.Node = Fragment()
		if props.Searchable {
			search = Input(css.Class("cf-picker-search", pickerSearchStyles),
				Attr("id", searchID), Attr("type", "search"), Attr("role", "combobox"), Attr("aria-label", uistate.T("picker.search", props.Label)),
				Attr("aria-controls", listID), Attr("aria-expanded", "true"), Attr("aria-autocomplete", "list"),
				Attr("aria-activedescendant", activeID), Placeholder(props.Placeholder), Value(query.Get()), onQueryOption, onKeyOption)
		}
		listboxArgs := []any{Attr("id", listID), Attr("role", "listbox"), Attr("tabindex", "0"), Attr("aria-label", props.Label), Attr("aria-activedescendant", activeID), css.Class("cf-picker-list", pickerListStyles), onKeyOption}
		for _, row := range rows {
			listboxArgs = append(listboxArgs, row)
		}
		listbox = Div(listboxArgs...)
		panelChildren := []uic.Node{search, listbox}
		if props.OnCreate != nil && strings.TrimSpace(query.Get()) != "" && len(filtered) == 0 {
			panelChildren = append(panelChildren, uic.CreateElement(pickerCreateRow, pickerCreateRowProps{
				ID: props.ID + "-create", Query: strings.TrimSpace(query.Get()), OnCreate: props.OnCreate,
				OnChoose: func() { open.Set(false); focusPickerID(props.ID) },
			}))
		}
		panelArgs := make([]any, 0, len(panelChildren))
		for _, child := range panelChildren {
			panelArgs = append(panelArgs, child)
		}
		panel = Fragment(panelArgs...)
	}
	// Register the picker as an overlay surface so it participates in the shared
	// overlay stack. Nested surfaces get a z-index above their dialog and own
	// outside-click dismissal, so a click on an option cannot dismiss the dialog.
	initialFocus := listID
	if props.Searchable {
		initialFocus = searchID
	}
	popup = uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open: open.Get(), Target: uic.PortalTarget{Selector: componentProps.PortalTarget},
		SurfaceID: props.ID + "-surface", Kind: uic.OverlayKindPopover, Role: "region", LabelledBy: props.ID,
		CloseOnEscape: true, CloseOnOutsideClick: true, RestoreFocus: true,
		InitialFocusSelector: "#" + initialFocus, BaseZIndex: 3501,
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

func pickerOptionRow(props pickerOptionRowProps) uic.Node {
	selected := "false"
	if props.Active {
		selected = "true"
	}
	option := Button(css.Class("cf-picker-option", pickerOptionStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("role", "option"), Attr("aria-selected", selected),
		Attr("aria-disabled", boolAttr(props.Option.Disabled)), Attr("tabindex", "-1"),
		OnClick(func() {
			if !props.Option.Disabled && props.OnChoose != nil {
				props.OnChoose(props.Option.Value)
			}
		}),
		Attr("data-depth", props.Depth), props.Option.Label, If(props.Option.Secondary != "", Span(css.Class("cf-picker-secondary", secondaryTextStyles), props.Option.Secondary)),
	)
	indent := props.Depth
	if indent > 24 {
		indent = 24
	}
	for range indent {
		option = Div(css.Class("cf-picker-indent", css.New(tw.Ml1)), option)
	}
	return option
}

type pickerCreateRowProps struct {
	ID, Query string
	OnCreate  func(string)
	OnChoose  func()
}

func pickerCreateRow(props pickerCreateRowProps) uic.Node {
	return Button(css.Class("cf-picker-create", css.New(tw.Flex, tw.WFull, tw.ItemsCenter, tw.Gap2, tw.Rounded, tw.Px2, tw.Py2, tw.TextLeft, tw.TextFg)),
		Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", uistate.T("picker.createAria", props.Query)),
		OnClick(func() {
			if props.OnCreate != nil {
				props.OnCreate(props.Query)
			}
			if props.OnChoose != nil {
				props.OnChoose()
			}
		}),
		uistate.T("picker.create", props.Query),
	)
}

// MultiPickerProps configures a controlled listbox that toggles multiple values.
type MultiPickerProps struct {
	ID, Label, Placeholder string
	Options                []PickerOption
	Selected               []string
	OnToggle               func(value string)
	TestID                 string
}

// MultiPicker renders a portalled multi-select sheet. Each option has
// aria-selected state and click toggles it through OnToggle; Escape closes and
// restores focus to the trigger.
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
	listID := props.ID + "-listbox"
	active := uic.UseState(0)
	closePicker := func() { open.Set(false); focusPickerID(props.ID) }
	onTrigger := OnClick(func() { active.Set(0); open.Set(true) })
	trigger := Button(css.Class("cf-multipicker-trigger", pickerTriggerStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("aria-label", props.Label), Attr("style", "border: 1px solid var(--border); box-sizing: border-box"), Attr("aria-haspopup", "listbox"), Attr("aria-expanded", boolAttr(open.Get())), Attr("aria-controls", listID), onTrigger,
		props.Placeholder)
	if props.TestID != "" {
		trigger = Button(css.Class("cf-multipicker-trigger", pickerTriggerStyles),
			Attr("id", props.ID), Attr("type", "button"), Attr("data-testid", props.TestID), Attr("aria-label", props.Label), Attr("style", "border: 1px solid var(--border); box-sizing: border-box"), Attr("aria-haspopup", "listbox"), Attr("aria-expanded", boolAttr(open.Get())), Attr("aria-controls", listID), onTrigger, props.Placeholder)
	}
	options := FilterPickerOptions(props.Options, "")
	var panel uic.Node = Fragment()
	if open.Get() {
		rows := make([]uic.Node, 0, len(options))
		for index, option := range options {
			selected := containsPickerValue(props.Selected, option.Value)
			rows = append(rows, uic.CreateElement(multiPickerOptionRow, multiPickerOptionRowProps{
				ID: props.ID + "-option-" + strconv.Itoa(index), Option: option, Selected: selected, Active: index == active.Get(),
				OnToggle: func(value string) {
					if props.OnToggle != nil {
						props.OnToggle(value)
					}
				},
				OnMove: func(delta int) {
					next := PickerNextIndex(options, index, delta)
					if next >= 0 {
						active.Set(next)
						focusPickerID(props.ID + "-option-" + strconv.Itoa(next))
					}
				},
			}))
		}
		listArgs := []any{Attr("id", listID), Attr("role", "listbox"), Attr("aria-label", props.Label), Attr("aria-multiselectable", "true"), Attr("aria-activedescendant", props.ID+"-option-"+strconv.Itoa(active.Get())), css.Class("cf-multipicker-list", multiListStyles)}
		for _, row := range rows {
			listArgs = append(listArgs, row)
		}
		panel = Div(css.Class("cf-multipicker-sheet", multiSheetStyles),
			Div(Attr("id", props.ID+"-title"), props.Label), Div(listArgs...),
		)
	}
	initialFocus := ""
	if len(options) > 0 {
		initialFocus = "#" + props.ID + "-option-0"
	}
	overlay := uic.AccessibleOverlay(uic.AccessibleOverlayProps{
		Open: open.Get(), Target: uic.PortalTarget{Selector: componentProps.PortalTarget}, SurfaceID: props.ID + "-surface",
		Kind: uic.OverlayKindSheet, Role: "dialog", LabelledBy: props.ID + "-title", Modal: true,
		TrapFocus: true, RestoreFocus: true, CloseOnEscape: true, CloseOnOutsideClick: true,
		InitialFocusSelector: initialFocus, Child: panel, OnDismiss: closePicker,
	})
	return Fragment(trigger, overlay)
}

type multiPickerOptionRowProps struct {
	ID       string
	Option   PickerOption
	Selected bool
	Active   bool
	OnToggle func(string)
	OnMove   func(int)
}

func multiPickerOptionRow(props multiPickerOptionRowProps) uic.Node {
	onKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		delta := 0
		if event.GetKey() == "ArrowDown" {
			delta = 1
		} else if event.GetKey() == "ArrowUp" {
			delta = -1
		}
		if delta != 0 {
			event.PreventDefault()
			if props.OnMove != nil {
				props.OnMove(delta)
			}
		}
	})
	selected := "false"
	if props.Selected {
		selected = "true"
	}
	class := "cf-multipicker-option"
	if props.Active {
		class += " is-active"
	}
	return Button(css.Class(class, multiOptionStyles),
		Attr("id", props.ID), Attr("type", "button"), Attr("role", "option"), Attr("aria-selected", selected), Attr("aria-disabled", boolAttr(props.Option.Disabled)),
		OnKeyDown(onKey),
		OnClick(func() {
			if !props.Option.Disabled && props.OnToggle != nil {
				props.OnToggle(props.Option.Value)
			}
		}), props.Option.Label)
}

func containsPickerValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func focusPickerID(id string) {
	document := js.Global().Get("document")
	if document.Truthy() {
		if element := document.Call("getElementById", id); element.Truthy() {
			element.Call("focus")
		}
	}
}

func boolAttr(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
