// SPDX-License-Identifier: MIT

// Package domain defines the core CashFlux entity types and their enumerations.
// It is pure Go (no platform dependencies, no build tags) so it can be reused by
// the WebAssembly UI and exercised by native tests.
package domain

// AccountClass distinguishes things you own from things you owe.
type AccountClass string

const (
	ClassAsset     AccountClass = "asset"
	ClassLiability AccountClass = "liability"
)

// AllAccountClasses lists every valid account class.
var AllAccountClasses = []AccountClass{ClassAsset, ClassLiability}

func (c AccountClass) String() string { return string(c) }

// Valid reports whether c is a known account class.
func (c AccountClass) Valid() bool {
	switch c {
	case ClassAsset, ClassLiability:
		return true
	default:
		return false
	}
}

// AccountType is the specific kind of account.
type AccountType string

const (
	TypeChecking     AccountType = "checking"
	TypeDebit        AccountType = "debit"
	TypeSavings      AccountType = "savings"
	TypeCash         AccountType = "cash"
	TypeCreditCard   AccountType = "credit_card"
	TypeLineOfCredit AccountType = "line_of_credit"
	TypeLoan         AccountType = "loan"
	TypePersonalLoan AccountType = "personal_loan"
	TypeMortgage     AccountType = "mortgage"
	// TypeUtilities is a utility/HOA/service account — a recurring obligation you
	// owe (electric, water, HOA dues), so it classes as a liability.
	TypeUtilities  AccountType = "utilities"
	TypeInvestment AccountType = "investment"
	TypeRetirement AccountType = "retirement"
	TypeCrypto     AccountType = "crypto"
	// TypeProperty represents a real-estate asset (e.g. a home) whose value is a
	// periodic user-entered estimate rather than a reconciled cash balance.
	TypeProperty AccountType = "property"
	// TypeVehicle represents a vehicle asset (e.g. a car) whose value is a
	// periodic user-entered estimate rather than a reconciled cash balance.
	TypeVehicle AccountType = "vehicle"
	TypeOther   AccountType = "other"
)

// AllAccountTypes lists every valid account type.
var AllAccountTypes = []AccountType{
	TypeChecking, TypeDebit, TypeSavings, TypeCash, TypeCreditCard,
	TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage, TypeUtilities,
	TypeInvestment, TypeRetirement, TypeCrypto, TypeProperty, TypeVehicle, TypeOther,
}

func (t AccountType) String() string { return string(t) }

// SecurityType is the kind of security a Holding represents, so stocks/securities
// investments can be categorized (and allocated) distinctly from balance-tracked
// "traditional" investment accounts. Empty is treated as SecurityOther.
type SecurityType string

const (
	SecurityStock      SecurityType = "stock"
	SecurityETF        SecurityType = "etf"
	SecurityMutualFund SecurityType = "mutual_fund"
	SecurityBond       SecurityType = "bond"
	SecurityCrypto     SecurityType = "crypto"
	SecurityCash       SecurityType = "cash"
	SecurityOther      SecurityType = "other"
)

// AllSecurityTypes lists every valid security type (picker order).
var AllSecurityTypes = []SecurityType{
	SecurityStock, SecurityETF, SecurityMutualFund, SecurityBond, SecurityCrypto, SecurityCash, SecurityOther,
}

func (s SecurityType) String() string { return string(s) }

// Normalized returns the security type, mapping the empty value to SecurityOther.
func (s SecurityType) Normalized() SecurityType {
	if s == "" {
		return SecurityOther
	}
	return s
}

// Valid reports whether s is a known security type (empty is allowed — it means
// "unspecified" and normalizes to other).
func (s SecurityType) Valid() bool {
	if s == "" {
		return true
	}
	for _, v := range AllSecurityTypes {
		if v == s {
			return true
		}
	}
	return false
}

// Valid reports whether t is a known account type.
func (t AccountType) Valid() bool {
	for _, v := range AllAccountTypes {
		if v == t {
			return true
		}
	}
	return false
}

// Class returns the natural account class for a type (liabilities are debts).
func (t AccountType) Class() AccountClass {
	switch t {
	case TypeCreditCard, TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage, TypeUtilities:
		return ClassLiability
	default:
		return ClassAsset
	}
}

// IsLiability reports whether the type represents money owed.
func (t AccountType) IsLiability() bool { return t.Class() == ClassLiability }

// IsSavingsLike reports whether the type is a savings or investment vehicle you
// deliberately fund each month — savings, investment, retirement, or crypto. It
// selects the accounts the zero-based Budgets view lists under "Savings &
// investments" for a per-account monthly savings budget.
func (t AccountType) IsSavingsLike() bool {
	switch t {
	case TypeSavings, TypeInvestment, TypeRetirement, TypeCrypto:
		return true
	default:
		return false
	}
}

