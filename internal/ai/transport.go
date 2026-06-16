//go:build js && wasm

package ai

import "syscall/js"

// DefaultBaseURL is the OpenAI API base; configurable for proxies/compat servers.
const DefaultBaseURL = "https://api.openai.com/v1"

// SendChat posts a chat-completions request to baseURL using the user's apiKey,
// asynchronously. On success it calls onResult with the assistant's content; on
// any failure (build, network, API, or empty response) it calls onError with a
// plain-English message. Exactly one of the callbacks runs. This is the only
// place that talks to the network; the request/response shaping is pure (ai.go).
func SendChat(apiKey, baseURL, model string, messages []Message, temperature float64, onResult func(string), onError func(string)) {
	body, err := BuildRequest(model, messages, temperature)
	if err != nil {
		onError(err.Error())
		return
	}
	postCompletions(apiKey, baseURL, body, onResult, onError)
}

// SendVisionChat posts a multimodal chat-completions request (a system prompt, a
// user instruction, and one image as a data/URL) using a vision-capable model.
// Same async contract as SendChat: exactly one of onResult/onError runs.
func SendVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, onResult func(string), onError func(string)) {
	body, err := BuildVisionRequest(model, systemPrompt, userText, imageURL, temperature)
	if err != nil {
		onError(err.Error())
		return
	}
	postCompletions(apiKey, baseURL, body, onResult, onError)
}

// postCompletions sends a prebuilt request body to the chat-completions endpoint
// and routes the parsed result (or a plain-English error) to the callbacks. It
// owns the fetch promise chain and releases its js.Funcs when done.
func postCompletions(apiKey, baseURL string, body []byte, onResult func(string), onError func(string)) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	opts := map[string]any{
		"method": "POST",
		"headers": map[string]any{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		"body": string(body),
	}

	var onText, onResp, onCatch js.Func
	release := func() { onText.Release(); onResp.Release(); onCatch.Release() }

	onResp = js.FuncOf(func(_ js.Value, args []js.Value) any {
		return args[0].Call("text") // a promise resolving to the response body
	})
	onText = js.FuncOf(func(_ js.Value, args []js.Value) any {
		content, err := ParseResponse([]byte(args[0].String()))
		if err != nil {
			onError(err.Error())
		} else {
			onResult(content)
		}
		release()
		return nil
	})
	onCatch = js.FuncOf(func(_ js.Value, args []js.Value) any {
		msg := "network error"
		if len(args) > 0 {
			msg = args[0].Call("toString").String()
		}
		onError("ai: couldn't reach OpenAI — " + msg)
		release()
		return nil
	})

	js.Global().Call("fetch", baseURL+"/chat/completions", opts).
		Call("then", onResp).
		Call("then", onText).
		Call("catch", onCatch)
}
