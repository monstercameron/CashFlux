// SPDX-License-Identifier: MIT

package i18n

// keyGateKeys holds English copy for the AI key-gate marker (R24).
var keyGateKeys = Catalog{
	"keygate.needsKey": "needs a key",
	// Shown on hover. It names the alternative rather than only the obstacle: a
	// control that says "unavailable" and stops is worse than one that says what
	// the app does instead.
	"keygate.needsKeyHint": "This one uses AI, so it needs your API key. Everything else on this screen works without one.",
}

func init() {
	for k, v := range keyGateKeys {
		english[k] = v
	}
}
