// SPDX-License-Identifier: MIT

package i18n

// assistantControlsKeys holds the English strings for the redesigned assistant
// control cell — the full-width toolbar above the chat that groups the model /
// thinking / privacy selectors and the New chat / Edit prompt actions. Merged via
// init so this file does not touch en.go.
var assistantControlsKeys = Catalog{
	// Accessible name for the control cell as a labelled group of settings.
	"assistant.controlsLabel": "Assistant settings",
}

func init() {
	for k, v := range assistantControlsKeys {
		english[k] = v
	}
}
