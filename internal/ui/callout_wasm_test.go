// SPDX-License-Identifier: MIT

//go:build js && wasm

package ui

import (
	"strings"
	"testing"

	. "github.com/monstercameron/GoWebComponents/v7/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v7/testkit/render"
)

func TestCalloutRoleFollowsToneAndLive(t *testing.T) {
	cases := []struct {
		name, tone, live, wantRole, wantClass string
	}{
		{"danger alert", CalloutDanger, CalloutLiveAlert, "alert", "callout-danger"},
		{"warn alert is a status", CalloutWarn, CalloutLiveAlert, "status", "callout-warn"},
		{"ok status", CalloutOK, CalloutLiveStatus, "status", "callout-ok"},
		{"danger status", CalloutDanger, CalloutLiveStatus, "status", "callout-danger"},
		{"info not live", "", "", "", "callout-info"},
		{"unknown tone is info", "loud", CalloutLiveStatus, "status", "callout-info"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := render.New(t)
			defer f.Cleanup()
			f.Render(Callout(CalloutProps{Tone: tc.tone, Live: tc.live, Body: Span("Heads up"), TestID: "co"}))
			root := uiNodeByTestID(f, "co")
			if root == nil {
				t.Fatal("Callout root is missing")
			}
			if got := root.Attr("role"); got != tc.wantRole {
				t.Fatalf("role = %q, want %q", got, tc.wantRole)
			}
			if !containsClass(root.Attr("class"), "callout") || !containsClass(root.Attr("class"), tc.wantClass) {
				t.Fatalf("class = %q, want callout and %s", root.Attr("class"), tc.wantClass)
			}
		})
	}
}

func TestCalloutRendersTitleBodyAndAction(t *testing.T) {
	f := render.New(t)
	defer f.Cleanup()
	f.Render(Callout(CalloutProps{
		Tone: CalloutWarn, Title: "Balance is stale", Body: Span("Last updated 40 days ago."),
		Action: Button(Attr("data-testid", "fix"), "Update now"), TestID: "co",
	}))
	root := uiNodeByTestID(f, "co")
	text := root.Text()
	for _, want := range []string{"Balance is stale", "Last updated 40 days ago.", "Update now"} {
		if !strings.Contains(text, want) {
			t.Fatalf("callout text %q is missing %q", text, want)
		}
	}
	if buttonByTestID(f, "fix") == nil {
		t.Fatal("the Action node was not rendered")
	}
	if root.Attr("role") != "" {
		t.Fatalf("a callout without Live must carry no role, got %q", root.Attr("role"))
	}
}
