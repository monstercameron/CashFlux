//go:build js && wasm

package ai

import "syscall/js"

// DefaultBaseURL is the OpenAI API base; configurable for proxies/compat servers.
const DefaultBaseURL = "https://api.openai.com/v1"

// noopCancel is returned when a request never started (e.g. a build error), so
// callers can always invoke the returned cancel safely.
func noopCancel() {}

// SendChat posts a chat-completions request to baseURL using the user's apiKey,
// asynchronously. On success it calls onResult with the assistant's content and
// the call's token usage; on any failure (build, network, API, or empty response)
// it calls onError with a plain-English message. Exactly one of the callbacks
// runs. It returns a cancel function that aborts the in-flight request (and any
// pending retry); after cancel the callbacks won't fire. This is the only place
// that talks to the network; the request/response shaping is pure (ai.go).
func SendChat(apiKey, baseURL, model string, messages []Message, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	body, err := BuildRequest(model, messages, temperature)
	if err != nil {
		onError(err.Error())
		return noopCancel
	}
	return postCompletions(apiKey, baseURL, body, onResult, onError)
}

// SendVisionChat posts a multimodal chat-completions request (a system prompt, a
// user instruction, and one image as a data/URL) using a vision-capable model.
// Same async contract as SendChat; returns a cancel function.
func SendVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, onResult func(string, Usage), onError func(string)) func() {
	body, err := BuildVisionRequest(model, systemPrompt, userText, imageURL, temperature)
	if err != nil {
		onError(err.Error())
		return noopCancel
	}
	return postCompletions(apiKey, baseURL, body, onResult, onError)
}

// SendStructuredVisionChat is SendVisionChat that additionally constrains the
// reply to the given JSON schema (structured outputs). The reply's content is a
// JSON string matching the schema — decode it with json.Unmarshal. Same async
// contract; returns a cancel function.
func SendStructuredVisionChat(apiKey, baseURL, model, systemPrompt, userText, imageURL string, temperature float64, schemaName string, schema []byte, onResult func(string, Usage), onError func(string)) func() {
	body, err := BuildStructuredVisionRequest(model, systemPrompt, userText, imageURL, temperature, schemaName, schema)
	if err != nil {
		onError(err.Error())
		return noopCancel
	}
	return postCompletions(apiKey, baseURL, body, onResult, onError)
}

// postCompletions sends a prebuilt request body to the chat-completions endpoint
// and routes the parsed result (or a plain-English error) to the callbacks. It
// owns the fetch promise chain, releases its js.Funcs per attempt, and retries
// transient failures (429/5xx/network) with exponential backoff (see
// IsRetryable / RetryDelayMS), giving up with the last error after MaxRetries.
// It returns a cancel function: calling it aborts the in-flight fetch via an
// AbortController, clears any pending retry timer, and suppresses the callbacks.
func postCompletions(apiKey, baseURL string, body []byte, onResult func(string, Usage), onError func(string)) func() {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	controller := js.Global().Get("AbortController").New()
	opts := map[string]any{
		"method": "POST",
		"headers": map[string]any{
			"Authorization": "Bearer " + apiKey,
			"Content-Type":  "application/json",
		},
		"body":   string(body),
		"signal": controller.Get("signal"),
	}

	cancelled := false
	var pendingTimer js.Value
	timerSet := false
	cancel := func() {
		if cancelled {
			return
		}
		cancelled = true
		controller.Call("abort")
		if timerSet {
			js.Global().Call("clearTimeout", pendingTimer)
			timerSet = false
		}
	}

	var attempt func(n int)
	attempt = func(n int) {
		if cancelled {
			return
		}
		var onText, onResp, onCatch js.Func
		release := func() { onText.Release(); onResp.Release(); onCatch.Release() }

		// retryOrFail schedules another attempt after a backoff when the failure is
		// transient and attempts remain; otherwise it reports msg.
		retryOrFail := func(status int, msg string) {
			ms, ok := RetryDelayMS(n)
			if !IsRetryable(status) || !ok {
				onError(msg)
				return
			}
			var timer js.Func
			timer = js.FuncOf(func(js.Value, []js.Value) any {
				timer.Release()
				timerSet = false
				attempt(n + 1)
				return nil
			})
			pendingTimer = js.Global().Call("setTimeout", timer, ms)
			timerSet = true
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
			if cancelled {
				return nil
			}
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
			// fetch rejects on network failure, a blocked cross-origin request, or an
			// abort; a cancelled request stays silent, otherwise treat as retryable.
			release()
			if cancelled {
				return nil
			}
			retryOrFail(0, "Couldn't reach OpenAI. Check your internet connection and try again.")
			return nil
		})

		js.Global().Call("fetch", baseURL+"/chat/completions", opts).
			Call("then", onResp).
			Call("then", onText).
			Call("catch", onCatch)
	}
	attempt(0)
	return cancel
}
