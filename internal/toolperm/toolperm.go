// SPDX-License-Identifier: MIT

// Package toolperm answers one question in a structured, testable way: if I
// approve this, what exactly happens?
//
// The assistant's approval card used to show a single prose sentence written by
// each tool. That sentence says what the tool intends, but it cannot be read at a
// glance for the three things a person actually decides on — what gets read, what
// gets changed and how much of it, and whether the change can be taken back. This
// package holds that as data: a declarative table of what each tool touches, plus
// a scope pulled out of the call's own arguments.
//
// It is pure (no syscall/js, no appstate) so the table is unit-testable, and the
// table is the single place a new tool's permissions get described — a tool with no
// entry is reported as an unknown change rather than silently as a safe one.
package toolperm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Verb is what a tool does to some data. The set is deliberately tiny: these are
// the distinctions a person weighs before approving, not a full CRUD vocabulary.
type Verb string

const (
	// VerbRead means the tool looks at the data and changes nothing.
	VerbRead Verb = "Reads"
	// VerbAdd means new records appear.
	VerbAdd Verb = "Adds"
	// VerbChange means existing records are edited in place.
	VerbChange Verb = "Changes"
	// VerbDelete means records go away.
	VerbDelete Verb = "Deletes"
)

// Access is one thing a tool touches: a verb, the kind of record, how many of them
// when that is knowable, and the slice of data it is limited to.
type Access struct {
	Verb   Verb
	Entity string
	// Count is how many records are affected, or 0 when the number depends on the
	// data rather than on the request. Zero prints as an unquantified phrase
	// ("matching transactions") rather than as a misleading "0".
	Count int
	// Scope narrows the access in the user's own terms ("payees containing
	// \"Trader Joe's\""). Empty when the access is not narrowed.
	Scope string
}

// Line renders one access as a plain-English phrase.
func (a Access) Line() string {
	entity := a.Entity
	if a.Count > 0 {
		entity = plural(a.Count, a.Entity)
	}
	out := string(a.Verb) + " " + entity
	if strings.TrimSpace(a.Scope) != "" {
		out += " " + strings.TrimSpace(a.Scope)
	}
	return out
}

// Permission is the whole structured answer for one tool call.
type Permission struct {
	// Tool is the tool's name, for the card's heading and for the audit trail.
	Tool string
	// Reads is what the tool needs to look at to do its job.
	Reads []Access
	// Writes is what it will change. Empty means the call only reads.
	Writes []Access
	// Reversible reports whether the change can be undone afterwards from the
	// Activity log. False is not "dangerous" — it is "check this one first".
	Reversible bool
	// Known reports whether this tool has an entry in the table. An unknown tool
	// is described conservatively; it is never described as harmless.
	Known bool
}

// ReadOnly reports whether approving this changes nothing.
func (p Permission) ReadOnly() bool { return len(p.Writes) == 0 }

// rule describes one tool's permissions. writes/reads are built from the call's
// arguments, which is where the scope and any knowable count come from.
type rule struct {
	reads      func(args map[string]any) []Access
	writes     func(args map[string]any) []Access
	reversible bool
}

// fixed returns an accessor for a rule whose accesses do not depend on arguments.
func fixed(list ...Access) func(map[string]any) []Access {
	return func(map[string]any) []Access { return list }
}

// read is a shorthand for one read access with no scope.
func read(entity string) Access { return Access{Verb: VerbRead, Entity: entity} }

