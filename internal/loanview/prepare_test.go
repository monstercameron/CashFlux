package loanview

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPrepareLoanSchedulesAndExactExtra(t *testing.T) {
	a := domain.Account{ID: "loan", Name: "Loan", Currency: "USD", Type: domain.TypeLoan, TermMonths: 12, InterestRateAPR: ptr(0)}
	in := Input{Account: a, Balance: money.New(120000, "USD"), TermMonths: 12, ExtraText: "12.34", Now: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	r := Prepare(in)
	if !r.BalanceKnown || r.ExtraErr != nil || r.ExtraMinor != 1234 {
		t.Fatalf("bad parse/facts: %+v", r)
	}
	if len(r.BaseRows) != 12 || len(r.ExtraRows) == 0 || r.MonthsSaved <= 0 || r.InterestSaved < 0 {
		t.Fatalf("bad schedules: %+v", r)
	}
}

func TestPrepareLoanInputAndBalanceErrors(t *testing.T) {
	a := domain.Account{Currency: "JPY", Type: domain.TypeMortgage, InterestRateAPR: ptr(0)}
	for _, tc := range []struct {
		name, extra  string
		term         int
		balanceErr   error
		wantExtraErr bool
	}{
		{name: "excess precision", extra: "1.5", term: 12, wantExtraErr: true},
		{name: "negative", extra: "-1", term: 12, wantExtraErr: true},
		{name: "bad term", extra: "1", term: 1201},
		{name: "balance failure", extra: "1", term: 12, balanceErr: errors.New("bad balance")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Prepare(Input{Account: a, Balance: money.New(1000, "JPY"), BalanceErr: tc.balanceErr, TermMonths: tc.term, ExtraText: tc.extra, Now: time.Now()})
			if tc.wantExtraErr && r.ExtraErr == nil {
				t.Fatalf("input error hidden: %+v", r)
			}
			if tc.name == "bad term" && r.TermErr == nil {
				t.Fatalf("term error missing")
			}
			if tc.name == "balance failure" && (r.BalanceKnown || len(r.BaseRows) != 0) {
				t.Fatalf("balance failure became schedule: %+v", r)
			}
		})
	}
}
func ptr(v float64) *float64 { return &v }
