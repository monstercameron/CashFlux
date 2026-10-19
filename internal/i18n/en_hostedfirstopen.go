// SPDX-License-Identifier: MIT

package i18n

var hostedFirstOpenKeys = Catalog{
	"hostedFirstOpen.emptyTitle":   "This account is ready for a budget",
	"hostedFirstOpen.emptyIntro":   "There is no saved budget on this server yet. Start a new budget or bring in a backup.",
	"hostedFirstOpen.startBudget":  "Start your budget",
	"hostedFirstOpen.importBackup": "Import a backup",
	"hostedFirstOpen.checkFailed":  "CashFlux couldn't check this account's saved data. Check your connection and try again.",
}

func init() {
	for key, value := range hostedFirstOpenKeys {
		english[key] = value
	}
}
