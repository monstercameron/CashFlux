// SPDX-License-Identifier: MIT

// Package txnform prepares transaction editor drafts and presentation decisions.
// It has no persistence or browser dependencies; callers commit a successful plan.
package txnform

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/amountmath"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/catname"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/textutil"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnmove"
	"github.com/monstercameron/CashFlux/internal/txnprov"
	"github.com/monstercameron/CashFlux/internal/txnreview"
)

// Draft holds the editable facts independently of the stored transaction.
type Draft struct {
	Desc, Payee, Amount, Direction, CategoryID, Date, Tags, Note string
	AccountID, TransferAccountID                                 string
	Owners                                                       []string
	Cleared, Exclude, DebtPayment                                bool
}

// Seed creates initial field values. A missing row has blank amount/date fields.
// An unrepresentable magnitude remains signed so no amount is silently changed.
func Seed(t domain.Transaction, found bool) Draft {
	d := Draft{Desc: t.Desc, Payee: t.Payee, Direction: "out", CategoryID: t.CategoryID,
		Tags: strings.Join(t.Tags, ", "), Note: t.Note, AccountID: t.AccountID,
		TransferAccountID: t.TransferAccountID, Owners: t.Owners(), Cleared: t.Cleared,
		Exclude: t.ExcludeFromReports, DebtPayment: t.BillAccountID != ""}
	if t.Amount.IsPositive() {
		d.Direction = "in"
	}
	if found {
		magnitude, err := t.Amount.AbsChecked()
		if err != nil {
			magnitude = t.Amount
		}
		d.Amount = money.FormatMinor(magnitude.Amount, currency.Decimals(t.Amount.Currency))
		d.Date = dateutil.FormatDate(t.Date)
	}
	return d
}

// Plan contains a validated replacement and any field error or refused move.
type Plan struct {
	Transaction          domain.Transaction
	SyncDirectionOut     bool
	UnclassifiesTransfer bool
	Failure              Message
	Err                  error
}

// Build validates a draft without mutating its source or writing anything.
func Build(original domain.Transaction, d Draft, accounts []domain.Account, txns []domain.Transaction, now time.Time) Plan {
	t := original
	want := amountmath.Out
	if d.Direction == "in" {
		want = amountmath.In
	}
	// Resolve the sign using checked operations; MinInt64 is a valid expense
	// even though its positive magnitude cannot be represented.
	parsed, err := money.ParseMinor(strings.TrimSpace(d.Amount), currency.Decimals(t.Amount.Currency))
	amt, resolved := parsed, want
	if err == nil {
		if parsed == 0 {
			err = amountmath.ErrZeroAmount
		} else if parsed < 0 {
			resolved = amountmath.Out
		} else if want == amountmath.Out {
			amt, err = money.Neg(parsed)
		}
	}
	if err == nil && t.IsTransfer() {
		amt = editMagnitude(amt)
		if t.Amount.IsNegative() {
			negative, negErr := money.Neg(amt)
			if negErr == nil {
				amt = negative
			}
		}
	}

	if err != nil {
		key := "transactions.amountNotANumber"
		if errors.Is(err, amountmath.ErrZeroAmount) {
			key = "transactions.positiveAmount"
		}
		return Plan{Failure: Message{Key: key}, Err: fmt.Errorf("transaction draft amount: %w", err)}
	}
	date, err := dateutil.ParseDate(strings.TrimSpace(d.Date))
	if err != nil {
		return Plan{Failure: Message{Key: "transactions.invalidDate"}, Err: fmt.Errorf("transaction draft date: %w", err)}
	}
	t.Desc = strings.TrimSpace(d.Desc)
	t.Payee = strings.TrimSpace(d.Payee)
	t.Amount = money.New(amt, t.Amount.Currency)
	confirms := txnprov.ConfirmsCategory(original, domain.Transaction{CategoryID: d.CategoryID})
	t.CategoryID = d.CategoryID
	t.Date = date
	t.SetOwners(append([]string(nil), d.Owners...))
	t.Tags = textutil.CommaFields(d.Tags)
	if confirms {
		t = txnreview.Confirm(t, d.CategoryID)
	}
	t = t.MarkCleared(d.Cleared, now)
	t.Note = strings.TrimSpace(d.Note)
	t.ExcludeFromReports = d.Exclude
	t, err = txnclassify.Apply(t, d.TransferAccountID, d.DebtPayment && IsLiability(accounts, d.TransferAccountID), accounts)
	if err != nil {
		return Plan{Err: fmt.Errorf("transaction draft classification: %w", err)}
	}
	if t.HasSplits() && !t.SplitsReconcile() {
		return Plan{Failure: Message{Key: "transactions.splitAmountMismatch"}, Err: errors.New("transaction draft: splits do not reconcile")}
	}
	if d.AccountID != "" && d.AccountID != t.AccountID {
		move := txnmove.PlanMove(original, d.AccountID, accounts, txns)
		if move.Err != nil {
			args := []any{move.ToName}
			if move.Reason == txnmove.ReasonCurrency {
				args = []any{t.Amount.Currency, move.ToName}
			}
			return Plan{Failure: Message{Key: MoveErrorKey(move.Reason), Args: args}, Err: move.Err}
		}
		t.AccountID = d.AccountID
	}
	return Plan{Transaction: t, SyncDirectionOut: !original.IsTransfer() && resolved != want, UnclassifiesTransfer: original.IsTransfer() && !t.IsTransfer()}
}

