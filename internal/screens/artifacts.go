// SPDX-License-Identifier: MIT

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
	"github.com/monstercameron/CashFlux/internal/pagination"
	uiw "github.com/monstercameron/CashFlux/internal/ui"
	"github.com/monstercameron/CashFlux/internal/ui/tw"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v4/css"
	. "github.com/monstercameron/GoWebComponents/v4/html/shorthand"
	"github.com/monstercameron/GoWebComponents/v4/ui"
)

// Artifacts is the manager for user-stored assets: upload an image or import a
// CSV dataset, see them listed with their size, and delete them. A storage meter
// shows how much of the browser's local storage the whole dataset is using, since
// artifact bytes live in that single autosaved blob. Custom-page Image and Table
// widgets reference these by id.
func Artifacts() ui.Node {
	app := appstate.Default
	if app == nil {
		return uiw.Card(uiw.CardProps{Body: P(css.Class("empty"), uistate.T("common.notReady"))})
	}
	rev := ui.UseState(0)
	_ = rev.Get()
	refresh := func() { rev.Set(rev.Get() + 1) }
	// Re-render after modal saves (ArtifactEditHost bumps the shared data revision).
	_ = uistate.UseDataRevision().Get()
	// Surface upload/save failures instead of swallowing them — a large image can
	// blow the localStorage quota (the whole dataset is one blob), and silently
	// dropping the file leaves the user confused (C66, reliability). The real error
	// text (e.g. the quota message) is shown so the cause is clear.
	notice := uistate.UseNotice()
	notify := func(text string) { notice.Set(notice.Get().With(text, true)) }

	// Pagination: a receipt-heavy vault runs to dozens of rows (and each image
	// row carries a thumbnail), so the ledger renders one page at a time.
	pageS := ui.UseState(1)
	prevPage := ui.UseEvent(Prevent(func() { pageS.Set(pageS.Get() - 1) }))
	nextPage := ui.UseEvent(Prevent(func() { pageS.Set(pageS.Get() + 1) }))

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

	// One page of the vault at a time (deleting off the last page clamps back).
	const artifactsPageSize = 10
	totalFiles := len(list)
	curPage := pagination.Clamp(pageS.Get(), totalFiles, artifactsPageSize)
	totalPages := pagination.TotalPages(totalFiles, artifactsPageSize)
	pageItems := pagination.Slice(list, curPage, artifactsPageSize)

	var rows []ui.Node
	for _, a := range pageItems {
		rows = append(rows, ui.CreateElement(artifactRow, artifactRowProps{
			Artifact: a, Refresh: refresh,
			ReferencedBy: refCount[a.ID],
			UsedByPages:  pageRefCount[a.ID],
			Notice:       notify,
		}))
	}
	listBody := P(css.Class("empty"), uistate.T("artifacts.empty"))
	if len(rows) > 0 {
		// NOTE (G20): a bottom add-artifact CTA was tried but a conditional On* button
		// here shifts hook ordering when the row count changes (the no-On*-in-variable-
		// position rule), breaking the screen. The always-visible upload card at the top
		// already provides the affordance, so the list stays a plain row container.
		listBody = Div(css.Class("rows"), rows)
	}

	// Pager: shown only past one page. Mirrors the to-do ledger's pager (range
	// caption + Prev/Next disabled at the ends).
	var pager ui.Node = Fragment()
	if totalPages > 1 {
		first := (curPage-1)*artifactsPageSize + 1
		last := first + artifactsPageSize - 1
		if last > totalFiles {
			last = totalFiles
		}
		prevArgs := []any{css.Class("todo-page-btn"), Type("button"), Attr("data-testid", "artifacts-prev"), Attr("aria-label", uistate.T("todo.pagePrev")), OnClick(prevPage), uiw.Icon(icon.ChevronLeft, css.Class(tw.W4, tw.H4)), Span(uistate.T("todo.pagePrev"))}
		if curPage <= 1 {
			prevArgs = append(prevArgs, Attr("disabled", ""))
		}
		nextArgs := []any{css.Class("todo-page-btn"), Type("button"), Attr("data-testid", "artifacts-next"), Attr("aria-label", uistate.T("todo.pageNext")), OnClick(nextPage), Span(uistate.T("todo.pageNext")), uiw.Icon(icon.ChevronRight, css.Class(tw.W4, tw.H4))}
		if curPage >= totalPages {
			nextArgs = append(nextArgs, Attr("disabled", ""))
		}
		pager = Div(css.Class("todo-pager"),
			Span(css.Class("todo-pager-range"), Attr("data-testid", "artifacts-pager-range"), uistate.T("todo.pageRange", first, last, totalFiles)),
			Div(css.Class("todo-pager-nav"),
				Button(prevArgs...),
				Span(css.Class("todo-pager-page"), uistate.T("todo.pageOf", curPage, totalPages)),
				Button(nextArgs...),
			),
		)
	}

	// Storage meter: combined localStorage dataset size + IndexedDB blob bytes.
	// Use DatasetBytesWithBlobs so both storage locations are counted, but do NOT
	// call any IDB Get in the render path — that would block the single-threaded
	// wasm runtime waiting for a JS callback that can never fire.
	total := app.DatasetBytesWithBlobs()
	blobUsage := app.BlobStoreUsage()

	// The near-limit quota story lives in the hero takeaway (art.takeNearLimit)
	// — the old dismissible banner duplicated it word-for-word one section down.
	storageLabel := uistate.T("artifacts.storage", artifacts.HumanSize(total))
	if blobUsage > 0 {
		storageLabel = uistate.T("artifacts.storageIDB",
			artifacts.HumanSize(int(blobUsage)),
			artifacts.HumanSize(total),
		)
	}

	// ── Hero: the storage footprint, the vault chips, and the takeaway. ────────
	imgCount, csvCount, pdfCount := 0, 0, 0
	for _, a := range list {
		switch a.Kind {
		case artifacts.KindImage:
			imgCount++
		case artifacts.KindPDF:
			pdfCount++
		default:
			csvCount++
		}
	}
	attachedCount, pageBoundCount := 0, 0
	for aid, n := range refCount {
		if n > 0 && aid != "" {
			attachedCount++
		}
	}
	for _, n := range pageRefCount {
		if n > 0 {
			pageBoundCount++
		}
	}
	heroUsed := total
	if int(blobUsage) > heroUsed {
		heroUsed = int(blobUsage)
	}

	fileLine := uistate.T("artifacts.fileWordN", len(list))
	if len(list) == 1 {
		fileLine = uistate.T("artifacts.fileWordOne")
	}
	eyebrow := fileLine + " · " + uistate.T("artifacts.eyebrowTail")

	chips := []ui.Node{}
	if imgCount > 0 {
		chips = append(chips, rptChip(uistate.T("artifacts.chipImages"), fmt.Sprintf("%d", imgCount), ""))
	}
	if csvCount > 0 {
		chips = append(chips, rptChip(uistate.T("artifacts.chipCSV"), fmt.Sprintf("%d", csvCount), ""))
	}
	if pdfCount > 0 {
		chips = append(chips, rptChip(uistate.T("artifacts.chipPDF"), fmt.Sprintf("%d", pdfCount), ""))
	}
	if attachedCount > 0 {
		chips = append(chips, rptChip(uistate.T("artifacts.chipAttach"), fmt.Sprintf("%d", attachedCount), rptToneCls("pos")))
	}
	if pageBoundCount > 0 {
		chips = append(chips, rptChip(uistate.T("artifacts.chipPages"), fmt.Sprintf("%d", pageBoundCount), ""))
	}

	takeaway := uistate.T("art.takeEmpty")
	if len(list) > 0 {
		imgWord := uistate.T("art.imageWordN", imgCount)
		if imgCount == 1 {
			imgWord = uistate.T("art.imageWordOne")
		}
		csvWord := uistate.T("art.csvWordN", csvCount)
		if csvCount == 1 {
			csvWord = uistate.T("art.csvWordOne")
		}
		takeaway = uistate.T("art.takeStored", imgWord, csvWord, artifacts.HumanSize(heroUsed))
		if blobUsage > 0 && artifactstore.NearLimit(blobUsage) {
			takeaway += " " + uistate.T("art.takeNearLimit")
		}
	}

	heroBody := Div(css.Class("rpt-hero"), Attr("id", "sec-vault-hero"),
		P(css.Class("rpt-hero-eyebrow", tw.TextDim), eyebrow),
		Div(css.Class("rpt-hero-main"),
			Div(
				Div(css.Class("rpt-hero-label", tw.TextDim), uistate.T("artifacts.heroLabel")),
				Div(ClassStr("rpt-hero-value "+tw.Fold(tw.FontDisplay)), artifacts.HumanSize(heroUsed)),
				Div(css.Class("vault-meter"), artifactStorageBar(total, blobUsage)),
				P(css.Class("muted", tw.Mt1, tw.Text12), storageLabel),
			),
		),
		If(len(chips) > 0, Div(css.Class("debt-chips"), chips)),
		P(ClassStr("rpt-takeaway "+tw.Fold(tw.FontDisplay)), Attr("data-testid", "vault-takeaway"), takeaway),
	)
	heroTile := rptTile("vault-hero", "1 / span 4", rptSection("", uistate.T("artifacts.heroTitle"), nil, heroBody))

	// Upload actions ride the vault section's header so the page stays two tiles.
	uploadActions := Div(css.Class(tw.Flex, tw.Gap2, tw.FlexWrap, tw.ItemsCenter),
		Button(css.Class("btn btn-primary"), Type("button"), OnClick(uploadImage), uistate.T("artifacts.uploadImage")),
		Button(css.Class("btn"), Type("button"), OnClick(importCSV), uistate.T("artifacts.importCSV")),
	)

	return Div(css.Class("bento bento-vault"),
		heroTile,
		rptTile("vault-list", "1 / span 4",
			rptSection("sec-vault-list", uistate.T("artifacts.vaultTitle"), uploadActions, Fragment(
				P(css.Class("muted"), uistate.T("artifacts.uploadDesc")),
				listBody,
				pager,
			))),
	)
}

