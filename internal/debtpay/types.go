// Package debtpay classifies possible debt payments and allocates their amounts
// without performing persistence or changing transactions.
package debtpay

import "time"

// AccountKind identifies debt-account behavior relevant to payment allocation.
type AccountKind string

const (
	// AccountLoan is an amortizing installment debt.
	AccountLoan AccountKind = "loan"
	// AccountCard is revolving credit; card payments reduce principal in this model.
	AccountCard AccountKind = "card"
)

// Account is the classification view of an account needed by this package.
type Account struct {
	ID        string
	Name      string
	Lender    string
	Currency  string
	Liability bool
	Kind      AccountKind
	ScopeID   string
}

// Transaction is the classification view of a ledger transaction.
type Transaction struct {
	ID                string
	AccountID         string
	Date              time.Time
	Payee             string
	Description       string
	AmountMinor       int64
	Currency          string
	TransferGroupID   string
	TransferAccountID string
	BillAccountID     string
	ScopeID           string
	MemberID          string
}

// PayeeRule is a previously confirmed payee-to-debt association.
type PayeeRule struct {
	Payee         string
	DebtAccountID string
	ScopeID       string
}

// RecurringPayment describes a known debt payment cadence for matching.
type RecurringPayment struct {
	ID            string
	Label         string
	Payee         string
	DebtAccountID string
	AmountMinor   int64
	Currency      string
	NextDue       time.Time
	ScopeID       string
}

// Dataset contains the evidence used to classify a transaction. All related
// records must belong to the transaction's scope; empty scope IDs match only
// other empty scope IDs.
type Dataset struct {
	Accounts     []Account
	Transactions []Transaction
	PayeeRules   []PayeeRule
	Recurring    []RecurringPayment
}

// EvidenceKind describes why a debt-payment candidate was found.
type EvidenceKind string

const (
	EvidenceLiabilityCounterpart EvidenceKind = "liability_counterpart"
	EvidencePayeeRule            EvidenceKind = "payee_rule"
	EvidenceBillAccount          EvidenceKind = "bill_account"
	EvidenceDescriptorPattern    EvidenceKind = "descriptor_pattern"
	EvidenceRecurringPattern     EvidenceKind = "recurring_pattern"
)

// Evidence is one independently explainable classification signal.
type Evidence struct {
	Kind        EvidenceKind
	Description string
	Score       int
}

// Confidence is the classifier's deterministic strength label.
type Confidence string

const (
	ConfidenceNone   Confidence = "none"
	ConfidenceMedium Confidence = "medium"
	ConfidenceStrong Confidence = "strong"
)

// Candidate is a suggestion only; classification never mutates ledger data.
type Candidate struct {
	TxnID                 string
	TargetDebtAccountID   string
	Evidence              []Evidence
	Confidence            Confidence
	ExistingCounterpartID string
	MemberID              string
}

// Payment is a positive cash amount in integer minor units.
type Payment struct {
	TxnID       string
	Date        time.Time
	AmountMinor int64
	Currency    string
}

// Loan contains the balance and rate immediately before a payment.
type Loan struct {
	AccountID      string
	Kind           AccountKind
	BalanceMinor   int64
	APRBasisPoints int64
	HasAPR         bool
}

// Split is an amount breakdown in integer minor units.
type Split struct {
	PrincipalMinor int64
	InterestMinor  int64
	FeesMinor      int64
	EscrowMinor    int64
}

// Allocation is an explainable payment breakdown. OverpaymentMinor is the
// principal allocated beyond the recorded pre-payment balance.
type Allocation struct {
	PaymentMinor     int64
	PrincipalMinor   int64
	InterestMinor    int64
	FeesMinor        int64
	EscrowMinor      int64
	Estimated        bool
	Method           AllocationMethod
	Overpayment      bool
	OverpaymentMinor int64
}

// AllocationMethod identifies the source of an allocation.
type AllocationMethod string

const (
	AllocationStatement AllocationMethod = "statement"
	AllocationCard      AllocationMethod = "card_payment"
	AllocationAmortized AllocationMethod = "amortization_estimate"
)

// ClassifiedPayment is one payment and its accepted allocation for analytics.
type ClassifiedPayment struct {
	TxnID         string
	DebtAccountID string
	Date          time.Time
	Currency      string
	Allocation    Allocation
}

// Period is a date interval with an inclusive start and exclusive end.
type Period struct {
	ID    string
	Start time.Time
	End   time.Time
}

// PeriodTotals are debt-payment amounts for one period and currency.
type PeriodTotals struct {
	PeriodID       string
	Currency       string
	CashPaidMinor  int64
	PrincipalMinor int64
	InterestMinor  int64
	FeesMinor      int64
}
