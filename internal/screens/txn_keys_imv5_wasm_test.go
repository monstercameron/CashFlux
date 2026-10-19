// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import "testing"

func TestIMV5EnterHandlerIgnoresMenuAndDialogTargets(t *testing.T) {
	for _, tt := range []struct {
		name, tag, role string
		inOverlay       bool
		want            bool
	}{
		{name: "import menu item", tag: "div", role: "menuitem", inOverlay: true, want: true},
		{name: "menu container", tag: "div", role: "menu", inOverlay: true, want: true},
		{name: "import button", tag: "button", want: true},
		{name: "dialog content", tag: "div", inOverlay: true, want: true},
		{name: "ordinary page target", tag: "div", want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := txnEnterActionIgnored(tt.tag, tt.role, tt.inOverlay); got != tt.want {
				t.Fatalf("txnEnterActionIgnored(%q, %q, %t) = %t, want %t", tt.tag, tt.role, tt.inOverlay, got, tt.want)
			}
		})
	}
}
