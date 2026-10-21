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
	"strconv"
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
	Rows  []string // ordered retained rows, each with a stable 1-based display index
}

// BuildCorpus flattens the user's documents and artifacts into ranked-searchable
// Sources. Documents contribute their filename plus each retained extracted row's
// date / description / amount / category; artifacts contribute their name,
// columns, and every retained row cell. Query output is bounded separately, so
// late rows remain searchable without letting a large source dominate the answer.
func BuildCorpus(docs []domain.Document, arts []domain.Artifact) []Source {
	var out []Source
	for _, d := range docs {
		var b strings.Builder
		b.WriteString(d.Filename)
		b.WriteString("\n")
		rows := make([]string, 0, len(d.Extracted))
		for i, r := range d.Extracted {
			line := strings.TrimSpace(r.Date + " " + r.Description + " " + r.Amount + " " + r.Category)
			rows = append(rows, line)
			b.WriteString("Row ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(": ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		title := strings.TrimSpace(d.Filename)
		if title == "" {
			title = "Document"
		}
		out = append(out, Source{ID: d.ID, Title: title, Route: "/documents", Text: b.String(), Rows: rows})
	}
	for _, a := range arts {
		var b strings.Builder
		b.WriteString(a.Name)
		b.WriteString("\n")
		b.WriteString(strings.Join(a.Columns, " "))
		b.WriteString("\n")
		rows := make([]string, 0, len(a.Rows))
		for i, row := range a.Rows {
			line := strings.TrimSpace(strings.Join(row, " "))
			rows = append(rows, line)
			b.WriteString("Row ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(": ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		title := strings.TrimSpace(a.Name)
		if title == "" {
			title = "Attachment"
		}
		out = append(out, Source{ID: a.ID, Title: title, Route: "/artifacts", Text: b.String(), Rows: rows})
	}
	return out
}

// Hit is a scored source, most-relevant first, with one bounded evidence excerpt.
type Hit struct {
	Source           Source
	Score            int // count of distinct question keywords found in the source text
	Excerpt          string
	Row              int // 1-based source row for Excerpt; zero means title/metadata matched
	ExcerptTruncated bool
}

// Result is the outcome of a query. TotalHits counts matching sources before the
// result limit; Truncated means one or more matching sources were omitted.
type Result struct {
	Grounded  bool
	Hits      []Hit
	TotalHits int
	Truncated bool
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
		score := scoreText(s.Text, kws)
		if score > 0 {
			excerpt, row, clipped := excerptFor(s, kws)
			hits = append(hits, Hit{Source: s, Score: score, Excerpt: excerpt, Row: row, ExcerptTruncated: clipped})
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
	total := len(hits)
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return Result{Grounded: true, Hits: hits, TotalHits: total, Truncated: total > len(hits)}
}

const maxExcerptRunes = 800

func excerptFor(s Source, kws map[string]bool) (string, int, bool) {
	best, bestScore, bestRow := s.Title, 0, 0
	for i, row := range s.Rows {
		score := scoreText(row, kws)
		if score > bestScore {
			best, bestScore, bestRow = row, score, i+1
		}
	}
	if bestScore == 0 {
		best = strings.TrimSpace(s.Title)
	}
	return centerExcerpt(best, kws), bestRow, len([]rune(strings.TrimSpace(best))) > maxExcerptRunes
}

func scoreText(text string, kws map[string]bool) int {
	low, score := strings.ToLower(text), 0
	for kw := range kws {
		if strings.Contains(low, kw) {
			score++
		}
	}
	return score
}

func centerExcerpt(text string, kws map[string]bool) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxExcerptRunes {
		return string(runes)
	}
	low := strings.ToLower(string(runes))
	center := -1
	for kw := range kws {
		if i := strings.Index(low, strings.ToLower(kw)); i >= 0 {
			runeIndex := len([]rune(low[:i]))
			if center < 0 || runeIndex < center {
				center = runeIndex
			}
		}
	}
	if center < 0 {
		center = 0
	}
	start := center - maxExcerptRunes/2
	if start < 0 {
		start = 0
	}
	if start+maxExcerptRunes > len(runes) {
		start = len(runes) - maxExcerptRunes
	}
	end := start + maxExcerptRunes
	prefix, suffix := "", ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(runes) {
		suffix = "…"
	}
	return prefix + string(runes[start:end]) + suffix
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
