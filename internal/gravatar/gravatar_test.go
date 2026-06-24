// SPDX-License-Identifier: MIT

package gravatar

import (
	"strings"
	"testing"
)

func TestHash(t *testing.T) {
	tests := []struct {
		email string
		want  string
	}{
		// The canonical Gravatar documentation example.
		{"MyEmailAddress@example.com ", "0bc83cb571cd1c50ba6f3e8a78ef1346"},
		// Trimming + lowercasing make these all equal to the example.
		{"  myemailaddress@example.com", "0bc83cb571cd1c50ba6f3e8a78ef1346"},
		{"MYEMAILADDRESS@EXAMPLE.COM", "0bc83cb571cd1c50ba6f3e8a78ef1346"},
		// A plain MD5 vector independent of email casing.
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
	}
	for _, tc := range tests {
		if got := Hash(tc.email); got != tc.want {
			t.Errorf("Hash(%q) = %q, want %q", tc.email, got, tc.want)
		}
	}
}

func TestURL(t *testing.T) {
	const h = "0bc83cb571cd1c50ba6f3e8a78ef1346"
	if got := URL("myemailaddress@example.com", 0); got != base+h+"?s=80&d=identicon" {
		t.Errorf("default-size URL = %q", got)
	}
	if got := URL("myemailaddress@example.com", 128); got != base+h+"?s=128&d=identicon" {
		t.Errorf("sized URL = %q", got)
	}
	// Clamping.
	if got := URL("x@y.com", 9999); !strings.Contains(got, "?s=2048&") {
		t.Errorf("oversize should clamp to 2048: %q", got)
	}
	if got := URL("x@y.com", -5); !strings.Contains(got, "?s=80&") {
		t.Errorf("negative size should use default 80: %q", got)
	}
	if !strings.Contains(URL("x@y.com", 80), "&d=identicon") {
		t.Error("URL should carry the identicon fallback")
	}
}
