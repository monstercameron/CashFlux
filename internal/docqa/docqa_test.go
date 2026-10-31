// SPDX-License-Identifier: MIT

package docqa

import (
	"fmt"
	"strings"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func corpus() []Source {
	docs := []domain.Document{
		{ID: "d1", Filename: "March statement.pdf", Extracted: []domain.DocumentRow{
			{Date: "2025-03-04", Description: "Whole Foods", Amount: "-52.10"},
			{Date: "2025-03-09", Description: "Insurance renewal premium", Amount: "-140.00"},
		}},
		{ID: "d2", Filename: "April statement.pdf", Extracted: []domain.DocumentRow{
			{Date: "2025-04-02", Description: "Netflix", Amount: "-15.99"},
		}},
	}
	arts := []domain.Artifact{
		{ID: "a1", Name: "car insurance policy", Columns: []string{"field", "value"}, Rows: [][]string{{"renews", "June 1"}}},
	}
	return BuildCorpus(docs, arts)
}

func TestQueryGrounded(t *testing.T) {
	r := Query(corpus(), "what was on the March statement?", 3)
	if !r.Grounded || len(r.Hits) == 0 {
		t.Fatalf("expected grounded hits, got %+v", r)
	}
	if r.Hits[0].Source.ID != "d1" {
		t.Errorf("top hit = %s want d1", r.Hits[0].Source.ID)
	}
	if r.Hits[0].Source.Route != "/documents" {
		t.Errorf("route = %s", r.Hits[0].Source.Route)
	}
}

func TestQueryInsuranceRenewal(t *testing.T) {
	r := Query(corpus(), "when does my insurance renew?", 3)
	if !r.Grounded {
		t.Fatal("expected grounded")
	}
	// Both the March statement (renewal premium) and the policy mention insurance;
	// the policy artifact should rank because it matches "insurance" + "renew".
	found := false
	for _, h := range r.Hits {
		if h.Source.ID == "a1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected policy artifact a1 in hits: %+v", r.Hits)
	}
}

func TestQueryRefusesWhenAbsent(t *testing.T) {
	r := Query(corpus(), "what is my cryptocurrency wallet balance?", 3)
	if r.Grounded || len(r.Hits) != 0 {
		t.Errorf("expected ungrounded refusal, got %+v", r)
	}
}

func TestQueryEmptyQuestion(t *testing.T) {
	if r := Query(corpus(), "the a of", 3); r.Grounded {
		t.Error("stopword-only question should not be grounded")
	}
}

func TestBuildCorpusIncludesArtifactRowsAfterTwoHundred(t *testing.T) {
	rows := make([][]string, 240)
	for i := range rows {
		rows[i] = []string{fmt.Sprintf("ordinary row %d", i+1)}
	}
	rows[231] = []string{"late row", "policy renews November"}
	corpus := BuildCorpus(nil, []domain.Artifact{{ID: "a-late", Name: "Policy data", Columns: []string{"description"}, Rows: rows}})
	result := Query(corpus, "when does policy renew", 3)
	if !result.Grounded || result.TotalHits != 1 {
		t.Fatalf("late retained row was not found: %+v", result)
	}
	if result.Hits[0].Row != 232 || !strings.Contains(result.Hits[0].Excerpt, "policy renews November") {
		t.Fatalf("hit lost row identity or matching evidence: %+v", result.Hits[0])
	}
}

func TestQueryExcerptCentersOnMatchingRowAndReportsSourceTruncation(t *testing.T) {
	long := strings.Repeat("unrelated text ", 100)
	corpus := []Source{
		{ID: "doc", Title: "Statement", Text: long + " exactmatch " + long, Rows: []string{long + " exactmatch " + long}},
		{ID: "doc-2", Title: "Exactmatch duplicate", Text: "exactmatch"},
	}
	result := Query(corpus, "find exactmatch", 1)
	if len(result.Hits) != 1 || !result.Truncated || result.TotalHits != 2 {
		t.Fatalf("unexpected page metadata: %+v", result)
	}
	if !strings.Contains(result.Hits[0].Excerpt, "exactmatch") || len([]rune(result.Hits[0].Excerpt)) > maxExcerptRunes+2 {
		t.Fatalf("excerpt did not center the match within its bound: length=%d excerpt=%q", len([]rune(result.Hits[0].Excerpt)), result.Hits[0].Excerpt)
	}
	if !result.Hits[0].ExcerptTruncated {
		t.Fatal("long matching row did not report that its excerpt was shortened")
	}
}
