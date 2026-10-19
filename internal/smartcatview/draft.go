// SPDX-License-Identifier: MIT

package smartcatview

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/monstercameron/CashFlux/internal/duedate"
	"github.com/monstercameron/CashFlux/internal/rapidcapture"
	"github.com/monstercameron/CashFlux/internal/smart"
	"github.com/monstercameron/CashFlux/internal/smartai"
)

// DraftFreeCode identifies the deterministic sentence parser.
const DraftFreeCode = "SMART-T22F"

// DraftAICode identifies the optional AI sentence parser.
const DraftAICode = "SMART-T22"

// DraftFill contains optional fields to merge into a transaction form.
type DraftFill struct {
	// AmountMajor is the parser's major-unit decimal string.
	AmountMajor string
	// Income is true when the sentence described money coming IN.
	Income bool
	Payee  string
	// Date is zero when the sentence named none.
	Date time.Time
	// CategoryID is set only by the Smart+ path, and only when the model named a
	// category the household actually has.
	CategoryID string
}

// Empty reports whether the parse found nothing worth applying.
func (f DraftFill) Empty() bool {
	return f.AmountMajor == "" && f.Payee == "" && f.Date.IsZero() && f.CategoryID == ""
}

// ParseSentence parses a capture sentence relative to an explicit local date.
func ParseSentence(raw string, now time.Time) DraftFill {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DraftFill{}
	}
	out := DraftFill{}
	// The date first, so its words are lifted out before the merchant label is read
	// and "yesterday" cannot end up as part of the payee.
	d := duedate.Parse(raw, now)
	if !d.Due.IsZero() {
		out.Date = d.Due
	}
	rest := d.Title
	if rest == "" {
		rest = raw
	}
	drafts := rapidcapture.Parse(rest)
	if len(drafts) == 0 {
		return out
	}
	first := drafts[0]
	out.AmountMajor = first.MajorString
	out.Payee = titleDraftLabel(cleanDraftLabel(first.Label))
	out.Income = txnSentenceIsIncome(rest)
	return out
}

// incomePhrases are the only leading phrases that flip a sentence to money IN.
var incomePhrases = []string{
	"got paid", "got a refund", "received a refund", "received", "refund",
	"paid me", "earned", "income", "deposit",
}

// txnSentenceIsIncome reports whether the sentence describes money coming in.
func txnSentenceIsIncome(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	for _, phrase := range incomePhrases {
		if strings.HasPrefix(low, phrase) {
			rest := strings.TrimPrefix(low, phrase)
			if rest == "" {
				return true
			}
			next, _ := utf8.DecodeRuneInString(rest)
			if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' {
				return true
			}
		}
	}
	return false
}

// leadWords are the verbs and prepositions that glue a capture sentence together
// and are never part of a merchant's name.
var leadWords = map[string]bool{
	"spent": true, "paid": true, "bought": true, "at": true, "on": true, "for": true,
	"from": true, "to": true, "in": true, "a": true, "an": true, "the": true, "got": true,
}

// cleanDraftLabel strips connective words from the ends of the merchant label,
// leaving "whole foods" rather than "spent at whole foods". Interior words remain.
func cleanDraftLabel(s string) string {
	words := strings.Fields(s)
	for len(words) > 0 && leadWords[strings.ToLower(words[0])] {
		words = words[1:]
	}
	for len(words) > 0 && leadWords[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// titleDraftLabel gives parsed merchant names readable display casing without
// changing the original sentence or any transaction description.
func titleDraftLabel(s string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(s)))
	for i, word := range words {
		first, size := utf8.DecodeRuneInString(word)
		if first != utf8.RuneError || size != 0 {
			words[i] = string(unicode.ToTitle(first)) + word[size:]
		}
	}
	return strings.Join(words, " ")
}

// DraftSummary formats a parse using the caller's date and translation preferences.
func DraftSummary(f DraftFill, fmtDate func(time.Time) string, translate func(string, ...any) string) string {
	parts := []string{}
	if f.Payee != "" {
		parts = append(parts, f.Payee)
	}
	if f.AmountMajor != "" {
		if f.Income {
			parts = append(parts, translate("sm15.amountIn", f.AmountMajor))
		} else {
			parts = append(parts, translate("sm15.amountOut", f.AmountMajor))
		}
	}
	if !f.Date.IsZero() && fmtDate != nil {
		parts = append(parts, fmtDate(f.Date))
	}
	return strings.Join(parts, " · ")
}

// ParseAIDraft resolves an AI response against the real household catalog.
func ParseAIDraft(text string, catalog smartai.Catalog, now time.Time) (DraftFill, bool) {
	d, ok := smartai.ParseTxnDraft(strings.TrimSpace(text), catalog, now)
	if !ok {
		return DraftFill{}, false
	}
	return DraftFill{AmountMajor: d.AmountMajor, Income: !d.Negative, Payee: d.Payee, Date: d.Date, CategoryID: d.CategoryID}, true
}

// DraftView describes visibility and labels without rendering UI.
type DraftView struct {
	Raw                      string
	Offer                    DraftFill
	FromAI, Visible, ShowAsk bool
	DetailKey, Code, AskKey  string
}

// PresentDraft chooses the AI offer when available and applies feature preferences.
func PresentDraft(sentence string, local, ai DraftFill, provider, loading bool, settings smart.Settings) DraftView {
	raw := strings.TrimSpace(sentence)
	aiEnabled := provider && settings.IsEnabled(DraftAICode) && !settings.IsMuted(DraftAICode)
	freeEnabled := settings.IsEnabled(DraftFreeCode) && !settings.IsMuted(DraftFreeCode)
	v := DraftView{Raw: raw, Offer: local, Code: DraftFreeCode, DetailKey: "sm15.readLocal", AskKey: "sm15.aiAction"}
	if !ai.Empty() {
		v.Offer = ai
		v.FromAI = true
		v.Code = DraftAICode
		v.DetailKey = "catSuggest.aiSource"
	}
	v.Visible = (freeEnabled || aiEnabled) && settings.DensityOrDefault().Shows(smart.AffordanceFieldAssist)
	v.ShowAsk = aiEnabled && raw != "" && !v.FromAI
	if loading {
		v.AskKey = "sm15.aiPending"
	}
	return v
}
