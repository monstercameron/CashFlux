// SPDX-License-Identifier: MIT

// Package catscheme provides the starter category scheme for a new household —
// a sensible default set of income and expense categories (with a few
// sub-categories) used for onboarding and the "reset categories" action. Pure
// Go, no platform dependencies, unit-tested on native Go: it returns ID-less
// Items; the caller (appstate/store) assigns IDs and resolves parents by name.
package catscheme

import "github.com/monstercameron/CashFlux/internal/domain"

// Item is a default category definition without an ID. Parent names a top-level
// Item in the same scheme ("" for a top-level category), so the scheme is
// self-contained and the persistence layer can wire ParentIDs after assigning IDs.
type Item struct {
	Name   string
	Kind   domain.CategoryKind
	Color  string
	Parent string
}

// Default returns the starter scheme: common income and expense categories for a
// general (simple-tracking) household. Deterministic and ordered.
func Default() []Item {
	return []Item{
		// Income
		{Name: "Salary", Kind: domain.KindIncome, Color: "#54b884"},
		{Name: "Other income", Kind: domain.KindIncome, Color: "#7c83ff"},

		// Expenses — top level
		{Name: "Housing", Kind: domain.KindExpense, Color: "#d8716f"},
		{Name: "Groceries", Kind: domain.KindExpense, Color: "#cfa14e"},
		{Name: "Dining out", Kind: domain.KindExpense, Color: "#e08e45"},
		{Name: "Transportation", Kind: domain.KindExpense, Color: "#5aa9d6"},
		{Name: "Health", Kind: domain.KindExpense, Color: "#6fcaa3"},
		{Name: "Entertainment", Kind: domain.KindExpense, Color: "#b06fd6"},
		{Name: "Shopping", Kind: domain.KindExpense, Color: "#d66f9e"},
		{Name: "Savings", Kind: domain.KindExpense, Color: "#54b884"},
		{Name: "Debt payments", Kind: domain.KindExpense, Color: "#a35353"},
		{Name: "Other", Kind: domain.KindExpense, Color: "#8a8a90"},

		// Sub-categories (Parent → top-level Name above)
		{Name: "Rent / Mortgage", Kind: domain.KindExpense, Color: "#d8716f", Parent: "Housing"},
		{Name: "Utilities", Kind: domain.KindExpense, Color: "#c98583", Parent: "Housing"},
		{Name: "Fuel", Kind: domain.KindExpense, Color: "#5aa9d6", Parent: "Transportation"},
		{Name: "Public transit", Kind: domain.KindExpense, Color: "#6fb3d6", Parent: "Transportation"},
	}
}
