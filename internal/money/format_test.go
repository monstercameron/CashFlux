package money

import (
	"errors"
	"testing"
)

func TestFormatMinor(t *testing.T) {
	tests := []struct {
		amount   int64
		decimals int
		want     string
	}{
		{-24055, 2, "-240.55"},
		{5, 2, "0.05"},
		{1000, 2, "10.00"},
		{0, 2, "0.00"},
		{1500, 0, "1500"},
		{-7, 0, "-7"},
		{123456, 2, "1234.56"},
	}
	for _, tt := range tests {
		if got := FormatMinor(tt.amount, tt.decimals); got != tt.want {
			t.Errorf("FormatMinor(%d, %d) = %q, want %q", tt.amount, tt.decimals, got, tt.want)
		}
	}
}

func TestParseMinor(t *testing.T) {
	tests := []struct {
		s        string
		decimals int
		want     int64
	}{
		{"240.55", 2, 24055},
		{"-240.55", 2, -24055},
		{"+10", 2, 1000},
		{"0.5", 2, 50},
		{".5", 2, 50},
		{"150", 0, 150},
		{"1234.56", 2, 123456},
		{"  12.30 ", 2, 1230},
	}
	for _, tt := range tests {
		got, err := ParseMinor(tt.s, tt.decimals)
		if err != nil {
			t.Errorf("ParseMinor(%q, %d) error: %v", tt.s, tt.decimals, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseMinor(%q, %d) = %d, want %d", tt.s, tt.decimals, got, tt.want)
		}
	}
}

func TestParseMinorErrors(t *testing.T) {
	for _, s := range []string{"", "abc", "1.234", "12.3.4", "1,234.00", "--5"} {
		if _, err := ParseMinor(s, 2); !errors.Is(err, ErrInvalidAmount) {
			t.Errorf("ParseMinor(%q) err = %v, want ErrInvalidAmount", s, err)
		}
	}
	if _, err := ParseMinor("150.5", 0); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("ParseMinor with too many decimals should error, got %v", err)
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	for _, amt := range []int64{0, 1, -1, 99, 100, -24055, 1234567} {
		s := FormatMinor(amt, 2)
		back, err := ParseMinor(s, 2)
		if err != nil {
			t.Fatalf("round trip parse %q: %v", s, err)
		}
		if back != amt {
			t.Errorf("round trip %d -> %q -> %d", amt, s, back)
		}
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"0.00", "0.00"},
		{"12.50", "12.50"},
		{"123.45", "123.45"},
		{"1234.56", "1,234.56"},
		{"1234567.89", "1,234,567.89"},
		{"-1234567", "-1,234,567"},
		{"1000", "1,000"},
		{"100000", "100,000"},
	}
	for _, tt := range tests {
		if got := Group(tt.in); got != tt.want {
			t.Errorf("Group(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatAccounting(t *testing.T) {
	tests := []struct {
		amount   int64
		decimals int
		symbol   string
		want     string
	}{
		{123456, 2, "$", "$1,234.56"},
		{-24055, 2, "$", "($240.55)"},
		{0, 2, "$", "$0.00"},
		{5, 2, "$", "$0.05"},
		{-100000000, 2, "$", "($1,000,000.00)"},
		{150000, 0, "¥", "¥150,000"},
		{-7, 0, "€", "(€7)"},
	}
	for _, tt := range tests {
		if got := FormatAccounting(tt.amount, tt.decimals, tt.symbol); got != tt.want {
			t.Errorf("FormatAccounting(%d, %d, %q) = %q, want %q", tt.amount, tt.decimals, tt.symbol, got, tt.want)
		}
	}
}

func TestMoneyFormatMethod(t *testing.T) {
	if got := New(-24055, "USD").Format(2); got != "-240.55" {
		t.Errorf("Money.Format = %q, want -240.55", got)
	}
}
