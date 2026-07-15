// SPDX-License-Identifier: MIT

package docqa

import (
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
