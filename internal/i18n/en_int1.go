// SPDX-License-Identifier: MIT

package i18n

var int1Keys = Catalog{
	"sync.lockedDifferentPasscode":    "The server copy uses a different passcode",
	"sync.identityUnknownLabel":       "Account not confirmed",
	"sync.replaceServerCopyAction":    "Replace the server copy",
	"transactions.bulkDeleteOne":      "%d transaction was deleted.",
	"transactions.bulkDeleteMany":     "%d transactions were deleted.",
	"transactions.bulkNotDeletedOne":  "%d transaction was not deleted.",
	"transactions.bulkNotDeletedMany": "%d transactions were not deleted.",
}

func init() {
	for key, value := range int1Keys {
		english[key] = value
	}
}
