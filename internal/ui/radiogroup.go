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
// keys, Home and End move and choose. The look lives in the radio-group styles, so every
// page that uses it gets the same hover, focus and selection motion.
func RadioGroup(props RadioGroupProps) uic.Node {
	return uic.CreateElement(radioGroup, props)
}

func radioGroup(props RadioGroupProps) uic.Node {
	groupID := uic.UseId()
	class := "radio-group"
	if props.Class != "" {
		class += " " + props.Class
	}
	return Div(ClassStr(class), Attr("role", "radiogroup"), Attr("aria-label", props.Label),
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

// anyChecked reports whether the group's value names an enabled option.
func anyChecked(group RadioGroupProps) bool {
	for _, option := range group.Options {
		if option.Value == group.Value && !option.Disabled {
			return true
		}
	}
	return false
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
