// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/importsafe"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"

	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// csvPreflightInfo is the computed pre-commit preview for a staged CSV import
// (#57): counts, balance impact on the import account, the implausible-jump
// flag, per-row duplicate reasons, and likely transfer pairs. Built by
// buildCSVPreflight in documents.go; nothing writes until the user confirms.
type csvPreflightInfo struct {
	Staged       appstate.StagedImport
	AccountID    string
	Total, Dupes int
	AcctName     string
	Dec          int
	HasBal       bool
	BeforeMinor  int64
	NetMinor     int64
	AfterMinor   int64
	Jump         bool
	Whys         []importsafe.WhyDup
	Pairs        []importsafe.Pair
	// Batch is the same file staged through internal/staging — every row
	// normalized, hashed and given a verdict, from the app's one duplicate rule
	// (C689). It is what the counts below are read from, so the preview cannot
	// reach a different verdict from the import that follows it.
	Batch    staging.Batch
	Failures []store.CSVRowError
}

// csvPreflightCard renders the staged import's pre-commit preview with the
// only two ways forward: Import now, or Cancel (which drops the staged bytes).
func csvPreflightCard(info *csvPreflightInfo, confirm, cancel ui.Handler) ui.Node {
	if info == nil {
		return Fragment()
	}
	newRows := info.Batch.Counts.New
	signedNet := importdraft.SignedMinor(info.NetMinor, info.Dec)

	// Duplicate why-matched rows (capped at 6 — enough to recognize the file).
	var whyRows []ui.Node
	for i, w := range info.Whys {
		if i == 6 {
			whyRows = append(whyRows, P(css.Class("muted", tw.Text12), uistate.T("documents.preflightMoreDups", len(info.Whys)-6)))
			break
		}
		reason := uistate.T("documents.preflightDupLedger")
		if w.InBatch {
			reason = uistate.T("documents.preflightDupBatch")
		}
		whyRows = append(whyRows, P(css.Class("muted", tw.Text12), Attr("data-testid", "csv-preflight-why"),
			w.Date+" · "+w.Desc+" · "+money.FormatMinor(w.AmountMinor, info.Dec)+" — "+reason))
	}
	// Transfer-pair notes (capped at 4).
	var pairRows []ui.Node
	for i, p := range info.Pairs {
		if i == 4 {
			break
		}
		pairRows = append(pairRows, P(css.Class("muted", tw.Text12), Attr("data-testid", "csv-preflight-pair"),
			uistate.T("documents.preflightPairLine", p.IncomingDesc, money.FormatMinor(p.AmountMinor, info.Dec), p.OtherDesc)))
	}

	// C689: the outcomes that are NOT plain duplicates, each said in its own
	// words. A row repeated inside the file and a row that cannot be read at all
	// were previously either invisible or lumped in with "skipped", and they mean
	// opposite things — one is the file being redundant, the other is money that
	// will silently not arrive.
	c := info.Batch.Counts
	var extra []ui.Node
	if c.RepeatedInFile > 0 {
		extra = append(extra, P(css.Class("muted", tw.Text12), Attr("data-testid", "csv-preflight-repeated"),
			uistate.T("documents.preflightRepeated", c.RepeatedInFile)))
	}
	if c.Unusable > 0 {
		extra = append(extra, P(css.Class("t-caption"), Attr("data-testid", "csv-preflight-unusable"),
			Attr("role", "alert"), uistate.T("documents.preflightUnusable", c.Unusable)))
	}
	if len(info.Failures) > 0 {
		failureCopy := uistate.T("documents.preflightFailed", importCount(len(info.Failures), "row", "rows"), csvSkipDetail(info.Failures))
		if len(info.Failures) == 1 {
			failureCopy = uistate.T("documents.preflightFailedOne", csvSkipDetail(info.Failures))
		}
		extra = append(extra, P(css.Class("t-caption"), Attr("data-testid", "csv-preflight-failures"),
			failureCopy))
	}

	return Div(css.Class("notice", tw.Mt2), Attr("data-testid", "csv-preflight"),
		P(Attr("data-testid", "csv-preflight-counts"),
			uistate.T("documents.preflightCounts", importCount(newRows, "new transaction", "new transactions"), info.Dupes)),
		extra,
		If(info.HasBal, P(Attr("data-testid", "csv-preflight-balance"),
			uistate.T("documents.preflightBalance", info.AcctName,
				money.FormatMinor(info.BeforeMinor, info.Dec), money.FormatMinor(info.AfterMinor, info.Dec), signedNet))),
		If(info.Jump, Div(css.Class("notice notice-warn", tw.Mt1), Attr("role", "alert"),
			Attr("data-testid", "csv-preflight-jump"),
			uistate.T("documents.preflightJumpWarn"))),
		If(len(whyRows) > 0, Details(css.Class(tw.Mt1), Attr("data-testid", "csv-preflight-dups"),
			Summary(uistate.T("documents.preflightWhyDups", info.Dupes)),
			Div(whyRows))),
		If(len(pairRows) > 0, Div(css.Class(tw.Mt1), Attr("data-testid", "csv-preflight-transfers"),
			P(css.Class(tw.Text13), uistate.T("documents.preflightPairs", len(info.Pairs))),
			Div(pairRows))),
		Div(css.Class(tw.Flex, tw.Gap2, tw.Mt2),
			Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "csv-preflight-confirm"),
				OnClick(confirm), uistate.T("documents.preflightImportNow", newRows)),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "csv-preflight-cancel"),
				OnClick(cancel), uistate.T("action.cancel")),
		),
	)
}

