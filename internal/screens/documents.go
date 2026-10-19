// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/accountselect"
	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/browserstore"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/importmap"
	"github.com/monstercameron/CashFlux/internal/importsafe"
	"github.com/monstercameron/CashFlux/internal/ledger"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/pdftext"
	"github.com/monstercameron/CashFlux/internal/receiptmatch"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/staging"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/CashFlux/internal/workspace"
	"github.com/monstercameron/GoWebComponents/v5/css"
	. "github.com/monstercameron/GoWebComponents/v5/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v5/router"
	"github.com/monstercameron/GoWebComponents/v5/state"
	"github.com/monstercameron/GoWebComponents/v5/ui"
)

const visionSystemPrompt = "You extract transactions from receipt and bank-statement images. " +
	"Return each transaction with: date (YYYY-MM-DD), description, amount (negative for money out / " +
	"expenses, positive for money in, as a string), and category."

const (
	maxImportCSVBytes   = 25 * 1024 * 1024
	maxImportRows       = 10000
	draftReviewPageSize = 100
)

// draftReviewWindow returns the zero-based slice bounds for a one-based review
// page. It clamps stale page numbers after rows are removed and never returns
// more than draftReviewPageSize rows.
func draftReviewWindow(total, page int) (start, end, currentPage, pageCount int) {
	if total <= 0 {
		return 0, 0, 1, 1
	}
	pageCount = (total + draftReviewPageSize - 1) / draftReviewPageSize
	if page < 1 {
		page = 1
	}
	if page > pageCount {
		page = pageCount
	}
	start = (page - 1) * draftReviewPageSize
	end = start + draftReviewPageSize
	if end > total {
		end = total
	}
	return start, end, page, pageCount
}

// draftDuplicateCount counts rows already present in the selected account and
// repeated rows within the complete draft, matching the import skip behavior.
func draftDuplicateCount(rows []extract.Row, seen map[string]bool) int {
	count := 0
	batchSeen := make(map[string]bool, len(rows))
	for _, row := range rows {
		sig := row.Signature()
		if seen[sig] || batchSeen[sig] {
			count++
		}
		batchSeen[sig] = true
	}
	return count
}

// resolveInitialImportAccount selects the import panel's opening account. Once
// initialized, the panel keeps the user's selection in its importAcct state.
func resolveInitialImportAccount(input accountselect.ResolveInput) string {
	id, _ := accountselect.Resolve(input)
	return id
}

// statementImportSchema constrains the model's reply to a transactions array (strict
// structured outputs) for the statement-PDF source. Category is a free string here; it's
// constrained to the user's EXISTING categories by the prompt and enforced on import
// (unknown → unmapped), so the model can't create orphan categories. (Moved here from the
// former standalone statement-import screen when the PDF flow was folded into DocumentsPanel.)
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

// statementSystemPrompt builds the statement-PDF extraction instruction, listing the
// household's existing category names so the model maps to them best-effort and leaves the
// category blank otherwise — never inventing a new one.
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
		reason := localizedCSVReason(strings.TrimSpace(r.Reason))
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

func localizedCSVReason(reason string) string {
	lower := strings.ToLower(reason)
	switch {
	case reason == "amount is required":
		return uistate.T("tc4n6.amountRequired")
	case strings.Contains(lower, "possible duplicate"):
		return uistate.T("tc4n6.possibleDuplicate")
	case strings.Contains(lower, "too many decimal places"):
		return uistate.T("tc4n6.amountPrecision")
	case strings.Contains(lower, "invalid amount"):
		return uistate.T("tc4n6.amountInvalid")
	case strings.Contains(lower, "invalid cleared"):
		return uistate.T("tc4n6.clearedInvalid")
	case strings.Contains(lower, "member_ids"):
		return uistate.T("tc4n6.memberIDsInvalid")
	case strings.Contains(lower, "date"):
		return uistate.T("tc4n6.dateInvalid")
	case strings.Contains(lower, "currency"):
		return uistate.T("tc4n6.currencyInvalid")
	case strings.Contains(lower, "repeated"):
		return uistate.T("tc4n6.repeatedID")
	default:
		return uistate.T("documents.skipReasonGeneric")
	}
}

func importMapWarningSummary(warnings []importmap.RowWarning) string {
	dateCount, amountCount := 0, 0
	for _, warning := range warnings {
		switch warning.Field {
		case "amount":
			amountCount++
		default:
			dateCount++
		}
	}
	var parts []string
	if dateCount > 0 {
		parts = append(parts, uistate.T("tc4n6.ambiguousDates", dateCount))
	}
	if amountCount > 0 {
		parts = append(parts, uistate.T("tc4n6.ambiguousAmounts", amountCount))
	}
	return strings.Join(parts, " ")
}

func statementRowsForDraft(rows []statement.Row, decimals int) []extract.Row {
	out := make([]extract.Row, 0, len(rows))
	for _, row := range rows {
		out = append(out, extract.Row{
			Date: row.Date.Format("2006-01-02"), Description: row.Description,
			Amount: money.FormatMinor(row.Amount, decimals), Category: row.Category,
		})
	}
	return out
}

func statementImportCounts(ambiguousDates, ambiguousNumbers, invalidRows int) string {
	return " " + uistate.T("impc.rowCounts", ambiguousDates, ambiguousNumbers, invalidRows)
}

func importCount(n int, singular, pluralWord string) string {
	word := singular
	if n != 1 {
		word = pluralWord
	}
	return uistate.T("tc4n6.count", n, word)
}

func statementParseOptions(dateOrder, numberFormat string) statement.ParseOptions {
	options := statement.ParseOptions{DateOrder: statement.DateOrder(dateOrder)}
	switch numberFormat {
	case "dot":
		options.NumberFormat = statement.NumberFormat{DecimalSep: '.', GroupSep: ','}
	case "comma":
		options.NumberFormat = statement.NumberFormat{DecimalSep: ',', GroupSep: '.'}
	}
	return options
}

func importNumberFormatChoice(choice string) importmap.NumberFormat {
	switch choice {
	case "dot":
		return importmap.NumberFormatDotDecimal
	case "comma":
		return importmap.NumberFormatCommaDecimal
	default:
		return importmap.NumberFormatAuto
	}
}

func mapImportErrors(rows []importmap.RowError) []store.CSVRowError {
	out := make([]store.CSVRowError, 0, len(rows))
	for _, row := range rows {
		field := row.Field
		switch field {
		case "date":
			field = uistate.T("tc4n6.fieldDate")
		case "amount":
			field = uistate.T("tc4n6.fieldAmount")
		case "debit":
			field = uistate.T("tc4n6.fieldDebit")
		case "credit":
			field = uistate.T("tc4n6.fieldCredit")
		case "balance":
			field = uistate.T("tc4n6.fieldBalance")
		}
		out = append(out, store.CSVRowError{Line: row.Index + 1, Reason: uistate.T("tc4n6.rowFieldFailure", row.Index+1, field)})
	}
	return out
}

// prepareCSVPreflight computes the exact preview inputs before a CSV import is acknowledged.
func prepareCSVPreflight(app *appstate.App, accountID string, data []byte) (*csvPreflightInfo, bool, error) {
	accounts := app.Accounts()
	csvAccounts := make([]store.CSVAccount, 0, len(accounts))
	for _, account := range accounts {
		csvAccounts = append(csvAccounts, store.CSVAccount{ID: account.ID, Name: account.Name, Currency: account.Currency})
	}
	baseCurrency := app.Settings().BaseCurrency
	if baseCurrency == "" {
		baseCurrency = "USD"
	}
	txns, skipped, err := store.TransactionsFromCSVResilientForAccountList(data, baseCurrency, accountID, csvAccounts)
	if err != nil {
		return nil, false, err
	}
	if len(txns) == 0 && len(skipped) == 0 {
		return nil, true, nil
	}

	info := &csvPreflightInfo{Total: len(txns) + len(skipped), Failures: skipped}
	info.Whys = importsafe.Duplicates(txns, app.Transactions(), accountID)
	info.Dupes = len(info.Whys)
	base := app.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	inputs := make([]staging.Input, 0, len(txns))
	for i := range txns {
		txn := txns[i]
		inputs = append(inputs, staging.Input{
			Date: txn.Date, Description: txn.Desc, AmountMinor: txn.Amount.Amount,
			HasAmount: true, AccountID: txn.AccountID, Transaction: &txn,
		})
	}
	info.Batch = staging.Stage(inputs, app.Transactions(), accountID, base)
	info.Pairs = importsafe.TransferPairs(txns, app.Transactions(), accountID, 3)
	if account, ok := domain.AccountByID(accounts, accountID); ok {
		info.Dec = currency.Decimals(account.Currency)
		info.AcctName = account.Name
		if balance, balanceErr := ledger.Balance(account, app.Transactions()); balanceErr == nil {
			info.HasBal = true
			info.BeforeMinor = balance.Amount
			net, after, impactErr := importsafe.Impact(balance.Amount, txns, account.ID, account.Currency)
			info.NetMinor, info.AfterMinor = net, after
			if impactErr != nil {
				info.HasBal = false
			} else {
				info.Jump = importsafe.JumpWarning(balance.Amount, info.NetMinor)
			}
		}
	}
	return info, false, nil
}

