// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// RadioGroupProps configures a RadioGroup.
type RadioGroupProps struct {
	// Label names the group for assistive technology. Required.
	Label string
	// Value is the chosen option's value. An unknown or empty value leaves none chosen.
	Value string
	// Options are the choices, in order.
	Options []RadioOption
	// OnChange receives the value of a newly chosen option.
	OnChange func(string)
	// Class holds extra classes for the group, for a page that needs to place it.
	Class string
}

// RadioGroup renders a single-choice group with radiogroup semantics: each option is a
// radio with aria-checked, only the chosen option sits in the tab order, and the arrow
// keys, Home and End move and choose. One indicator slides behind the chosen option, so
// the selection travels from one choice to the next rather than fading in place. The
// look lives in the radio-group styles, so every page gets the same motion.
func RadioGroup(props RadioGroupProps) uic.Node {
	return uic.CreateElement(radioGroup, props)
}

func radioGroup(props RadioGroupProps) uic.Node {
	groupID := uic.UseId()
	class := "radio-group"
	if props.Class != "" {
		class += " " + props.Class
	}
	index := radioIndex(props.Options, props.Value)
	if index < 0 {
		class += " is-empty"
	}
	return Div(ClassStr(class), Attr("role", "radiogroup"), Attr("aria-label", props.Label),
		// Custom properties are written as a raw style attribute: the Style helper only sets
		// standard properties, so the indicator would never see its index.
		Attr("style", "--radio-count:"+strconv.Itoa(len(props.Options))+";--radio-index:"+strconv.Itoa(max(index, 0))),
		Span(ClassStr("radio-indicator"), Attr("aria-hidden", "true")),
		MapKeyed(props.Options, func(option RadioOption) any { return option.Value }, func(option RadioOption) uic.Node {
			return uic.CreateElement(radioOption, radioOptionProps{Option: option, Group: props, GroupID: groupID})
		}),
	)
}

type radioOptionProps struct {
	Option  RadioOption
	Group   RadioGroupProps
	GroupID string
}

func radioOption(props radioOptionProps) uic.Node {
	option, group := props.Option, props.Group
	focus := uic.UseFocusManager()
	choose := uic.UseEvent(func(event uic.Event) {
		event.PreventDefault()
		if !option.Disabled && group.OnChange != nil {
			group.OnChange(option.Value)
		}
	})
	onKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		next, handled := RadioKeyTarget(group.Options, group.Value, event.GetKey())
		if !handled {
			return
		}
		event.PreventDefault()
		if group.OnChange != nil {
			group.OnChange(next)
		}
		focus.FocusByID(props.GroupID + "-" + next)
	})
	checked := option.Value == group.Value && !option.Disabled
	// The chosen option takes the tab stop. With nothing chosen, the first enabled one does.
	tabStop := checked || (!anyChecked(group) && firstEnabled(group.Options) == option.Value)
	tabIndex := "-1"
	if tabStop && !option.Disabled {
		tabIndex = "0"
	}
	args := []any{ID(props.GroupID + "-" + option.Value), ClassStr(radioOptionClass(checked, option.Disabled)),
		Type("button"), Attr("role", "radio"), Attr("aria-checked", strconv.FormatBool(checked)),
		Attr("tabindex", tabIndex), Disabled(option.Disabled), OnClick(choose), OnKeyDown(onKey),
	}
	if option.TestID != "" {
		args = append(args, Attr("data-testid", option.TestID))
	}
	args = append(args, option.Label)
	return Button(args...)
}

func radioOptionClass(checked, disabled bool) string {
	class := "radio-option"
	if checked {
		class += " is-checked"
	}
	if disabled {
		class += " is-disabled"
	}
	return class
}

// radioIndex is the position of the chosen option, or -1 when none is chosen.
func radioIndex(options []RadioOption, value string) int {
	for i, option := range options {
		if option.Value == value && !option.Disabled {
			return i
		}
	}
	return -1
}

// anyChecked reports whether the group's value names an enabled option.
func anyChecked(group RadioGroupProps) bool {
	return radioIndex(group.Options, group.Value) >= 0
}

// firstEnabled is the value of the first enabled option, which takes the tab stop when
// nothing is chosen.
func firstEnabled(options []RadioOption) string {
	for _, option := range options {
		if !option.Disabled {
			return option.Value
		}
	}
	return ""
}
