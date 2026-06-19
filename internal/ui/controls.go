//go:build js && wasm

package ui

import (
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/uistate"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/ui"
)

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
	options := props.Options
	selected := props.Selected
	onSelect := props.OnSelect
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
	}
	args := []any{Class("seg"), Attr("role", "radiogroup"), OnKeyDown(func(e uic.KeyboardEvent) {
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
	return Button(
		Class(cls),
		Type("button"),
		Attr("role", "radio"),
		Attr("aria-checked", checked),
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
	return Div(Class("rpill"),
		Button(Class("rstep"), Type("button"), Attr("aria-label", prevLabel), OnClick(func() {
			if onPrev != nil {
				onPrev()
			}
		}), Icon(icon.ChevronLeft, Class("w-4 h-4"))),
		Span(Class("rlabel fig"), props.Label),
		Button(Class("rstep"), Type("button"), Attr("aria-label", nextLabel), OnClick(func() {
			if onNext != nil {
				onNext()
			}
		}), Icon(icon.ChevronRight, Class("w-4 h-4"))),
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
		Class(cls),
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
	return Div(Class("toggle-row"),
		Span(props.Label),
		Toggle(ToggleProps{On: props.On, OnChange: props.OnChange, Label: props.Label}),
	)
}

// SwatchProps configures a single color Swatch.
type SwatchProps struct {
	Color    string
	Selected bool
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
	return Div(
		Class(cls),
		Attr("role", "radio"),
		Attr("aria-checked", checked),
		Attr("aria-label", props.Color),
		Attr("tabindex", "0"), // focusable: a div needs this to be keyboard-reachable
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
	onSelect := props.OnSelect
	return Div(Class("flex gap-2 items-center"), Attr("role", "radiogroup"), Attr("aria-label", uistate.T("a11y.accentColor")),
		MapKeyed(props.Colors,
			func(c string) any { return c },
			func(c string) uic.Node {
				color := c
				return uic.CreateElement(swatch, SwatchProps{
					Color:    color,
					Selected: color == props.Selected,
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
