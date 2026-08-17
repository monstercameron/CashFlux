// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/csv"
	"strconv"
	"strings"

	uiw "github.com/monstercameron/CashFlux/internal/ui"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/holdingimport"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

// holdingImportProps carries the accounts a paste can land in.
type holdingImportProps struct {
	Accounts []domain.Account
	// OnDone refreshes the surrounding surface after a commit.
	OnDone func()
}

// holdingImportPanel is the paste-a-brokerage-export importer (C376).
//
// Adding an investment account meant typing every position by hand while every
// brokerage exports exactly this table. The panel takes a paste, guesses the
// column mapping from the header, and shows what committing WOULD do — added,
// updated, or skipped with a reason — before writing anything.
//
// The preview is the feature, not the parser. An import that just says "12 rows"
// asks the user to trust a black box with their portfolio; showing that 3 are
// new, 8 update existing positions and 1 could not be read is what makes the
// commit button safe to press.
func holdingImportPanel(props holdingImportProps) ui.Node {
	open := ui.UseState(false)
	raw := ui.UseState("")
	acct := ui.UseState("")
	// committed keeps the last result on screen after a commit, so the panel
	// reports what it did rather than resetting to an empty box that looks like
	// nothing happened.
	committed := ui.UseState("")

	toggle := ui.UseEvent(Prevent(func() { open.Set(!open.Get()); committed.Set("") }))
	onRaw := ui.UseEvent(func(v string) { raw.Set(v); committed.Set("") })
	onAcct := ui.UseEvent(func(e ui.Event) { acct.Set(e.GetValue()) })

	// Investment accounts only: pasting a portfolio into a checking account is
	// not a thing to make possible and then warn about.
	eligible := make([]domain.Account, 0, len(props.Accounts))
	for _, a := range props.Accounts {
		// Investment, retirement and crypto accounts all hold positions; the
		// importer is about a table of holdings, not about one account type.
		switch a.Type {
		case domain.TypeInvestment, domain.TypeRetirement, domain.TypeCrypto:
		default:
			continue
		}
		if !a.Archived {
			eligible = append(eligible, a)
		}
	}
	target := acct.Get()
	if target == "" && len(eligible) == 1 {
		// One investment account is not a choice; picking it for the user removes
		// a required step that has exactly one answer.
		target = eligible[0].ID
	}

	profile, rows, plan, parseErr := holdingImportPlan(raw.Get(), target)

	commit := ui.UseEvent(Prevent(func() {
		app := appstate.Default
		if app == nil || target == "" {
			return
		}
		_, _, cur, err := holdingImportPlan(raw.Get(), target)
		if err != "" {
			return
		}
		var wrote int
		for _, c := range cur {
			if c.Action == holdingimport.ActionSkip {
				continue
			}
			if putErr := app.PutHolding(c.After); putErr != nil {
				uistate.PostNotice(putErr.Error(), true)
				return
			}
			wrote++
		}
		s := holdingimport.Summarize(cur)
		committed.Set(uistate.T("holdingImport.done", s.Add, s.Update, s.Skip))
		raw.Set("")
		uistate.PostNotice(uistate.T("holdingImport.done", s.Add, s.Update, s.Skip), false)
		uistate.BumpDataRevision()
		if props.OnDone != nil {
			props.OnDone()
		}
	}))

	if !open.Get() {
		return Div(css.Class("hld-import-entry"),
			Button(css.Class("btn btn-tool"), Type("button"), Attr("data-testid", "hld-import-open"),
				OnClick(toggle), uistate.T("holdingImport.open")))
	}

	acctOpts := []ui.Node{Option(Value(""), SelectedIf(target == ""), uistate.T("holdingImport.pickAccount"))}
	for _, a := range eligible {
		acctOpts = append(acctOpts, Option(Value(a.ID), SelectedIf(target == a.ID), a.Name))
	}

	summary := holdingimport.Summarize(plan)
	var body ui.Node = Fragment()
	switch {
	case strings.TrimSpace(raw.Get()) == "":
		body = P(css.Class("muted"), uistate.T("holdingImport.hint"))
	case parseErr != "":
		body = P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "hld-import-error"), parseErr)
	default:
		body = Fragment(
			P(css.Class("muted"), Attr("data-testid", "hld-import-summary"),
				Attr("role", "status"), Attr("aria-live", "polite"),
				uistate.T("holdingImport.summary", summary.Add, summary.Update, summary.Skip)),
			holdingImportTable(plan, target, props.Accounts),
			// The mapping the header produced, stated plainly. A guess the user
			// cannot see is a guess they cannot correct.
			P(css.Class("hld-import-map"), uistate.T("holdingImport.mapped", holdingImportMapping(profile, rows))),
		)
	}

	canCommit := target != "" && parseErr == "" && summary.Writes() > 0

	return Div(css.Class("hld-import"), Attr("data-testid", "hld-import"),
		Div(css.Class("hld-import-head"),
			H4(css.Class("t-caption"), uistate.T("holdingImport.title")),
			Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "hld-import-close"),
				OnClick(toggle), uistate.T("action.cancel"))),
		Label(css.Class("hld-import-ctrl"),
			Span(css.Class("t-caption"), uistate.T("holdingImport.account")),
			Select(css.Class("field"), Attr("data-testid", "hld-import-account"),
				Attr("aria-label", uistate.T("holdingImport.account")), OnChange(onAcct), acctOpts)),
		uiw.AreaField(raw.Get(), css.Class("field hld-import-text"), Attr("rows", "8"),
			Attr("data-testid", "hld-import-paste"),
			Attr("aria-label", uistate.T("holdingImport.pasteLabel")),
			Placeholder(uistate.T("holdingImport.placeholder")), OnInput(onRaw)),
		body,
		If(committed.Get() != "", P(css.Class("muted"), Attr("data-testid", "hld-import-done"), committed.Get())),
		Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "hld-import-commit"),
			Attr("aria-disabled", ariaBool(!canCommit)), Attr("disabled", disabledIf(!canCommit)),
			OnClick(commit), uistate.T("holdingImport.commit", summary.Writes())),
	)
}

