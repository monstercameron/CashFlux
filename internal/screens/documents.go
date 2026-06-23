//go:build js && wasm

package screens

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pdftext"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/router"
	"github.com/monstercameron/GoWebComponents/state"
	"github.com/monstercameron/GoWebComponents/ui"
)

const visionSystemPrompt = "You extract transactions from receipt and bank-statement images. " +
	"Return each transaction with: date (YYYY-MM-DD), description, amount (negative for money out / " +
	"expenses, positive for money in, as a string), and category."

// visionExtractionSchema constrains the vision reply to a transactions array
// (OpenAI structured outputs, strict). All fields required + additionalProperties
// false, as strict mode demands; the parser (extract.ParseRows) reads the
// resulting {"transactions":[…]} object.
const visionExtractionSchema = `{
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

// textExtractionSystemPrompt is the system prompt for "Extract with AI" on pasted
// statement text (C74). It mirrors visionSystemPrompt but operates on raw text.
const textExtractionSystemPrompt = "You extract bank/credit-card transactions from pasted statement text. " +
	"Return ONLY a JSON object: {\"transactions\":[{\"date\":\"YYYY-MM-DD\",\"description\":\"...\",\"amount\":\"...\",\"category\":\"\"}]}. " +
	"Amount: negative for money out/expenses (e.g. \"-45.00\"), positive for money in. No prose, no markdown."

// Documents imports transactions two ways: paste CSV (no AI), or read a receipt/
// statement image with the OpenAI vision model (bring-your-own-key). Extracted
// rows are reviewed, then imported into a chosen account through the validated
// write path.
func Documents() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	nav := router.UseNavigate()
	rev := state.UseAtom("rev:documents", 0)
	csvText := ui.UseState("")
	stmtText := ui.UseState("")
	msg := ui.UseState("")

	accounts := app.Accounts()
	defaultAcc := ""
	if len(accounts) > 0 {
		defaultAcc = accounts[0].ID
	}
	imageURL := ui.UseState("")
	aiLoading := ui.UseState(false)
	aiErr := ui.UseState("")
	needsKey := ui.UseState(false)
	draft := ui.UseState([]extract.Row{})
	importAcct := ui.UseState(defaultAcc)
	receiptMode := ui.UseState(false)  // import the draft as ONE split transaction
	receiptTotal := ui.UseState("")    // the receipt's single total (defaults to the line sum)
	receiptMerchant := ui.UseState("") // optional store name

	// C74 — import-map wizard state: populated when statement auto-map fails or
	// the user explicitly opens the wizard. wizardHeader holds the parsed column
	// names; wizardRawRows holds the raw CSV records (post-header) for Apply.
	wizardVisible := ui.UseState(false)
	wizardHeader := ui.UseState([]string{})
	wizardRawRows := ui.UseState([][][]string{})
	// Each wizard field tracks which column index maps to it ("-1" = not present).
	wizardDate := ui.UseState("-1")
	wizardDescCol := ui.UseState("-1")
	wizardAmount := ui.UseState("-1")
	wizardDebit := ui.UseState("-1")
	wizardCredit := ui.UseState("-1")

	// C74 — profile save/load
	profileName := ui.UseState("")
	showProfiles := ui.UseState(false)

	onCsv := ui.UseEvent(func(v string) { csvText.Set(v) })
	onStmt := ui.UseEvent(func(v string) { stmtText.Set(v) })

	// C74 wizard — one UseEvent per wizard field (fixed set, not a loop).
	onWizardDate := ui.UseEvent(func(e ui.Event) { wizardDate.Set(e.GetValue()) })
	onWizardDesc := ui.UseEvent(func(e ui.Event) { wizardDescCol.Set(e.GetValue()) })
	onWizardAmount := ui.UseEvent(func(e ui.Event) { wizardAmount.Set(e.GetValue()) })
	onWizardDebit := ui.UseEvent(func(e ui.Event) { wizardDebit.Set(e.GetValue()) })
	onWizardCredit := ui.UseEvent(func(e ui.Event) { wizardCredit.Set(e.GetValue()) })
	onProfileName := ui.UseEvent(func(v string) { profileName.Set(v) })

	// recordDocument saves a best-effort audit record of an import (logged by
	// appstate on failure). Declared before the import handlers that call it.
	recordDocument := func(kind domain.DocumentKind, accountID string, rows []extract.Row) {
		_ = app.PutDocument(domain.Document{
			ID: id.New(), Kind: kind, UploadedAt: time.Now(), AccountID: accountID,
			Status: domain.DocImported, Extracted: toDocumentRows(rows),
		})
	}

	// chooseCsvFile opens a file picker for .csv files and feeds the bytes
	// directly into the CSV import pipeline, skipping the paste step (C60).
	chooseCsvFile := ui.UseEvent(func() {
		pickFile(".csv,text/csv", func(_, _ string, data []byte) {
			if len(data) == 0 {
				msg.Set(uistate.T("documents.csvFileEmpty"))
				return
			}
			n, skipped, err := app.ImportTransactionsCSV(data, importAcct.Get())
			if err != nil {
				friendly := strings.TrimPrefix(err.Error(), "store: ")
				msg.Set(uistate.T("documents.csvError", friendly))
				return
			}
			if n > 0 {
				recordDocument(domain.DocCSV, "", nil)
			}
			summary := uistate.T("documents.importedCsv", plural(n, "transaction"))
			if len(skipped) > 0 {
				summary += " " + uistate.T("documents.importedCsvSkipped", plural(len(skipped), "row"))
			}
			msg.Set(summary)
			rev.Set(rev.Get() + 1)
		})
	})
	onAcct := ui.UseEvent(func(e ui.Event) { importAcct.Set(e.GetValue()) })
	onReceiptTotal := ui.UseEvent(func(v string) { receiptTotal.Set(v) })
	onReceiptMerchant := ui.UseEvent(func(v string) { receiptMerchant.Set(v) })

	importCSV := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(csvText.Get())
		if data == "" {
			msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		n, skipped, err := app.ImportTransactionsCSV([]byte(data), importAcct.Get())
		if err != nil {
			// Don't surface the internal "store:" package prefix to the user (C27).
			friendly := strings.TrimPrefix(err.Error(), "store: ")
			msg.Set(uistate.T("documents.csvError", friendly))
			return
		}
		if n > 0 {
			recordDocument(domain.DocCSV, "", nil)
		}
		summary := uistate.T("documents.importedCsv", plural(n, "transaction"))
		if len(skipped) > 0 {
			summary += " " + uistate.T("documents.importedCsvSkipped", plural(len(skipped), "row"))
		}
		msg.Set(summary)
		rev.Set(rev.Get() + 1)
	}))

	// parseStatement (C74) parses a pasted bank/credit-card statement in any common
	// delimited format — varying column orders/labels, signed-amount or separate
	// debit/credit columns — into draft rows. It guesses the column mapping
	// automatically (statement.MapColumns) and feeds the result into the same review
	// → dedupe → import pipeline the image/CSV paths use, so the user can edit, drop,
	// and commit rows (duplicates are skipped on import). Per-row parse failures are
	// reported, not fatal.
	//
	// When the auto-mapping cannot locate a date or amount column (C74 wizard path),
	// the wizard panel is shown so the user can assign columns manually.
	parseStatement := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(stmtText.Get())
		if data == "" {
			msg.Set(uistate.T("documents.stmtEmpty"))
			return
		}
		// Parse amounts at the import account's currency precision. ParseAny
		// auto-detects the format — OFX 1.x/2.x (bank/card statements) as well as the
		// delimited CSV/TSV/pipe path — so a pasted .ofx/.qfx statement now imports
		// through the same review pipeline (C74 Tier 2).
		dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
		stRows, err := statement.ParseAny(strings.NewReader(data), dec)
		if err != nil {
			// Tier 3: scanned/image-only or encrypted PDF — don't show the wizard,
			// show a helpful message directing the user to Extract with AI or image import.
			if errors.Is(err, pdftext.ErrNoText) || errors.Is(err, pdftext.ErrEncrypted) ||
				strings.Contains(err.Error(), "no text found") || strings.Contains(err.Error(), "PDF is encrypted") {
				msg.Set(uistate.T("documents.scannedPdf"))
				return
			}
			// If the error is about missing columns, offer the wizard instead of a
			// bare error message (C74 wizard path). Also try to populate the wizard
			// header from a raw Parse so the user can see column names.
			if strings.Contains(err.Error(), "could not find a date") ||
				strings.Contains(err.Error(), "could not find") {
				// Try a raw Parse to extract header/column info for the wizard.
				raw, rerr := statement.Parse(data, dec)
				// Even if rerr is non-nil, raw.Delimiter is set from the first line,
				// which is enough to populate the wizard column list.
				header, rawRecs := extractRawHeaderAndRows(data, raw.Delimiter)
				_ = rerr
				wizardHeader.Set(header)
				wizardRawRows.Set(rawRecs)
				// Pre-fill wizard fields from whatever was auto-detected (-1 = absent).
				wizardDate.Set(strconv.Itoa(raw.Columns.Date))
				wizardDescCol.Set(strconv.Itoa(raw.Columns.Description))
				wizardAmount.Set(strconv.Itoa(raw.Columns.Amount))
				wizardDebit.Set(strconv.Itoa(raw.Columns.Debit))
				wizardCredit.Set(strconv.Itoa(raw.Columns.Credit))
				wizardVisible.Set(true)
				msg.Set(uistate.T("documents.stmtError", "Columns couldn't be detected automatically — use the mapping wizard below."))
				return
			}
			msg.Set(uistate.T("documents.stmtError", strings.TrimPrefix(err.Error(), "statement: ")))
			return
		}
		rows := make([]extract.Row, 0, len(stRows))
		for _, r := range stRows {
			rows = append(rows, extract.Row{
				Date:        r.Date.Format("2006-01-02"),
				Description: r.Description,
				Amount:      money.FormatMinor(r.Amount, dec),
			})
		}
		if len(rows) == 0 {
			msg.Set(uistate.T("documents.stmtNoneFound"))
			return
		}
		// Successful parse — hide wizard, populate draft.
		wizardVisible.Set(false)
		draft.Set(rows)
		msg.Set(uistate.T("documents.stmtParsed", plural(len(rows), "row")))
		rev.Set(rev.Get() + 1)
	}))

	// applyWizard (C74) applies the user's manual column assignments via
	// importmap.Apply to produce draft rows from the raw statement records.
	applyWizard := ui.UseEvent(Prevent(func() {
		dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
		atoi := func(s string) int {
			v, _ := strconv.Atoi(s)
			return v
		}
		p := importmap.Profile{
			Name:        "manual",
			DateCol:     atoi(wizardDate.Get()),
			DescCol:     atoi(wizardDescCol.Get()),
			AmountCol:   atoi(wizardAmount.Get()),
			DebitCol:    atoi(wizardDebit.Get()),
			CreditCol:   atoi(wizardCredit.Get()),
			BalanceCol:  -1,
			CurrencyCol: -1,
			SkipRows:    0,
			Decimals:    dec,
		}
		rawRecs := wizardRawRows.Get()
		// Flatten [][][]string to [][]string (each outer entry is one record set).
		flat := make([][]string, 0)
		for _, group := range rawRecs {
			flat = append(flat, group...)
		}
		stRows := importmap.Apply(p, flat)
		if len(stRows) == 0 {
			msg.Set("No rows matched — check your column assignments.")
			return
		}
		rows := make([]extract.Row, 0, len(stRows))
		for _, r := range stRows {
			rows = append(rows, extract.Row{
				Date:        r.Date.Format("2006-01-02"),
				Description: r.Description,
				Amount:      money.FormatMinor(r.Amount, dec),
			})
		}
		wizardVisible.Set(false)
		draft.Set(rows)
		msg.Set(uistate.T("documents.stmtParsed", plural(len(rows), "row")))
		rev.Set(rev.Get() + 1)
	}))

	// saveProfile (C74) saves the current wizard mapping as a named profile.
	saveProfile := ui.UseEvent(Prevent(func() {
		name := strings.TrimSpace(profileName.Get())
		if name == "" {
			return
		}
		atoi := func(s string) int { v, _ := strconv.Atoi(s); return v }
		p := importmap.Profile{
			Name:        name,
			DateCol:     atoi(wizardDate.Get()),
			DescCol:     atoi(wizardDescCol.Get()),
			AmountCol:   atoi(wizardAmount.Get()),
			DebitCol:    atoi(wizardDebit.Get()),
			CreditCol:   atoi(wizardCredit.Get()),
			BalanceCol:  -1,
			CurrencyCol: -1,
			Decimals:    currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get())),
		}
		saved, err := app.SaveImportProfile(importmap.SavedProfile{Profile: p})
		if err != nil {
			msg.Set("Couldn't save: " + err.Error())
			return
		}
		profileName.Set("")
		msg.Set(uistate.T("documents.profileSaved", saved.Profile.Name))
		rev.Set(rev.Get() + 1)
	}))

	// createCadenceReminder (C74) creates a monthly to-do nudging the user to
	// import this account's statement next cycle, reusing app.PutTask like the
	// freshness nudge.
	createCadenceReminder := ui.UseEvent(func() {
		acctName := ""
		for _, a := range accounts {
			if a.ID == importAcct.Get() {
				acctName = a.Name
				break
			}
		}
		label := "Import bank statement"
		if acctName != "" {
			label = "Import " + acctName + " statement"
		}
		// Due one month from today.
		due := time.Now().AddDate(0, 1, 0)
		t := domain.Task{
			ID:         id.New(),
			Title:      label,
			Due:        due,
			Status:     domain.StatusOpen,
			Priority:   domain.PriorityMedium,
			Source:     domain.SourceNudge,
			Recurrence: domain.CadenceMonthly,
		}
		if err := app.PutTask(t); err != nil {
			msg.Set("Couldn't create reminder: " + err.Error())
			return
		}
		msg.Set(uistate.T("documents.cadenceCreated", due.Format("Jan 2, 2006")))
		rev.Set(rev.Get() + 1)
	})

	chooseImage := ui.UseEvent(func() {
		pickImageDataURL(func(u string) {
			imageURL.Set(u)
			aiErr.Set("")
			needsKey.Set(false)
			draft.Set([]extract.Row{})
		})
	})

	settings := app.Settings()
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := pr.BackendActive()
	aiModel := settings.OpenAIModel
	if aiModel == "" || aiModel == "gpt-4o-mini" {
		aiModel = "gpt-4o" // vision needs a vision-capable model
	}
	readAI := ui.UseEvent(func() {
		if settings.OpenAIKey == "" && !useBackendAI {
			needsKey.Set(true)
			return
		}
		if imageURL.Get() == "" {
			aiErr.Set(uistate.T("documents.chooseImageFirst"))
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		needsKey.Set(false)
		onResult := func(content string, _ ai.Usage) {
			aiLoading.Set(false)
			rows, err := extract.ParseRows(content)
			if err != nil {
				aiErr.Set(err.Error())
				return
			}
			if len(rows) == 0 {
				aiErr.Set(uistate.T("documents.noneFound"))
				return
			}
			draft.Set(rows)
		}
		onError := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackendAI {
			ai.SendProxyStructuredVisionChat(pr.ServerURL, pr.ServerToken, aiModel, visionSystemPrompt,
				"Extract every transaction you can read from this image.", imageURL.Get(), 0.1,
				"transactions", []byte(visionExtractionSchema), onResult, onError)
		} else {
			ai.SendStructuredVisionChat(settings.OpenAIKey, ai.DefaultBaseURL, aiModel, visionSystemPrompt,
				"Extract every transaction you can read from this image.", imageURL.Get(), 0.1,
				"transactions", []byte(visionExtractionSchema), onResult, onError)
		}
	})

	// extractWithAI (C74) sends the pasted statement text to the LLM for extraction,
	// using the same pipeline (extract.ParseRows → draft.Set) as the image import path.
	// Gated on OpenAI key / backend, like readAI.
	extractWithAI := ui.UseEvent(func() {
		if settings.OpenAIKey == "" && !useBackendAI {
			needsKey.Set(true)
			return
		}
		data := strings.TrimSpace(stmtText.Get())
		if data == "" {
			aiErr.Set(uistate.T("documents.stmtEmpty"))
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		needsKey.Set(false)
		model := settings.OpenAIModel
		if model == "" {
			model = "gpt-4o-mini"
		}
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: textExtractionSystemPrompt},
			{Role: ai.RoleUser, Content: data},
		}
		onResult := func(content string, _ ai.Usage) {
			aiLoading.Set(false)
			rows, err := extract.ParseRows(content)
			if err != nil {
				aiErr.Set(err.Error())
				return
			}
			if len(rows) == 0 {
				aiErr.Set(uistate.T("documents.noneFound"))
				return
			}
			draft.Set(rows)
			msg.Set(uistate.T("documents.stmtParsed", plural(len(rows), "row")))
		}
		onError := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, msgs, 0.1, onResult, onError)
		} else {
			ai.SendChat(settings.OpenAIKey, ai.DefaultBaseURL, model, msgs, 0.1, onResult, onError)
		}
	})

	// categorizeDraft (C74) runs two passes over the draft rows:
	// Pass 1 — deterministic rules (free, no network).
	// Pass 2 — AI for still-uncategorized rows (BYO-key or backend, opt-in).
	categorizeDraft := ui.UseEvent(func() {
		rows := draft.Get()
		if len(rows) == 0 {
			return
		}
		// Pass 1: deterministic rules (free, local)
		appRules := app.Rules()
		changed := false
		updated := make([]extract.Row, len(rows))
		copy(updated, rows)
		for i, r := range updated {
			if r.Category == "" {
				if cat := rules.Category(appRules, r.Description, r.Description); cat != "" {
					updated[i].Category = cat
					changed = true
				}
			}
		}
		if changed {
			draft.Set(updated)
		}
		// Pass 2: AI for still-uncategorized rows (BYO-key, opt-in)
		if settings.OpenAIKey == "" && !useBackendAI {
			if changed {
				msg.Set(uistate.T("documents.categorizing"))
			}
			return
		}
		// Gather uncategorized descriptions
		type pending struct {
			idx  int
			desc string
		}
		var pendings []pending
		for i, r := range updated {
			if r.Category == "" {
				pendings = append(pendings, pending{i, r.Description})
			}
		}
		if len(pendings) == 0 {
			msg.Set(uistate.T("documents.categorizing"))
			return
		}
		// Build category list
		cats := app.Categories()
		catNames := make([]string, 0, len(cats))
		for _, c := range cats {
			catNames = append(catNames, c.Name)
		}
		// Build prompt
		var sb strings.Builder
		sb.WriteString("Given these household spending categories: ")
		sb.WriteString(strings.Join(catNames, ", "))
		sb.WriteString("\n\nFor each transaction description below, reply with ONLY the best-fit category name from the list above (or empty string if none fits). One line per transaction, same order.\n\n")
		for j, p := range pendings {
			sb.WriteString(strconv.Itoa(j+1) + ". " + p.desc + "\n")
		}
		model := settings.OpenAIModel
		if model == "" {
			model = "gpt-4o-mini"
		}
		msgs := []ai.Message{
			{Role: ai.RoleSystem, Content: "You are a household finance assistant. Categorize transaction descriptions."},
			{Role: ai.RoleUser, Content: sb.String()},
		}
		aiLoading.Set(true)
		onResult := func(content string, _ ai.Usage) {
			aiLoading.Set(false)
			lines := strings.Split(strings.TrimSpace(content), "\n")
			cur := draft.Get()
			next := make([]extract.Row, len(cur))
			copy(next, cur)
			for j, p := range pendings {
				if j < len(lines) {
					cat := strings.TrimSpace(lines[j])
					// Strip leading "1. " numbering if the model echoes it back
					if idx := strings.Index(cat, ". "); idx >= 0 && idx < 3 {
						cat = cat[idx+2:]
					}
					if cat != "" {
						next[p.idx].Category = cat
					}
				}
			}
			draft.Set(next)
			msg.Set(uistate.T("documents.categorizing"))
		}
		onError := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, pr.ServerToken, model, msgs, 0.1, onResult, onError)
		} else {
			ai.SendChat(settings.OpenAIKey, ai.DefaultBaseURL, model, msgs, 0.1, onResult, onError)
		}
	})

	importDraft := ui.UseEvent(Prevent(func() {
		rows := draft.Get()
		result, err := app.ImportReviewedDocumentRows(domain.DocImage, importAcct.Get(), rows)
		if err != nil {
			aiErr.Set(uistate.T("documents.chooseAccount"))
			return
		}
		draft.Set([]extract.Row{})
		imageURL.Set("")
		aiErr.Set("")
		summary := uistate.T("documents.importedImage", plural(result.Imported, "transaction"))
		if result.Skipped > 0 {
			summary += uistate.T("documents.skipped", plural(result.Skipped, "duplicate"))
		}
		msg.Set(summary)
		rev.Set(rev.Get() + 1)
	}))

	// importReceipt imports the reviewed lines as ONE transaction split across
	// categories (receipt mode), instead of N standalone transactions.
	importReceipt := ui.UseEvent(Prevent(func() {
		rows := draft.Get()
		lines := make([]extract.ReceiptLine, 0, len(rows))
		for _, r := range rows {
			lines = append(lines, extract.ReceiptLine{Description: r.Description, Category: r.Category, Amount: absAmount(r.Amount)})
		}
		rec := extract.Receipt{Merchant: strings.TrimSpace(receiptMerchant.Get()), Total: absAmount(receiptTotal.Get()), Lines: lines}
		tx, err := app.ImportReceipt(rec, importAcct.Get(), time.Now())
		if err != nil {
			aiErr.Set(strings.TrimPrefix(err.Error(), "appstate: "))
			return
		}
		recordDocument(domain.DocImage, importAcct.Get(), rows)
		draft.Set([]extract.Row{})
		imageURL.Set("")
		receiptTotal.Set("")
		receiptMerchant.Set("")
		receiptMode.Set(false)
		aiErr.Set("")
		msg.Set("Imported the receipt as one transaction split across " + plural(len(tx.Splits), "category") + ".")
		rev.Set(rev.Get() + 1)
	}))

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
		next := append([]extract.Row{}, cur...)
		next[i] = r
		draft.Set(next)
	}

	// Draft review list: each row can be removed before importing.
	rows := draft.Get()
	// The review amounts format in the chosen import account's currency (falling
	// back to base), so they read in accounting style like the rest of the app.
	reviewCur := app.Settings().BaseCurrency
	if reviewCur == "" {
		reviewCur = "USD"
	}
	for _, a := range accounts {
		if a.ID == importAcct.Get() {
			reviewCur = a.Currency
			break
		}
	}

	// Receipt mode toggle: built here so OnChange has direct access to state setters.
	recBaseCur := app.Settings().BaseCurrency
	if recBaseCur == "" {
		recBaseCur = "USD"
	}
	recDec := currency.Decimals(recBaseCur)
	receiptToggle := uiw.ToggleRow(uiw.ToggleRowProps{
		Label: "Import as one receipt (split across categories)",
		On:    receiptMode.Get(),
		OnChange: func(on bool) {
			if on && strings.TrimSpace(receiptTotal.Get()) == "" {
				var sum int64
				for _, r := range rows {
					if m, err := money.ParseMinor(absAmount(r.Amount), recDec); err == nil {
						sum += m
					}
				}
				receiptTotal.Set(money.FormatMinor(sum, recDec))
			}
			receiptMode.Set(on)
		},
	})

	// G14 §4: build the set of existing transaction signatures for the chosen
	// account so the review list can badge already-imported rows before import.
	seenSigs := map[string]bool{}
	if importAcct.Get() != "" {
		dec := currency.Decimals(reviewCur)
		for _, t := range app.Transactions() {
			if t.AccountID != importAcct.Get() {
				continue
			}
			sig := extract.Row{
				Date:   t.Date.Format("2006-01-02"),
				Amount: money.FormatMinor(t.Amount.Amount, dec),
			}.Signature()
			seenSigs[sig] = true
		}
	}

	// G14 §1 / §7: "Start over" clears the draft when persisted sample rows appear
	// on first load. Declared here so it can close over draft.
	clearDraft := ui.UseEvent(func() { draft.Set([]extract.Row{}) })

	// Import history: sort newest first before rendering.
	deleteDoc := func(docID string) {
		_ = app.DeleteDocument(docID)
		rev.Set(rev.Get() + 1)
	}
	docs := app.Documents()
	sort.Slice(docs, func(i, j int) bool { return docs[i].UploadedAt.After(docs[j].UploadedAt) })

	return Div(
		ImageImportCard(imageImportCardProps{
			ImageURL:  imageURL.Get(),
			AILoading: aiLoading.Get(),
			AIErr:     aiErr.Get(),
			NeedsKey:  needsKey.Get(),
			OnChoose:  chooseImage,
			OnReadAI:  readAI,
			Nav:       nav,
		}),
		uiw.EntityListSection(uiw.EntityListSectionProps{
			Title: uistate.T("documents.stmtTitle"),
			Body: Fragment(
				P(css.Class("muted"), uistate.T("documents.stmtDesc")),
				Form(OnSubmit(parseStatement),
					// G14 §5: parse actions above the textarea so they are always
					// visible at 768px — the tall textarea no longer pushes them off-screen.
					Div(Style(map[string]string{"margin-bottom": "0.5rem"}),
						Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.stmtParse")),
						Button(css.Class("btn"), Type("button"), Attr("data-testid", "extract-ai-btn"),
							OnClick(extractWithAI), uistate.T("documents.extractAI")),
					),
					// G14 §2: collapsed to 3 rows by default (was 8) to avoid pushing action
					// buttons off-screen on short viewports. Expands naturally as the user types.
					Textarea(css.Class("field field-wide"), Attr("rows", "3"),
						Placeholder("Posting Date,Description,Debit,Credit\n06/01/2026,SALARY ACH,,4200.00\n06/02/2026,WHOLE FOODS,86.40,"),
						OnInput(onStmt),
					),
				),
				// C74 — per-bank import cadence reminder: creates a monthly to-do so the
				// user is nudged to import next cycle. Off by default; single click.
				Div(css.Class(tw.Mt2, tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
					P(css.Class("muted"), uistate.T("documents.cadenceDesc")),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "cadence-reminder-btn"),
						OnClick(createCadenceReminder), uistate.T("documents.cadenceBtn")),
				),
			),
		}),
		// C74 — Map columns wizard: shown when auto-detect fails or user requests it.
		// Exactly 5 fixed selects (Date / Desc / Amount / Debit / Credit) — never in
		// a variable-length loop, so UseEvent hooks stay at stable render positions.
		If(wizardVisible.Get(),
			wizardCard(
				wizardHeader.Get(),
				wizardDate.Get(), wizardDescCol.Get(), wizardAmount.Get(),
				wizardDebit.Get(), wizardCredit.Get(),
				onWizardDate, onWizardDesc, onWizardAmount, onWizardDebit, onWizardCredit,
				applyWizard,
				profileName.Get(), onProfileName, saveProfile,
			),
		),
		// C74 — Saved profile picker: shown/hidden via the "Saved mappings" toggle.
		// Each profile row is its own component (owns its Apply/Delete handlers).
		If(wizardVisible.Get() || showProfiles.Get(),
			savedProfilesCard(app, accounts, importAcct.Get(),
				showProfiles.Get(),
				func() { showProfiles.Set(!showProfiles.Get()) },
				func(sp importmap.SavedProfile) {
					// Apply a saved profile to the current raw rows.
					rawRecs := wizardRawRows.Get()
					flat := make([][]string, 0)
					for _, g := range rawRecs {
						flat = append(flat, g...)
					}
					if len(flat) == 0 {
						msg.Set("Paste a statement above, then click Parse before applying a profile.")
						return
					}
					dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
					sp.Profile.Decimals = dec
					stRows := importmap.Apply(sp.Profile, flat)
					if len(stRows) == 0 {
						msg.Set("No rows matched with that profile — check column assignments.")
						return
					}
					rows2 := make([]extract.Row, 0, len(stRows))
					for _, r := range stRows {
						rows2 = append(rows2, extract.Row{
							Date:        r.Date.Format("2006-01-02"),
							Description: r.Description,
							Amount:      money.FormatMinor(r.Amount, dec),
						})
					}
					wizardVisible.Set(false)
					draft.Set(rows2)
					msg.Set(uistate.T("documents.stmtParsed", plural(len(rows2), "row")))
					rev.Set(rev.Get() + 1)
				},
				func(id string) {
					_ = app.DeleteImportProfile(id)
					rev.Set(rev.Get() + 1)
				},
			),
		),
		// C74 — Suggest categories button: shown when draft is non-empty.
		// Pass 1 (deterministic rules) runs free; Pass 2 (AI) only fires with a key.
		If(len(rows) > 0,
			Div(css.Class(tw.Mt2, tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "suggest-categories-btn"),
					OnClick(categorizeDraft), uistate.T("documents.suggestCategories")),
				If(aiLoading.Get(), Span(css.Class("muted"), uistate.T("documents.categorizing"))),
			),
		),
		// Review results sit below the import inputs that produce them (G14 §1): a
		// parsed statement / scanned receipt's draft rows no longer pop in *above* the
		// card the user just acted in.
		DraftReviewList(draftReviewListProps{
			Rows:              rows,
			Accounts:          accounts,
			Categories:        app.Categories(),
			ReviewCur:         reviewCur,
			ImportAcctID:      importAcct.Get(),
			ReceiptMode:       receiptMode.Get(),
			ReceiptTotal:      receiptTotal.Get(),
			ReceiptMerchant:   receiptMerchant.Get(),
			RecBaseCur:        recBaseCur,
			SeenSigs:          seenSigs,
			ClearDraft:        clearDraft,
			Toggle:            receiptToggle,
			OnAcctChange:      onAcct,
			OnReceiptTotal:    onReceiptTotal,
			OnReceiptMerchant: onReceiptMerchant,
			OnImportDraft:     importDraft,
			OnImportReceipt:   importReceipt,
			OnRemoveDraft:     removeDraft,
			OnUpdateDraft:     updateDraft,
		}),
		SpendSummaryCard(spendSummaryCardProps{
			Rows:         rows,
			Accounts:     accounts,
			ImportAcctID: importAcct.Get(),
			BaseCurrency: app.Settings().BaseCurrency,
		}),
		CsvImportCard(csvImportCardProps{
			Accounts:     accounts,
			ImportAcctID: importAcct.Get(),
			Msg:          msg.Get(),
			OnChooseFile: chooseCsvFile,
			OnAcctChange: onAcct,
			OnCsvInput:   onCsv,
			OnImportCSV:  importCSV,
		}),
		ImportHistoryList(importHistoryListProps{
			Docs:     docs,
			Accounts: accounts,
			OnDelete: deleteDoc,
		}),
	)
}

type draftRowProps struct {
	Index       int
	Row         extract.Row
	Currency    string            // for accounting-formatting the review amount (C27)
	Categories  []domain.Category // existing categories, so editing picks a real one (C60)
	IsDuplicate bool              // G14 §4: row matches an existing transaction — show badge
	OnRemove    func(int)
	OnUpdate    func(int, extract.Row)
}

// draftCategoryOptions builds the draft-row category picker: a "no category" entry,
// every existing category, and — when the AI's extracted category matches none of
// them — the extracted value itself as a final option, so editing constrains to
// real categories without silently dropping the AI's suggestion (C60).
func draftCategoryOptions(cats []domain.Category, current string) []ui.Node {
	opts := []ui.Node{Option(Value(""), SelectedIf(current == ""), uistate.T("transactions.noCategory"))}
	found := false
	for _, c := range cats {
		opts = append(opts, Option(Value(c.Name), SelectedIf(current == c.Name), c.Name))
		if c.Name == current {
			found = true
		}
	}
	if current != "" && !found {
		opts = append(opts, Option(Value(current), SelectedIf(true), current))
	}
	return opts
}

// DraftRow renders one extracted transaction in the review list. It can be edited
// inline (date, description, amount, category) or removed. All hooks are declared
// unconditionally so the edit toggle never reorders them.
func DraftRow(props draftRowProps) ui.Node {
	r := props.Row
	rm := ui.UseEvent(Prevent(func() { props.OnRemove(props.Index) }))
	editing := ui.UseState(false)
	dateS := ui.UseState(r.Date)
	descS := ui.UseState(r.Description)
	amtS := ui.UseState(r.Amount)
	catS := ui.UseState(r.Category)
	onDate := ui.UseEvent(func(v string) { dateS.Set(v) })
	onDesc := ui.UseEvent(func(v string) { descS.Set(v) })
	onAmt := ui.UseEvent(func(v string) { amtS.Set(v) })
	onCat := ui.UseEvent(func(e ui.Event) { catS.Set(e.GetValue()) })
	startEdit := ui.UseEvent(Prevent(func() {
		dateS.Set(r.Date)
		descS.Set(r.Description)
		amtS.Set(r.Amount)
		catS.Set(r.Category)
		editing.Set(true)
	}))
	cancelEdit := ui.UseEvent(Prevent(func() { editing.Set(false) }))
	saveEdit := ui.UseEvent(Prevent(func() {
		props.OnUpdate(props.Index, extract.Row{
			Date: dateS.Get(), Description: descS.Get(), Amount: amtS.Get(), Category: catS.Get(),
		})
		editing.Set(false)
	}))

	// Land the cursor in the first field when the inline editor opens (§6.7).
	// Draft rows have no stable id, so key the element by its list index.
	draftFieldID := "draft-edit-" + strconv.Itoa(props.Index)
	editKey := "closed"
	if editing.Get() {
		editKey = "open"
	}
	ui.UseEffect(func() func() {
		if editing.Get() {
			focusByID(draftFieldID)
		}
		return nil
	}, editKey)

	if editing.Get() {
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveEdit),
				Input(css.Class("field"), Attr("id", draftFieldID), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("documents.descPlaceholder")), Value(descS.Get()), OnInput(onDesc)),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("documents.amountPlaceholder")), Value(amtS.Get()), OnInput(onAmt)),
				// Category is a select of existing categories (plus the AI's extracted
				// value when it doesn't match one) so editing can't introduce an
				// orphan/typo category on import (C60).
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.categoryPlaceholder")), OnChange(onCat), draftCategoryOptions(props.Categories, catS.Get())),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
			),
		)
	}

	meta := r.Date
	if r.Category != "" {
		meta += " · " + r.Category
	}
	// Show the amount in accounting style (parentheses for negatives) like the
	// rest of the app (C27/C2); fall back to the raw string if it won't parse
	// (e.g. while the AI value is still being corrected).
	amtText := r.Amount
	if props.Currency != "" {
		if minor, err := money.ParseMinor(strings.TrimSpace(r.Amount), currency.Decimals(props.Currency)); err == nil {
			amtText = fmtMoney(money.New(minor, props.Currency))
		}
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), textutil.FirstNonEmpty(r.Description, uistate.T("documents.noDescription"))),
			Span(css.Class("row-meta"), meta),
		),
		// G14 §4: amber "Already imported" badge for rows that match an existing
		// transaction by date+amount — will be skipped on import.
		If(props.IsDuplicate,
			Span(css.Class("badge badge-warn"), "Already imported"),
		),
		Span(css.Class("amount fig"), amtText),
		// G14 §4: icon-only edit button (matches the × remove affordance; label
		// preserved via aria-label for screen readers).
		Button(css.Class("btn-del"), Type("button"),
			Attr("aria-label", uistate.T("documents.editRow")),
			Title(uistate.T("documents.editRow")),
			OnClick(startEdit),
			uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("documents.removeRow")), Title(uistate.T("documents.removeRow")), OnClick(rm), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

type docHistoryRowProps struct {
	Doc         domain.Document
	AccountName string
	OnDelete    func(string)
}

// DocHistoryRow renders one recorded import in the history list, with a remove
// button. It owns its own click handler (per the no-hooks-in-loops rule).
func DocHistoryRow(props docHistoryRowProps) ui.Node {
	d := props.Doc
	del := ui.UseEvent(Prevent(func() { props.OnDelete(d.ID) }))
	meta := docKindLabel(d.Kind) + " · " + d.UploadedAt.Format("Jan 2, 2006") + " · " + docStatusLabel(d.Status)
	if len(d.Extracted) > 0 {
		meta += " · " + plural(len(d.Extracted), "row")
	}
	if props.AccountName != "" {
		meta += " · " + props.AccountName
	}
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), textutil.FirstNonEmpty(d.Filename, docKindLabel(d.Kind))),
			Span(css.Class("row-meta"), meta),
		),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("documents.deleteHistTitle")), Title(uistate.T("documents.deleteHistTitle")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// docKindLabel localizes a document kind.
func docKindLabel(k domain.DocumentKind) string {
	if k == domain.DocImage {
		return uistate.T("documents.kindImage")
	}
	return uistate.T("documents.kindCsv")
}

// docStatusLabel localizes a document status.
func docStatusLabel(s domain.DocumentStatus) string {
	switch s {
	case domain.DocPending:
		return uistate.T("documents.statusPending")
	case domain.DocExtracted:
		return uistate.T("documents.statusExtracted")
	case domain.DocFailed:
		return uistate.T("documents.statusFailed")
	default:
		return uistate.T("documents.statusImported")
	}
}

// csvAcctSelect renders the account picker used by the CSV import form. It is
// extracted as a helper so the select's option list can be built outside the
// Documents render function without calling On*/UseX (framework gotcha: no hook
// calls inside variable-length loops).
func csvAcctSelect(accounts []domain.Account, selected string, onChange ui.Handler) ui.Node {
	opts := make([]ui.Node, 0, len(accounts))
	for _, a := range accounts {
		opts = append(opts, Option(Value(a.ID), SelectedIf(selected == a.ID), a.Name))
	}
	args := make([]any, 0, 3+len(opts))
	args = append(args, css.Class("field"), Attr("aria-label", uistate.T("documents.csvAccount")), OnChange(onChange))
	for _, o := range opts {
		args = append(args, o)
	}
	return Select(args...)
}

// reviewCurrencyFor returns the currency code to format/parse review amounts in:
// the chosen import account's currency, falling back to the base currency, then
// USD. Shared by the statement parser and the review list (C74).
func reviewCurrencyFor(app *appstate.App, accounts []domain.Account, acctID string) string {
	cur := app.Settings().BaseCurrency
	if cur == "" {
		cur = "USD"
	}
	if acc, ok := domain.AccountByID(accounts, acctID); ok && acc.Currency != "" {
		cur = acc.Currency
	}
	return cur
}

// toDocumentRows maps reviewed extract rows to the persisted document-row shape.
func toDocumentRows(rows []extract.Row) []domain.DocumentRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]domain.DocumentRow, len(rows))
	for i, r := range rows {
		out[i] = domain.DocumentRow{Date: r.Date, Description: r.Description, Amount: r.Amount, Category: r.Category}
	}
	return out
}

// absAmount returns the magnitude of an extracted amount string (dropping a
// currency symbol and any sign), so receipt lines and total are positive figures
// the receipt math can sum; ImportReceipt re-applies the expense sign.
func absAmount(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "$", "")
	s = strings.TrimPrefix(s, "-")
	return strings.TrimSpace(s)
}

// pickImageDataURL opens a file picker for images and calls onData with the
// chosen file as a base64 data: URL. The data never leaves the device except to
// OpenAI when the user clicks Read.
func pickImageDataURL(onData func(string)) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", "image/*")
	// On a phone (the primary device for "snap a receipt") this asks the browser to
	// open the rear camera directly rather than the file browser. Desktop browsers
	// ignore it and still show a file picker.
	input.Set("capture", "environment")

	var onChange, onLoad js.Func
	onLoad = js.FuncOf(func(this js.Value, _ []js.Value) any {
		onData(this.Get("result").String())
		onLoad.Release()
		return nil
	})
	onChange = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		if files.Length() > 0 {
			reader := js.Global().Get("FileReader").New()
			reader.Set("onload", onLoad)
			reader.Call("readAsDataURL", files.Index(0))
		} else {
			onLoad.Release()
		}
		onChange.Release()
		return nil
	})
	input.Set("onchange", onChange)
	input.Call("click")
}

// ---------------------------------------------------------------------------
// C74 — import-map wizard helpers
// ---------------------------------------------------------------------------

// extractRawHeaderAndRows splits raw delimited text into a header slice and a
// [][][]string of records (each record is a []string of fields). The outer
// slice wraps each record in its own single-element slice so it can be stored
// in a ui.UseState([][][]string) and later flattened to [][]string for Apply.
func extractRawHeaderAndRows(text string, delim rune) (header []string, rows [][][]string) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return nil, nil
	}
	sep := string(delim)
	header = splitLine(lines[0], sep)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		rows = append(rows, [][]string{splitLine(line, sep)})
	}
	return header, rows
}

// splitLine splits a single CSV/TSV/pipe line by sep without full csv.Reader
// overhead. Used only to populate the wizard header for display purposes.
func splitLine(line, sep string) []string {
	line = strings.TrimRight(line, "\r")
	parts := strings.Split(line, sep)
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.Trim(strings.TrimSpace(p), `"`)
	}
	return out
}

