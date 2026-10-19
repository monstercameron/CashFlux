// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBC5B1SweepCopyIsAvailableAndDoesNotExposeRawErrors(t *testing.T) {
	for _, key := range []string{"sweep.errorSafe", "sweep.applyError", "sweep.capacityLimited", "sweep.ineligibleEnvelope", "sweep.poolRolloverOff"} {
		value := english[key]
		if strings.TrimSpace(value) == "" || strings.Contains(value, key) {
			t.Errorf("English[%q] = %q, want user-facing copy", key, value)
		}
	}
}
