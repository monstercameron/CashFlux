//go:build js && wasm

package ai

import "syscall/js"

// DefaultBaseURL is the OpenAI API base; configurable for proxies/compat servers.
const DefaultBaseURL = "https://api.openai.com/v1"

// SendChat posts a chat-completions request to baseURL using the user's apiKey,
// asynchronously. On success it calls onResult with the assistant's content and
// the call's token usage; on any failure (build, network, API, or empty response)
// it calls onError with a plain-English message. Exactly one of the callbacks
// runs. This is the only place that talks to the network; the request/response
// shaping is pure (ai.go).
func SendChat(apiKey, baseURL, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) {
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
func SendVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, onResult func(string, Usage), onError func(string)) {
	body, err := BuildVisionRequest(model, systemPrompt, userText, imageURL, temperature)
	if err != nil {
		onError(err.Error())
		return
	}
	postCompletions(apiKey, baseURL, body, onResult, onError)
}

// postCompletions sends a prebuilt request body to the chat-completions endpoint
// and routes the parsed result (or a plain-English error) to the callbacks. It
// owns the fetch promise chain, releases its js.Funcs per attempt, and retries
// transient failures (429/5xx/network) with exponential backoff (see
// IsRetryable / RetryDelayMS), giving up with the last error after MaxRetries.
func postCompletions(apiKey, baseURL string, body []byte, onResult func(string, Usage), onError func(string)) {
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

	var attempt func(n int)
	attempt = func(n int) {
		var onText, onResp, onCatch js.Func
		release := func() { onText.Release(); onResp.Release(); onCatch.Release() }

		// retryOrFail schedules another attempt after a backoff when the failure is
		// transient and attempts remain; otherwise it reports msg. Returns true when
		// a retry was scheduled (so the caller stops).
		retryOrFail := func(status int, msg string) {
			ms, ok := RetryDelayMS(n)
			if !IsRetryable(status) || !ok {
				onError(msg)
				return
			}
			var timer js.Func
			timer = js.FuncOf(func(js.Value, []js.Value) any {
				timer.Release()
				attempt(n + 1)
				return nil
			})
			js.Global().Call("setTimeout", timer, ms)
		}

		// status is captured from the response in onResp and read in onText.
		status := 0
		onResp = js.FuncOf(func(_ js.Value, args []js.Value) any {
			status = args[0].Get("status").Int()
			return args[0].Call("text") // a promise resolving to the response body
		})
		onText = js.FuncOf(func(_ js.Value, args []js.Value) any {
			data := []byte(args[0].String())
			release()
			switch {
			case status >= 400:
				retryOrFail(status, ErrorMessage(status, data))
			default:
				content, err := ParseResponse(data)
				if err != nil {
					onError(err.Error())
				} else {
					onResult(content, ParseUsage(data))
				}
			}
			return nil
		})
		onCatch = js.FuncOf(func(_ js.Value, args []js.Value) any {
			// fetch rejects on network failure or a blocked cross-origin request;
			// treat as status 0 (retryable).
			release()
			retryOrFail(0, "Couldn't reach OpenAI. Check your internet connection and try again.")
			return nil
		})

		js.Global().Call("fetch", baseURL+"/chat/completions", opts).
			Call("then", onResp).
			Call("then", onText).
			Call("catch", onCatch)
	}
	attempt(0)
}
