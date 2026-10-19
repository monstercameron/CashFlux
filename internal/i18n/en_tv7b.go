// SPDX-License-Identifier: MIT

package i18n

var tv7bKeys = Catalog{
	"tv7b.due":              "Due %s",
	"tv7b.less":             "Show fewer",
	"tv7b.scheduled":        "%s scheduled",
	"tv7b.futureAccessible": "%s posted. %s scheduled. Due %s.",
}

func init() {
	for key, value := range tv7bKeys {
		english[key] = value
	}
}
