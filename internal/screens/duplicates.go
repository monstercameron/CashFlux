// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"fmt"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/auditview"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dedupe"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/prefs"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// dupeGroupProps is the props bag passed to a single duplicate-group card.
// Each group gets its own component so that per-row hooks occupy stable
// positions — never called inside the variable-length outer loop.
type dupeGroupProps struct {
	Group    dedupe.Group
	Txns     map[string]domain.Transaction // keyed by ID, for full field access
	AccByID  map[string]domain.Account
	BaseCur  string
	Prefs    prefs.Prefs // the user's date format, for the confirmation copy (C652)
	OnDelete func(id string)
	OnMerge  func(g dedupe.Group) // C87: merge-group action
}

// dupeGroup renders one card for a set of likely-duplicate transactions.
// The first transaction is treated as the one to keep (labelled "Keep");
// the remaining duplicates each get a "Delete duplicate" button.
// Because this is its own component (called via ui.CreateElement from the outer
// MapKeyed), UseEvent calls inside it are at unconditional, stable positions.
func dupeGroup(props dupeGroupProps) ui.Node {
	g := props.Group

	// C87: merge event — UseEvent at a stable, unconditional position in this component.
	//
	// C571: the confirmation named a count and nothing else — "the first entry is
	// kept" is true but useless when every entry in the group looks alike on screen.
	// It now names the survivor the way the ledger does (description, date) and
	// states the arithmetic, so the user can verify which transaction lives through
	// the operation before committing to it.
	merge := ui.UseEvent(func() {
		others := len(g.IDs) - 1
		survivor := ""
		if len(g.IDs) > 0 {
			survivor = dupEntryLabel(props.Txns[g.IDs[0]], g.Description)
		}
		removed := uistate.T("duplicates.mergeRemovedMany", others)
		if others == 1 {
			removed = uistate.T("duplicates.mergeRemovedOne")
		}
		msg := uistate.T("duplicates.mergeConfirmNamed", len(g.IDs), survivor, g.Date, removed)
		uistate.ConfirmModalLabeled(msg, uistate.T("duplicates.mergeConfirmBtn"), true, func(ok bool) {
			if ok && props.OnMerge != nil {
				props.OnMerge(g)
			}
		})
	})

	// Format the shared amount for the group header.
	dec := currency.Decimals(g.Currency)
	sym := currency.Symbol(g.Currency)
	absAmt := g.Amount
	if absAmt < 0 {
		absAmt = -absAmt
	}
	amtStr := sym + fmtMinorAmount(absAmt, dec)
	sign := "+"
	if g.Amount < 0 {
		sign = "-"
	}
	amtDisplay := sign + amtStr
	amtClass := "text-up"
	if g.Amount < 0 {
		amtClass = "text-down"
	}

	// Group header: payee · date · amount.
	header := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap4),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)), g.Description),
			Span(css.Class("t-caption", tw.TextDim), g.Date),
		),
		Span(ClassStr("t-body "+tw.Fold(tw.FontMedium)+" "+tw.ColorClass(amtClass)), amtDisplay),
	)

	// Badge: number of entries in this group.
	groupCount := fmt.Sprintf(uistate.T("duplicates.groupCount"), len(g.IDs))
	if len(g.IDs) == 1 {
		groupCount = uistate.T("duplicates.groupCountOne")
	}
	badge := Span(css.Class("badge"), groupCount)

	// C688: name the account. "Same day, same amount, same description" is only a
	// duplicate WITHIN one account — the grouping enforces that now, and saying so
	// is what lets a reader confirm it rather than take it on trust before pressing
	// a button that deletes rows.
	acctLabel := Fragment()
	if a, ok := props.AccByID[g.AccountID]; ok && a.Name != "" {
		acctLabel = Span(css.Class("t-caption", tw.TextDim), Attr("data-testid", "dupe-group-account"),
			uistate.T("duplicates.onAccount", a.Name))
	}

	titleRow := Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap2, tw.Mb3),
		badge,
		acctLabel,
		Div(css.Class(tw.Flex1)),
		Span(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.keepNote")),
		Button(
			css.Class("btn btn-primary btn-sm"),
			Attr("type", "button"),
			Attr("data-testid", "dup-merge-btn"),
			Attr("aria-label", uistate.T("duplicates.mergeAria")),
			OnClick(merge),
			uistate.T("duplicates.mergeBtn"),
		),
	)

	// TXC-4: preview what the non-lossy merge will carry over from the removed rows
	// onto the kept one, so a receipt/note appearing after a merge is never a surprise.
	var mergePreview ui.Node = Fragment()
	if len(g.IDs) >= 2 {
		if items := dupMergeCarryItems(props.Txns, g.IDs); len(items) > 0 {
			mergePreview = P(css.Class("t-caption", tw.TextDim, tw.Mb2), Attr("data-testid", "dup-merge-preview"),
				uistate.T("duplicates.mergeCarry", strings.Join(items, " · ")))
		}
	}

	// Per-transaction rows — each is its own component (dupeRow) so that
	// UseEvent hooks are not inside a variable-length loop body.
	rows := MapKeyed(
		g.IDs,
		func(id string) any { return id },
		func(id string) ui.Node {
			t, _ := props.Txns[id]
			accName := ""
			if a, ok := props.AccByID[t.AccountID]; ok {
				accName = a.Name
			}
			isFirst := len(g.IDs) > 0 && id == g.IDs[0]
			kept := ""
			if !isFirst && len(g.IDs) > 0 {
				kept = dupEntryIdentity(props.Txns[g.IDs[0]], props.AccByID, g, props.Prefs)
			}
			return ui.CreateElement(dupeRow, dupeRowProps{
				TxnID:    id,
				Date:     g.Date,
				AccName:  accName,
				Label:    dupEntryLabel(t, g.Description),
				Removed:  dupEntryIdentity(t, props.AccByID, g, props.Prefs),
				Kept:     kept,
				IsFirst:  isFirst,
				OnDelete: props.OnDelete,
			})
		},
	)

	return uiw.Card(uiw.CardProps{
		Body: Div(
			titleRow,
			mergePreview,
			header,
			Div(css.Class(tw.Mt3, tw.Flex, tw.FlexCol, tw.Gap2), rows),
		),
	})
}

