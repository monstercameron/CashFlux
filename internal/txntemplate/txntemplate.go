// SPDX-License-Identifier: MIT

// Package txntemplate is the pure logic for transaction quick-templates
// ("favourites"): the in-memory Store that holds them, its JSON codec for the
// single KV blob they persist under, validation, and Apply — which turns a
// template into a draft domain.Transaction the quick-add form can consume.
//
// It is platform-independent (no syscall/js) and unit-tested on native Go. The
// wasm state seam (internal/uistate) and UI (internal/screens) are thin shells
// over this package.
package txntemplate

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
)

// Validation errors returned by Validate. They are sentinel values so callers
// (and tests) can match a specific failing rule with errors.Is.
var (
	// ErrNameRequired means the template has no (non-blank) name.
	ErrNameRequired = errors.New("txntemplate: name is required")
	// ErrAmountZero means the template's amount is zero (nothing to pre-fill).
	ErrAmountZero = errors.New("txntemplate: amount must not be zero")
	// ErrAccountRequired means no account is set for the template.
	ErrAccountRequired = errors.New("txntemplate: account is required")
	// ErrCategoryRequired means no category is set for the template.
	ErrCategoryRequired = errors.New("txntemplate: category is required")
)

// Store is the full set of saved transaction templates. It is a plain value
// holding the ordered list; callers mutate a copy and persist the whole thing as
// one JSON blob (mirroring the KV-blob pattern used elsewhere in the app).
type Store struct {
	Items []domain.TxnTemplate `json:"items"`
}

// Upsert inserts t, or replaces an existing template with the same ID. When t.ID
// is empty a new one is assigned (callers may pass their own ID for deterministic
// output — e.g. tests — in which case none is generated).
func (s *Store) Upsert(t domain.TxnTemplate) {
	if strings.TrimSpace(t.ID) == "" {
		t.ID = id.NewWithPrefix("tmpl")
	}
	for i := range s.Items {
		if s.Items[i].ID == t.ID {
			s.Items[i] = t
			return
		}
	}
	s.Items = append(s.Items, t)
}

// Delete removes the template with the given ID, if present. It is a no-op when
// no template matches.
func (s *Store) Delete(id string) {
	if id == "" {
		return
	}
	out := s.Items[:0]
	for _, t := range s.Items {
		if t.ID != id {
			out = append(out, t)
		}
	}
	s.Items = out
}

// Marshal serialises the store to a compact JSON string for KV persistence.
func Marshal(s Store) (string, error) {
	if s.Items == nil {
		s.Items = []domain.TxnTemplate{}
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Unmarshal parses a persisted store blob. It is tolerant: an empty string or
// unparseable/garbage JSON yields an empty Store with a nil error, so a corrupt
// KV value never blocks the feature (it simply reads as "no templates yet").
func Unmarshal(raw string) (Store, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Store{}, nil
	}
	var s Store
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Store{}, nil
	}
	return s, nil
}

// Validate reports the first failing rule for a template: a name is required, the
// amount must be non-zero, and both an account and a category must be present.
func Validate(t domain.TxnTemplate) error {
	if strings.TrimSpace(t.Name) == "" {
		return ErrNameRequired
	}
	if t.AmountMinor == 0 {
		return ErrAmountZero
	}
	if strings.TrimSpace(t.AccountID) == "" {
		return ErrAccountRequired
	}
	if strings.TrimSpace(t.CategoryID) == "" {
		return ErrCategoryRequired
	}
	return nil
}

// Apply builds a draft domain.Transaction from a template, dated now. The draft's
// ID is left blank (the add path assigns a real one on save), the amount is signed
// by the template's direction (negative for an expense, positive for income), and
// the note is copied into Desc because the quick-add form treats Desc as the
// required description. Tags are copied defensively so the draft never aliases the
// template's slice.
func Apply(t domain.TxnTemplate, now time.Time) domain.Transaction {
	var tags []string
	if len(t.Tags) > 0 {
		tags = append([]string(nil), t.Tags...)
	}
	return domain.Transaction{
		AccountID:  t.AccountID,
		Date:       now,
		Payee:      strings.TrimSpace(t.Payee),
		Desc:       strings.TrimSpace(t.Note),
		CategoryID: t.CategoryID,
		Amount:     money.New(t.SignedMinor(), t.Currency),
		Tags:       tags,
		Source:     domain.TxnSourceManual,
	}
}
