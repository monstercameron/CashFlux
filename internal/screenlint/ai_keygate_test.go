// SPDX-License-Identifier: MIT

package screenlint

import (
	"strings"
	"testing"
)

// TestAIGatedSurfacesSaySoBeforeTheClick is the R24 ratchet.
//
// Every AI-gated control in the app already refused politely when pressed without
// a key. That is the wrong moment: the person has decided to use the feature,
// clicked, and only then been told it is unavailable — which reads as the app
// changing its mind rather than as a prerequisite they could have seen.
//
// A lint rather than a one-off audit, because the failure mode is drift: the audit
// was done once before and the surfaces added afterwards quietly reintroduced it.
// A file that refuses on the key must also mark its control as key-gated, or say
// in its own words why it does not need to.
func TestAIGatedSurfacesSaySoBeforeTheClick(t *testing.T) {
	// The marker a surface uses to say it announces its own gate. Either the
	// shared component, or its own copy key naming the requirement up front.
	markers := []string{"KeyGateMark(", "needsKey", "NeedsKey", "needKey"}

	for rel, text := range readInternal(t) {
		if !strings.HasPrefix(rel, "screens/") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		// A file that never refuses on a missing key has no gate to announce.
		if !strings.Contains(text, `OpenAIKey == ""`) {
			continue
		}
		// The assistant itself is exempt: the WHOLE surface is the AI feature, and
		// it shows the full key explainer rather than a per-control mark.
		if strings.Contains(text, "assistant.keyCallout") || strings.Contains(text, "KeyExplainer") {
			continue
		}
		marked := false
		for _, m := range markers {
			if strings.Contains(text, m) {
				marked = true
				break
			}
		}
		if !marked {
			t.Errorf("%s refuses without an API key but never says so before the click — "+
				"add KeyGateMark(AIKeyConfigured()) to the control, or a needsKey line explaining the gate up front", rel)
		}
	}
}

// TestTheKeyGateIsCheckedTheSameWayEverywhere guards the other half of the R24
// audit: a surface that checks only the direct key, and forgets the backend proxy,
// tells a household on the shared server key that a working feature is unavailable.
//
// The two-part condition was written out by hand at every site. One of them
// checking half of it is the sort of bug nobody finds by reading, so any NEW site
// has to go through the shared helper.
func TestTheKeyGateIsCheckedTheSameWayEverywhere(t *testing.T) {
	// The surfaces that predate the shared helper and still spell the condition
	// out. Each has been read and checks BOTH halves. The list may shrink as they
	// migrate; it must not grow.
	grandfathered := map[string]bool{
		"screens/allocate.go":             true,
		"screens/bills_smart.go":          true,
		"screens/documents.go":            true,
		"screens/insights.go":             true,
		"screens/receipt_split_flow.go":   true,
		"screens/recurring_review.go":     true,
		"screens/smartai.go":              true,
		"screens/dashboard_hero.go":       true,
		"screens/smart_adapter.go":        true,
		"screens/chat_agent_reference.go": true,
	}
	for rel, text := range readInternal(t) {
		if !strings.HasPrefix(rel, "screens/") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if !strings.Contains(text, `OpenAIKey == ""`) || grandfathered[rel] {
			continue
		}
		if !strings.Contains(text, "AIKeyConfigured()") {
			t.Errorf("%s spells out the AI key check by hand — use AIKeyConfigured(), which covers "+
				"both a direct key and a configured backend; checking only one tells a household on the "+
				"shared key that a working feature is unavailable", rel)
		}
	}
}

// TestEveryGrandfatheredSurfaceChecksBothHalves reads the hand-written conditions
// and asserts each one considers the backend as well as the direct key. It is the
// check that makes the grandfather list above safe to have.
func TestEveryGrandfatheredSurfaceChecksBothHalves(t *testing.T) {
	for rel, text := range readInternal(t) {
		if !strings.HasPrefix(rel, "screens/") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			if !strings.Contains(line, `OpenAIKey == ""`) {
				continue
			}
			// The condition must mention the backend on the same line, or the
			// surrounding code must resolve it (BackendActive / useBackend).
			if strings.Contains(line, "Backend") || strings.Contains(line, "backend") {
				continue
			}
			if strings.Contains(text, "BackendActive()") {
				continue
			}
			t.Errorf("%s: %q refuses on the direct key alone and the file never consults the backend",
				rel, strings.TrimSpace(line))
		}
	}
}
