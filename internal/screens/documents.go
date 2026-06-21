//go:build js && wasm

package screens

import (
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
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/spendsummary"
	"github.com/monstercameron/CashFlux/internal/statement"
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
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

// Documents imports transactions two ways: paste CSV (no AI), or read a receipt/
// statement image with the OpenAI vision model (bring-your-own-key). Extracted
// rows are reviewed, then imported into a chosen account through the validated
// write path.
func Documents() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}

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
	draft := ui.UseState([]extract.Row{})
	importAcct := ui.UseState(defaultAcc)
	receiptMode := ui.UseState(false)  // import the draft as ONE split transaction
	receiptTotal := ui.UseState("")    // the receipt's single total (defaults to the line sum)
	receiptMerchant := ui.UseState("") // optional store name

	onCsv := ui.UseEvent(func(v string) { csvText.Set(v) })
	onStmt := ui.UseEvent(func(v string) { stmtText.Set(v) })
	onAcct := ui.UseEvent(func(e ui.Event) { importAcct.Set(e.GetValue()) })
	onReceiptTotal := ui.UseEvent(func(v string) { receiptTotal.Set(v) })
	onReceiptMerchant := ui.UseEvent(func(v string) { receiptMerchant.Set(v) })

	// recordDocument saves a best-effort audit record of an import (logged by
	// appstate on failure). Declared before the import handlers that call it.
	recordDocument := func(kind domain.DocumentKind, accountID string, rows []extract.Row) {
		_ = app.PutDocument(domain.Document{
			ID: id.New(), Kind: kind, UploadedAt: time.Now(), AccountID: accountID,
			Status: domain.DocImported, Extracted: toDocumentRows(rows),
		})
	}

	importCSV := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(csvText.Get())
		if data == "" {
			msg.Set(uistate.T("documents.csvEmpty"))
			return
		}
		n, err := app.ImportTransactionsCSV([]byte(data))
		if err != nil {
			// Don't surface the internal "store:" package prefix to the user (C27).
			friendly := strings.TrimPrefix(err.Error(), "store: ")
			msg.Set(uistate.T("documents.csvError", friendly))
			return
		}
		if n > 0 {
			recordDocument(domain.DocCSV, "", nil)
		}
		msg.Set(uistate.T("documents.importedCsv", plural(n, "transaction")))
		rev.Set(rev.Get() + 1)
	}))

	// parseStatement (C74) parses a pasted bank/credit-card statement in any common
	// delimited format — varying column orders/labels, signed-amount or separate
	// debit/credit columns — into draft rows. It guesses the column mapping
	// automatically (statement.MapColumns) and feeds the result into the same review
	// → dedupe → import pipeline the image/CSV paths use, so the user can edit, drop,
	// and commit rows (duplicates are skipped on import). Per-row parse failures are
	// reported, not fatal.
	parseStatement := ui.UseEvent(Prevent(func() {
		data := strings.TrimSpace(stmtText.Get())
		if data == "" {
			msg.Set(uistate.T("documents.stmtEmpty"))
			return
		}
		// Parse amounts at the import account's currency precision.
		dec := currency.Decimals(reviewCurrencyFor(app, accounts, importAcct.Get()))
		st, err := statement.Parse(data, dec)
		if err != nil {
			msg.Set(uistate.T("documents.stmtError", strings.TrimPrefix(err.Error(), "statement: ")))
			return
		}
		rows := make([]extract.Row, 0, len(st.Rows))
		for _, r := range st.Rows {
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
		draft.Set(rows)
		if n := len(st.Errors); n > 0 {
			msg.Set(uistate.T("documents.stmtParsedWithErrors", plural(len(rows), "row"), plural(n, "row")))
		} else {
			msg.Set(uistate.T("documents.stmtParsed", plural(len(rows), "row")))
		}
		rev.Set(rev.Get() + 1)
	}))

	chooseImage := ui.UseEvent(func() {
		pickImageDataURL(func(u string) {
			imageURL.Set(u)
			aiErr.Set("")
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
			aiErr.Set(uistate.T("documents.needKey"))
			return
		}
		if imageURL.Get() == "" {
			aiErr.Set(uistate.T("documents.chooseImageFirst"))
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
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
	draftBody := ui.Node(nil)
	if len(rows) > 0 {
		items := make([]ui.Node, 0, len(rows))
		draftCats := app.Categories()
		for i, r := range rows {
			items = append(items, ui.CreateElement(DraftRow, draftRowProps{Index: i, Row: r, Currency: reviewCur, Categories: draftCats, OnRemove: removeDraft, OnUpdate: updateDraft}))
		}
		acctOptions := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			acctOptions = append(acctOptions, Option(Value(a.ID), SelectedIf(importAcct.Get() == a.ID), a.Name))
		}

		// Receipt mode imports the lines as ONE transaction split across categories
		// (so a grocery receipt counts once against the card charge). The receipt
		// math runs in the base currency, matching appstate.ImportReceipt.
		recCur := app.Settings().BaseCurrency
		if recCur == "" {
			recCur = "USD"
		}
		recDec := currency.Decimals(recCur)
		toggle := uiw.ToggleRow(uiw.ToggleRowProps{
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

		var footer ui.Node
		if receiptMode.Get() {
			recLines := make([]extract.ReceiptLine, 0, len(rows))
			for _, r := range rows {
				recLines = append(recLines, extract.ReceiptLine{Description: r.Description, Category: r.Category, Amount: absAmount(r.Amount)})
			}
			resid, residErr := (extract.Receipt{Total: absAmount(receiptTotal.Get()), Lines: recLines}).Residual(recDec)
			reconciled := residErr == nil && resid == 0
			var remainderLine ui.Node
			switch {
			case reconciled:
				remainderLine = P(css.Class("muted"), "Lines add up to the total — ready to import as one transaction.")
			case residErr != nil:
				remainderLine = P(css.Class("err"), Attr("role", "alert"), "Check the amounts — one couldn't be read as a number.")
			default:
				off := resid
				if off < 0 {
					off = -off
				}
				remainderLine = P(css.Class("err"), Attr("role", "alert"), "Lines are off from the total by "+fmtMoney(money.New(off, recCur))+" — adjust the lines or the total to import.")
			}
			importBtn := []any{css.Class("btn btn-primary"), Type("submit")}
			if !reconciled {
				importBtn = append(importBtn, Attr("disabled", "disabled"))
			}
			importBtn = append(importBtn, "Import receipt")
			footer = Div(
				Div(css.Class("form-grid"),
					Input(css.Class("field"), Type("text"), Attr("aria-label", "Store name (optional)"), Placeholder("Store name (optional)"), Value(receiptMerchant.Get()), OnInput(onReceiptMerchant)),
					Input(css.Class("field"), Type("text"), Attr("aria-label", "Receipt total"), Placeholder("Receipt total"), Value(receiptTotal.Get()), OnInput(onReceiptTotal)),
				),
				remainderLine,
				Form(css.Class("form-grid"), OnSubmit(importReceipt),
					Select(css.Class("field"), Attr("aria-label", "Import into account"), OnChange(onAcct), acctOptions),
					Button(importBtn...),
				),
			)
		} else {
			footer = Form(css.Class("form-grid"), OnSubmit(importDraft),
				Select(css.Class("field"), Attr("aria-label", "Import into account"), OnChange(onAcct), acctOptions),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.importThese")),
			)
		}

		draftBody = Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("documents.reviewTitle", plural(len(rows), "transaction"))),
			P(css.Class("muted"), uistate.T("documents.reviewDesc")),
			toggle,
			Div(css.Class("rows"), items),
			footer,
		)
	}

	// Monthly-spend summary of the rows awaiting import: out vs in vs net per
	// month, so the user sees what a statement says they spent before committing
	// any rows. Amounts read at the chosen account's currency precision.
	summaryBody := ui.Node(nil)
	if len(rows) > 0 {
		cur := app.Settings().BaseCurrency
		if cur == "" {
			cur = "USD"
		}
		if acc, ok := domain.AccountByID(accounts, importAcct.Get()); ok && acc.Currency != "" {
			cur = acc.Currency
		}
		months := spendsummary.Summarize(rows, currency.Decimals(cur))
		sumRows := make([]ui.Node, 0, len(months))
		for _, m := range months {
			label := m.Month
			if label == "" {
				label = uistate.T("documents.summaryUndated")
			}
			sumRows = append(sumRows, Div(css.Class("row"),
				Span(css.Class("row-desc"), label),
				Span(css.Class("muted"), plural(m.Count, "row")),
				Span(css.Class("amount fig"), uistate.T("documents.summaryOutIn",
					fmtMoney(money.New(m.Out, cur)), fmtMoney(money.New(m.In, cur)), fmtMoney(money.New(m.Net(), cur)))),
			))
		}
		summaryBody = Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("documents.summaryTitle")),
			P(css.Class("muted"), uistate.T("documents.summaryDesc")),
			Div(css.Class("rows"), sumRows),
		)
	}

	// Import history: every recorded document, newest first.
	deleteDoc := func(docID string) {
		_ = app.DeleteDocument(docID)
		rev.Set(rev.Get() + 1)
	}
	docs := app.Documents()
	sort.Slice(docs, func(i, j int) bool { return docs[i].UploadedAt.After(docs[j].UploadedAt) })
	historyCard := Section(css.Class("card"),
		H2(css.Class("card-title"), uistate.T("documents.historyTitle")),
		IfElse(len(docs) == 0,
			P(css.Class("empty"), uistate.T("documents.historyEmpty")),
			Div(css.Class("rows"), MapKeyed(docs,
				func(d domain.Document) any { return d.ID },
				func(d domain.Document) ui.Node {
					name := ""
					if a, ok := domain.AccountByID(accounts, d.AccountID); ok {
						name = a.Name
					}
					return ui.CreateElement(DocHistoryRow, docHistoryRowProps{Doc: d, AccountName: name, OnDelete: deleteDoc})
				},
			)),
		),
	)

	return Div(
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("documents.imageTitle")),
			P(css.Class("muted"), uistate.T("documents.imageDesc")),
			Div(css.Class(tw.Flex, tw.FlexWrap, tw.Gap2, tw.ItemsCenter),
				Button(css.Class("btn"), Type("button"), OnClick(chooseImage), uistate.T("documents.chooseImage")),
				If(imageURL.Get() != "", Span(css.Class("muted"), uistate.T("documents.imageReady"))),
				Button(css.Class("btn btn-primary", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), OnClick(readAI), uiw.Icon(icon.Sparkles, css.Class("shrink-0", tw.W4, tw.H4)), IfElse(aiLoading.Get(), Text(uistate.T("documents.reading")), Text(uistate.T("documents.readAI")))),
			),
			If(aiErr.Get() != "", P(css.Class("err"), Attr("role", "alert"), aiErr.Get())),
		),
		draftBody,
		summaryBody,
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("documents.stmtTitle")),
			P(css.Class("muted"), uistate.T("documents.stmtDesc")),
			Form(OnSubmit(parseStatement),
				Textarea(css.Class("field field-wide"), Attr("rows", "8"),
					Placeholder("Posting Date,Description,Debit,Credit\n06/01/2026,SALARY ACH,,4200.00\n06/02/2026,WHOLE FOODS,86.40,"),
					OnInput(onStmt),
				),
				Div(Style(map[string]string{"margin-top": "0.6rem"}),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.stmtParse")),
				),
			),
		),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("documents.csvTitle")),
			P(css.Class("muted"), uistate.T("documents.csvDesc")),
			Form(OnSubmit(importCSV),
				Textarea(css.Class("field field-wide"), Attr("rows", "8"),
					Placeholder("date,payee,amount,account\n2026-06-01,Salary,4200.00,Checking\n2026-06-02,Groceries,-86.40,Checking"),
					OnInput(onCsv),
				),
				Div(Style(map[string]string{"margin-top": "0.6rem"}),
					Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("documents.import")),
				),
			),
			If(msg.Get() != "", P(css.Class("muted"), msg.Get())),
		),
		historyCard,
	)
}

type draftRowProps struct {
	Index      int
	Row        extract.Row
	Currency   string            // for accounting-formatting the review amount (C27)
	Categories []domain.Category // existing categories, so editing picks a real one (C60)
	OnRemove   func(int)
	OnUpdate   func(int, extract.Row)
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
		Span(css.Class("amount fig"), amtText),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"), Title(uistate.T("documents.editRow")), OnClick(startEdit), uiw.Icon(icon.Pencil, css.Class("shrink-0", tw.W4, tw.H4)), Span(uistate.T("action.edit"))),
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
