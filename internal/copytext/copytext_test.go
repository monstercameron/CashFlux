// SPDX-License-Identifier: MIT

package copytext

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestOfCarriesAllThreeParts(t *testing.T) {
	got := Of("smart.a4.title", "Move idle cash from %s to earn more", "Joint Savings")
	if got.Key != "smart.a4.title" {
		t.Errorf("Key = %q", got.Key)
	}
	if len(got.Args) != 1 || got.Args[0] != "Joint Savings" {
		t.Errorf("Args = %v", got.Args)
	}
	if want := "Move idle cash from Joint Savings to earn more"; got.Fallback != want {
		t.Errorf("Fallback = %q, want %q", got.Fallback, want)
	}
}

func TestResolvePrefersTheCatalog(t *testing.T) {
	tr := func(key string, args ...any) string {
		if key == "smart.a4.title" {
			return "Déplacez les liquidités depuis " + args[0].(string)
		}
		return key
	}
	got := Of("smart.a4.title", "Move idle cash from %s to earn more", "Joint Savings").Resolve(tr)
	if want := "Déplacez les liquidités depuis Joint Savings"; got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

// The case that decides whether this is safe to roll out incrementally: a
// catalog that has not caught up with a new detector must show that detector's
// English, never the raw key.
func TestResolveFallsBackWhenTheCatalogDoesNotKnowTheKey(t *testing.T) {
	echoKey := func(key string, args ...any) string { return key }
	txt := Of("smart.brandnew.title", "A brand new finding about %s", "your card")
	if got := txt.Resolve(echoKey); got != "A brand new finding about your card" {
		t.Errorf("Resolve = %q — an unknown key must degrade to English, not to a key name", got)
	}
	empty := func(key string, args ...any) string { return "" }
	if got := txt.Resolve(empty); got != "A brand new finding about your card" {
		t.Errorf("Resolve on an empty translation = %q", got)
	}
	if got := txt.Resolve(nil); got != "A brand new finding about your card" {
		t.Errorf("Resolve with no translator = %q", got)
	}
}

func TestPlainIsVisiblyUntranslatable(t *testing.T) {
	p := Plain("Not converted yet")
	if p.Key != "" {
		t.Error("Plain invented a key")
	}
	if got := p.Resolve(func(string, ...any) string { return "translated" }); got != "Not converted yet" {
		t.Errorf("Resolve = %q — Plain copy has no key to translate through", got)
	}
	if p.String() != "Not converted yet" {
		t.Errorf("String = %q", p.String())
	}
}

func TestEmpty(t *testing.T) {
	var zero Text
	if !zero.Empty() {
		t.Error("the zero Text is not Empty")
	}
	if !Plain("").Empty() {
		t.Error("Plain(\"\") is not Empty")
	}
	if Of("k", "x").Empty() {
		t.Error("a real Text reports Empty")
	}
}

// The persistence property this type exists for: a stored notification must
// carry enough to be re-rendered in a language chosen later.
func TestRoundTripsThroughJSONWithEnoughToReTranslate(t *testing.T) {
	orig := Of("notify.paycheck", "Your paycheck landed — %s", "$4,700.00")
	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Text
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Key != orig.Key {
		t.Errorf("key lost: %q", back.Key)
	}
	if len(back.Args) != 1 || back.Args[0] != "$4,700.00" {
		t.Errorf("args lost: %v — without them the sentence can never be rebuilt", back.Args)
	}
	if back.Fallback != orig.Fallback {
		t.Errorf("fallback lost: %q", back.Fallback)
	}
	// And it re-renders in a new language from the stored parts alone.
	fr := func(key string, args ...any) string {
		if key == "notify.paycheck" {
			return "Votre salaire est arrivé — " + args[0].(string)
		}
		return key
	}
	if got, want := back.Resolve(fr), "Votre salaire est arrivé — $4,700.00"; got != want {
		t.Errorf("re-render = %q, want %q", got, want)
	}
}

func TestNumericAndScalarArgumentsRoundTripWithTheirFormattingTypes(t *testing.T) {
	orig := Of("test.scalarArgs", "%d|%d|%.1f|%s|%t", int(7), int64(8), 2.5, "hello", true)
	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Text
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := fmt.Sprintf("%T", back.Args[0]), "int64"; got != want {
		t.Errorf("decoded int type = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%T", back.Args[1]), "int64"; got != want {
		t.Errorf("decoded int64 type = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%T", back.Args[2]), "float64"; got != want {
		t.Errorf("decoded float type = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%T", back.Args[3]), "string"; got != want {
		t.Errorf("decoded string type = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%T", back.Args[4]), "bool"; got != want {
		t.Errorf("decoded bool type = %q, want %q", got, want)
	}
	translator := func(_ string, args ...any) string {
		return fmt.Sprintf("%d|%d|%.1f|%s|%t", args...)
	}
	if got, want := back.Resolve(translator), "7|8|2.5|hello|true"; got != want {
		t.Errorf("resolved = %q, want %q", got, want)
	}
}

func TestOverdueIntegerRendersAfterRoundTrip(t *testing.T) {
	orig := Of("notify.taskBodyOverdue", "Overdue by %d days.", 26)
	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Text
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	translator := func(_ string, args ...any) string {
		return fmt.Sprintf("Overdue by %d days.", args...)
	}
	if got, want := back.Resolve(translator), "Overdue by 26 days."; got != want {
		t.Errorf("resolved = %q, want %q", got, want)
	}
}

func TestOldStoredIntegralNumberRendersAsInteger(t *testing.T) {
	const oldFeed = `{"key":"notify.taskBodyOverdue","args":[26],"text":"Overdue by 26 days."}`
	var back Text
	if err := json.Unmarshal([]byte(oldFeed), &back); err != nil {
		t.Fatalf("unmarshal old feed: %v", err)
	}
	translator := func(_ string, args ...any) string {
		return fmt.Sprintf("Overdue by %d days.", args...)
	}
	if got, want := back.Resolve(translator), "Overdue by 26 days."; got != want {
		t.Errorf("resolved = %q, want %q", got, want)
	}
}

func TestResolveUsesFallbackForTranslatorFormattingError(t *testing.T) {
	txt := Of("notify.taskBodyOverdue", "Overdue by %d days.", 26)
	translator := func(_ string, _ ...any) string { return "Overdue by %!d(float64=26) days." }
	if got, want := txt.Resolve(translator), "Overdue by 26 days."; got != want {
		t.Errorf("resolved = %q, want fallback %q", got, want)
	}
}