// CategoryKind classifies a category as income or expense.
type CategoryKind string

const (
	KindIncome  CategoryKind = "income"
	KindExpense CategoryKind = "expense"
)

// AllCategoryKinds lists every valid category kind.
var AllCategoryKinds = []CategoryKind{KindIncome, KindExpense}

func (k CategoryKind) String() string { return string(k) }

// Valid reports whether k is a known category kind.
func (k CategoryKind) Valid() bool {
	switch k {
	case KindIncome, KindExpense:
		return true
	default:
		return false
	}
}

// Scope marks whether an entity belongs to one member or the whole household.
// A shared (group-level) entity uses GroupOwnerID as its owner.
type Scope string

const (
	ScopeIndividual Scope = "individual"
	ScopeShared     Scope = "shared"
)

// AllScopes lists every valid scope.
var AllScopes = []Scope{ScopeIndividual, ScopeShared}

func (s Scope) String() string { return string(s) }

// Valid reports whether s is a known scope.
func (s Scope) Valid() bool {
	switch s {
	case ScopeIndividual, ScopeShared:
		return true
	default:
		return false
	}
}

// Period is a budgeting period.
type Period string

const (
	// PeriodWeekly is a 7-day rolling window anchored to the household week-start.
	PeriodWeekly Period = "weekly"
	// PeriodBiweekly is a 14-day window on a stable fortnightly grid (every 2 weeks).
	PeriodBiweekly Period = "biweekly"
	// PeriodSemimonthly splits each calendar month into two halves: the 1st–15th and the 16th–end.
	PeriodSemimonthly Period = "semimonthly"
	// PeriodMonthly is a full calendar month.
	PeriodMonthly Period = "monthly"
	// PeriodQuarterly is a three-month calendar quarter (Q1–Q4).
	PeriodQuarterly Period = "quarterly"
	// PeriodYearly is the full calendar year (Jan 1 – Dec 31).
	PeriodYearly Period = "yearly"
)

// AllPeriods lists every valid period in display order.
var AllPeriods = []Period{
	PeriodWeekly, PeriodBiweekly, PeriodSemimonthly,
	PeriodMonthly, PeriodQuarterly, PeriodYearly,
}

func (p Period) String() string { return string(p) }

// Label returns a human-friendly name for the period.
func (p Period) Label() string {
	switch p {
	case PeriodWeekly:
		return "Weekly"
	case PeriodBiweekly:
		return "Every 2 weeks"
	case PeriodSemimonthly:
		return "Twice a month"
	case PeriodQuarterly:
		return "Quarterly"
	case PeriodYearly:
		return "Year"
	default:
		return "Monthly"
	}
}

// Valid reports whether p is a known period.
func (p Period) Valid() bool {
	switch p {
	case PeriodWeekly, PeriodBiweekly, PeriodSemimonthly, PeriodMonthly, PeriodQuarterly, PeriodYearly:
		return true
	default:
		return false
	}
}

// TaskStatus is the completion state of a to-do item.
type TaskStatus string

const (
	StatusOpen TaskStatus = "open"
	StatusDone TaskStatus = "done"
)

// AllTaskStatuses lists every valid task status.
var AllTaskStatuses = []TaskStatus{StatusOpen, StatusDone}

func (s TaskStatus) String() string { return string(s) }

// Valid reports whether s is a known task status.
func (s TaskStatus) Valid() bool {
	switch s {
	case StatusOpen, StatusDone:
		return true
	default:
		return false
	}
}

// TaskPriority ranks a to-do item.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "med"
	PriorityHigh   TaskPriority = "high"
)

// AllTaskPriorities lists every valid task priority.
var AllTaskPriorities = []TaskPriority{PriorityLow, PriorityMedium, PriorityHigh}

func (p TaskPriority) String() string { return string(p) }

// Valid reports whether p is a known task priority.
func (p TaskPriority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}

// GoalKind is what a goal measures its progress by. A goal need not be financial:
// a checklist goal is driven by its linked to-dos, a milestone is a single
// done/not-done step, and a habit tracks a streak of recurring check-ins.
type GoalKind string

const (
	// GoalKindFinancial is a savings target measured in money (TargetAmount /
	// CurrentAmount). This is the default: the empty string is treated as
	// financial for backwards-compatibility with goals created before this field.
	GoalKindFinancial GoalKind = "financial"
	// GoalKindChecklist measures progress by its linked to-dos: percent complete =
	// completed linked tasks / total linked tasks. It has no money target.
	GoalKindChecklist GoalKind = "checklist"
	// GoalKindMilestone is a single binary objective — done or not done — with an
	// optional target date. Completion is recorded in Goal.DoneAt.
	GoalKindMilestone GoalKind = "milestone"
	// GoalKindHabit tracks a streak of recurring check-ins toward a target count
	// (e.g. "log expenses weekly for 12 weeks"); progress = check-ins / HabitTarget.
	GoalKindHabit GoalKind = "habit"
)

