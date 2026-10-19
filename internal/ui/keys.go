// SPDX-License-Identifier: MIT

// Package ui holds reusable, props-driven view primitives shared across CashFlux.
package ui

// KeyBinding associates one exact key with an action and its help text keys.
type KeyBinding struct {
	Key          string
	Action       string
	HelpKey      string
	HelpLabelKey string
}

// KeyContext describes UI states that affect whether a shortcut may run.
type KeyContext struct {
	Editable   bool
	DialogOpen bool
}

// ActionFor finds the action bound to key in bindings.
//
// Editable controls keep their keystrokes. An open dialog consumes known
// shortcuts without running them, preventing a page-level shortcut from
// opening a second overlay. The bool reports whether the key was handled.
func ActionFor(bindings []KeyBinding, key string, ctx KeyContext) (string, bool) {
	if ctx.Editable {
		return "", false
	}
	for _, binding := range bindings {
		if binding.Key != key {
			continue
		}
		if ctx.DialogOpen {
			return "", true
		}
		return binding.Action, true
	}
	return "", false
}