// dupMergeCarryItems lists, in plain English, what a non-lossy merge (dedupe.Merge)
// will carry onto the kept (first) entry from the removed duplicates: receipts it
// doesn't already have, and any empty identity/link field a removed row can fill.
// Empty result = the entries carry the same details, so nothing is surprising.
func dupMergeCarryItems(txns map[string]domain.Transaction, ids []string) []string {
	if len(ids) < 2 {
		return nil
	}
	survivor := txns[ids[0]]
	others := make([]domain.Transaction, 0, len(ids)-1)
	for _, id := range ids[1:] {
		if t, ok := txns[id]; ok {
			others = append(others, t)
		}
	}
	var items []string
	have := make(map[string]bool, len(survivor.Attachments))
	for _, a := range survivor.Attachments {
		have[a.ArtifactID] = true
	}
	newAtt := 0
	for _, o := range others {
		for _, a := range o.Attachments {
			if !have[a.ArtifactID] {
				have[a.ArtifactID] = true
				newAtt++
			}
		}
	}
	if newAtt > 0 {
		items = append(items, uistate.T("duplicates.carryReceipts", plural(newAtt, "receipt")))
	}
	fill := func(surv string, pick func(domain.Transaction) string, label string) {
		if surv != "" {
			return
		}
		for _, o := range others {
			if pick(o) != "" {
				items = append(items, label)
				return
			}
		}
	}
	fill(survivor.CategoryID, func(t domain.Transaction) string { return t.CategoryID }, uistate.T("duplicates.carryCategory"))
	fill(survivor.Note, func(t domain.Transaction) string { return t.Note }, uistate.T("duplicates.carryNote"))
	fill(survivor.Payee, func(t domain.Transaction) string { return t.Payee }, uistate.T("duplicates.carryPayee"))
	fill(survivor.BillAccountID, func(t domain.Transaction) string { return t.BillAccountID }, uistate.T("duplicates.carryLink"))
	fill(survivor.SubscriptionName, func(t domain.Transaction) string { return t.SubscriptionName }, uistate.T("duplicates.carryLink"))
	// C652: tags and cleared status are UNIONED by dedupe.Merge — the kept entry
	// comes out carrying tags it did not have, and cleared if any copy was. Both
	// change how the row reads and how it counts in reconciliation, so they belong
	// in the preview rather than being discovered afterwards.
	// Literal keys on both branches, not a key held in a variable: the i18n
	// coverage scan only sees literal keys passed to T, and a dynamic key is a
	// missing string nobody finds until it renders as its own name.
	if tags := dupMergeNewTags(survivor, others); len(tags) == 1 {
		items = append(items, uistate.T("duplicates.carryTags", tags[0]))
	} else if len(tags) > 1 {
		items = append(items, uistate.T("duplicates.carryTagsMany", strings.Join(tags, ", ")))
	}
	if !survivor.Cleared {
		for _, o := range others {
			if o.Cleared {
				items = append(items, uistate.T("duplicates.carryCleared"))
				break
			}
		}
	}
	return items
}

