// SPDX-License-Identifier: MIT

package i18n

// spotlightKeys holds English copy for the assistant's guided highlight (PS8).
var spotlightKeys = Catalog{
	// %s = what the highlighted control does.
	"spotlight.pointing": "Highlighted: %s",
}

func init() {
	for k, v := range spotlightKeys {
		english[k] = v
	}
}
