// SPDX-License-Identifier: MIT

package domain

import (
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/dashlayout"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/widgetcfg"
)

// GroupOwnerID is the owner identifier used for shared (household-level) entities.
const GroupOwnerID = "group"

// MemberRole is the permission tier assigned to a household member.
// It controls what operations the member may perform when role-enforcement
// is wired (future tickets). The zero value ("") is treated as RoleAdmin
// by the memberrole package to give existing members full access after
// a schema migration.
type MemberRole string

const (
	// RoleOwner is the primary household owner — full control including
	// member management. Assigned automatically to the default member
	// (IsDefault=true) when a dataset is first created.
	RoleOwner MemberRole = "owner"

	// RoleAdmin is a trusted household member with full access to financial
	// entities but no member-management permission. This is the default for
	// all non-default members and for legacy rows that pre-date the role field.
	RoleAdmin MemberRole = "admin"

	// RoleViewer is a read-only member who can see data but may not create,
	// edit, or delete any entity.
	RoleViewer MemberRole = "viewer"
)

// Member is a person in the household/group. Members are owners of individual
// pools and are labels within the single local dataset (no auth).
type Member struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	IsDefault bool   `json:"isDefault,omitempty"`
	// Role is the member's permission tier. The zero value is treated as
	// RoleAdmin by the memberrole package for backwards-compatibility with
	// datasets created before this field existed.
	Role   MemberRole     `json:"role,omitempty"`
	Prefs  MemberPrefs    `json:"prefs,omitempty"`
	Custom map[string]any `json:"custom,omitempty"`
}

// MemberPrefs holds a member's personal overrides layered over the household
// defaults (§1.19). Every field is optional ("" / zero = inherit the household
// value); resolution is in internal/memberprefs. Kept as plain strings so the
// domain stays dependency-light. DateStyle mirrors prefs.DateStyle values
// ("iso"/"us"/"eu"/"long"); DefaultAccountID / DefaultMemberID seed the quick-add
// for this member.
type MemberPrefs struct {
	DateStyle        string `json:"dateStyle,omitempty"`
	DefaultAccountID string `json:"defaultAccountId,omitempty"`
	DefaultMemberID  string `json:"defaultMemberId,omitempty"`
}

// IsZero reports whether no per-member preference is set (all inherit).
func (p MemberPrefs) IsZero() bool {
	return p.DateStyle == "" && p.DefaultAccountID == "" && p.DefaultMemberID == ""
}

// Account is anything you own (asset) or owe (liability). Optional fields apply
// only to the relevant class; a zero value means "unset".
type Account struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OwnerID string `json:"ownerId"` // member ID, or GroupOwnerID

	// OwnershipShares optionally records fractional ownership of this account as
	// integer percentage points per member (e.g. {"m1": 60, "m2": 40}). When
	// non-empty the values must sum to exactly 100. An empty/nil map means the
	// account is owned wholly by OwnerID (the binary-ownership default); any
	// stored JSON that pre-dates this field deserialises to nil with no migration
	// needed. NetByOwner honours these shares instead of OwnerID when set.
	OwnershipShares map[string]int `json:"ownershipShares,omitempty"`

	Scope Scope        `json:"scope"`
	Class AccountClass `json:"class"`
	Type  AccountType  `json:"type"`
	// Institution is the name of the financial institution that holds this account
	// (e.g. "Chase", "Wells Fargo", "Fidelity"). Optional; omitted from JSON when
	// empty so existing stored rows round-trip to "" with no migration needed.
	Institution string `json:"institution,omitempty"`
	// InstitutionID references a domain.Institution entity (AC10) — the structured
	// institution directory that grounds Multi-Institution Analytics with a real
	// entity instead of the free-text Institution string. Optional; empty means the
	// account belongs to no directory institution. When an institution is deleted,
	// reassign-on-delete clears this back to "" (accounts fall back to no-institution).
	// Additive: existing rows round-trip to "" with no migration.
	InstitutionID string `json:"institutionId,omitempty"`
	// DocRefs are the documents attached to this account (AC8): statements, contracts,
	// titles, payoff letters — each a reference to a stored domain.Artifact plus dated
	// filing metadata and an optional renewal/expiry date (AC17). The blob GC retains
	// any artifact referenced here (see internal/artifactref). Additive; empty for
	// existing rows.
	DocRefs []AccountDocRef `json:"docRefs,omitempty"`
	// BeneficiaryNote is a free-text beneficiary / transfer-on-death (TOD) note for
	// this account (AC16) — e.g. "TOD to Jane Doe (spouse)", "beneficiary form on file
	// with Fidelity". Plain text that travels in the estate emergency pack. NEVER store
	// logins or passwords here — that is the encrypted credential vault's job. Additive;
	// empty for existing rows.
	BeneficiaryNote string      `json:"beneficiaryNote,omitempty"`
	Currency        string      `json:"currency"`
	OpeningBalance  money.Money `json:"openingBalance"`
	BalanceAsOf     time.Time   `json:"balanceAsOf"`
	// VarName is an optional explicit variable name for this account in the formula/widget
	// engine. When set, the account's figures are exposed as account_<slug(VarName)>_* (e.g.
	// account_checking_balance) instead of the name-derived slug. Empty = derive from Name.
	VarName string `json:"varName,omitempty"`

	// Liability-only fields.
	CreditLimit     money.Money `json:"creditLimit,omitempty"`
	InterestRateAPR float64     `json:"interestRateApr,omitempty"`
	MinPayment      money.Money `json:"minPayment,omitempty"`
	DueDayOfMonth   int         `json:"dueDayOfMonth,omitempty"`
	// StatementDay is the day of the month (1–31) a liability's statement closes —
	// distinct from DueDayOfMonth, which is THE payment due day (the day a payment is
	// owed). The statement-close day feeds the real billing cycle: the on-time payment
	// window runs statement-close → due day, and TX9 bill-matching tightens its
	// occurrence window to that cycle. Zero (the default) means unknown; omitted from
	// JSON when zero so existing rows round-trip with no migration needed. Liability-only.
	StatementDay    int    `json:"statementDay,omitempty"`
	Lender          string `json:"lender,omitempty"`
	IncludeInPayoff *bool  `json:"includeInPayoff,omitempty"` // nil = default (every liability but a mortgage)

	// APY is the account's annual percentage yield as a percent (e.g. 4.4 for
	// 4.4%). Optional and asset-side: when set on a savings/investment account it
	// drives the interest-aware goal ETA (goalinterest.Project) — a goal linked to
	// this account projects its finish date with monthly compounding. Distinct from
	// ExpectedReturnAPR (the allocation-engine scoring input): APY is the concrete,
	// user-entered yield the goal projection compounds. Zero (the default) means no
	// yield and the goal falls back to linear pace math. Omitted from JSON when zero
	// so existing rows round-trip with no migration needed.
	APY float64 `json:"apy,omitempty"`

	// Allocation-engine attributes (asset-side).
	ExpectedReturnAPR float64   `json:"expectedReturnApr,omitempty"`
	LiquidityScore    int       `json:"liquidityScore,omitempty"` // 0..100
	StabilityScore    int       `json:"stabilityScore,omitempty"` // 0..100
	LockUntil         time.Time `json:"lockUntil,omitempty"`

	// MonthlySavings is the amount the household plans to put into this savings/investment
	// account each month — the per-account monthly savings budget the zero-based view
	// counts toward "assigned". Zero (the default) means no planned contribution. Stored
	// in the account's own currency; omitted from JSON when zero so existing rows
	// round-trip unchanged.
	MonthlySavings money.Money `json:"monthlySavings,omitempty"`

	// RevalueDays optionally overrides the staleness/revaluation cadence for this
	// account, in whole days (AC5). Manual-asset accounts — property, vehicles,
	// crypto — should not share checking's staleness clock: a house is worth
	// re-estimating quarterly, not nagged monthly. Zero (the default) means "use
	// the per-type cadence" (see internal/revalue); a positive value is an
	// explicit per-account override that wins over the type default. Omitted from
	// JSON when zero so existing rows round-trip with no migration needed.
	RevalueDays int `json:"revalueDays,omitempty"`

	// ExcludeFromNetWorth, when true, keeps this account visible in its class
	// views but omits it from the net_worth / assets / liabilities figures (AC11).
	// For accounts a household tracks but does not consider part of their own net
	// worth — a managed trust, a child's custodial account, a business account.
	// The net-worth surface DISCLOSES how many accounts are excluded by choice,
	// exactly as it discloses accounts dropped for a missing exchange rate — the
	// figure is never silently reduced. Omitted from JSON when false so existing
	// rows round-trip with no migration needed.
	ExcludeFromNetWorth bool `json:"excludeFromNetWorth,omitempty"`

	Archived bool           `json:"archived,omitempty"`
	Custom   map[string]any `json:"custom,omitempty"`

	// Notes is free-text the user attaches to the account (e.g. "joint account with
	// mum", branch, reminders). Plain text — for secrets/logins use the encrypted
	// credential vault, never this field (it rides the dataset export + sync).
	Notes string `json:"notes,omitempty"`
}

