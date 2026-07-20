// SPDX-License-Identifier: MIT

package smartengine

import (
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/smart"
)

// bl3MissedBill reported obvious payments as "missed" because it only looked for
// a payment LINKED to the liability account. In the sample dataset the July car
// and student-loan payments are autoposted from recurring rules into Joint
// Checking as plain categorized expenses — never tied back to the loan — so the
// engine flagged them missed and the assistant told the user to pay a debt that
// was already paid. These tests pin the fix against the exact sample shapes.

// checkingID is the funding account the sample-style unlinked payments land in.
const checkingID = "acct-checking"

// missedBillInput builds an Input dated a few days after the July due dates,
// holding a checking account plus whatever liability/transactions the case adds.
func missedBillInput(now time.Time, accounts []domain.Account, txns []domain.Transaction) Input {
	in := Input{Now: now, Base: "USD", Rates: currency.Rates{Base: "USD"}}
	in.Accounts = append([]domain.Account{
		{ID: checkingID, Name: "Joint Checking", Type: domain.TypeChecking,
			Class: domain.ClassAsset, Currency: "USD"},
	}, accounts...)
	in.Transactions = txns
	return in
}

// carLoan and studentLoan mirror the sample dataset's liabilities.
func carLoan() domain.Account {
	return domain.Account{
		ID: "acct-carloan", Name: "Marcus's Car Loan", Type: domain.TypeLoan,
		Class: domain.ClassLiability, Currency: "USD", DueDayOfMonth: 15,
		MinPayment: usd(62000), Lender: "Apex Auto Finance",
		OpeningBalance: usd(-3800000), InterestRateAPR: 7.4,
	}
}

func studentLoan() domain.Account {
	return domain.Account{
		ID: "acct-studentloan", Name: "Priya's Student Loan", Type: domain.TypeLoan,
		Class: domain.ClassLiability, Currency: "USD", DueDayOfMonth: 5,
		MinPayment: usd(32000), Lender: "EdFinance Servicing",
		OpeningBalance: usd(-3800000), InterestRateAPR: 5.5,
	}
}

// unlinkedPayment is a plain checking expense (no TransferAccountID) — the shape
// of an autoposted recurring bill or a hand-entered payment.
func unlinkedPayment(id, desc string, when time.Time, minor int64) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: checkingID, Date: when, Desc: desc, Amount: usd(minor)}
}

func TestBL3UnlinkedPaymentNotReportedMissed(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		loan     domain.Account
		payDay   int
		payDesc  string
		payMinor int64 // signed (negative = outflow)
		wantAmt  string
		wantAcct string
	}{
		{
			name:     "car loan paid from checking, unlinked",
			loan:     carLoan(),
			payDay:   15,
			payDesc:  "Car payment (Marcus)",
			payMinor: -62000,
			wantAmt:  "$620",
			wantAcct: "Joint Checking",
		},
		{
			name:     "student loan paid from checking, unlinked",
			loan:     studentLoan(),
			payDay:   5,
			payDesc:  "Student loan payment",
			payMinor: -32000,
			wantAmt:  "$320",
			wantAcct: "Joint Checking",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pay := unlinkedPayment("pay", c.payDesc,
				time.Date(2026, 7, c.payDay, 0, 0, 0, 0, time.UTC), c.payMinor)
			in := missedBillInput(now, []domain.Account{c.loan}, []domain.Transaction{pay})

			got := bl3MissedBill(in)
			ins, ok := findInsight(got, "SMART-BL3")
			if !ok {
				t.Fatalf("expected a SMART-BL3 insight, got %+v", got)
			}
			// It must NOT read as a missed/overdue alert.
			if ins.Severity == smart.SeverityAlert {
				t.Errorf("unlinked-but-present payment must not be an alert; got severity %v", ins.Severity)
			}
			if strings.Contains(strings.ToLower(ins.Title+" "+ins.Detail), "may have been missed") ||
				strings.Contains(strings.ToLower(ins.Detail), "no matching payment") {
				t.Errorf("must not claim the payment is missing.\n title:  %q\n detail: %q", ins.Title, ins.Detail)
			}
			// It must surface the matching payment as evidence: amount + account.
			if !strings.Contains(ins.Detail, c.wantAmt) {
				t.Errorf("detail must cite the matched amount %q; got %q", c.wantAmt, ins.Detail)
			}
			if !strings.Contains(ins.Detail, c.wantAcct) {
				t.Errorf("detail must name the account %q holding the payment; got %q", c.wantAcct, ins.Detail)
			}
		})
	}
}