// MoveErrorKey selects the existing translation for an account move refusal.
func MoveErrorKey(reason txnmove.Reason) string {
	switch reason {
	case txnmove.ReasonCurrency:
		return "transactions.accountMoveCurrency"
	case txnmove.ReasonSelfTransfer:
		return "transactions.accountMoveSelf"
	default:
		return "transactions.accountMoveRefused"
	}
}

// IsLiability reports whether a selected account is a card or loan.
func IsLiability(accounts []domain.Account, id string) bool {
	if id == "" {
		return false
	}
	a, ok := domain.AccountByID(accounts, id)
	return ok && a.Class == domain.ClassLiability
}

// SuggestionTransaction applies the draft magnitude and direction for matching.
// Unlike save, a typed sign here does not override the direction control.
func SuggestionTransaction(t domain.Transaction, raw, direction string) domain.Transaction {
	amount, err := money.ParseMinor(strings.TrimSpace(raw), currency.Decimals(t.Amount.Currency))
	if err != nil {
		amount = 0
	} else {
		amount = editMagnitude(amount)
	}
	if direction != "in" {
		negative, negErr := money.Neg(amount)
		if negErr == nil {
			amount = negative
		}
	}

	t.Amount = money.New(amount, t.Amount.Currency)
	return t
}

// Option is a UI-independent picker value and label.
type Option struct{ Value, Label string }

// Categories filters by direction and sorts full paths in stable natural order.
func Categories(categories []domain.Category, direction string) ([]Option, domain.CategoryKind) {
	kind := domain.KindExpense
	if direction == "in" {
		kind = domain.KindIncome
	}
	matching := make([]domain.Category, 0, len(categories))
	for _, c := range categories {
		if c.Kind == kind {
			matching = append(matching, c)
		}
	}
	options := make([]Option, 0, len(matching))
	for _, c := range matching {
		options = append(options, Option{c.ID, catname.Path(matching, c)})
	}
	sort.SliceStable(options, func(i, j int) bool { return catname.Less(options[i].Label, options[j].Label) })
	return options, kind
}

// SplitLine is a resolved read-only split line. Empty Category means uncategorized.
type SplitLine struct {
	Category, Owner string
	Amount          money.Money
}

// Splits resolves category/owner names while retaining split order and currency.
func Splits(t domain.Transaction, categories []domain.Category, members []domain.Member) []SplitLine {
	cats := make(map[string]string, len(categories))
	owners := make(map[string]string, len(members))
	for _, c := range categories {
		cats[c.ID] = c.Name
	}
	for _, m := range members {
		owners[m.ID] = m.Name
	}
	lines := make([]SplitLine, 0, len(t.Splits))
	for _, s := range t.Splits {
		owner := ""
		if s.MemberID != "" {
			owner = owners[s.MemberID]
			if owner == "" {
				owner = s.MemberID
			}
		}
		lines = append(lines, SplitLine{cats[s.CategoryID], owner, s.Amount})
	}
	return lines
}

// SimilarWrites confirms only still-present, non-transfer candidates, in offer order.
func SimilarWrites(candidates []similartxns.Candidate, live []domain.Transaction, category string) ([]domain.Transaction, []string) {
	byID := make(map[string]domain.Transaction, len(live))
	for _, t := range live {
		byID[t.ID] = t
	}
	var writes []domain.Transaction
	var ids []string
	for _, c := range candidates {
		if t, ok := byID[c.Txn.ID]; ok && !t.IsTransfer() {
			writes = append(writes, txnreview.Confirm(t, category))
			ids = append(ids, t.ID)
		}
	}
	return writes, ids
}

// Phrase returns the payee or, when absent, description for a rule or task.
func Phrase(t domain.Transaction) string {
	if t.Payee != "" {
		return strings.TrimSpace(t.Payee)
	}
	return strings.TrimSpace(t.Desc)
}

