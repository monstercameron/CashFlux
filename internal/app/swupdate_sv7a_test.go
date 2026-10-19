// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"syscall/js"
	"testing"
)

func TestParseBuildMessage(t *testing.T) {
	tests := []struct {
		name      string
		data      js.Value
		wantType  string
		wantBuild string
	}{
		{
			name:      "object announcement",
			data:      js.ValueOf(map[string]any{"type": "update-ready", "buildId": "abc-1"}),
			wantType:  "update-ready",
			wantBuild: "abc-1",
		},
		{
			name:     "legacy string message",
			data:     js.ValueOf("SKIP_WAITING"),
			wantType: "SKIP_WAITING",
		},
		{
			name: "missing fields",
			data: js.ValueOf(map[string]any{"type": 3, "buildId": false}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotBuild := parseBuildMessage(tt.data)
			if gotType != tt.wantType || gotBuild != tt.wantBuild {
				t.Fatalf("parseBuildMessage() = (%q, %q), want (%q, %q)", gotType, gotBuild, tt.wantType, tt.wantBuild)
			}
		})
	}
}
