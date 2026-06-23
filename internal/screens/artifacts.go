//go:build js && wasm

package screens

import (
	"encoding/base64"
	"fmt"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/artifactstore"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/css"
	. "github.com/monstercameron/GoWebComponents/html/shorthand"
	"github.com/monstercameron/GoWebComponents/ui"
)

// Artifacts is the manager for user-stored assets: upload an image or import a
// CSV dataset, see them listed with their size, and delete them. A storage meter
// shows how much of the browser's local storage the whole dataset is using, since
// artifact bytes live in that single autosaved blob. Custom-page Image and Table
// widgets reference these by id.
func Artifacts() ui.Node {
	app := appstate.Default
	if app == nil {
		return Section(css.Class("card"), P(css.Class("empty"), uistate.T("common.notReady")))
	}
	rev := ui.UseState(0)
	_ = rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }
	// Surface upload/save failures instead of swallowing them — a large image can
	// blow the localStorage quota (the whole dataset is one blob), and silently
	// dropping the file leaves the user confused (C66, reliability). The real error
	// text (e.g. the quota message) is shown so the cause is clear.
	notice := uistate.UseNotice()
	notify := func(text string) { notice.Set(notice.Get().With(text, true)) }

	uploadImage := func() {
		pickFile("image/*", func(name, mime string, data []byte) {
			if len(data) == 0 {
				return
			}
			art := domain.Artifact{
				ID: id.New(), Name: name, Kind: artifacts.KindImage, MIME: mime,
				Bytes: data, CreatedAt: time.Now(),
			}
			art.Size = artifacts.Size(art)
			if err := app.PutArtifact(art); err != nil {
				notify(err.Error())
				return
			}
			refresh()
		})
	}
	importCSV := func() {
		pickFile(".csv,text/csv", func(name, _ string, data []byte) {
			cols, rows, err := artifacts.ParseCSV(data)
			if err != nil {
				notify(err.Error())
				return
			}
			art := domain.Artifact{
				ID: id.New(), Name: name, Kind: artifacts.KindCSV,
				Columns: cols, Rows: rows, CreatedAt: time.Now(),
			}
			art.Size = artifacts.Size(art)
			if err := app.PutArtifact(art); err != nil {
				notify(err.Error())
				return
			}
			refresh()
		})
	}

	// Count how many transactions reference each artifact (L29), so each row can
	// show "Referenced by N transaction(s)".
	refCount := map[string]int{}
	for _, t := range app.Transactions() {
		for _, att := range t.Attachments {
			refCount[att.ArtifactID]++
		}
	}
	// Count how many custom-page widgets bind each artifact (C66), so the delete
	// guard can warn before breaking a page that uses this image or dataset.
	pageRefCount := map[string]int{}
	for _, pg := range app.CustomPages() {
		for _, w := range pg.Widgets {
			if w.Binding.ArtifactID != "" {
				pageRefCount[w.Binding.ArtifactID]++
			}
		}
	}
	list := app.Artifacts()
	var rows []ui.Node
	for _, a := range list {
		rows = append(rows, ui.CreateElement(artifactRow, artifactRowProps{
			Artifact: a, Refresh: refresh,
			ReferencedBy: refCount[a.ID],
			UsedByPages:  pageRefCount[a.ID],
			Notice:       notify,
		}))
	}
	listBody := P(css.Class("empty"), uistate.T("artifacts.empty"))
	if len(rows) > 0 {
		listBody = Div(css.Class("rows"), rows)
	}

	// Storage meter: combined localStorage dataset size + IndexedDB blob bytes.
	// Use DatasetBytesWithBlobs so both storage locations are counted, but do NOT
	// call any IDB Get in the render path — that would block the single-threaded
	// wasm runtime waiting for a JS callback that can never fire.
	total := app.DatasetBytesWithBlobs()
	blobUsage := app.BlobStoreUsage()

	// Quota nudge: shown once when IndexedDB usage is near the recommended cap.
	// Dismissed by the user via component state for the session; a page refresh
	// re-evaluates usage and may not show again if usage has dropped.
	quotaDismissed := ui.UseState(false)
	var quotaNudge ui.Node = Fragment()
	if blobUsage > 0 && artifactstore.NearLimit(blobUsage) && !quotaDismissed.Get() {
		quotaNudge = Div(css.Class("notice notice-warn", tw.Mt2, tw.Flex, tw.ItemsCenter, tw.Gap2),
			Span(uistate.T("artifacts.quotaWarn", artifacts.HumanSize(int(blobUsage)))),
			Button(css.Class("btn btn-sm"), Type("button"),
				OnClick(func() { quotaDismissed.Set(true) }),
				uistate.T("action.dismiss"),
			),
		)
	}

	storageLabel := uistate.T("artifacts.storage", artifacts.HumanSize(total))
	if blobUsage > 0 {
		storageLabel = uistate.T("artifacts.storageIDB",
			artifacts.HumanSize(int(blobUsage)),
			artifacts.HumanSize(total),
		)
	}

	return Div(
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("artifacts.uploadTitle")),
			P(css.Class("muted"), uistate.T("artifacts.uploadDesc")),
			Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap),
				Button(css.Class("btn btn-primary"), Type("button"), OnClick(uploadImage), uistate.T("artifacts.uploadImage")),
				Button(css.Class("btn"), Type("button"), OnClick(importCSV), uistate.T("artifacts.importCSV")),
			),
			P(css.Class("muted", tw.Mt2), storageLabel),
			artifactStorageBar(total, blobUsage),
			quotaNudge,
		),
		Section(css.Class("card"),
			H2(css.Class("card-title"), uistate.T("artifacts.listTitle")),
			listBody,
		),
	)
}

