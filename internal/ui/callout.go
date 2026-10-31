// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	uic "github.com/monstercameron/GoWebComponents/v7/ui"
)

// Callout tones. An empty or unknown tone renders as CalloutInfo.
const (
	CalloutInfo   = "info"
	CalloutWarn   = "warn"
	CalloutDanger = "danger"
	CalloutOK     = "ok"
)

// Callout live-region modes for CalloutProps.Live.
const (
	// CalloutLiveAlert announces the callout at once. It becomes role="alert" on a danger
	// callout and role="status" on any other tone.
	CalloutLiveAlert = "alert"
	// CalloutLiveStatus announces the callout politely as role="status".
	CalloutLiveStatus = "status"
)

// CalloutProps configures a Callout.
type CalloutProps struct {
	// Tone is one of CalloutInfo, CalloutWarn, CalloutDanger or CalloutOK. It defaults to CalloutInfo.
	Tone string
	// Title is an optional bold first line.
	Title string
	// Body is the message. It may be nil when the Title says enough.
	Body uic.Node
	// Action is an optional node placed after the message, such as a button or a link.
	Action uic.Node
	// Live makes the callout a live region: CalloutLiveAlert, CalloutLiveStatus, or empty for
	// none. A danger callout with CalloutLiveAlert uses role="alert"; any other callout with Live
	// set uses role="status".
	Live string
	// TestID is the data-testid of the root. Empty adds none.
	TestID string
}

// Callout renders an inline notice: a toned box with an optional title, a body and an action.
// It owns no hooks, so it is safe inside a variable-length loop.
func Callout(props CalloutProps) uic.Node {
	tone := calloutTone(props.Tone)
	args := []any{ClassStr("callout callout-" + tone)}
	if role := calloutRole(tone, props.Live); role != "" {
		args = append(args, Attr("role", role))
	}
	if props.TestID != "" {
		args = append(args, Attr("data-testid", props.TestID))
	}
	text := []any{ClassStr("callout-text")}
	if props.Title != "" {
		text = append(text, Strong(ClassStr("callout-title"), props.Title))
	}
	if props.Body != nil {
		text = append(text, Div(ClassStr("callout-body"), props.Body))
	}
	args = append(args, Div(text...))
	if props.Action != nil {
		args = append(args, Div(ClassStr("callout-action"), props.Action))
	}
	return Div(args...)
}

// calloutTone maps a caller tone to a known one, falling back to info.
func calloutTone(tone string) string {
	switch tone {
	case CalloutWarn, CalloutDanger, CalloutOK:
		return tone
	default:
		return CalloutInfo
	}
}

// calloutRole returns the ARIA role for a tone and live mode, or "" when the callout is not live.
func calloutRole(tone, live string) string {
	switch live {
	case CalloutLiveAlert:
		if tone == CalloutDanger {
			return "alert"
		}
		return "status"
	case CalloutLiveStatus:
		return "status"
	default:
		return ""
	}
}
