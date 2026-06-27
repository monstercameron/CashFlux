// SPDX-License-Identifier: MIT

package i18n

// sampleBannerKeys are the compact sample-mode chip strings (R41), kept separate
// from en.go (concurrent WIP) like the other feature key files.
var sampleBannerKeys = Catalog{
	// Short status label for the compact chip (replaces the long full-width
	// banner sentence). The full explanation moves to the chip's title tooltip.
	"sample.chipLabel": "Sample data",
	"sample.chipTitle": "You're exploring with sample data — start fresh whenever you're ready.",
}

func init() {
	for k, v := range sampleBannerKeys {
		english[k] = v
	}
}
