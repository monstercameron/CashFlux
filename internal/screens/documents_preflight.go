// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"sort"
	"strings"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/importsafe"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// csvPreflightInfo is the computed pre-commit preview for a staged CSV import
// (#57): counts, balance impact on the import account, the implausible-jump
// flag, per-row duplicate reasons, and likely transfer pairs. Built by
// buildCSVPreflight in documents.go; nothing writes until the user confirms.
type csvPreflightInfo struct {
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
}

// csvPreflightCard renders the staged import's pre-commit preview with the
// only two ways forward: Import now, or Cancel (which drops the staged bytes).
func csvPreflightCard(info *csvPreflightInfo, confirm, cancel ui.Handler) ui.Node {
	if info == nil {
		return Fragment()
	}
	newRows := info.Total - info.Dupes
	signedNet := money.FormatMinor(info.NetMinor, info.Dec)
	if info.NetMinor > 0 {
		signedNet = "+" + signedNet
	}

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

	return Div(css.Class("notice", tw.Mt2), Attr("data-testid", "csv-preflight"),
		P(Attr("data-testid", "csv-preflight-counts"),
			uistate.T("documents.preflightCounts", plural(newRows, "new transaction"), info.Dupes)),
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
	acc, ok := domain.AccountByID(accounts, acctID)
	if !ok {
		return Fragment()
	}
	dec := currency.Decimals(acc.Currency)
	var net int64
	for _, r := range rows {
		if amt, err := money.ParseMinor(strings.TrimSpace(r.Amount), dec); err == nil {
			net += amt
		}
	}
	bal, err := ledger.Balance(acc, app.Transactions())
	if err != nil {
		return Fragment()
	}
	signed := money.FormatMinor(net, dec)
	if net > 0 {
		signed = "+" + signed
	}
	return Div(css.Class("notice", tw.Mt2), Attr("data-testid", "review-balance-impact"),
		P(uistate.T("documents.preflightBalance", acc.Name,
			money.FormatMinor(bal.Amount, dec), money.FormatMinor(bal.Amount+net, dec), signed)),
		If(importsafe.JumpWarning(bal.Amount, net), Div(css.Class("notice notice-warn", tw.Mt1),
			Attr("role", "alert"), Attr("data-testid", "review-jump-warn"),
			uistate.T("documents.preflightJumpWarn"))),
	)
}

// importHistorySection lists the most recent import runs (newest first, capped
// at five) with their full result and — while the run's pre-import checkpoint
// is still in the #55 ring — a one-click roll-back for that exact import.
func importHistorySection(docs []domain.Document, accounts []domain.Account, onRollback func(domain.Document)) ui.Node {
	// Newest first by upload time (store order isn't chronological — the
	// sample dataset seeds history rows with past dates).
	sorted := append([]domain.Document(nil), docs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].UploadedAt.After(sorted[j].UploadedAt) })
	if len(sorted) > 5 {
		sorted = sorted[:5]
	}
	if len(sorted) == 0 {
		return Fragment()
	}
	nameOf := func(id string) string {
		for _, a := range accounts {
			if a.ID == id {
				return a.Name
			}
		}
		return ""
	}
	keyOf := func(d domain.Document) any { return d.ID }
	render := func(d domain.Document) ui.Node {
		return ui.CreateElement(importRunRow, importRunRowProps{Doc: d, AcctName: nameOf(d.AccountID), OnRollback: onRollback})
	}
	return Div(css.Class(tw.Mt3), Attr("data-testid", "import-history"),
		P(css.Class("t-caption"), uistate.T("documents.historyHeading")),
		P(css.Class("muted", tw.Text12), uistate.T("documents.historyHint")),
		Div(css.Class("rows"), MapKeyed(sorted, keyOf, render)),
	)
}

// importRunRowProps feeds one import-history row.
type importRunRowProps struct {
	Doc        domain.Document
	AcctName   string
	OnRollback func(domain.Document)
}

// importRunRow is its own component so its click hook sits at a stable
// position per row. The roll-back button only renders while the run's
// checkpoint is still in the ring.
func importRunRow(p importRunRowProps) ui.Node {
	rollback := ui.UseEvent(Prevent(func() { p.OnRollback(p.Doc) }))
	d := p.Doc
	kindLabel := uistate.T("documents.historyKindCSV")
	if d.Kind == domain.DocImage {
		kindLabel = uistate.T("documents.historyKindDoc")
	}
	imported := d.RowCount
	if imported == 0 {
		imported = len(d.Extracted)
	}
	result := uistate.T("documents.historyImported", plural(imported, "transaction"))
	if d.SkippedCount > 0 {
		result += " " + uistate.T("documents.historySkipped", d.SkippedCount)
	}
	when := d.UploadedAt.Format("Jan 2, 2006 3:04 PM")
	label := kindLabel
	if p.AcctName != "" {
		label += " · " + p.AcctName
	}
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
