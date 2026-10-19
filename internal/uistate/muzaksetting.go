// SPDX-License-Identifier: MIT

package uistate

// MuzakEnabledFromSetting reports whether the saved music setting explicitly
// enables playback. Missing and unrecognized values are disabled.
func MuzakEnabledFromSetting(value string) bool {
	return value == "1"
}

// MuzakSettingValue encodes a music choice for persistence.
func MuzakSettingValue(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}
