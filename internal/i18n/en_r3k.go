// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r3k.linkTo": "Link to…",
	} {
		english[key] = value
	}
}