// dupMergeNewTags is the tags a merge will add to the kept entry — the ones only
// a removed copy carries, compared case-insensitively the way dedupe.Merge does.
func dupMergeNewTags(survivor domain.Transaction, others []domain.Transaction) []string {
	seen := make(map[string]bool, len(survivor.Tags))
	for _, tg := range survivor.Tags {
		seen[strings.ToLower(tg)] = true
	}
	var out []string
	for _, o := range others {
		for _, tg := range o.Tags {
			k := strings.ToLower(tg)
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, "#"+tg)
		}
	}
	return out
}

// dupEntryLabel names one entry of a duplicate group for a confirmation sentence:
// its own description, falling back to the payee, then to the group's shared
// description. A confirmation that says "this duplicate" tells the user nothing
// they can check; one that says "Costco #1128" does (C571).
func dupEntryLabel(t domain.Transaction, groupDesc string) string {
	if d := strings.TrimSpace(t.Desc); d != "" {
		return d
	}
	if p := strings.TrimSpace(t.Payee); p != "" {
		return p
	}
	return groupDesc
}

// dupEntryIdentity names one entry of a duplicate group the way the ledger row
// does: description, date, amount, account. It is the smallest description that
// still distinguishes two near-identical copies, which is exactly what a
// duplicate confirmation has to do (C652).
func dupEntryIdentity(t domain.Transaction, accByID map[string]domain.Account, g dedupe.Group, pr prefs.Prefs) string {
	acct := uistate.T("duplicates.noAccount")
	if a, ok := accByID[t.AccountID]; ok && strings.TrimSpace(a.Name) != "" {
		acct = a.Name
	}
	// The user's own date format. `dedupe.Group.Date` is a raw ISO string, and
	// printing it here would put "2026-08-14" in the one dialog whose job is to be
	// legible, while every other date on screen follows the preference.
	return uistate.T("duplicates.entryIdentity",
		dupEntryLabel(t, g.Description), pr.FormatDate(t.Date), fmtMoney(t.Amount), acct)
}

// dupeRowProps is the props bag for a single transaction entry within a group.
type dupeRowProps struct {
	TxnID   string
	Date    string
	AccName string
	Label   string // this entry's own name, for the delete confirmation (C571)
	IsFirst bool   // first = "keep" row; others = deletable duplicates
	// C652: the two identities the confirmation has to hold side by side. The
	// sentence "the entry kept at the top of the group is untouched" was true and
	// unusable — in a group of near-identical rows the only thing that separates
	// them is the account and the amount, so the confirmation has to print both,
	// for the copy being removed AND for the one being kept.
	Removed, Kept string
	OnDelete      func(id string)
}

