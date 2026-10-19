package debtpay

import (
	"math"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	date := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	checking := Account{ID: "checking", Name: "Checking", Currency: "USD", ScopeID: "household"}
	loan := Account{ID: "loan", Name: "Harbor Loan", Lender: "Harbor", Currency: "USD", Liability: true, Kind: AccountLoan, ScopeID: "household"}
	baseTxn := Transaction{ID: "debit", AccountID: checking.ID, Date: date, Payee: "Harbor AutoPay", AmountMinor: -25000, Currency: "USD", ScopeID: "household", MemberID: "member-1"}

	tests := []struct {
		name            string
		txn             Transaction
		data            Dataset
		wantAccount     string
		wantConfidence  Confidence
		wantCounterpart string
	}{
		{name: "reciprocal liability row is strong", txn: baseTxn, data: Dataset{Accounts: []Account{checking, loan}, Transactions: []Transaction{{ID: "credit", AccountID: loan.ID, Date: date.AddDate(0, 0, 2), AmountMinor: 25000, Currency: "USD", ScopeID: "household"}}}, wantAccount: loan.ID, wantConfidence: ConfidenceStrong, wantCounterpart: "credit"},
		{name: "confirmed payee rule is strong", txn: baseTxn, data: Dataset{Accounts: []Account{checking, loan}, PayeeRules: []PayeeRule{{Payee: "harbor autopay", DebtAccountID: loan.ID, ScopeID: "household"}}}, wantAccount: loan.ID, wantConfidence: ConfidenceStrong},
		{name: "bill account is strong attribution", txn: withBill(baseTxn, loan.ID), data: Dataset{Accounts: []Account{checking, loan}}, wantAccount: loan.ID, wantConfidence: ConfidenceStrong},
		{name: "descriptor and recurring amount date are medium", txn: baseTxn, data: Dataset{Accounts: []Account{checking, loan}, Recurring: []RecurringPayment{{ID: "monthly", DebtAccountID: loan.ID, AmountMinor: 25000, Currency: "USD", NextDue: date.AddDate(0, 0, 1), ScopeID: "household"}}}, wantAccount: loan.ID, wantConfidence: ConfidenceMedium},
		{name: "refund to liability is ignored", txn: func() Transaction { x := baseTxn; x.AccountID = loan.ID; x.AmountMinor = 25000; return x }(), data: Dataset{Accounts: []Account{checking, loan}}},
		{name: "liability to liability balance transfer is ignored", txn: func() Transaction { x := baseTxn; x.AccountID = loan.ID; x.AmountMinor = -25000; return x }(), data: Dataset{Accounts: []Account{loan, {ID: "card", Name: "Card", Currency: "USD", Liability: true, ScopeID: "household"}}}},
		{name: "multi-currency rule cannot auto-match", txn: baseTxn, data: Dataset{Accounts: []Account{checking, {ID: loan.ID, Name: loan.Name, Currency: "EUR", Liability: true, ScopeID: "household"}}, PayeeRules: []PayeeRule{{Payee: baseTxn.Payee, DebtAccountID: loan.ID, ScopeID: "household"}}}},
		{name: "already linked transfer is ignored", txn: func() Transaction { x := baseTxn; x.TransferGroupID = "group"; return x }(), data: Dataset{Accounts: []Account{checking, loan}, PayeeRules: []PayeeRule{{Payee: baseTxn.Payee, DebtAccountID: loan.ID, ScopeID: "household"}}}},
		{name: "scope mismatch is ignored", txn: baseTxn, data: Dataset{Accounts: []Account{checking, {ID: loan.ID, Name: loan.Name, Currency: "USD", Liability: true, ScopeID: "another-household"}}, PayeeRules: []PayeeRule{{Payee: baseTxn.Payee, DebtAccountID: loan.ID, ScopeID: "another-household"}}}},
		{name: "equal evidence across debts is ambiguous", txn: baseTxn, data: Dataset{Accounts: []Account{checking, loan, {ID: "loan2", Name: "Harbor loan 2", Currency: "USD", Liability: true, ScopeID: "household"}}, PayeeRules: []PayeeRule{{Payee: baseTxn.Payee, DebtAccountID: loan.ID, ScopeID: "household"}, {Payee: baseTxn.Payee, DebtAccountID: "loan2", ScopeID: "household"}}}},
		{name: "untracked debt has no hidden class", txn: baseTxn, data: Dataset{Accounts: []Account{checking}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.txn, tt.data)
			wantConfidence := tt.wantConfidence
			if wantConfidence == "" {
				wantConfidence = ConfidenceNone
			}
			if got.TxnID != tt.txn.ID || got.TargetDebtAccountID != tt.wantAccount || got.Confidence != wantConfidence || got.ExistingCounterpartID != tt.wantCounterpart || got.MemberID != tt.txn.MemberID {
				t.Fatalf("Classify() = %#v", got)
			}
		})
	}
}

