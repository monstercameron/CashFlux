// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/base64"
	"strings"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/money"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// statementImportSchema constrains the model's reply to a transactions array (strict
// structured outputs). Category is a free string here; it's constrained to the user's
// EXISTING categories by the prompt and enforced on import (unknown → unmapped), so the
// model can't create orphan categories.
const statementImportSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["transactions"],
  "properties": {
    "transactions": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["date", "description", "amount", "category"],
        "properties": {
          "date": {"type": "string"},
          "description": {"type": "string"},
          "amount": {"type": "string"},
          "category": {"type": "string"}
        }
      }
    }
  }
}`

// statementSystemPrompt builds the extraction instruction, listing the household's
// existing category names so the model maps to them best-effort and leaves the category
// blank otherwise — never inventing a new one.
func statementSystemPrompt(catNames []string) string {
	base := "You extract EVERY transaction from an attached bank or credit-card statement (a PDF). " +
		"For each transaction return: date (YYYY-MM-DD), description (the merchant/payee, tidied), " +
		"amount as a string (negative for money out/expenses like \"-45.00\", positive for money in), and category. " +
		"Skip non-transaction lines (opening/closing balance, totals, interest summaries, payment-due notices). "
	if len(catNames) > 0 {
		base += "For category, use EXACTLY one of these existing categories when one clearly fits, " +
			"otherwise return an empty string \"\" — NEVER invent a category not in this list. " +
			"Existing categories: " + strings.Join(catNames, ", ") + "."
	} else {
		base += "Return an empty string \"\" for every category."
	}
	return base
}

// StatementImportBody is the body of the "Import statement" flip modal (mounted at the
// shell root by app.StatementImportHost). The user attaches a statement PDF; it's sent
// straight to the AI (which reads the PDF's text and page images natively — no
// client-side rendering), which returns transactions with categories best-effort mapped
// to the household's existing set (unmatched → left blank). The reused review table lets
// the user quick-edit every row before importing.
func StatementImportBody(_ struct{}) ui.Node {
	app := appstate.Default
	_ = uistate.UseDataRevision().Get()
	openAtom := uistate.UseStatementImportOpen()

	fileData := ui.UseState("") // data:application/pdf;base64,…
	fileName := ui.UseState("")
	loading := ui.UseState(false)
	errText := ui.UseState("")
	needsKey := ui.UseState(false)
	draft := ui.UseState([]extract.Row(nil))

	accounts := app.Accounts()
	defaultAcc := ""
	if len(accounts) > 0 {
		defaultAcc = accounts[0].ID
	}
	importAcct := ui.UseState(defaultAcc)

	settings := app.Settings()
	noopStr := ui.UseEvent(func(string) {}) // no-op for the reused review table's receipt-only inputs
	aiModel := settings.OpenAIModel
	if aiModel == "" || aiModel == "gpt-5.4-mini" {
		aiModel = "gpt-5.5" // needs a vision-capable model to read the PDF's pages
	}

	choose := ui.UseEvent(func() {
		pickFile(".pdf,application/pdf", func(name, mime string, data []byte) {
			if len(data) == 0 {
				errText.Set(uistate.T("statementimport.readErr"))
				return
			}
			if len(data) > 45*1024*1024 { // OpenAI caps files at 50MB; stay under it
				errText.Set(uistate.T("statementimport.tooLarge"))
				return
			}
			mt := mime
			if mt == "" {
				mt = "application/pdf"
			}
			fileData.Set("data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(data))
			fileName.Set(name)
			errText.Set("")
			needsKey.Set(false)
			draft.Set(nil)
		})
	})

	runAI := ui.UseEvent(func() {
		if fileData.Get() == "" {
			errText.Set(uistate.T("statementimport.chooseFirst"))
			return
		}
		if settings.OpenAIKey == "" {
			// The optional cloud proxy doesn't carry file uploads — statement import is
			// BYO-key. (Everything else works on the backend.)
			needsKey.Set(true)
			return
		}
		var catNames []string
		for _, c := range app.Categories() {
			if c.Kind == domain.KindExpense || c.Kind == domain.KindIncome {
				catNames = append(catNames, c.Name)
			}
		}
		loading.Set(true)
		errText.Set("")
		ai.SendStructuredFileChat(settings.OpenAIKey, ai.DefaultBaseURL, aiModel,
			statementSystemPrompt(catNames), "Extract every transaction from this statement.",
			firstNonEmpty(fileName.Get(), "statement.pdf"), fileData.Get(), 0.1,
			"transactions", []byte(statementImportSchema),
			func(content string, _ ai.Usage) {
				loading.Set(false)
				rows, err := extract.ParseRows(content)
				if err != nil {
					errText.Set(err.Error())
					return
				}
				if len(rows) == 0 {
					errText.Set(uistate.T("statementimport.noneFound"))
					return
				}
				draft.Set(rows)
			},
			func(e string) { loading.Set(false); errText.Set(e) })
	})

	removeDraft := func(i int) {
		cur := draft.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := make([]extract.Row, 0, len(cur)-1)
		next = append(next, cur[:i]...)
		next = append(next, cur[i+1:]...)
		draft.Set(next)
	}
	updateDraft := func(i int, r extract.Row) {
		cur := draft.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := append([]extract.Row(nil), cur...)
		next[i] = r
		draft.Set(next)
	}
	onAcct := ui.UseEvent(func(v string) { importAcct.Set(v) })
	clearDraft := ui.UseEvent(Prevent(func() { draft.Set(nil) }))

	importDraft := ui.UseEvent(Prevent(func() {
		result, err := app.ImportReviewedDocumentRows(domain.DocImage, importAcct.Get(), draft.Get())
		if err != nil {
			errText.Set(uistate.T("documents.chooseAccount"))
			return
		}
		summary := uistate.T("documents.importedImage", plural(result.Imported, "transaction"))
		if result.Skipped > 0 {
			summary += uistate.T("documents.skipped", plural(result.Skipped, "duplicate"))
		}
		uistate.PostNotice(summary, false)
		uistate.BumpDataRevision()
		openAtom.Set(false)
	}))
	onCancel := ui.UseEvent(Prevent(func() { openAtom.Set(false) }))

	// Currency for the review amounts (the chosen import account's, else base).
	reviewCur := settings.BaseCurrency
	if reviewCur == "" {
		reviewCur = "USD"
	}
	for _, a := range accounts {
		if a.ID == importAcct.Get() {
			reviewCur = a.Currency
			break
		}
	}
	// Dedupe signatures for the chosen account so the review list badges already-imported rows.
	seenSigs := map[string]bool{}
	if importAcct.Get() != "" {
		dec := currency.Decimals(reviewCur)
		for _, t := range app.Transactions() {
			if t.AccountID != importAcct.Get() {
				continue
			}
			seenSigs[extract.Row{Date: t.Date.Format("2006-01-02"), Amount: money.FormatMinor(t.Amount.Amount, dec)}.Signature()] = true
		}
	}

	rows := draft.Get()

	// ---- render ----
	var body ui.Node
	switch {
	case needsKey.Get():
		body = P(css.Class("notice"), Attr("data-testid", "statementimport-needskey"), uistate.T("statementimport.needsKey"))
	case len(rows) > 0:
		body = DraftReviewList(draftReviewListProps{
			Rows: rows, Accounts: accounts, Categories: app.Categories(),
			ReviewCur: reviewCur, ImportAcctID: importAcct.Get(),
			ReceiptMode: false, RecBaseCur: reviewCur, SeenSigs: seenSigs,
			ClearDraft: clearDraft, Toggle: Fragment(),
			OnAcctChange: onAcct, OnReceiptTotal: noopStr, OnReceiptMerchant: noopStr,
			OnImportDraft: importDraft, OnImportReceipt: importDraft,
			OnRemoveDraft: removeDraft, OnUpdateDraft: updateDraft,
		})
	default:
		// Empty / pre-scan state: choose a file, then Read.
		fileLabel := uistate.T("statementimport.noFile")
		if n := strings.TrimSpace(fileName.Get()); n != "" {
			fileLabel = n
		}
		body = Div(css.Class(tw.FlexCol, tw.Gap3),
			P(css.Class("muted", tw.Text13), Style(map[string]string{"margin": "0"}), uistate.T("statementimport.intro")),
			Div(css.Class("statement-drop"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "statementimport-choose"), OnClick(choose),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("statementimport.choose"))),
				Span(css.Class("statement-file", tw.TextDim), Attr("data-testid", "statementimport-filename"), fileLabel)),
			If(loading.Get(), P(css.Class("muted"), Attr("data-testid", "statementimport-loading"), uistate.T("statementimport.reading"))),
			buttonWithDisabled(fileData.Get() == "" || loading.Get(),
				[]any{css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "statementimport-run"), OnClick(runAI)},
				uistate.T("statementimport.run")),
		)
	}

	return Div(css.Class(tw.FlexCol, tw.Gap3),
		If(errText.Get() != "", P(css.Class("err"), Attr("role", "alert"), Attr("data-testid", "statementimport-err"), errText.Get())),
		body,
		If(len(rows) == 0 && !needsKey.Get(), Div(css.Class("txnlink-footer"),
			Button(css.Class("btn"), Type("button"), Attr("data-testid", "statementimport-cancel"), OnClick(onCancel), uistate.T("action.cancel")))),
		P(css.Class("t-caption", tw.TextFaint), Style(map[string]string{"margin": "0"}), uistate.T("statementimport.privacy")),
	)
}
