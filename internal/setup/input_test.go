package setup

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareIncome(t *testing.T) {
	tests := []struct {
		name, raw, currency string
		wantSet             bool
		wantMinor           int64
		wantErr             bool
	}{
		{"usd cents", "123.45", "USD", true, 12345, false},
		{"yen whole units", "12345", "JPY", true, 12345, false},
		{"blank skip", "  ", "USD", false, 0, false},
		{"excess usd precision", "1.001", "USD", false, 0, true},
		{"fractional yen", "1.1", "JPY", false, 0, true},
		{"negative", "-1.00", "USD", false, 0, true},
		{"overflow", "92233720368547758.08", "USD", false, 0, true},
		{"bad currency", "1", "XXX", false, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareIncome(tt.raw, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PrepareIncome() error = %v, wantErr %t", err, tt.wantErr)
			}
			if got.Set != tt.wantSet || got.MonthlyIncomeMinor != tt.wantMinor {
				t.Fatalf("PrepareIncome() = %+v, want Set=%t minor=%d", got, tt.wantSet, tt.wantMinor)
			}
		})
	}
}

func TestPrepareAccount(t *testing.T) {
	tests := []struct {
		name, accountName, raw, currency string
		typeOf                           domain.AccountType
		wantMinor                        int64
		wantClass                        domain.AccountClass
		wantErr                          bool
	}{
		{"asset exact cents", " Wallet ", "100.01", "USD", domain.TypeChecking, 10001, domain.ClassAsset, false},
		{"liability positive owed", "Card", "250", "USD", domain.TypeCreditCard, 25000, domain.ClassLiability, false},
		{"jpy", "Cash", "1234", "JPY", domain.TypeCash, 1234, domain.ClassAsset, false},
		{"blank balance zero", "Cash", "", "USD", domain.TypeCash, 0, domain.ClassAsset, false},
		{"blank name", " ", "1", "USD", domain.TypeChecking, 0, "", true},
		{"invalid amount", "Cash", "bad", "USD", domain.TypeCash, 0, "", true},
		{"negative amount", "Cash", "-1", "USD", domain.TypeCash, 0, "", true},
		{"excess precision", "Cash", "1.001", "USD", domain.TypeCash, 0, "", true},
		{"overflow", "Cash", "92233720368547758.08", "USD", domain.TypeCash, 0, "", true},
		{"invalid currency", "Cash", "1", "XXX", domain.TypeCash, 0, "", true},
		{"invalid type", "Cash", "1", "USD", "unknown", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrepareAccount(tt.accountName, tt.raw, tt.currency, tt.typeOf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("PrepareAccount() error = %v, wantErr %t", err, tt.wantErr)
			}
			if tt.wantErr {
				if got != (AccountPlan{}) {
					t.Fatalf("error returned partial account plan: %+v", got)
				}
				return
			}
			if got.Name == "" || got.OpeningBalance.Amount != tt.wantMinor || got.OpeningBalance.Currency != tt.currency || got.Class != tt.wantClass {
				t.Fatalf("PrepareAccount() = %+v", got)
			}
		})
	}
}

func TestPrepareInputErrorsAreClassifiable(t *testing.T) {
	if _, err := PrepareIncome("-1", "USD"); !errors.Is(err, ErrNegativeAmount) {
		t.Fatalf("negative income error = %v", err)
	}
	if _, err := PrepareIncome("92233720368547758.08", "USD"); !errors.Is(err, money.ErrOverflow) {
		t.Fatalf("overflow income error = %v", err)
	}
	if _, err := PrepareAccount("Card", "1", "USD", domain.TypeCreditCard); err != nil {
		t.Fatal(err)
	}
}
