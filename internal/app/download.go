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
