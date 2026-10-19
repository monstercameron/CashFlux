// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r4k.split":       "Split…",
		"r4k.linkChooser": "Link to…",
		"r4k.editActions": "More actions",
	} {
		english[key] = value
	}
}
