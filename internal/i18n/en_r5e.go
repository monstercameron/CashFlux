// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r5e.moreActions":       "More actions",
		"r5e.suggestedCategory": "Suggested: %s · Use",
	} {
		english[key] = value
	}
}
