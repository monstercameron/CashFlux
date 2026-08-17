// SPDX-License-Identifier: MIT

package i18n

// reviewScopeKeys holds the strings for the two questions the Review surface now
// answers out loud: which charges it is working through (C554) and how many one
// click changes (C653), plus the in-context category creation (C556) and the
// cross-page correction links (C559).
//
// The register throughout is "say the number". Every one of these tickets is the
// same complaint in a different place — a control that acted on a population the
// user could not see and could not count — so a string here that names a scope
// without its size is not doing its job.
//
// Counts arrive already pluralized (the caller runs them through
// screens.plural, e.g. "1 charge" / "251 charges") rather than as a bare %d
// beside a hard-coded plural noun. A queue or a filtered view narrowed to
// exactly one charge is an ordinary state here, not an edge case, and "1 charges
// are waiting" in the surface whose whole point is stating counts accurately
// would undercut every other number on it.
var reviewScopeKeys = Catalog{
	// --- C554: the queue scope -------------------------------------------------
	"review.scopeLabel": "Reviewing",
	"review.scopeView":  "This view (%d)",
	"review.scopeQueue": "Everything (%d)",
	// The sentence under the control. It states what the selected scope covers in
	// the user's own filter terms, and how much sits OUTSIDE it — so choosing is a
	// decision, not a guess. A scope that states only its own size leaves the
	// reader unable to tell a small queue from a narrow filter.
	//
	// The "by" fragments are phrased to sit INSIDE these sentences. The return
	// crumb's version of the same fact follows the words "Back to", so it reads as
	// a destination; dropping that one in here produced "…are in Transactions,
	// filtered to "coffee"".
	"review.scopeBySearch":  "your search for “%s”",
	"review.scopeByFilters": "your %s",
	"review.scopeByView":    "the current view",

	"review.scopeViewNote":    "%s match %s. %s waiting outside it.",
	"review.scopeViewNoteAll": "%s match %s — that is the whole queue.",
	"review.scopeQueueNote":   "All %s awaiting review, including the %s that match %s.",
	"review.scopeAllNote":     "Reviewing all %s awaiting review.",
	// An empty SCOPE is not an empty queue.
	"review.emptyViewTitle": "Nothing to review in this view",
	"review.emptyViewSub":   "Your filters cover no charges that need review. %s still waiting in the whole queue — switch to Everything above.",
	// The "N more are / 1 more is" and "N are / 1 is" fragments, so the verb agrees
	// with the count instead of the sentence quietly assuming a plural.
	"review.waitingOne":  "1 more is",
	"review.waitingMany": "%d more are",
	"review.leftOne":     "1 is",
	"review.leftMany":    "%d are",

	// --- C653: the commit scope ------------------------------------------------
	"review.commitScopeLabel": "This click changes",
	// Not "1 selected charge": nothing has been selected in one-at-a-time review —
	// this is simply the card on screen, and borrowing bulk mode's vocabulary for
	// it would have the user hunting for a selection they never made.
	"review.scopeThisCharge":  "Just this charge",
	"review.scopeAllMerchant": "All %d from %s",
	"review.categorizeAllN":   "Categorize all %d",
	"review.snoozeN":          "Snooze all %d",
	"review.dismissN":         "Not a problem — all %d",
	// The second step. It restates the count, merchant, category, total and dates
	// before anything is written. The merchant is quoted because these are raw bank
	// descriptors — "all 122 AMAZON MKTPLACE PMTS AMZN.COM/BILL WA charges" is a
	// sentence you have to read twice to find the boundary of.
	"review.applyAllTitle": "Categorize %d charges?",
	"review.applyAllAsk":   "Categorize all %d charges from “%s” as %s?",
	"review.applyAllFacts": "%s total · %s",
	"review.applyAllGo":    "Yes, categorize %d",
	"review.dateRange":     "%s – %s",
	// When the world moved between the confirmation and the click. The count on the
	// button came from a snapshot; if something else resolved one of those charges
	// first, the user is owed the real number rather than a queue that quietly drops
	// by a different amount than the one they agreed to.
	"review.wroteFewer": "Categorized %d of %d — the others had already been filed.",

	// --- C556: create a category without losing the card ------------------------
	"review.newCategory":     "New category",
	"review.newCategoryAria": "Create a new category or sub-category for this charge",

	// --- C559: the corrections that leave Review --------------------------------
	// The links say where they go AND that they leave, because they sit a few
	// millimetres from "New category", which looks the same and does not.
	"review.linksLead":         "Need something that isn’t here?",
	"review.gotoCategories":    "Manage categories",
	"review.gotoRules":         "Make this a rule",
	"review.leavesReviewAria":  "%s — leaves Review and comes back to this card",
	"review.leavesReviewGlyph": "↗",
	"review.returnLabelView":   "Review, this view (%d left)",
	"review.returnLabelQueue":  "Review (%d left)",
}

func init() {
	for k, v := range reviewScopeKeys {
		english[k] = v
	}
}