// reviewImpactCard shows where the import account's balance lands if every
// reviewed draft row commits (#57), with the implausible-jump warning. Hidden
// in receipt mode (one merchant total, not a statement) and when the rows or
// account can't be resolved. Pure renderer — amounts parse exactly as the
// import will parse them.
func reviewImpactCard(app *appstate.App, accounts []domain.Account, acctID string, rows []extract.Row, receiptMode bool) ui.Node {
	if len(rows) == 0 || receiptMode || app == nil {
		return Fragment()
	}
	preview := importdraft.RawPreview(accounts, app.Transactions(), acctID, rows)
	if !preview.HasBalance {
		return Fragment()
	}
	acc, dec, net := preview.Account, preview.Decimals, preview.Net
	signed := importdraft.SignedMinor(net, dec)
	return Div(css.Class("notice", tw.Mt2), Attr("data-testid", "review-balance-impact"),
		P(uistate.T("documents.preflightBalance", acc.Name,
			money.FormatMinor(preview.Before, dec), money.FormatMinor(preview.After, dec), signed)),
		If(preview.Jump, Div(css.Class("notice notice-warn", tw.Mt1),
			Attr("role", "alert"), Attr("data-testid", "review-jump-warn"),
			uistate.T("documents.preflightJumpWarn"))),
	)
}

// learnReceiptChoices extends the learn tally (#58): every confirmed receipt —
// attached or created — records its merchant→category choices so future
// categorization suggestions remember what this household calls this store.
func learnReceiptChoices(merchant string, splits []domain.CategorySplit) {
	merchant = strings.TrimSpace(merchant)
	if merchant == "" {
		return
	}
	for _, categoryID := range importdraft.LearnedCategories(splits) {
		uistate.IncrementLearnTally(merchant, categoryID)
	}
}

// receiptMatchOffer renders the #58 attach-to-existing offer: the ledger
// charges whose amount exactly matches the reviewed receipt near its date,
// each with a one-click Attach. "Create new" stays the Import button below.
func receiptMatchOffer(cands []receiptmatch.Candidate, onAttach func(txnID string)) ui.Node {
	keyOf := func(c receiptmatch.Candidate) any { return c.Txn.ID }
	render := func(c receiptmatch.Candidate) ui.Node {
		return ui.CreateElement(receiptMatchRow, receiptMatchRowProps{Cand: c, OnAttach: onAttach})
	}
	return Div(css.Class("notice", tw.Mt2), Attr("data-testid", "receipt-match-offer"),
		P(uistate.T("documents.receiptMatchLead", len(cands))),
		Div(css.Class("rows"), MapKeyed(cands, keyOf, render)),
		P(css.Class("muted", tw.Text12), uistate.T("documents.receiptMatchHint")),
	)
}

// receiptMatchRowProps feeds one candidate row.
type receiptMatchRowProps struct {
	Cand     receiptmatch.Candidate
	OnAttach func(txnID string)
}

