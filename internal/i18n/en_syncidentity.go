// SPDX-License-Identifier: MIT

package i18n

var syncIdentityKeys = Catalog{
	"sync.identityUnknown": "Can't confirm which account this is, so sync is paused. Your changes are kept on this device.",
	"sync.localReplacementFailed": "CashFlux could not save your server data on this device. Reload this page to retry.",
}

func init() {
	for k, v := range syncIdentityKeys {
		english[k] = v
	}
}