// AllGoalKinds lists every valid goal kind in display order.
var AllGoalKinds = []GoalKind{GoalKindFinancial, GoalKindChecklist, GoalKindMilestone, GoalKindHabit}

func (k GoalKind) String() string { return string(k) }

// Valid reports whether k is a known, non-empty goal kind.
func (k GoalKind) Valid() bool {
	for _, v := range AllGoalKinds {
		if v == k {
			return true
		}
	}
	return false
}

// Label returns a human-friendly name for the goal kind.
func (k GoalKind) Label() string {
	switch k {
	case GoalKindChecklist:
		return "Checklist"
	case GoalKindMilestone:
		return "Milestone"
	case GoalKindHabit:
		return "Habit"
	default:
		return "Savings"
	}
}

// IsFinancial reports whether the kind tracks money (the default). Non-financial
// kinds (checklist/milestone/habit) don't use TargetAmount/CurrentAmount.
func (k GoalKind) IsFinancial() bool {
	return k == "" || k == GoalKindFinancial
}

// RelatedType names the kind of entity a task is linked to.
type RelatedType string

const (
	RelatedNone        RelatedType = "none"
	RelatedAccount     RelatedType = "account"
	RelatedBudget      RelatedType = "budget"
	RelatedGoal        RelatedType = "goal"
	RelatedTransaction RelatedType = "transaction"
	RelatedDocument    RelatedType = "document"
)

// AllRelatedTypes lists every valid related type.
var AllRelatedTypes = []RelatedType{
	RelatedNone, RelatedAccount, RelatedBudget, RelatedGoal, RelatedTransaction, RelatedDocument,
}

func (r RelatedType) String() string { return string(r) }

// Valid reports whether r is a known related type.
func (r RelatedType) Valid() bool {
	for _, v := range AllRelatedTypes {
		if v == r {
			return true
		}
	}
	return false
}

// TaskSource records how a task was created.
type TaskSource string

const (
	SourceManual TaskSource = "manual"
	SourceAI     TaskSource = "ai"
	SourceNudge  TaskSource = "nudge"
)

// AllTaskSources lists every valid task source.
var AllTaskSources = []TaskSource{SourceManual, SourceAI, SourceNudge}

func (s TaskSource) String() string { return string(s) }

// Valid reports whether s is a known task source.
func (s TaskSource) Valid() bool {
	switch s {
	case SourceManual, SourceAI, SourceNudge:
		return true
	default:
		return false
	}
}

// TxnSource records the provenance of a transaction — how it entered the ledger.
// It is set at each creation path so the ledger can show (and filter by) where a
// row came from. An empty value means the source was not recorded (e.g. a row
// created before provenance tracking) and reads as "—" in the UI.
type TxnSource string

const (
	// TxnSourceManual is a row a person entered by hand (quick-add, the add form, a
	// balance reconcile adjustment, or a user-initiated transfer).
	TxnSourceManual TxnSource = "manual"
	// TxnSourceImported is a row parsed from an uploaded file (CSV import).
	TxnSourceImported TxnSource = "imported"
	// TxnSourceScanned is a row extracted from an uploaded document or receipt image
	// (the vision/extract import path).
	TxnSourceScanned TxnSource = "scanned"
	// TxnSourceRecurring is a row generated automatically from a recurring rule, bill,
	// or goal contribution.
	TxnSourceRecurring TxnSource = "recurring"
	// TxnSourceAssistant is a row created by the in-app AI assistant (chat agent).
	TxnSourceAssistant TxnSource = "assistant"
)

// AllTxnSources lists every known transaction source, in display order.
var AllTxnSources = []TxnSource{
	TxnSourceManual, TxnSourceImported, TxnSourceScanned, TxnSourceRecurring, TxnSourceAssistant,
}

func (s TxnSource) String() string { return string(s) }

// Valid reports whether s is a known, non-empty transaction source.
func (s TxnSource) Valid() bool {
	switch s {
	case TxnSourceManual, TxnSourceImported, TxnSourceScanned, TxnSourceRecurring, TxnSourceAssistant:
		return true
	default:
		return false
	}
}

// Label is the human-readable name shown in the ledger's Source column and filter.
// An unset or unknown source reads as an em dash so untracked rows are obvious
// rather than blank.
func (s TxnSource) Label() string {
	switch s {
	case TxnSourceManual:
		return "Manual"
	case TxnSourceImported:
		return "Imported"
	case TxnSourceScanned:
		return "Scanned"
	case TxnSourceRecurring:
		return "Recurring"
	case TxnSourceAssistant:
		return "Assistant"
	default:
		return "—"
	}
}
