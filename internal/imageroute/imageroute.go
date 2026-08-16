// SPDX-License-Identifier: MIT

// Package imageroute decides what to DO with a picture somebody dropped into the
// chat (AG12).
//
// CashFlux already has three pipelines that can consume an image: propose a single
// transaction, propose a split, or file it as a document. The missing piece was
// never the extraction — it was the choice. Asking the user "is this a receipt, a
// statement, or a document?" before they have even seen what was read out of it
// puts the classification burden on the person holding the phone, which is exactly
// backwards: the app is the one that just looked at it.
//
// So this makes the choice from what the reading actually contains, and — the part
// that matters — it is honest about how sure it is. A confident route goes straight
// to its pipeline's preview; an unsure one still picks a route but says it is
// guessing, so the wrong guess costs one click rather than a wrong record. Nothing
// here writes anything: every destination lands on a preview the user approves.
//
// Pure: no syscall/js, no appstate. Table-tested on native Go.
package imageroute

import "strings"

// Reading is what the vision model got out of the image, reduced to the facts the
// routing decision turns on.
type Reading struct {
	// Merchant is the payee, when one was read.
	Merchant string
	// TotalMinor is the document's total, when one was read.
	TotalMinor int64
	// LineItems is how many line items a receipt listed.
	LineItems int
	// Rows is how many dated transaction rows were read — the signal that
	// separates a statement from a receipt.
	Rows int
	// Text is the raw text read from the image, used only to spot an explicit
	// split cue. It is never routed on alone.
	Text string
	// Unreadable reports that the model could not make sense of the image at all.
	Unreadable bool
}

// Destination is which pipeline the image belongs in.
type Destination int

const (
	// ToDocument files the image as an attachment. It is the fallback, and it is
	// a real outcome rather than a failure: a photo of a warranty or a contract
	// has nothing to extract and is still worth keeping.
	ToDocument Destination = iota
	// ToTransaction proposes one transaction from a receipt.
	ToTransaction
	// ToSplit proposes a shared expense.
	ToSplit
	// ToStatement opens the multi-row review the CSV importer uses.
	ToStatement
)

// Route is the decision, with the confidence attached.
type Route struct {
	Destination Destination
	// Sure reports whether the reading justifies going straight to the
	// destination's preview. When false the caller still routes there, but says it
	// is a guess — being wrong then costs one click instead of a wrong record.
	Sure bool
	// Why is the plain-English reason, shown to the user. A routing decision the
	// person cannot see the basis for is one they cannot correct.
	Why string
}

// statementRowThreshold is how many dated rows make something a statement rather
// than a detailed receipt. A long supermarket receipt has many LINE ITEMS but one
// date; a statement has many dates.
const statementRowThreshold = 3

// splitCues are the phrases that mean somebody wants this shared. They are matched
// against the image text only as a TIE-BREAKER on an otherwise-receipt reading —
// routing to a split on the word "split" alone would send a receipt from a shop
// called "Split Coffee" down the wrong pipeline.
var splitCues = []string{"split with", "share with", "shared with", "half each", "50/50", "going halves"}

// Decide picks the destination for a reading.
//
// The order of the checks is the priority: an unreadable image can only be filed,
// many dated rows can only be a statement, and everything else is a receipt whose
// only remaining question is whether it is shared.
func Decide(r Reading) Route {
	if r.Unreadable {
		return Route{Destination: ToDocument, Sure: true,
			Why: "I couldn't read anything I recognised, so I'll keep it as a document."}
	}
	if r.Rows >= statementRowThreshold {
		return Route{Destination: ToStatement, Sure: true,
			Why: "This looks like a statement — I found several dated rows, so I'll open the import review."}
	}
	if r.TotalMinor <= 0 && strings.TrimSpace(r.Merchant) == "" {
		// Something was read, but nothing that identifies a purchase. Filing it is
		// the only honest option; proposing a transaction with no merchant and no
		// amount would be a record somebody has to go and fix.
		return Route{Destination: ToDocument, Sure: true,
			Why: "I read some text but no merchant or total, so I'll keep it as a document."}
	}
	if hasSplitCue(r.Text) {
		return Route{Destination: ToSplit, Sure: true,
			Why: "This mentions splitting it, so I'll set up a shared expense."}
	}
	if r.TotalMinor > 0 && strings.TrimSpace(r.Merchant) != "" {
		return Route{Destination: ToTransaction, Sure: true,
			Why: "This looks like a receipt — I'll propose one transaction for you to check."}
	}
	// Half a receipt: enough to be worth proposing, not enough to be sure. The
	// route still goes to the transaction preview, flagged as a guess.
	return Route{Destination: ToTransaction, Sure: false,
		Why: "This might be a receipt, but I only got part of it — check the details before saving."}
}

// hasSplitCue reports whether the text explicitly asks for the cost to be shared.
func hasSplitCue(text string) bool {
	low := strings.ToLower(text)
	for _, cue := range splitCues {
		if strings.Contains(low, cue) {
			return true
		}
	}
	return false
}

// Label names a destination for a button.
func (d Destination) Label() string {
	switch d {
	case ToTransaction:
		return "Add as a transaction"
	case ToSplit:
		return "Split this"
	case ToStatement:
		return "Review the rows"
	default:
		return "Keep as a document"
	}
}

// Alternatives returns the other destinations, so the user can override the guess
// without starting again. Being wrong should cost a click, not a re-upload — the
// picture has already been read, and making somebody take it again to correct the
// app's classification is the least forgivable version of this feature.
func (d Destination) Alternatives() []Destination {
	all := []Destination{ToTransaction, ToSplit, ToStatement, ToDocument}
	out := make([]Destination, 0, len(all)-1)
	for _, x := range all {
		if x != d {
			out = append(out, x)
		}
	}
	return out
}
