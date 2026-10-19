// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"bc4b42.raise":          "Raise",
		"bc4b42.lower":          "Lower",
		"bc4b42.keep":           "Keep",
		"bc4b42.recommendation": "%s the limit to %s, based on the %s.",
		"bc4b42.window":         "median of the last %d %s",
		"bc4b42.oneOutlier":     "1 unusual period left out.",
		"bc4b42.manyOutliers":   "%d unusual periods left out.",
	} {
		english[key] = value
	}
}