// dupeRow is a single entry row inside a duplicate group card. It is its own
// component so that UseEvent is called at a stable, unconditional position
// (never inside an outer variable-length loop). The first entry is marked
// "Keep"; all others get a "Delete duplicate" button.
func dupeRow(props dupeRowProps) ui.Node {
	// C571: the old confirmation read "Delete this duplicate transaction? This can't
	// be undone." — both halves wrong. It named nothing the user could check against
	// the three near-identical rows in front of them, and the claim about undo
	// contradicted the code, which captures an audit point and posts an undoable
	// toast. A confirmation that overstates the risk teaches people to click through
	// confirmations. It now names the entry, says the kept one is untouched, and
	// states the reversal path the app actually provides.
	//
	// C652: it now prints the two entries as a pair. Naming only the one being
	// removed still leaves the reader unable to check WHICH copy survives, which
	// is the whole question in a group of three rows that differ by one account.
	del := ui.UseEvent(func() {
		msg := uistate.T("duplicates.deleteConfirmPair", props.Removed, props.Kept)
		uistate.ConfirmModalLabeled(msg, uistate.T("duplicates.deleteConfirmBtn"), true, func(ok bool) {
			if ok {
				props.OnDelete(props.TxnID)
			}
		})
	})

	rowClass := tw.Fold(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap3, tw.Py2)

	if props.IsFirst {
		return Div(css.Class(rowClass, tw.BorderB),
			Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
				Span(css.Class("t-caption", tw.TextDim), props.AccName),
				Span(css.Class("t-caption", tw.TextFaint), uistate.T("duplicates.keepLabel")),
			),
			Span(css.Class("badge"), uistate.T("duplicates.keepBadge")),
		)
	}

	return Div(css.Class(rowClass),
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap1),
			Span(css.Class("t-caption", tw.TextDim), props.AccName),
			Span(css.Class("t-caption", tw.TextFaint), props.Date),
		),
		Button(
			css.Class("btn btn-danger btn-sm"),
			Attr("type", "button"),
			Attr("data-testid", "dup-delete-btn"),
			Attr("aria-label", fmt.Sprintf(uistate.T("duplicates.deleteAria"), props.TxnID)),
			OnClick(del),
			uistate.T("duplicates.deleteBtn"),
		),
	)
}

// duplicatesPanelProps is the props bag for DuplicatesPanel. Currently empty
// — the panel reads all its data from appstate.Default — but typed so it can be
// embedded via ui.CreateElement and have its hook state isolated from parents.
type duplicatesPanelProps struct{}

