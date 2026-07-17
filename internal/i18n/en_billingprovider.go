// SPDX-License-Identifier: MIT

package i18n

// billingProviderKeys are the strings for the Cloud subscribe provider picker
// (Stripe vs PayPal, 2026-07-17). Kept separate from en.go (concurrent WIP) like
// the other feature key files.
var billingProviderKeys = Catalog{
	"settings.cloudPayWith":   "Pay with",
	"settings.cloudPayStripe": "Card (Stripe)",
	"settings.cloudPayPayPal": "PayPal",
}

func init() {
	for k, v := range billingProviderKeys {
		english[k] = v
	}
}
