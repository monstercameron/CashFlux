// SPDX-License-Identifier: MIT

package i18n

func init() {
	for key, value := range map[string]string{
		"splitEditor.left":        "Left to assign: %s",
		"splitEditor.ownerColumn": "Who",
		"splitEditor.newCategory": "New category",
		"splitEditor.cellLabel":   "%d categories",
		"splitEditor.r4Saved":     "Split saved",
	} {
		english[key] = value
	}
}
