// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"syscall/js"
	"testing"

	"github.com/monstercameron/GoWebComponents/v5/testkit/render"
)

func fakeImportFileBrowser(t *testing.T) js.Value {
	t.Helper()
	document, reader := js.Global().Get("document"), js.Global().Get("FileReader")
	state := js.Global().Get("Function").New(`
		const state = {reads: []};
		const body = {
			appendChild(node) { node.parentNode = this; },
			removeChild(node) { node.parentNode = null; }
		};
		globalThis.document = {
			body, head: {appendChild() {}}, querySelector() {return null;}, getElementById() {return null;},
			createElement(tag) {
				const node = {files: [], setAttribute() {}, click() {this.opened = this.parentNode === body;}};
				if (tag === 'input') state.input = node;
				return node;
			}
		};
		globalThis.FileReader = class {
			readAsDataURL(file) { state.reads.push({reader: this, file}); }
		};
		state.choose = file => {state.input.files = [file]; state.input.onchange();};
		state.finish = (index, data) => {const r=state.reads[index].reader; r.result=data; r.onload();};
		state.fail = index => state.reads[index].reader.onerror();
		return state;
	`).Invoke()
	t.Cleanup(func() {
		js.Global().Set("document", document)
		js.Global().Set("FileReader", reader)
	})
	return state
}

func importTestFile(name, mime string, size int) js.Value {
	return js.ValueOf(map[string]any{"name": name, "type": mime, "size": size})
}

func TestImportUXFileSelectionAttachedAndFenced(t *testing.T) {
	for _, source := range []string{"pdf", "receipt"} {
		t.Run(source, func(t *testing.T) {
			calls := fakeImportFetch(t)
			f, current, _ := mountImportUX(t)
			current().chooseSource(source)
			f.Stabilize()
			browser := fakeImportFileBrowser(t)
			current().chooseImportFile(source)
			if !browser.Get("input").Get("opened").Bool() {
				t.Fatal("native file input was clicked without being attached")
			}
			name, mime, data := "statement.pdf", "application/pdf", "data:application/pdf;base64,JVBERi0="
			if source == "receipt" {
				name, mime, data = "receipt.png", "image/png", "data:image/png;base64,aW1hZ2U="
			}
			browser.Call("choose", importTestFile(name, mime, 5))
			if browser.Get("input").Get("parentNode").Truthy() {
				t.Fatal("file chooser helper leaked after selection")
			}
			browser.Call("finish", 0, data)
			f.Stabilize()
			if source == "pdf" && (current().pdfName.Get() != name || len(current().pdfBytes.Get()) != 5) {
				t.Fatal("selected PDF metadata or bytes missing")
			}
			if source == "receipt" && (current().imageName.Get() != name || current().imageSize.Get() != 5 || current().imageURL.Get() != data) {
				t.Fatal("selected image metadata or bytes missing")
			}
			current().loadImportFile(source, importTestFile("late.pdf", mime, 5))
			current().chooseSource("")
			browser.Call("finish", 1, data)
			f.Stabilize()
			if current().pdfName.Get() == "late.pdf" || current().imageName.Get() == "late.pdf" {
				t.Fatal("stale read changed the chosen source")
			}
			if calls.Length() != 0 {
				t.Fatal("file selection sent a network request")
			}
		})
	}
}

func TestImportUXFileReadFailureAndCancel(t *testing.T) {
	fakeImportFetch(t)
	f, current, _ := mountImportUX(t)
	current().chooseSource("pdf")
	f.Stabilize()
	browser := fakeImportFileBrowser(t)
	current().loadImportFile("pdf", importTestFile("statement.pdf", "application/pdf", 5))
	browser.Call("fail", 0)
	f.Stabilize()
	if current().aiErr.Get() == "" || current().pdfData.Get() != "" {
		t.Fatal("file read failure did not keep the source empty with a message")
	}
	current().chooseImportFile("pdf")
	browser.Get("input").Call("oncancel")
	if browser.Get("input").Get("parentNode").Truthy() {
		t.Fatal("cancelled native input leaked")
	}
}

func TestImportUXDropLoadsFileAndHonorsBusyState(t *testing.T) {
	for _, source := range []string{"pdf", "receipt"} {
		t.Run(source, func(t *testing.T) {
			fakeImportFetch(t)
			f, current, _ := mountImportUX(t)
			current().chooseSource(source)
			f.Stabilize()
			browser := fakeImportFileBrowser(t)
			name, mime, data, testID := "statement.pdf", "application/pdf", "data:application/pdf;base64,JVBERi0=", "statementimport-dropzone"
			if source == "receipt" {
				name, mime, data, testID = "receipt.png", "image/png", "data:image/png;base64,aW1hZ2U=", "import-image-dropzone"
			}
			// The fixture's synthetic Event has no file fields. Supply only the
			// native dataTransfer property via its prototype, restoring it below.
			restore := js.Global().Get("Function").New("file", `
				const old = Object.getOwnPropertyDescriptor(Object.prototype, 'dataTransfer');
				Object.defineProperty(Object.prototype, 'dataTransfer', {configurable:true, value:{files:[file]}});
				return () => {if(old) Object.defineProperty(Object.prototype,'dataTransfer',old); else delete Object.prototype.dataTransfer;};
			`).Invoke(importTestFile(name, mime, 5))
			defer restore.Invoke()
			findByTestID(f, "div", testID).Dispatch("ondrop", render.Event{})
			if browser.Get("reads").Length() != 1 {
				t.Fatal("drop did not read the chosen file")
			}
			browser.Call("finish", 0, data)
			f.Stabilize()
			if source == "pdf" && current().pdfName.Get() != name || source == "receipt" && current().imageName.Get() != name {
				t.Fatal("drop did not update source metadata")
			}
			current().checkpointSaving.Set(true)
			f.Stabilize()
			findByTestID(f, "div", testID).Dispatch("ondrop", render.Event{})
			if browser.Get("reads").Length() != 1 {
				t.Fatal("drop changed the source during a durable write")
			}
		})
	}
}
