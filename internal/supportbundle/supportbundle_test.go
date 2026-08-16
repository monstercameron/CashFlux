// SPDX-License-Identifier: MIT

package supportbundle

import (
	"strings"
	"testing"
	"time"
)

func TestABundleCarriesWhatAMaintainerNeeds(t *testing.T) {
	got := Build(Input{
		AppVersion: "1.2.3", BuildDate: "2026-08-16", UserAgent: "Chrome/150",
		Counts:       map[string]int{"transactions": 4210, "accounts": 6},
		Flags:        map[string]string{"theme": "dark", "sync": "on"},
		RecentErrors: []string{"budget evaluate: divide by zero"},
		At:           time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC),
	})
	for _, want := range []string{"1.2.3", "Chrome/150", "transactions: 4210", "theme: dark", "divide by zero"} {
		if !strings.Contains(got, want) {
			t.Errorf("bundle missing %q:\n%s", want, got)
		}
	}
}

func TestABundleIsSafeToPasteIntoAPublicIssue(t *testing.T) {
	// This is the rule the whole package exists for: somebody WILL paste it
	// without reading it first.
	got := Build(Input{
		AppVersion: "1.2.3",
		Counts:     map[string]int{"transactions": 10},
		Flags:      map[string]string{"aiKey": "configured"},
	})
	if !strings.Contains(got, "No transactions, names, notes or amounts") {
		t.Fatalf("the bundle does not state what it excludes:\n%s", got)
	}
	// The input shape has no field that could carry a payee, a note or a name —
	// so there is nothing to filter, which is the point.
	for _, leak := range []string{"Trader Joe", "@", "$"} {
		if strings.Contains(got, leak) {
			t.Errorf("bundle contains %q:\n%s", leak, got)
		}
	}
}

func TestRedactRemovesKeysAndTokens(t *testing.T) {
	// Under-redacting puts somebody's API key in a public issue, and there is no
	// round trip that fixes that.
	got := Redact("request failed with sk-proj-abc123def456 and Bearer eyJhbGciOi")
	if strings.Contains(got, "sk-proj") || strings.Contains(got, "eyJhbGciOi") {
		t.Fatalf("a secret survived redaction: %q", got)
	}
	if !strings.Contains(got, "[api key]") || !strings.Contains(got, "[token]") {
		t.Fatalf("redaction did not mark what it removed: %q", got)
	}
}

func TestRedactRemovesEmailAddresses(t *testing.T) {
	got := Redact("failed to notify cam@example.com about the sync")
	if strings.Contains(got, "example.com") {
		t.Fatalf("an address survived: %q", got)
	}
	if !strings.Contains(got, "[email]") {
		t.Fatalf("redaction did not mark it: %q", got)
	}
}

func TestRedactRemovesMoney(t *testing.T) {
	// A figure is the most identifying thing a finance app can leak short of a
	// name, and a fault report rarely needs it.
	got := Redact("budget Groceries over by $1,240.55 and £30")
	if strings.Contains(got, "1,240") || strings.Contains(got, "30") {
		t.Fatalf("an amount survived: %q", got)
	}
	if strings.Count(got, "[amount]") != 2 {
		t.Fatalf("redaction marked %d amounts, want 2: %q", strings.Count(got, "[amount]"), got)
	}
}

func TestALoneCurrencySymbolIsNotAnAmount(t *testing.T) {
	got := Redact("the $ sign rendered wrong")
	if !strings.Contains(got, "$ sign") {
		t.Fatalf("a bare symbol was redacted as money: %q", got)
	}
}

func TestRedactLeavesOrdinaryMessagesAlone(t *testing.T) {
	msg := "budget evaluate: divide by zero in period range"
	if got := Redact(msg); got != msg {
		t.Fatalf("an ordinary message was altered:\n%q\n%q", msg, got)
	}
}

func TestErrorsAreBoundedSoTheBundleStaysReadable(t *testing.T) {
	var errs []string
	for i := 0; i < 50; i++ {
		errs = append(errs, "error number "+string(rune('a'+i%26)))
	}
	got := Build(Input{RecentErrors: errs})
	if strings.Count(got, "error number") > maxErrors {
		t.Fatalf("bundle carried more than %d errors", maxErrors)
	}
}

func TestTwoBundlesFromTheSameStateAreIdentical(t *testing.T) {
	// Map iteration order would otherwise make every bundle differ, and a diff
	// between two of them would mean nothing.
	in := Input{
		Counts: map[string]int{"z": 1, "a": 2, "m": 3},
		Flags:  map[string]string{"zeta": "on", "alpha": "off"},
	}
	first := Build(in)
	for i := 0; i < 5; i++ {
		if again := Build(in); again != first {
			t.Fatal("two bundles from the same input differ")
		}
	}
}

func TestAMissingVersionReadsAsUnknownNotAsBlank(t *testing.T) {
	got := Build(Input{})
	if !strings.Contains(got, "Version: unknown") {
		t.Fatalf("a missing version left a blank:\n%s", got)
	}
}

func TestRedactDoesNotEatOrdinaryWordsAfterAMarker(t *testing.T) {
	// "the key was rejected" must survive: redacting "was" would make the error
	// unreadable and teach maintainers to distrust the bundle.
	msg := "the key was rejected by the provider"
	if got := Redact(msg); got != msg {
		t.Fatalf("an ordinary sentence was redacted:\n%q\n%q", msg, got)
	}
}

func TestRedactTakesTheTokenAfterAMarker(t *testing.T) {
	got := Redact("sent Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6 upstream")
	if strings.Contains(got, "eyJhbGciOiJIUzI1NiIsInR5cCI6") {
		t.Fatalf("a bearer token survived: %q", got)
	}
}
