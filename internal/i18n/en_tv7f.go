// SPDX-License-Identifier: MIT

package i18n

// tv7fKeys names the transaction bulk toolbar for assistive technology.
var tv7fKeys = Catalog{
	"transactions.bulkToolbarLabel": "%d selected",
	"transactions.bulkSearchOwners": "Search owners",
}

func init() {
	for key, value := range tv7fKeys {
		english[key] = value
	}
}
