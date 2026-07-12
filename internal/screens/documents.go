// SPDX-License-Identifier: MIT

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
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/router"
	"github.com/monstercameron/GoWebComponents/v4/state"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

const visionSystemPrompt = "You extract transactions from receipt and bank-statement images. " +
	"Return each transaction with: date (YYYY-MM-DD), description, amount (negative for money out / " +
	"expenses, positive for money in, as a string), and category."

// csvSkipDetail turns the per-row CSV import errors into a short, plain-English
// "which/why" clause (C16) — e.g. "line 3: bad amount; line 7: bad date (+2 more)".
// Returns "" when nothing was skipped. Capped at a few rows so the toast stays short.
func csvSkipDetail(rows []store.CSVRowError) string {
	if len(rows) == 0 {
		return ""
	}
	const maxShown = 3
	parts := make([]string, 0, maxShown)
	for i, r := range rows {
		if i >= maxShown {
			break
		}
		reason := strings.TrimSpace(r.Reason)
		if reason == "" {
			reason = uistate.T("documents.skipReasonGeneric")
		}
		parts = append(parts, uistate.T("documents.skipLine", r.Line, reason))
	}
	detail := strings.Join(parts, "; ")
	if len(rows) > maxShown {
		detail += " " + uistate.T("documents.skipMore", len(rows)-maxShown)
	}
	return detail
}

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

// documentsPanelProps is the props bag for DocumentsPanel. Currently empty —
// the panel reads all its data from appstate.Default — but typed so it can be
// embedded via ui.CreateElement and have its hook state isolated from parents.
type documentsPanelProps struct{}

// ImportPanelBody is the exported handle for mounting the import panel inside the
// shell-root import flip modal (ImportPanelHost). DocumentsPanel's props type is
// unexported, so the app package embeds this wrapper instead. The empty struct keeps
// it CreateElement-compatible, matching StatementImportBody.
func ImportPanelBody(_ struct{}) ui.Node {
	return ui.CreateElement(DocumentsPanel, documentsPanelProps{})
}

