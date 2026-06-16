package domain

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/dateutil"
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
	OpeningBalance money.Money  `json:"openingBalance"`
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

// Formula is a saved sandboxed-formula calculation the user named to reuse — a
// custom KPI over their live figures. Expr is evaluated by internal/formula
// against the live Env; Enabled lets the user keep one without surfacing it.
type Formula struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Expr    string `json:"expr"`
	Enabled bool   `json:"enabled,omitempty"`
}

// AllocationProfile is a saved set of capital-allocation criterion weights — a
// named mix of how much to favor returns, stability, liquidity, and debt
// reduction. The Allocate screen maps these to the scoring engine's weights. The
// weights are plain floats (need not sum to 1; scoring normalizes by their total).
type AllocationProfile struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Returns       float64 `json:"returns"`
	Stability     float64 `json:"stability"`
	Liquidity     float64 `json:"liquidity"`
	DebtReduction float64 `json:"debtReduction"`
}

// RecurringCadence is how often a recurring cash flow repeats.
type RecurringCadence string

const (
	CadenceWeekly    RecurringCadence = "weekly"
	CadenceMonthly   RecurringCadence = "monthly"
	CadenceQuarterly RecurringCadence = "quarterly"
	CadenceYearly    RecurringCadence = "yearly"
)

// Next returns the date one cadence step after from. Month-based cadences use
// dateutil.AddMonths; weekly adds 7 days. An unknown cadence is treated as
// monthly.
func (c RecurringCadence) Next(from time.Time) time.Time {
	switch c {
	case CadenceWeekly:
		return from.AddDate(0, 0, 7)
	case CadenceQuarterly:
		return dateutil.AddMonths(from, 3)
	case CadenceYearly:
		return dateutil.AddMonths(from, 12)
	default:
		return dateutil.AddMonths(from, 1)
	}
}

// Recurring is a scheduled cash flow that repeats on a cadence — a bill, a
// paycheck, a subscription. Amount is signed (negative = money out) and carries
// its currency, mirroring Transaction; Autopost (later) turns due ones into real
// transactions. The forecast/planning features read these.
type Recurring struct {
	ID         string           `json:"id"`
	Label      string           `json:"label"`
	Amount     money.Money      `json:"amount"`
	Cadence    RecurringCadence `json:"cadence"`
	NextDue    time.Time        `json:"nextDue"`
	AccountID  string           `json:"accountId,omitempty"`
	CategoryID string           `json:"categoryId,omitempty"`
	Autopost   bool             `json:"autopost,omitempty"`
}

// Advance returns a copy with NextDue moved one cadence forward — used after a
// due occurrence is posted.
func (r Recurring) Advance() Recurring {
	r.NextDue = r.Cadence.Next(r.NextDue)
	return r
}

// MonthlyEquivalent normalizes the amount to a per-month figure (minor units) so
// cadences can be summed or compared: weekly is scaled by 52/12, quarterly by
// 1/3, yearly by 1/12; monthly is unchanged. Integer math truncates.
func (r Recurring) MonthlyEquivalent() int64 {
	a := r.Amount.Amount
	switch r.Cadence {
	case CadenceWeekly:
		return a * 52 / 12
	case CadenceQuarterly:
		return a / 3
	case CadenceYearly:
		return a / 12
	default:
		return a
	}
}

// SavedInsight is an AI-generated insight the user pinned to revisit later, kept
// separate from to-do tasks so saving an explanation doesn't clutter the list.
type SavedInsight struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// DocumentKind is the source type of an imported document.
type DocumentKind string

const (
	// DocCSV is a pasted/uploaded CSV statement.
	DocCSV DocumentKind = "csv"
	// DocImage is a receipt or statement image read by the vision model.
	DocImage DocumentKind = "image"
)

// DocumentStatus tracks an imported document through its lifecycle.
type DocumentStatus string

const (
	// DocPending means uploaded but not yet read.
	DocPending DocumentStatus = "pending"
	// DocExtracted means a read produced reviewable rows, not yet committed.
	DocExtracted DocumentStatus = "extracted"
	// DocImported means the rows were committed to the ledger.
	DocImported DocumentStatus = "imported"
	// DocFailed means reading the document failed.
	DocFailed DocumentStatus = "failed"
)

// DocumentRow is one extracted line from a document, as reviewed before import.
// Fields are strings (the user can edit them); it mirrors the parser's row shape
// but is persisted independently so a document record isn't tied to the parser.
type DocumentRow struct {
	Date        string `json:"date,omitempty"`
	Description string `json:"description,omitempty"`
	Amount      string `json:"amount,omitempty"`
	Category    string `json:"category,omitempty"`
}

// Document records an imported statement or receipt and the rows read from it, so
// an import can be reviewed, audited, or re-run later. It is the persistent
// counterpart to the in-flight import on the Documents screen.
type Document struct {
	ID         string         `json:"id"`
	Filename   string         `json:"filename,omitempty"`
	Kind       DocumentKind   `json:"kind"`
	UploadedAt time.Time      `json:"uploadedAt"`
	AccountID  string         `json:"accountId,omitempty"`
	MemberID   string         `json:"memberId,omitempty"`
	Status     DocumentStatus `json:"status"`
	Extracted  []DocumentRow  `json:"extracted,omitempty"`
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
