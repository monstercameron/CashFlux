package currency

import (
	"errors"
	"testing"
)

func TestConvertBetween(t *testing.T) {
	rates := Rates{
		Base:  "USD",
		Rates: map[string]float64{"EUR": 1.10, "GBP": 1.25},
	}

	tests := []struct {
		name    string
		amt     int64
		from    string
		to      string
		want    int64
		wantErr bool
		errIs   error
	}{
		{
			name: "same currency is identity",
			amt:  5000, from: "USD", to: "USD",
			want: 5000,
		},
		{
			name: "same currency EUR identity",
			amt:  9999, from: "EUR", to: "EUR",
			want: 9999,
		},
		{
			// 50.00 USD → (÷1.10) 45.45 EUR, rounds to 4545
			name: "USD to EUR cross-currency",
			amt:  5000, from: "USD", to: "EUR",
			want: 4545,
		},
		{
			// 50.00 EUR → 55.00 USD → 44.00 GBP  (5000 EUR cents)
			// 50 EUR × 1.10 = 55 USD ÷ 1.25 = 44.00 GBP = 4400 GBP cents
			name: "EUR to GBP via base",
			amt:  5000, from: "EUR", to: "GBP",
			want: 4400,
		},
		{
			name: "missing source rate returns error",
			amt:  100, from: "JPY", to: "USD",
			wantErr: true, errIs: ErrUnknownRate,
		},
		{
			name: "missing target rate returns error",
			amt:  100, from: "USD", to: "JPY",
			wantErr: true, errIs: ErrUnknownRate,
		},
		{
			name: "negative amount same currency is identity",
			amt:  -1000, from: "USD", to: "USD",
			want: -1000,
		},
		{
			// 10.00 GBP → 12.50 USD → 11.36 EUR (rounds)
			name: "GBP to EUR cross-currency",
			amt:  1000, from: "GBP", to: "EUR",
			want: 1136,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertBetween(tt.amt, tt.from, tt.to, rates)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ConvertBetween(%d, %q, %q) expected error, got nil", tt.amt, tt.from, tt.to)
				}
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Errorf("error = %v, want errors.Is(%v)", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertBetween(%d, %q, %q) unexpected error: %v", tt.amt, tt.from, tt.to, err)
			}
			if got != tt.want {
				t.Errorf("ConvertBetween(%d, %q, %q) = %d, want %d", tt.amt, tt.from, tt.to, got, tt.want)
			}
		})
	}
}
