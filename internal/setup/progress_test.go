package setup_test

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/setup"
)

// makeAccounts returns a slice of n stub domain.Account values.
func makeAccounts(n int) []domain.Account {
	out := make([]domain.Account, n)
	for i := range out {
		out[i] = domain.Account{ID: "a", Name: "Checking"}
	}
	return out
}

// makeMembers returns a slice of n stub domain.Member values.
func makeMembers(n int) []domain.Member {
	out := make([]domain.Member, n)
	for i := range out {
		out[i] = domain.Member{ID: "m", Name: "Alice"}
	}
	return out
}

// --- Compute ---

func TestCompute(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		currencyConfirmed bool
		monthlyIncome     int64
		accounts          []domain.Account
		members           []domain.Member
		want              setup.Progress
	}{
		{
			name:    "all zero → nothing done",
			want:    setup.Progress{},
		},
		{
			name:              "currency confirmed only",
			currencyConfirmed: true,
			want:              setup.Progress{CurrencyDone: true},
		},
		{
			name:          "income > 0",
			monthlyIncome: 100000,
			want:          setup.Progress{IncomeDone: true},
		},
		{
			name:     "one account",
			accounts: makeAccounts(1),
			want:     setup.Progress{AccountDone: true},
		},
		{
			name:    "one member → MembersDone false (solo household, optional step)",
			members: makeMembers(1),
			want:    setup.Progress{MembersDone: false},
		},
		{
			name:    "two members → MembersDone true",
			members: makeMembers(2),
			want:    setup.Progress{MembersDone: true},
		},
		{
			name:    "three members → MembersDone true",
			members: makeMembers(3),
			want:    setup.Progress{MembersDone: true},
		},
		{
			name:              "all fields satisfied",
			currencyConfirmed: true,
			monthlyIncome:     500000,
			accounts:          makeAccounts(2),
			members:           makeMembers(2),
			want:              setup.Progress{CurrencyDone: true, IncomeDone: true, AccountDone: true, MembersDone: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := setup.Compute(tc.currencyConfirmed, tc.monthlyIncome, tc.accounts, tc.members)
			if got != tc.want {
				t.Errorf("Compute() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// --- AllRequired ---

func TestAllRequired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    setup.Progress
		want bool
	}{
		{
			name: "all zero → not required",
			p:    setup.Progress{},
			want: false,
		},
		{
			name: "currency only → not required",
			p:    setup.Progress{CurrencyDone: true},
			want: false,
		},
		{
			name: "account only → not required",
			p:    setup.Progress{AccountDone: true},
			want: false,
		},
		{
			name: "currency + account → required done",
			p:    setup.Progress{CurrencyDone: true, AccountDone: true},
			want: true,
		},
		{
			name: "currency + account + income → required done (income optional until R12)",
			p:    setup.Progress{CurrencyDone: true, AccountDone: true, IncomeDone: true},
			want: true,
		},
		{
			// Members < 2 means MembersDone=false, but AllRequired must still be true
			// because Members is an optional step.
			name: "currency + account, members not done → AllRequired still true",
			p:    setup.Progress{CurrencyDone: true, AccountDone: true, MembersDone: false},
			want: true,
		},
		{
			name: "all four done → required done",
			p:    setup.Progress{CurrencyDone: true, IncomeDone: true, AccountDone: true, MembersDone: true},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := setup.AllRequired(tc.p)
			if got != tc.want {
				t.Errorf("AllRequired(%+v) = %v, want %v", tc.p, got, tc.want)
			}
		})
	}
}

// --- NextIncompleteStep ---

func TestNextIncompleteStep(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		p        setup.Progress
		wantStep setup.Step
		wantOK   bool
	}{
		{
			name:     "all zero → first step is Currency",
			p:        setup.Progress{},
			wantStep: setup.StepCurrency,
			wantOK:   true,
		},
		{
			name:     "currency done → next is Income",
			p:        setup.Progress{CurrencyDone: true},
			wantStep: setup.StepIncome,
			wantOK:   true,
		},
		{
			name:     "currency + income done → next is Account",
			p:        setup.Progress{CurrencyDone: true, IncomeDone: true},
			wantStep: setup.StepAccount,
			wantOK:   true,
		},
		{
			name:     "currency + income + account done → next is Members",
			p:        setup.Progress{CurrencyDone: true, IncomeDone: true, AccountDone: true},
			wantStep: setup.StepMembers,
			wantOK:   true,
		},
		{
			name:   "all done → ok=false",
			p:      setup.Progress{CurrencyDone: true, IncomeDone: true, AccountDone: true, MembersDone: true},
			wantOK: false,
		},
		{
			// Verifies canonical order: currency is checked before account even if
			// account is already done.
			name:     "account done but currency not → Currency is next",
			p:        setup.Progress{AccountDone: true},
			wantStep: setup.StepCurrency,
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotStep, gotOK := setup.NextIncompleteStep(tc.p)
			if gotOK != tc.wantOK {
				t.Errorf("NextIncompleteStep(%+v) ok = %v, want %v", tc.p, gotOK, tc.wantOK)
			}
			if gotOK && gotStep != tc.wantStep {
				t.Errorf("NextIncompleteStep(%+v) step = %v (%d), want %v (%d)",
					tc.p, gotStep, gotStep, tc.wantStep, tc.wantStep)
			}
		})
	}
}

// --- Step.String ---

func TestStepString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		step setup.Step
		want string
	}{
		{setup.StepCurrency, "Currency"},
		{setup.StepIncome, "Income"},
		{setup.StepAccount, "Account"},
		{setup.StepMembers, "Members"},
		{setup.Step(99), "Unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.step.String(); got != tc.want {
				t.Errorf("Step(%d).String() = %q, want %q", tc.step, got, tc.want)
			}
		})
	}
}

// --- IsFirstRun ---

func TestIsFirstRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		wizardShownOnce  bool
		accounts         []domain.Account
		want             bool
	}{
		{
			name:            "flag false, no accounts → first run",
			wizardShownOnce: false,
			accounts:        nil,
			want:            true,
		},
		{
			name:            "flag false, has accounts → still first run (flag gates, not count)",
			wizardShownOnce: false,
			accounts:        makeAccounts(3),
			want:            true,
		},
		{
			name:            "flag true, no accounts → not first run",
			wizardShownOnce: true,
			accounts:        nil,
			want:            false,
		},
		{
			// Power user wiped all accounts after real use; flag is set, so no re-trigger.
			name:            "flag true, no accounts (post-wipe) → not first run",
			wizardShownOnce: true,
			accounts:        makeAccounts(0),
			want:            false,
		},
		{
			name:            "flag true, has accounts → not first run",
			wizardShownOnce: true,
			accounts:        makeAccounts(2),
			want:            false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := setup.IsFirstRun(tc.wizardShownOnce, tc.accounts)
			if got != tc.want {
				t.Errorf("IsFirstRun(wizardShownOnce=%v, accounts=%d) = %v, want %v",
					tc.wizardShownOnce, len(tc.accounts), got, tc.want)
			}
		})
	}
}
