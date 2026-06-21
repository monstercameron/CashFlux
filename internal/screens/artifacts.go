//go:build js && wasm

package screens

import (
	"encoding/base64"
	"strings"
	"syscall/js"
	"time"

	"github.com/monstercameron/CashFlux/internal/appstate"
	"github.com/monstercameron/CashFlux/internal/artifacts"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/icon"
	"github.com/monstercameron/CashFlux/internal/id"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/uistate"
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
		return Section(ClassStr("card"), P(ClassStr("empty"), uistate.T("common.notReady")))
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

	list := app.Artifacts()
	var rows []ui.Node
	for _, a := range list {
		rows = append(rows, ui.CreateElement(artifactRow, artifactRowProps{Artifact: a, Refresh: refresh}))
	}
	listBody := P(ClassStr("empty"), uistate.T("artifacts.empty"))
	if len(rows) > 0 {
		listBody = Div(ClassStr("rows"), rows)
	}

	// Storage meter: total serialized dataset size (what hits localStorage).
	total := app.DatasetBytes()

	return Div(
		Section(ClassStr("card"),
			Div(ClassStr("flex gap-2 flex-wrap"),
				Button(ClassStr("btn btn-primary"), Type("button"), OnClick(uploadImage), uistate.T("artifacts.uploadImage")),
				Button(ClassStr("btn"), Type("button"), OnClick(importCSV), uistate.T("artifacts.importCSV")),
			),
			P(ClassStr("muted mt-2"), uistate.T("artifacts.storage", artifacts.HumanSize(total))),
		),
		Section(ClassStr("card"), listBody),
	)
}

type artifactRowProps struct {
	Artifact domain.Artifact
	Refresh  func()
}

// artifactRow is one artifact entry with a delete action — its own component so
// the delete hook is stable across the list.
func artifactRow(props artifactRowProps) ui.Node {
	a := props.Artifact
	del := func() {
		if appstate.Default == nil {
			return
		}
		if err := appstate.Default.DeleteArtifact(a.ID); err == nil && props.Refresh != nil {
			props.Refresh()
		}
	}
	var preview ui.Node = Fragment()
	if a.Kind == artifacts.KindImage && len(a.Bytes) > 0 {
		preview = Img(Attr("src", artifacts.DataURL(a.MIME, a.Bytes)), Attr("alt", a.Name),
			ClassStr("w-10 h-10 object-cover rounded mr-2"))
	}
	meta := a.Kind
	if len(a.Rows) > 0 {
		meta = a.Kind + " · " + itoaPct0(len(a.Rows)) + " rows"
	}
	return Div(ClassStr("row"),
		Div(ClassStr("row-main flex items-center"),
			preview,
			Div(
				Div(ClassStr("row-desc"), a.Name),
				Div(ClassStr("row-meta"), meta+" · "+artifacts.HumanSize(a.Size)),
			),
		),
		Button(ClassStr("btn-del"), Type("button"), Attr("aria-label", uistate.T("action.delete")), Title(uistate.T("action.delete")), OnClick(del), uiw.Icon(icon.Close, ClassStr("w-4 h-4"))),
	)
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