func validateCSVRowLimit(data []byte, limit int) error {
	if limit <= 0 {
		return nil
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	firstLine := text
	if i := strings.IndexAny(text, "\r\n"); i >= 0 {
		firstLine = text[:i]
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = statement.DetectDelimiter(firstLine)
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil {
		return err
	}
	count := 0
	for {
		_, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		count++
		if count > limit {
			return errors.New("too many CSV rows")
		}
	}
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

// ImportModalFormID is the id of the hidden form DocumentsPanel renders so the
// transactions Import flip modal's footer Save (rendered by FlipPanel, outside this
// component) can submit it by id and commit the ready import. Shared with
// ImportPanelHost's FlipPanelProps.FormID.
const ImportModalFormID = "import-modal-form"

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
	// When this panel is shown inside the transactions Import flip modal, its footer
	// Save/Cancel dismiss the modal by flipping this shared atom. On the /documents
	// route the atom is simply always false and never read, so this is a no-op there.
	importModalOpen := uistate.UseImportPanelOpen()
	memberLens := uistate.UseMemberLens()
	csvText := ui.UseState("")
	stmtText := ui.UseState("")
	msg := ui.UseState("")
	cadenceMsg := ui.UseState("") // C18: inline confirmation shown next to the reminder button

	// C88: pre-import duplicate warning state.
	// csvPreflightS holds the staged CSV import's pre-commit preview (#57);
	// non-nil = the preflight card is visible and pendingCSV holds the bytes.
	csvPreflightS := ui.UseState((*csvPreflightInfo)(nil))
	// csvDupWarn holds the human-readable warning string (non-empty = warning visible).
	// pendingCSV holds the raw CSV bytes awaiting user confirmation; cleared on import or cancel.
	csvDupWarn := ui.UseState("")
	pendingCSV := ui.UseState([]byte(nil))

	accounts := app.Accounts()
	// Saved defaults seed a new import only. The selector stays controlled by
	// importAcct afterward, so the user's choice remains authoritative.
	defaultMemberID := ""
	if len(memberLens) == 1 {
		defaultMemberID = memberLens[0]
	}
	defaultAcc := resolveInitialImportAccount(accountselect.ResolveInput{
		Accounts: accounts, Transactions: app.Transactions(), ActiveMemberID: defaultMemberID,
		Members: app.Members(), WorkspaceDefaultID: app.DefaultAccountID(),
	})
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
	// #62: the review-stage rows live in a session ATOM, not component state, so
	// navigating away mid-review keeps them and the dashboard resume card can
	// jump back into the wizard. The mount effect below restores the stage.
	draft := uistate.UseImportDraftRows()
	reviewPage := ui.UseState(1)
	importAcct := ui.UseState(defaultAcc)
	receiptMode := ui.UseState(false)  // import the draft as ONE split transaction
	receiptTotal := ui.UseState("")    // the receipt's single total (defaults to the line sum)
	receiptMerchant := ui.UseState("") // optional store name

	// Two-stage import wizard: "input" (the Add-your-data hub — CSV, statement text,
	// statement PDF, receipt image) → "review" (the shared draft table). Any AI
	// extraction that yields rows advances to review; the lossless CSV path imports
	// directly from the input stage and never enters review.
	stage := ui.UseState("input")
	// setDraft is the single place draft rows are set, so the wizard stage stays in
	// lockstep with the draft: non-empty rows show the review stage, an emptied draft
	// (import done, cleared, or last row removed) returns to the input hub.
	setDraft := func(rows []extract.Row) { setDocumentsDraft(draft, stage, reviewPage, rows) }
	// #62: rows restored from the session atom (a mid-review navigation away and
	// back) re-enter the review stage instead of stranding on the input hub.
	ui.UseEffect(func() func() {
		if len(draft.Get()) > 0 {
			stage.Set("review")
		}
		return nil
	}, "resume-mount")

	// Statement-PDF source, folded in from the former standalone "Import statement"
	// modal (StatementImportBody): the attached PDF as a data URL for the AI call, its
	// raw bytes for the optional Artifacts copy, its name, and whether to keep a copy.
	pdfData := ui.UseState("")
	pdfBytes := ui.UseState([]byte(nil))
	pdfName := ui.UseState("")
	pdfSaveCopy := ui.UseState(false)

	// Stage-1 source selection. "" shows the visual document-type picker (a grid of
	// tiles grouped into the Smart / Smart+ branches); a non-empty value shows that one
	// source's form, animated in over the grid. The set: "csv", "stmt", "pdf", "receipt".
	source := ui.UseState("")
	onPickCSV := ui.UseEvent(Prevent(func() { source.Set("csv") }))
	onPickStmt := ui.UseEvent(Prevent(func() { source.Set("stmt") }))
	onPickPDF := ui.UseEvent(Prevent(func() { source.Set("pdf") }))
	onPickReceipt := ui.UseEvent(Prevent(func() { source.Set("receipt") }))
	onPickAmazon := ui.UseEvent(Prevent(func() { source.Set("amazon") })) // TX4
	backToTypes := ui.UseEvent(Prevent(func() { source.Set("") }))

	// Wizard stage navigation. goReview/goInput move between the two stages without
	// touching the draft (so "← Back" then "Review →" is lossless). goInput also resets
	// the source picker to the grid, so "Add more data" lands on the type chooser.
	// goArtifacts closes the modal and navigates to the Artifacts vault — the
	// replacement for the old inline import-history list.
	goReview := ui.UseEvent(Prevent(func() {
		next := transitionImportFlow(importFlowState{Stage: importFlowStage(stage.Get()), Source: importFlowSource(source.Get())}, importActionNext)
		stage.Set(string(next.Stage))
		source.Set(string(next.Source))
	}))
	goInput := ui.UseEvent(Prevent(func() {
		next := transitionImportFlow(importFlowState{Stage: importFlowStage(stage.Get()), Source: importFlowSource(source.Get())}, importActionAddMore)
		stage.Set(string(next.Stage))
		source.Set(string(next.Source))
	}))
	goArtifacts := ui.UseEvent(Prevent(func() {
		importModalOpen.Set(false)
		nav.Navigate(uistate.RoutePath("/artifacts"))
	}))
	// #57 honesty: the recent-imports fallback link — when a run's safety snapshot
	// has aged out of the checkpoint ring so its per-run Roll back is gone, the
	// history still points to the Activity timeline where the change can be reviewed.
	goActivity := ui.UseEvent(Prevent(func() {
		importModalOpen.Set(false)
		nav.Navigate(uistate.RoutePath("/activity"))
	}))

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
	dateOrderChoice := ui.UseState("")
	numberFormatChoice := ui.UseState("auto")
	statementRaw := ui.UseState("")
	statementAmbiguousDates := ui.UseState(0)
	statementAmbiguousNumbers := ui.UseState(0)
	statementInvalidRows := ui.UseState(0)
	statementFormatOptionsVisible := ui.UseState(false)

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
	// skipped + cpID enrich the history row (#57): the full result and the
	// pre-import checkpoint that can roll this exact run back.
	recordDocument := func(kind domain.DocumentKind, accountID string, rows []extract.Row, rowCount, skipped int, cpID string) {
		recordDocumentAs(app, id.New(), kind, accountID, rows, rowCount, skipped, 0, 0, cpID)
	}
	checkpointSaving := ui.UseState(false)
	checkpointRun := func(label string, operation func(string)) {
		if checkpointSaving.Get() {
			return
		}
		checkpointSaving.Set(true)
		uistate.SaveCheckpointThen(label, func(cpID string, err error) {
			checkpointSaving.Set(false)
			if err != nil {
				uistate.PostNotice(uistate.T("enc34.checkpointFailed"), true)
				return
			}
			operation(cpID)
		})
	}

	csvContext := csvImportContext{
		App: app, Accounts: accounts, ImportAccount: importAcct, CSVText: csvText,
		DuplicateWarn: csvDupWarn, PendingCSV: pendingCSV, Preflight: csvPreflightS,
		Message: msg, ModalOpen: importModalOpen, Revision: rev, CheckpointRun: checkpointRun,
	}
	commitCSVImport := csvContext.commitCSVImport

	// confirmCSV is the preflight's "Import now" handler: commits the pending
	// CSV bytes staged when the preview card was built (#57, ex-C88).
	confirmCSV := ui.UseEvent(func() {
		data := pendingCSV.Get()
		if len(data) == 0 {
			return
		}
		commitCSVImport(data)
	})
	cancelCSVPreflight := ui.UseEvent(Prevent(func() {
		pendingCSV.Set(nil)
		csvPreflightS.Set(nil)
	}))

	// buildCSVPreflight parses the incoming bytes exactly as the import would
	// (#57) and stages them behind a pre-commit preview: row/duplicate counts
	// with per-row "why matched" detail, the balance impact on the import
	// account with an implausible-jump warning, and incoming rows that look
	// like the second leg of a transfer already in the ledger. Nothing writes
	// until the user confirms.
	buildCSVPreflight := func(data []byte) {
		info, empty, err := prepareCSVPreflight(app, importAcct.Get(), data)
		if err != nil {
			msg.Set(uistate.T("documents.csvError", strings.TrimPrefix(err.Error(), "store: ")))
			return
		}
		if empty {
			msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		pendingCSV.Set(data)
		csvDupWarn.Set("")
		msg.Set("")
		csvPreflightS.Set(info)
	}

	// chooseCsvFile opens a file picker for .csv files and feeds the bytes into
	// the same staged preflight the paste path uses (C60 → #57).
	chooseCsvFile := ui.UseEvent(func() {
		pickFileLimited(".csv,text/csv", maxImportCSVBytes, func(_, limit int) {
			msg.Set(uistate.T("impc.fileTooLarge", "CSV", fileLimitLabel(limit)))
		}, func(_, _ string, data []byte) {
			if len(data) == 0 {
				msg.Set(uistate.T("documents.csvFileEmpty"))
				return
			}
			if err := validateCSVRowLimit(data, maxImportRows); err != nil {
				msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
				return
			}
			buildCSVPreflight(data)
		})
	})
	onAcct := ui.UseEvent(func(e ui.Event) { importAcct.Set(e.GetValue()) })
	onReceiptTotal := ui.UseEvent(func(v string) { receiptTotal.Set(v) })
	onReceiptMerchant := ui.UseEvent(func(v string) { receiptMerchant.Set(v) })

	// importCSV is the paste-path import handler (#57): every first click runs
	// the staged preflight — the commit happens from the preview card's
	// "Import now" (confirmCSV).
	importCSV := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(csvText.Get())
		if data == "" {
			// QA H2/CF-03: a file picked via "Choose a CSV file" stages its bytes
			// while the paste box stays empty — the primary Import must work with
			// those staged bytes, not demand a paste.
			if staged := pendingCSV.Get(); len(staged) > 0 {
				commitCSVImport(staged)
				return
			}
			msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		if err := validateCSVRowLimit([]byte(data), maxImportRows); err != nil {
			msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
			return
		}
		buildCSVPreflight([]byte(data))
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
		upperData := strings.ToUpper(data)
		isStructured := strings.HasPrefix(data, "%PDF") || strings.HasPrefix(data, "PK\x03\x04") ||
			strings.HasPrefix(upperData, "OFXHEADER:") || strings.HasPrefix(upperData, "<?XML") ||
			strings.HasPrefix(upperData, "<?OFX") || strings.HasPrefix(upperData, "<OFX")
		if !isStructured {
			if err := validateCSVRowLimit([]byte(data), maxImportRows); err != nil {
				msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
				return
			}
		}
		// Parse amounts at the import account's currency precision. ParseAny
		// auto-detects the format — OFX 1.x/2.x (bank/card statements) as well as the
		// delimited CSV/TSV/pipe path — so a pasted .ofx/.qfx statement now imports
		// through the same review pipeline (C74 Tier 2).
		dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
		statementRaw.Set(data)
		statementFormatOptionsVisible.Set(true)
		options := statementParseOptions(dateOrderChoice.Get(), numberFormatChoice.Get())
		var st statement.Statement
		var stErr error
		var stRows []statement.Row
		if strings.HasPrefix(data, "%PDF") || strings.HasPrefix(strings.ToUpper(data), "OFXHEADER:") || strings.HasPrefix(data, "<OFX") || strings.HasPrefix(data, "<?OFX") {
			stRows, stErr = statement.ParseAny(strings.NewReader(data), dec)
		} else {
			st, stErr = statement.ParseWithOptions(data, dec, options)
			stRows = st.Rows
			statementAmbiguousDates.Set(st.AmbiguousDates)
			statementAmbiguousNumbers.Set(st.AmbiguousNumbers)
			statementInvalidRows.Set(len(st.Errors))
			if st.DateOrder != "" {
				dateOrderChoice.Set(string(st.DateOrder))
			}
			if st.NumberFormat.DecimalSep == ',' {
				numberFormatChoice.Set("comma")
			} else if st.NumberFormat.DecimalSep == '.' && st.NumberFormat.GroupSep == ',' {
				numberFormatChoice.Set("dot")
			}
		}
		err := stErr
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
				if len(rawRecs) > maxImportRows {
					msg.Set(uistate.T("impc.tooManyRows", maxImportRows))
					return
				}
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
		rows := statementRowsForDraft(stRows, dec)
		if len(rows) == 0 {
			msg.Set(uistate.T("documents.stmtNoneFound"))
			return
		}
		// Successful parse — hide wizard, populate draft.
		wizardVisible.Set(false)
		setDraft(rows)
		msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")) + statementImportCounts(statementAmbiguousDates.Get(), statementAmbiguousNumbers.Get(), statementInvalidRows.Get()))
		rev.Set(rev.Get() + 1)
	}))
	onDateOrderChoice := ui.UseEvent(func(e ui.Event) { dateOrderChoice.Set(e.GetValue()) })
	onNumberFormatChoice := ui.UseEvent(func(e ui.Event) { numberFormatChoice.Set(e.GetValue()) })
	applyStatementFormats := ui.UseEvent(Prevent(func() {
		data := statementRaw.Get()
		if data == "" {
			return
		}
		dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
		options := statementParseOptions(dateOrderChoice.Get(), numberFormatChoice.Get())
		st, err := statement.ParseWithOptions(data, dec, options)
		if err != nil {
			msg.Set(uistate.T("documents.stmtError", strings.TrimPrefix(err.Error(), "statement: ")))
			return
		}
		statementAmbiguousDates.Set(st.AmbiguousDates)
		statementAmbiguousNumbers.Set(st.AmbiguousNumbers)
		statementInvalidRows.Set(len(st.Errors))
		statementFormatOptionsVisible.Set(true)
		rows := statementRowsForDraft(st.Rows, dec)
		if len(rows) == 0 {
			msg.Set(uistate.T("impc.noValidRows", len(st.Errors)))
			return
		}
		setDraft(rows)
		msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")) + statementImportCounts(st.AmbiguousDates, st.AmbiguousNumbers, len(st.Errors)))
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
			Name:         "manual",
			DateCol:      atoi(wizardDate.Get()),
			DescCol:      atoi(wizardDescCol.Get()),
			AmountCol:    atoi(wizardAmount.Get()),
			DebitCol:     atoi(wizardDebit.Get()),
			CreditCol:    atoi(wizardCredit.Get()),
			BalanceCol:   -1,
			CurrencyCol:  -1,
			SkipRows:     0,
			Decimals:     dec,
			NumberFormat: importNumberFormatChoice(numberFormatChoice.Get()),
			DateOrder:    statement.DateOrder(dateOrderChoice.Get()),
		}
		rawRecs := wizardRawRows.Get()
		// Flatten [][][]string to [][]string (each outer entry is one record set).
		flat := make([][]string, 0)
		for _, group := range rawRecs {
			flat = append(flat, group...)
		}
		stRows, rowErrors, rowWarnings := importmap.ApplyWithDiagnostics(p, flat)
		if len(stRows) == 0 {
			msg.Set(uistate.T("documents.wizardNoRows", csvSkipDetail(mapImportErrors(rowErrors))))
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
		setDraft(rows)
		message := uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows"))
		if len(rowErrors) > 0 {
			message += " " + csvSkipDetail(mapImportErrors(rowErrors))
		}
		if len(rowWarnings) > 0 {
			message += " " + importMapWarningSummary(rowWarnings)
		}
		msg.Set(message)
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
			Name:         name,
			DateCol:      atoi(wizardDate.Get()),
			DescCol:      atoi(wizardDescCol.Get()),
			AmountCol:    atoi(wizardAmount.Get()),
			DebitCol:     atoi(wizardDebit.Get()),
			CreditCol:    atoi(wizardCredit.Get()),
			BalanceCol:   -1,
			CurrencyCol:  -1,
			Decimals:     currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get())),
			NumberFormat: importNumberFormatChoice(numberFormatChoice.Get()),
			DateOrder:    statement.DateOrder(dateOrderChoice.Get()),
		}
		saved, err := app.SaveImportProfile(importmap.SavedProfile{Profile: p}, activeImportProfileWorkspaceID())
		if err != nil {
			msg.Set(uistate.T("tc4n6.profileSaveFailed", err.Error()))
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
			cadenceMsg.Set(uistate.T("tc4n6.reminderFailed", err.Error()))
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
				setDraft([]extract.Row{})
			},
			func(e string) { aiErr.Set(e) },
		)
	})

	settings := app.Settings()
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := uistate.BackendConnectionActive(pr)
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
			setDraft(rows)
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

	// choosePDF attaches a statement PDF (folded in from the former Import-statement
	// modal). The AI reads the PDF's text + page images natively, so there's no
	// client-side rendering; we just keep the data URL for the call and the raw bytes
	// for the optional Artifacts copy.
	choosePDF := ui.UseEvent(func() {
		pickFile(".pdf,application/pdf", func(name, mime string, data []byte) {
			if len(data) == 0 {
				aiErr.Set(uistate.T("statementimport.readErr"))
				return
			}
			if len(data) > 45*1024*1024 { // OpenAI caps files at 50MB; stay under it
				aiErr.Set(uistate.T("statementimport.tooLarge"))
				return
			}
			mt := mime
			if mt == "" {
				mt = "application/pdf"
			}
			pdfData.Set("data:" + mt + ";base64," + base64.StdEncoding.EncodeToString(data))
			pdfBytes.Set(data)
			pdfName.Set(name)
			aiErr.Set("")
			needsKey.Set(false)
			setDraft([]extract.Row{})
		})
	})
	onTogglePdfSave := ui.UseEvent(func() { pdfSaveCopy.Set(!pdfSaveCopy.Get()) })

	// runPDFAI sends the attached statement PDF to the model for extraction, mapping
	// categories to the household's existing expense/income set (unmatched → blank).
	// Statement PDF import is BYO-key only — the optional cloud proxy doesn't carry
	// file uploads — so it gates on the OpenAI key, not the backend.
	runPDFAI := ui.UseEvent(func() {
		if pdfData.Get() == "" {
			aiErr.Set(uistate.T("statementimport.chooseFirst"))
			return
		}
		if settings.OpenAIKey == "" {
			needsKey.Set(true)
			return
		}
		var catNames []string
		for _, c := range app.Categories() {
			if c.Kind == domain.KindExpense || c.Kind == domain.KindIncome {
				catNames = append(catNames, c.Name)
			}
		}
		pdfModel := aiModel
		if pdfModel == "" {
			pdfModel = "gpt-5.5" // needs a vision-capable model to read the PDF's pages
		}
		aiLoading.Set(true)
		aiErr.Set("")
		needsKey.Set(false)
		ai.SendStructuredFileChat(settings.OpenAIKey, ai.DefaultBaseURL, pdfModel,
			statementSystemPrompt(catNames), "Extract every transaction from this statement.",
			firstNonEmpty(pdfName.Get(), "statement.pdf"), pdfData.Get(), 0,
			"transactions", []byte(statementImportSchema),
			func(content string, _ ai.Usage) {
				aiLoading.Set(false)
				rows, err := extract.ParseRows(content)
				if err != nil {
					aiErr.Set(err.Error())
					return
				}
				if len(rows) == 0 {
					aiErr.Set(uistate.T("statementimport.noneFound"))
					return
				}
				setDraft(rows)
			},
			func(e string) { aiLoading.Set(false); aiErr.Set(e) })
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
			setDraft(rows)
			msg.Set(uistate.T("documents.stmtParsed", importCount(len(rows), "row", "rows")))
		}
		onError := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, uistate.EffectiveServerToken(pr.ServerToken), model, msgs, 0.1, onResult, onError)
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
			setDraft(updated)
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
			setDraft(next)
			msg.Set(uistate.T("documents.categorizing"))
		}
		onError := func(e string) { aiLoading.Set(false); aiErr.Set(e) }
		if useBackendAI {
			ai.SendProxyChat(pr.ServerURL, uistate.EffectiveServerToken(pr.ServerToken), model, msgs, 0.1, onResult, onError)
		} else {
			ai.SendChat(settings.OpenAIKey, ai.DefaultBaseURL, model, msgs, 0.1, onResult, onError)
		}
	})

	// doImportDraft imports the reviewed draft rows as N standalone transactions.
	// Returns whether the import succeeded (false leaves the modal open so the error
	// shows). Wrapped by importDraft (the draft card's button) and reused by the
	// modal footer's Save.
	doImportDraft := func() {
		rows := draft.Get()
		// #55: checkpoint before the reviewed-rows import commits.
		checkpointRun(uistate.T("ckpt.beforeDocImport", importCount(len(rows), "row", "rows")), func(cpID string) {
			result, err := app.ImportReviewedDocumentRows(domain.DocImage, importAcct.Get(), rows)
			if err != nil {
				aiErr.Set(strings.TrimPrefix(err.Error(), "appstate: "))
				return
			}
			// #57: attach the pre-import checkpoint to the history record appstate
			// just created, so this run's roll-back is one click from the history.
			if cpID != "" && result.DocumentID != "" {
				for _, d := range app.Documents() {
					if d.ID == result.DocumentID {
						d.CheckpointID = cpID
						_ = app.PutDocument(d)
						break
					}
				}
			}
			// If the draft came from a statement PDF and the user asked to keep a copy,
			// store the source PDF in the Artifacts vault. Its bytes go to the IndexedDB
			// blob store (not the SQLite dataset) via StoreBlobForArtifact, so the record
			// stays lightweight; the Put blocks on IDB, so run it off the event goroutine.
			if pdfSaveCopy.Get() && len(pdfBytes.Get()) > 0 {
				pdf := domain.Artifact{
					ID: id.New(), Name: firstNonEmpty(pdfName.Get(), "statement.pdf"),
					Kind: artifacts.KindPDF, MIME: "application/pdf",
					Bytes: pdfBytes.Get(), CreatedAt: time.Now(),
				}
				pdf.Size = artifacts.Size(pdf)
				go func() {
					stored, serr := app.StoreBlobForArtifact(pdf)
					if serr != nil {
						uistate.PostNotice(serr.Error(), true)
						return
					}
					if perr := app.PutArtifact(stored); perr != nil {
						uistate.PostNotice(perr.Error(), true)
						return
					}
					app.RefreshBlobUsage()
					uistate.RequestPersist()
				}()
			}
			setDraft([]extract.Row{})
			imageURL.Set("")
			imageDraftAtom.Set("") // C98: clear persisted image after successful import
			pdfData.Set("")
			pdfBytes.Set(nil)
			pdfName.Set("")
			pdfSaveCopy.Set(false)
			aiErr.Set("")
			summary := uistate.T("impc.importSummary", result.Imported, result.Skipped, len(result.Failed), len(result.PossibleDuplicates))
			if detail := csvSkipDetail(result.Failed); detail != "" {
				summary += " " + detail
			}
			msg.Set(summary)
			// C364: undo story for a committed image/statement import.
			if result.Imported > 0 {
				postUndoStory(uistate.T("documents.importUndo", importCount(result.Imported, "transaction", "transactions")))
			}
			rev.Set(rev.Get() + 1)
		})
	}
	importDraft := ui.UseEvent(Prevent(func() { doImportDraft() }))

	// importReceipt imports the reviewed lines as ONE transaction split across
	// categories (receipt mode), instead of N standalone transactions.
	// doImportReceipt imports the reviewed lines as ONE transaction split across
	// categories (receipt mode). Returns whether it succeeded. Wrapped by
	// importReceipt and reused by the modal footer's Save.
	doImportReceipt := func() {
		rows := draft.Get()
		lines := make([]extract.ReceiptLine, 0, len(rows))
		for _, r := range rows {
			lines = append(lines, extract.ReceiptLine{Description: r.Description, Category: r.Category, Amount: absAmount(r.Amount)})
		}
		rec := extract.Receipt{Merchant: strings.TrimSpace(receiptMerchant.Get()), Total: absAmount(receiptTotal.Get()), Lines: lines}
		// #55: checkpoint before the receipt import commits.
		checkpointRun(uistate.T("ckpt.beforeReceiptImport"), func(cpID string) {
			tx, err := app.ImportReceipt(rec, importAcct.Get(), time.Now())
			if err != nil {
				aiErr.Set(strings.TrimPrefix(err.Error(), "appstate: "))
				return
			}
			recordDocument(domain.DocImage, importAcct.Get(), rows, len(rows), 0, cpID)
			// #58: a confirmed create also teaches the learn tally this merchant's
			// category choices, same as an attach.
			learnReceiptChoices(rec.Merchant, tx.Splits)
			setDraft([]extract.Row{})
			imageURL.Set("")
			imageDraftAtom.Set("") // C98: clear persisted image after successful import
			receiptTotal.Set("")
			receiptMerchant.Set("")
			receiptMode.Set(false)
			aiErr.Set("")
			msg.Set(uistate.T("tc4n6.receiptImported", importCount(len(tx.Splits), "category", "categories")))
			// C364: undo story for a committed receipt import.
			postUndoStory(uistate.T("documents.receiptImportUndo"))
			rev.Set(rev.Get() + 1)
		})
	}
	importReceipt := ui.UseEvent(Prevent(func() { doImportReceipt() }))

	// saveModalImport is the transactions Import flip modal's footer Save: it commits
	// whatever is ready — a reviewed draft (as split receipt or N transactions), else
	// pasted CSV — then closes the modal. With nothing entered it just closes (Done).
	// A failed commit keeps the modal open so its error message stays visible. The
	// per-flow inner buttons (CSV Import, Import draft) remain for in-place use; this
	// is the pinned footer action the standardized modals all have. It submits a
	// dedicated hidden form (importModalFormID) so FlipPanel's footer Save can drive it.
	saveModalImport := ui.UseEvent(Prevent(func() {
		switch {
		case len(draft.Get()) > 0 && receiptMode.Get():
			doImportReceipt()
		case len(draft.Get()) > 0:
			doImportDraft()
		default:
			if raw := []byte(strings.TrimSpace(csvText.Get())); len(raw) > 0 {
				commitCSVImport(raw)
			} else if staged := pendingCSV.Get(); len(staged) > 0 {
				// QA H2/CF-03: a chosen CSV file awaiting the duplicate confirmation
				// counts as "something entered" — footer Save commits it rather than
				// silently closing over the staged import.
				commitCSVImport(staged)
			} else {
				importModalOpen.Set(false)
			}
		}
	}))

	removeDraft := func(i int) {
		cur := draft.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := make([]extract.Row, 0, len(cur)-1)
		next = append(next, cur[:i]...)
		next = append(next, cur[i+1:]...)
		setDraft(next)
	}

	updateDraft := func(i int, r extract.Row) {
		cur := draft.Get()
		if i < 0 || i >= len(cur) {
			return
		}
		next := append([]extract.Row{}, cur...)
		next[i] = r
		setDraft(next)
	}

	// Draft review list: each row can be removed before importing.
	rows := draft.Get()
	reviewStart, reviewEnd, currentReviewPage, reviewPageCount := draftReviewWindow(len(rows), reviewPage.Get())
	visibleRows := rows[reviewStart:reviewEnd]
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

	// #58: receipt-to-transaction matching — when the reviewed receipt's total
	// matches an existing charge near its date, offer "attach to that charge"
	// instead of always creating a new transaction (the classic double-entry:
	// the card import already recorded the purchase the receipt documents).
	var receiptCands []receiptmatch.Candidate
	if receiptMode.Get() {
		if totalMinor, terr := money.ParseMinor(absAmount(receiptTotal.Get()), recDec); terr == nil && totalMinor > 0 {
			recWhen := time.Now()
			if len(rows) > 0 {
				if d, derr := dateutil.ParseDate(strings.TrimSpace(rows[0].Date)); derr == nil {
					recWhen = d
				}
			}
			receiptCands = receiptmatch.Match(totalMinor, receiptMerchant.Get(), recWhen, app.Transactions(), receiptmatch.DefaultWindowDays)
		}
	}
	// attachReceipt documents the chosen existing transaction with the reviewed
	// lines (splits + reviewed), records the merchant→category choices in the
	// learn tally, and clears the draft — no new row is created.
	attachReceipt := func(txnID string) {
		rowsNow := draft.Get()
		lines := make([]extract.ReceiptLine, 0, len(rowsNow))
		for _, r := range rowsNow {
			lines = append(lines, extract.ReceiptLine{Description: r.Description, Category: r.Category, Amount: absAmount(r.Amount)})
		}
		rec := extract.Receipt{Merchant: strings.TrimSpace(receiptMerchant.Get()), Total: absAmount(receiptTotal.Get()), Lines: lines}
		tx, err := app.AttachReceiptToTransaction(txnID, rec)
		if err != nil {
			aiErr.Set(strings.TrimPrefix(err.Error(), "appstate: "))
			return
		}
		learnReceiptChoices(rec.Merchant, tx.Splits)
		setDraft([]extract.Row{})
		imageURL.Set("")
		imageDraftAtom.Set("")
		receiptTotal.Set("")
		receiptMerchant.Set("")
		receiptMode.Set(false)
		aiErr.Set("")
		// A toast, not the panel msg line — the attach clears the draft, which
		// swaps the review stage's layout out from under an inline message.
		uistate.PostNotice(uistate.T("documents.receiptAttached", tx.Desc, importCount(len(tx.Splits), "category", "categories")), false)
		uistate.BumpDataRevision()
		rev.Set(rev.Get() + 1)
	}
	receiptToggle := uiw.ToggleRow(uiw.ToggleRowProps{
		Label: uistate.T("documents.receiptToggle"),
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
	clearDraft := ui.UseEvent(func() { setDraft([]extract.Row{}) })

	// #57: per-run import roll-back — restores the pre-import checkpoint that
	// rode the history record. Confirmed; a restore replaces the LIVE dataset.
	rollbackImport := func(d domain.Document) {
		uistate.ConfirmModal(uistate.T("documents.rollbackConfirm"), true, func(ok bool) {
			if !ok {
				return
			}
			if uistate.RestoreCheckpoint(d.CheckpointID) {
				uistate.PostNotice(uistate.T("documents.rolledBack"), false)
				rev.Set(rev.Get() + 1)
			} else {
				uistate.PostNotice(uistate.T("ckpt.restoreErr"), true)
			}
		})
	}

	// Statement-PDF source card (Stage 1), folded in from the former standalone
	// "Import statement" modal. Choose a PDF → the AI reads it natively → rows land in
	// the shared review draft (which advances to Stage 2). Optionally keeps a copy of
	// the source PDF in the Artifacts vault on import.
	pdfFileLabel := uistate.T("statementimport.noFile")
	if n := strings.TrimSpace(pdfName.Get()); n != "" {
		pdfFileLabel = n
	}
	pdfCard := Fragment(
		importFormHeader(true, uiw.Icon(icon.FileText, css.Class(tw.W5, tw.H5)),
			uistate.T("documents.typePdfTitle"), uistate.T("documents.pdfFormDesc")),
		Div(css.Class("doc-form-body"),
			Div(css.Class("statement-drop"),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "statementimport-choose"), OnClick(choosePDF),
					uiw.Icon(icon.FileText, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("statementimport.choose"))),
				Span(css.Class("statement-file", tw.TextDim), Attr("data-testid", "statementimport-filename"), pdfFileLabel)),
			If(aiLoading.Get(), P(css.Class("muted"), Attr("data-testid", "statementimport-loading"), uistate.T("statementimport.reading"))),
			buttonWithDisabled(pdfData.Get() == "" || aiLoading.Get(),
				[]any{css.Class("btn btn-plus", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "statementimport-run"), OnClick(runPDFAI)},
				uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("statementimport.run"))),
			If(pdfData.Get() != "", Label(css.Class("statement-savedoc", tw.Flex, tw.ItemsCenter, tw.Gap2), Style(map[string]string{"cursor": "pointer"}),
				Input(append([]any{css.Class("cf-check"), Type("checkbox"), Attr("data-testid", "statementimport-savedoc"), OnChange(onTogglePdfSave)}, checkedAttr(pdfSaveCopy.Get())...)...),
				Div(css.Class("row-main"),
					Span(uistate.T("statementimport.saveDoc")),
					Span(css.Class("row-meta", tw.TextDim), uistate.T("statementimport.saveDocHint"))))),
		),
	)

	flowScreen := importFlowScreenFor(importFlowState{Stage: importFlowStage(stage.Get()), Source: importFlowSource(source.Get())})
	inReview := flowScreen == importScreenReview

	return Div(
		// Hidden form the transactions Import flip modal's footer Save submits (by id)
		// to commit the ready import and close. Empty and invisible; on the /documents
		// route nothing references it, so it's inert. Kept unconditional so its OnSubmit
		// hook stays at a stable render position.
		Form(Attr("id", ImportModalFormID), OnSubmit(saveModalImport)),

		// STAGE 1 — "Add your data": a visual document-type picker split into a Smart
		// (deterministic) branch and a Smart+ (generative-AI) branch. Picking a type
		// animates in that one source's form; AI/parse extractions advance to the review
		// stage, while the lossless CSV path imports directly.
		If(!inReview, Fragment(
			// A standing "Review →" shortcut whenever a draft already exists (e.g. after
			// stepping back from review to add another source).
			If(len(rows) > 0, Div(css.Class(tw.Mb2, tw.Flex, tw.JustifyEnd),
				Button(css.Class("btn btn-primary"), Type("button"), Attr("data-testid", "import-review-btn"),
					OnClick(goReview), uistate.T("documents.reviewStage")))),

			IfElse(source.Get() == "",
				importTypePicker(importTypePickerProps{
					OnCSV: onPickCSV, OnStatement: onPickStmt, OnOrder: onPickAmazon,
					OnPDF: onPickPDF, OnReceipt: onPickReceipt,
				}),
				// ---- the selected source's form (animates in over the grid) ----
				Div(css.Class("doc-form"), Attr("data-testid", "import-source-form"),
					Div(css.Class("doc-form-head"),
						Button(css.Class("btn"), Type("button"), Attr("data-testid", "import-back-types"),
							OnClick(backToTypes), uistate.T("documents.backToTypes"))),

					// Smart · CSV / spreadsheet — lossless, imports directly.
					If(source.Get() == "csv", Fragment(
						importFormHeader(false, uiw.Icon(icon.Upload, css.Class(tw.W5, tw.H5)),
							uistate.T("documents.typeCsvTitle"), uistate.T("documents.csvFormDesc")),
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
							Preflight:    csvPreflightCard(csvPreflightS.Get(), confirmCSV, cancelCSVPreflight),
						}),
					)),

					// Smart · statement text — deterministic column auto-mapping (with a
					// Smart+ AI-extract escalation for layouts the parser can't map).
					If(source.Get() == "stmt", Fragment(
						importFormHeader(false, uiw.Icon(icon.Landmark, css.Class(tw.W5, tw.H5)),
							uistate.T("documents.typeStmtTitle"), uistate.T("documents.stmtFormDesc")),
						Div(css.Class("doc-form-body"),
							Form(OnSubmit(parseStatement),
								Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2), Style(map[string]string{"margin-bottom": "0.5rem"}),
									Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.stmtParse")),
									Button(css.Class("btn btn-plus", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Attr("data-testid", "extract-ai-btn"),
										OnClick(extractWithAI),
										uiw.Icon(icon.Sparkles, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("documents.extractAI")),
										// R24: the control says it needs a key BEFORE it is pressed. The
										// plain "Parse" button beside it is the no-key sibling and works
										// regardless, which is what makes this a label rather than a wall.
										KeyGateMark(AIKeyConfigured())),
								),
								Textarea(css.Class("field field-wide"), Attr("rows", "4"),
									Placeholder(uistate.T("documents.stmtSample")),
									OnInput(onStmt),
								),
							),
							Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
								P(css.Class("muted", tw.Text12), uistate.T("documents.cadenceDesc")),
								Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "cadence-reminder-btn"),
									OnClick(createCadenceReminder), uistate.T("documents.cadenceBtn")),
								If(cadenceMsg.Get() != "",
									Span(css.Class("text-up", tw.Text12), Attr("role", "status"),
										Attr("data-testid", "cadence-reminder-msg"), cadenceMsg.Get())),
							),
						),
					)),

					// Smart · Amazon order history (TX4) — local parse + charge matching.
					If(source.Get() == "amazon", Fragment(
						importFormHeader(false, uiw.Icon(icon.Box, css.Class(tw.W5, tw.H5)),
							uistate.T("orderimport.cardTitle"), uistate.T("orderimport.cardDesc")),
						ui.CreateElement(OrderImportCard, orderImportCardProps{}),
					)),

					// Smart+ · statement PDF (AI reads the file natively).
					If(source.Get() == "pdf", pdfCard),

					// Smart+ · receipt photo (vision AI).
					If(source.Get() == "receipt", Fragment(
						importFormHeader(true, uiw.Icon(icon.Receipt, css.Class(tw.W5, tw.H5)),
							uistate.T("documents.typeReceiptTitle"), uistate.T("documents.receiptFormDesc")),
						ImageImportCard(imageImportCardProps{
							ImageURL:  imageURL.Get(),
							AILoading: aiLoading.Get(),
							AIErr:     aiErr.Get(),
							NeedsKey:  needsKey.Get(),
							OnChoose:  chooseImage,
							OnReadAI:  readAI,
							Nav:       nav,
						}),
					)),

					// Column-map wizard + saved profiles belong to the statement-text flow.
					If(source.Get() == "stmt" && wizardVisible.Get(),
						Fragment(
							wizardCard(
								wizardHeader.Get(),
								wizardDate.Get(), wizardDescCol.Get(), wizardAmount.Get(),
								wizardDebit.Get(), wizardCredit.Get(),
								onWizardDate, onWizardDesc, onWizardAmount, onWizardDebit, onWizardCredit,
								applyWizard,
								profileName.Get(), onProfileName, saveProfile,
							),
							statementFormatControls(dateOrderChoice.Get(), numberFormatChoice.Get(),
								statementAmbiguousDates.Get(), statementAmbiguousNumbers.Get(), statementInvalidRows.Get(),
								onDateOrderChoice, onNumberFormatChoice, applyStatementFormats),
						),
					),
					If(source.Get() == "stmt" && statementFormatOptionsVisible.Get() && !inReview,
						statementFormatControls(dateOrderChoice.Get(), numberFormatChoice.Get(),
							statementAmbiguousDates.Get(), statementAmbiguousNumbers.Get(), statementInvalidRows.Get(),
							onDateOrderChoice, onNumberFormatChoice, applyStatementFormats)),
					If(source.Get() == "stmt" && (wizardVisible.Get() || showProfiles.Get()),
						savedProfilesCard(app, accounts, importAcct.Get(),
							showProfiles.Get(),
							func() { showProfiles.Set(!showProfiles.Get()) },
							func(sp importmap.SavedProfile) {
								dateOrderChoice.Set(string(sp.Profile.DateOrder))
								// Apply a saved profile to the current raw rows.
								rawRecs := wizardRawRows.Get()
								flat := make([][]string, 0)
								for _, g := range rawRecs {
									flat = append(flat, g...)
								}
								if len(flat) == 0 {
									msg.Set(uistate.T("tc4n6.profilePasteFirst"))
									return
								}
								dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
								sp.Profile.Decimals = dec
								stRows, rowErrors, rowWarnings := importmap.ApplyWithDiagnostics(sp.Profile, flat)
								if len(stRows) == 0 {
									msg.Set(uistate.T("tc4n6.profileNoRows"))
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
								setDraft(rows2)
								message := uistate.T("documents.stmtParsed", importCount(len(rows2), "row", "rows"))
								if len(rowErrors) > 0 {
									message += " " + csvSkipDetail(mapImportErrors(rowErrors))
								}
								if len(rowWarnings) > 0 {
									message += " " + importMapWarningSummary(rowWarnings)
								}
								msg.Set(message)
								rev.Set(rev.Get() + 1)
							},
							func(id string) {
								_ = app.DeleteImportProfile(id, activeImportProfileWorkspaceID())
								rev.Set(rev.Get() + 1)
							},
						),
					),
				),
			),
			// #57: recent import runs with their full results and per-run
			// roll-back (while the pre-import checkpoint is still in the ring).
			importHistorySection(app.Documents(), accounts, app.Transactions(), rollbackImport, goActivity),
		)),

		// STAGE 2 — "Review & import": the shared editable draft table + spend summary,
		// reached when any AI source yields rows. The footer Save commits; "← Add more
		// data" returns to the hub; "Past imports →" opens the Artifacts vault (the
		// replacement for the old inline import-history list).
		If(inReview, Fragment(
			Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Mb2),
				Button(css.Class("btn"), Type("button"), Attr("data-testid", "import-back-btn"),
					OnClick(goInput), uistate.T("documents.backToInput")),
				A(css.Class("btn", "btn-ghost"), Href(uistate.RoutePath("/artifacts")),
					Attr("data-testid", "import-artifacts-link"), OnClick(goArtifacts),
					uistate.T("documents.pastImportsLink"))),
			// C74 — Suggest categories button: shown when draft is non-empty.
			// Pass 1 (deterministic rules) runs free; Pass 2 (AI) only fires with a key.
			If(len(rows) > 0,
				Div(css.Class(tw.Mt2, tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "suggest-categories-btn"),
						OnClick(categorizeDraft), uistate.T("impc.suggestCategoriesAll")),
					If(aiLoading.Get(), Span(css.Class("muted"), uistate.T("documents.categorizing"))),
				),
			),
			If(source.Get() == "stmt" && statementRaw.Get() != "",
				Div(css.Class("card", tw.Mt2), Attr("data-testid", "statement-format-preview"),
					H3(css.Class("card-title"), uistate.T("impc.formatTitle")),
					P(css.Class("muted", tw.Text12), statementImportCounts(statementAmbiguousDates.Get(), statementAmbiguousNumbers.Get(), statementInvalidRows.Get())),
					Div(css.Class("form-grid"),
						Div(Label(For("statement-date-order"), uistate.T("impc.dateOrder")),
							Select(css.Class("field"), Attr("id", "statement-date-order"), Attr("data-testid", "statement-date-order"),
								Attr("aria-label", uistate.T("impc.dateOrder")), OnChange(onDateOrderChoice),
								Option(Value(string(statement.MonthDay)), SelectedIf(dateOrderChoice.Get() == string(statement.MonthDay)), uistate.T("impc.monthDay")),
								Option(Value(string(statement.DayMonth)), SelectedIf(dateOrderChoice.Get() == string(statement.DayMonth)), uistate.T("impc.dayMonth")))),
						Div(Label(For("statement-number-format"), uistate.T("impc.numberFormat")),
							Select(css.Class("field"), Attr("id", "statement-number-format"), Attr("data-testid", "statement-number-format"),
								Attr("aria-label", uistate.T("impc.numberFormat")), OnChange(onNumberFormatChoice),
								Option(Value("auto"), SelectedIf(numberFormatChoice.Get() == "auto"), uistate.T("impc.detectAutomatically")),
								Option(Value("dot"), SelectedIf(numberFormatChoice.Get() == "dot"), uistate.T("impc.dotDecimal")),
								Option(Value("comma"), SelectedIf(numberFormatChoice.Get() == "comma"), uistate.T("impc.commaDecimal")))),
					),
					Button(css.Class("btn"), Type("button"), Attr("data-testid", "statement-apply-formats"),
						OnClick(applyStatementFormats), uistate.T("impc.applyFormats")),
				),
			),
			// Review results sit below the import inputs that produce them (G14 §1): a
			// parsed statement / scanned receipt's draft rows no longer pop in *above* the
			// card the user just acted in.
			paginatedDraftReview(draftReviewProps{
				Rows:              rows,
				VisibleRows:       visibleRows,
				Start:             reviewStart,
				Page:              currentReviewPage,
				PageCount:         reviewPageCount,
				Accounts:          accounts,
				Categories:        app.Categories(),
				ReviewCur:         reviewCur,
				ImportAcctID:      importAcct.Get(),
				SeenSigs:          seenSigs,
				ReceiptMode:       receiptMode.Get(),
				ReceiptTotal:      receiptTotal.Get(),
				ReceiptMerchant:   receiptMerchant.Get(),
				RecBaseCur:        recBaseCur,
				ClearDraft:        clearDraft,
				Toggle:            receiptToggle,
				OnPage:            func(page int) { reviewPage.Set(page) },
				OnAcctChange:      onAcct,
				OnReceiptTotal:    onReceiptTotal,
				OnReceiptMerchant: onReceiptMerchant,
				OnImportDraft:     importDraft,
				OnImportReceipt:   importReceipt,
				SavingCheckpoint:  checkpointSaving.Get(),
				OnRemoveDraft:     removeDraft,
				OnUpdateDraft:     updateDraft,
			}),
			// #58: when the receipt's total matches an existing charge, lead with
			// the attach-to-existing offer — "Import" below stays the create-new path.
			If(receiptMode.Get() && len(receiptCands) > 0,
				receiptMatchOffer(receiptCands, attachReceipt)),
			// #57: pre-commit balance impact for the reviewed draft — where the
			// import account's balance lands if every row commits, with the
			// implausible-jump warning.
			reviewImpactCard(app, accounts, importAcct.Get(), rows, receiptMode.Get()),
			SpendSummaryCard(spendSummaryCardProps{
				Rows:         rows,
				Accounts:     accounts,
				ImportAcctID: importAcct.Get(),
				BaseCurrency: app.Settings().BaseCurrency,
			}),
		)),
	)
}

