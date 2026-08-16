// SPDX-License-Identifier: MIT

// Package supportbundle builds the diagnostic somebody attaches to a bug report
// (PS10).
//
// Piggy's version of this attaches the whole conversation to a support ticket,
// which works because their AI runs on their servers and the transcript is already
// theirs. Ours cannot do that and should not want to: the conversation contains
// the household's money, and the whole promise of the app is that it stays on the
// device.
//
// So this is the local-first version. It collects what a maintainer genuinely needs
// to reproduce a fault — versions, settings SHAPE, recent errors, counts — and
// nothing that identifies anybody. The rule it is built around is that the bundle
// must be safe to paste into a public GitHub issue without reading it first,
// because that is exactly what somebody will do.
//
// That rule is enforced by construction rather than by filtering: this package is
// given only the redacted values, and a test asserts that the shape it produces
// contains no free text from the household's data at all.
//
// Pure: no syscall/js. Table-tested on native Go.
package supportbundle

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Input is what the bundle is built from. Every field is a version, a count, a
// flag, or an error message the app itself produced — never a payee, a name, a
// note, or an amount.
type Input struct {
	// AppVersion and BuildDate identify the code.
	AppVersion, BuildDate string
	// UserAgent is the browser, which is where rendering faults live.
	UserAgent string
	// Counts are how many of each entity exist. A fault that only appears at scale
	// is invisible without them, and a count identifies nobody.
	Counts map[string]int
	// Flags are the on/off settings whose state changes behaviour: which theme,
	// whether sync is on, whether an AI key is configured. Values are booleans or
	// short enum tokens, never anything typed by a person.
	Flags map[string]string
	// RecentErrors are the app's own logged errors, newest first. They are the
	// single most useful thing in the bundle and the most likely to leak, so the
	// caller must pass them already redacted — see Redact.
	RecentErrors []string
	// At is when the bundle was taken.
	At time.Time
}

// maxErrors bounds how many errors ride along. The most recent handful is what
// diagnoses a fault; a hundred is a log dump nobody reads and a bigger surface for
// something private to hide in.
const maxErrors = 10

// Build renders the bundle as Markdown, ready to paste into an issue.
func Build(in Input) string {
	var b strings.Builder
	b.WriteString("### CashFlux diagnostic\n\n")
	b.WriteString("_No transactions, names, notes or amounts are included._\n\n")

	fmt.Fprintf(&b, "- Version: %s\n", fallbackStr(in.AppVersion, "unknown"))
	fmt.Fprintf(&b, "- Built: %s\n", fallbackStr(in.BuildDate, "unknown"))
	fmt.Fprintf(&b, "- Browser: %s\n", fallbackStr(in.UserAgent, "unknown"))
	if !in.At.IsZero() {
		fmt.Fprintf(&b, "- Taken: %s\n", in.At.UTC().Format(time.RFC3339))
	}

	if len(in.Counts) > 0 {
		b.WriteString("\n**How much data**\n\n")
		for _, k := range sortedKeys(in.Counts) {
			fmt.Fprintf(&b, "- %s: %d\n", k, in.Counts[k])
		}
	}
	if len(in.Flags) > 0 {
		b.WriteString("\n**Settings**\n\n")
		for _, k := range sortedKeys(in.Flags) {
			fmt.Fprintf(&b, "- %s: %s\n", k, in.Flags[k])
		}
	}
	if len(in.RecentErrors) > 0 {
		b.WriteString("\n**Recent errors**\n\n```\n")
		errs := in.RecentErrors
		if len(errs) > maxErrors {
			errs = errs[:maxErrors]
		}
		for _, e := range errs {
			b.WriteString(strings.TrimSpace(e) + "\n")
		}
		b.WriteString("```\n")
	}
	return b.String()
}

// secretPrefixes are word beginnings that mean the word itself is a secret.
var secretPrefixes = []struct{ prefix, label string }{
	{"sk-", "[api key]"},
	{"eyJ", "[token]"}, // a JWT always starts with the base64 of `{"`
}

