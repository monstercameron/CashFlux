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
	TypeInvestment   AccountType = "investment"
	TypeRetirement   AccountType = "retirement"
	TypeCrypto       AccountType = "crypto"
	TypeOther        AccountType = "other"
)

// AllAccountTypes lists every valid account type.
var AllAccountTypes = []AccountType{
	TypeChecking, TypeDebit, TypeSavings, TypeCash, TypeCreditCard,
	TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage,
	TypeInvestment, TypeRetirement, TypeCrypto, TypeOther,
}

func (t AccountType) String() string { return string(t) }

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
	case TypeCreditCard, TypeLineOfCredit, TypeLoan, TypePersonalLoan, TypeMortgage:
		return ClassLiability
	default:
		return ClassAsset
	}
}

// IsLiability reports whether the type represents money owed.
func (t AccountType) IsLiability() bool { return t.Class() == ClassLiability }

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
