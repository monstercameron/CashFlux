//go:build js && wasm

package screens

import (
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/id"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
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
			msg.Set(uistate.T("documents.csvError", err.Error()))
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
	aiModel := settings.OpenAIModel
	if aiModel == "" || aiModel == "gpt-4o-mini" {
		aiModel = "gpt-4o" // vision needs a vision-capable model
	}
	readAI := ui.UseEvent(func() {
		if settings.OpenAIKey == "" {
			aiErr.Set(uistate.T("documents.needKey"))
			return
		}
		if imageURL.Get() == "" {
			aiErr.Set(uistate.T("documents.chooseImageFirst"))
			return
		}
		aiLoading.Set(true)
		aiErr.Set("")
		ai.SendStructuredVisionChat(settings.OpenAIKey, ai.DefaultBaseURL, aiModel, visionSystemPrompt,
			"Extract every transaction you can read from this image.", imageURL.Get(), 0.1,
			"transactions", []byte(visionExtractionSchema),
			func(content string, _ ai.Usage) {
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
			},
			func(e string) { aiLoading.Set(false); aiErr.Set(e) },
		)
	})

	importDraft := ui.UseEvent(Prevent(func() {
		rows := draft.Get()
		acc, ok := accByIDFrom(accounts, importAcct.Get())
		if !ok {
			aiErr.Set(uistate.T("documents.chooseAccount"))
			return
		}
		dec := currency.Decimals(acc.Currency)
		// Skip rows that already exist in the chosen account (same date + amount).
		seen := map[string]bool{}
		for _, t := range app.Transactions() {
			if t.AccountID != acc.ID {
				continue
			}
			sig := extract.Row{Date: dateutil.FormatDate(t.Date), Amount: money.FormatMinor(t.Amount.Amount, dec)}.Signature()
			seen[sig] = true
		}
		fresh := extract.FilterNew(rows, seen)
		skipped := len(rows) - len(fresh)
		rows = fresh

		// Category resolution: prefer the row's own category (matched by name); when
		// it's missing or unknown, fall back to the user's saved rules + implicit
		// category-name rules against the description (a rule can add tags too).
		catByName := map[string]string{}
		autoRules := app.Rules()
		for _, c := range app.Categories() {
			catByName[strings.ToLower(c.Name)] = c.ID
			autoRules = append(autoRules, rules.Rule{Match: c.Name, SetCategoryID: c.ID})
		}
		n := 0
		for _, r := range rows {
			amt, err := money.ParseMinor(strings.TrimSpace(r.Amount), dec)
			if err != nil || amt == 0 {
				continue
			}
			date, derr := dateutil.ParseDate(strings.TrimSpace(r.Date))
			if derr != nil {
				date = time.Now()
			}
			desc := strings.TrimSpace(r.Description)
			cid := catByName[strings.ToLower(r.Category)]
			var tags []string
			if cid == "" {
				if mr := rules.FirstMatch(autoRules, desc); mr != nil {
					cid = mr.SetCategoryID
					tags = mr.SetTags
				}
			}
			t := domain.Transaction{
				ID: id.New(), AccountID: acc.ID, Date: date, Desc: desc,
				CategoryID: cid, Tags: tags, Amount: money.New(amt, acc.Currency),
			}
			if err := app.PutTransaction(t); err == nil {
				n++
			}
		}
		if n > 0 {
			recordDocument(domain.DocImage, acc.ID, rows)
		}
		draft.Set([]extract.Row{})
		imageURL.Set("")
		aiErr.Set("")
		summary := uistate.T("documents.importedImage", plural(n, "transaction"))
		if skipped > 0 {
			summary += uistate.T("documents.skipped", plural(skipped, "duplicate"))
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
	draftBody := ui.Node(nil)
	if len(rows) > 0 {
		items := make([]ui.Node, 0, len(rows))
		for i, r := range rows {
			items = append(items, ui.CreateElement(DraftRow, draftRowProps{Index: i, Row: r, OnRemove: removeDraft, OnUpdate: updateDraft}))
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
					if a, ok := accByIDFrom(accounts, d.AccountID); ok {
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
				Button(Class("btn btn-primary"), Type("button"), OnClick(readAI), IfElse(aiLoading.Get(), Text(uistate.T("documents.reading")), Text(uistate.T("documents.readAI")))),
			),
			If(aiErr.Get() != "", P(Class("err"), aiErr.Get())),
		),
		draftBody,
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

	if editing.Get() {
		return Div(Class("row"),
			Form(Class("form-grid"), OnSubmit(saveEdit),
				Input(Class("field"), Type("date"), Value(dateS.Get()), OnInput(onDate)),
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
	return Div(Class("row"),
		Div(Class("row-main"),
			Span(Class("row-desc"), firstNonEmpty(r.Description, uistate.T("documents.noDescription"))),
			Span(Class("row-meta"), meta),
		),
		Span(Class("amount fig"), r.Amount),
		Button(Class("btn"), Type("button"), Title(uistate.T("documents.editRow")), OnClick(startEdit), uistate.T("action.edit")),
		Button(Class("btn-del"), Type("button"), Title(uistate.T("documents.removeRow")), OnClick(rm), "✕"),
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
			Span(Class("row-desc"), firstNonEmpty(d.Filename, docKindLabel(d.Kind))),
			Span(Class("row-meta"), meta),
		),
		Button(Class("btn-del"), Type("button"), Title(uistate.T("documents.deleteHistTitle")), OnClick(del), "✕"),
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

// accByIDFrom finds an account by id in a slice.
func accByIDFrom(accounts []domain.Account, id string) (domain.Account, bool) {
	for _, a := range accounts {
		if a.ID == id {
			return a, true
		}
	}
	return domain.Account{}, false
}

// firstNonEmpty returns a if non-empty, else b.
func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
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
