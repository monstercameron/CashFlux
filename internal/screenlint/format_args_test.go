// SPDX-License-Identifier: MIT

package screenlint

import (
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/i18n"
)

func TestUistateTranslationArgumentCounts(t *testing.T) {
	offenders, err := FindTranslationArgCountOffenders(screensRoot())
	if err != nil {
		t.Fatalf("scan screens: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("screen translation argument mismatches: %+v", offenders)
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

func TestOwnedScreenTranslationsRenderWithoutFormattingErrors(t *testing.T) {
	catalog := i18n.DefaultBundle().Langs[i18n.English]
	cases := map[string][]struct {
		key  string
		args []any
	}{
		"credit": {
			{key: "credit.balanceOfLimit", args: []any{"$500", "$1,000"}},
			{key: "credit.aggUtil", args: []any{"50%"}},
			{key: "credit.missingLimitNote", args: []any{2}},
			{key: "credit.utilTarget"},
			{key: "credit.f.util.curve"},
		},
		"duplicates": {
			{key: "duplicates.groupCount", args: []any{3}},
			{key: "duplicates.deleteAria", args: []any{"transaction-1"}},
			{key: "duplicates.headline", args: []any{4, 2}},
		},
		"investments": {
			{key: "investments.deleteHoldingAria", args: []any{"Index fund"}},
			{key: "investments.addedFlash", args: []any{"Index fund"}},
			{key: "investments.costBasisLabel", args: []any{"$"}},
			{key: "investments.costBasisPlaceholder", args: []any{"$"}},
			{key: "investments.priceLabel", args: []any{"$"}},
			{key: "investments.pricePlaceholder", args: []any{"$"}},
		},
		"loans": {
			{key: "loans.extraPlaceholder", args: []any{"$"}},
		},
		"setup": {
			{key: "setup.acctAlreadyHave", args: []any{2}},
			{key: "setup.membersAlready", args: []any{3}},
		},
		"smart": {
			{key: "smart.stripMore", args: []any{2}},
			{key: "smart.peekAlertsAria", args: []any{"9+"}},
		},
	}
	for page, translations := range cases {
		t.Run(page, func(t *testing.T) {
			for _, tc := range translations {
				template, ok := catalog[tc.key]
				if !ok {
					t.Fatalf("missing English translation %q", tc.key)
				}
				got := template
				if len(tc.args) > 0 {
					got = fmt.Sprintf(template, tc.args...)
				}
				if strings.Contains(got, "%!") {
					t.Errorf("%s rendered formatting error: %q", tc.key, got)
				}
			}
		})
	}
}
