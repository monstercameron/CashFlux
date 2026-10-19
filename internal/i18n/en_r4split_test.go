// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestR4SplitEnglishCopy(t *testing.T) {
	bundle := Bundle{Default: English, Langs: map[Lang]Catalog{English: english}}
	for key, want := range map[string]string{
		"splitEditor.left":        "Left to assign: $64.00",
		"splitEditor.ownerColumn": "Who",
		"splitEditor.newCategory": "New category",
		"splitEditor.cellLabel":   "Split · 2 categories",
		"splitEditor.r4Saved":     "Split saved",
	} {
		got := english[key]
		if key == "splitEditor.left" {
			got = bundle.T(English, key, "$64.00")
		}
		if key == "splitEditor.cellLabel" {
			got = bundle.T(English, key, 2)
		}
		if got != want {
			t.Errorf("English[%q] = %q, want %q", key, got, want)
		}
	}
}
