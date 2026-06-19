// Package translate is the pure resolver behind CashFlux's AI-assisted UI
// translation. English (from the i18n catalog) is always the source of truth;
// other locales are filled by an AI translator and cached. This package holds
// the cache model and the pure functions that decide what text to show, which
// keys still need translating, and whether a translation preserved its
// placeholders — with no AI call, clock, or platform dependency, so every rule
// is table-tested on native Go.
//
// A cached entry is keyed by locale + a hash of the English source text. That
// keying does double duty: identical English strings dedupe to one translation,
// and changing the English copy changes the hash, which automatically
// invalidates (and triggers re-translation of) the stale cached value.
package translate

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"time"
)

// Translation is one cached, locale-specific rendering of an English source
// string. SourceHash is HashSource(SourceText); a cache entry is valid only
// while it still matches the live English source. UserEdited marks a
// human-overridden translation that the background translator must never
// overwrite.
type Translation struct {
	Locale     string    `json:"locale"`
	Key        string    `json:"key"`
	SourceText string    `json:"sourceText"`
	SourceHash string    `json:"sourceHash"`
	Text       string    `json:"text"`
	Model      string    `json:"model,omitempty"`
	UserEdited bool      `json:"userEdited,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// HashSource returns a short, stable hex digest of an English source string,
// used as the cache key and the staleness check.
func HashSource(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:8]) // 64 bits is ample for catalog-sized sets
}

// cacheID is the store identity of a translation: its locale plus source hash,
// so identical sources in a locale share one entry (dedupe).
func cacheID(locale, sourceHash string) string {
	return locale + ":" + sourceHash
}

// Cache is an in-memory index of translations by locale + source hash. The zero
// value is not usable; build one with NewCache.
type Cache struct {
	byID map[string]Translation
}

// NewCache returns an empty cache.
func NewCache() *Cache {
	return &Cache{byID: map[string]Translation{}}
}

// Put stores or replaces a translation, deriving the entry key from its locale
// and source hash. A zero SourceHash is filled from SourceText first.
func (c *Cache) Put(t Translation) {
	if t.SourceHash == "" {
		t.SourceHash = HashSource(t.SourceText)
	}
	if c.byID == nil {
		c.byID = map[string]Translation{}
	}
	c.byID[cacheID(t.Locale, t.SourceHash)] = t
}

// get returns the cached translation for a locale and source hash, if present.
func (c *Cache) get(locale, sourceHash string) (Translation, bool) {
	t, ok := c.byID[cacheID(locale, sourceHash)]
	return t, ok
}

// Resolve returns the best text to display for a key in a locale, given the live
// English source. The order — never blanking — is:
//
//  1. a cached translation whose SourceHash still matches the source, with text;
//  2. the English source itself;
//  3. the key, only if the source is empty (a programming error worth surfacing).
//
// Because the cache is keyed by source hash, an out-of-date translation (the
// English copy changed) is silently skipped in favour of English until it is
// re-translated.
func (c *Cache) Resolve(locale, key, source string) string {
	if t, ok := c.get(locale, HashSource(source)); ok && t.Text != "" {
		return t.Text
	}
	if source != "" {
		return source
	}
	return key
}

// MissingKeys returns the keys in sources (key→English source) that have no
// current, non-empty translation for locale — either never translated or stale
// (the source changed since it was cached). The result is sorted and unique, so
// the caller can batch exactly the work that remains.
func (c *Cache) MissingKeys(locale string, sources map[string]string) []string {
	var missing []string
	for key, source := range sources {
		if t, ok := c.get(locale, HashSource(source)); !ok || t.Text == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// Dedupe groups a source catalog (key→English source) by source hash, returning
// hash→sorted keys. Identical English strings collapse to one hash, so the
// translator runs once per distinct string rather than once per key.
func Dedupe(sources map[string]string) map[string][]string {
	out := map[string][]string{}
	for key, source := range sources {
		h := HashSource(source)
		out[h] = append(out[h], key)
	}
	for h := range out {
		sort.Strings(out[h])
	}
	return out
}

// placeholderRE matches the placeholder tokens CashFlux strings use: brace names
// like {count}/{name}, and printf-style verbs like %s/%d/%v (and an escaped %%).
var placeholderRE = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}|%%|%[#0-9.+\- ]*[bcdeEfFgGoqsxXvt]`)

// Placeholders returns the multiset of placeholder tokens in s, in order of
// appearance — what a translation must preserve exactly.
func Placeholders(s string) []string {
	return placeholderRE.FindAllString(s, -1)
}

// ValidatePlaceholders reports whether translated preserves exactly the same
// placeholder tokens (same set and counts) as source. A mismatch means the
// translation would break interpolation and the caller should fall back to
// English for that string.
func ValidatePlaceholders(source, translated string) bool {
	want := countTokens(Placeholders(source))
	got := countTokens(Placeholders(translated))
	if len(want) != len(got) {
		return false
	}
	for tok, n := range want {
		if got[tok] != n {
			return false
		}
	}
	return true
}

// countTokens tallies how many times each token appears.
func countTokens(tokens []string) map[string]int {
	out := make(map[string]int, len(tokens))
	for _, t := range tokens {
		out[t]++
	}
	return out
}