func TestBL3LinkedPaymentSilent(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	// A two-legged transfer: the out-leg from checking carries TransferAccountID
	// to the loan (how the sample amortizes loans) — a linked payment.
	out := domain.Transaction{
		ID: "out", AccountID: checkingID, TransferAccountID: "acct-carloan",
		Date: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), Amount: usd(-62000), Desc: "Car payment (Marcus)",
	}
	in := missedBillInput(now, []domain.Account{carLoan()}, []domain.Transaction{out})
	if got := bl3MissedBill(in); len(got) != 0 {
		t.Fatalf("a payment linked to the loan must silence BL3 entirely; got %+v", got)
	}
}

func TestBL3GenuinelyMissedStillAlerts(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		txns []domain.Transaction
	}{
		{name: "no payment at all", txns: nil},
		{
			name: "look-alike name but wrong amount",
			txns: []domain.Transaction{unlinkedPayment("p", "Car payment (Marcus)",
				time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), -10000)}, // $100, not $620
		},
		{
			name: "right amount but unrelated payee",
			txns: []domain.Transaction{unlinkedPayment("p", "Groceries",
				time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), -62000)},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := missedBillInput(now, []domain.Account{carLoan()}, c.txns)
			ins, ok := findInsight(bl3MissedBill(in), "SMART-BL3")
			if !ok {
				t.Fatalf("expected a SMART-BL3 alert for a genuine miss")
			}
			if ins.Severity != smart.SeverityAlert {
				t.Errorf("a genuine miss must be an alert; got %v", ins.Severity)
			}
			if !strings.Contains(ins.Title, "may have been missed") {
				t.Errorf("genuine miss title should read 'may have been missed'; got %q", ins.Title)
			}
			// The alert must show its search (expected amount + window) as evidence.
			if !strings.Contains(ins.Detail, "$620") || !strings.Contains(ins.Detail, "day") {
				t.Errorf("alert detail must show the expected amount and the days searched; got %q", ins.Detail)
			}
		})
	}
}

func TestMatchBillPaymentAcrossCurrency(t *testing.T) {
	// The payment is booked in EUR from a travel card; the expected figure is in
	// USD base. amountsClose compares in base minor, so an FX-converted payment of
	// the right size still matches. Rate: 1 EUR = 1.10 USD.
	now := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.10}}
	loan := carLoan()
	in := Input{Now: now, Base: "USD", Rates: rates,
		Accounts: []domain.Account{
			{ID: checkingID, Name: "Joint Checking", Class: domain.ClassAsset, Currency: "USD"}, loan,
		},
		Transactions: []domain.Transaction{{
			ID: "eur", AccountID: checkingID, Desc: "Car payment (Marcus)",
			Date: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), Amount: money.New(-56400, "EUR"), // ~$620.40
		}},
	}
	expected := abs64(in.toBaseMinor(loan.MinPayment.Amount, loan.Currency))
	m := in.matchBillPayment(loan, expected,
		time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), now)
	if m.Linked || m.Candidate == nil {
		t.Fatalf("expected an unlinked cross-currency candidate; got %+v", m)
	}
}

func TestAmountsClose(t *testing.T) {
	cases := []struct {
		cand, exp int64
		want      bool
	}{
		{62000, 62000, true},  // exact
		{62500, 62000, true},  // within 5%
		{60000, 62000, true},  // within 5% (tol = 3100)
		{58000, 62000, false}, // 6.4% off
		{32050, 32000, true},  // within the $1 floor
		{31000, 32000, false}, // $10 off, above 5% of a small amount? 5% = 1600 -> 1000<=1600 true
	}
	// The last case: 5% of 32000 = 1600; |31000-32000| = 1000 <= 1600 → true.
	cases[len(cases)-1].want = true
	for _, c := range cases {
		if got := amountsClose(c.cand, c.exp); got != c.want {
			t.Errorf("amountsClose(%d, %d) = %v, want %v", c.cand, c.exp, got, c.want)
		}
	}
}

func TestBillNameTokensAndOverlap(t *testing.T) {
	cases := []struct {
		acctText string
		txnLabel string
		want     bool
	}{
		{"Marcus's Car Loan Apex Auto Finance", "Car payment (Marcus)", true},      // shares "marcus"
		{"Priya's Student Loan EdFinance Servicing", "Student loan payment", true}, // shares "student"
		{"Marcus's Car Loan", "Groceries", false},                                  // nothing shared
		{"Marcus's Car Loan", "Student loan payment", false},                       // only generic "loan", filtered
	}
	for _, c := range cases {
		got := tokensOverlap(billNameTokens(c.acctText), billNameTokens(c.txnLabel))
		if got != c.want {
			t.Errorf("overlap(%q, %q) = %v, want %v", c.acctText, c.txnLabel, got, c.want)
		}
	}
}
