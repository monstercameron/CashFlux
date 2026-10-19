// SPDX-License-Identifier: MIT

package i18n

// tv7e1Keys contains concise transaction copy for the TV7E.1 cleanup.
var tv7e1Keys = Catalog{
	"tv7e1.upcomingOne":  "%s due %s",
	"tv7e1.upcomingMany": "%d bills due by %s, %s total",
}

func init() {
	for key, value := range tv7e1Keys {
		english[key] = value
	}
}
