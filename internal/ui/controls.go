//go:build js && wasm

package ui

import (
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
}

// Segmented renders the candidate-C segmented toggle (`.seg`): a row of mutually
// exclusive options with the selected one highlighted. Generic and reused
// wherever a small either/or choice is needed (time resolution, theme, etc.).
func Segmented(props SegmentedProps) uic.Node {
	return uic.CreateElement(segmented, props)
}

func segmented(props SegmentedProps) uic.Node {
	return Div(Class("seg"),
		MapKeyed(props.Options,
			func(o SegOption) any { return o.Value },
			func(o SegOption) uic.Node {
				return uic.CreateElement(segButton, segButtonProps{
					Value:    o.Value,
					Label:    o.Label,
					Active:   o.Value == props.Selected,
					OnSelect: props.OnSelect,
				})
			},
		),
	)
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
	return Button(
		Class(cls),
		Type("button"),
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
	Label  string
	OnPrev func()
	OnNext func()
}

// StepperPill renders the candidate-C range pill (`.rpill`): a centered label
// flanked by previous/next chevrons. Generic — reused for any stepped value
// (period from/to, paging, etc.).
func StepperPill(props StepperPillProps) uic.Node {
	return uic.CreateElement(stepperPill, props)
}

func stepperPill(props StepperPillProps) uic.Node {
	onPrev, onNext := props.OnPrev, props.OnNext
	return Div(Class("rpill"),
		Button(Class("rstep"), Type("button"), OnClick(func() {
			if onPrev != nil {
				onPrev()
			}
		}), "‹"),
		Span(Class("rlabel fig"), props.Label),
		Button(Class("rstep"), Type("button"), OnClick(func() {
			if onNext != nil {
				onNext()
			}
		}), "›"),
	)
}
