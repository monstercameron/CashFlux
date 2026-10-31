// SPDX-License-Identifier: MIT

package txntable

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
	"github.com/monstercameron/CashFlux/internal/splitdraft"
	"github.com/monstercameron/CashFlux/internal/txncalendarview"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnfilter"
	"github.com/monstercameron/CashFlux/internal/txnlinks"
)

// Labels supplies presentation-edge formatting. Functions used by a projection
// must be provided by its caller.
type Labels struct {
	T      Translator
	Ledger func(money.Money, bool) (string, string)
	Money  func(money.Money) string
}

// RowContext holds immutable row lookup data and the explicit current date.
type RowContext struct {
	Index             Index
	Filter            txnfilter.Criteria
	Now               time.Time
	Labels            Labels
	Learned           func(string) bool
	Resolve           func(string) string
	Balances          map[string]money.Money
	Groups            map[string]domain.TxnLink
	GroupTransactions map[string]domain.Transaction
	Rates             currency.Rates
}

// RowInput contains frame values and already-resolved optional smart annotations.
type RowInput struct {
	ID                string
	Description       string
	Payee             string
	Category          string
	Account           string
	Source            string
	Amount            money.Money
	Date              time.Time
	Cleared           bool
	Selected          bool
	SuggestionsReady  bool
	Automatic         bool
	Duplicate         bool
	Spike             string
	SuggestedCategory string
	FollowUpOpen      int
	FollowUpTotal     int
}

// Row is the presentation-independent row read model. Rendering adds callbacks and nodes.
type Row struct {
	ID                  string
	Date                string
	Amount              string
	AmtTone             string
	Desc                string
	Account             string
	Category            string
	CategoryTooltip     string
	CategoryID          string
	SuggestedCategory   string
	Source              string
	AmountMoney         money.Money
	Tags                []string
	MemberNames         []string
	MemberColors        []string
	ShowSuggestions     bool
	OwnersResolved      bool
	Cleared             bool
	Reconciled          bool
	Reviewed            bool
	Queued              bool
	Duplicate           bool
	Unfiled             bool
	AutoCategory        bool
	Selected            bool
	SpikeWhy            string
	Receipts            int
	Attachment          domain.AttachmentRef
	BillAccountID       string
	SubscriptionName    string
	ExcludedFromReports bool
	HasNote             bool
	Note                string
	HasSplits           bool
	IsTransfer          bool
	TransferWith        string
	IsDebtPayment       bool
	SuspectTransfer     bool
	IsIncome            bool
	IsRefund            bool
	IsRefunded          bool
	IsBillMatched       bool
	ShowBalance         bool
	EventName           string
	Balance             string
	BalTone             string
	FollowUpOpen        int
	FollowUpTotal       int
	GroupSize           int
	GroupTotal          string
	InfoText            string
	FlashKey            string
}

