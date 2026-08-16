// SPDX-License-Identifier: MIT

package amountmath

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestParseSigned(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		decimals     int
		want         Direction
		wantMinor    int64
		wantResolved Direction
	}{
		// The ordinary path: magnitude typed, direction control decides.
		{"plain expense", "50", 2, Out, -5000, Out},
		{"plain income", "50", 2, In, 5000, In},
		{"decimal expense", "45.99", 2, Out, -4599, Out},
		{"decimal income", "45.99", 2, In, 4599, In},
		{"whitespace is trimmed", "  12.34  ", 2, Out, -1234, Out},
		{"zero-decimal currency", "500", 0, Out, -500, Out},

		// C546: a typed minus must never be flipped into its opposite. These are
		// the exact cases that stored +50 income when the user typed -50 expense.
		{"typed minus with out stays out", "-50", 2, Out, -5000, Out},
		{"typed minus with in becomes out", "-50", 2, In, -5000, Out},
		{"typed minus decimal with in becomes out", "-45.99", 2, In, -4599, Out},
		{"typed minus zero-decimal with in becomes out", "-500", 0, In, -500, Out},

		// An explicit plus is how people write a positive number; it is not an
		// assertion of income, so it must not override the control.
		{"typed plus with out stays out", "+50", 2, Out, -5000, Out},
		{"typed plus with in stays in", "+50", 2, In, 5000, In},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minor, resolved, err := ParseSigned(tt.in, tt.decimals, tt.want)
			if err != nil {
				t.Fatalf("ParseSigned(%q, %d, %v) unexpected error: %v", tt.in, tt.decimals, tt.want, err)
			}
			if minor != tt.wantMinor {
				t.Errorf("ParseSigned(%q, %d, %v) minor = %d, want %d", tt.in, tt.decimals, tt.want, minor, tt.wantMinor)
			}
			if resolved != tt.wantResolved {
				t.Errorf("ParseSigned(%q, %d, %v) resolved = %v, want %v", tt.in, tt.decimals, tt.want, resolved, tt.wantResolved)
			}
		})
	}
}

// TestParseSignedNeverInvertsTypedIntent is the regression guard for C546 stated
// as a property rather than as examples: whatever the direction control says, an
// amount typed with a minus sign can only ever be stored negative.
func TestParseSignedNeverInvertsTypedIntent(t *testing.T) {
	for _, want := range []Direction{Out, In} {
		for _, in := range []string{"-1", "-0.01", "-50", "-1234.56", "  -7  "} {
			minor, resolved, err := ParseSigned(in, 2, want)
			if err != nil {
				t.Fatalf("ParseSigned(%q, 2, %v) unexpected error: %v", in, want, err)
			}
			if minor >= 0 {
				t.Errorf("ParseSigned(%q, 2, %v) = %d, want a negative amount — a typed minus was inverted", in, want, minor)
			}
			if resolved != Out {
				t.Errorf("ParseSigned(%q, 2, %v) resolved = %v, want Out so the control can follow", in, want, resolved)
			}
		}
	}
}

func TestParseSignedRejectsZero(t *testing.T) {
	for _, in := range []string{"0", "0.00", "-0.00", "+0", "  0  "} {
		_, _, err := ParseSigned(in, 2, Out)
		if !errors.Is(err, ErrZeroAmount) {
			t.Errorf("ParseSigned(%q) error = %v, want ErrZeroAmount", in, err)
		}
	}
}

func TestParseSignedRejectsJunk(t *testing.T) {
	for _, in := range []string{"", "   ", "abc", "1.2.3", "5*", "--5", "1,234"} {
		_, _, err := ParseSigned(in, 2, Out)
		if err == nil {
			t.Errorf("ParseSigned(%q) = nil error, want a rejection", in)
			continue
		}
		if errors.Is(err, ErrZeroAmount) {
			t.Errorf("ParseSigned(%q) reported a zero amount, want an invalid one", in)
			continue
		}
		if !errors.Is(err, money.ErrInvalidAmount) {
			t.Errorf("ParseSigned(%q) error = %v, want it to wrap money.ErrInvalidAmount", in, err)
		}
	}
}

// TestParseSignedFailureKeepsTheRequestedDirection matters for the UI: on a
// rejected save the control must not jump, or the user fixes the number and
// silently gets the other direction.
func TestParseSignedFailureKeepsTheRequestedDirection(t *testing.T) {
	for _, want := range []Direction{Out, In} {
		for _, in := range []string{"", "abc", "0"} {
			_, resolved, err := ParseSigned(in, 2, want)
			if err == nil {
				t.Fatalf("ParseSigned(%q, 2, %v) unexpectedly succeeded", in, want)
			}
			if resolved != want {
				t.Errorf("ParseSigned(%q, 2, %v) resolved = %v on failure, want the direction unchanged", in, want, resolved)
			}
		}
	}
}

func TestDirectionRoundTrip(t *testing.T) {
	tests := []struct {
		in   string
		want Direction
	}{
		{"out", Out},
		{"in", In},
		{"IN", In},
		{"  in  ", In},
		{"", Out},
		{"anything else", Out},
	}
	for _, tt := range tests {
		if got := DirectionFrom(tt.in); got != tt.want {
			t.Errorf("DirectionFrom(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	for _, d := range []Direction{Out, In} {
		if got := DirectionFrom(d.String()); got != d {
			t.Errorf("DirectionFrom(%v.String()) = %v, want %v", d, got, d)
		}
	}
	if Out.String() != "out" || In.String() != "in" {
		t.Errorf("Direction.String() = %q/%q, want \"out\"/\"in\" — the forms persist these tokens", Out.String(), In.String())
	}
}
