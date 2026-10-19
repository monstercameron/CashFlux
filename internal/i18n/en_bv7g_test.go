// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestBV7GDialogCopyUsesNumberedStepsAndOneCoverAction(t *testing.T) {
	bundle := DefaultBundle()
	for i, key := range []string{
		"bv7g.monthclose.step1", "bv7g.monthclose.step2", "bv7g.monthclose.step3",
		"bv7g.monthclose.step4", "bv7g.monthclose.step5",
	} {
		got := bundle.T(English, key)
		if !strings.HasPrefix(got, string(rune('1'+i))+".") {
			t.Errorf("%s = %q, want step number %d", key, got, i+1)
		}
		if strings.Contains(got, "·") || strings.Contains(strings.ToUpper(got), "LAST MONTH") {
			t.Errorf("%s contains disallowed dialog copy %q", key, got)
		}
	}
	for _, key := range []string{"bv7g.cover.action"} {
		if got := bundle.T(English, key); got != "Cover from another budget" {
			t.Errorf("%s = %q, want the shared action name", key, got)
		}
	}
	if got := bundle.T(English, "bv7g.adjust.everyHint"); got != "Updates every budget period, including past periods." {
		t.Fatalf("permanent scope disclosure changed unexpectedly: %q", got)
	}
}
