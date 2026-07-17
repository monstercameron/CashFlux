// SPDX-License-Identifier: MIT

package i18n

// lane3NavKeys holds copy for the redesigned five-slot mobile bottom bar and
// its More sheet (UX-01 dual-navigation fix). Merged via init so this file
// does not touch en.go.
var lane3NavKeys = Catalog{
	"nav.mobileHome":      "Home",
	"nav.mobileMore":      "More",
	"nav.mobileMoreSheet": "More destinations",
}

func init() {
	for k, v := range lane3NavKeys {
		english[k] = v
	}
}
