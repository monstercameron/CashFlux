// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestTC2EUncomparableTransactionCopy(t *testing.T) {
	if got := DefaultBundle().T(English, "tc2e.rowsCannotCompare", 12, "EUR"); got != "12 rows can't be compared (no EUR rate)" {
		t.Fatalf("conversion issue copy = %q", got)
	}
}