// holdingImportPlan parses a paste and plans it against the target account.
// Returns the profile, the parsed rows, the plan, and a human-readable error.
//
// It is a plain function rather than state so the preview and the commit are
// computed the SAME way: a commit that re-derived the plan differently from the
// preview would write something other than what the user approved.
func holdingImportPlan(text, accountID string) (holdingimport.Profile, [][]string, []holdingimport.Change, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return holdingimport.Profile{}, nil, nil, ""
	}
	rows, err := parseDelimited(text)
	if err != nil {
		return holdingimport.Profile{}, nil, nil, uistate.T("holdingImport.errParse", err.Error())
	}
	if len(rows) < 2 {
		return holdingimport.Profile{}, rows, nil, uistate.T("holdingImport.errNoRows")
	}
	dec := 2
	if app := appstate.Default; app != nil {
		dec = currency.Decimals(app.Settings().BaseCurrency)
	}
	p := holdingimport.GuessProfile(rows[0], dec)
	if vErr := p.Validate(); vErr != nil {
		return p, rows, nil, uistate.T("holdingImport.errProfile", vErr.Error())
	}
	parsed := holdingimport.Parse(p, rows)
	if accountID == "" {
		return p, rows, nil, ""
	}
	var existing []domain.Holding
	if app := appstate.Default; app != nil {
		existing = app.Holdings()
	}
	return p, rows, holdingimport.Plan(accountID, existing, parsed, id.New), ""
}

