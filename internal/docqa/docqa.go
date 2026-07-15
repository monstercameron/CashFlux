// SPDX-License-Identifier: MIT

// Package docqa assembles a grounded retrieval corpus over the user's attached
// artifacts and imported documents, so the assistant can answer questions like
// "what was on the March statement?" or "when does my insurance renew?" from the
// user's OWN records instead of hallucinating a document (AG13).
//
// It does the deterministic, testable half: turn documents/artifacts into Source
// records, rank them against a question by keyword overlap, and build a cited
// answer SCAFFOLD (the ranked source text plus its citation handle). The model
// composes the prose from that scaffold. When nothing in the corpus is relevant it
// returns a graceful refusal rather than a guess.
//
// Pure Go, no syscall/js: unit-tested on native Go.
package docqa

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// Source is one retrievable artifact/document in the corpus: a stable ID (for the
// citation link), a human Title, a Route+ID the UI opens, and the flattened Text
// the question is matched against.
type Source struct {
	ID    string
	Title string
	Route string // e.g. "/documents" or "/artifacts"
	Text  string
}

// BuildCorpus flattens the user's documents and artifacts into ranked-searchable
// Sources. Documents contribute their filename plus each extracted row's date /
// description / amount / category; artifacts contribute their name, columns, and
// row cells (capped so a huge sheet doesn't dominate).
func BuildCorpus(docs []domain.Document, arts []domain.Artifact) []Source {
	var out []Source
	for _, d := range docs {
		var b strings.Builder
		b.WriteString(d.Filename)
		b.WriteString("\n")
		for _, r := range d.Extracted {
			b.WriteString(strings.TrimSpace(r.Date + " " + r.Description + " " + r.Amount + " " + r.Category))
			b.WriteString("\n")
		}
		title := strings.TrimSpace(d.Filename)
		if title == "" {
			title = "Document"
		}
		out = append(out, Source{ID: d.ID, Title: title, Route: "/documents", Text: b.String()})
	}
	for _, a := range arts {
		var b strings.Builder
		b.WriteString(a.Name)
		b.WriteString("\n")
		b.WriteString(strings.Join(a.Columns, " "))
		b.WriteString("\n")
		const maxRows = 200
		for i, row := range a.Rows {
			if i >= maxRows {
				break
			}
			b.WriteString(strings.Join(row, " "))
			b.WriteString("\n")
		}
		title := strings.TrimSpace(a.Name)
		if title == "" {
			title = "Attachment"
		}
		out = append(out, Source{ID: a.ID, Title: title, Route: "/artifacts", Text: b.String()})
	}
	return out
}

// Hit is a scored source, most-relevant first.
type Hit struct {
	Source Source
	Score  int // count of distinct question keywords found in the source text
}

// Result is the outcome of a query: the ranked hits (possibly empty) and a
// Grounded flag that is false when nothing matched, so the caller refuses instead
// of composing an answer from thin air.
type Result struct {
	Grounded bool
	Hits     []Hit
}

// stopwords are common words dropped from the keyword set so ranking keys on the
// meaningful terms of a question.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "of": true, "on": true, "in": true,
	"is": true, "was": true, "what": true, "when": true, "does": true, "do": true,
	"my": true, "me": true, "to": true, "for": true, "and": true, "how": true,
	"much": true, "did": true, "i": true, "it": true, "that": true, "this": true,
}

// Query ranks the corpus against question and returns up to limit grounded hits.
// A source scores by how many distinct question keywords appear in its text;
// zero-score sources are dropped. When no source scores, Grounded is false.
func Query(corpus []Source, question string, limit int) Result {
	kws := keywords(question)
	if len(kws) == 0 {
		return Result{}
	}
	var hits []Hit
	for _, s := range corpus {
		low := strings.ToLower(s.Text)
		score := 0
		for kw := range kws {
			if strings.Contains(low, kw) {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, Hit{Source: s, Score: score})
		}
	}
	if len(hits) == 0 {
		return Result{}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Source.ID < hits[j].Source.ID
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return Result{Grounded: true, Hits: hits}
}

// keywords lowercases question and returns its distinct non-stopword tokens of
// length >= 2.
func keywords(question string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(question), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(w) >= 2 && !stopwords[w] {
			out[w] = true
		}
	}
	return out
}