// DeriveRow projects one frame row; absent enrichment is valid and nil maps are safe.
func DeriveRow(c RowContext, in RowInput) Row {
	tx := c.Index.Transactions[in.ID]
	category := in.Category
	if strings.TrimSpace(category) == "" {
		category = c.Labels.T("transactions.uncategorized")
	}
	count, splitTooltip := splitdraft.CellSummary(tx)
	if tx.HasSplits() {
		category = c.Labels.T("splitEditor.cellLabel", count)
	}
	if tx.IsTransfer() {
		category = TransferLabel(c.Index.AccountNames[tx.AccountID], c.Index.AccountNames[tx.TransferAccountID], txnclassify.TransferOutgoing(tx, c.Index.Accounts[tx.AccountID]))
	}
	text, tone := c.Labels.Ledger(in.Amount, tx.IsTransfer())
	_, suspect := txnclassify.Suspected(tx, c.Index.CategoryNames[tx.CategoryID])
	through := c.Index.ReconciledThrough[tx.AccountID]
	reconciled := in.Cleared && !through.IsZero() && !tx.Date.After(through)
	r := Row{
		ID:                  in.ID,
		AmountMoney:         in.Amount,
		ShowSuggestions:     in.SuggestionsReady,
		Date:                LedgerDate(in.Date, c.Filter.From, c.Filter.To, c.Now),
		Amount:              text,
		AmtTone:             tone,
		Desc:                Description(in.Description, in.Payee, c.Learned, c.Resolve),
		Tags:                tx.Tags,
		Account:             in.Account,
		Category:            category,
		CategoryID:          tx.CategoryID,
		SuggestedCategory:   in.SuggestedCategory,
		Source:              in.Source,
		MemberNames:         OwnerNames(tx.Owners(), c.Index.MemberNames),
		MemberColors:        OwnerColors(tx.Owners(), c.Index.MemberColors),
		OwnersResolved:      true,
		Cleared:             in.Cleared,
		Reconciled:          reconciled,
		Reviewed:            tx.Reviewed,
		Queued:              reviewqueue.Needs(tx),
		Duplicate:           in.Duplicate,
		SpikeWhy:            in.Spike,
		Unfiled:             tx.CategoryID == "" && !tx.HasSplits(),
		AutoCategory:        in.Automatic,
		Selected:            in.Selected,
		Receipts:            len(tx.Attachments),
		BillAccountID:       tx.BillAccountID,
		SubscriptionName:    tx.SubscriptionName,
		ExcludedFromReports: tx.ExcludeFromReports,
		HasNote:             tx.Note != "",
		Note:                tx.Note,
		HasSplits:           tx.HasSplits(),
		IsTransfer:          tx.IsTransfer(),
		TransferWith:        c.Index.AccountNames[tx.TransferAccountID],
		IsDebtPayment:       tx.BillAccountID != "",
		SuspectTransfer:     suspect,
		IsIncome:            tx.IsIncome(),
		IsRefund:            c.Index.Refund[in.ID],
		IsRefunded:          c.Index.Refunded[in.ID],
		IsBillMatched:       c.Index.BillMatched[in.ID],
		EventName:           c.Index.EventNames[in.ID],
		ShowBalance:         c.Balances != nil,
		FollowUpOpen:        in.FollowUpOpen,
		FollowUpTotal:       in.FollowUpTotal,
		FlashKey:            FlashKey(tx),
	}
	if r.Receipts > 0 {
		r.Attachment = tx.Attachments[0]
	}
	if tx.HasSplits() {
		r.CategoryTooltip = splitTooltip
	} else if tx.IsTransfer() {
		r.CategoryTooltip = c.Labels.T(TransferCategoryKey(tx.Amount), r.TransferWith)
	}
	r.Balance, r.BalTone = txncalendarview.Balance(c.Balances, in.ID)
	if group, ok := c.Groups[in.ID]; ok {
		r.GroupSize = len(group.TxnIDs)
		if total, err := GroupSum(txnlinks.GroupMembers(group, c.GroupTransactions), c.Rates); err == nil {
			r.GroupTotal = c.Labels.Money(total)
		}
	}
	r.InfoText = rowInfo(c.Labels.T, r, splitTooltip)
	return r
}

