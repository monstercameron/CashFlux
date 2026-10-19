// SPDX-License-Identifier: MIT

package i18n

var r6fKeys = Catalog{
	"r6f.weekSpent":     "Spent %s",
	"r6f.weekNet":       "Net %s",
	"r6f.weekScheduled": "%d scheduled",
}

func init() {
	for key, value := range r6fKeys {
		english[key] = value
	}
}
