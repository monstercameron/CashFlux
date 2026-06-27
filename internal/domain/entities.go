// SPDX-License-Identifier: MIT

package domain

import (
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
	IncludeInPayoff *bool       `json:"includeInPayoff,omitempty"` // nil = default (every liability but a mortgage)

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
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Kind       CategoryKind   `json:"kind"`
	Color      string         `json:"color,omitempty"`
	ParentID   string         `json:"parentId,omitempty"`
	Deductible bool           `json:"deductible,omitempty"`
	Custom     map[string]any `json:"custom,omitempty"`
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
	Attachments       []AttachmentRef `json:"attachments,omitempty"`
	Custom            map[string]any  `json:"custom,omitempty"`
	// Reviewed marks an entry the user has explicitly confirmed on entry, so
	// auto-review workflows (ActionFlagReview) skip tagging it "needs-review"
	// (L43 — suppress the auto-tag on confident manual entry).
	Reviewed bool `json:"reviewed,omitempty"`
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
	CadenceWeekly      RecurringCadence = "weekly"
	CadenceBiweekly    RecurringCadence = "biweekly"    // every 14 days (C152) — common payday/bill cycle
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
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Scope      Scope          `json:"scope"`
	OwnerID    string         `json:"ownerId"`
	CategoryID string         `json:"categoryId"`
	Period     Period         `json:"period"`
	Limit      money.Money    `json:"limit"`
	Rollover   bool           `json:"rollover,omitempty"`
	Custom     map[string]any `json:"custom,omitempty"`
}

// Goal is a savings target, individual or shared.
type Goal struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Scope         Scope       `json:"scope"`
	OwnerID       string      `json:"ownerId"`
	TargetAmount  money.Money `json:"targetAmount"`
	CurrentAmount money.Money `json:"currentAmount"`
	TargetDate    time.Time   `json:"targetDate,omitempty"`
	AccountID     string      `json:"accountId,omitempty"`
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
}

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
	ID            string    `json:"id"`
	AccountID     string    `json:"accountId"`
	BalanceMinor  int64     `json:"balanceMinor"`
	Currency      string    `json:"currency,omitempty"`
	AsOf          time.Time `json:"asOf"`
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
}
