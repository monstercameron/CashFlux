// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestImportSafetyCopyIsPresent(t *testing.T) {
	bundle := DefaultBundle()
	for _, key := range []string{
		"impc.aiDisclosure", "impc.fileTooLarge", "impc.backupTooLarge",
		"impc.tooManyRows", "impc.rowCounts", "impc.pageRange",
		"impc.previousPage", "impc.nextPage", "impc.duplicateCount",
		"impc.clearAllRows", "impc.importAllRows", "impc.suggestCategoriesAll",
		"impc.dateOrder", "impc.numberFormat", "impc.applyFormats",
	} {
		if got := bundle.T(English, key); got == "" || got == key {
			t.Errorf("English import copy %q = %q", key, got)
		}
	}
}
