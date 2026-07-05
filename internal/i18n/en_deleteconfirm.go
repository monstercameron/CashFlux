// SPDX-License-Identifier: MIT

package i18n

// deleteConfirmKeys holds confirm-dialog copy for destructive actions that were
// previously instant (v1.0 polish: no saved user artifact should vanish on a
// single misclick). Merged via init so this file does not touch en.go.
var deleteConfirmKeys = Catalog{
	"planning.thisScenario":         "this scenario",
	"planning.deleteConfirm":        "Delete the scenario \"%s\"? This can't be undone.",
	"allocate.thisProfile":          "this profile",
	"allocate.deleteProfileConfirm": "Delete the allocation profile \"%s\"? This can't be undone.",
}

func init() {
	for k, v := range deleteConfirmKeys {
		english[k] = v
	}
}
