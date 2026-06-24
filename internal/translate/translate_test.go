// SPDX-License-Identifier: MIT

package translate

import (
	"reflect"
	"testing"
)

func TestResolveOrder(t *testing.T) {
	c := NewCache()
	c.Put(Translation{Locale: "es", Key: "nav.accounts", SourceText: "Accounts", Text: "Cuentas"})

	// 1. Cached translation whose source still matches.
	if got := c.Resolve("es", "nav.accounts", "Accounts"); got != "Cuentas" {
		t.Errorf("cached resolve = %q, want Cuentas", got)
	}
	// 2. No translation → English source.
	if got := c.Resolve("es", "nav.budgets", "Budgets"); got != "Budgets" {
		t.Errorf("missing resolve = %q, want English source Budgets", got)
	}
	// 3. Empty source → the key (last resort).
	if got := c.Resolve("es", "nav.orphan", ""); got != "nav.orphan" {
		t.Errorf("empty-source resolve = %q, want the key", got)
	}
}

func TestStaleTranslationFallsBackToEnglish(t *testing.T) {
	c := NewCache()
	c.Put(Translation{Locale: "es", Key: "greeting", SourceText: "Hello", Text: "Hola"})
	// The English source changed; the cached "Hola" is for the old hash and must
	// be ignored in favour of the new English until re-translated.
	if got := c.Resolve("es", "greeting", "Hello there"); got != "Hello there" {
		t.Errorf("stale resolve = %q, want new English source", got)
	}
	// The original source still resolves to the cached translation.
	if got := c.Resolve("es", "greeting", "Hello"); got != "Hola" {
		t.Errorf("unchanged resolve = %q, want Hola", got)
	}
}

func TestMissingKeys(t *testing.T) {
	c := NewCache()
	c.Put(Translation{Locale: "es", SourceText: "Accounts", Text: "Cuentas"})
	c.Put(Translation{Locale: "es", SourceText: "Goals", Text: ""}) // empty = not translated
	sources := map[string]string{
		"a": "Accounts", // translated
		"b": "Budgets",  // never translated
		"g": "Goals",    // cached but empty
	}
	got := c.MissingKeys("es", sources)
	want := []string{"b", "g"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingKeys = %v, want %v", got, want)
	}
}

func TestDedupe(t *testing.T) {
	// "OK" appears under two keys → one hash with both keys, sorted.
	sources := map[string]string{
		"dialog.ok":  "OK",
		"confirm.ok": "OK",
		"nav.home":   "Home",
	}
	got := Dedupe(sources)
	if len(got) != 2 {
		t.Fatalf("Dedupe produced %d groups, want 2", len(got))
	}
	okHash := HashSource("OK")
	if !reflect.DeepEqual(got[okHash], []string{"confirm.ok", "dialog.ok"}) {
		t.Errorf("OK group = %v, want sorted [confirm.ok dialog.ok]", got[okHash])
	}
}

func TestHashSourceStableAndDistinct(t *testing.T) {
	if HashSource("Accounts") != HashSource("Accounts") {
		t.Error("HashSource not stable for equal input")
	}
	if HashSource("Accounts") == HashSource("Budgets") {
		t.Error("HashSource collided on different input")
	}
}

func TestPlaceholders(t *testing.T) {
	got := Placeholders("Hi {name}, you have %d new alerts (%s)")
	want := []string{"{name}", "%d", "%s"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Placeholders = %v, want %v", got, want)
	}
}

func TestValidatePlaceholders(t *testing.T) {
	tests := []struct {
		name              string
		source, translate string
		want              bool
	}{
		{"identical", "Hello {name}", "Hola {name}", true},
		{"reordered same set", "{a} and {b}", "{b} y {a}", true},
		{"dropped placeholder", "Hello {name}", "Hola", false},
		{"added placeholder", "Hello", "Hola {name}", false},
		{"printf preserved", "%d items left", "quedan %d", true},
		{"printf count mismatch", "%d of %d", "%d", false},
		{"no placeholders", "Save", "Guardar", true},
		{"escaped percent", "100%% done", "100%% hecho", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePlaceholders(tt.source, tt.translate); got != tt.want {
				t.Errorf("ValidatePlaceholders(%q, %q) = %v, want %v", tt.source, tt.translate, got, tt.want)
			}
		})
	}
}

func TestPutFillsSourceHash(t *testing.T) {
	c := NewCache()
	c.Put(Translation{Locale: "fr", SourceText: "Save", Text: "Enregistrer"})
	// Resolve relies on the derived hash matching, proving Put filled it.
	if got := c.Resolve("fr", "save", "Save"); got != "Enregistrer" {
		t.Errorf("resolve after auto-hash = %q, want Enregistrer", got)
	}
}
