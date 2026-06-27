// SPDX-License-Identifier: MIT

// Package smart is the pure, platform-independent spine of the SMART feature
// series — CashFlux's optional, per-page intelligence layer. It defines the
// vocabulary every smart feature shares so the UI can treat them uniformly:
//
//   - A Feature is one catalog entry (e.g. SMART-A1 "Balance anomaly watch"),
//     tagged with the Page it lives on and its Tier — Free (deterministic, runs
//     on-device at no cost) or AI (needs an LLM inference provider, so it costs
//     money per call). This split is what lets the UI be honest about cost.
//   - An Insight is one finding a Free engine produces: a glanceable headline,
//     a plain-English reason, a severity, an optional money figure, and an
//     optional one-tap Action. Insights are deterministic and dismissable.
//   - Settings record which features the user has opted into and which insights
//     they have dismissed. Free (deterministic) features are ON by default;
//     AI features are OFF by default. See Settings.IsEnabled.
//
// The package holds NO syscall/js and no transport. Free engines (the rule
// items) are pure functions over domain data that return []Insight. AI features
// (the "[AI]" items) read the catalog for their cost estimate and model routing
// but place their calls from the wasm layer. Everything here is unit-tested on
// native Go.
package smart

import "github.com/monstercameron/CashFlux/internal/money"

// Page is the app page a smart feature belongs to. The values match the SMART
// sub-series prefixes (A=Accounts, T=Transactions, …) so a feature code and its
// page never drift apart.
type Page string

const (
	PageAccounts      Page = "accounts"
	PageTransactions  Page = "transactions"
	PageBudgets       Page = "budgets"
	PageGoals         Page = "goals"
	PageTodos         Page = "todos"
	PagePlanning      Page = "planning"
	PageAllocate      Page = "allocate"
	PageSubscriptions Page = "subscriptions"
	PageBills         Page = "bills"
	// PageHub holds cross-app meta-features (e.g. the proactive digest) that do
	// not belong to one data page. The hub renders them in their own section.
	PageHub Page = "hub"
)

// pageLabels is the human label for each page, used in catalog/settings UI.
var pageLabels = map[Page]string{
	PageAccounts:      "Accounts",
	PageTransactions:  "Transactions",
	PageBudgets:       "Budgets",
	PageGoals:         "Goals",
	PageTodos:         "To-dos",
	PagePlanning:      "Planning",
	PageAllocate:      "Allocate",
	PageSubscriptions: "Subscriptions",
	PageBills:         "Bills",
	PageHub:           "Smart hub",
}

// Valid reports whether p is one of the known pages.
func (p Page) Valid() bool { _, ok := pageLabels[p]; return ok }

// Label returns the human page name, or the raw value if unknown.
func (p Page) Label() string {
	if l, ok := pageLabels[p]; ok {
		return l
	}
	return string(p)
}

// Tier classifies what running a feature costs the user. It is the heart of the
// product's cost-transparency promise: Free features never leave the device and
// never cost a cent; AI features need a configured inference provider and are
// billed per call by that provider.
type Tier string

const (
	// TierFree is a deterministic, on-device feature: rules, math, heuristics
	// over local data. No model, no network, no cost. Private and instant.
	TierFree Tier = "free"
	// TierAI needs an LLM inference provider (the user's own key). It costs money
	// per call and sends the relevant figures to that provider.
	TierAI Tier = "ai"
)

// Valid reports whether t is a known tier.
func (t Tier) Valid() bool { return t == TierFree || t == TierAI }

// Label returns a short human label for the tier.
func (t Tier) Label() string {
	switch t {
	case TierFree:
		return "Free"
	case TierAI:
		return "AI"
	default:
		return string(t)
	}
}

// Severity ranks an insight for sorting and visual weight. Higher severities
// sort first and draw a stronger tone; SeverityInfo is a calm, neutral note.
type Severity int

const (
	// SeverityInfo is a neutral observation ("income posted").
	SeverityInfo Severity = iota
	// SeverityNudge is a gentle, optional suggestion ("consider moving idle cash").
	SeverityNudge
	// SeverityWarn is something worth attention soon ("balance dips before payday").
	SeverityWarn
	// SeverityAlert is the most urgent tone ("a bill looks missed").
	SeverityAlert
)

// String returns the stable lowercase token for the severity.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityNudge:
		return "nudge"
	case SeverityWarn:
		return "warn"
	case SeverityAlert:
		return "alert"
	default:
		return "info"
	}
}

