package pwcheck

import (
	"strings"
	"testing"
)

func TestValidatePIN(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		wantOK bool
	}{
		{"too short", "12345", false},
		{"non-numeric", "12ab56", false},
		{"all same", "000000", false},
		{"ascending", "123456", false},
		{"descending", "654321", false},
		{"common 4-digit padded? no", "696969", true}, // not on list, not trivial
		{"common pin 1004 variant", "100400", true},
		{"on common-pin list", "159753", false},
		{"good", "836194", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(PIN, tt.value)
			if got.OK != tt.wantOK {
				t.Errorf("Validate(PIN, %q).OK = %v, want %v (issues: %v)", tt.value, got.OK, tt.wantOK, got.Issues)
			}
			if !got.OK && len(got.Issues) == 0 {
				t.Errorf("Validate(PIN, %q) failed but reported no issues", tt.value)
			}
		})
	}
}

func TestTrivialPINScoresZero(t *testing.T) {
	for _, v := range []string{"000000", "123456", "111111", "654321"} {
		if s := Validate(PIN, v).Score; s != 0 {
			t.Errorf("trivial PIN %q scored %d, want 0", v, s)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		wantOK bool
	}{
		{"too short", "abc123", false},
		{"common", "password", false},
		{"common cased", "PassWord", false},
		{"common with spaces trimmed", "  password123  ", false},
		{"contains app name", "cashflux99", false},
		{"decent", "tr0ub4dor!x", true},
		{"long unique", "horsestaple battery 9", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(Password, tt.value)
			if got.OK != tt.wantOK {
				t.Errorf("Validate(Password, %q).OK = %v, want %v (issues: %v)", tt.value, got.OK, tt.wantOK, got.Issues)
			}
		})
	}
}

func TestValidatePassphrase(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		wantOK bool
	}{
		{"too few words", "correct horse", false},
		{"too short overall", "a b c d", false},
		{"four good words", "correct horse battery staple", true},
		{"long enough", "rainy gravel orbit lantern", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(Passphrase, tt.value)
			if got.OK != tt.wantOK {
				t.Errorf("Validate(Passphrase, %q).OK = %v, want %v (issues: %v)", tt.value, got.OK, tt.wantOK, got.Issues)
			}
		})
	}
}

func TestContextScreening(t *testing.T) {
	ctx := Context{Terms: []string{"Smith", "Checking"}}
	got := ValidateWithContext(Password, "smithfamily22", ctx)
	if got.OK {
		t.Errorf("password built on a household name should be rejected (issues: %v)", got.Issues)
	}
	if !strings.Contains(strings.Join(got.Issues, " "), "smith") {
		t.Errorf("expected the issue to name the matched term, got %v", got.Issues)
	}
	// Short terms (<3 chars) must not trigger false positives.
	if r := ValidateWithContext(Password, "longenoughsecret", Context{Terms: []string{"ab"}}); !r.OK {
		t.Errorf("a 2-char context term should not reject (issues: %v)", r.Issues)
	}
}

func TestScoreMonotonicWithLength(t *testing.T) {
	short := Validate(Password, "ab1!cd2@").Score        // 8 chars, mixed
	long := Validate(Password, "ab1!cd2@ef3#gh4$").Score // 16 chars, mixed
	if long < short {
		t.Errorf("longer password scored lower (%d < %d)", long, short)
	}
	if long < 3 {
		t.Errorf("a 16-char mixed password should score at least 3, got %d", long)
	}
}

func TestStrongSecretsAreOKAndScored(t *testing.T) {
	r := Validate(Passphrase, "violet anchor meadow tunnel ridge")
	if !r.OK {
		t.Fatalf("strong passphrase rejected: %v", r.Issues)
	}
	if r.Score < 3 {
		t.Errorf("strong passphrase score = %d, want >= 3", r.Score)
	}
}

func TestUnknownKind(t *testing.T) {
	if r := Validate(Kind("totp"), "whatever"); r.OK {
		t.Error("unknown credential kind should not be OK")
	}
}
