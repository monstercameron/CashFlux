// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/uistate"
)

func TestIMV6UndoableImportToastGetsTenSecondsOfReadingTime(t *testing.T) {
	for _, tc := range []struct {
		name   string
		notice uistate.Notice
		want   int
	}{
		{name: "import undo", notice: uistate.Notice{}.WithDurable("Imported 3 transactions", true), want: 10000},
		{name: "ordinary notice", notice: uistate.Notice{}.WithTransient("Ready"), want: 8000},
		{name: "error", notice: uistate.Notice{Text: "Could not save", Err: true}, want: 8000},
		{name: "non-dataset undo flag", notice: uistate.Notice{Text: "Info", Undoable: true, NonDataset: true}, want: 8000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := toastTimeoutFor(tc.notice); got != tc.want {
				t.Fatalf("toast timeout = %dms, want %dms", got, tc.want)
			}
		})
	}
}