// secretMarkers are words that mean the word AFTER them is a secret. They are
// separate from the prefixes because a bearer token has no distinguishing shape of
// its own — only its position gives it away, and matching on "Bearer " as a prefix
// silently matched nothing, since splitting on whitespace puts the marker and the
// token in different fields. The test caught that; the shape of the fix is why the
// two cases are now modelled separately instead of as one list.
var secretMarkers = map[string]string{
	"bearer":         "[token]",
	"authorization:": "[token]",
	"token":          "[token]",
	"key":            "[api key]",
}

// Redact removes the things an error message can accidentally carry: keys, tokens,
// email addresses and anything that looks like money.
//
// It is deliberately aggressive. Over-redacting costs a maintainer one round trip
// asking for more detail; under-redacting puts somebody's API key in a public
// issue, and there is no round trip that fixes that.
func Redact(msg string) string {
	out := redactSecrets(msg)
	out = redactEmails(out)
	out = redactAmounts(out)
	return out
}

// redactSecrets removes words that ARE secrets and words that FOLLOW a marker
// naming one.
func redactSecrets(s string) string {
	fields := strings.Fields(s)
	changed := false
	for i, f := range fields {
		for _, p := range secretPrefixes {
			if strings.HasPrefix(f, p.prefix) {
				fields[i], changed = p.label, true
				break
			}
		}
	}
	for i, f := range fields {
		label, marks := secretMarkers[strings.ToLower(strings.Trim(f, ":="))]
		if !marks || i+1 >= len(fields) {
			continue
		}
		// Only a word that could plausibly BE a secret is taken: "the key was
		// rejected" must not have "was" redacted out of it.
		if looksOpaque(fields[i+1]) {
			fields[i+1], changed = label, true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(fields, " ")
}

// minOpaqueLength is how long a word must be before it can be a secret. Real keys
// and tokens are far longer than this; ordinary English words that follow the word
// "key" are shorter.
const minOpaqueLength = 12

// looksOpaque reports whether a word could be a credential: long, and not an
// ordinary word. Being wrong in the redacting direction costs a maintainer one
// round trip; being wrong the other way is unrecoverable.
func looksOpaque(word string) bool {
	w := strings.Trim(word, ".,;:'\"")
	if len(w) < minOpaqueLength {
		return false
	}
	for _, r := range w {
		if r >= 'A' && r <= 'Z' {
			return true
		}
		if r >= '0' && r <= '9' {
			return true
		}
		if r == '-' || r == '_' || r == '.' {
			return true
		}
	}
	return false
}

// redactEmails replaces anything shaped like an address.
func redactEmails(s string) string {
	fields := strings.Fields(s)
	changed := false
	for i, f := range fields {
		at := strings.Index(f, "@")
		if at > 0 && strings.Contains(f[at:], ".") {
			fields[i] = "[email]"
			changed = true
		}
	}
	if !changed {
		return s
	}
	return strings.Join(fields, " ")
}

// redactAmounts replaces anything that reads as money. A fault report rarely needs
// the figure, and a figure is the most identifying thing a finance app can leak
// short of a name.
func redactAmounts(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '$' && runes[i] != '£' && runes[i] != '€' {
			b.WriteRune(runes[i])
			continue
		}
		j := i + 1
		for j < len(runes) && (isDigit(runes[j]) || runes[j] == ',' || runes[j] == '.') {
			j++
		}
		if j == i+1 {
			// A lone currency symbol with no number after it is just a symbol.
			b.WriteRune(runes[i])
			continue
		}
		b.WriteString("[amount]")
		i = j - 1
	}
	return b.String()
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

// sortedKeys returns a map's keys in order, so two bundles from the same state are
// identical and a diff between them means something.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// fallbackStr returns a default for an empty value, so the bundle never has a
// blank where a fact should be — a missing version reads as a bug in the bundle
// rather than as an unknown.
func fallbackStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