// IsLiability reports whether the account is a debt, using its STORED class — the
// source of truth. This honours an explicit override (e.g. an "Other"-type account the
// user flagged as a liability), where AccountType.Class() alone would not. Prefer this
// over t.Type.IsLiability() anywhere a formula asks "is this account money owed?".
func (a Account) IsLiability() bool { return a.Class == ClassLiability }

// Category classifies transactions as income or expense; categories may nest.
type Category struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Kind       CategoryKind `json:"kind"`
	Color      string       `json:"color,omitempty"`
	ParentID   string       `json:"parentId,omitempty"`
	Deductible bool         `json:"deductible,omitempty"`
	// CategoryClass groups the category for flex budgeting (BG2): fixed,
	// non-monthly, or flex. Empty reads as ClassFlex — see Category.ClassOf.
	CategoryClass CategoryClass  `json:"categoryClass,omitempty"`
	Custom        map[string]any `json:"custom,omitempty"`
}

// Transaction is a single money movement. A positive Amount is income, a
// negative Amount is an expense. When TransferAccountID is set, the transaction
// is a transfer leg and is excluded from income/expense totals.
type Transaction struct {
	ID                string          `json:"id"`
	AccountID         string          `json:"accountId"`
	Date              time.Time       `json:"date"`
	Payee             string          `json:"payee,omitempty"`
	Desc              string          `json:"desc"`
	CategoryID        string          `json:"categoryId,omitempty"`
	Amount            money.Money     `json:"amount"`
	Splits            []CategorySplit `json:"splits,omitempty"`
	TransferAccountID string          `json:"transferAccountId,omitempty"`
	Cleared           bool            `json:"cleared,omitempty"`
	Tags              []string        `json:"tags,omitempty"`
	MemberID          string          `json:"memberId,omitempty"`
	SourceDocID       string          `json:"sourceDocId,omitempty"`
	// Source records how this transaction entered the ledger (manual entry, CSV
	// import, document scan, recurring rule, AI assistant). Empty = not recorded
	// (e.g. created before provenance tracking); reads as "—". See domain.TxnSource.
	Source      TxnSource       `json:"source,omitempty"`
	Attachments []AttachmentRef `json:"attachments,omitempty"`
	Custom      map[string]any  `json:"custom,omitempty"`
	// Reviewed marks an entry the user has explicitly confirmed on entry, so
	// auto-review workflows (ActionFlagReview) skip tagging it "needs-review"
	// (L43 — suppress the auto-tag on confident manual entry).
	Reviewed bool `json:"reviewed,omitempty"`
	// BillAccountID marks this transaction as a recurring BILL PAYMENT toward a
	// liability account (id). The Debt page reads the most recent such payment as
	// the account's actual monthly payment (distinct from its minimum), and links
	// back to the payments as proof. Empty = not a bill payment.
	BillAccountID string `json:"billAccountId,omitempty"`
	// SubscriptionName marks this transaction as a payment toward a SUBSCRIPTION,
	// keyed by the subscription's name (subscriptions are detected from history and
	// have no stable id, so the name is the link). The Subscriptions page reads the
	// most recent such payment as the subscription's last confirmed payment and links
	// back to the payments as proof. Empty = not a subscription payment.
	SubscriptionName string `json:"subscriptionName,omitempty"`
}

