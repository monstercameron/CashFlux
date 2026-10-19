// SPDX-License-Identifier: MIT
package txnform

import (
	"strings"
	"time"
	"unicode"

	"github.com/monstercameron/CashFlux/internal/acctsort"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/merchantstats"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/payeealias"
	"github.com/monstercameron/CashFlux/internal/similartxns"
	"github.com/monstercameron/CashFlux/internal/txnclassify"
	"github.com/monstercameron/CashFlux/internal/txnmove"
)

// Message names existing translated copy and its interpolation arguments.
type Message struct {
	Key  string
	Args []any
}

// AccountView holds the options and consequences of changing the posted account.
type AccountView struct {
	Show    bool
	Options []Option
	Refusal Message
	Lines   []Message
}

// AccountPreview builds the account picker and move explanation without writing.
func AccountPreview(t domain.Transaction, selected string, accounts []domain.Account, txns []domain.Transaction) AccountView {
	ordered := acctsort.ForPicker(accounts)
	v := AccountView{Show: len(ordered) >= 2, Options: accountOptions(ordered, selected)}
	if selected == "" || selected == t.AccountID {
		return v
	}
	p := txnmove.PlanMove(t, selected, ordered, txns)
	if p.Err != nil {
		v.Refusal = Message{Key: MoveErrorKey(p.Reason), Args: []any{p.ToName}}
		if p.Reason == txnmove.ReasonCurrency {
			v.Refusal.Args = []any{t.Amount.Currency, p.ToName}
		}
		return v
	}
	v.Lines = []Message{{Key: "transactions.accountMoveEffect", Args: []any{p.FromName, p.ToName}}}
	if p.HasPartner {
		v.Lines = append(v.Lines, Message{Key: "transactions.accountMovePartner"})
	}
	if p.BreaksReconciliation {
		v.Lines = append(v.Lines, Message{Key: "transactions.accountMoveReconciled", Args: []any{p.FromName}})
	}
	return v
}

func accountOptions(ordered []domain.Account, selected string) []Option {
	visible := make([]domain.Account, 0, len(ordered))
	types := make(map[string]map[domain.AccountType]bool, len(ordered))
	for _, a := range ordered {
		if a.Archived && a.ID != selected {
			continue
		}
		visible = append(visible, a)
		name := strings.ToLower(strings.TrimSpace(a.Name))
		if types[name] == nil {
			types[name] = make(map[domain.AccountType]bool)
		}
		types[name][a.Type] = true
	}
	out := make([]Option, 0, len(visible))
	for _, a := range visible {
		label := a.Name
		if len(types[strings.ToLower(strings.TrimSpace(a.Name))]) > 1 {
			cue := strings.ReplaceAll(string(a.Type), "_", " ")
			if cue == "" {
				cue = "Other"
			} else {
				r := []rune(cue)
				r[0] = unicode.ToUpper(r[0])
				cue = string(r)
			}
			label += " · " + cue
		}
		out = append(out, Option{a.ID, label})
	}
	return out
}

// MovementView describes the classifier's pair, consequence, and debt claim.
type MovementView struct {
	Show, ShowSign bool
	Options        []Option
	Pair, Effect   Message
	DebtAccount    string
}

// MovementPreview uses the same classification validation as Build.
func MovementPreview(t domain.Transaction, selected string, debtPay bool, accounts []domain.Account, txns []domain.Transaction) MovementView {
	_, choices := AccountChoices(accounts, t)
	v := MovementView{Show: len(choices) > 0, ShowSign: selected != ""}
	for _, c := range choices {
		v.Options = append(v.Options, Option{c.AccountID, c.Name})
	}
	if selected != "" {
		p := txnclassify.PreviewApply(t, selected, debtPay && IsLiability(accounts, selected), accounts, txns)
		switch {
		case p.Err != nil:
			v.Pair = Message{Key: "sol1.transferUnavailable"}
		case p.HasPartner && p.Partner.Exact:
			v.Pair = Message{Key: "transactions.classifyPairFound", Args: []any{p.CounterpartyName}}
		case p.HasPartner:
			v.Pair = Message{Key: "transactions.classifyPairNear", Args: []any{p.CounterpartyName, formatMoney(p.Partner.Txn.Amount)}}
		default:
			v.Pair = Message{Key: "transactions.classifyPairNone", Args: []any{p.CounterpartyName}}
		}
	}
	if a, ok := domain.AccountByID(accounts, selected); ok {
		if a.Class == domain.ClassLiability {
			v.Effect = Message{Key: "transactions.classifyEffectDebt", Args: []any{a.Name}}
			v.DebtAccount = a.Name
		} else {
			v.Effect = Message{Key: "transactions.classifyEffectNeutral"}
		}
	}
	return v
}

func formatMoney(m money.Money) string {
	return money.FormatAccounting(m.Amount, currency.Decimals(m.Currency), currency.Symbol(m.Currency))
}

