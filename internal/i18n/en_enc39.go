// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, text := range (Catalog{
		"enc39.revokeFailed": "Signed out on this device. We couldn't reach the server to end the session there — it will expire on its own.",
	}) {
		english[key] = text
	}
}
