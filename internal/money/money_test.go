package money

import (
	"errors"
	"testing"
)

func TestNewNormalizesCurrency(t *testing.T) {
	m := New(100, " usd ")
	if m.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", m.Currency)
	}
	if m.Amount != 100 {
		t.Fatalf("amount = %d, want 100", m.Amount)
	}
}

func TestPredicates(t *testing.T) {
	tests := []struct {
		name           string
		m              Money
		zero, pos, neg bool
	}{
		{"zero", New(0, "USD"), true, false, false},
		{"positive", New(5, "USD"), false, true, false},
		{"negative", New(-5, "USD"), false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.m.IsZero() != tt.zero {
				t.Errorf("IsZero = %v, want %v", tt.m.IsZero(), tt.zero)
			}
			if tt.m.IsPositive() != tt.pos {
				t.Errorf("IsPositive = %v, want %v", tt.m.IsPositive(), tt.pos)
			}
			if tt.m.IsNegative() != tt.neg {
				t.Errorf("IsNegative = %v, want %v", tt.m.IsNegative(), tt.neg)
			}
		})
	}
}

func TestNegAndAbs(t *testing.T) {
	if got := New(-250, "EUR").Neg(); !got.Equal(New(250, "EUR")) {
		t.Errorf("Neg = %v, want 250 EUR", got)
	}
	if got := New(-250, "EUR").Abs(); !got.Equal(New(250, "EUR")) {
		t.Errorf("Abs(neg) = %v, want 250 EUR", got)
	}
	if got := New(250, "EUR").Abs(); !got.Equal(New(250, "EUR")) {
		t.Errorf("Abs(pos) = %v, want 250 EUR", got)
	}
}

func TestAddSub(t *testing.T) {
	tests := []struct {
		name string
		a, b Money
		add  int64
		sub  int64
	}{
		{"simple", New(100, "USD"), New(50, "USD"), 150, 50},
		{"with negative", New(100, "USD"), New(-30, "USD"), 70, 130},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum, err := tt.a.Add(tt.b)
			if err != nil {
				t.Fatalf("Add error: %v", err)
			}
			if sum.Amount != tt.add {
				t.Errorf("Add amount = %d, want %d", sum.Amount, tt.add)
			}
			diff, err := tt.a.Sub(tt.b)
			if err != nil {
				t.Fatalf("Sub error: %v", err)
			}
			if diff.Amount != tt.sub {
				t.Errorf("Sub amount = %d, want %d", diff.Amount, tt.sub)
			}
		})
	}
}

func TestCurrencyMismatch(t *testing.T) {
	a, b := New(100, "USD"), New(100, "EUR")
	if _, err := a.Add(b); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Add err = %v, want ErrCurrencyMismatch", err)
	}
	if _, err := a.Sub(b); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Sub err = %v, want ErrCurrencyMismatch", err)
	}
	if _, err := a.Cmp(b); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Cmp err = %v, want ErrCurrencyMismatch", err)
	}
}

func TestCmp(t *testing.T) {
	tests := []struct {
		a, b Money
		want int
	}{
		{New(1, "USD"), New(2, "USD"), -1},
		{New(2, "USD"), New(2, "USD"), 0},
		{New(3, "USD"), New(2, "USD"), 1},
	}
	for _, tt := range tests {
		got, err := tt.a.Cmp(tt.b)
		if err != nil {
			t.Fatalf("Cmp error: %v", err)
		}
		if got != tt.want {
			t.Errorf("Cmp(%v,%v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSum(t *testing.T) {
	if got, _ := Sum(); !got.Equal(Money{}) {
		t.Errorf("Sum() = %v, want zero value", got)
	}
	got, err := Sum(New(10, "USD"), New(20, "USD"), New(-5, "USD"))
	if err != nil {
		t.Fatalf("Sum error: %v", err)
	}
	if !got.Equal(New(25, "USD")) {
		t.Errorf("Sum = %v, want 25 USD", got)
	}
	if _, err := Sum(New(10, "USD"), New(20, "EUR")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("Sum mismatch err = %v, want ErrCurrencyMismatch", err)
	}
}