func TestClassifyCounterpartRequiresExactSameCurrencyAndUniqueRow(t *testing.T) {
	date := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	txn := Transaction{ID: "debit", AccountID: "checking", Date: date, AmountMinor: -10000, Currency: "USD", ScopeID: "s"}
	debt := Account{ID: "debt", Currency: "USD", Liability: true, ScopeID: "s"}
	rows := []Transaction{
		{ID: "fx", AccountID: "debt", Date: date, AmountMinor: 10000, Currency: "EUR", ScopeID: "s"},
		{ID: "late", AccountID: "debt", Date: date.AddDate(0, 0, 8), AmountMinor: 10000, Currency: "USD", ScopeID: "s"},
		{ID: "one", AccountID: "debt", Date: date, AmountMinor: 10000, Currency: "USD", ScopeID: "s"},
		{ID: "two", AccountID: "debt", Date: date, AmountMinor: 10000, Currency: "USD", ScopeID: "s"},
	}
	got := Classify(txn, Dataset{Accounts: []Account{{ID: "checking", Currency: "USD", ScopeID: "s"}, debt}, Transactions: rows})
	if got.Confidence != ConfidenceNone {
		t.Fatalf("ambiguous counterpart should not suggest a winner: %#v", got)
	}
}

func TestAllocate(t *testing.T) {
	tests := []struct {
		name    string
		payment Payment
		loan    Loan
		split   *Split
		want    Allocation
		wantErr error
	}{
		{name: "statement split with escrow", payment: Payment{AmountMinor: 12000}, loan: Loan{BalanceMinor: 100000}, split: &Split{PrincipalMinor: 8000, InterestMinor: 3000, FeesMinor: 500, EscrowMinor: 500}, want: Allocation{PaymentMinor: 12000, PrincipalMinor: 8000, InterestMinor: 3000, FeesMinor: 500, EscrowMinor: 500, Method: AllocationStatement}},
		{name: "statement split overrides estimate", payment: Payment{AmountMinor: 10000}, loan: Loan{BalanceMinor: 100000, Kind: AccountLoan, APRBasisPoints: 1200, HasAPR: true}, split: &Split{PrincipalMinor: 9000, InterestMinor: 1000}, want: Allocation{PaymentMinor: 10000, PrincipalMinor: 9000, InterestMinor: 1000, Method: AllocationStatement}},
		{name: "monthly interest estimate", payment: Payment{AmountMinor: 10000}, loan: Loan{Kind: AccountLoan, BalanceMinor: 100000, APRBasisPoints: 1200, HasAPR: true}, want: Allocation{PaymentMinor: 10000, PrincipalMinor: 9000, InterestMinor: 1000, Estimated: true, Method: AllocationAmortized}},
		{name: "bankers rounding even tie rounds down", payment: Payment{AmountMinor: 1}, loan: Loan{Kind: AccountLoan, BalanceMinor: 50, APRBasisPoints: 1200, HasAPR: true}, want: Allocation{PaymentMinor: 1, PrincipalMinor: 1, Estimated: true, Method: AllocationAmortized}},
		{name: "bankers rounding odd tie rounds up", payment: Payment{AmountMinor: 2}, loan: Loan{Kind: AccountLoan, BalanceMinor: 150, APRBasisPoints: 1200, HasAPR: true}, want: Allocation{PaymentMinor: 2, PrincipalMinor: 0, InterestMinor: 2, Estimated: true, Method: AllocationAmortized}},
		{name: "card payment is principal", payment: Payment{AmountMinor: 5000}, loan: Loan{Kind: AccountCard, BalanceMinor: 4000}, want: Allocation{PaymentMinor: 5000, PrincipalMinor: 5000, Method: AllocationCard, Overpayment: true, OverpaymentMinor: 1000}},
		{name: "loan overpayment is kept", payment: Payment{AmountMinor: 5000}, loan: Loan{Kind: AccountLoan, BalanceMinor: 1000, APRBasisPoints: 0, HasAPR: true}, want: Allocation{PaymentMinor: 5000, PrincipalMinor: 5000, Estimated: true, Method: AllocationAmortized, Overpayment: true, OverpaymentMinor: 4000}},
		{name: "large amount uses exact arithmetic", payment: Payment{AmountMinor: math.MaxInt64}, loan: Loan{Kind: AccountLoan, BalanceMinor: math.MaxInt64, APRBasisPoints: 1200, HasAPR: true}, want: Allocation{PaymentMinor: math.MaxInt64, PrincipalMinor: math.MaxInt64 - 92233720368547758, InterestMinor: 92233720368547758, Estimated: true, Method: AllocationAmortized}},
		{name: "invalid statement sum", payment: Payment{AmountMinor: 100}, loan: Loan{Kind: AccountLoan, HasAPR: true}, split: &Split{PrincipalMinor: 99}, wantErr: ErrInvalidSplit},
		{name: "negative statement component", payment: Payment{AmountMinor: 100}, loan: Loan{}, split: &Split{PrincipalMinor: 101, FeesMinor: -1}, wantErr: ErrInvalidSplit},
		{name: "loan without rate cannot estimate", payment: Payment{AmountMinor: 100}, loan: Loan{Kind: AccountLoan}, wantErr: ErrInvalidLoan},
		{name: "zero payment rejected", payment: Payment{}, loan: Loan{Kind: AccountCard}, wantErr: ErrInvalidPayment},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Allocate(tt.payment, tt.loan, tt.split)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("Allocate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Allocate() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Allocate() = %#v, want %#v", got, tt.want)
			}
			if got.PrincipalMinor+got.InterestMinor+got.FeesMinor+got.EscrowMinor != got.PaymentMinor {
				t.Fatalf("allocation does not preserve payment: %#v", got)
			}
		})
	}
}

