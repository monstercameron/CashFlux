// SPDX-License-Identifier: MIT

package styles

import (
	"os"
	"strings"
	"testing"
)

func TestTV8JLightFallbackTokensAreSelfSufficient(t *testing.T) {
	resetSheet()
	registerGenerated()
	css := Build()
	start := strings.Index(css, `[data-theme="light"]{`)
	if start < 0 {
		t.Fatal("light theme token rule missing")
	}
	end := strings.Index(css[start:], "}")
	rule := css[start : start+end+1]
	for _, want := range []string{"--down:#b3322f", "--accent-ink:#102318", "--primary-button-ink:#102318"} {
		if !strings.Contains(rule, want) {
			t.Errorf("light theme block missing self-sufficient token %q: %s", want, rule)
		}
	}
}

func TestTV8JBootSkeletonPreviewsLedgerRows(t *testing.T) {
	markup, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatalf("read boot markup: %v", err)
	}
	page := string(markup)
	for _, want := range []string{".sk-ledger-row", "class=\"sk-block sk-ledger-row sk-ledger\""} {
		if !strings.Contains(page, want) {
			t.Errorf("boot skeleton missing ledger preview %q", want)
		}
	}
	if strings.Contains(page, "<div class=\"sk-block sk-hero\"></div>") || strings.Contains(page, "<div class=\"sk-grid\">") {
		t.Fatal("boot skeleton still presents dashboard hero and tile grid")
	}
}

func TestTV8JRailSectionsUseReadableSentenceCaseTypography(t *testing.T) {
	resetSheet()
	registerTV8J()
	css := Build()
	for _, want := range []string{".rail-section{", "text-transform:none", "letter-spacing:normal", "font-size:var(--type-12)", "font-weight:500"} {
		if !strings.Contains(css, want) {
			t.Errorf("rail section rule missing %q: %s", want, css)
		}
	}
}
