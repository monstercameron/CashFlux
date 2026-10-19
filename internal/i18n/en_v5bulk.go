// SPDX-License-Identifier: MIT

package i18n

// v5BulkKeys explains why the bulk owner action is unavailable before an
// owner has been selected.
var v5BulkKeys = Catalog{
	"transactions.bulkOwnerApplyHint": "Choose at least one owner to enable Apply.",
}

func init() {
	for key, value := range v5BulkKeys {
		english[key] = value
	}
}
