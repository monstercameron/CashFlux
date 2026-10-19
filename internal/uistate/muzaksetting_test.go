// SPDX-License-Identifier: MIT

package uistate

import "testing"

func TestMuzakEnabledFromSetting(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "missing", value: "", want: false},
		{name: "explicit off", value: "0", want: false},
		{name: "explicit on", value: "1", want: true},
		{name: "unknown", value: "yes", want: false},
		{name: "unknown numeric", value: "2", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MuzakEnabledFromSetting(tt.value); got != tt.want {
				t.Errorf("MuzakEnabledFromSetting(%q) = %t, want %t", tt.value, got, tt.want)
			}
		})
	}
}

func TestMuzakSettingRoundTrip(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		value := MuzakSettingValue(enabled)
		if got := MuzakEnabledFromSetting(value); got != enabled {
			t.Errorf("setting round trip for %t: value %q decoded as %t", enabled, value, got)
		}
	}
}
