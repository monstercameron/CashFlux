// SPDX-License-Identifier: MIT

package i18n

// studioBuildKeys holds the English strings for the redesigned Build-widget
// chrome (the node-canvas builder's masthead, command bar groups, and preview
// head). Merged via init so this file does not touch en.go.
var studioBuildKeys = Catalog{
	"vbld.title":       "Build a widget",
	"vbld.lede":        "Wire data into a visual on the node canvas — the live preview updates as you build.",
	"vbld.startFrom":   "Start from",
	"vbld.thisCard":    "This card",
	"vbld.canvas":      "Canvas",
	"vbld.deleteTitle": "Delete this card from My cards",
	"vbld.previewHint": "Rendered by the real engine at its true dashboard size",
}

func init() {
	for k, v := range studioBuildKeys {
		english[k] = v
	}
}
