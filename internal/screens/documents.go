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
	"github.com/monstercameron/CashFlux/internal/textutil"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
		return Section(Class("card"), P(Class("empty"), uistate.T("common.notReady")))
	}

	rev := state.UseAtom("rev:documents", 0)
	csvText := ui.UseState("")
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

	onCsv := ui.UseEvent(func(v string) { csvText.Set(v) })
	onAcct := ui.UseEvent(func(e ui.Event) { importAcct.Set(e.GetValue()) })

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

	chooseImage := ui.UseEvent(func() {
		pickImageDataURL(func(u string) {
			imageURL.Set(u)
			aiErr.Set("")
			draft.Set([]extract.Row{})
		})
	})

	settings := app.Settings()
	pr := uistate.UsePrefs().Get().Normalize()
	useBackendAI := strings.TrimSpace(pr.ServerURL) != "" && strings.TrimSpace(pr.ServerToken) != ""
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
		for i, r := range rows {
			items = append(items, ui.CreateElement(DraftRow, draftRowProps{Index: i, Row: r, Currency: reviewCur, OnRemove: removeDraft, OnUpdate: updateDraft}))
		}
		acctOptions := make([]ui.Node, 0, len(accounts))
		for _, a := range accounts {
			acctOptions = append(acctOptions, Option(Value(a.ID), SelectedIf(importAcct.Get() == a.ID), a.Name))
		}
		draftBody = Section(Class("card"),
			H2(Class("card-title"), uistate.T("documents.reviewTitle", plural(len(rows), "transaction"))),
			P(Class("muted"), uistate.T("documents.reviewDesc")),
			Div(Class("rows"), items),
			Form(Class("form-grid"), OnSubmit(importDraft),
				Select(Class("field"), OnChange(onAcct), acctOptions),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("documents.importThese")),
			),
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
			sumRows = append(sumRows, Div(Class("row"),
				Span(Class("row-desc"), label),
				Span(Class("muted"), plural(m.Count, "row")),
				Span(Class("amount fig"), uistate.T("documents.summaryOutIn",
					fmtMoney(money.New(m.Out, cur)), fmtMoney(money.New(m.In, cur)), fmtMoney(money.New(m.Net(), cur)))),
			))
		}
		summaryBody = Section(Class("card"),
			H2(Class("card-title"), uistate.T("documents.summaryTitle")),
			P(Class("muted"), uistate.T("documents.summaryDesc")),
			Div(Class("rows"), sumRows),
		)
	}

	// Import history: every recorded document, newest first.
	deleteDoc := func(docID string) {
		_ = app.DeleteDocument(docID)
		rev.Set(rev.Get() + 1)
	}
	docs := app.Documents()
	sort.Slice(docs, func(i, j int) bool { return docs[i].UploadedAt.After(docs[j].UploadedAt) })
	historyCard := Section(Class("card"),
		H2(Class("card-title"), uistate.T("documents.historyTitle")),
		IfElse(len(docs) == 0,
			P(Class("empty"), uistate.T("documents.historyEmpty")),
			Div(Class("rows"), MapKeyed(docs,
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
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("documents.imageTitle")),
			P(Class("muted"), uistate.T("documents.imageDesc")),
			Div(Class("flex flex-wrap gap-2 items-center"),
				Button(Class("btn"), Type("button"), OnClick(chooseImage), uistate.T("documents.chooseImage")),
				If(imageURL.Get() != "", Span(Class("muted"), uistate.T("documents.imageReady"))),
				Button(Class("btn btn-primary inline-flex items-center gap-1.5"), Type("button"), OnClick(readAI), uiw.Icon(icon.Sparkles, Class("w-4 h-4 shrink-0")), IfElse(aiLoading.Get(), Text(uistate.T("documents.reading")), Text(uistate.T("documents.readAI")))),
			),
			If(aiErr.Get() != "", P(Class("err"), Attr("role", "alert"), aiErr.Get())),
		),
		draftBody,
		summaryBody,
		Section(Class("card"),
			H2(Class("card-title"), uistate.T("documents.csvTitle")),
			P(Class("muted"), uistate.T("documents.csvDesc")),
			Form(OnSubmit(importCSV),
				Textarea(Class("field field-wide"), Attr("rows", "8"),
					Placeholder("date,payee,amount,account\n2026-06-01,Salary,4200.00,Checking\n2026-06-02,Groceries,-86.40,Checking"),
					OnInput(onCsv),
				),
				Div(Style(map[string]string{"margin-top": "0.6rem"}),
					Button(Class("btn btn-primary"), Type("submit"), uistate.T("documents.import")),
				),
			),
			If(msg.Get() != "", P(Class("muted"), msg.Get())),
		),
		historyCard,
	)
}

type draftRowProps struct {
	Index    int
	Row      extract.Row
	Currency string // for accounting-formatting the review amount (C27)
	OnRemove func(int)
	OnUpdate func(int, extract.Row)
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
	onCat := ui.UseEvent(func(v string) { catS.Set(v) })
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
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Attr("id", draftFieldID), Type("date"), Value(dateS.Get()), OnInput(onDate)),
				Input(Class("field"), Type("text"), Placeholder(uistate.T("documents.descPlaceholder")), Value(descS.Get()), OnInput(onDesc)),
				Input(Class("field"), Type("text"), Placeholder(uistate.T("documents.amountPlaceholder")), Value(amtS.Get()), OnInput(onAmt)),
				Input(Class("field"), Type("text"), Placeholder(uistate.T("documents.categoryPlaceholder")), Value(catS.Get()), OnInput(onCat)),
				Button(Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(Class("btn"), Type("button"), OnClick(cancelEdit), uistate.T("action.cancel")),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), textutil.FirstNonEmpty(r.Description, uistate.T("documents.noDescription"))),
			Span(Class("row-meta"), meta),
		),
		Span(Class("amount fig"), amtText),
		Button(Class("btn inline-flex items-center gap-1.5"), Type("button"), Title(uistate.T("documents.editRow")), OnClick(startEdit), uiw.Icon(icon.Pencil, Class("w-4 h-4 shrink-0")), Span(uistate.T("action.edit"))),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("documents.removeRow")), Title(uistate.T("documents.removeRow")), OnClick(rm), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), textutil.FirstNonEmpty(d.Filename, docKindLabel(d.Kind))),
			Span(Class("row-meta"), meta),
		),
		Button(Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("documents.deleteHistTitle")), Title(uistate.T("documents.deleteHistTitle")), OnClick(del), uiw.Icon(icon.Close, Class("w-4 h-4"))),
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

// pickImageDataURL opens a file picker for images and calls onData with the
// chosen file as a base64 data: URL. The data never leaves the device except to
// OpenAI when the user clicks Read.
func pickImageDataURL(onData func(string)) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", "image/*")

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
