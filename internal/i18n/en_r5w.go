// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r5w.suggestedCategory":    "%s, suggested %s",
		"r5w.suggestCategorySmart": "Suggest a category (Smart)",
	} {
		english[key] = value
	}
}