// AttachmentRef links a transaction to an Artifact-backed receipt, document, or
// imported source. ArtifactID points at domain.Artifact.ID; Kind/MIME are cached
// display hints so transaction rows can show a paperclip/preview without loading
// the full artifact bytes.
type AttachmentRef struct {
	ArtifactID string `json:"artifactId"`
	Name       string `json:"name,omitempty"`
	Kind       string `json:"kind,omitempty"`
	MIME       string `json:"mime,omitempty"`
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
// named mix of how much to favor returns, stability, liquidity, debt reduction,
// and goal progress. The Allocate screen maps these to the scoring engine's
// weights. The weights are plain floats (need not sum to 1; scoring normalizes
// by their total). GoalProgress is optional — older saved profiles without it
// load as 0, which simply means goal progress doesn't influence their ranking.
type AllocationProfile struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Returns       float64 `json:"returns"`
	Stability     float64 `json:"stability"`
	Liquidity     float64 `json:"liquidity"`
	DebtReduction float64 `json:"debtReduction"`
	GoalProgress  float64 `json:"goalProgress,omitempty"`
}

// PlanItemKind distinguishes a recurring monthly assumption from a one-time one.
type PlanItemKind string

const (
	// PlanItemRecurring is a cash flow applied every month of the horizon.
	PlanItemRecurring PlanItemKind = "recurring"
	// PlanItemOneTime is a single cash flow in a specific horizon month.
	PlanItemOneTime PlanItemKind = "one_time"
)

// PlanItem is one assumption in a Plan: a labeled cash flow that is either a
// recurring monthly amount or a one-time amount in a specific horizon month.
// Amounts are integer minor units; positive is an inflow, negative an outflow.
type PlanItem struct {
	ID     string       `json:"id"`
	Label  string       `json:"label"`
	Kind   PlanItemKind `json:"kind"`
	Amount int64        `json:"amount"`
	Month  int          `json:"month,omitempty"` // one-time only: 1-based month within the horizon
}

// Plan is a saved what-if scenario: a starting balance (the base scenario)
// projected over HorizonMonths under a set of assumptions (Items). The Planning
// screen runs it through the forecast engine to show a net-worth curve. Amounts
// are integer minor units in the household base currency.
type Plan struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	HorizonMonths int        `json:"horizonMonths"`
	StartBalance  int64      `json:"startBalance"`
	Items         []PlanItem `json:"items,omitempty"`
}

// RecurringCadence is how often a recurring cash flow repeats.
type RecurringCadence string

const (
	CadenceDaily       RecurringCadence = "daily" // every day — used by the goal review cadence
	CadenceWeekly      RecurringCadence = "weekly"
	CadenceBiweekly    RecurringCadence = "biweekly" // every 14 days (C152) — common payday/bill cycle
	CadenceMonthly     RecurringCadence = "monthly"
	CadenceSemimonthly RecurringCadence = "semimonthly" // twice a month, ~1st & 15th (C152)
	CadenceQuarterly   RecurringCadence = "quarterly"
	CadenceYearly      RecurringCadence = "yearly"
)