// DuplicatesPanel is the registered component that groups duplicate transactions
// and lets the user delete or merge them. Extracted from DuplicatesScreen() so it
// can be embedded on /transactions without duplicating logic (FEATURE_MAP §5.3 /
// §5.7b). Per-row hook state (UseEvent) lives inside dupeRow; per-group layout
// lives inside dupeGroup. DuplicatesPanel itself holds no per-item hooks — only
// UseDataRevision() to react to data changes.
func DuplicatesPanel(props duplicatesPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	_ = uistate.UseDataRevision().Get()
	// C652: the confirmation prints dates, and they follow the user's format like
	// every other date on screen — not the raw ISO string the dedupe group carries.
	pr := uistate.UsePrefs().Get()

	txns := app.Transactions()
	accounts := app.Accounts()

	accByID := make(map[string]domain.Account, len(accounts))
	for _, a := range accounts {
		accByID[a.ID] = a
	}
	txnByID := make(map[string]domain.Transaction, len(txns))
	for _, t := range txns {
		txnByID[t.ID] = t
	}

	groups := dedupe.FindDuplicates(txns)
	total := dedupe.Count(groups)

	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}

	// Plain func passed down as a prop — no hook here.
	//
	// C652: the undo point is captured BEFORE the delete, not after. postUndoStory
	// captured it afterwards, which seals the state that already has the row
	// missing — so the toast offered an Undo whose nearest restore point was
	// whatever the 4-second autosave ticker happened to have taken. The
	// confirmation promises recovery; this is what makes the promise true. Same
	// rule the ledger's own row delete follows ("capture BEFORE the write").
	deleteTxn := func(id string) {
		auditview.CaptureNow()
		if err := app.DeleteTransaction(id); err != nil {
			uistate.PostNotice(err.Error(), false)
			return
		}
		// C364: name the reversal path (Ctrl+Z / Activity) on duplicate resolution.
		uistate.PostUndoable(uistate.T("toast.undoStory", uistate.T("duplicates.deleted")))
		uistate.BumpDataRevision() // re-render the panel so the resolved group drops off
	}

	// C87: merge a duplicate group — keep the first entry (union tags/cleared),
	// delete the rest. No hook here; plain func passed as a prop.
	mergeTxns := func(g dedupe.Group) {
		if len(g.IDs) < 2 {
			return
		}
		survivorID := g.IDs[0]
		survivor, ok := txnByID[survivorID]
		if !ok {
			return
		}
		others := make([]domain.Transaction, 0, len(g.IDs)-1)
		for _, id := range g.IDs[1:] {
			if t, ok := txnByID[id]; ok {
				others = append(others, t)
			}
		}
		// C688: merging keeps one row and DELETES the rest, so the last thing before
		// the writes is a check that they are all on one account. The grouping rule
		// already guarantees it; this catches a group assembled by any other route,
		// where the failure is deleting a real payment from another account that
		// nothing downstream would notice, because the survivor looks just like it.
		if !dedupe.SameAccount(survivor, others) {
			uistate.PostNotice(uistate.T("duplicates.crossAccountRefused"), true)
			return
		}
		// C652: same rule as the delete — the restore point goes in before the
		// writes, not after them.
		auditview.CaptureNow()
		merged := dedupe.Merge(survivor, others)
		if err := app.PutTransaction(merged); err != nil {
			uistate.PostNotice(err.Error(), false)
			return
		}
		for _, t := range others {
			if err := app.DeleteTransaction(t.ID); err != nil {
				uistate.PostNotice(err.Error(), false)
				return
			}
		}
		// C364: a merge collapses several rows into one — tell the undo story.
		// C571: name the survivor in the result too, so the toast confirms the same
		// fact the confirmation promised rather than a generic "merged".
		uistate.PostUndoable(uistate.T("toast.undoStory",
			uistate.T("duplicates.merged")+" "+
				uistate.T("duplicates.survivorLabel", dupEntryLabel(survivor, g.Description))))
		uistate.BumpDataRevision() // re-render the panel so the merged group drops off
	}

	_ = base // reserved for future per-group currency display

	// Empty state.
	if len(groups) == 0 {
		return uiw.Card(uiw.CardProps{
			Body: Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap2),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), uistate.T("duplicates.emptyTitle")),
				P(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.emptyBody")),
			),
		})
	}

	// Summary banner.
	summary := uiw.Card(uiw.CardProps{
		Body: Div(css.Class(tw.Flex, tw.ItemsCenter, tw.Gap3),
			Div(css.Class(tw.Flex1, tw.Flex, tw.FlexCol, tw.Gap1),
				P(ClassStr("t-body "+tw.Fold(tw.FontMedium)), dupHeadline(total, len(groups))),
				P(css.Class("t-caption", tw.TextDim), uistate.T("duplicates.hint")),
			),
		),
	})

	// One card per duplicate group.
	groupCards := MapKeyed(
		groups,
		func(g dedupe.Group) any {
			if len(g.IDs) > 0 {
				return g.IDs[0]
			}
			return g.Date + "|" + g.Description
		},
		func(g dedupe.Group) ui.Node {
			return ui.CreateElement(dupeGroup, dupeGroupProps{
				Group:    g,
				Txns:     txnByID,
				AccByID:  accByID,
				BaseCur:  base,
				Prefs:    pr,
				OnDelete: deleteTxn,
				OnMerge:  mergeTxns,
			})
		},
	)

	return Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap5),
		summary,
		Div(css.Class(tw.Flex, tw.FlexCol, tw.Gap4), groupCards),
	)
}

// DuplicatesScreen is the /duplicates route — a thin shell that delegates
// entirely to DuplicatesPanel. Routes remain registered (pending rail regroup);
// logic lives in DuplicatesPanel so it can also be embedded on /transactions.
// dupHeadline renders the duplicates summary with correct grammar for the common
// single-duplicate case (the plural read "1 possible duplicate entries across 1
// groups"). A single duplicate is always in a single group.
func dupHeadline(total, groups int) string {
	if total == 1 {
		return uistate.T("duplicates.headlineOne")
	}
	return fmt.Sprintf(uistate.T("duplicates.headline"), total, groups)
}

func DuplicatesScreen() ui.Node {
	return ui.CreateElement(DuplicatesPanel, duplicatesPanelProps{})
}

// DuplicatesPanelBody is the exported handle for mounting the duplicates review inside
// the shell-root "Review duplicates" flip modal (app.DuplicatesHost). DuplicatesPanel's
// props type is unexported, so the app package embeds this wrapper instead. The empty
// struct keeps it CreateElement-compatible, matching ImportPanelBody.
func DuplicatesPanelBody(_ struct{}) ui.Node {
	return ui.CreateElement(DuplicatesPanel, duplicatesPanelProps{})
}