// wizardColumnOptions builds a <select> option list for picking a column from
// the parsed header. The first option is "— not present —" (value "-1"); each
// subsequent option is the column name with its 0-based index as the value.
func wizardColumnOptions(header []string, selected string) []ui.Node {
	opts := []ui.Node{Option(Value("-1"), SelectedIf(selected == "-1" || selected == ""), uistate.T("documents.wizardNone"))}
	for i, h := range header {
		v := strconv.Itoa(i)
		label := h
		if label == "" {
			label = "Column " + v
		}
		opts = append(opts, Option(Value(v), SelectedIf(selected == v), label+" (col "+v+")"))
	}
	return opts
}

// wizardCard renders the column-mapping wizard panel (C74). It shows one
// <select> per required or optional field — always a fixed set of five, never
// inside a variable-length loop — so the UseEvent hooks that drive them stay
// at stable render positions (the hooks live in Documents(), not here).
//
// The "Extract with AI" alternative is noted for discoverability; the vision
// path already exists in the image card above.
func wizardCard(
	header []string,
	selDate, selDesc, selAmount, selDebit, selCredit string,
	onDate, onDesc, onAmount, onDebit, onCredit ui.Handler,
	onApply ui.Handler,
	profileNameVal string,
	onProfileName ui.Handler,
	onSaveProfile ui.Handler,
) ui.Node {
	colOpts := func(sel string) []ui.Node { return wizardColumnOptions(header, sel) }

	return uiw.Card(uiw.CardProps{
		TestID: "import-wizard",
		Header: H2(css.Class("card-title"), uistate.T("documents.wizardTitle")),
		Body: Fragment(
			P(css.Class("muted"), uistate.T("documents.wizardDesc")),
			P(css.Class("muted"), uistate.T("documents.wizardOrAI")),
			Form(OnSubmit(onApply),
				Div(css.Class("form-grid"),
					Div(
						Label(For("wiz-date"), uistate.T("documents.wizardDate")),
						Select(css.Class("field"), Attr("id", "wiz-date"),
							Attr("aria-label", uistate.T("documents.wizardDate")),
							OnChange(onDate), colOpts(selDate)),
					),
					Div(
						Label(For("wiz-desc"), uistate.T("documents.wizardDesc2")),
						Select(css.Class("field"), Attr("id", "wiz-desc"),
							Attr("aria-label", uistate.T("documents.wizardDesc2")),
							OnChange(onDesc), colOpts(selDesc)),
					),
					Div(
						Label(For("wiz-amount"), uistate.T("documents.wizardAmount")),
						Select(css.Class("field"), Attr("id", "wiz-amount"),
							Attr("aria-label", uistate.T("documents.wizardAmount")),
							OnChange(onAmount), colOpts(selAmount)),
					),
					Div(
						Label(For("wiz-debit"), uistate.T("documents.wizardDebit")),
						Select(css.Class("field"), Attr("id", "wiz-debit"),
							Attr("aria-label", uistate.T("documents.wizardDebit")),
							OnChange(onDebit), colOpts(selDebit)),
					),
					Div(
						Label(For("wiz-credit"), uistate.T("documents.wizardCredit")),
						Select(css.Class("field"), Attr("id", "wiz-credit"),
							Attr("aria-label", uistate.T("documents.wizardCredit")),
							OnChange(onCredit), colOpts(selCredit)),
					),
				),
				Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter, tw.Mt2),
					Button(css.Class("btn btn-primary"), Type("submit"),
						Attr("data-testid", "wizard-apply-btn"),
						uistate.T("documents.wizardApply")),
				),
			),
			Div(css.Class(tw.Mt2),
				H3(css.Class("card-title"), uistate.T("documents.profileSaveTitle")),
				Form(css.Class("form-grid"), OnSubmit(onSaveProfile),
					Input(css.Class("field"), Type("text"),
						Attr("aria-label", uistate.T("documents.profileSaveTitle")),
						Placeholder(uistate.T("documents.profileNamePlaceholder")),
						Value(profileNameVal),
						OnInput(onProfileName),
					),
					Button(css.Class("btn"), Type("submit"), uistate.T("documents.profileSave")),
				),
			),
		),
	})
}

