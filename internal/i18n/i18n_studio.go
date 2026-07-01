// SPDX-License-Identifier: MIT

package i18n

// studioKeys holds the English strings for the /studio hub and its three panels
// (Build widget, Manage widgets, My pages). Registered at init time so they
// merge into the source catalog alongside the existing screen keys.
var studioKeys = Catalog{
	// Tab control
	"studio.tabsLabel":   "Studio section",
	"studio.tabDesign":   "Design",
	"studio.tabFormulas": "Formulas",
	"studio.tabFields":   "Custom fields",
	"studio.tabBuild":    "Build widget",
	"studio.tabManage":   "Manage widgets",
	"studio.tabPages":    "My pages",

	// My pages panel — create form
	"studio.pageName":   "Page name",
	"studio.createPage": "Create page",

	// My pages panel — row actions
	"studio.goToPage":       "Go to page",
	"studio.deletePage":     "Delete page",
	"studio.deletePageAria": "Delete page %s",

	// My pages panel — empty state
	"studio.pagesEmpty": "No custom pages yet. Use Create page above to build your first one.",
}

func init() {
	for k, v := range studioKeys {
		english[k] = v
	}
}