type draftReviewProps struct {
	Rows              []extract.Row
	VisibleRows       []extract.Row
	Start             int
	Page              int
	PageCount         int
	Accounts          []domain.Account
	Categories        []domain.Category
	ReviewCur         string
	ImportAcctID      string
	SeenSigs          map[string]bool
	ReceiptMode       bool
	ReceiptTotal      string
	ReceiptMerchant   string
	SavingCheckpoint  bool
	RecBaseCur        string
	ClearDraft        ui.Handler
	Toggle            ui.Node
	OnPage            func(int)
	OnAcctChange      ui.Handler
	OnReceiptTotal    ui.Handler
	OnReceiptMerchant ui.Handler
	OnImportDraft     ui.Handler
	OnImportReceipt   ui.Handler
	OnRemoveDraft     func(int)
	OnUpdateDraft     func(int, extract.Row)
}

// paginatedDraftReview renders only the selected page while calculating row
// counts, duplicate status, totals, and receipt reconciliation from the full
// draft. Row callbacks retain their absolute draft index across pages.
func paginatedDraftReview(props draftReviewProps) ui.Node {
	if len(props.Rows) == 0 {
		return nil
	}
	dupCount := draftDuplicateCount(props.Rows, props.SeenSigs)
	items := make([]ui.Node, 0, len(props.VisibleRows))
	for i, row := range props.Rows {
		sig := row.Signature()
		if i < props.Start || i >= props.Start+len(props.VisibleRows) {
			continue
		}
		dup := props.SeenSigs[sig]
		for earlier := 0; earlier < i && !dup; earlier++ {
			dup = props.Rows[earlier].Signature() == sig
		}
		items = append(items, ui.CreateElement(DraftRow, draftRowProps{
			Index:       i,
			Row:         row,
			Currency:    props.ReviewCur,
			Categories:  props.Categories,
			IsDuplicate: dup,
			ColorBySign: !props.ReceiptMode,
			OnRemove:    props.OnRemoveDraft,
			OnUpdate:    props.OnUpdateDraft,
		}))
	}
	start, end, _, _ := draftReviewWindow(len(props.Rows), props.Page)
	onPrev := ui.UseEvent(Prevent(func() { props.OnPage(props.Page - 1) }))
	onNext := ui.UseEvent(Prevent(func() { props.OnPage(props.Page + 1) }))
	pageControls := func() ui.Node {
		return Div(css.Class(tw.Flex, tw.ItemsCenter, tw.JustifyBetween, tw.Gap2, tw.Mt2, tw.Mb2),
			If(props.Page > 1, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "draft-page-prev"),
				OnClick(onPrev), uistate.T("impc.previousPage"))),
			Span(Attr("data-testid", "draft-page-range"), uistate.T("impc.pageRange", start+1, end, len(props.Rows))),
			If(props.Page < props.PageCount, Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "draft-page-next"),
				OnClick(onNext), uistate.T("impc.nextPage"))),
		)
	}
	var summary ui.Node = Fragment()
	if dupCount > 0 {
		summary = Div(css.Class("notice notice-warn"), Attr("data-testid", "draft-all-duplicate-count"),
			uistate.T("impc.duplicateCount", dupCount, len(props.Rows)))
	}
	clear := Button(css.Class("btn btn-sm"), Type("button"), Attr("data-testid", "draft-clear-all"),
		OnClick(props.ClearDraft), uistate.T("impc.clearAllRows"))
	var body ui.Node
	if props.ReceiptMode {
		recCur := props.RecBaseCur
		if recCur == "" {
			recCur = "USD"
		}
		dec := currency.Decimals(recCur)
		lines := make([]extract.ReceiptLine, 0, len(props.Rows))
		for _, row := range props.Rows {
			lines = append(lines, extract.ReceiptLine{Description: row.Description, Category: row.Category, Amount: absAmount(row.Amount)})
		}
		residual, err := (extract.Receipt{Total: absAmount(props.ReceiptTotal), Lines: lines}).Residual(dec)
		reconciled := err == nil && residual == 0
		var receiptStatus ui.Node = P(css.Class("muted"), uistate.T("documents.linesReconciled"))
		if err != nil {
			receiptStatus = P(css.Class("err"), Attr("role", "alert"), uistate.T("documents.linesUnreadable"))
		} else if !reconciled {
			receiptStatus = P(css.Class("err"), Attr("role", "alert"), uistate.T("documents.linesOffBy", fmtMoney(money.New(draftAbsInt64(residual), recCur))))
		}
		body = Fragment(
			Form(css.Class("form-grid"), OnSubmit(props.OnImportReceipt),
				Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("documents.storeName")),
					Placeholder(uistate.T("documents.storeNamePh")), OnInput(props.OnReceiptMerchant), uiw.FieldValue(props.ReceiptMerchant)),
				Input(css.Class("field"), Type("text"), Attr("aria-label", uistate.T("documents.receiptTotal")),
					Placeholder(uistate.T("documents.receiptTotal")), OnInput(props.OnReceiptTotal), uiw.FieldValue(props.ReceiptTotal)),
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.importAccount")), OnChange(props.OnAcctChange), draftAccountOptions(props.Accounts, props.ImportAcctID)),
				receiptStatus,
				buttonWithDisabled(!reconciled || props.SavingCheckpoint, []any{css.Class("btn btn-primary"), Type("submit")}, IfElse(props.SavingCheckpoint, Text(uistate.T("enc34.saving")), Text(uistate.T("documents.importReceipt")))),
			),
		)
	} else {
		dec := currency.Decimals(props.ReviewCur)
		var inMinor, outMinor int64
		seen := make(map[string]bool, len(props.Rows))
		for _, row := range props.Rows {
			sig := row.Signature()
			if props.SeenSigs[sig] || seen[sig] {
				seen[sig] = true
				continue
			}
			seen[sig] = true
			if minor, err := money.ParseMinor(strings.TrimSpace(row.Amount), dec); err == nil {
				if minor >= 0 {
					inMinor += minor
				} else {
					outMinor -= minor
				}
			}
		}
		body = Fragment(
			Div(css.Class("draft-summary"),
				draftSumItem(uistate.T("documents.sumIn"), fmtMoney(money.New(inMinor, props.ReviewCur)), "in"),
				draftSumItem(uistate.T("documents.sumOut"), fmtMoney(money.New(outMinor, props.ReviewCur)), "out"),
				draftSumNet(inMinor-outMinor, props.ReviewCur)),
			Form(css.Class("draft-actionrow"), OnSubmit(props.OnImportDraft),
				Select(css.Class("field"), Attr("aria-label", uistate.T("documents.importAccount")), OnChange(props.OnAcctChange), draftAccountOptions(props.Accounts, props.ImportAcctID)),
				buttonWithDisabled(props.SavingCheckpoint, []any{css.Class("btn btn-primary"), Type("submit"), Attr("data-testid", "draft-import-all")}, IfElse(props.SavingCheckpoint, Text(uistate.T("enc34.saving")), Text(uistate.T("impc.importAllRows", len(props.Rows))))),
			),
		)
	}
	return uiw.EntityListSection(uiw.EntityListSectionProps{
		Title:      uistate.T("documents.reviewTitle", importCount(len(props.Rows), "transaction", "transactions")),
		ClassParts: []any{"card-step-active"},
		Body: Fragment(summary,
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter), clear, props.Toggle),
			P(css.Class("muted"), uistate.T("documents.reviewDesc")),
			pageControls(),
			Div(css.Class("rows draft-ledger"), Attr("data-testid", "draft-review-page"), items),
			pageControls(), body,
		),
	})
}