// savedProfilesCard renders the saved-profile picker. It is shown alongside
// the wizard or via the "Saved mappings" toggle. Each profile row is its own
// component so Apply/Delete hooks stay at stable render positions.
func savedProfilesCard(
	app *appstate.App,
	_ []domain.Account,
	_ string,
	shown bool,
	onToggle func(),
	onApply func(importmap.SavedProfile),
	onDelete func(string),
) ui.Node {
	profiles := app.ImportProfiles()
	return uiw.Card(uiw.CardProps{
		Header: Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
			H2(css.Class("card-title"), uistate.T("documents.profileLoadTitle")),
			Button(css.Class("btn btn-sm"), Type("button"), OnClick(func() { onToggle() }),
				IfElse(shown, Text("Hide"), Text("Show"))),
		),
		Body: If(shown,
			IfElse(len(profiles) == 0,
				P(css.Class("empty"), uistate.T("documents.profileNone")),
				Div(css.Class("rows"), MapKeyed(profiles,
					func(sp importmap.SavedProfile) any { return sp.ID },
					func(sp importmap.SavedProfile) ui.Node {
						return ui.CreateElement(ProfileRow, profileRowProps{
							Profile:  sp,
							OnApply:  onApply,
							OnDelete: onDelete,
						})
					},
				)),
			),
		),
	})
}

// profileRowProps configures a ProfileRow component.
type profileRowProps struct {
	Profile  importmap.SavedProfile
	OnApply  func(importmap.SavedProfile)
	OnDelete func(string)
}

// ProfileRow renders one saved import profile in the profile picker list.
// It owns its Apply and Delete handlers so hooks stay at a stable render
// position (per the no-hooks-in-loops rule).
func ProfileRow(props profileRowProps) ui.Node {
	sp := props.Profile
	apply := ui.UseEvent(func() { props.OnApply(sp) })
	del := ui.UseEvent(func() { props.OnDelete(sp.ID) })
	meta := "Date col " + strconv.Itoa(sp.Profile.DateCol) +
		" · Desc col " + strconv.Itoa(sp.Profile.DescCol) +
		" · Amount col " + strconv.Itoa(sp.Profile.AmountCol)
	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), sp.Profile.Name),
			Span(css.Class("row-meta"), meta),
		),
		Button(css.Class("btn btn-sm"), Type("button"), OnClick(apply),
			uistate.T("documents.profileLoad")),
		Button(css.Class("btn-del"), Type("button"),
			Attr("aria-label", uistate.T("documents.profileDelete")),
			Title(uistate.T("documents.profileDelete")),
			OnClick(del),
			uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}
