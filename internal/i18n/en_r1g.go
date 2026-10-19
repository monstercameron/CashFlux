// SPDX-License-Identifier: MIT

package i18n

// r1gKeys contains the concise labels used by the review dialog polish pass.
var r1gKeys = Catalog{
	"review.connectAI":   "Set up Smart+ in Settings",
	"review.openCharges": "Open charges from %s",
}

func init() {
	for key, value := range r1gKeys {
		english[key] = value
	}
}
