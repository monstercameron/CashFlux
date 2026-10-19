// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestCloud1EnglishStateAndSafetyCopy(t *testing.T) {
	for _, key := range []string{
		"cloud1.off", "cloud1.settingUp", "cloud1.synced", "cloud1.paused", "cloud1.attention",
		"cloud1.step.where", "cloud1.step.signin", "cloud1.step.protect", "cloud1.step.first",
		"cloud1.recoveryLimit", "cloud1.deleteCaution", "cloud1.useServerConfirm",
	} {
		if strings.TrimSpace(english[key]) == "" {
			t.Errorf("missing Cloud state/safety text: %s", key)
		}
	}
	for key, value := range english {
		if strings.HasPrefix(key, "cloud1.") && strings.Contains(value, "readable JSON") {
			t.Errorf("Cloud offers an unprotected upload: %s", key)
		}
	}
}
