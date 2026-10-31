// SPDX-License-Identifier: MIT

package accountselect

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalview"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Transfer form errors. ParseTransfer wraps one of them, so callers choose the
// user-facing message with errors.Is.
var (
	// ErrTransferAmount means the amount is not a positive amount in the source currency.
	ErrTransferAmount = errors.New("accountselect: transfer amount must be a positive amount")
	// ErrTransferExtra means the received amount or the fee is not a valid amount.
	ErrTransferExtra = errors.New("accountselect: received amount and fee must be valid amounts")
	// ErrTransferSameAccount means the source and destination are the same account.
	ErrTransferSameAccount = errors.New("accountselect: transfer source and destination must differ")
	// ErrTransferUnknownAccount means the source or destination is not an existing account.
	ErrTransferUnknownAccount = errors.New("accountselect: transfer account does not exist")
	// ErrTransferAccountNotAllowed means the source or destination exists but a
	// transfer cannot use it: it is archived, the source is not liquid cash, or the
	// destination is a valuation-only holding.
	ErrTransferAccountNotAllowed = errors.New("accountselect: transfer account cannot be used for a transfer")
	// ErrTransferDate means the date is not a valid calendar date.
	ErrTransferDate = errors.New("accountselect: transfer date is not a valid date")
)

// TransferCandidate is one account offered in a transfer picker.
type TransferCandidate struct {
	ID string
	// Label is the account name, with its currency code appended unless the name
	// already carries it.
	Label string
	// LiabilityPayment is true when picking this account as the destination pays
	// down a debt.
	LiabilityPayment bool
}

// TransferCandidates lists the accounts a transfer can start from and land in.
// Money moves from liquid cash only, so a retirement account or a condo cannot be
// a source. It lands anywhere except a valuation-only holding (property or
// vehicle). Archived accounts are never offered, and each side excludes the
// account picked on the other. Both lists are sorted for picking.
func TransferCandidates(accounts []domain.Account, fromID, toID string) (from, to []TransferCandidate) {
	for _, a := range acctsort.ForPicker(accounts) {
		if a.Archived {
			continue
		}
		label := a.Name
		if a.Currency != "" && !strings.Contains(strings.ToUpper(a.Name), "("+strings.ToUpper(a.Currency)+")") {
			label += " (" + a.Currency + ")"
		}
		if a.ID != toID && canTransferFrom(a) {
			from = append(from, TransferCandidate{ID: a.ID, Label: label})
		}
		if a.ID != fromID && canTransferTo(a) {
			to = append(to, TransferCandidate{ID: a.ID, Label: label, LiabilityPayment: a.Class == domain.ClassLiability})
		}
	}
	return from, to
}

// canTransferFrom reports whether money can leave the account: it is active and
// holds liquid cash.
func canTransferFrom(a domain.Account) bool {
	return !a.Archived && goalview.LiquidSource(a.Type)
}

// canTransferTo reports whether money can land in the account: it is active and
// not a valuation-only holding (property or vehicle).
func canTransferTo(a domain.Account) bool {
	return !a.Archived && a.Type != domain.TypeProperty && a.Type != domain.TypeVehicle
}

func findAccount(accounts []domain.Account, id string) (domain.Account, bool) {
	if id == "" {
		return domain.Account{}, false
	}
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Account{}, false
}

// checkTransferAccounts applies the TransferCandidates rules to a chosen pair:
// both accounts must exist, the source must be an active liquid account and the
// destination an active account that can receive money. It wraps
// ErrTransferUnknownAccount or ErrTransferAccountNotAllowed.
func checkTransferAccounts(accounts []domain.Account, fromID, toID string) error {
	from, ok := findAccount(accounts, fromID)
	if !ok {
		return fmt.Errorf("%w: %q", ErrTransferUnknownAccount, fromID)
	}
	to, ok := findAccount(accounts, toID)
	if !ok {
		return fmt.Errorf("%w: %q", ErrTransferUnknownAccount, toID)
	}
	if !canTransferFrom(from) {
		return fmt.Errorf("%w: source %q", ErrTransferAccountNotAllowed, fromID)
	}
	if !canTransferTo(to) {
		return fmt.Errorf("%w: destination %q", ErrTransferAccountNotAllowed, toID)
	}
	return nil
}