// Next returns the date one cadence step after from. Month-based cadences use
// dateutil.AddMonths; weekly adds 7 days. An unknown cadence is treated as
// monthly.
func (c RecurringCadence) Next(from time.Time) time.Time {
	switch c {
	case CadenceDaily:
		return from.AddDate(0, 0, 1)
	case CadenceWeekly:
		return from.AddDate(0, 0, 7)
	case CadenceBiweekly:
		return from.AddDate(0, 0, 14)
	case CadenceSemimonthly:
		// Twice a month on a 1st/15th rhythm: before the 15th → the 15th of the same
		// month; on/after the 15th → the 1st of the next month.
		if from.Day() < 15 {
			return time.Date(from.Year(), from.Month(), 15, from.Hour(), from.Minute(), from.Second(), from.Nanosecond(), from.Location())
		}
		next := dateutil.AddMonths(from, 1)
		return time.Date(next.Year(), next.Month(), 1, next.Hour(), next.Minute(), next.Second(), next.Nanosecond(), next.Location())
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
	// Autopay marks a bill the biller charges automatically (the user doesn't pay it
	// by hand). Distinct from Autopost (which posts the due occurrence into the
	// ledger): Autopay is informational — it relaxes "you need to pay this" framing
	// and lets reminders read "Autopay — make sure funds are available" (C157).
	Autopay bool `json:"autopay,omitempty"`
	// SmoothIntoBudgets opts an annual/quarterly bill into sinking-fund smoothing
	// (XC3): instead of a single large hit landing in one budget period, the off
	// periods accrue a virtual monthly set-aside and the landing period is offset so
	// it reads roughly on-pace. Only annual and quarterly cadences smooth; monthly and
	// shorter cadences ignore the flag. A system-managed sinking-fund goal ("Set aside
	// for <label>") is created and maintained while this is on, and dissolved when the
	// recurring is deleted or the flag is cleared. Additive; existing recurrings load
	// with it off. JSON round-trips; no store migration needed.
	SmoothIntoBudgets bool `json:"smoothIntoBudgets,omitempty"`
}

// Smooths reports whether this recurring participates in sinking-fund smoothing:
// the opt-in flag is set AND the cadence is one that benefits from smoothing
// (annual or quarterly — a large amount spread over several off periods). Monthly
// and shorter cadences never smooth even with the flag set, because there are no
// off periods to accrue across.
func (r Recurring) Smooths() bool {
	if !r.SmoothIntoBudgets {
		return false
	}
	switch r.Cadence {
	case CadenceQuarterly, CadenceYearly:
		return true
	default:
		return false
	}
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
	case CadenceBiweekly:
		return a * 26 / 12 // 26 biweekly periods a year
	case CadenceSemimonthly:
		return a * 2 // twice a month
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

// ChatMessage is one turn in an Insights conversation. Role is "user" or
// "assistant"; Tokens records the assistant reply's token usage (0 for user turns).
type ChatMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	Tokens    int       `json:"tokens,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Conversation is a saved Insights chat: an ordered list of messages with a title
// the user can return to, switch between, and delete. Messages are embedded so a
// conversation round-trips as a single JSON row (and a single export entry).
type Conversation struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Named     bool          `json:"named,omitempty"` // title was AI-generated; don't auto-derive over it
	Messages  []ChatMessage `json:"messages,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
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
	// RowCount is the number of transactions imported from this document. For CSV
	// imports the raw rows aren't retained, so this records the count for the
	// import-history row (C11) when Extracted is empty.
	RowCount int `json:"rowCount,omitempty"`
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
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Scope      Scope       `json:"scope"`
	OwnerID    string      `json:"ownerId"`
	CategoryID string      `json:"categoryId"`
	Period     Period      `json:"period"`
	Limit      money.Money `json:"limit"`
	Rollover   bool        `json:"rollover,omitempty"`
	// Methodology overrides the household-level budgeting method for this
	// individual budget. An empty string means "inherit the global method".
	// Valid values are the budgeting.Method* constants ("simple",
	// "zero-based", "envelope"). JSON-persisted; no store migration needed —
	// existing budgets without this field load with the empty string, which
	// correctly falls back to the global method.
	Methodology string         `json:"methodology,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
	// VarName is an optional explicit variable name for this budget in the formula/widget
	// engine. When set, the budget's figures are exposed as budget_<slug(VarName)>_* (e.g.
	// budget_rent_remaining) instead of the name-derived slug — so a user can pick a short,
	// stable handle that survives a display-name change. Empty = derive from Name.
	VarName string `json:"varName,omitempty"`
	// RecurringCover, when set, is a standing arrangement that re-applies a cover into
	// this budget at the start of each new period: move AmountMinor of limit into this
	// budget, split across Sources by weight. Nil = no recurring coverage. JSON-
	// persisted; existing budgets load with nil (no migration needed).
	RecurringCover *RecurringCover `json:"recurringCover,omitempty"`
	// CoveredAt is when this budget last received cover money (its limit was topped up
	// from another budget). The UI shows a "Covered" flag while it falls in the current
	// period, then it quietly ages out. Zero = never covered.
	CoveredAt time.Time `json:"coveredAt,omitempty"`
	// CategoryIDs, when non-empty, makes this a MULTI-CATEGORY budget that tracks the
	// combined spend of every listed category (each still rolls up its sub-categories).
	// Empty = a single-category budget tracking CategoryID (the historical shape). New
	// field, additive — existing budgets load with nil and behave exactly as before.
	CategoryIDs []string `json:"categoryIds,omitempty"`
	// Notes is a free-text note attached to the budget (why it exists, review reminders).
	// Plain text; rides the dataset export/sync. Empty = no note. Additive.
	Notes string `json:"notes,omitempty"`
	// PeriodBoosts is a one-time, PER-PERIOD limit adjustment keyed by the period's start
	// date ("2006-01-02"): the effective cap for that period is Limit + PeriodBoosts[key].
	// A "top up this month only" adds to the current period's entry without touching the
	// base Limit (so next period reverts). Values are minor units in the budget's currency
	// and may be negative (this-period cover pulled FROM this budget). Additive; existing
	// budgets load with nil (no boosts).
	PeriodBoosts map[string]int64 `json:"periodBoosts,omitempty"`
	// PeriodNotes is a per-period journal keyed by the period's start date ("2006-01-02"):
	// one short note explaining that period ("December was high because we hosted"). Distinct
	// from Notes, which is the budget's standing note; these are anchored to a single period
	// and surface in the row expand for the viewed period (BG16). Additive; existing budgets
	// load with nil (no per-period notes) and export/import round-trips like every other map.
	PeriodNotes map[string]string `json:"periodNotes,omitempty"`
	// TargetKind is the budget's optional funding-target shape (BG1). Empty
	// (TargetNone) means the budget has no target beyond its Limit. The kind
	// drives how "still needed to fund this period" is computed and feeds BG4's
	// underfunded quick-fill. Additive; existing budgets load with TargetNone.
	TargetKind TargetKind `json:"targetKind,omitempty"`
	// TargetAmount is the target level in minor units: the refill ceiling for
	// TargetRefillUpTo, the fixed per-period amount for TargetSetAside, or the
	// lump-sum goal for TargetByDate. Zero when TargetKind is TargetNone.
	TargetAmount money.Money `json:"targetAmount,omitempty"`
	// TargetDate is the deadline for a TargetByDate target — the date by which
	// TargetAmount should be accumulated. Zero for other target kinds.
	TargetDate time.Time `json:"targetDate,omitempty"`
	// LinkedGoalID links a TargetByDate budget to a goal that owns the
	// accumulation (a by-date target is a goal wearing a budget's clothes). When
	// set, the by-date "needed" reads from the goal's pace instead of being
	// re-derived here. Empty = no linked goal. Additive.
	LinkedGoalID string `json:"linkedGoalId,omitempty"`
	// RolloverCapPeriods caps how much unused budget a rollover budget may carry
	// forward, as a multiple of the period limit: a neglected budget accumulates at
	// most RolloverCapPeriods × limit of surplus, so a fictional cushion can't build
	// up unbounded (BG5). Zero or negative means UNCAPPED, preserving the historical
	// rollover behavior; existing budgets load with 0 and keep carrying without a
	// ceiling. A carried-forward DEFICIT (envelope debt) is never clamped — the cap
	// limits surplus, not debt. Additive.
	RolloverCapPeriods int `json:"rolloverCapPeriods,omitempty"`
}

// HasTarget reports whether the budget has a funding target beyond its limit (BG1).
func (b Budget) HasTarget() bool { return b.TargetKind != TargetNone && b.TargetKind != "" }

// PeriodNote returns the journal note recorded for the period starting on the given date
// (empty if none) — the per-period annotation shown in that period's row expand (BG16).
func (b Budget) PeriodNote(periodStart time.Time) string {
	if b.PeriodNotes == nil {
		return ""
	}
	return b.PeriodNotes[periodStart.Format("2006-01-02")]
}

// WithPeriodNote returns a copy of the budget with the note set for the period starting on
// the given date. Trimming to empty deletes the entry (and drops the map when it empties),
// so a cleared note doesn't linger in the dataset.
func (b Budget) WithPeriodNote(periodStart time.Time, note string) Budget {
	key := periodStart.Format("2006-01-02")
	note = strings.TrimSpace(note)
	m := make(map[string]string, len(b.PeriodNotes)+1)
	for k, v := range b.PeriodNotes {
		m[k] = v
	}
	if note == "" {
		delete(m, key)
	} else {
		m[key] = note
	}
	if len(m) == 0 {
		m = nil
	}
	b.PeriodNotes = m
	return b
}

// PeriodBoost returns the one-time limit adjustment recorded for the period starting on
// the given date (0 if none) — the amount added to the base Limit for that period only.
func (b Budget) PeriodBoost(periodStart time.Time) int64 {
	if b.PeriodBoosts == nil {
		return 0
	}
	return b.PeriodBoosts[periodStart.Format("2006-01-02")]
}

// WithPeriodBoost returns a copy of the budget with delta added to the period-start's
// one-time boost (creating the map on first use, and dropping the entry when it nets to
// zero so the map doesn't accumulate cleared boosts).
func (b Budget) WithPeriodBoost(periodStart time.Time, delta int64) Budget {
	key := periodStart.Format("2006-01-02")
	m := make(map[string]int64, len(b.PeriodBoosts)+1)
	for k, v := range b.PeriodBoosts {
		m[k] = v
	}
	m[key] += delta
	if m[key] == 0 {
		delete(m, key)
	}
	if len(m) == 0 {
		m = nil
	}
	b.PeriodBoosts = m
	return b
}

// TrackedCategoryIDs is the set of categories a budget counts spend against: the
// explicit CategoryIDs for a multi-category budget, otherwise the single CategoryID.
// Always the source of truth for "what does this budget track" — callers should use it
// rather than reading CategoryID directly.
func (b Budget) TrackedCategoryIDs() []string {
	if len(b.CategoryIDs) > 0 {
		return b.CategoryIDs
	}
	if b.CategoryID != "" {
		return []string{b.CategoryID}
	}
	return nil
}

// TracksCategory reports whether the budget directly tracks categoryID (not counting
// sub-category rollup, which the budgeting engine layers on separately).
func (b Budget) TracksCategory(categoryID string) bool {
	for _, id := range b.TrackedCategoryIDs() {
		if id == categoryID {
			return true
		}
	}
	return false
}

// CoverShare is one source budget's weighted share in a recurring cover. Weight is a
// fixed ratio; WeightFormula, when non-empty, is evaluated in that source budget's
// context each period instead (so a share can track e.g. cf_budget_priority).
type CoverShare struct {
	BudgetID      string `json:"budgetId"`
	Weight        int    `json:"weight"`
	WeightFormula string `json:"weightFormula,omitempty"`
}

// RecurringCover is a per-period, standing cover arrangement stored on the destination
// budget. Each new period the app moves an amount of limit into the destination, split
// across Sources in proportion to their weights. LastAppliedPeriod is the start date
// (YYYY-MM-DD) of the period last covered, so it applies at most once per period.
//
// The amount is AmountMinor (fixed) unless AmountFormula is set, in which case that
// formula is evaluated in the destination budget's context each period (so e.g.
// `overspend` re-covers whatever the shortfall is that period). Likewise a source's
// weight can be a fixed Weight or a per-source WeightFormula.
type RecurringCover struct {
	AmountMinor       int64          `json:"amountMinor"`
	AmountFormula     string         `json:"amountFormula,omitempty"`
	Sources           []CoverShare   `json:"sources"`
	LastAppliedPeriod string         `json:"lastAppliedPeriod,omitempty"`
	Custom            map[string]any `json:"custom,omitempty"` // user-defined "cover" custom fields (metadata on the standing rule)
}

// Goal is a target the household works toward. By default it is a savings
// target (Kind financial: TargetAmount / CurrentAmount), but a goal need not be
// financial — see GoalKind. Any goal, of any kind, may have to-dos linked to it
// (Task.RelatedType=RelatedGoal, RelatedID=Goal.ID); for a checklist goal those
// linked to-dos drive its percent complete.
type Goal struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Scope         Scope       `json:"scope"`
	OwnerID       string      `json:"ownerId"`
	TargetAmount  money.Money `json:"targetAmount"`
	CurrentAmount money.Money `json:"currentAmount"`
	TargetDate    time.Time   `json:"targetDate,omitempty"`
	AccountID     string      `json:"accountId,omitempty"`
	// Kind selects how the goal measures progress (financial / checklist /
	// milestone / habit). The empty string is treated as financial for backwards-
	// compatibility with goals created before this field. JSON round-trips
	// automatically; no store migration is needed. See domain.GoalKind.
	Kind GoalKind `json:"kind,omitempty"`
	// DoneAt records when a milestone goal was marked complete (zero = not done).
	// Only meaningful for GoalKindMilestone.
	DoneAt time.Time `json:"doneAt,omitempty"`
	// HabitCadence is the expected check-in rhythm for a habit goal (weekly,
	// monthly, …); HabitTarget is how many check-ins complete it; CheckIns is the
	// recorded check-in timestamps. Only meaningful for GoalKindHabit.
	HabitCadence RecurringCadence `json:"habitCadence,omitempty"`
	HabitTarget  int              `json:"habitTarget,omitempty"`
	CheckIns     []time.Time      `json:"checkIns,omitempty"`
	// Archived marks a completed goal as moved to the "Achieved" section.
	// JSON round-trips automatically; no store schema change is needed.
	Archived bool `json:"archived,omitempty"`
	// IsSinkingFund marks the goal as a sinking fund — a bucket you save
	// into regularly for an irregular future expense (car repairs, holidays,
	// annual subscriptions, etc.). Sinking funds are displayed in a dedicated
	// "Sinking funds" section on the Goals screen with their monthly set-aside
	// contribution shown. JSON round-trips automatically; no store migration needed.
	IsSinkingFund bool `json:"isSinkingFund,omitempty"`
	// CategoryID links the sinking fund to a spending category so draw-downs
	// can be matched against categorized transactions (C192). Optional — the
	// field is meaningful mainly for sinking funds but is not constrained to
	// them. JSON round-trips automatically; no store migration needed.
	CategoryID string         `json:"categoryId,omitempty"`
	Custom     map[string]any `json:"custom,omitempty"`
	// VarName is an optional explicit variable name for this goal in the formula/widget
	// engine. When set, the goal's figures are exposed as goal_<slug(VarName)>_* (e.g.
	// goal_emergency_remaining) instead of the name-derived slug. Empty = derive from Name.
	VarName string `json:"varName,omitempty"`
	// Contributions logs financial contributions (oldest first) so the most recent
	// one can be undone from the goal's ⋯ menu. It is capped at MaxGoalContributions
	// entries — older ones drop off, which only limits how far "undo" can walk back.
	// JSON round-trips automatically; no store migration needed.
	Contributions []GoalContribution `json:"contributions,omitempty"`
	// MonthlyContribution is an explicit amount to assign to this goal each month
	// under zero-based budgeting ("give every dollar a job"). When set (> 0) it is
	// what the Budgets zero-based view counts toward the assigned total; when zero
	// the view falls back to the target-date-derived pace (goals.MonthlyNeeded).
	// This lets an open-ended savings/investing goal (no target date) still take a
	// flat monthly assignment. Optional/additive — existing goals load with zero.
	MonthlyContribution money.Money `json:"monthlyContribution,omitempty"`
	// AccountIDs are the accounts this goal is funded from / draws on (0..N). It
	// generalises the single AccountID (kept for the linked-account drill-down and
	// back-compat); LinkedAccountIDs unions the two so readers see one list. Additive.
	AccountIDs []string `json:"accountIds,omitempty"`
	// BudgetIDs are the budgets this goal is associated with (0..N) — the budget
	// lines that feed it. Additive; JSON round-trips; no store migration.
	BudgetIDs []string `json:"budgetIds,omitempty"`
	// Allocations are VIRTUAL earmarks: amounts of specific accounts' existing
	// balances reserved for this goal WITHOUT moving money or posting a transaction.
	// Their sum is how much of the target is already "set aside in place" — distinct
	// from CurrentAmount (committed contributions) and Contributions (the undo log).
	// Additive; JSON round-trips.
	Allocations []GoalAllocation `json:"allocations,omitempty"`
	// ReviewCadence is how often the household wants to revisit this goal (weekly,
	// monthly, quarterly, …). When set, the Goals screen flags the goal for review
	// once the cadence has elapsed since LastReviewedAt. Empty = never nags. Additive.
	ReviewCadence RecurringCadence `json:"reviewCadence,omitempty"`
	// LastReviewedAt is when the goal was last created, edited, contributed to, or
	// explicitly marked reviewed — the anchor the ReviewCadence staleness check counts
	// from. Zero on legacy goals (which carry no cadence, so they never nag). Additive.
	LastReviewedAt time.Time `json:"lastReviewedAt,omitempty"`
	// GoalImageArtifactID references a domain.Artifact (an image) shown as a small
	// "vision" banner on the goal card (GL6) — the picture of what you're saving for.
	// It reuses the artifacts/blobstore join exactly like transaction receipts: the
	// goal holds only the artifact ID, and the blob GC (internal/artifactref) counts
	// it as a live reference so the image is never swept. Empty = no image. Optional/
	// additive; JSON round-trips; no store migration needed.
	GoalImageArtifactID string `json:"goalImageArtifactId,omitempty"`
	// PausedUntil pauses the goal until this date (GL7). While paused, contributions
	// aren't expected and the pace logic treats the goal as not-behind (it stops
	// scolding) — pausing is a CHOSEN state, not a failure. Zero = not paused. The
	// projected finish shifts by the paused span (see goals.PauseProjection), and the
	// goal resurfaces once at pause end via a gentle, dismissible nudge. Optional/
	// additive; JSON round-trips; no store migration needed.
	PausedUntil time.Time `json:"pausedUntil,omitempty"`
	// Pledges records each household member's pledged MONTHLY contribution toward a
	// shared goal (GL5): memberID → pledged amount. Small, additive data that
	// survives the multi-user sync upgrade (TX15-stubbed today). Empty on a solo or
	// unpledged goal. The pledge shape is intentionally independent of actual
	// contributions, which are attributed separately via GoalContribution.MemberID.
	Pledges map[string]money.Money `json:"pledges,omitempty"`
	// EssentialBasisMinor records the essential-month figure (base-currency minor
	// units) an emergency-fund target was last derived from (GL3). The re-suggest
	// flag compares a freshly derived figure against this to notice >10% drift.
	// Zero when the target was never auto-derived. Optional/additive; JSON round-trips.
	EssentialBasisMinor int64 `json:"essentialBasisMinor,omitempty"`
}

// IsPaused reports whether the goal is paused at reference time now — its
// PausedUntil is set and still in the future. A paused goal's pace stops
// scolding and its contributions aren't expected (GL7).
func (g Goal) IsPaused(now time.Time) bool {
	return !g.PausedUntil.IsZero() && g.PausedUntil.After(now)
}

// GoalAllocation is one virtual earmark: Amount of AccountID's balance reserved for a
// goal. No transaction is posted — it is a non-destructive reservation the user can add
// or clear freely. Callers guard the reservation so the sum earmarked against a given
// account across all goals never exceeds that account's balance.
type GoalAllocation struct {
	AccountID string      `json:"accountId"`
	Amount    money.Money `json:"amount"`
}

// LinkedAccountIDs returns the goal's linked accounts as one de-duplicated list — the
// multi-select AccountIDs unioned with the legacy single AccountID (when set). Order is
// stable: the legacy AccountID first (if present and not already listed), then AccountIDs.
func (g Goal) LinkedAccountIDs() []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	add(g.AccountID)
	for _, id := range g.AccountIDs {
		add(id)
	}
	return out
}

// AllocatedMinor sums the goal's virtual earmarks in minor units. All allocations share
// the goal's target currency (the allocate UI stores them in it), so this is a plain sum
// with no FX; a zero-value goal yields 0.
func (g Goal) AllocatedMinor() int64 {
	var sum int64
	for _, a := range g.Allocations {
		sum += a.Amount.Amount
	}
	return sum
}

// GoalContribution is one recorded contribution to a financial goal, retained so
// the contribution can be undone. TxnID links the ledger entry posted for it (when
// the "also move money" path was used), so undo can remove that entry too.
type GoalContribution struct {
	Amount money.Money `json:"amount"`
	TxnID  string      `json:"txnId,omitempty"`
	At     time.Time   `json:"at"`
	// MemberID attributes this contribution to a household member on a shared goal
	// (GL5), so actual funding can be measured against each member's pledge. Empty
	// on solo goals or legacy contributions recorded before attribution — those
	// fall back to the contributing member's context at read time. Additive.
	MemberID string `json:"memberId,omitempty"`
}

// MaxGoalContributions bounds the retained contribution log per goal. Contributions
// are infrequent, so this is generous; it exists only so the log can't grow without
// limit over a goal's life.
const MaxGoalContributions = 50

// RecordContribution appends c to the goal's contribution log, dropping the oldest
// entries beyond MaxGoalContributions. Pure; the caller persists the result.
func (g Goal) RecordContribution(c GoalContribution) Goal {
	g.Contributions = append(append([]GoalContribution(nil), g.Contributions...), c)
	if len(g.Contributions) > MaxGoalContributions {
		g.Contributions = g.Contributions[len(g.Contributions)-MaxGoalContributions:]
	}
	return g
}

// PopLastContribution removes and returns the most recent contribution (for undo),
// yielding the updated goal, the popped entry, and ok=false when the log is empty.
// Pure; the caller persists the result.
func (g Goal) PopLastContribution() (Goal, GoalContribution, bool) {
	if len(g.Contributions) == 0 {
		return g, GoalContribution{}, false
	}
	cp := append([]GoalContribution(nil), g.Contributions...)
	last := cp[len(cp)-1]
	g.Contributions = cp[:len(cp)-1]
	return g, last, true
}

// EffectiveKind returns the goal's kind, resolving the empty zero value to
// GoalKindFinancial so callers can switch on a concrete kind without special-
// casing legacy goals stored before the Kind field existed.
func (g Goal) EffectiveKind() GoalKind {
	if g.Kind == "" {
		return GoalKindFinancial
	}
	return g.Kind
}

// IsFinancial reports whether the goal tracks money (its kind is financial or
// the legacy empty default). Non-financial goals ignore TargetAmount/CurrentAmount.
func (g Goal) IsFinancial() bool { return g.EffectiveKind().IsFinancial() }

// IsMilestoneDone reports whether a milestone goal has been marked complete.
func (g Goal) IsMilestoneDone() bool { return !g.DoneAt.IsZero() }

// EarmarkKind identifies what kind of entity an Earmark is targeting.
const (
	EarmarkKindAccount = "account" // an asset account earmark
	EarmarkKindDebt    = "debt"    // a liability paydown earmark
)

// Earmark records that a specific amount has been mentally assigned to an
// account or debt paydown destination without moving any cash. It is created
// by ApplyAllocation and survives reload via the store. Goals do not use
// Earmarks — goal contributions bump Goal.CurrentAmount directly.
type Earmark struct {
	ID              string      `json:"id"`
	DestinationID   string      `json:"destinationId"`
	DestinationKind string      `json:"destinationKind"` // EarmarkKindAccount or EarmarkKindDebt
	Amount          money.Money `json:"amount"`
	Currency        string      `json:"currency"`
	CreatedAt       time.Time   `json:"createdAt"`
	Note            string      `json:"note,omitempty"`
}

// WidgetBinding declares where a custom widget gets its data, kept as plain,
// declarative fields so the binding is config — not code — and stays inspectable
// and JSON-serializable. The pure evaluator (internal/widgetspec) interprets it
// against the live engine context; an empty binding renders an empty widget.
//
//   - Source    names the data the widget reads (e.g. "transactions", "accounts",
//     "budgets", "goals", "tasks", "" for a pure-formula KPI, or "artifact").
//   - Filter    is an optional txnfilter-style criterion string for list/table widgets.
//   - Expr      is an optional sandboxed formula (internal/formula) — the value for a
//     KPI widget, or a derived column/condition for others.
//   - ArtifactID references a stored Artifact (image or dataset) for Image/Table widgets.
//   - Columns   names the fields/columns a list or table widget displays, in order.
type WidgetBinding struct {
	Source     string   `json:"source,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	Expr       string   `json:"expr,omitempty"`
	ArtifactID string   `json:"artifactId,omitempty"`
	Columns    []string `json:"columns,omitempty"`
}

// PageWidget is one widget instance placed on a custom page. Type selects a
// registered widget template (KPI/list/chart/text/table/image); Config holds the
// template's settings (the same widgetcfg.Config shape the dashboard widgets use);
// Binding declares the widget's data source. The instance is identified by ID,
// which is also the id dashlayout.Pack uses to place it on the page's grid.
type PageWidget struct {
	ID      string           `json:"id"`
	Type    string           `json:"type"`
	Title   string           `json:"title,omitempty"`
	Config  widgetcfg.Config `json:"config,omitempty"`
	Binding WidgetBinding    `json:"binding,omitempty"`
	// Spec, when set, binds this page widget to the unified widget engine (the
	// same WidgetSpec the widget designer publishes): the tile's body hydrates
	// through widgetengine (scalar KPIs, collection/series pipelines) instead of
	// the legacy Binding. The page keeps its own tile chrome and layout; only
	// the body rendering is delegated. Legacy fields are ignored when Spec is set.
	Spec *WidgetSpec `json:"spec,omitempty"`
}

// CustomPage is a user-authored page: its own left-rail entry (Name + Icon),
// user-controlled position (Order) and visibility (Hidden), and a bento grid of
// custom widgets. Unlike the built-in dashboard — whose layout lives in
// localStorage — a custom page is user content, so its Layout and Widgets live in
// the dataset and travel with export/import. Slug is the stable URL segment
// (the page is reached at /p/<slug>); ID is the stable storage key.
type CustomPage struct {
	ID        string            `json:"id"`
	Slug      string            `json:"slug"`
	Name      string            `json:"name"`
	Icon      string            `json:"icon,omitempty"`
	Order     int               `json:"order,omitempty"`
	Hidden    bool              `json:"hidden,omitempty"`
	Layout    []dashlayout.Item `json:"layout,omitempty"`
	Widgets   []PageWidget      `json:"widgets,omitempty"`
	CreatedAt time.Time         `json:"createdAt,omitempty"`
}

// Artifact is a user-stored binary or tabular asset — an uploaded image or an
// imported dataset (CSV/JSON) — kept in the dataset so custom widgets (Image,
// Table) can reference it by ID and so it travels with export/import. Images keep
// their raw Bytes + MIME (base64-encoded in JSON); datasets keep parsed Columns +
// Rows (and may keep Bytes for re-parsing). Size is the byte length, cached so the
// UI can show a storage meter without re-measuring.
type Artifact struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"` // see internal/artifacts: image | csv | json
	MIME      string     `json:"mime,omitempty"`
	Bytes     []byte     `json:"bytes,omitempty"`
	BlobRef   *BlobRef   `json:"blobRef,omitempty"`
	Columns   []string   `json:"columns,omitempty"`
	Rows      [][]string `json:"rows,omitempty"`
	Size      int        `json:"size,omitempty"`
	CreatedAt time.Time  `json:"createdAt,omitempty"`
}

