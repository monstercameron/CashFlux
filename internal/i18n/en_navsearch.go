// SPDX-License-Identifier: MIT

package i18n

// Strings for the rail's destination filter. The box exists because the rail
// holds around thirty destinations across four groups, most behind collapsed
// headers — so finding one meant remembering which section it lived in.
func init() {
	for k, v := range map[string]string{
		"railsearch.label":       "Filter the menu (Alt+M)",
		"railsearch.placeholder": "Jump to…",
		// The count is a live region, so it is phrased as a statement a screen
		// reader can read on its own rather than as a fragment beside the box.
		"railsearch.countOne":  "%d destination matches",
		"railsearch.countMany": "%d destinations match",
		"railsearch.noMatch":   "Nothing in the menu matches “%s”.",
		"railsearch.clear":     "Clear the filter",
		// A force push that refuses says why. "Make my copy win" doing nothing at
		// all is indistinguishable from a broken button.
		"sync.forceRefused": "This device's copy was not sent: %s.",
		// Named for what it does rather than what it is: the hint sits under an
		// empty result and tells you the one thing that will help.
		"railsearch.hintShorter": "Try a shorter word.",
	} {
		english[k] = v
	}
}
