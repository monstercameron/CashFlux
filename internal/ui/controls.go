// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"fmt"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

// focusRadioAt moves DOM focus to the nth [role=radio] child of the element
// identified by groupID. Called after arrow-key navigation so that the newly
// selected option receives focus synchronously (before the re-render updates
// tabindex). If the element or the index is out of range the call is a no-op.
func focusRadioAt(groupID string, index int) {
	doc := js.Global().Get("document")
	if doc.IsNull() || doc.IsUndefined() {
		return
	}
	el := doc.Call("getElementById", groupID)
	if el.IsNull() || el.IsUndefined() {
		return
	}
	radios := el.Call("querySelectorAll", "[role=radio]")
	if radios.IsNull() || radios.IsUndefined() {
		return
	}
	n := radios.Get("length").Int()
	if index < 0 || index >= n {
		return
	}
	radios.Call("item", index).Call("focus")
}

// SegOption is one choice in a Segmented control: a stable value and its label.
type SegOption struct {
	Value string
	Label string
}

// SegmentedProps configures a Segmented control.
type SegmentedProps struct {
	Options  []SegOption
	Selected string
	OnSelect func(value string)
	Label    string // accessible group name (role="radiogroup"); optional
}

// Segmented renders the candidate-C segmented toggle (`.seg`): a row of mutually
// exclusive options with the selected one highlighted. Generic and reused
// wherever a small either/or choice is needed (time resolution, theme, etc.).
func Segmented(props SegmentedProps) uic.Node {
	return uic.CreateElement(segmented, props)
}

func segmented(props SegmentedProps) uic.Node {
	groupID := uic.UseId()
	options := props.Options
	selected := props.Selected
	onSelect := props.OnSelect
	// Roving tabindex (ARIA radiogroup): exactly one option is a Tab stop — the
	// checked one, or the first when none is checked — and arrows move between them.
	anySelected := false
	firstVal := ""
	if len(options) > 0 {
		firstVal = options[0].Value
	}
	for _, o := range options {
		if o.Value == selected {
			anySelected = true
			break
		}
	}
	// move advances selection by delta and synchronously focuses the new radio so
	// keyboard users get immediate visual feedback even before the re-render.
	move := func(delta int) {
		if onSelect == nil || len(options) == 0 {
			return
		}
		i := 0
		for idx, o := range options {
			if o.Value == selected {
				i = idx
				break
			}
		}
		next := (i + delta + len(options)) % len(options)
		onSelect(options[next].Value)
		focusRadioAt(groupID, next)
	}
	// Sliding pill (§6.16): a single absolutely-positioned indicator slides under
	// the active segment, so switching animates instead of snapping a per-button
	// background between elements. Segments are content-sized (variable width), so
	// the pill is positioned by measuring the active button's offset at render time
	// and writing a STANDARD transform/width (not a CSS custom property — the html
	// Style() helper drops `--` keys; setProperty via js does not). The CSS keeps
	// `.seg-btn.active` background transparent so the pill shows through; this is a
	// wasm app, so the effect always runs and the indicator is never missing.
	uic.UseEffect(func() func() {
		doc := js.Global().Get("document")
		grp := doc.Call("getElementById", groupID)
		if !grp.Truthy() {
			return nil
		}
		pill := grp.Call("querySelector", ".seg-pill")
		if !pill.Truthy() {
			return nil
		}
		active := grp.Call("querySelector", ".seg-btn.active")
		st := pill.Get("style")
		if !active.Truthy() {
			st.Call("setProperty", "opacity", "0")
			return nil
		}
		left := active.Get("offsetLeft").Float()
		width := active.Get("offsetWidth").Float()
		st.Call("setProperty", "opacity", "1")
		st.Call("setProperty", "transform", fmt.Sprintf("translateX(%gpx)", left))
		st.Call("setProperty", "width", fmt.Sprintf("%gpx", width))
		return nil
	}, selected, len(options))

	args := []any{ID(groupID), css.Class("seg"), Attr("role", "radiogroup"),
		Div(css.Class("seg-pill"), Attr("aria-hidden", "true")),
		OnKeyDown(func(e uic.KeyboardEvent) {
			switch e.GetKey() {
			case "ArrowLeft", "ArrowUp":
				e.PreventDefault()
				move(-1)
			case "ArrowRight", "ArrowDown":
				e.PreventDefault()
				move(1)
			}
		})}
	if props.Label != "" {
		args = append(args, Attr("aria-label", props.Label))
	}
	args = append(args,
		MapKeyed(options,
			func(o SegOption) any { return o.Value },
			func(o SegOption) uic.Node {
				return uic.CreateElement(segButton, segButtonProps{
					Value:    o.Value,
					Label:    o.Label,
					Active:   o.Value == selected,
					TabStop:  o.Value == selected || (!anySelected && o.Value == firstVal),
					OnSelect: onSelect,
				})
			},
		),
	)
	return Div(args...)
}

