// SPDX-License-Identifier: MIT

package styles

// registerDtxPolish applies the July 2026 UX-review polish to the two most-used
// surfaces — the Dashboard first viewport and the Transactions workspace. It is
// registered LAST (after every other surface, including registerDashHeroSurface and
// the txn toolbar), so these equal-specificity rules win the cascade over the base
// design system without editing rules_gen.go or the shared surface files.
//
// Two goals:
//   - Dashboard: shrink the net-worth hero so the actionable "Needs attention"
//     digest — already first in the bento order — rises into the first viewport
//     instead of sitting below a tall greeting/figure/sparkline band.
//   - Transactions: widen the description column (the main reading content) and hide
//     the pager when a short ledger already fits on one page.
func registerDtxPolish() {
	// --- Dashboard hero: reclaim vertical space --------------------------------------
	// The hero answers "what's my position?" in one glance; it does not need to own the
	// whole fold. Trim its padding, the headline figure, the sparkline, and the inter-
	// section gaps so the bento's first row (Needs attention) clears the fold.
	rule(".home-hero",
		padding("1.15rem 1.8rem 1.05rem"),
		marginBottom("0.9rem"),
	)
	rule(".home-hero-top",
		marginBottom("0.5rem"),
	)
	// The net-worth figure stays the visual anchor but a touch smaller — 3.1rem was
	// billboard-sized for a number the KPI row also carries.
	rule(".home-hero-nw-fig",
		fontSize("2.5rem"),
	)
	// A living sparkline, not a hero-height chart.
	rule(".home-hero-spark svg",
		height("54px"),
	)
	// Tighten the two stacked sections below the headline.
	rule(".home-hero-stats",
		marginTop("0.7rem"),
		paddingTop("0.6rem"),
	)
	rule(".home-hero-actions",
		marginTop("0.7rem"),
	)

	// --- Transactions: widen the description column ----------------------------------
	// The ledger table is table-layout:fixed, so the unsized Description column absorbs
	// whatever the sized columns leave. The 2026-07-17 audit put reading priority on the
	// description, but the Account/Category columns were still greedy; trim them so the
	// description reads with room instead of clipping mid-merchant. (Scoped to the
	// ledger's .bento-ledger table — no other table is affected.)
	rule(".bento-ledger .txn-table th:nth-child(5), .bento-ledger .txn-table td:nth-child(5)",
		width("150px"),
	)
	rule(".bento-ledger .txn-table th:nth-child(6), .bento-ledger .txn-table td:nth-child(6)",
		width("128px"),
	)

	// --- Transactions: hide the pager when everything fits on one page ---------------
	// txnTableWidget wraps the table in .txn-onepage when the whole set is within the
	// smallest page size, so the pager can never do anything useful — hide it rather
	// than show a dead "1–N of N / Page 1 of 1" control.
	rule(".txn-onepage .std-pager",
		display("none"),
	)
}