// BlobRef points to a backend content-addressed blob that carries artifact bytes
// outside the synced JSON snapshot.
type BlobRef struct {
	Hash string `json:"hash"`
	MIME string `json:"mime,omitempty"`
	Size int    `json:"size,omitempty"`
}

// Task is a budgeting-related to-do item.
type Task struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Notes       string         `json:"notes,omitempty"`
	Due         time.Time      `json:"due,omitempty"`
	Status      TaskStatus     `json:"status"`
	Priority    TaskPriority   `json:"priority"`
	ParentID    string         `json:"parentId,omitempty"` // parent task for nested sub-tasks (C72)
	RelatedType RelatedType    `json:"relatedType,omitempty"`
	RelatedID   string         `json:"relatedId,omitempty"`
	MemberID    string         `json:"memberId,omitempty"`
	Source      TaskSource     `json:"source,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
	// Recurrence controls auto-spawning: when a recurring task is completed,
	// a fresh open occurrence is created with Due advanced one cadence step.
	// Empty (zero value) means the task is a one-shot and no successor is spawned.
	Recurrence RecurringCadence `json:"recurrence,omitempty"`
	// Order is the manual position of this task among its siblings (same ParentID), used by
	// the "Custom order" sort mode and set by drag-and-drop reordering. Lower sorts first;
	// ties fall back to the smart order. Additive — existing tasks load with 0. JSON
	// round-trips; no store migration.
	Order int `json:"order,omitempty"`
	// Resolve is an optional data-condition that auto-completes the task when the
	// underlying money situation resolves (a refund posts, an account is
	// reconciled, a recurring is cancelled). Set by whatever creates the task
	// (a smart flag, the assistant, or the workflow engine); nil = manual task,
	// closed only by hand. Additive — existing tasks load with nil. See XC8.
	Resolve *TaskResolve `json:"resolve,omitempty"`
}

// SubscriptionIgnore records that the user has marked a detected subscription as
// "not a subscription" — it is suppressed from the detected list indefinitely.
// SubName is the transaction Desc / display name that identifies the charge, and
// is the join key against Subscription.Name (case-insensitive). Only one ignore
// record per SubName is kept; IgnoreSubscription deduplicates on save.
type SubscriptionIgnore struct {
	ID        string    `json:"id"`
	SubName   string    `json:"subName"`
	IgnoredOn time.Time `json:"ignoredOn"`
}

// SubscriptionCancellation records that the user has explicitly cancelled a
// detected subscription. SubName is the subscription's display name (the
// transaction Desc that identifies the recurring charge) and is the join key
// against Subscription.Name. Only one cancellation record per SubName is kept;
// MarkSubscriptionCancelled deduplicates on save.
type SubscriptionCancellation struct {
	ID          string    `json:"id"`
	SubName     string    `json:"subName"`
	CancelledOn time.Time `json:"cancelledOn"`
}

// BalanceSnapshot records a point-in-time value for an account. It is appended
// automatically by appstate.PutAccount whenever the account's balance changes, so
// the user can see how an illiquid asset (property, vehicle, investment, other)
// appreciated or depreciated over time. No schema migration is needed — the
// "balance_snapshots" table is created on first write alongside all other entity
// tables, and JSON round-trips automatically.
type BalanceSnapshot struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountId"`
	BalanceMinor int64     `json:"balanceMinor"`
	Currency     string    `json:"currency,omitempty"`
	AsOf         time.Time `json:"asOf"`
}

