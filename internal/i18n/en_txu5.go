// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"txu5.range":         "%s–%s of %s",
		"txu5.member":        "Member: %s",
		"txu5.reviewCallout": "%s need review — Review now",
	} {
		english[key] = value
	}
}