func draftAccountOptions(accounts []domain.Account, selected string) []ui.Node {
	options := make([]ui.Node, 0, len(accounts))
	for _, account := range accounts {
		options = append(options, Option(Value(account.ID), SelectedIf(selected == account.ID), account.Name))
	}
	return options
}

func draftAbsInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
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
	categoryPickerOpen := ui.UseState(false)
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
	openCategoryPicker := ui.UseEvent(Prevent(func() { categoryPickerOpen.Set(true) }))
	chooseDraftCategory := func(categoryID string) {
		if app := appstate.Default; app != nil {
			for _, category := range app.Categories() {
				if category.ID == categoryID {
					r.Category = category.Name
					props.OnUpdate(props.Index, r)
					break
				}
			}
		}
		categoryPickerOpen.Set(false)
	}
	categoryKind := domain.KindExpense
	if amount, err := money.ParseMinor(strings.TrimSpace(r.Amount), currency.Decimals(props.Currency)); err == nil && amount > 0 {
		categoryKind = domain.KindIncome
	}
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
		return Div(css.Class("row-edit"), Attr("data-testid", "draft-review-row"),
			Form(css.Class("draft-edit-grid"), OnSubmit(saveEdit),
				Input(css.Class("field"), Attr("id", draftFieldID), Type("date"), OnInput(onDate), uiw.FieldValue(dateS.Get())),
				Input(css.Class("field"), Type("text"), Placeholder(uistate.T("documents.descPlaceholder")), OnInput(onDesc), uiw.FieldValue(descS.Get())),
				Input(css.Class("field fig"), Type("text"), Placeholder(uistate.T("documents.amountPlaceholder")), OnInput(onAmt), uiw.FieldValue(amtS.Get())),
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
			OnClick(openCategoryPicker), Span("+ "), uistate.T("documents.category"))
	}

	return Div(css.Class("row"), Attr("data-testid", "draft-review-row"),
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
		If(categoryPickerOpen.Get(), ui.CreateElement(CategoryPicker, CategoryPickerProps{
			Kind:    categoryKind,
			OnPick:  chooseDraftCategory,
			OnClose: func() { categoryPickerOpen.Set(false) },
		})),
	)
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
//
// It reports whether the picker was actually opened. A browser can refuse a
// programmatic click on a file input — most often when the call has drifted out of
// the user-activation window — and it does so by throwing, not by any signal the
// page can see afterwards. Callers that must never no-op silently (C564's receipt
// split) use the result to say so instead of leaving a dead click.
func pickImageDataURL(onData func(string), onErr func(string)) (opened bool) {
	defer func() {
		if r := recover(); r != nil {
			opened = false
		}
	}()
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
	// C623: the input is ATTACHED before it is clicked.
	//
	// It used to stay detached, on the reasoning that a detached input still opens
	// the dialog inside a user gesture and that a refusal would throw and be caught
	// as "not opened". Browsers do not cooperate with either half: a click on a
	// detached input is SILENTLY ignored, no exception is raised, and this function
	// then returned true having opened nothing. "Split from receipt…" therefore
	// posted "Choose a photo of the receipt to read.", opened no picker, reported no
	// error, and let its own toast expire — an action with no next step at all.
	//
	// Attaching it makes the click a real one. The element is hidden and removed as
	// soon as the dialog resolves, so the page CSS / focus-trap concern that
	// motivated the detached version does not get a chance to apply: it is never
	// focusable, never laid out, and never outlives the pick.
	input.Set("style", "position:fixed;left:-9999px;width:1px;height:1px;opacity:0")
	input.Set("tabindex", "-1")
	input.Set("aria-hidden", "true")
	body := doc.Get("body")
	attached := false
	if body.Truthy() {
		body.Call("appendChild", input)
		attached = true
	}
	// Remove the node once the browser is done with it, whichever way it ends.
	// "cancel" fires when the user dismisses the dialog; change covers the pick.
	if attached {
		var cleanup js.Func
		cleanup = js.FuncOf(func(js.Value, []js.Value) any {
			defer cleanup.Release()
			if p := input.Get("parentNode"); p.Truthy() {
				p.Call("removeChild", input)
			}
			return nil
		})
		input.Call("addEventListener", "cancel", cleanup)
		input.Call("addEventListener", "change", cleanup)
	}
	input.Call("click")
	return true
}

