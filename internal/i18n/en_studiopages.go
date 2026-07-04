// SPDX-License-Identifier: MIT

package i18n

// studioPagesKeys holds the English strings for the redesigned My-pages
// surface (the Studio tab): the page registry and its composer rail. Merged
// via init so this file does not touch en.go.
var studioPagesKeys = Catalog{
	"spg.eyebrow":         "Studio",
	"spg.title":           "Your pages",
	"spg.lede":            "Blank canvases with their own address in your navigation — fill them with the widgets you design and build here.",
	"spg.registryKicker":  "Page registry",
	"spg.countNone":       "No pages yet",
	"spg.countOne":        "1 page",
	"spg.countMany":       "%d pages",
	"spg.empty":           "Nothing here yet — name a page on the right and it appears in your navigation.",
	"spg.compTitle":       "Create a page",
	"spg.compLede":        "Name it and it shows up in your navigation, ready for widgets.",
	"spg.namePlaceholder": "e.g. Side hustle",
	"spg.footTitle":       "What you'll get",
	"spg.livesAt":         "It will live at",
	"spg.footHint":        "Starts empty — add widgets right on the page, or publish them from Design and Build.",
	"spg.widgetsNone":     "Empty",
	"spg.widgetsOne":      "1 widget",
	"spg.widgetsMany":     "%d widgets",
	"spg.open":            "Open →",
	"spg.deleteWarn":      "Delete this page? Its widgets and layout go with it.",
	"spg.deleteYes":       "Delete page",
}

func init() {
	for k, v := range studioPagesKeys {
		english[k] = v
	}
}
