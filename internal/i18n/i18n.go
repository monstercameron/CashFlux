// Package i18n is CashFlux's central language store: every user-facing string is
// looked up by a stable dot-namespaced key (e.g. "nav.accounts") in a per-language
// Catalog. English is the source language and the fallback; other languages are
// added by importing a JSON bundle. The whole set round-trips through ExportJSON /
// ImportJSON so all supported languages can be handed to translators and loaded
// back. Pure Go, no platform dependencies, unit-tested on native Go.
package i18n

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Lang is a language tag (BCP-47-ish short code, e.g. "en", "es", "fr").
type Lang string

// English is the source language and the fallback for any missing key.
const English Lang = "en"

// Catalog maps message keys to translated strings for a single language.
type Catalog map[string]string

// Bundle holds every language's Catalog plus the default (source) language used
// as the fallback. The zero value is not usable; build one with NewBundle.
type Bundle struct {
	Default Lang
	Langs   map[Lang]Catalog
}

// NewBundle returns an empty bundle whose default/fallback language is def.
func NewBundle(def Lang) *Bundle {
	return &Bundle{Default: def, Langs: map[Lang]Catalog{def: {}}}
}

// Set records msg for key in the given language, creating the language's catalog
// if needed.
func (b *Bundle) Set(lang Lang, key, msg string) {
	if b.Langs == nil {
		b.Langs = map[Lang]Catalog{}
	}
	if b.Langs[lang] == nil {
		b.Langs[lang] = Catalog{}
	}
	b.Langs[lang][key] = msg
}

// T returns the translation of key in lang, falling back to the default language
// and then to the key itself when the key is unknown or untranslated. When args
// are supplied the result is formatted with fmt.Sprintf, so catalog strings may
// contain verbs like "%s" / "%d".
func (b *Bundle) T(lang Lang, key string, args ...any) string {
	msg, ok := b.lookup(lang, key)
	if !ok {
		if msg, ok = b.lookup(b.Default, key); !ok {
			msg = key // surface the key so a missing translation is obvious
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// lookup returns the non-empty translation for key in lang, if present.
func (b *Bundle) lookup(lang Lang, key string) (string, bool) {
	if c, ok := b.Langs[lang]; ok {
		if msg, ok := c[key]; ok && msg != "" {
			return msg, true
		}
	}
	return "", false
}

// Languages returns the languages in the bundle, sorted, with the default first.
func (b *Bundle) Languages() []Lang {
	rest := make([]Lang, 0, len(b.Langs))
	for l := range b.Langs {
		if l != b.Default {
			rest = append(rest, l)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	return append([]Lang{b.Default}, rest...)
}

// MissingKeys returns the keys present in the default language but missing (or
// empty) in lang — the translation coverage gap — sorted for stable output.
func (b *Bundle) MissingKeys(lang Lang) []string {
	var missing []string
	for key := range b.Langs[b.Default] {
		if _, ok := b.lookup(lang, key); !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// ExportJSON serializes every language's catalog to indented JSON keyed by
// language code — the portable bundle translators edit and re-import.
func (b *Bundle) ExportJSON() ([]byte, error) {
	return json.MarshalIndent(b.Langs, "", "  ")
}

// ImportJSON merges a JSON bundle (language code → key → message) into b,
// overwriting per-key collisions and adding new languages. Existing keys not
// present in the import are left untouched.
func (b *Bundle) ImportJSON(data []byte) error {
	var in map[Lang]Catalog
	if err := json.Unmarshal(data, &in); err != nil {
		return fmt.Errorf("i18n: import: %w", err)
	}
	if b.Langs == nil {
		b.Langs = map[Lang]Catalog{}
	}
	for lang, cat := range in {
		if b.Langs[lang] == nil {
			b.Langs[lang] = Catalog{}
		}
		for key, msg := range cat {
			b.Langs[lang][key] = msg
		}
	}
	return nil
}