// Heading returns the original descriptor and optional distinct payee subtitle.
func Heading(t domain.Transaction) (string, string) {
	title, payee := strings.TrimSpace(t.Desc), strings.TrimSpace(t.Payee)
	if title == "" {
		title = payee
	}
	if payee != "" && payee != title {
		return title, payee
	}
	return title, ""
}

// OfferRow contains the five-row offer preview's formatted evidence.
type OfferRow struct {
	Name, Date, Amount string
	AlreadyCategorized bool
}

// OfferView contains the recategorization offer and any hidden row count.
type OfferView struct {
	Rows      []OfferRow
	Offer     Message
	Remaining int
}

// SimilarOffer preserves candidate order and shows at most five evidence rows.
func SimilarOffer(candidates []similartxns.Candidate, resolver *payeealias.Resolver) OfferView {
	v := OfferView{Offer: Message{Key: "similartxns.offer", Args: []any{len(candidates)}}}
	if len(candidates) == 1 {
		v.Offer = Message{Key: "similartxns.offerOne"}
	}
	for i, c := range candidates {
		if i >= 5 {
			v.Remaining = len(candidates) - 5
			break
		}
		raw := c.Txn.Payee
		if raw == "" {
			raw = c.Txn.Desc
		}
		v.Rows = append(v.Rows, OfferRow{resolver.Resolve(raw), c.Txn.Date.Format("Jan 2"), formatMoney(c.Txn.Amount), c.AlreadyCategorized})
	}
	return v
}

// MerchantView holds the merchant story, including checked currency conversion.
type MerchantView struct {
	Show           bool
	Merchant, Base string
	Stats          merchantstats.Stats
	ThisMagnitude  int64
	HasThis        bool
}

// MerchantContext gathers matching expense history, ignoring unavailable FX rates.
func MerchantContext(t domain.Transaction, txns []domain.Transaction, resolver *payeealias.Resolver, base string, fx map[string]float64, now time.Time, weekStart time.Weekday) MerchantView {
	if t.IsTransfer() {
		return MerchantView{}
	}
	raw := t.Payee
	if raw == "" {
		raw = t.Desc
	}
	merchant := strings.TrimSpace(resolver.Resolve(raw))
	if merchant == "" {
		return MerchantView{}
	}
	if base == "" {
		base = "USD"
	}
	rates := currency.Rates{Base: base, Rates: fx}
	toBase := func(m money.Money) (int64, bool) {
		amount := m.Amount
		var err error
		if m.Currency != "" && m.Currency != base {
			amount, err = currency.ConvertBetween(amount, m.Currency, base, rates)
		}
		if err != nil {
			return 0, false
		}
		return editMagnitude(amount), true
	}
	var charges []merchantstats.Charge
	for _, row := range txns {
		if row.IsTransfer() || !row.Amount.IsNegative() {
			continue
		}
		name := row.Payee
		if name == "" {
			name = row.Desc
		}
		if strings.ToLower(strings.TrimSpace(resolver.Resolve(name))) != strings.ToLower(merchant) {
			continue
		}
		magnitude, ok := toBase(row.Amount)
		if !ok {
			continue
		}
		charges = append(charges, merchantstats.Charge{Date: row.Date, Minor: magnitude})
	}
	stats := merchantstats.Compute(charges, now, weekStart)
	magnitude, has := toBase(t.Amount)
	return MerchantView{stats.Enough, merchant, base, stats, magnitude, has}
}

// SplitUpdate preserves all live facts except the independently edited breakdown.
func SplitUpdate(live, updated domain.Transaction) (domain.Transaction, string) {
	live.Splits = append([]domain.CategorySplit(nil), updated.Splits...)
	key := "splitEditor.cleared"
	if live.HasSplits() {
		key = "splitEditor.saved"
	}
	return live, key
}

// ReceiptArtifact constructs the artifact linked by an independently durable upload.
func ReceiptArtifact(id, name, mime string, data []byte, now time.Time) domain.Artifact {
	return domain.Artifact{ID: id, Name: name, Kind: "image", MIME: mime, Bytes: data, Size: len(data), CreatedAt: now}
}

// PartnerRemoval reports whether editing away a transfer needs a named confirmation.
func PartnerRemoval(original, updated domain.Transaction, expandedIDs []string) (string, bool) {
	if original.IsTransfer() && !updated.IsTransfer() && len(expandedIDs) > 1 {
		return expandedIDs[1], true
	}
	return "", false
}

// PartnerPrompt supplies the existing transfer-partner deletion detail.
func PartnerPrompt(partner domain.Transaction, accounts []domain.Account) Message {
	account, _ := domain.AccountByID(accounts, partner.AccountID)
	name := partner.Payee
	if name == "" {
		name = partner.Desc
	}
	return Message{Key: "transactions.deleteConfirmDetail", Args: []any{name, dateutil.FormatDate(partner.Date), formatMoney(partner.Amount), account.Name}}
}

// CategoryChanged reports whether a saved classification merits a similar-row offer.
func CategoryChanged(original, updated domain.Transaction) bool {
	return updated.CategoryID != "" && updated.CategoryID != original.CategoryID
}
