// SPDX-License-Identifier: MIT

package i18n

var quarantineKeys = Catalog{
	"datasetQuarantine.message":       "We couldn't read your saved data. It is kept safe and nothing has been overwritten.",
	"datasetQuarantine.download":      "Download the unreadable copy",
	"datasetQuarantine.startFresh":    "Start fresh (keeps the unreadable copy)",
	"datasetQuarantine.hideForNow":    "Hide for now",
	"datasetQuarantine.started":       "A fresh, empty dataset is ready. Your unreadable copy is still kept safe.",
	"datasetQuarantine.confirm":       "Start with an empty dataset? Your unreadable copy will stay saved.",
	"datasetQuarantine.confirmButton": "Start fresh",
}

func init() {
	for k, v := range quarantineKeys {
		english[k] = v
	}
}