// CurrencyOf returns the currency code of the account, or "" when the id is empty
// or unknown.
func CurrencyOf(accounts []domain.Account, accountID string) string {
	if accountID == "" {
		return ""
	}
	for _, a := range accounts {
		if a.ID == accountID {
			return a.Currency
		}
	}
	return ""
}

// ParseOptionalMinor parses an optional amount field in the given currency. A blank
// field and an explicit zero both mean none (0, true). Anything else must parse as
// a non-negative amount.
func ParseOptionalMinor(s, ccy string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, true
	}
	v, err := money.ParseMinor(s, currency.Decimals(ccy))
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// AmountOK reports whether amount parses as a positive amount in the source
// account's currency.
func AmountOK(accounts []domain.Account, fromID, amount string) bool {
	v, err := parseAmount(amount, CurrencyOf(accounts, fromID))
	return err == nil && v > 0
}

func parseAmount(s, ccy string) (int64, error) {
	return money.ParseMinor(strings.TrimSpace(s), currency.Decimals(ccy))
}

// TransferInput is the raw text of the transfer form.
type TransferInput struct {
	FromID, ToID string
	// Amount is in the source currency. Received is the optional landed amount in the
	// destination currency and Fee the optional fee in the source currency.
	Amount, Received, Fee string
	// Date is YYYY-MM-DD. Blank means the transfer posts now.
	Date string
	// Desc is the description. Blank takes DefaultDesc.
	Desc        string
	DefaultDesc string
}

// ParseTransfer validates the transfer form and returns the parameters to post.
// The accounts must satisfy the TransferCandidates rules. The error wraps
// ErrTransferSameAccount, ErrTransferUnknownAccount, ErrTransferAccountNotAllowed,
// ErrTransferAmount, ErrTransferExtra or ErrTransferDate; an ErrTransferExtra
// error names whether the received amount or the fee is the bad one.
func ParseTransfer(accounts []domain.Account, in TransferInput) (appstate.TransferParams, error) {
	if in.FromID != "" && in.FromID == in.ToID {
		return appstate.TransferParams{}, fmt.Errorf("%w: %q", ErrTransferSameAccount, in.FromID)
	}
	if err := checkTransferAccounts(accounts, in.FromID, in.ToID); err != nil {
		return appstate.TransferParams{}, err
	}
	fromCcy := CurrencyOf(accounts, in.FromID)
	amount, err := parseAmount(in.Amount, fromCcy)
	if err != nil {
		return appstate.TransferParams{}, fmt.Errorf("%w: %w", ErrTransferAmount, err)
	}
	if amount <= 0 {
		return appstate.TransferParams{}, fmt.Errorf("%w: %d", ErrTransferAmount, amount)
	}
	received, receivedOK := ParseOptionalMinor(in.Received, CurrencyOf(accounts, in.ToID))
	fee, feeOK := ParseOptionalMinor(in.Fee, fromCcy)
	if !receivedOK {
		return appstate.TransferParams{}, fmt.Errorf("%w: received amount %q", ErrTransferExtra, strings.TrimSpace(in.Received))
	}
	if !feeOK {
		return appstate.TransferParams{}, fmt.Errorf("%w: fee %q", ErrTransferExtra, strings.TrimSpace(in.Fee))
	}
	var when time.Time
	if date := strings.TrimSpace(in.Date); date != "" {
		parsed, err := dateutil.ParseDate(date)
		if err != nil {
			return appstate.TransferParams{}, fmt.Errorf("%w: %w", ErrTransferDate, err)
		}
		when = parsed
	}
	desc := strings.TrimSpace(in.Desc)
	if desc == "" {
		desc = in.DefaultDesc
	}
	return appstate.TransferParams{
		FromAccountID: in.FromID, ToAccountID: in.ToID, AmountMinor: amount,
		ReceivedMinor: received, FeeMinor: fee, Date: when, Desc: desc,
	}, nil
}

