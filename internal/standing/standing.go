// SPDX-License-Identifier: MIT

// Package standing holds the durable decisions a household has told the app to
// remember, so it stops asking (WF-SM4): keep at least this much liquid, never
// suggest drawing from that account.
//
// # It constrains ADVICE, not the household
//
// The rule that shapes the whole package: a standing instruction limits what the
// app RECOMMENDS. It never limits what somebody may do with their own money. An
// app that refuses to record a withdrawal because it breaks a rule the user set
// last March has mistaken itself for the bank — and would be wrong at exactly
// the moment it mattered, since the reason to break your own emergency-fund
// floor is usually an emergency.
//
// So there is no Allowed or Blocked here. There is Spendable, which is what the
// app may PROPOSE moving, and Breached, which reports when the household's own
// instruction no longer matches reality — as an observation, never a refusal.
//
// # What this package deliberately does NOT hold
//
// "This merchant is always groceries" belongs to internal/rules, which already
// applies it at import and can be edited in the workbench. "Don't flag this
// annual payment" belongs to internal/flagverdict. "Priya owns this" is a
// MemberID on the record itself. Re-modelling any of them here would create a
// second place to look and a second answer when the two disagree.
package standing

import (
	"encoding/json"
	"strings"
	"time"
)

// Kind is what an instruction constrains.
type Kind string

const (
	// KeepLiquid: never propose spending the household's cash below an amount.
	KeepLiquid Kind = "keep_liquid"
	// NeverDrawFrom: never propose taking money out of this account. Retirement
	// savings, mostly — the money is real, and the app should stop offering it as
	// the answer to every shortfall.
	NeverDrawFrom Kind = "never_draw_from"
)

// Valid reports whether k is a kind this package acts on.
func (k Kind) Valid() bool { return k == KeepLiquid || k == NeverDrawFrom }

// Instruction is one standing decision.
type Instruction struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`
	// Subject is the account the instruction is about, for the kinds that name
	// one. Empty for household-wide instructions like KeepLiquid.
	Subject string `json:"subject,omitempty"`
	// AmountMinor is the floor, for the kinds that carry one.
	AmountMinor int64 `json:"amountMinor,omitempty"`
	// Note is why, in the household's own words. Never required, and never
	// interpreted — it exists so that a rule found a year later can be judged.
	Note string    `json:"note,omitempty"`
	At   time.Time `json:"at"`
}

// Book is the set of standing instructions. The zero value is usable and means
// the app has been told nothing, which is different from having been told to
// keep zero.
type Book struct {
	Instructions []Instruction `json:"instructions,omitempty"`
}

// Load parses a Book. Empty or malformed input yields an empty book: an
// unreadable book means no instructions, and the cost of that is advice the
// household has to correct again — noticeable, and recoverable. The opposite
// failure, inventing a constraint, silently withholds good advice with no
// symptom at all.
func Load(raw string) Book {
	var b Book
	if strings.TrimSpace(raw) == "" {
		return b
	}
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return Book{}
	}
	b.Instructions = valid(b.Instructions)
	return b
}

// Marshal encodes the book for persistence.
func (b Book) Marshal() string {
	out, err := json.Marshal(Book{Instructions: valid(b.Instructions)})
	if err != nil {
		return "{}"
	}
	return string(out)
}

func valid(in []Instruction) []Instruction {
	out := make([]Instruction, 0, len(in))
	for _, i := range in {
		i.ID = strings.TrimSpace(i.ID)
		if i.ID == "" || !i.Kind.Valid() {
			continue
		}
		if i.Kind == NeverDrawFrom && strings.TrimSpace(i.Subject) == "" {
			continue // an instruction about no account constrains nothing
		}
		if i.Kind == KeepLiquid && i.AmountMinor < 0 {
			continue // a negative floor is not a floor
		}
		out = append(out, i)
	}
	return out
}

// Set records an instruction, replacing any earlier one that would contradict
// it: a second "keep at least" replaces the first, because two floors is not a
// state a household can hold, and the app choosing between them silently would
// be picking a number nobody said.
func (b Book) Set(i Instruction) Book {
	if strings.TrimSpace(i.ID) == "" || !i.Kind.Valid() {
		return b
	}
	out := make([]Instruction, 0, len(b.Instructions)+1)
	for _, e := range b.Instructions {
		if e.ID == i.ID {
			continue
		}
		if e.Kind == KeepLiquid && i.Kind == KeepLiquid {
			continue // one floor at a time
		}
		if e.Kind == NeverDrawFrom && i.Kind == NeverDrawFrom && e.Subject == i.Subject {
			continue // already said, for this account
		}
		out = append(out, e)
	}
	return Book{Instructions: append(out, i)}
}

// Forget drops an instruction. Every instruction has to be removable: a rule
// somebody set once and cannot lift becomes a reason to distrust the whole
// feature.
func (b Book) Forget(id string) Book {
	out := make([]Instruction, 0, len(b.Instructions))
	for _, e := range b.Instructions {
		if e.ID != id {
			out = append(out, e)
		}
	}
	return Book{Instructions: out}
}

// KeepLiquidMinor is the household's cash floor and whether one was set. Zero
// with ok=false means nothing was said; zero with ok=true means somebody
// deliberately set no floor, and the two must not be confused.
func (b Book) KeepLiquidMinor() (int64, bool) {
	for _, i := range b.Instructions {
		if i.Kind == KeepLiquid {
			return i.AmountMinor, true
		}
	}
	return 0, false
}

// MayProposeDrawingFrom reports whether the app may SUGGEST taking money out of
// an account. The name is long on purpose: it is not "may withdraw".
func (b Book) MayProposeDrawingFrom(accountID string) bool {
	for _, i := range b.Instructions {
		if i.Kind == NeverDrawFrom && i.Subject == accountID {
			return false
		}
	}
	return true
}

// SpendableMinor is how much of the household's liquid cash the app may propose
// moving or spending: everything above the floor, never negative.
//
// Below the floor it returns zero rather than a negative number — the app has
// nothing to propose, which is a different statement from "you are $3,000 short"
// and is what a caller sizing a suggestion needs. Breached reports the shortfall
// for callers that want to say it out loud.
func (b Book) SpendableMinor(liquidMinor int64) int64 {
	floor, ok := b.KeepLiquidMinor()
	if !ok {
		if liquidMinor < 0 {
			return 0
		}
		return liquidMinor
	}
	if liquidMinor <= floor {
		return 0
	}
	return liquidMinor - floor
}

// Breached reports whether the household is already below its own floor, and by
// how much.
//
// It is an OBSERVATION. Somebody who dipped into their emergency fund knows they
// did; being told is useful, being scolded is not, and being prevented would
// have been wrong at the moment it mattered.
func (b Book) Breached(liquidMinor int64) (shortMinor int64, breached bool) {
	floor, ok := b.KeepLiquidMinor()
	if !ok || liquidMinor >= floor {
		return 0, false
	}
	return floor - liquidMinor, true
}

// Len is how many instructions the app is holding — the number a surface shows
// so that "what does it remember about us" has an answer at a glance.
func (b Book) Len() int { return len(b.Instructions) }
