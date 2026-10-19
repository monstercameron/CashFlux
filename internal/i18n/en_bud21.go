// SPDX-License-Identifier: MIT

package i18n

// bud21Keys contains the accessible period-control name requested by BUD2.1.
var bud21Keys = Catalog{
	"bud21.periodPillName": "%s, choose period",
}

func init() {
	for key, value := range bud21Keys {
		english[key] = value
	}
}
