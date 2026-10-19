// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestShortcutIMV5EnterIgnoresImportMenuAndDialogTargets(t *testing.T) {
	for _, tt := range []struct {
		name, tag, role string
		inOverlay       bool
		want            bool
	}{
		{name: "import menu item", tag: "div", role: "menuitem", inOverlay: true, want: true},
		{name: "menu container", tag: "div", role: "menu", inOverlay: true, want: true},
		{name: "button", tag: "button", role: "", want: true},
		{name: "dialog content", tag: "div", role: "", inOverlay: true, want: true},
		{name: "ordinary page target", tag: "div", role: "", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := globalEnterTargetIgnored(tt.tag, tt.role, tt.inOverlay); got != tt.want {
				t.Fatalf("globalEnterTargetIgnored(%q, %q, %t) = %t, want %t", tt.tag, tt.role, tt.inOverlay, got, tt.want)
			}
		})
	}
}
