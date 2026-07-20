// SPDX-License-Identifier: MIT

package portfolio

// AccountValue pairs one investment account's own ledger balance with the market
// value of the tracked securities held inside it, both in base-currency minor
// units. It is the input to Reconcile — the UI layer builds one per active
// investment account. BalanceMinor is the figure that feeds net worth; a
// balance-tracked ("traditional") account simply carries SecuritiesMinor == 0.
type AccountValue struct {
	AccountID       string
	Name            string
	BalanceMinor    int64
	SecuritiesMinor int64
}

// AccountReconcile is the per-account breakdown Reconcile produces: how much of the
// account's balance is explained by tracked securities versus everything else (cash
// and untracked positions). UntrackedMinor is BalanceMinor − SecuritiesMinor and can
// be negative when the recorded balance lags the holdings' market value — reported
// honestly via BalanceBehind rather than clamped to zero.
type AccountReconcile struct {
	AccountID       string
	Name            string
	BalanceMinor    int64
	SecuritiesMinor int64
	UntrackedMinor  int64
	BalanceBehind   bool
}

// Reconciliation explains how the tracked-securities value relates to the
// investment-account balances that feed net worth, as one exact identity:
//
//	SecuritiesMinor + UntrackedMinor = AccountsTotalMinor
//
// AccountsTotalMinor is the sum of the investment-account balances (what net worth
// counts). SecuritiesMinor is the total market value of tracked holdings.
// UntrackedMinor is the remainder — cash and untracked balance — and is negative
// when recorded balances are behind the holdings' market value (BalanceBehind).
type Reconciliation struct {
	SecuritiesMinor    int64
	UntrackedMinor     int64
	AccountsTotalMinor int64
	BalanceBehind      bool
	Accounts           []AccountReconcile
}

// Reconcile computes the securities / untracked / accounts-total breakdown from the
// per-account balances and their tracked-securities values, in both total and
// per-account form. Accounts preserve the input order. The returned identity always
// holds exactly: SecuritiesMinor + UntrackedMinor == AccountsTotalMinor.
func Reconcile(accts []AccountValue) Reconciliation {
	r := Reconciliation{Accounts: make([]AccountReconcile, 0, len(accts))}
	for _, a := range accts {
		untracked := a.BalanceMinor - a.SecuritiesMinor
		r.SecuritiesMinor += a.SecuritiesMinor
		r.AccountsTotalMinor += a.BalanceMinor
		r.Accounts = append(r.Accounts, AccountReconcile{
			AccountID:       a.AccountID,
			Name:            a.Name,
			BalanceMinor:    a.BalanceMinor,
			SecuritiesMinor: a.SecuritiesMinor,
			UntrackedMinor:  untracked,
			BalanceBehind:   untracked < 0,
		})
	}
	r.UntrackedMinor = r.AccountsTotalMinor - r.SecuritiesMinor
	r.BalanceBehind = r.UntrackedMinor < 0
	return r
}
