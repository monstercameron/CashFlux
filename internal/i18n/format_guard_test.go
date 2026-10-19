// SPDX-License-Identifier: MIT

package i18n

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var formatVerb = regexp.MustCompile(`%(?:\[[0-9]+\])?[-+#0 ']*[0-9]*(?:\.[0-9]+)?(?:\[[0-9]+\])?([a-zA-Z%])`)

func TestBudgetSuggestionEvidenceFormatsAllVariants(t *testing.T) {
	bundle := DefaultBundle()
	cases := []struct {
		name string
		key  string
		args []any
		want string
	}{
		{"no unusual months", "sm12.reasonNoOutliers", []any{3}, "Based on a 3-month median."},
		{"one unusual month", "sm12.reasonOneOutlier", []any{3}, "3-month median, 1 unusual month left out."},
		{"multiple unusual months", "sm12.reasonManyOutliers", []any{3, 2}, "3-month median, 2 unusual months left out."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bundle.T(English, tc.key, tc.args...)
			if strings.Contains(got, "%!") {
				t.Fatalf("%s formatted with Go error: %q", tc.key, got)
			}
			if got != tc.want {
				t.Errorf("%s = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestEverySmartCatalogKeyFormatsWithRepresentativeArguments(t *testing.T) {
	bundle := DefaultBundle()
	for key, template := range bundle.Langs[English] {
		if !strings.HasPrefix(key, "smart.") {
			continue
		}
		t.Run(key, func(t *testing.T) {
			args := representativeFormatArgs(template)
			got := bundle.T(English, key, args...)
			if strings.Contains(got, "%!") {
				t.Fatalf("formatting %q with %d representative arguments returned %q", key, len(args), got)
			}
		})
	}
}

func TestBV7ISmartU1TitleFormatsItsFeatureArguments(t *testing.T) {
	bundle := DefaultBundle()
	got := bundle.T(English, "smart.u1.title", "Coffee shop", "$120.00")
	if strings.Contains(got, "%!") || got != "Coffee shop charge: $120.00" {
		t.Fatalf("smart.u1.title formatted incorrectly: %q", got)
	}
}

func representativeFormatArgs(template string) []any {
	var args []any
	for _, match := range formatVerb.FindAllStringSubmatch(template, -1) {
		verb := match[1][0]
		switch verb {
		case '%':
			continue
		case 'd', 'i', 'o', 'u', 'b', 'x', 'X', 'c', 'U':
			args = append(args, 7)
		case 'f', 'F', 'e', 'E', 'g', 'G':
			args = append(args, 7.5)
		case 't':
			args = append(args, true)
		default:
			args = append(args, fmt.Sprintf("value-%d", len(args)+1))
		}
	}
	return args
}