type segButtonProps struct {
	Value    string
	Label    string
	Active   bool
	TabStop  bool // the single roving Tab stop in the radiogroup
	OnSelect func(value string)
}

// segButton is its own component so each option's click hook stays stable as the
// option list changes (the On*-hooks-in-loops rule).
func segButton(props segButtonProps) uic.Node {
	cls := "seg-btn"
	if props.Active {
		cls = "seg-btn active"
	}
	value, onSelect := props.Value, props.OnSelect
	checked := "false"
	if props.Active {
		checked = "true"
	}
	tabidx := "-1"
	if props.TabStop {
		tabidx = "0"
	}
	return Button(
		ClassStr(cls),
		Type("button"),
		Attr("role", "radio"),
		Attr("aria-checked", checked),
		Attr("tabindex", tabidx),
		OnClick(func() {
			if onSelect != nil {
				onSelect(value)
			}
		}),
		props.Label,
	)
}

// StepperPillProps configures a StepperPill.
type StepperPillProps struct {
	Label     string
	OnPrev    func()
	OnNext    func()
	PrevLabel string // accessible name for the ‹ button; default "Previous"
	NextLabel string // accessible name for the › button; default "Next"
}

// StepperPill renders the candidate-C range pill (`.rpill`): a centered label
// flanked by previous/next chevrons. Generic — reused for any stepped value
// (period from/to, paging, etc.).
func StepperPill(props StepperPillProps) uic.Node {
	return uic.CreateElement(stepperPill, props)
}

func stepperPill(props StepperPillProps) uic.Node {
	onPrev, onNext := props.OnPrev, props.OnNext
	prevLabel, nextLabel := props.PrevLabel, props.NextLabel
	if prevLabel == "" {
		prevLabel = "Previous"
	}
	if nextLabel == "" {
		nextLabel = "Next"
	}
	return Div(css.Class("rpill"),
		Button(css.Class("rstep"), Type("button"), Attr("aria-label", prevLabel), OnClick(func() {
			if onPrev != nil {
				onPrev()
			}
		}), Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4))),
		Span(css.Class("rlabel fig"), props.Label),
		Button(css.Class("rstep"), Type("button"), Attr("aria-label", nextLabel), OnClick(func() {
			if onNext != nil {
				onNext()
			}
		}), Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))),
	)
}

// ToggleProps configures a Toggle switch.
type ToggleProps struct {
	On       bool
	OnChange func(on bool)
	Label    string // accessible name (the switch has no visible text); optional
}

// Toggle renders the candidate-C pill switch (`.switch`). Generic on/off control
// reused by settings rows and anywhere a boolean is edited.
func Toggle(props ToggleProps) uic.Node { return uic.CreateElement(toggle, props) }

func toggle(props ToggleProps) uic.Node {
	cls := "switch"
	if props.On {
		cls += " on"
	}
	on, onChange := props.On, props.OnChange
	checked := "false"
	if on {
		checked = "true"
	}
	toggleFn := func() {
		if onChange != nil {
			onChange(!on)
		}
	}
	args := []any{
		ClassStr(cls),
		Attr("role", "switch"),
		Attr("aria-checked", checked),
		Attr("tabindex", "0"), // focusable: it's a div, so it needs this to be reachable
		OnClick(toggleFn),
		// Space/Enter operate the switch (PreventDefault on Space stops page scroll).
		OnKeyDown(func(e uic.KeyboardEvent) {
			if k := e.GetKey(); k == " " || k == "Spacebar" || k == "Enter" {
				e.PreventDefault()
				toggleFn()
			}
		}),
	}
	if props.Label != "" {
		args = append(args, Attr("aria-label", props.Label))
	}
	return Div(args...)
}

