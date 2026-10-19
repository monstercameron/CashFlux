// SPDX-License-Identifier: MIT

package i18n

var swUpdateKeys = Catalog{
	"swupdate.updateReady":    "An update is ready",
	"swupdate.updateMessage":  "A newer version of CashFlux is ready. Reload to start using it.",
	"swupdate.staleMessage":   "This tab is running an older version of CashFlux. Reload before you keep working, so your changes are saved with the current version.",
	"swupdate.reload":         "Reload",
	"swupdate.later":          "Later",
	"swupdate.reloadAria":     "Reload CashFlux to use the update",
	"swupdate.dismissAria":    "Hide the update message for now",
	"swupdate.applyingUpdate": "Applying the update…",
}

func init() {
	for k, v := range swUpdateKeys {
		english[k] = v
	}
}
