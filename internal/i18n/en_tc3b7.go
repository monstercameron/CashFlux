// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"documents.preflightFailed":    "%s need attention before import: %s",
		"documents.preflightFailedOne": "1 row needs attention before import: %s",
		"documents.wizardNoRows":       "No rows could be mapped. Check the column choices. %s",
		"documents.ambiguousDates":     "%d dates could mean month/day or day/month; they were read as month/day.",
	} {
		english[key] = value
	}
}
