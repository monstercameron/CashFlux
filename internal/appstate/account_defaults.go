// SPDX-License-Identifier: MIT

package appstate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/validate"
)

var (
	// ErrMemberNotFound is returned when an operation names a member that does not exist.
	ErrMemberNotFound = errors.New("appstate: member not found")
	// ErrDefaultAccountArchived is returned when an archived account is offered as a default.
	ErrDefaultAccountArchived = errors.New("appstate: archived account cannot be a default")
)

// SetMemberDefaultAccount makes the account the member's default for new
// transactions and returns the member's display name. The account must exist and
// must not be archived. The error wraps ErrMemberNotFound, ErrAccountNotFound or
// ErrDefaultAccountArchived for those causes. Like the household default account, it needs
// edit rights rather than member-management rights, so an editor can choose a default account
// without being able to add or remove members.
func (a *App) SetMemberDefaultAccount(memberID, accountID string) (string, error) {
	if err := a.roleGuard(); err != nil {
		return "", err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", fmt.Errorf("appstate: member default account id is required")
	}
	account, found, err := a.store.GetAccount(accountID)
	if err != nil {
		return "", fmt.Errorf("appstate: validate default account %q: %w", accountID, err)
	}
	if !found {
		return "", fmt.Errorf("%w: %q", ErrAccountNotFound, accountID)
	}
	if account.Archived {
		return "", fmt.Errorf("%w: %q", ErrDefaultAccountArchived, accountID)
	}
	return a.writeMemberDefaultAccount(memberID, accountID)
}

// ClearMemberDefaultAccount removes the member's default account and returns the
// member's display name. The error wraps ErrMemberNotFound when no member has
// memberID.
func (a *App) ClearMemberDefaultAccount(memberID string) (string, error) {
	if err := a.roleGuard(); err != nil {
		return "", err
	}
	return a.writeMemberDefaultAccount(memberID, "")
}

// writeMemberDefaultAccount stores accountID, or clears the default when it is
// empty, on the member's preferences.
func (a *App) writeMemberDefaultAccount(memberID, accountID string) (string, error) {
	for _, member := range a.Members() {
		if member.ID != memberID {
			continue
		}
		member.Prefs.DefaultAccountID = accountID
		if is := validate.ValidateMember(member); !is.OK() {
			return "", fmt.Errorf("appstate: save default account for member %q: %w", memberID, is)
		}
		if err := a.store.PutMember(member); err != nil {
			return "", fmt.Errorf("appstate: save default account for member %q: %w", memberID, err)
		}
		a.log.Info("member default account saved", "id", memberID)
		return member.Name, nil
	}
	return "", fmt.Errorf("%w: %q", ErrMemberNotFound, memberID)
}