func rowInfo(t Translator, r Row, splitTooltip string) string {
	info := ""
	if r.FollowUpTotal > 0 {
		key := "uxt5.openFollowUps"
		if r.FollowUpOpen == 1 {
			key = "uxt5.openFollowUp"
		}
		info = Info(info, t(key, r.FollowUpOpen))
	}
	if r.Receipts > 0 {
		key := "uxt5.receipts"
		if r.Receipts == 1 {
			key = "uxt5.receipt"
		}
		info = Info(info, t(key, r.Receipts))
	}
	if r.AutoCategory {
		key := "uxt5.autoAdded"
		if r.Queued {
			key = "uxt5.autoNeedsReview"
		}
		info = Info(info, t(key))
	}
	if r.Reconciled {
		info = Info(info, t("uxt5.reconciled"))
	}
	if r.Duplicate {
		info = Info(info, t("sm8.detail"))
	}
	if r.Queued && (r.Duplicate || r.SpikeWhy != "") {
		info = Info(info, t("uxt5.review"))
	}
	if r.SpikeWhy != "" && (r.Duplicate || r.Queued) {
		info = Info(info, r.SpikeWhy)
	}
	if r.SuspectTransfer {
		info = Info(info, t("transactions.badgeSuspectTransferTitle"))
	}
	if r.EventName != "" {
		info = Info(info, t("events.chipTitle", r.EventName))
	}
	if r.HasSplits {
		info = Info(info, splitTooltip)
	}
	if r.IsDebtPayment {
		with := r.TransferWith
		if with == "" {
			with = t("transactions.badgeDebtPayment")
		}
		info = Info(info, t("transactions.badgeDebtPaymentTitle", with))
	}
	if r.GroupSize > 1 && r.GroupTotal != "" {
		info = Info(info, t("txnlinks.groupBadgeTitle", r.GroupSize, r.GroupTotal))
	}
	if r.IsRefund {
		info = Info(info, t("txnlinks.refundBadge"))
	} else if r.IsRefunded {
		info = Info(info, t("txnlinks.refundedBadge"))
	}
	if r.ExcludedFromReports {
		info = Info(info, t("transactions.excludeHint"))
	}
	return info
}

// TransferLabel names the household movement, requiring both account names.
func TransferLabel(from, to string, outgoing bool) string {
	if strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
		return ""
	}
	if !outgoing {
		from, to = to, from
	}
	return from + " \u2192 " + to
}

// FlashKey hashes the transaction content, preserving the previous fallback for invalid JSON values.
func FlashKey(tx domain.Transaction) string {
	encoded, err := json.Marshal(tx)
	if err != nil {
		return tx.ID
	}
	digest := sha256.Sum256(encoded)
	return tx.ID + ":" + hex.EncodeToString(digest[:])
}

// FrameColumns resolves the table's frame fields once for bounded per-row reads.
type FrameColumns struct {
	ID, Date, Amount, Currency, Payee, Description, Account, Category, Cleared, Source domain.Field
}

// ReadFrame resolves the required fields; missing columns remain safe empty fields.
func ReadFrame(frame domain.Frame) FrameColumns {
	id, _ := frame.Column("id")
	date, _ := frame.Column("date")
	amount, _ := frame.Column("amount")
	currency, _ := frame.Column("currency")
	payee, _ := frame.Column("payee")
	desc, _ := frame.Column("desc")
	account, _ := frame.Column("account")
	category, _ := frame.Column("category")
	cleared, _ := frame.Column("cleared")
	source, _ := frame.Column("source")
	return FrameColumns{id, date, amount, currency, payee, desc, account, category, cleared, source}
}

// RowAt reconstructs calendar dates in UTC and keeps monetary values as int64 minor units.
func (c FrameColumns) RowAt(i int) RowInput {
	cleared := false
	if i >= 0 && i < len(c.Cleared.Values) {
		cleared, _ = c.Cleared.Values[i].(bool)
	}
	return RowInput{ID: c.ID.Str(i), Date: time.Unix(int64(c.Date.Num(i)), 0).UTC(),
		Amount: money.New(c.Amount.Int64(i), c.Currency.Str(i)), Payee: c.Payee.Str(i), Description: c.Description.Str(i),
		Account: c.Account.Str(i), Category: c.Category.Str(i), Cleared: cleared, Source: c.Source.Str(i)}
}

// IndexOf returns the first matching frame ID for focus restoration.
func (c FrameColumns) IndexOf(id string) (int, bool) {
	for i := range c.ID.Values {
		if c.ID.Str(i) == id {
			return i, true
		}
	}
	return 0, false
}

// SuggestionEligible gates optional categorization without changing the transaction.
func SuggestionEligible(row domain.Transaction, ready, enabled bool) bool {
	return ready && enabled && row.CategoryID == "" && !row.HasSplits() && !row.IsTransfer()
}
