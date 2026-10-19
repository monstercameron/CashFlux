// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestUXT4EnglishTransactionLabels(t *testing.T) {
	for _, tt := range []struct {
		key  string
		args []any
		want string
	}{
		{key: "uxt4.review", want: "Review"},
		{key: "uxt4.keyboardShortcuts", want: "Keyboard shortcuts"},
		{key: "uxt4.deleteOne", args: []any{1}, want: "Delete 1 transaction?"},
		{key: "uxt4.deleteMany", args: []any{3}, want: "Delete 3 transactions?"},
		{key: "uxt4.delete", want: "Delete"},
	} {
		if got := DefaultBundle().T(English, tt.key, tt.args...); got != tt.want {
			t.Errorf("English %q = %q, want %q", tt.key, got, tt.want)
		}
	}
}