// receiptMatchRow is its own component so the Attach click hook sits at a
// stable position per row.
func receiptMatchRow(p receiptMatchRowProps) ui.Node {
	attach := ui.UseEvent(Prevent(func() { p.OnAttach(p.Cand.Txn.ID) }))
	t := p.Cand.Txn
	why := importdraft.MatchReason(p.Cand, uistate.T)
	return Div(css.Class("row"), Attr("data-testid", "receipt-match-row"),
		Style(map[string]string{"display": "flex", "justify-content": "space-between", "align-items": "center", "gap": "1rem"}),
		Div(
			Div(t.Date.Format("Jan 2, 2006")+" · "+t.Desc+" · "+fmtMoney(t.Amount)),
			Div(css.Class(tw.TextFaint, tw.Text12), why),
		),
		Button(css.Class("btn btn-primary", tw.ShrinkO), Type("button"), Attr("data-testid", "receipt-match-attach"),
			Title(uistate.T("documents.receiptAttachTitle")), OnClick(attach), uistate.T("documents.receiptAttachBtn")),
	)
}

// importHistorySection lists the most recent import runs (newest first, capped
// at five) with their full result and — while the run's pre-import checkpoint
// is still in the #55 ring — a one-click roll-back for that exact import. When a
// run's snapshot has aged out of the ring its Roll back is gone, so the section
// hint is honest about that and offers the always-available fallback: a link to
// the Activity timeline (onViewActivity) where any import can still be reviewed.
func importHistorySection(docs []domain.Document, accounts []domain.Account, txns []domain.Transaction, onRollback func(domain.Document), onViewActivity ui.Handler) ui.Node {
	// Newest first by upload time (store order isn't chronological — the
	// sample dataset seeds history rows with past dates).
	sorted := ui.UseMemo(func() []domain.Document { return importdraft.RecentDocuments(docs) }, docs, len(docs))
	if len(sorted) == 0 {
		return Fragment()
	}
	keyOf := func(d domain.Document) any { return d.ID }
	render := func(d domain.Document) ui.Node {
		return ui.CreateElement(importRunRow, importRunRowProps{Doc: d, AcctName: importdraft.AccountName(accounts, d.AccountID), Txns: txns, OnRollback: onRollback})
	}
	return Div(css.Class(tw.Mt3), Attr("data-testid", "import-history"),
		P(css.Class("t-caption"), uistate.T("documents.historyHeading")),
		P(css.Class("muted", tw.Text12), uistate.T("documents.historyHintHonest")),
		Div(css.Class("rows"), MapKeyed(sorted, keyOf, render)),
		A(css.Class("btn-link", tw.Text12), Href(uistate.RoutePath("/activity")),
			Attr("data-testid", "import-history-activity"), OnClick(onViewActivity),
			uistate.T("documents.historyActivityLink")),
	)
}

// importRunRowProps feeds one import-history row.
type importRunRowProps struct {
	Doc      domain.Document
	AcctName string
	// Txns is the live ledger, so the row can say how many of this run's
	// transactions are still in it (C687).
	Txns       []domain.Transaction
	OnRollback func(domain.Document)
}

// importRunRow is its own component so its click hook sits at a stable
// position per row. The roll-back button only renders while the run's
// checkpoint is still in the ring.
func importRunRow(p importRunRowProps) ui.Node {
	rollback := ui.UseEvent(Prevent(func() { p.OnRollback(p.Doc) }))
	d := p.Doc
	label, result, when := importdraft.HistoryText(d, p.AcctName, p.Txns, uistate.T)
	return Div(css.Class("row"), Attr("data-testid", "import-history-row"),
		Style(map[string]string{"display": "flex", "justify-content": "space-between", "align-items": "center", "gap": "1rem"}),
		Div(
			Div(label),
			Div(css.Class(tw.TextFaint, tw.Text12), when+" · "+result),
		),
		If(uistate.HasCheckpoint(d.CheckpointID),
			Button(css.Class("btn", tw.ShrinkO), Type("button"), Attr("data-testid", "import-rollback"),
				Title(uistate.T("documents.rollbackTitle")), OnClick(rollback), uistate.T("documents.rollbackBtn"))),
	)
}
