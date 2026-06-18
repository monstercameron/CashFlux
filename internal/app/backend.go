//go:build js && wasm

package app

import (
	"strings"
	"syscall/js"
)

const defaultBackendURL = "http://127.0.0.1:8081"

func uploadOpenAIKeyToBackend(endpoint, token, key string, onDone func(), onError func(string)) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = defaultBackendURL
	}
	token = strings.TrimSpace(token)
	key = strings.TrimSpace(key)
	if token == "" {
		onError("Add a backend token before uploading the key.")
		return
	}
	if key == "" {
		onError("Add your OpenAI key before uploading it.")
		return
	}
	body := `{"provider":"openai","key":` + js.Global().Get("JSON").Call("stringify", key).String() + `}`
	opts := map[string]any{
		"method": "POST",
		"headers": map[string]any{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
		"body": body,
	}
	var onText, onResp, onCatch js.Func
	release := func() {
		onText.Release()
		onResp.Release()
		onCatch.Release()
	}
	status := 0
	onResp = js.FuncOf(func(_ js.Value, args []js.Value) any {
		status = args[0].Get("status").Int()
		return args[0].Call("text")
	})
	onText = js.FuncOf(func(_ js.Value, args []js.Value) any {
		text := args[0].String()
		release()
		if status >= 200 && status < 300 {
			onDone()
			return nil
		}
		if strings.TrimSpace(text) == "" {
			text = "Backend rejected the key upload."
		}
		onError(text)
		return nil
	})
	onCatch = js.FuncOf(func(_ js.Value, args []js.Value) any {
		release()
		onError("Couldn't reach the backend server.")
		return nil
	})
	js.Global().Call("fetch", endpoint+"/v1/ai/key", opts).Call("then", onResp).Call("then", onText).Call("catch", onCatch)
}
