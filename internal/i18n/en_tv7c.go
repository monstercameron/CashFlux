// SPDX-License-Identifier: MIT

package i18n

var tv7cKeys = Catalog{
	"transactions.calAccountDueAria": "%s is due on this day for %s.",
}

func init() {
	for key, value := range tv7cKeys {
		english[key] = value
	}
}