func TestAllocatePropertyPreservesPayment(t *testing.T) {
	for balance := int64(0); balance < 10000; balance += 137 {
		for rate := int64(0); rate <= 3000; rate += 113 {
			for payment := int64(1); payment < 5000; payment += 127 {
				got, err := Allocate(Payment{AmountMinor: payment}, Loan{Kind: AccountLoan, BalanceMinor: balance, APRBasisPoints: rate, HasAPR: true}, nil)
				if err != nil {
					t.Fatal(err)
				}
				if sum := got.PrincipalMinor + got.InterestMinor + got.FeesMinor + got.EscrowMinor; sum != payment {
					t.Fatalf("balance=%d rate=%d payment=%d sum=%d", balance, rate, payment, sum)
				}
			}
		}
	}
}

func TestAggregate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	periods := []Period{{ID: "jan", Start: start, End: start.AddDate(0, 1, 0)}, {ID: "feb", Start: start.AddDate(0, 1, 0), End: start.AddDate(0, 2, 0)}}
	payments := []ClassifiedPayment{
		{TxnID: "a", Date: start, Currency: "USD", Allocation: Allocation{PaymentMinor: 12000, PrincipalMinor: 9000, InterestMinor: 2500, FeesMinor: 500}},
		{TxnID: "b", Date: start.AddDate(0, 0, 30), Currency: "EUR", Allocation: Allocation{PaymentMinor: 5000, PrincipalMinor: 4000, InterestMinor: 1000}},
		{TxnID: "c", Date: start.AddDate(0, 1, 0), Currency: "USD", Allocation: Allocation{PaymentMinor: 7000, PrincipalMinor: 6500, InterestMinor: 500}},
	}
	got, err := Aggregate(payments, periods)
	if err != nil {
		t.Fatal(err)
	}
	want := []PeriodTotals{
		{PeriodID: "jan", Currency: "EUR", CashPaidMinor: 5000, PrincipalMinor: 4000, InterestMinor: 1000},
		{PeriodID: "jan", Currency: "USD", CashPaidMinor: 12000, PrincipalMinor: 9000, InterestMinor: 2500, FeesMinor: 500},
		{PeriodID: "feb", Currency: "USD", CashPaidMinor: 7000, PrincipalMinor: 6500, InterestMinor: 500},
	}
	if !equalTotals(got, want) {
		t.Fatalf("Aggregate() = %#v, want %#v", got, want)
	}
}

func TestAggregateRejectsOverflowAndInvalidPeriod(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	period := Period{ID: "p", Start: date, End: date.AddDate(0, 1, 0)}
	_, err := Aggregate(nil, []Period{{ID: "bad", Start: date, End: date}})
	if err != ErrInvalidPeriod {
		t.Fatalf("Aggregate() error = %v, want %v", err, ErrInvalidPeriod)
	}
	_, err = Aggregate([]ClassifiedPayment{
		{Date: date, Currency: "USD", Allocation: Allocation{PaymentMinor: math.MaxInt64}},
		{Date: date, Currency: "USD", Allocation: Allocation{PaymentMinor: 1}},
	}, []Period{period})
	if err == nil {
		t.Fatal("Aggregate() accepted overflowing total")
	}
	_, err = Aggregate([]ClassifiedPayment{{TxnID: "bad", Date: date, Currency: "USD", Allocation: Allocation{PaymentMinor: 5, PrincipalMinor: 4}}}, []Period{period})
	if err == nil {
		t.Fatal("Aggregate() accepted an allocation that does not preserve cash")
	}
}

func withBill(txn Transaction, accountID string) Transaction {
	txn.BillAccountID = accountID
	return txn
}

func equalTotals(a, b []PeriodTotals) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
