// Package setup provides helpers for determining whether a new user has
// completed the first-run onboarding wizard and which steps remain. It
// operates entirely on primitive values and domain slices so it can be
// tested natively without any wasm or store dependency.
package setup

import "github.com/monstercameron/CashFlux/internal/domain"

// Step identifies a single onboarding wizard step.
type Step int

const (
	// StepCurrency is the step where the user confirms their base currency.
	StepCurrency Step = iota
	// StepIncome is the step where the user enters their monthly household income.
	StepIncome
	// StepAccount is the step where the user adds at least one account.
	StepAccount
	// StepMembers is the step where the user optionally adds household members.
	StepMembers
)

// String returns a human-readable name for the step, satisfying fmt.Stringer.
func (s Step) String() string {
	switch s {
	case StepCurrency:
		return "Currency"
	case StepIncome:
		return "Income"
	case StepAccount:
		return "Account"
	case StepMembers:
		return "Members"
	default:
		return "Unknown"
	}
}

// Progress records which onboarding steps the current dataset has completed.
// Each field is independently derived from live data so callers can re-compute
// it cheaply on every render without caching.
type Progress struct {
	CurrencyDone bool
	IncomeDone   bool
	AccountDone  bool
	MembersDone  bool
}

// Compute derives a Progress snapshot from the household's current state.
//
//   - CurrencyDone: true when the user has explicitly confirmed a base currency
//     (currencyConfirmed flag, persisted in Settings).
//   - IncomeDone: true when monthlyIncomeMinor > 0 (any positive income recorded).
//   - AccountDone: true when at least one Account exists in the dataset.
//   - MembersDone: true when len(members) >= 2. A solo household has only the
//     default member and is considered "done" for this optional step because
//     household-member setup is not required for a single user.
// TODO(v1.x): the wizard's CurrencyDone uses the explicit currencyConfirmed flag,
// while Help's setup checklist marks currency done whenever a base currency is set
// (always true — USD is the default). So a household configured entirely through
// Settings sees Help say "all set up" but the wizard's dots still show Currency as
// incomplete until they click through. Deliberately left as-is for now: keying the
// wizard off "a base currency exists" would make StepCurrency trivially done and
// cause NextIncompleteStep to skip the currency step on a genuine first run, which
// is worse than the cosmetic signal mismatch. Reconcile by giving both readouts a
// single shared definition of "done" if the onboarding flow is revisited.
func Compute(currencyConfirmed bool, monthlyIncomeMinor int64, accounts []domain.Account, members []domain.Member) Progress {
	return Progress{
		CurrencyDone: currencyConfirmed,
		IncomeDone:   monthlyIncomeMinor > 0,
		AccountDone:  len(accounts) > 0,
		// Two or more members means a multi-person household has been set up.
		// A single member (solo) is treated as complete because the Members step
		// is optional — a user who never adds a second member should not be
		// blocked at this step.
		MembersDone: len(members) >= 2,
	}
}

// AllRequired reports whether all required onboarding steps are done.
//
// Required steps: Currency + Account.
//
// Income is intentionally excluded from the required set here. The monthly-
// income field does not exist in Settings yet; it lands in R12. Until that
// ticket ships, requiring IncomeDone would keep AllRequired permanently false
// for every user.
//
// TODO(R12): add `&& p.IncomeDone` once the Settings.MonthlyIncome field
// lands and Compute can receive a real income value from the store.
func AllRequired(p Progress) bool {
	return p.CurrencyDone && p.AccountDone
}

// NextIncompleteStep returns the first step (in canonical order
// Currency → Income → Account → Members) that is not yet done, together with
// ok=true. When every required step is finished it returns ok=false, signalling
// that the wizard should close (or be marked complete).
//
// Note that MembersDone=false does not by itself keep ok=true if all required
// steps are already done, because Members is optional — NextIncompleteStep
// iterates all four steps in order, but the wizard host is expected to call
// AllRequired to decide whether to show the wizard at all; this function simply
// reports what the next actionable step would be if the wizard is open.
func NextIncompleteStep(p Progress) (Step, bool) {
	steps := []struct {
		step Step
		done bool
	}{
		{StepCurrency, p.CurrencyDone},
		{StepIncome, p.IncomeDone},
		{StepAccount, p.AccountDone},
		{StepMembers, p.MembersDone},
	}
	for _, s := range steps {
		if !s.done {
			return s.step, true
		}
	}
	return 0, false
}

// IsFirstRun reports whether the onboarding wizard should be shown to this user.
//
// The gate is the wizardShownOnce flag stored in Settings, not the account
// count. Gating on account count would re-trigger the wizard for a power user
// who intentionally deletes all accounts and starts fresh after real use, which
// would be disruptive and incorrect. The once-flag is set by the wizard host
// immediately on first open and is never cleared, so the wizard appears exactly
// once per dataset.
func IsFirstRun(wizardShownOnce bool, accounts []domain.Account) bool {
	// accounts is accepted so callers are not forced to change the call-site
	// signature if a future ticket adds account-count logic here, but it is
	// intentionally unused for the reason documented above.
	_ = accounts
	return !wizardShownOnce
}
