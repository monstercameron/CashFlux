// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"uxt4.review":            "Review",
		"uxt4.keyboardShortcuts": "Keyboard shortcuts",
		"uxt4.deleteOne":         "Delete %d transaction?",
		"uxt4.deleteMany":        "Delete %d transactions?",
		"uxt4.delete":            "Delete",
	} {
		english[key] = value
	}
}
