// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "testing"

func TestInterceptPlainLeftClick(t *testing.T) {
	tests := []struct {
		name             string
		button           int
		defaultPrevented bool
		meta             bool
		control          bool
		shift            bool
		alt              bool
		want             bool
	}{
		{name: "plain left click", button: 0, want: true},
		{name: "middle click", button: 1},
		{name: "right click", button: 2},
		{name: "already prevented", button: 0, defaultPrevented: true},
		{name: "command click", button: 0, meta: true},
		{name: "control click", button: 0, control: true},
		{name: "shift click", button: 0, shift: true},
		{name: "alt click", button: 0, alt: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interceptPlainLeftClick(tt.button, tt.defaultPrevented, tt.meta, tt.control, tt.shift, tt.alt)
			if got != tt.want {
				t.Fatalf("interceptPlainLeftClick() = %v, want %v", got, tt.want)
			}
		})
	}
}
