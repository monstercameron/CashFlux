// SPDX-License-Identifier: MIT

// Package splitview prepares and validates the shared-expense calculator route.
// It keeps input parsing and split facts native so the screen only adapts state.
package splitview

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/split"
)

var (
	// ErrInvalidAmount indicates a malformed, nonpositive expense amount.
	ErrInvalidAmount = errors.New("splitview: invalid amount")
	// ErrInvalidWeight indicates a malformed or negative member weight.
	ErrInvalidWeight = errors.New("splitview: invalid weight")
	// ErrUnknownMember indicates that an input references an unavailable member.
	ErrUnknownMember = errors.New("splitview: unknown member")
	// ErrDuplicateMember indicates that a selected member appears more than once.
	ErrDuplicateMember = errors.New("splitview: duplicate member")
	// ErrNoPositiveWeight indicates that weighted mode has no positive weight basis.
	ErrNoPositiveWeight = errors.New("splitview: no positive weights")
	// ErrWeightOverflow indicates that weights cannot be safely combined.
	ErrWeightOverflow = errors.New("splitview: weight overflow")
	// ErrShareOverflow indicates that a computed share cannot be represented.
	ErrShareOverflow = errors.New("splitview: share overflow")
)

// Member is the identity information required to prepare display facts.
type Member struct {
	ID   string
	Name string
}

// Input contains the current calculator fields. Empty AmountText is a neutral
// empty form; empty weight strings retain the route's default weight of one.
type Input struct {
	AmountText string
	Decimals   int
	Members    []Member
	Selected   []string
	Weights    map[string]string
	Weighted   bool
	PayerID    string
}

// Model contains validated share and settle-up facts in selected-member order.
type Model struct {
	Amount    int64
	HasAmount bool
	Shares    []split.Share
	ShareByID map[string]int64
	NameByID  map[string]string
	Transfers []split.Transfer
}

// Prepare parses calculator fields and derives shares and payer-facing transfers.
// Invalid nonblank input returns an empty model with a typed error.
func Prepare(in Input) (Model, error) {
	model := Model{ShareByID: map[string]int64{}, NameByID: map[string]string{}}
	for _, member := range in.Members {
		if strings.TrimSpace(member.ID) == "" {
			continue
		}
		model.NameByID[member.ID] = member.Name
	}
	if strings.TrimSpace(in.AmountText) == "" {
		return model, nil
	}
	amount, err := money.ParseMinor(in.AmountText, in.Decimals)
	if err != nil || amount <= 0 {
		if err != nil {
			return Model{}, fmt.Errorf("%w: %w", ErrInvalidAmount, err)
		}
		return Model{}, ErrInvalidAmount
	}
	model.Amount, model.HasAmount = amount, true
	if in.PayerID != "" {
		if _, ok := model.NameByID[in.PayerID]; !ok {
			return Model{}, fmt.Errorf("%w: payer %q", ErrUnknownMember, in.PayerID)
		}
	}

	seen := make(map[string]struct{}, len(in.Selected))
	ids := make([]string, 0, len(in.Selected))
	for _, id := range in.Selected {
		if _, ok := model.NameByID[id]; !ok {
			return Model{}, fmt.Errorf("%w: selected member %q", ErrUnknownMember, id)
		}
		if _, ok := seen[id]; ok {
			return Model{}, fmt.Errorf("%w: selected member %q", ErrDuplicateMember, id)
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return model, nil
	}
	if !in.Weighted {
		model.Shares = split.Equal(amount, ids)
	} else {
		weights := make([]int64, len(ids))
		var total int64
		for i, id := range ids {
			text := strings.TrimSpace(in.Weights[id])
			weight := int64(1)
			if text != "" {
				parsed, parseErr := strconv.ParseInt(text, 10, 64)
				if parseErr != nil || parsed < 0 {
					return Model{}, fmt.Errorf("%w: member %q", ErrInvalidWeight, id)
				}
				weight = parsed
			}
			if weight > math.MaxInt64-total {
				return Model{}, fmt.Errorf("%w: member %q", ErrWeightOverflow, id)
			}
			total += weight
			weights[i] = weight
		}
		if total == 0 {
			return Model{}, ErrNoPositiveWeight
		}
		amounts, allocErr := money.Allocate(amount, weights)
		if allocErr != nil {
			return Model{}, fmt.Errorf("%w: %w", ErrShareOverflow, allocErr)
		}
		model.Shares = make([]split.Share, len(ids))
		for i, id := range ids {
			model.Shares[i] = split.Share{MemberID: id, Amount: amounts[i]}
		}
	}
	for _, share := range model.Shares {
		model.ShareByID[share.MemberID] = share.Amount
	}
	if in.PayerID != "" {
		for _, share := range model.Shares {
			if share.MemberID != in.PayerID && share.Amount > 0 {
				model.Transfers = append(model.Transfers, split.Transfer{From: share.MemberID, To: in.PayerID, Amount: share.Amount})
			}
		}
	}
	return model, nil
}