// DocumentsPanel is the registered component that owns the full import UI:
// CSV paste, statement paste/wizard, receipt vision, draft review, and import
// history. Extracted from Documents() so it can be embedded on /transactions
// without duplicating logic (FEATURE_MAP §5.3 / §5.7b).
func DocumentsPanel(props documentsPanelProps) ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}

	nav := router.UseNavigate()
	rev := state.UseAtom("rev:documents", 0)
	csvText := ui.UseState("")
	stmtText := ui.UseState("")
	msg := ui.UseState("")
	cadenceMsg := ui.UseState("") // C18: inline confirmation shown next to the reminder button

	// C88: pre-import duplicate warning state.
	// csvDupWarn holds the human-readable warning string (non-empty = warning visible).
	// pendingCSV holds the raw CSV bytes awaiting user confirmation; cleared on import or cancel.
	csvDupWarn := ui.UseState("")
	pendingCSV := ui.UseState([]byte(nil))

	accounts := app.Accounts()
	defaultAcc := ""
	if len(accounts) > 0 {
		defaultAcc = accounts[0].ID
	}
	imageURL := ui.UseState("")
	// C98: persist the chosen image URL across navigation (e.g. when the user
	// goes to Settings to add an OpenAI key and returns). The atom survives
	// component teardown; the UseEffect below restores it on mount.
	imageDraftAtom := uistate.UseImageDraft()
	ui.UseEffect(func() func() {
		if imageURL.Get() == "" {
			if saved := imageDraftAtom.Get(); saved != "" {
				imageURL.Set(saved)
			}
		}
		return nil
	}, "mount")
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
	recordDocument := func(kind domain.DocumentKind, accountID string, rows []extract.Row, rowCount int) {
		_ = app.PutDocument(domain.Document{
			ID: id.New(), Kind: kind, UploadedAt: time.Now(), AccountID: accountID,
			Status: domain.DocImported, Extracted: toDocumentRows(rows), RowCount: rowCount,
		})
	}

	// commitCSVImport is the shared write path used by both the paste and file-picker
	// flows after any duplicate warning has been acknowledged. It calls
	// ImportTransactionsCSV, resets the dup-warning state, and posts the summary.
	commitCSVImport := func(data []byte) {
		n, skipped, err := app.ImportTransactionsCSV(data, importAcct.Get())
		csvDupWarn.Set("")
		pendingCSV.Set(nil)
		if err != nil {
			friendly := strings.TrimPrefix(err.Error(), "store: ")
			msg.Set(uistate.T("documents.csvError", friendly))
			return
		}
		if n > 0 {
			recordDocument(domain.DocCSV, importAcct.Get(), nil, n)
		}
		summary := csvImportSummary(accounts, importAcct.Get(), n)
		if len(skipped) > 0 {
			summary += " " + uistate.T("documents.importedCsvSkipped", plural(len(skipped), "row"))
			if d := csvSkipDetail(skipped); d != "" {
				summary += " " + d
			}
		}
		msg.Set(summary)
		rev.Set(rev.Get() + 1)
	}

	// confirmCSV is the "Import anyway" handler: commits the pending CSV bytes
	// that were held after the user saw the duplicate warning (C88).
	confirmCSV := ui.UseEvent(func() {
		data := pendingCSV.Get()
		if len(data) == 0 {
			return
		}
		commitCSVImport(data)
	})

	// previewCSVDuplicates checks incoming CSV bytes for duplicate rows. If any
	// are found it stores the bytes and sets the warning text, returning true so
	// the caller knows to pause before importing. If no dupes, returns false and
	// the caller should proceed directly.
	previewCSVDuplicates := func(data []byte) (hasDupes bool) {
		total, dupes, err := app.PreviewCSVImport(data, importAcct.Get())
		if err != nil || dupes == 0 {
			return false
		}
		pendingCSV.Set(data)
		if dupes == total {
			csvDupWarn.Set(uistate.T("documents.dupWarnAllDups", dupes))
		} else {
			csvDupWarn.Set(uistate.T("documents.dupWarnBanner", dupes, total))
		}
		return true
	}

	// chooseCsvFile opens a file picker for .csv files and feeds the bytes
	// directly into the CSV import pipeline, skipping the paste step (C60).
	// C88: if duplicates are detected the bytes are staged and a warning is shown;
	// the user confirms via "Import anyway" before the write happens.
	chooseCsvFile := ui.UseEvent(func() {
		pickFile(".csv,text/csv", func(_, _ string, data []byte) {
			if len(data) == 0 {
				msg.Set(uistate.T("documents.csvFileEmpty"))
				return
			}
			if previewCSVDuplicates(data) {
				// Warning is now visible; import is held until user confirms.
				return
			}
			commitCSVImport(data)
		})
	})
	onAcct := ui.UseEvent(func(e ui.Event) { importAcct.Set(e.GetValue()) })
	onReceiptTotal := ui.UseEvent(func(v string) { receiptTotal.Set(v) })
	onReceiptMerchant := ui.UseEvent(func(v string) { receiptMerchant.Set(v) })

	// importCSV is the paste-path import handler (C88: two-step with dup warning).
	// First call: runs a preview — if duplicates are found, stores the bytes and
	// surfaces the warning; the user then clicks "Import anyway" (confirmCSV).
	// If no duplicates, proceeds directly to commitCSVImport.
	importCSV := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(csvText.Get())
		if data == "" {
			msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		raw := []byte(data)
		if previewCSVDuplicates(raw) {
			// Duplicate warning is now shown; import waits for user confirmation.
			return
		}
		commitCSVImport(raw)
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
				// Pre-fill wizard fields from whatever was auto-detected, with a
				// name-based fallback (C15): if auto-detect found -1 for a field,
				// scan header names for case-insensitive keyword matches so that
				// common header names like "Date", "Amount", "Description" are
				// pre-selected rather than left as "— not present —".
				wizardDate.Set(guessWizardField(header, []string{"date", "posted", "trans"}, raw.Columns.Date))
				wizardDescCol.Set(guessWizardField(header, []string{"desc", "memo", "narr", "detail", "note", "ref"}, raw.Columns.Description))
				wizardAmount.Set(guessWizardField(header, []string{"amount", "value", "amt", "sum"}, raw.Columns.Amount))
				wizardDebit.Set(guessWizardField(header, []string{"debit", "withdrawal", "dr"}, raw.Columns.Debit))
				wizardCredit.Set(guessWizardField(header, []string{"credit", "deposit", "cr"}, raw.Columns.Credit))
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
			cadenceMsg.Set("Couldn't create reminder: " + err.Error())
			return
		}
		// C18: confirm right next to the button (not in the far-away top message).
		cadenceMsg.Set(uistate.T("documents.cadenceCreated", due.Format("Jan 2, 2006")))
		rev.Set(rev.Get() + 1)
	})

	chooseImage := ui.UseEvent(func() {
		pickImageDataURL(
			func(u string) {
				imageURL.Set(u)
				imageDraftAtom.Set(u) // C98: persist across navigation
				aiErr.Set("")
				needsKey.Set(false)
				draft.Set([]extract.Row{})
			},
			func(e string) { aiErr.Set(e) },
		)
	})

	settings := app.Settings()
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := pr.BackendActive()
	aiModel := settings.OpenAIModel
	if aiModel == "" || aiModel == "gpt-5.4-mini" {
		aiModel = "gpt-5.5" // vision/receipt extraction: use the flagship vision model
	}
	readAI := ui.UseEvent(func() {
		if imageURL.Get() == "" {
			aiErr.Set(uistate.T("documents.chooseImageFirst"))
			return
		}
		if settings.OpenAIKey == "" && !useBackendAI {
			needsKey.Set(true)
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
		// Temperature 0 → omitted (omitempty) → the model's default. gpt-5.x rejects any
		// non-default temperature ("Only the default (1) value is supported").
		if useBackendAI {
			ai.SendProxyStructuredVisionChat(pr.ServerURL, pr.ServerToken, aiModel, visionSystemPrompt,
				"Extract every transaction you can read from this image.", imageURL.Get(), 0,
				"transactions", []byte(visionExtractionSchema), onResult, onError)
		} else {
			ai.SendStructuredVisionChat(settings.OpenAIKey, ai.DefaultBaseURL, aiModel, visionSystemPrompt,
				"Extract every transaction you can read from this image.", imageURL.Get(), 0,
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
			model = "gpt-5.4-mini"
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
			model = "gpt-5.4-mini"
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
		imageDraftAtom.Set("") // C98: clear persisted image after successful import
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
		recordDocument(domain.DocImage, importAcct.Get(), rows, len(rows))
		draft.Set([]extract.Row{})
		imageURL.Set("")
		imageDraftAtom.Set("") // C98: clear persisted image after successful import
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
		// Removing an import record is permanent — confirm first (v1.0).
		uistate.ConfirmModal(uistate.T("documents.deleteConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			_ = app.DeleteDocument(docID)
			rev.Set(rev.Get() + 1)
		})
	}
	docs := app.Documents()
	sort.Slice(docs, func(i, j int) bool { return docs[i].UploadedAt.After(docs[j].UploadedAt) })

	return Div(
		// §8.9: lead with the no-key CSV import so a user without an OpenAI key is never
		// blocked from importing; the (key-gated) AI image/statement import follows as the
		// richer option. This also tightens the flow — importing populates the review draft
		// rendered just below — instead of stranding the CSV card at the bottom of the page.
		CsvImportCard(csvImportCardProps{
			Accounts:     accounts,
			ImportAcctID: importAcct.Get(),
			Msg:          msg.Get(),
			DupWarn:      csvDupWarn.Get(),
			OnChooseFile: chooseCsvFile,
			OnAcctChange: onAcct,
			OnCsvInput:   onCsv,
			OnImportCSV:  importCSV,
			OnConfirmCSV: confirmCSV,
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
					If(cadenceMsg.Get() != "",
						Span(css.Class("text-up", tw.Text12), Attr("role", "status"),
							Attr("data-testid", "cadence-reminder-msg"), cadenceMsg.Get())),
				),
			),
		}),
		// C13: the AI-powered image import comes LAST, after both no-AI paths (CSV +
		// statement paste), behind a labelled separator — so a user without an OpenAI
		// key is never led with a gated feature.
		Div(css.Class("doc-section-sep"), Attr("role", "separator"),
			Span(uistate.T("documents.aiSectionLabel"))),
		ImageImportCard(imageImportCardProps{
			ImageURL:  imageURL.Get(),
			AILoading: aiLoading.Get(),
			AIErr:     aiErr.Get(),
			NeedsKey:  needsKey.Get(),
			OnChoose:  chooseImage,
			OnReadAI:  readAI,
			Nav:       nav,
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
	ColorBySign bool              // statement mode: tint the amount by direction (money in/out); off for receipts (all positive line prices)
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
		// A purpose-built editor grid (date · description · amount · category on one
		// aligned row, actions right-aligned below) rather than the generic auto-fit
		// form-grid, which wrapped its six controls unpredictably in the modal width.
		return Div(css.Class("row-edit"),
			Form(css.Class("draft-edit-grid"), OnSubmit(saveEdit),
				Input(css.Class("field"), Attr("id", draftFieldID), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("documents.descPlaceholder")), Value(descS.Get()), OnInput(onDesc)),
				Input(css.Class("field fig"), Type("text"), Placeholder(uistate.T("documents.amountPlaceholder")), Value(amtS.Get()), OnInput(onAmt)),
				// Category is a select of existing categories (plus the AI's extracted
				// value when it doesn't match one) so editing can't introduce an
				// orphan/typo category on import (C60).
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.categoryPlaceholder")), OnChange(onCat), draftCategoryOptions(props.Categories, catS.Get())),
				Div(css.Class("draft-edit-actions"),
					Button(css.Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				),
			),
		)
	}

	// Show the amount in accounting style (parentheses for negatives) like the
	// rest of the app (C27/C2); fall back to the raw string if it won't parse
	// (e.g. while the AI value is still being corrected). In statement mode, tint
	// it by direction so money-in and money-out separate at a glance.
	amtText := r.Amount
	amtClass := "amount fig"
	minor, perr := money.ParseMinor(strings.TrimSpace(r.Amount), currency.Decimals(props.Currency))
	if props.Currency != "" && perr == nil {
		amtText = fmtMoney(money.New(minor, props.Currency))
	}
	if props.ColorBySign && perr == nil {
		switch {
		case minor > 0:
			amtClass = "amount fig amount-income"
		case minor < 0:
			amtClass = "amount fig amount-expense"
		}
	}

	// Category state is a primary review task, so surface it as a chip — and when
	// the AI left it blank (no confident match to an existing category), show a
	// "+ Category" affordance instead of nothing, so blank rows read as actionable
	// rather than disappearing.
	var catNode ui.Node = Fragment()
	if r.Category != "" {
		catNode = Span(css.Class("draft-cat-chip"), r.Category)
	} else {
		catNode = Button(css.Class("draft-cat-add"), Type("button"),
			Attr("aria-label", uistate.T("documents.addCategory")),
			OnClick(startEdit), Span("+ "), uistate.T("documents.category"))
	}

	return Div(css.Class("row"),
		Div(css.Class("row-main"),
			Span(css.Class("row-desc"), textutil.FirstNonEmpty(r.Description, uistate.T("documents.noDescription"))),
			Div(css.Class("draft-subline"),
				Span(css.Class("row-meta"), r.Date),
				// G14 §4: amber "Already imported" badge for rows that match an existing
				// transaction by date+amount — will be skipped on import.
				If(props.IsDuplicate, Span(css.Class("badge badge-warn"), uistate.T("documents.alreadyImported"))),
				catNode,
			),
		),
		Span(css.Class(amtClass), amtText),
		Div(css.Class("draft-row-actions"),
			// G14 §4: icon-only edit button (matches the × remove affordance; label
			// preserved via aria-label for screen readers).
			Button(css.Class("btn-del"), Type("button"),
				Attr("aria-label", uistate.T("documents.editRow")),
				Title(uistate.T("documents.editRow")),
				OnClick(startEdit),
				uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
			Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("documents.removeRow")), Title(uistate.T("documents.removeRow")), OnClick(rm), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
		),
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
	// CSV imports don't retain raw rows, so fall back to RowCount for the count (C11).
	count := len(d.Extracted)
	if count == 0 {
		count = d.RowCount
	}
	if count > 0 {
		meta += " · " + plural(count, "transaction")
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

// maxImageBytes is the client-side size cap for vision/OCR uploads (C97).
// Vision APIs reject or charge heavily for very large images; 10 MB covers
// any reasonable receipt or statement photo while blocking accidental uploads
// of raw camera bursts or multi-page scans.
const maxImageBytes = 10 * 1024 * 1024 // 10 MB

// pickImageDataURL opens a file picker for images and calls onData with the
// chosen file as a base64 data: URL. The data never leaves the device except to
// OpenAI when the user clicks Read. If the chosen file fails the lightweight
// client-side checks (type or size), onErr is called with a human-readable
// message and the file is not read (C97).
func pickImageDataURL(onData func(string), onErr func(string)) {
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
			file := files.Index(0)

			// C97: validate MIME type — must be an image.
			mimeType := file.Get("type").String()
			if !strings.HasPrefix(mimeType, "image/") {
				onErr(uistate.T("documents.imageTypeInvalid"))
				onLoad.Release()
				onChange.Release()
				return nil
			}

			// C97: validate file size — reject files over the cap.
			size := file.Get("size").Int()
			if size > maxImageBytes {
				onErr(uistate.T("documents.imageTooLarge"))
				onLoad.Release()
				onChange.Release()
				return nil
			}

			reader := js.Global().Get("FileReader").New()
			reader.Set("onload", onLoad)
			reader.Call("readAsDataURL", file)
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

// guessWizardField returns the pre-selected value for a wizard dropdown (as a
// string index). If the auto-detector already found the column (detected >= 0),
// that index is used verbatim. Otherwise the header names are scanned with a
// case-insensitive contains-match against each keyword in turn; the first match
// wins. Falls back to "-1" (not present) when nothing matches. This implements
// C15: pre-populate wizard dropdowns from detected header text so that common
// column names ("Date", "Amount", "Description", etc.) are auto-selected even
// when the full auto-mapping fails.
func guessWizardField(header []string, keywords []string, detected int) string {
	if detected >= 0 {
		return strconv.Itoa(detected)
	}
	lower := make([]string, len(header))
	for i, h := range header {
		lower[i] = strings.ToLower(h)
	}
	for _, kw := range keywords {
		for i, h := range lower {
			if strings.Contains(h, kw) {
				return strconv.Itoa(i)
			}
		}
	}
	return "-1"
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

// Documents is the /documents route — a thin shell that delegates entirely to
// DocumentsPanel. Routes remain registered (pending rail regroup); logic lives
// in DocumentsPanel so it can also be embedded on /transactions.
func Documents() ui.Node {
	return ui.CreateElement(DocumentsPanel, documentsPanelProps{})
}
