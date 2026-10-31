// SPDX-License-Identifier: MIT

package app

import "testing"

func TestDefaultSettingsTabRedirectRespectsLatestNavigation(t *testing.T) {
	for _, tc := range []struct {
		name, rawTab, latest string
		wantRedirect         bool
	}{
		{name: "bare settings", latest: "/settings", wantRedirect: true},
		{name: "trailing slash", latest: "/settings/", wantRedirect: true},
		{name: "left for help", latest: "/help"},
		{name: "left for about", latest: "/about"},
		{name: "left for subscriptions", latest: "/subscriptions"},
		{name: "selected another tab", latest: "/settings/cloud"},
		{name: "existing tab", rawTab: "cloud", latest: "/settings/cloud"},
		{name: "unavailable location", latest: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/settings"
			var destinations []string
			// Register while Settings is current, then move before the passive
			// effect runs. This pins the ordering behind the browser regression.
			effect := defaultSettingsTabRedirect(tc.rawTab, func() string { return path }, func(target string) {
				destinations = append(destinations, target)
			})
			path = tc.latest
			if cleanup := effect(); cleanup != nil {
				t.Fatal("redirect effect unexpectedly retained cleanup work")
			}
			if tc.wantRedirect {
				if len(destinations) != 1 || destinations[0] != "/settings/household" {
					t.Fatalf("destinations = %v, want one default-tab navigation", destinations)
				}
			} else if len(destinations) != 0 {
				t.Fatalf("stale Settings effect replaced %q with %v", path, destinations)
			}
		})
	}
}
