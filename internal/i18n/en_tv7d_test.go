// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestTV7DReviewCopyUsesPlainCommaMeta(t *testing.T) {
	for _, tc := range []struct {
		key, want string
	}{
		{key: "review.tierCountPlain", want: "4 merchants, 175 charges"},
		{key: "review.selectionCountPlain", want: "4 merchants, 175 charges"},
		{key: "review.undoAvailable", want: "Undo stays available"},
		{key: "review.keyboardHintBulkPlain", want: "j / k moves, Space selects, Enter marks reviewed, 1 for one at a time"},
		{key: "review.keyboardHintSinglePlain", want: "j / k moves, Enter marks reviewed, s snoozes, d dismisses, b for several at once"},
		{key: "transactions.newCategoryOption", want: "New category…"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			got := english[tc.key]
			if strings.Contains(got, "%s") {
				got = fmt.Sprintf(got, "4 merchants", "175 charges")
			}
			if got != tc.want {
				t.Fatalf("%s = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}
