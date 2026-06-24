// SPDX-License-Identifier: MIT

package i18n

// a11yKeys is the accessibility-sweep set of English strings registered into
// the source catalog at init time — kept separate from en.go (user WIP) so
// a11y changes do not land in the user's working tree. Uses the same
// init()-merge pattern as en_enterprise.go and en_home.go.
var a11yKeys = Catalog{
	// Notification Center — aria-label for the "Clear all" button, giving
	// screen readers more context than the visible two-word label alone.
	"notifications.clearAllAria": "Clear all notifications",
}

func init() {
	for k, v := range a11yKeys {
		english[k] = v
	}
}
