package domain

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/money"
)

// GroupOwnerID is the owner identifier used for shared (household-level) entities.
const GroupOwnerID = "group"

// Member is a person in the household/group. Members are owners of individual
// pools and are labels within the single local dataset (no auth).
type Member struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Color     string         `json:"color,omitempty"`
	IsDefault bool           `json:"isDefault,omitempty"`
	Custom    map[string]any `json:"custom,omitempty"`
}

// Account is anything you own (asset) or owe (liability). Optional fields apply
// only to the relevant class; a zero value means "unset".
type Account struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	OwnerID        string       `json:"ownerId"` // member ID, or GroupOwnerID
	Scope          Scope        `json:"scope"`
	Class          AccountClass `json:"class"`
	Type           AccountType  `json:"type"`
	Currency       string       `json:"currency"`
	OpeningBalance money.Money   `json:"openingBalance"`
	BalanceAsOf    time.Time    `json:"balanceAsOf"`

	// Liability-only fields.
	CreditLimit     money.Money `json:"creditLimit,omitempty"`
	InterestRateAPR float64     `json:"interestRateApr,omitempty"`
	MinPayment      money.Money `json:"minPayment,omitempty"`
	DueDayOfMonth   int         `json:"dueDayOfMonth,omitempty"`
	Lender          string      `json:"lender,omitempty"`

	// Allocation-engine attributes (asset-side).
	ExpectedReturnAPR float64   `json:"expectedReturnApr,omitempty"`
	LiquidityScore    int       `json:"liquidityScore,omitempty"` // 0..100
	StabilityScore    int       `json:"stabilityScore,omitempty"` // 0..100
	LockUntil         time.Time `json:"lockUntil,omitempty"`

	Archived bool           `json:"archived,omitempty"`
	Custom   map[string]any `json:"custom,omitempty"`
}

// Category classifies transactions as income or expense; categories may nest.
type Category struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Kind     CategoryKind   `json:"kind"`
	Color    string         `json:"color,omitempty"`
	ParentID string         `json:"parentId,omitempty"`
	Custom   map[string]any `json:"custom,omitempty"`
}

// Transaction is a single money movement. A positive Amount is income, a
// negative Amount is an expense. When TransferAccountID is set, the transaction
// is a transfer leg and is excluded from income/expense totals.
type Transaction struct {
	ID                string         `json:"id"`
	AccountID         string         `json:"accountId"`
	Date              time.Time      `json:"date"`
	Payee             string         `json:"payee,omitempty"`
	Desc              string         `json:"desc"`
	CategoryID        string         `json:"categoryId,omitempty"`
	Amount            money.Money    `json:"amount"`
	TransferAccountID string         `json:"transferAccountId,omitempty"`
	Cleared           bool           `json:"cleared,omitempty"`
	Tags              []string       `json:"tags,omitempty"`
	MemberID          string         `json:"memberId,omitempty"`
	SourceDocID       string         `json:"sourceDocId,omitempty"`
	Custom            map[string]any `json:"custom,omitempty"`
}

// IsTransfer reports whether the transaction is a transfer between accounts.
func (t Transaction) IsTransfer() bool { return t.TransferAccountID != "" }

// IsIncome reports whether the transaction counts as income (positive,
// non-transfer).
func (t Transaction) IsIncome() bool { return !t.IsTransfer() && t.Amount.IsPositive() }

// IsExpense reports whether the transaction counts as an expense (negative,
// non-transfer).
func (t Transaction) IsExpense() bool { return !t.IsTransfer() && t.Amount.IsNegative() }

// Budget is a spending limit for a category, owned by a member (individual) or
// the group (shared).
type Budget struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Scope      Scope          `json:"scope"`
	OwnerID    string         `json:"ownerId"`
	CategoryID string         `json:"categoryId"`
	Period     Period         `json:"period"`
	Limit      money.Money    `json:"limit"`
	Custom     map[string]any `json:"custom,omitempty"`
}

// Goal is a savings target, individual or shared.
type Goal struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Scope         Scope          `json:"scope"`
	OwnerID       string         `json:"ownerId"`
	TargetAmount  money.Money    `json:"targetAmount"`
	CurrentAmount money.Money    `json:"currentAmount"`
	TargetDate    time.Time      `json:"targetDate,omitempty"`
	AccountID     string         `json:"accountId,omitempty"`
	Custom        map[string]any `json:"custom,omitempty"`
}

// Task is a budgeting-related to-do item.
type Task struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Notes       string         `json:"notes,omitempty"`
	Due         time.Time      `json:"due,omitempty"`
	Status      TaskStatus     `json:"status"`
	Priority    TaskPriority   `json:"priority"`
	RelatedType RelatedType    `json:"relatedType,omitempty"`
	RelatedID   string         `json:"relatedId,omitempty"`
	MemberID    string         `json:"memberId,omitempty"`
	Source      TaskSource     `json:"source,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
}
