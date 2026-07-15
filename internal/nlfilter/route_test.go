// SPDX-License-Identifier: MIT

package nlfilter

import "testing"

func TestDecide(t *testing.T) {
	tests := []struct {
		name                      string
		freeOK, aiEnabled, hasKey bool
		want                      Route
	}{
		{"local parse wins", true, true, true, RouteLocal},
		{"local parse wins even with ai off", true, false, false, RouteLocal},
		{"no structure, ai off -> plain text", false, false, false, RoutePlainText},
		{"ai on but no key -> needs key", false, true, false, RouteNeedsKey},
		{"ai on with key -> ai", false, true, true, RouteAI},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.freeOK, tc.aiEnabled, tc.hasKey); got != tc.want {
				t.Errorf("Decide(%v,%v,%v) = %v, want %v", tc.freeOK, tc.aiEnabled, tc.hasKey, got, tc.want)
			}
		})
	}
}
