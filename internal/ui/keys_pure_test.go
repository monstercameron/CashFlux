// SPDX-License-Identifier: MIT

package ui

import "testing"

func txnKeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "/", Action: "search"},
		{Key: "j", Action: "next"},
		{Key: "k", Action: "previous"},
		{Key: "x", Action: "select"},
		{Key: "e", Action: "edit"},
		{Key: "Enter", Action: "edit"},
		{Key: "c", Action: "category"},
		{Key: "r", Action: "review"},
		{Key: "?", Action: "help"},
	}
}

func TestActionForTransactionBindings(t *testing.T) {
	tests := []struct {
		key  string
		want string
		ok   bool
	}{
		{"/", "search", true}, {"j", "next", true}, {"k", "previous", true},
		{"x", "select", true}, {"e", "edit", true}, {"Enter", "edit", true},
		{"c", "category", true}, {"r", "review", true}, {"?", "help", true},
		{"Escape", "", false}, {"J", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, ok := ActionFor(txnKeyBindings(), tt.key, KeyContext{})
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ActionFor(%q) = (%q, %t), want (%q, %t)", tt.key, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestActionForContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  KeyContext
		key  string
		want string
		ok   bool
	}{
		{"ordinary ledger", KeyContext{}, "j", "next", true},
		{"input focused", KeyContext{Editable: true}, "j", "", false},
		{"dialog consumes known shortcut", KeyContext{DialogOpen: true}, "j", "", true},
		{"editable dialog keeps typed question mark", KeyContext{Editable: true, DialogOpen: true}, "?", "", false},
		{"dialog leaves unrelated key", KeyContext{DialogOpen: true}, "z", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ActionFor(txnKeyBindings(), tt.key, tt.ctx)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("ActionFor(%q, %+v) = (%q, %t), want (%q, %t)", tt.key, tt.ctx, got, ok, tt.want, tt.ok)
			}
		})
	}
}
