//go:build js && wasm

package ai

import (
	"encoding/json"
	"strings"
	"syscall/js"
)

func SendProxyChat(endpoint, token, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	body, err := BuildProxyChatRequest(model, messages, temperature)
	if err != nil {
		onError(err.Error())
		return noopCancel
	}
	return postProxyCompletion(endpoint, token, "/v1/ai/chat", body, onResult, onError)
}

func SendProxyStructuredVisionChat(endpoint, token, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	body, err := BuildProxyStructuredVisionRequest(model, systemPrompt, userText, imageURL, temperature, schemaName, schema)
	if err != nil {
		onError(err.Error())
		return noopCancel
	}
	return postProxyCompletion(endpoint, token, "/v1/ai/vision", body, onResult, onError)
}

func postProxyCompletion(endpoint, token, path string, body []byte, onResult func(string, Usage), onError func(string)) func() {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	token = strings.TrimSpace(token)
	if endpoint == "" || token == "" {
		onError("Backend URL and token are required.")
		return noopCancel
	}
	controller := js.Global().Get("AbortController").New()
	cancelled := false
	cancel := func() {
		if cancelled {
			return
		}
		cancelled = true
		controller.Call("abort")
	}
	opts := map[string]any{
		"method": "POST",
		"headers": map[string]any{
			"Authorization": "Bearer " + token,
			"Content-Type":  "application/json",
		},
		"body":   string(body),
		"signal": controller.Get("signal"),
	}
	var onText, onResp, onCatch js.Func
	release := func() { onText.Release(); onResp.Release(); onCatch.Release() }
	status := 0
	onResp = js.FuncOf(func(_ js.Value, args []js.Value) any {
		status = args[0].Get("status").Int()
		return args[0].Call("text")
	})
	onText = js.FuncOf(func(_ js.Value, args []js.Value) any {
		data := []byte(args[0].String())
		release()
		if cancelled {
			return nil
		}
		if status >= 400 {
			onError(strings.TrimSpace(string(data)))
			return nil
		}
		var out ProxyCompletion
		if err := json.Unmarshal(data, &out); err != nil {
			onError("Couldn't read the backend AI response.")
			return nil
		}
		onResult(out.Content, out.Usage)
		return nil
	})
	onCatch = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		release()
		if !cancelled {
			onError("Couldn't reach the backend server.")
		}
		return nil
	})
	js.Global().Call("fetch", endpoint+path, opts).
		Call("then", onResp).
		Call("then", onText).
		Call("catch", onCatch)
	return cancel
}
