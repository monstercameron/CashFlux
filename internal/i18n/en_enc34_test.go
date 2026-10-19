// SPDX-License-Identifier: MIT

package i18n

import "testing"

func TestCheckpointFailureNoticeIsPlainEnglish(t *testing.T) {
	if got, want := DefaultBundle().T(English, "enc34.checkpointFailed"), "Couldn't save a restore point, so nothing was changed. Try again."; got != want {
		t.Fatalf("checkpoint failure notice = %q, want %q", got, want)
	}
}
