// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strconv"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// SegmentedControlProps configures either pressed choice buttons or a tablist.
// Classes stay caller-owned so existing surfaces retain their appearance.
type SegmentedControlProps struct {
	Label, Class, ButtonClass, ActiveClass, CountClass string
	Selected                                           string
	Tabs                                               bool
	Options                                            []SegmentOption
	OnSelect                                           func(string)
	Attrs                                              []any
}

// SegmentedControl renders stable keyed choices. Each button owns its event
// hook, so options may appear, disappear or reorder without moving handlers.
// Tabs use aria-selected and automatic activation with ArrowLeft/ArrowRight,
// Home and End, skipping disabled tabs. Other choice buttons use aria-pressed.
func SegmentedControl(props SegmentedControlProps) uic.Node {
	return uic.CreateElement(segmentedControl, props)
}

func segmentedControl(props SegmentedControlProps) uic.Node {
	groupID := uic.UseId()
	role := "group"
	if props.Tabs {
		role = "tablist"
		props.Selected = segmentTabStop(props.Options, props.Selected)
	}
	args := []any{ClassStr(props.Class), Attr("role", role), Attr("aria-label", props.Label)}
	args = append(args, props.Attrs...)
	args = append(args, MapKeyed(props.Options, func(option SegmentOption) any { return option.Key }, func(option SegmentOption) uic.Node {
		return uic.CreateElement(segmentButton, segmentButtonProps{Option: option, Control: props, GroupID: groupID})
	}))
	return Div(args...)
}

type segmentButtonProps struct {
	Option  SegmentOption
	Control SegmentedControlProps
	GroupID string
}

func segmentButton(props segmentButtonProps) uic.Node {
	option, control := props.Option, props.Control
	focus := uic.UseFocusManager()
	selectOption := uic.UseEvent(func(event uic.Event) {
		event.PreventDefault()
		if !option.Disabled && control.OnSelect != nil {
			control.OnSelect(option.Key)
		}
	})
	onKey := uic.UseEvent(func(event uic.KeyboardEvent) {
		if !control.Tabs || option.Disabled {
			return
		}
		next, handled := segmentKeyTarget(control.Options, option.Key, event.GetKey())
		if !handled {
			return
		}
		event.PreventDefault()
		if control.OnSelect != nil {
			control.OnSelect(next)
		}
		focus.FocusByID(props.GroupID + "-" + next)
	})
	active := option.Key == control.Selected
	if control.Tabs && option.Disabled {
		active = false
	}
	class := control.ButtonClass
	if active && control.ActiveClass != "" {
		class += " " + control.ActiveClass
	}
	if option.Class != "" {
		class += " " + option.Class
	}
	args := []any{ID(props.GroupID + "-" + option.Key), ClassStr(class), Type("button"), Disabled(option.Disabled), OnClick(selectOption), OnKeyDown(onKey)}
	if control.Tabs {
		args = append(args, Attr("role", "tab"), Attr("aria-selected", strconv.FormatBool(active)))
		tabIndex := "-1"
		if active && !option.Disabled {
			tabIndex = "0"
		}
		args = append(args, Attr("tabindex", tabIndex))
	} else {
		args = append(args, Attr("aria-pressed", strconv.FormatBool(active)))
	}
	if option.TestID != "" {
		args = append(args, Attr("data-testid", option.TestID))
	}
	if option.AriaLabel != "" {
		args = append(args, Attr("aria-label", option.AriaLabel))
	}
	if option.Title != "" {
		args = append(args, Title(option.Title))
	}
	args = append(args, option.Label)
	if option.ShowCount {
		args = append(args, Span(ClassStr(control.CountClass), strconv.Itoa(option.Count)))
	}
	return Button(args...)
}
