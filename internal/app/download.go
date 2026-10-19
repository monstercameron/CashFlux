// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import "syscall/js"

// downloadBytes triggers a browser download of data as filename with the given
// MIME type, by building a Blob and clicking a transient anchor. This is the one
// spot that touches the DOM for file egress; the data itself comes from the pure
// store/appstate layer.
func downloadBytes(filename, mime string, data []byte) {
	doc := js.Global().Get("document")

	buf := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buf, data)
	parts := js.Global().Get("Array").New()
	parts.Call("push", buf)
	blob := js.Global().Get("Blob").New(parts, map[string]any{"type": mime})

	url := js.Global().Get("URL").Call("createObjectURL", blob)
	defer js.Global().Get("URL").Call("revokeObjectURL", url)

	a := doc.Call("createElement", "a")
	a.Set("href", url)
	a.Set("download", filename)
	doc.Get("body").Call("appendChild", a)
	a.Call("click")
	doc.Get("body").Call("removeChild", a)
}

// pickFile opens the OS file picker and reads the chosen file's bytes, invoking
// onLoad with them. accept is a file-input accept string (e.g. ".json"). The js
// callbacks are released once the read completes.
func pickFile(accept string, onLoad func([]byte)) {
	pickFileNamed(accept, func(_, _ string, data []byte) { onLoad(data) })
}

// pickFileLimited rejects a file before allocating a FileReader buffer when it
// exceeds maxBytes. onTooLarge receives the selected size for a friendly error.
func pickFileLimited(accept string, maxBytes int, onTooLarge func(size int), onLoad func([]byte)) {
	pickFileNamedLimited(accept, maxBytes, func(size int) { onTooLarge(size) }, func(_, _ string, data []byte) { onLoad(data) })
}

// pickFileNamed is like pickFile but also reports the chosen file's name and MIME
// type — needed when the handler must infer a format or label from the file (e.g.
// a custom font, whose family name comes from the file name). The browser's MIME
// type may be empty for some fonts; the caller can fall back to the name.
func pickFileNamed(accept string, onLoad func(name, mime string, data []byte)) {
	pickFileNamedLimited(accept, 0, nil, onLoad)
}

func pickFileNamedLimited(accept string, maxBytes int, onTooLarge func(size int), onLoad func(name, mime string, data []byte)) {
	input := js.Global().Get("document").Call("createElement", "input")
	input.Set("type", "file")
	if accept != "" {
		input.Set("accept", accept)
	}

	var changeCb js.Func
	changeCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		files := input.Get("files")
		if files.Length() == 0 {
			changeCb.Release()
			return nil
		}
		file := files.Index(0)
		size := file.Get("size").Int()
		if maxBytes > 0 && size > maxBytes {
			if onTooLarge != nil {
				onTooLarge(size)
			}
			changeCb.Release()
			return nil
		}
		name := file.Get("name").String()
		mime := file.Get("type").String()
		reader := js.Global().Get("FileReader").New()
		var loadCb js.Func
		loadCb = js.FuncOf(func(_ js.Value, _ []js.Value) any {
			u8 := js.Global().Get("Uint8Array").New(reader.Get("result"))
			buf := make([]byte, u8.Get("length").Int())
			js.CopyBytesToGo(buf, u8)
			onLoad(name, mime, buf)
			loadCb.Release()
			changeCb.Release()
			return nil
		})
		reader.Call("addEventListener", "load", loadCb)
		reader.Call("readAsArrayBuffer", file)
		return nil
	})
	input.Call("addEventListener", "change", changeCb)
	input.Call("click")
}
