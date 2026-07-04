// SPDX-License-Identifier: MIT

package i18n

// studioHubKeys holds the English strings for the Studio hub's Formulas and
// Custom-fields tab mastheads, so every tab opens with the same eyebrow +
// serif title + lede language. Merged via init so this file does not touch
// en.go.
var studioHubKeys = Catalog{
	"stuh.formulasTitle": "Formulas",
	"stuh.formulasLede":  "Every figure is a formula over atoms. Test expressions in the workbench, then shape the compound variables every page and widget reads.",
	"stuh.fieldsTitle":   "Custom fields",
	"stuh.fieldsLede":    "Your data's shape — give any entity its own fields, defined once and used everywhere.",
}

func init() {
	for k, v := range studioHubKeys {
		english[k] = v
	}
}