// ---------------------------------------------------------------------------
// C74 — import-map wizard helpers
// ---------------------------------------------------------------------------

// extractRawHeaderAndRows parses delimited text with the same quoted-field rules
// as the statement importer. The outer slice wraps each record in its own
// single-element slice so it can be stored in ui state and flattened for Apply.
func extractRawHeaderAndRows(text string, delim rune) (header []string, rows [][][]string) {
	if delim == 0 {
		firstLine := text
		if i := strings.IndexAny(text, "\r\n"); i >= 0 {
			firstLine = text[:i]
		}
		delim = statement.DetectDelimiter(firstLine)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(text, "\ufeff")))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil || len(records) == 0 {
		return nil, nil
	}
	header = records[0]
	for _, record := range records[1:] {
		rows = append(rows, [][]string{record})
	}
	return header, rows
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
						OnInput(onProfileName), uiw.FieldValue(profileNameVal)),
					Button(css.Class("btn"), Type("submit"), uistate.T("documents.profileSave")),
				),
			),
		),
	})
}

func statementFormatControls(dateOrder, numberFormat string, ambiguousDates, ambiguousNumbers, invalidRows int, onDateOrder, onNumberFormat, onApply ui.Handler) ui.Node {
	return Div(css.Class("card", tw.Mt2), Attr("data-testid", "statement-format-preview"),
		H3(css.Class("card-title"), uistate.T("impc.formatTitle")),
		P(css.Class("muted", tw.Text12), statementImportCounts(ambiguousDates, ambiguousNumbers, invalidRows)),
		Div(css.Class("form-grid"),
			Div(Label(For("statement-date-order"), uistate.T("impc.dateOrder")),
				Select(css.Class("field"), Attr("id", "statement-date-order"), Attr("data-testid", "statement-date-order"),
					Attr("aria-label", uistate.T("impc.dateOrder")), OnChange(onDateOrder),
					Option(Value(string(statement.MonthDay)), SelectedIf(dateOrder == string(statement.MonthDay)), uistate.T("impc.monthDay")),
					Option(Value(string(statement.DayMonth)), SelectedIf(dateOrder == string(statement.DayMonth)), uistate.T("impc.dayMonth")))),
			Div(Label(For("statement-number-format"), uistate.T("impc.numberFormat")),
				Select(css.Class("field"), Attr("id", "statement-number-format"), Attr("data-testid", "statement-number-format"),
					Attr("aria-label", uistate.T("impc.numberFormat")), OnChange(onNumberFormat),
					Option(Value("auto"), SelectedIf(numberFormat == "auto"), uistate.T("impc.detectAutomatically")),
					Option(Value("dot"), SelectedIf(numberFormat == "dot"), uistate.T("impc.dotDecimal")),
					Option(Value("comma"), SelectedIf(numberFormat == "comma"), uistate.T("impc.commaDecimal")))),
		),
		Button(css.Class("btn"), Type("button"), Attr("data-testid", "statement-apply-formats"),
			OnClick(onApply), uistate.T("impc.applyFormats")),
	)
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
	profiles := app.ImportProfiles(activeImportProfileWorkspaceID())
	return uiw.Card(uiw.CardProps{
		Header: Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
			H2(css.Class("card-title"), uistate.T("documents.profileLoadTitle")),
			Button(css.Class("btn btn-sm"), Type("button"), OnClick(func() { onToggle() }),
				IfElse(shown, Text(uistate.T("common.hide")), Text(uistate.T("common.show")))),
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

// activeImportProfileWorkspaceID reads the workspace registry used by the app
// shell, so saved mappings follow the same active dataset as the Documents page.
func activeImportProfileWorkspaceID() string {
	raw, ok := browserstore.Get("cashflux:workspaces")
	if !ok || raw == "" {
		return ""
	}
	var registry workspace.Registry
	if err := json.Unmarshal([]byte(raw), &registry); err != nil {
		return ""
	}
	if active, ok := registry.Active(); ok {
		return active.ID
	}
	return registry.ActiveID
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
