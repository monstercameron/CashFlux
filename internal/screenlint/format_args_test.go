// SPDX-License-Identifier: MIT

package screenlint

import (
	"strings"
	"testing"
)

func TestUistateTranslationArgumentCounts(t *testing.T) {
	offenders, err := FindTranslationArgCountOffenders(screensRoot())
	if err != nil {
		t.Fatalf("scan screens: %v", err)
	}
	var owned []FormatArgOffender
	for _, offender := range offenders {
		if strings.HasPrefix(offender.File, "budget") || strings.HasPrefix(offender.File, "txn") || strings.HasPrefix(offender.File, "transaction") {
			owned = append(owned, offender)
		}
	}
	if len(owned) > 0 {
		t.Errorf("owned screen translation argument mismatches: %+v", owned)
	}
	for _, offender := range offenders {
		t.Logf("out of ticket scope: %s:%d uistate.T(%q) has %d args; template has %d verbs", offender.File, offender.Line, offender.Key, offender.Got, offender.Want)
	}
}

func TestFormatVerbCountIgnoresEscapedPercentAndCountsPositionalVerbs(t *testing.T) {
	if got := countFormatVerbs("%s is %d%% of %[3]v"); got != 3 {
		t.Fatalf("countFormatVerbs = %d, want 3", got)
	}
	if got := countFormatVerbs("50% needs / 30% wants / 20% savings"); got != 0 {
		t.Fatalf("literal percentage copy counted as %d format verbs, want 0", got)
	}
}
