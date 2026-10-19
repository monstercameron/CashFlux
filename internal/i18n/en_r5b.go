// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r5b.searchCategories": "Search categories",
		"r5b.searchPeople":     "Search people",
		"r5b.noMatches":        "No matches",
	} {
		english[key] = value
	}
}