// parseDelimited reads a paste as CSV, falling back to TAB when the first line
// has no comma. Spreadsheet copy-paste is tab-separated, and it is the most
// likely way a portfolio reaches this box.
func parseDelimited(text string) ([][]string, error) {
	comma := ','
	if first, _, _ := strings.Cut(text, "\n"); !strings.Contains(first, ",") && strings.Contains(first, "\t") {
		comma = '\t'
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = comma
	// Brokerage exports are not always rectangular — a trailing total row or a
	// disclaimer line has a different field count, and rejecting the whole file
	// for it would be useless. Ragged rows are parsed and then judged per row.
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

// holdingImportTable renders the preview: one row per source row with what
// committing would do to it.
func holdingImportTable(plan []holdingimport.Change, accountID string, accounts []domain.Account) ui.Node {
	if len(plan) == 0 {
		return Fragment()
	}
	rows := make([]any, 0, len(plan))
	for _, c := range plan {
		rows = append(rows, holdingImportRow(c))
	}
	return Table(css.Class("hld-import-table"), Attr("data-testid", "hld-import-table"),
		Thead(Tr(
			Th(uistate.T("holdingImport.colAction")),
			Th(uistate.T("holdingImport.colPosition")),
			Th(uistate.T("holdingImport.colShares")),
			Th(uistate.T("holdingImport.colNote")))),
		Tbody(rows...))
}

// holdingImportRow is one preview row. An update states the share count it
// REPLACES, because "12" alone does not tell you whether that is a change.
func holdingImportRow(c holdingimport.Change) ui.Node {
	label := uistate.T("holdingImport.actionSkip")
	cls := "hld-import-skip"
	switch c.Action {
	case holdingimport.ActionAdd:
		label, cls = uistate.T("holdingImport.actionAdd"), "hld-import-add"
	case holdingimport.ActionUpdate:
		label, cls = uistate.T("holdingImport.actionUpdate"), "hld-import-update"
	}
	name := c.Row.Ticker
	if name == "" {
		name = c.Row.Name
	}
	shares := ""
	switch c.Action {
	case holdingimport.ActionUpdate:
		shares = uistate.T("holdingImport.sharesFrom",
			trimFloat(c.Before.Shares), trimFloat(c.After.Shares))
	case holdingimport.ActionAdd:
		shares = trimFloat(c.After.Shares)
	}
	return Tr(Attr("data-testid", "hld-import-row-"+strconv.Itoa(c.Row.Line)),
		Td(Span(ClassStr("hld-import-tag "+cls), label)),
		Td(name),
		Td(shares),
		Td(css.Class("muted"), c.Reason))
}

// holdingImportMapping states, in words, which source column became which field.
// A guess the user cannot see is a guess they cannot correct.
func holdingImportMapping(p holdingimport.Profile, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var parts []string
	for i, h := range rows[0] {
		f := p.Field(i)
		if f == holdingimport.FieldNone {
			continue
		}
		parts = append(parts, strings.TrimSpace(h)+" → "+holdingImportFieldLabel(f))
	}
	if len(parts) == 0 {
		return uistate.T("holdingImport.mapNone")
	}
	return strings.Join(parts, " · ")
}

func holdingImportFieldLabel(f holdingimport.Field) string {
	switch f {
	case holdingimport.FieldTicker:
		return uistate.T("holdingImport.fTicker")
	case holdingimport.FieldName:
		return uistate.T("holdingImport.fName")
	case holdingimport.FieldShares:
		return uistate.T("holdingImport.fShares")
	case holdingimport.FieldCostBasis:
		return uistate.T("holdingImport.fCost")
	case holdingimport.FieldPrice:
		return uistate.T("holdingImport.fPrice")
	case holdingimport.FieldAssetClass:
		return uistate.T("holdingImport.fClass")
	case holdingimport.FieldSector:
		return uistate.T("holdingImport.fSector")
	case holdingimport.FieldRegion:
		return uistate.T("holdingImport.fRegion")
	}
	return string(f)
}

// trimFloat renders a share count without trailing zeros — "12" not "12.000000".
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// disabledIf returns the value for a boolean HTML attribute.
func disabledIf(b bool) string {
	if b {
		return "disabled"
	}
	return ""
}