// FXKind says what a cross-currency transfer's note has to tell the user.
type FXKind int

const (
	// FXNone means no note: the pair is incomplete or shares a currency.
	FXNone FXKind = iota
	// FXConverted means the amount lands converted at the saved rate.
	FXConverted
	// FXNoRate means no saved rate covers the pair, so the amount would land unconverted.
	FXNoRate
)

// FXNote is the cross-currency reading of a transfer before it posts.
type FXNote struct {
	Kind FXKind
	// From and To are the two currency codes.
	From, To string
	// Landed is the typed amount converted into the destination currency. It is set
	// for FXConverted. Until an amount is typed it previews one whole unit.
	Landed money.Money
}

// TransferFXNote reads the cross-currency meaning of moving amount from one account
// to another at the saved rates.
func TransferFXNote(accounts []domain.Account, rates currency.Rates, fromID, toID, amount string) FXNote {
	if fromID == "" || toID == "" {
		return FXNote{}
	}
	if !IsCrossCurrency(accounts, fromID, toID) {
		return FXNote{}
	}
	from, _ := findAccount(accounts, fromID)
	to, _ := findAccount(accounts, toID)
	typed, err := parseAmount(amount, from.Currency)
	if err != nil || typed <= 0 {
		typed = oneUnit(from.Currency)
	}
	landed, err := rates.Convert(money.New(typed, from.Currency), to.Currency)
	if err != nil {
		return FXNote{Kind: FXNoRate, From: from.Currency, To: to.Currency}
	}
	return FXNote{Kind: FXConverted, From: from.Currency, To: to.Currency, Landed: landed}
}

// oneUnit is one whole unit of the currency in minor units.
func oneUnit(ccy string) int64 {
	unit := int64(1)
	for i := 0; i < currency.Decimals(ccy); i++ {
		unit *= 10
	}
	return unit
}

// BalancePreview is a transfer ready to preview: the parameters to price, the two
// account names and whether the destination is a debt being paid down.
type BalancePreview struct {
	Params           appstate.TransferParams
	FromName, ToName string
	ToLiability      bool
}

// PreviewTransfer assembles the preview of a transfer, reporting false until both
// accounts are picked, different, existing and allowed by the TransferCandidates
// rules, and the amount is positive. Received and fee
// text that does not parse counts as absent, because the form gates submit
// separately.
func PreviewTransfer(accounts []domain.Account, fromID, toID, amount, received, fee string) (BalancePreview, bool) {
	if fromID == "" || toID == "" || fromID == toID || checkTransferAccounts(accounts, fromID, toID) != nil {
		return BalancePreview{}, false
	}
	fromCcy := CurrencyOf(accounts, fromID)
	amountMinor, err := parseAmount(amount, fromCcy)
	if err != nil || amountMinor <= 0 {
		return BalancePreview{}, false
	}
	receivedMinor, _ := ParseOptionalMinor(received, CurrencyOf(accounts, toID))
	feeMinor, _ := ParseOptionalMinor(fee, fromCcy)
	p := BalancePreview{Params: appstate.TransferParams{
		FromAccountID: fromID, ToAccountID: toID, AmountMinor: amountMinor,
		ReceivedMinor: receivedMinor, FeeMinor: feeMinor,
	}}
	for _, a := range accounts {
		switch a.ID {
		case fromID:
			p.FromName = a.Name
		case toID:
			p.ToName = a.Name
			p.ToLiability = a.Class == domain.ClassLiability
		}
	}
	return p, true
}
