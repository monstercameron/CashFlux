// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range (Catalog{
		"integrity.unavailableAccount":     "an unavailable account",
		"integrity.reservationUnavailable": "We can’t check the goal reservations for %s. Review its balance, goal amounts, and exchange rates.",
		"integrity.reservationReview":      "Review reservations for %s",
	}) {
		english[key] = value
	}
}