// ToggleRowProps configures a labeled ToggleRow.
type ToggleRowProps struct {
	Label    string
	On       bool
	OnChange func(on bool)
}

// ToggleRow renders the candidate-C settings row (`.toggle-row`): a label on the
// left and a Toggle on the right. The common building block of settings forms.
func ToggleRow(props ToggleRowProps) uic.Node { return uic.CreateElement(toggleRow, props) }

func toggleRow(props ToggleRowProps) uic.Node {
	return Div(css.Class("toggle-row"),
		Span(props.Label),
		Toggle(ToggleProps{On: props.On, OnChange: props.OnChange, Label: props.Label}),
	)
}

// SwatchProps configures a single color Swatch.
type SwatchProps struct {
	Color    string
	Selected bool
	TabStop  bool // the single roving Tab stop in the swatch radiogroup
	OnSelect func()
}

// Swatch renders a single selectable color chip (`.swatch`).
func Swatch(props SwatchProps) uic.Node { return uic.CreateElement(swatch, props) }

func swatch(props SwatchProps) uic.Node {
	cls := "swatch"
	if props.Selected {
		cls += " sel"
	}
	checked := "false"
	if props.Selected {
		checked = "true"
	}
	onSelect := props.OnSelect
	selectFn := func() {
		if onSelect != nil {
			onSelect()
		}
	}
	tabidx := "-1"
	if props.TabStop {
		tabidx = "0" // roving tabindex: one Tab stop, arrows move within the group
	}
	return Div(
		ClassStr(cls),
		Attr("role", "radio"),
		Attr("aria-checked", checked),
		Attr("aria-label", props.Color),
		Attr("tabindex", tabidx),
		Style(map[string]string{"background": props.Color}),
		OnClick(selectFn),
		// Space/Enter pick the color (PreventDefault on Space stops page scroll).
		OnKeyDown(func(e uic.KeyboardEvent) {
			if k := e.GetKey(); k == " " || k == "Spacebar" || k == "Enter" {
				e.PreventDefault()
				selectFn()
			}
		}),
	)
}

// SwatchPickerProps configures a SwatchPicker.
type SwatchPickerProps struct {
	Colors   []string
	Selected string
	OnSelect func(color string)
}

// SwatchPicker renders a row of color Swatches with one selected — the accent
// picker reused by widget and global appearance settings.
func SwatchPicker(props SwatchPickerProps) uic.Node {
	return uic.CreateElement(swatchPicker, props)
}

func swatchPicker(props SwatchPickerProps) uic.Node {
	groupID := uic.UseId()
	onSelect := props.OnSelect
	colors := props.Colors
	// Roving tabindex + arrow-key navigation (ARIA radiogroup): one Tab stop, and
	// Left/Up/Right/Down move selection (which follows focus) between swatches.
	anySelected := false
	firstColor := ""
	if len(colors) > 0 {
		firstColor = colors[0]
	}
	for _, c := range colors {
		if c == props.Selected {
			anySelected = true
			break
		}
	}
	// move advances selection by delta and synchronously focuses the new swatch so
	// keyboard users get immediate visual feedback even before the re-render.
	move := func(delta int) {
		if onSelect == nil || len(colors) == 0 {
			return
		}
		i := 0
		for idx, c := range colors {
			if c == props.Selected {
				i = idx
				break
			}
		}
		next := (i + delta + len(colors)) % len(colors)
		onSelect(colors[next])
		focusRadioAt(groupID, next)
	}
	return Div(ID(groupID), css.Class(tw.Flex, tw.Gap2, tw.ItemsCenter), Attr("role", "radiogroup"), Attr("aria-label", uistate.T("a11y.accentColor")),
		OnKeyDown(func(e uic.KeyboardEvent) {
			switch e.GetKey() {
			case "ArrowLeft", "ArrowUp":
				e.PreventDefault()
				move(-1)
			case "ArrowRight", "ArrowDown":
				e.PreventDefault()
				move(1)
			}
		}),
		MapKeyed(colors,
			func(c string) any { return c },
			func(c string) uic.Node {
				color := c
				return uic.CreateElement(swatch, SwatchProps{
					Color:    color,
					Selected: color == props.Selected,
					TabStop:  color == props.Selected || (!anySelected && color == firstColor),
					OnSelect: func() {
						if onSelect != nil {
							onSelect(color)
						}
					},
				})
			},
		),
	)
}