type artifactRowProps struct {
	Artifact     domain.Artifact
	Refresh      func()
	ReferencedBy int          // how many transactions attach this artifact (L29)
	UsedByPages  int          // how many custom-page widgets reference this artifact (C66)
	Notice       func(string) // surface errors to the screen-level notice
}

// artifactRow is one artifact entry with rename and delete actions — its own
// component so its hooks stay stable across the list. Rename is inline (no
// modal required for a single text field). Delete is guarded when the artifact
// is referenced by custom-page widgets (C66).
func artifactRow(props artifactRowProps) ui.Node {
	a := props.Artifact
	app := appstate.Default

	renaming := ui.UseState(false)
	nameS := ui.UseState(a.Name)
	onName := ui.UseEvent(func(v string) { nameS.Set(v) })

	startRename := ui.UseEvent(Prevent(func() {
		nameS.Set(a.Name)
		renaming.Set(true)
	}))
	cancelRename := ui.UseEvent(Prevent(func() { renaming.Set(false) }))
	saveRename := ui.UseEvent(Prevent(func() {
		n := strings.TrimSpace(nameS.Get())
		if n == "" || app == nil {
			renaming.Set(false)
			return
		}
		updated := a
		updated.Name = n
		if err := app.PutArtifact(updated); err != nil {
			if props.Notice != nil {
				props.Notice(err.Error())
			}
		} else if props.Refresh != nil {
			props.Refresh()
		}
		renaming.Set(false)
	}))

	del := ui.UseEvent(Prevent(func() {
		if app == nil {
			return
		}
		if props.UsedByPages > 0 {
			// Warn instead of silently deleting an in-use artifact (C66).
			noun := "page"
			if props.UsedByPages != 1 {
				noun = "pages"
			}
			if props.Notice != nil {
				props.Notice(uistate.T("artifacts.deleteUsedBy", props.UsedByPages, noun))
			}
			return
		}
		if err := app.DeleteArtifact(a.ID); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}))

	var preview ui.Node = Fragment()
	if a.Kind == artifacts.KindImage && len(a.Bytes) > 0 {
		preview = Img(Attr("src", artifacts.DataURL(a.MIME, a.Bytes)), Attr("alt", a.Name),
			css.Class(tw.W10, tw.H10, tw.ObjectCover, tw.Rounded, tw.Mr2))
	}
	meta := a.Kind
	if len(a.Rows) > 0 {
		meta = a.Kind + " · " + itoaPct0(len(a.Rows)) + " rows"
	}

	var csvPreview ui.Node = Fragment()
	if a.Kind == artifacts.KindCSV && len(a.Columns) > 0 {
		headerCells := make([]any, 0, len(a.Columns))
		for _, col := range a.Columns {
			headerCells = append(headerCells, Th(col))
		}
		var bodyRows []any
		limit := len(a.Rows)
		if limit > 3 {
			limit = 3
		}
		for _, row := range a.Rows[:limit] {
			cells := make([]any, 0, len(row))
			for _, cell := range row {
				cells = append(cells, Td(cell))
			}
			bodyRows = append(bodyRows, Tr(cells...))
		}
		csvPreview = Div(Style(map[string]string{"overflow-x": "auto", "margin-top": "0.25rem", "font-size": "0.78rem"}),
			Table(css.Class("csv-preview"),
				Thead(Tr(headerCells...)),
				Tbody(bodyRows...),
			),
		)
	}

	if renaming.Get() {
		return Div(css.Class("row"),
			Form(css.Class("form-grid"), OnSubmit(saveRename),
				Input(css.Class("field"), Attr("aria-label", uistate.T("artifacts.renameLabel")),
					Value(nameS.Get()), OnInput(onName), Attr("data-testid", "artifact-rename-input")),
				Button(css.Class("btn btn-primary"), Type("submit"), uistate.T("action.save")),
				Button(css.Class("btn"), Type("button"), OnClick(cancelRename), uistate.T("action.cancel")),
			),
		)
	}

	// "Referenced" is a positive signal; "not referenced" is neutral muted.
	// Reorder meta: ref label first (Lena's primary question), then kind/size,
	// then upload date — so the most important info is at the top of the meta stack.
	refLabel := referencedByLabel(props.ReferencedBy)
	refClass := "row-meta"
	if props.ReferencedBy > 0 {
		refClass = "row-meta ref-positive"
	}
	uploadedOn := ""
	if !a.CreatedAt.IsZero() {
		uploadedOn = "Uploaded " + a.CreatedAt.Format("Jan 2, 2006")
	}

	return Div(css.Class("row"),
		Div(css.Class("row-main", tw.Flex, tw.ItemsCenter),
			preview,
			Div(
				Div(css.Class("row-desc"), Attr("data-testid", "artifact-name"), a.Name),
				Div(css.Class(refClass), Attr("data-testid", "artifact-refs"), refLabel),
				If(props.UsedByPages > 0,
					Div(css.Class("row-meta"), Attr("data-testid", "artifact-page-refs"),
						uistate.T("artifacts.usedByPages", props.UsedByPages))),
				Div(css.Class("row-meta"), meta+" · "+artifacts.HumanSize(a.Size)),
				If(uploadedOn != "", Div(css.Class("row-meta"), uploadedOn)),
				csvPreview,
			),
		),
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("aria-label", uistate.T("artifacts.renameTitle")), Title(uistate.T("artifacts.renameTitle")),
			OnClick(startRename), uiw.Icon(icon.Pencil, css.Class(tw.W4, tw.H4))),
		Button(css.Class("btn-del"), Type("button"), Attr("aria-label", uistate.T("action.delete")),
			Title(uistate.T("action.delete")), OnClick(del), uiw.Icon(icon.Close, css.Class(tw.W4, tw.H4))),
	)
}