// rules is the table. Every mutating tool the assistant can call has an entry;
// adding a tool without one makes its approval card say so, which is the intended
// pressure to describe it here.
var rules = map[string]rule{
	"add_task": {
		reads:      fixed(read("your existing to-dos, to avoid a duplicate")),
		writes:     func(a map[string]any) []Access { return []Access{{Verb: VerbAdd, Entity: "to-do", Count: 1}} },
		reversible: true,
	},
	"complete_task": {
		reads: fixed(read("your to-dos")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbChange, Entity: "to-do", Count: 1, Scope: scopePhrase(a, "title", "matching %q")}}
		},
		reversible: true,
	},
	"add_transaction": {
		reads: fixed(read("your accounts and categories, to place it")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbAdd, Entity: "transaction", Count: 1, Scope: scopePhrase(a, "account", "in %q")}}
		},
		reversible: true,
	},
	"create_category": {
		reads: fixed(read("your categories, to avoid a duplicate")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbAdd, Entity: "category", Count: 1, Scope: scopePhrase(a, "name", "named %q")}}
		},
		reversible: true,
	},
	"categorize_transactions": {
		reads: fixed(read("every transaction, to find the matches")),
		writes: func(a map[string]any) []Access {
			return []Access{{
				Verb:   VerbChange,
				Entity: "the category on matching transactions",
				Scope:  scopePhrase(a, "match", "whose payee or description contains %q"),
			}}
		},
		reversible: true,
	},
	"add_goal_contribution": {
		reads: fixed(read("your goals")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbChange, Entity: "goal", Count: 1, Scope: scopePhrase(a, "goal", "matching %q")}}
		},
		reversible: true,
	},
	"add_account": {
		reads:      fixed(read("your accounts, to avoid a duplicate")),
		writes:     func(a map[string]any) []Access { return []Access{{Verb: VerbAdd, Entity: "account", Count: 1}} },
		reversible: true,
	},
	"add_transfer": {
		reads: fixed(read("your accounts and their balances")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbAdd, Entity: "transfer between two accounts", Count: 1}}
		},
		reversible: true,
	},
	"update_account_balance": {
		reads: fixed(read("the account's current balance")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbChange, Entity: "account balance", Count: 1, Scope: scopePhrase(a, "account", "on %q")}}
		},
		reversible: true,
	},
	"merge_duplicate_transactions": {
		reads: fixed(read("the transactions being merged")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbDelete, Entity: "duplicate transaction", Count: countOf(a, "ids") - 1}}
		},
		// A merge removes rows; Activity records it, but the removed rows are gone.
		reversible: false,
	},
	"delete_transaction": {
		reads: fixed(read("the transaction being deleted")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbDelete, Entity: "transaction", Count: 1}}
		},
		reversible: false,
	},
	"dismiss_flagged_activity": {
		reads:      fixed(read("your flagged activity")),
		writes:     func(a map[string]any) []Access { return []Access{{Verb: VerbChange, Entity: "flag", Count: 1}} },
		reversible: true,
	},
	"remember_fact": {
		reads: fixed(read("the facts the assistant already remembers")),
		writes: func(a map[string]any) []Access {
			return []Access{{Verb: VerbAdd, Entity: "remembered fact", Count: 1, Scope: "shown and editable in Settings"}}
		},
		reversible: true,
	},
}

// For returns the structured permission for one tool call. Unrecognised tools get
// a conservative description rather than an empty one: the card should say "this
// changes your data and I can't tell you more" instead of implying safety.
func For(tool string, args json.RawMessage) Permission {
	tool = strings.TrimSpace(tool)
	decoded := decodeArgs(args)
	r, ok := rules[tool]
	if !ok {
		return Permission{
			Tool:   tool,
			Reads:  []Access{read("your financial data")},
			Writes: []Access{{Verb: VerbChange, Entity: "something this app has not described"}},
		}
	}
	p := Permission{Tool: tool, Reversible: r.reversible, Known: true}
	if r.reads != nil {
		p.Reads = r.reads(decoded)
	}
	if r.writes != nil {
		p.Writes = trimAccesses(r.writes(decoded))
	}
	return p
}

// trimAccesses drops accesses whose count resolved to a negative number, which can
// only happen when a call arrived with fewer arguments than it needs — the run
// itself will reject it, and the card should not claim "Deletes -1 transactions".
func trimAccesses(list []Access) []Access {
	out := make([]Access, 0, len(list))
	for _, a := range list {
		if a.Count < 0 {
			a.Count = 0
		}
		out = append(out, a)
	}
	return out
}

// decodeArgs reads a tool call's arguments into a generic map. A call whose
// arguments are missing or malformed yields an empty map, so scope phrases simply
// go unstated rather than the whole card failing.
func decodeArgs(args json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(args) == 0 {
		return out
	}
	if err := json.Unmarshal(args, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// scopePhrase formats one string argument into a scope phrase, or returns "" when
// the argument is absent — an unstated scope is better than an empty quoted one.
func scopePhrase(args map[string]any, key, format string) string {
	v, _ := args[key].(string)
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return fmt.Sprintf(format, v)
}

// countOf returns the length of an array argument, or 0 when it is absent.
func countOf(args map[string]any, key string) int {
	list, _ := args[key].([]any)
	return len(list)
}

// plural renders "1 transaction" / "3 transactions", pluralising the last word of a
// multi-word entity so "account balance" becomes "2 account balances".
func plural(n int, entity string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, entity)
	}
	words := strings.Fields(entity)
	if len(words) == 0 {
		return fmt.Sprintf("%d", n)
	}
	last := words[len(words)-1]
	if !strings.HasSuffix(last, "s") {
		words[len(words)-1] = last + "s"
	}
	return fmt.Sprintf("%d %s", n, strings.Join(words, " "))
}
