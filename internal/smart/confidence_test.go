// SPDX-License-Identifier: MIT

package smart

import (
	"strings"
	"testing"
)

func TestArithmeticFindingsAreCertainByDefault(t *testing.T) {
	// A budget-over notice restates what the ledger says; it is not a guess.
	if got := ConfidenceFor("SMART-B1"); got != ConfidenceCertain {
		t.Fatalf("ConfidenceFor(SMART-B1) = %v, want certain", got)
	}
	if ConfidenceCertain.Hedged() {
		t.Fatal("a certain finding reported as hedged, so every row would carry a tier chip")
	}
}

func TestInferredFindingsCarryTheirHedge(t *testing.T) {
	for _, tc := range []struct {
		feature string
		want    Confidence
	}{
		{"SMART-T2", ConfidencePossible},  // duplicates
		{"SMART-T6", ConfidencePossible},  // spending spike
		{"SMART-T7", ConfidenceLikely},    // missing transaction
		{"SMART-A1", ConfidenceLikely},    // balance anomaly
		{"SMART-T20", ConfidencePossible}, // new subscription
	} {
		got := ConfidenceFor(tc.feature)
		if got != tc.want {
			t.Errorf("ConfidenceFor(%s) = %v, want %v", tc.feature, got, tc.want)
		}
		if !got.Hedged() {
			t.Errorf("%s is an inference but reports as unhedged", tc.feature)
		}
	}
}

func TestResolvedConfidencePrefersWhatTheDetectorSaid(t *testing.T) {
	// A duplicate match on an identical reference number is not a guess, even
	// though its feature's findings usually are.
	i := Insight{Feature: "SMART-T2"}.WithConfidence(ConfidenceCertain)
	if got := i.ResolvedConfidence(); got != ConfidenceCertain {
		t.Fatalf("resolved = %v, want the detector's own answer", got)
	}
}

func TestAnUnsetConfidenceDoesNotSilentlyMeanCertain(t *testing.T) {
	// The zero Confidence value IS ConfidenceCertain, so a detector that never
	// set one must fall through to its feature's default rather than inheriting
	// the most reassuring answer by accident. This is the whole reason
	// ResolvedConfidence exists instead of reading the field.
	i := Insight{Feature: "SMART-T2"}
	if got := i.ResolvedConfidence(); got != ConfidencePossible {
		t.Fatalf("resolved = %v, want the feature default (possible), not the zero value", got)
	}
}

func TestLabelsReadAsHedgesAPersonWouldSay(t *testing.T) {
	for _, tc := range []struct {
		c    Confidence
		want string
	}{
		{ConfidenceCertain, "From your data"},
		{ConfidenceLikely, "Likely"},
		{ConfidencePossible, "Worth a look"},
	} {
		if got := tc.c.Label(); got != tc.want {
			t.Errorf("%v.Label() = %q, want %q", tc.c, got, tc.want)
		}
	}
}

func TestEveryInferredFeatureIsInTheCatalog(t *testing.T) {
	// A confidence entry for a feature that no longer exists is dead weight that
	// silently stops applying — and reads, at a glance, as coverage it isn't.
	known := map[string]bool{}
	for _, f := range Catalog() {
		known[f.Code] = true
	}
	for code := range inferredFeatures {
		if !known[code] {
			t.Errorf("%s carries a confidence tier but is not in the catalog", code)
		}
	}
}

func TestHeuristicSoundingCatalogEntriesDeclareTheirConfidence(t *testing.T) {
	// The confidence table lists the guessers; everything else defaults to
	// certain. That default is right for arithmetic and wrong for a new
	// heuristic, so this walks the catalog for detectors whose own summary
	// admits to inferring and asks that they say so explicitly.
	hedgeWords := []string{"looks like", "probably", "predict", "forecast", "guess", "might be", "possible"}
	for _, f := range Catalog() {
		if _, declared := inferredFeatures[f.Code]; declared {
			continue
		}
		summary := strings.ToLower(f.Summary)
		for _, w := range hedgeWords {
			if strings.Contains(summary, w) {
				t.Errorf("%s (%q) describes an inference but has no confidence tier — add one to inferredFeatures", f.Code, f.Summary)
				break
			}
		}
	}
}