// referencedByLabel renders the plain-English "Referenced by N transaction(s)"
// (or a not-referenced note) for an artifact (L29).
func referencedByLabel(n int) string {
	if n == 0 {
		return uistate.T("artifacts.referencedByNone")
	}
	noun := uistate.T("artifacts.referencedByCount", n)
	if n != 1 {
		noun += "s"
	}
	return uistate.T("artifacts.referencedBy", noun)
}

// itoaPct0 renders an int without a percent sign (small helper for counts).
func itoaPct0(n int) string {
	s := itoaPct(n)
	return strings.TrimSuffix(s, "%")
}

// pickFile opens a native file picker for the given accept filter and calls onData
// with the chosen file's name, MIME type, and raw bytes (decoded from the reader's
// data URL). Cleans up its JS callbacks. A no-op if nothing is chosen.
func pickFile(accept string, onData func(name, mime string, data []byte)) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", accept)

	var onChange, onLoad js.Func
	var fileName string
	onLoad = js.FuncOf(func(this js.Value, _ []js.Value) any {
		mime, data := decodeDataURL(this.Get("result").String())
		onData(fileName, mime, data)
		onLoad.Release()
		return nil
	})
	onChange = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		if files.Length() > 0 {
			f := files.Index(0)
			fileName = f.Get("name").String()
			reader := js.Global().Get("FileReader").New()
			reader.Set("onload", onLoad)
			reader.Call("readAsDataURL", f)
		} else {
			onLoad.Release()
		}
		onChange.Release()
		return nil
	})
	input.Set("onchange", onChange)
	input.Call("click")
}

// artifactStorageBar renders a thin progress bar showing how much of the practical
// storage limit (~10 MB combined) is used, with a warning tone when near the limit (C66).
func artifactStorageBar(totalBytes int, blobBytes int64) ui.Node {
	const limitBytes = 10 * 1024 * 1024 // 10 MB practical cap
	used := totalBytes
	if int(blobBytes) > used {
		used = int(blobBytes)
	}
	pct := float64(used) / float64(limitBytes) * 100
	if pct > 100 {
		pct = 100
	}
	cls := "storage-bar-fill"
	if pct > 80 {
		cls = "storage-bar-fill storage-bar-warn"
	}
	barStyle := map[string]string{"width": fmt.Sprintf("%.1f%%", pct)}
	return Div(css.Class("storage-bar", tw.Mt1),
		Div(css.Class(cls), Style(barStyle)),
	)
}

// decodeDataURL splits a `data:<mime>;base64,<payload>` URL into its MIME type and
// decoded bytes. Returns empty values if the URL isn't a base64 data URL.
func decodeDataURL(url string) (string, []byte) {
	if !strings.HasPrefix(url, "data:") {
		return "", nil
	}
	comma := strings.IndexByte(url, ',')
	if comma < 0 {
		return "", nil
	}
	meta := url[len("data:"):comma]
	mime := meta
	if i := strings.IndexByte(meta, ';'); i >= 0 {
		mime = meta[:i]
	}
	data, err := base64.StdEncoding.DecodeString(url[comma+1:])
	if err != nil {
		return mime, nil
	}
	return mime, data
}