// ActionKind is the type of one-tap follow-up an insight offers.
type ActionKind string

const (
	// ActionNone means the insight is informational; it can still be dismissed.
	ActionNone ActionKind = ""
	// ActionCreateTask turns the insight into a to-do (used by SMART-D1 and any
	// insight the user wants to act on later).
	ActionCreateTask ActionKind = "create_task"
	// ActionNavigate points the user at the relevant screen/entity.
	ActionNavigate ActionKind = "navigate"
	// ActionCreateGoal creates a new goal directly from the insight, using the
	// payload fields GoalName, GoalTarget, and GoalCurrency. On success the app
	// navigates to /goals and shows a confirmation toast.
	ActionCreateGoal ActionKind = "create_goal"
	// ActionCreateRecurring creates a new recurring cash-flow entry from the
	// insight, using RecurringLabel, RecurringAmount, RecurringCurrency, and
	// RecurringCadence. On success the app navigates to /planning.
	ActionCreateRecurring ActionKind = "create_recurring"
	// ActionCancelSubscription marks the named subscription cancelled today,
	// using SubscriptionName. On success the app navigates to /subscriptions.
	ActionCancelSubscription ActionKind = "cancel_subscription"
	// ActionAutomateGoal creates a pay-yourself-first scheduled workflow that
	// transfers GoalMonthlyAmount from a funding account to the goal's linked
	// account each month. On success the app shows a confirmation toast and
	// navigates to /planning. GoalID must be set; GoalMonthlyAmount is the
	// monthly contribution in the goal's currency minor units.
	ActionAutomateGoal ActionKind = "automate_goal"
)

// Action is an optional, single-tap follow-up attached to an Insight. It is
// declarative data — the wasm/UI layer interprets it — so this package stays
// pure. Fields are populated per Kind; unused fields stay zero.
type Action struct {
	Kind  ActionKind
	Label string // button text, e.g. "Add a to-do"

	// ActionCreateTask payload.
	TaskTitle string
	TaskNotes string

	// ActionNavigate payload.
	Route string // app route, e.g. "/subscriptions"

	// ActionCreateGoal payload. GoalCurrency defaults to the app base currency
	// when empty; GoalTarget is in minor units of GoalCurrency.
	GoalName     string
	GoalTarget   int64
	GoalCurrency string

	// ActionCreateRecurring payload. RecurringAmount is in minor units of
	// RecurringCurrency; RecurringCadence must be a valid domain.RecurringCadence
	// value ("weekly", "monthly", "quarterly", "yearly").
	RecurringLabel    string
	RecurringAmount   int64
	RecurringCurrency string
	RecurringCadence  string

	// ActionCancelSubscription payload.
	SubscriptionName string

	// ActionAutomateGoal payload. GoalID identifies the goal; GoalMonthlyAmount
	// is the recommended monthly contribution in the goal's currency minor units.
	GoalID            string
	GoalMonthlyAmount int64

	// Optional link back to the subject entity, for either kind.
	RelatedType string // e.g. "account", "transaction", "goal", "bill"
	RelatedID   string
}

// Insight is one deterministic finding from a Free (rule) engine. It is built to
// be glanceable first (Title + Severity + optional Amount) and explainable on
// demand (Detail). Key is a stable identity so a dismissal sticks across reloads
// even as the underlying data shifts slightly.
type Insight struct {
	// Feature is the catalog code that produced it (e.g. "SMART-A1").
	Feature string
	// Page is where the insight surfaces.
	Page Page
	// Key uniquely and stably identifies this insight for dismissal. Engines
	// build it from the feature code plus the subject (e.g. an account id), NOT
	// from volatile values, so a dismissed insight does not re-appear on every
	// recompute.
	Key string
	// Title is the glanceable headline in plain English.
	Title string
	// Detail is the one- or two-line reason / explanation.
	Detail string
	// Severity drives sort order and tone.
	Severity Severity
	// Amount is an optional headline figure (a saving, a shortfall, a balance).
	// Valid only when HasAmount is true.
	Amount    money.Money
	HasAmount bool
	// Action is an optional one-tap follow-up; nil means info-only.
	Action *Action
}

// WithAmount returns a copy of the insight carrying the given money figure as
// its headline amount. It is a small builder convenience for engines.
func (i Insight) WithAmount(m money.Money) Insight {
	i.Amount = m
	i.HasAmount = true
	return i
}

// WithAction returns a copy of the insight carrying the given action.
func (i Insight) WithAction(a Action) Insight {
	i.Action = &a
	return i
}