// Holding is a single investment position within an investment account. All
// money values are integer minor units (e.g. cents for USD). Shares is
// fractional because partial shares are common. No schema migration is needed
// — the "holdings" table is created on first write alongside all other entity
// tables, and JSON round-trips automatically.
type Holding struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"` // investment account this holding belongs to

	// Ticker is the exchange symbol (e.g. "AAPL"). Empty for funds or other
	// positions that have no ticker.
	Ticker string `json:"ticker,omitempty"`

	// Name is the human-readable name of the security or fund.
	Name string `json:"name"`

	// Shares is the number of units held (fractional shares allowed).
	Shares float64 `json:"shares"`

	// CostBasisMinor is the total acquisition cost in minor currency units
	// (e.g. cents). Used to compute unrealized gain/loss.
	CostBasisMinor int64 `json:"costBasisMinor"`

	// CurrentPriceMinorPerShare is the latest known price per share in minor
	// currency units. Multiply by Shares to get current market value.
	CurrentPriceMinorPerShare int64 `json:"currentPriceMinorPerShare"`

	// AssetClass is a broad category label, e.g. "Stocks", "Bonds", "Cash",
	// "Crypto", "Real Estate". Empty means unclassified.
	AssetClass string `json:"assetClass,omitempty"`

	// SecurityType categorizes the position (stock / ETF / mutual fund / bond /
	// crypto / cash / other), so securities investments read and allocate distinctly.
	// Empty normalizes to "other"; JSON round-trips with no store migration.
	SecurityType SecurityType `json:"securityType,omitempty"`
}
