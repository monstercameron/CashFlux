// SPDX-License-Identifier: MIT

package backendauth

import "testing"

func TestCompatibleVersion(t *testing.T) {
	tests := []struct {
		name, server, minimum string
		want                  bool
	}{
		{"current server supports v1 client", "2", "1", true},
		{"legacy prefixed response", "v1", "v1", true},
		{"newer server retains v1", "3", "1", true},
		{"server requires v2 client", "2", "2", false},
		{"server too old", "0", "1", false},
		{"missing minimum", "2", "", false},
		{"malformed version", "latest", "1", false},
		{"inverted range", "1", "2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompatibleVersion(tt.server, tt.minimum); got != tt.want {
				t.Fatalf("CompatibleVersion(%q, %q) = %t, want %t", tt.server, tt.minimum, got, tt.want)
			}
		})
	}
}
