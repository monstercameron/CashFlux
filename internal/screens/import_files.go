// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/importdraft"
	"github.com/monstercameron/CashFlux/internal/uistate"
	"github.com/monstercameron/GoWebComponents/v7/ui"
)

func (c *documentsController) chooseImportFile(source string) {
	if c.busy() || c.aiLoading.Get() {
		return
	}
	c.cancelAI()
	request := c.model.Begin()
	pickImportSourceFile(source, func(file js.Value) {
		if c.model.Accept(request) {
			c.loadImportFile(source, file)
		}
	})
}

// pickImportSourceFile attaches the native input before clicking it. Browsers
// can silently ignore a detached input; change/cancel both remove the helper.
func pickImportSourceFile(source string, onFile func(js.Value)) {
	doc := js.Global().Get("document")
	input := doc.Call("createElement", "input")
	input.Set("type", "file")
	input.Set("accept", ".pdf,application/pdf")
	if source == "receipt" {
		input.Set("accept", "image/*")
		input.Set("capture", "environment")
	}
	input.Set("style", "position:fixed;left:-9999px;width:1px;height:1px;opacity:0")
	input.Set("tabIndex", -1)
	var change, cancel js.Func
	cleanup := func() {
		input.Set("onchange", js.Null())
		input.Set("oncancel", js.Null())
		if parent := input.Get("parentNode"); parent.Truthy() {
			parent.Call("removeChild", input)
		}
		change.Release()
		cancel.Release()
	}
	change = js.FuncOf(func(js.Value, []js.Value) any {
		defer cleanup()
		if files := input.Get("files"); files.Length() > 0 {
			onFile(files.Index(0))
		}
		return nil
	})
	cancel = js.FuncOf(func(js.Value, []js.Value) any { cleanup(); return nil })
	input.Set("onchange", change)
	input.Set("oncancel", cancel)
	doc.Get("body").Call("appendChild", input)
	input.Call("click")
}

func (c *documentsController) dropImportFile(source string, event ui.Event) {
	event.PreventDefault()
	if c.busy() || c.aiLoading.Get() {
		return
	}
	transfer := event.JSValue().Get("dataTransfer")
	if !transfer.Truthy() {
		return
	}
	if files := transfer.Get("files"); files.Truthy() && files.Length() > 0 {
		c.loadImportFile(source, files.Index(0))
	}
}

func (c *documentsController) loadImportFile(source string, file js.Value) {
	c.cancelAI()
	request := c.model.Begin()
	name, mime, size := file.Get("name").String(), file.Get("type").String(), file.Get("size").Int()
	if key := importdraft.SourceFileError(source, name, mime, size); key != "" {
		c.model.Cancel()
		c.aiErr.Set(uistate.T(key))
		return
	}
	reader := js.Global().Get("FileReader").New()
	var loaded, failed js.Func
	cleanup := func() {
		reader.Set("onload", js.Null())
		reader.Set("onerror", js.Null())
		loaded.Release()
		failed.Release()
	}
	failed = js.FuncOf(func(js.Value, []js.Value) any {
		defer cleanup()
		if c.model.Accept(request) {
			c.aiErr.Set(uistate.T("importux.fileReadError"))
		}
		return nil
	})
	loaded = js.FuncOf(func(js.Value, []js.Value) any {
		defer cleanup()
		if !c.model.Accept(request) {
			return nil
		}
		data := reader.Get("result").String()
		if source == "pdf" {
			_, bytes := decodeDataURL(data)
			if len(bytes) == 0 {
				c.aiErr.Set(uistate.T("importux.fileReadError"))
				return nil
			}
			c.pdfBytes.Set(bytes)
			c.pdfData.Set(importdraft.PDFDataURL("application/pdf", bytes))
			c.pdfName.Set(name)
		} else {
			c.imageURL.Set(data)
			c.imageDraftAtom.Set(data)
			c.imageName.Set(name)
			c.imageSize.Set(size)
		}
		c.aiErr.Set("")
		c.needsKey.Set(false)
		return nil
	})
	reader.Set("onload", loaded)
	reader.Set("onerror", failed)
	reader.Call("readAsDataURL", file)
}