// LearnAlias returns trimmed names and whether a rename merits an alias offer.
func LearnAlias(original, newPayee string, hasLearned bool) (string, string, bool) {
	old, newName := strings.TrimSpace(original), strings.TrimSpace(newPayee)
	return old, newName, old != "" && newName != "" && !strings.EqualFold(old, newName) && !hasLearned
}

// ToggleOwner copies the selection before toggling the first matching owner.
func ToggleOwner(owners []string, owner string) []string {
	for i, v := range owners {
		if v == owner {
			return append(append([]string(nil), owners[:i]...), owners[i+1:]...)
		}
	}
	return append(append([]string(nil), owners...), owner)
}

// SplitKey selects the existing split-toggle label.
func SplitKey(open, hasSplits bool) string {
	if open {
		return "splitEditor.hide"
	}
	if hasSplits {
		return "splitEditor.edit"
	}
	return "splitEditor.toggle"
}

// ReceiptKey selects the existing attachment count label.
func ReceiptKey(count int) string {
	if count == 1 {
		return "transactions.receiptAttached"
	}
	return "transactions.receiptsAttached"
}

// FieldErrors maps a translated save error to its inline field without changing copy.
func FieldErrors(message, positive, number, date string) (string, string) {
	amountErr, dateErr := "", ""
	if message == positive || message == number {
		amountErr = message
	}
	if message == date {
		dateErr = message
	}
	return amountErr, dateErr
}

// Thumbnail is an attachment and the available artifact data URL.
type Thumbnail struct {
	Ref     domain.AttachmentRef
	DataURL string
}

// Thumbnails resolves attachment bytes without dropping missing artifacts.
func Thumbnails(refs []domain.AttachmentRef, arts []domain.Artifact) []Thumbnail {
	byID := make(map[string]domain.Artifact, len(arts))
	for _, a := range arts {
		byID[a.ID] = a
	}
	out := make([]Thumbnail, 0, len(refs))
	for _, ref := range refs {
		a := byID[ref.ArtifactID]
		url := ""
		if len(a.Bytes) > 0 {
			url = artifacts.DataURL(a.MIME, a.Bytes)
		}
		out = append(out, Thumbnail{ref, url})
	}
	return out
}

// SparklinePoints maps amounts onto the editor's 120 by 24 SVG. It retains
// the flat fallback when the historical signed range cannot be represented.
func SparklinePoints(series []int64) string {
	if len(series) < 2 {
		return ""
	}
	low, high := series[0], series[0]
	for _, v := range series {
		if v < low {
			low = v
		}
		if v > high {
			high = v
		}
	}
	span, spanErr := money.Sub(high, low)
	points := make([]string, len(series))
	for i, v := range series {
		x := float64(i) * 120 / float64(len(series)-1)
		y := 12.0
		if spanErr == nil && span > 0 {
			// A valid nonnegative range guarantees every difference fits too.
			diff, _ := money.Sub(v, low)
			y = 24 - float64(diff)*22/float64(span) - 1
		}
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	return strings.Join(points, " ")
}

// editMagnitude preserves the editor's existing signed fallback for MinInt64.
// Its positive magnitude cannot fit in Money. Changing that policy belongs in
// a separate user-facing validation change, rather than a component extraction.
func editMagnitude(amount int64) int64 {
	magnitude, err := money.Abs(amount)
	if err != nil {
		return amount
	}
	return magnitude
}

// AccountChoices supplies the editor's sorted accounts and movement counterparties.
func AccountChoices(accounts []domain.Account, t domain.Transaction) ([]domain.Account, []txnclassify.Choice) {
	sorted := acctsort.ForPicker(accounts)
	return sorted, txnclassify.Counterparties(t, sorted)
}

// RulePhrase uses a nonblank trimmed payee, falling back to the description.
func RulePhrase(t domain.Transaction) string {
	phrase := strings.TrimSpace(t.Payee)
	if phrase == "" {
		phrase = strings.TrimSpace(t.Desc)
	}
	return phrase
}

// SuggestionName resolves a proposed category only when it differs from selection.
func SuggestionName(categories []domain.Category, id, selected string, available bool) (string, bool) {
	if !available || id == selected {
		return "", false
	}
	for _, c := range categories {
		if c.ID == id {
			return c.Name, true
		}
	}
	return "", false
}

// Members supplies picker options and colors in member order.
func Members(members []domain.Member) ([]Option, map[string]string) {
	options := make([]Option, 0, len(members))
	colors := make(map[string]string, len(members))
	for _, m := range members {
		options = append(options, Option{m.ID, m.Name})
		colors[m.ID] = m.Color
	}
	return options, colors
}