type artifactRowProps struct {
	Artifact     domain.Artifact
	Refresh      func()
	ReferencedBy int          // how many transactions attach this artifact (L29)
	UsedByPages  int          // how many custom-page widgets reference this artifact (C66)
	Notice       func(string) // surface errors to the screen-level notice
}

// artifactRow is one artifact entry with rename, download, and delete actions —
// its own component so its hooks stay stable across the list. Rename opens the
// shell-root flip modal; delete is guarded when the artifact is referenced by
// custom-page widgets (C66).
func artifactRow(props artifactRowProps) ui.Node {
	a := props.Artifact
	app := appstate.Default

	// Rename opens the shell-root flip modal (ArtifactEditHost).
	startRename := ui.UseEvent(Prevent(func() { uistate.SetArtifactEdit(a.ID) }))

	// Download reconstructs the original file: CSV datasets re-serialize from
	// their parsed columns/rows; image bytes may live only in the IndexedDB blob
	// store, so the (blocking) fetch runs in a goroutine off the render path.
	download := ui.UseEvent(Prevent(func() {
		go func() {
			data, mime, name := a.Bytes, a.MIME, a.Name
			if a.Kind == artifacts.KindCSV {
				data, mime = artifacts.CSVBytes(a.Columns, a.Rows), "text/csv"
				if !strings.HasSuffix(strings.ToLower(name), ".csv") {
					name += ".csv"
				}
			} else if len(data) == 0 && app != nil {
				if b, err := app.GetBlobForArtifact(a.ID); err == nil && len(b) > 0 {
					data = b
				}
			}
			if len(data) == 0 {
				if props.Notice != nil {
					props.Notice(uistate.T("artifacts.downloadEmpty", a.Name))
				}
				return
			}
			if mime == "" {
				mime = "application/octet-stream"
			}
			downloadBytes(name, mime, data)
		}()
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
		// Deleting a file is permanent — confirm first (every other destructive
		// action in the app does; artifacts was the outlier).
		uistate.ConfirmModal(uistate.T("artifacts.deleteConfirm", a.Name), true, func(ok bool) {
			if !ok {
				return
			}
			if err := app.DeleteArtifact(a.ID); err == nil && props.Refresh != nil {
				props.Refresh()
			}
		})
	}))

	var preview ui.Node = Fragment()
	if a.Kind == artifacts.KindImage && len(a.Bytes) > 0 {
		// Wrap in a fixed-size container so a broken image (e.g. stub/corrupt bytes)
		// shows a neutral placeholder rather than the browser's broken-image icon.
		// The onerror string hides the <img> and reveals the sibling fallback div;
		// no framework hook is used here so it is safe inside a component render (G20).
		preview = Div(css.Class("artifact-thumb-wrap", tw.Mr2),
			Img(Attr("src", artifacts.DataURL(a.MIME, a.Bytes)), Attr("alt", a.Name),
				css.Class(tw.W10, tw.H10, tw.ObjectCover, tw.Rounded),
				Attr("onerror", "this.style.display='none';this.nextElementSibling.style.display='flex'")),
			Div(css.Class("artifact-thumb-fallback"),
				uiw.Icon(icon.FileText, css.Class(tw.W5, tw.H5))),
		)
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
						uistate.T("artifacts.usedByPages", props.UsedByPages, pageNoun(props.UsedByPages)))),
				Div(css.Class("row-meta"), meta+" · "+artifacts.HumanSize(a.Size)),
				If(uploadedOn != "", Div(css.Class("row-meta"), uploadedOn)),
				csvPreview,
			),
		),
		// A labeled Rename button — the sibling Data & People rows all carry a
		// visible icon+label primary row action, not a bare glyph.
		Button(css.Class("btn", tw.InlineFlex, tw.ItemsCenter, tw.Gap15), Type("button"),
			Attr("aria-label", uistate.T("artifacts.renameTitle")), Title(uistate.T("artifacts.renameTitle")),
			OnClick(startRename), uiw.Icon(icon.Pencil, css.Class(tw.ShrinkO, tw.W4, tw.H4)), Span(uistate.T("artifacts.renameTitle"))),
		uiw.KebabMenu(uiw.KebabMenuProps{
			ID:           "artifact-menu-" + a.ID,
			AriaLabel:    uistate.T("artifacts.menuAria"),
			ToggleTestID: "artifact-menu-btn-" + a.ID,
			Items: []ui.Node{
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "artifact-download-"+a.ID),
					Title(uistate.T("artifacts.downloadTitle")), OnClick(download), uistate.T("artifacts.download")),
				Button(css.Class("add-item"), Type("button"), Attr("role", "menuitem"),
					Attr("data-testid", "artifact-delete-"+a.ID), Attr("aria-label", uistate.T("action.delete")),
					Title(uistate.T("action.delete")), OnClick(del), uistate.T("action.delete")),
			},
		}),
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

// pageNoun pluralizes the custom-page noun for the used-by label ("Used by 1
// custom page" / "Used by 3 custom pages") — the label previously passed one
// argument to a two-verb format string and rendered "%!s(MISSING)".
func pageNoun(n int) string {
	if n == 1 {
		return "page"
	}
	return "pages"
}
