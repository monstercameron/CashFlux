// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestINT1SyncAndBulkDeleteCopy(t *testing.T) {
	bundle := Bundle{Default: English, Langs: map[Lang]Catalog{English: english}}
	cases := map[string]string{
		"sync.lockedDifferentPasscode":    "The server copy uses a different passcode",
		"sync.identityUnknownLabel":       "Account not confirmed",
		"sync.replaceServerCopyAction":    "Replace the server copy",
		"transactions.bulkDeleteOne":      "1 transaction was deleted.",
		"transactions.bulkDeleteMany":     "2 transactions were deleted.",
		"transactions.bulkNotDeletedOne":  "1 transaction was not deleted.",
		"transactions.bulkNotDeletedMany": "2 transactions were not deleted.",
	}
	for key, want := range cases {
		got := english[key]
		if key == "transactions.bulkDeleteOne" || key == "transactions.bulkNotDeletedOne" {
			got = bundle.T(English, key, 1)
		} else if key == "transactions.bulkDeleteMany" || key == "transactions.bulkNotDeletedMany" {
			got = bundle.T(English, key, 2)
		}
		if got != want {
			t.Errorf("English %q = %q, want %q", key, got, want)
		}
	}
}
