package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestTFallbackChain(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "greet", "Hello")
	b.Set("es", "greet", "Hola")

	if got := b.T("es", "greet"); got != "Hola" {
		t.Errorf("es greet = %q, want Hola", got)
	}
	// Missing in es → falls back to English.
	if got := b.T("es", "bye"); got != "bye" {
		t.Errorf("missing key everywhere = %q, want the key", got)
	}
	b.Set(English, "bye", "Goodbye")
	if got := b.T("es", "bye"); got != "Goodbye" {
		t.Errorf("es bye = %q, want English fallback Goodbye", got)
	}
	// Unknown language → English fallback.
	if got := b.T("fr", "greet"); got != "Hello" {
		t.Errorf("fr greet = %q, want English fallback Hello", got)
	}
}

func TestTFormatsArgs(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "balances", "%d balances could use a refresh")
	if got := b.T(English, "balances", 3); got != "3 balances could use a refresh" {
		t.Errorf("formatted = %q", got)
	}
}

func TestEmptyTranslationFallsBack(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "k", "English")
	b.Set("es", "k", "") // present but empty → treated as missing
	if got := b.T("es", "k"); got != "English" {
		t.Errorf("empty es value = %q, want English fallback", got)
	}
}

func TestMissingKeys(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "a", "A")
	b.Set(English, "b", "B")
	b.Set(English, "c", "C")
	b.Set("es", "a", "A-es")
	b.Set("es", "b", "") // empty counts as missing
	got := b.MissingKeys("es")
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MissingKeys(es) = %v, want %v", got, want)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "nav.accounts", "Accounts")
	b.Set("es", "nav.accounts", "Cuentas")

	data, err := b.ExportJSON()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	got := NewBundle(English)
	if err := got.ImportJSON(data); err != nil {
		t.Fatalf("import: %v", err)
	}
	if got.T("es", "nav.accounts") != "Cuentas" || got.T(English, "nav.accounts") != "Accounts" {
		t.Errorf("round-trip lost data: es=%q en=%q", got.T("es", "nav.accounts"), got.T(English, "nav.accounts"))
	}
}

func TestImportMergesAndOverwrites(t *testing.T) {
	b := NewBundle(English)
	b.Set(English, "keep", "Keep")
	b.Set(English, "over", "Old")
	if err := b.ImportJSON([]byte(`{"en":{"over":"New"},"es":{"hi":"Hola"}}`)); err != nil {
		t.Fatalf("import: %v", err)
	}
	if b.T(English, "keep") != "Keep" {
		t.Error("import dropped an existing key")
	}
	if b.T(English, "over") != "New" {
		t.Error("import did not overwrite")
	}
	if b.T("es", "hi") != "Hola" {
		t.Error("import did not add new language")
	}
}

func TestImportRejectsBadJSON(t *testing.T) {
	b := NewBundle(English)
	if err := b.ImportJSON([]byte("not json")); err == nil {
		t.Error("expected error on bad JSON")
	}
}

func TestLanguagesDefaultFirst(t *testing.T) {
	b := NewBundle(English)
	b.Set("fr", "x", "x")
	b.Set("es", "x", "x")
	got := b.Languages()
	want := []Lang{English, "es", "fr"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Languages = %v, want %v (default first, rest sorted)", got, want)
	}
}

func TestDefaultBundleSeedsEnglish(t *testing.T) {
	b := DefaultBundle()
	if b.T(English, "nav.accounts") != "Accounts" {
		t.Errorf("DefaultBundle missing nav.accounts: %q", b.T(English, "nav.accounts"))
	}
	if got := b.MissingKeys(English); len(got) != 0 {
		t.Errorf("English has missing keys against itself: %v", got)
	}
}

// TestDefaultCatalogQuality is the CI guard for the source-of-truth English
// catalog: every key must be dot-namespaced with no surrounding/embedded
// whitespace, and every key must define a non-empty string (a blank English
// value would silently surface the raw key in the UI, since lookup treats empty
// as missing). Values may legitimately carry leading/trailing spaces (suffix
// fragments like " · by %s") and literal "%", so those are intentionally not
// constrained here.
func TestDefaultCatalogQuality(t *testing.T) {
	cat := DefaultBundle().Langs[English]
	if len(cat) == 0 {
		t.Fatal("English catalog is empty")
	}
	for key, msg := range cat {
		switch {
		case key != strings.TrimSpace(key) || strings.ContainsAny(key, " \t\r\n"):
			t.Errorf("key %q contains whitespace", key)
		case !strings.Contains(key, "."):
			t.Errorf("key %q is not dot-namespaced", key)
		}
		if msg == "" {
			t.Errorf("key %q has an empty English value", key)
		}
	}
}
