// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
	"github.com/monstercameron/CashFlux/internal/receiptsplit"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/uistate"
)

// startReceiptSplitFlow drives XC11 from a transaction's ⋯ menu: pick a receipt
// image, have the vision pipeline read its line items, propose a category breakdown
// (receiptsplit.Propose), and pre-fill the shipped split editor for the user to
// review and save. It reuses the documents vision pipeline (same model, prompt,
// schema, and extract.ParseRows) rather than building a new AI call path.
//
// Gating: this is a BYO-key AI action. Without an OpenAI key (and no cloud proxy),
// it explains what is needed and leaves the manual split flow untouched.
func startReceiptSplitFlow(app *appstate.App, txn domain.Transaction) {
	if app == nil {
		return
	}
	settings := app.Settings()
	pr := uistate.CurrentPrefs().Normalize()
	useBackendAI := pr.BackendActive()
	if settings.OpenAIKey == "" && !useBackendAI {
		uistate.PostNotice(uistate.T("receiptsplit.needsKey"), true)
		return
	}

	pickImageDataURL(
		func(imageURL string) {
			runReceiptSplitVision(app, txn, imageURL, settings, pr, useBackendAI)
		},
		func(e string) { uistate.PostNotice(e, true) },
	)
}

// runReceiptSplitVision sends the chosen receipt image to the vision model, then
// turns the extracted rows into a proposed split and hands it to the editor.
func runReceiptSplitVision(app *appstate.App, txn domain.Transaction, imageURL string, settings store.Settings, pr prefs.Prefs, useBackendAI bool) {
	aiModel := settings.OpenAIModel
	if aiModel == "" || aiModel == "gpt-5.4-mini" {
		aiModel = "gpt-5.5" // receipt extraction needs the flagship vision model
	}
	uistate.PostNotice(uistate.T("receiptsplit.reading"), false)

	onResult := func(content string, _ ai.Usage) {
		rows, err := extract.ParseRows(content)
		if err != nil {
			uistate.PostNotice(err.Error(), true)
			return
		}
		if len(rows) == 0 {
			uistate.PostNotice(uistate.T("receiptsplit.noneFound"), true)
			return
		}
		proposeFromRows(app, txn, rows)
	}
	onError := func(e string) { uistate.PostNotice(e, true) }

	if useBackendAI {
		ai.SendProxyStructuredVisionChat(pr.ServerURL, pr.ServerToken, aiModel, visionSystemPrompt,
			"Extract every line item you can read from this receipt.", imageURL, 0,
			"transactions", []byte(visionExtractionSchema), onResult, onError)
	} else {
		ai.SendStructuredVisionChat(settings.OpenAIKey, ai.DefaultBaseURL, aiModel, visionSystemPrompt,
			"Extract every line item you can read from this receipt.", imageURL, 0,
			"transactions", []byte(visionExtractionSchema), onResult, onError)
	}
}

// proposeFromRows resolves each extracted row to a category (existing category name
// first, then the auto-categorization rules), builds the pure proposal, and opens
// the split editor pre-filled. When nothing useful can be proposed (no line matched
// a category, or a currency mismatch), it says so and leaves the manual flow.
func proposeFromRows(app *appstate.App, txn domain.Transaction, rows []extract.Row) {
	cur := txn.Amount.Currency
	if cur == "" {
		cur = app.Settings().BaseCurrency
	}
	dec := currency.Decimals(cur)
	cats := app.Categories()
	appRules := app.Rules()

	// Pre-resolve each row to a category id using full row context (the vision
	// category name, then rules on the description), then hand receiptsplit a simple
	// name->category MatchFunc so the pure package stays rules-agnostic.
	lineCat := make(map[string]string, len(rows))
	items := make([]receiptsplit.LineItem, 0, len(rows))
	for _, r := range rows {
		name := strings.TrimSpace(r.Description)
		minor, err := parseReceiptAmount(r.Amount, dec)
		if err != nil {
			continue // skip an unreadable line rather than aborting the whole proposal
		}
		cid := resolveExpenseCategoryName(cats, r.Category)
		if cid == "" {
			if m := rules.FirstMatch(appRules, name); m != nil {
				cid = m.SetCategoryID
			}
		}
		lineCat[name] = cid
		items = append(items, receiptsplit.LineItem{Name: name, Amount: money.New(minor, cur)})
	}

	match := func(name string) string { return lineCat[name] }
	proposal, ok := receiptsplit.Propose(items, receiptsplit.Target{
		Amount:     txn.Amount,
		CategoryID: txn.CategoryID,
	}, match)
	if !ok {
		uistate.PostNotice(uistate.T("receiptsplit.noProposal"), true)
		return
	}
	uistate.SetTxnSplitProposal(txn.ID, proposal.Splits, proposal.Note)
}

// parseReceiptAmount parses a vision-emitted line amount ("$12.34", "1,234.50") to a
// positive minor value; the sign is applied by receiptsplit from the transaction.
func parseReceiptAmount(s string, dec int) (int64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "$", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimPrefix(s, "-")
	v, err := money.ParseMinor(s, dec)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		v = -v
	}
	return v, nil
}

// resolveExpenseCategoryName maps a free-text category name to an existing expense
// category id: exact (case-insensitive) name, else a substring match either way.
// "" when nothing fits. Mirrors appstate's receipt-import resolution.
func resolveExpenseCategoryName(cats []domain.Category, name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, c := range cats {
		if c.Kind == domain.KindExpense && strings.ToLower(c.Name) == name {
			return c.ID
		}
	}
	for _, c := range cats {
		if c.Kind != domain.KindExpense {
			continue
		}
		cn := strings.ToLower(c.Name)
		if cn != "" && (strings.Contains(cn, name) || strings.Contains(name, cn)) {
			return c.ID
		}
	}
	return ""
}
