// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"r4c.who":                  "Who",
		"r4c.unassigned":           "No one assigned",
		"r4c.review":               "Review",
		"r4c.transferTo":           "Transfer → %s",
		"r4c.transferFrom":         "Transfer ← %s",
		"r4c.transferCounterparty": "Transfer with %s",
	} {
		english[key] = value
	}
}
