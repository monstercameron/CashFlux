// SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"fmt"
	"strings"
)

// responsesTextReply renders a Responses-API reply carrying one plain text answer,
// which is what the proxy now receives for reasoning models and tool turns. Tests
// that stub the upstream use this instead of hand-writing the shape, so a change to
// what the parser expects breaks in one place.
func responsesTextReply(text string, inTokens, outTokens int) string {
	body, err := json.Marshal(map[string]any{
		"status": "completed",
		"output": []any{map[string]any{
			"type":    "message",
			"role":    "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": text}},
		}},
		"usage": map[string]any{"input_tokens": inTokens, "output_tokens": outTokens},
	})
	if err != nil {
		panic(fmt.Sprintf("responsesTextReply: %v", err))
	}
	return string(body)
}

// responsesToolCallReply renders a Responses-API reply in which the model asks for
// one function call instead of answering — the turn that used to be impossible over
// the proxy at all.
func responsesToolCallReply(callID, name, arguments string, inTokens, outTokens int) string {
	body, err := json.Marshal(map[string]any{
		"status": "completed",
		"output": []any{map[string]any{
			"type":      "function_call",
			"call_id":   callID,
			"name":      name,
			"arguments": arguments,
		}},
		"usage": map[string]any{"input_tokens": inTokens, "output_tokens": outTokens},
	})
	if err != nil {
		panic(fmt.Sprintf("responsesToolCallReply: %v", err))
	}
	return string(body)
}

// responsesIncompleteReply renders the reply that started all of this: a call that
// succeeded and was billed, but whose output is empty because the model spent the
// whole budget reasoning.
func responsesIncompleteReply(reason string, inTokens, outTokens int) string {
	body, err := json.Marshal(map[string]any{
		"status":             "incomplete",
		"incomplete_details": map[string]any{"reason": reason},
		"output":             []any{map[string]any{"type": "reasoning", "summary": []any{}}},
		"usage":              map[string]any{"input_tokens": inTokens, "output_tokens": outTokens},
	})
	if err != nil {
		panic(fmt.Sprintf("responsesIncompleteReply: %v", err))
	}
	return string(body)
}

// decodeResponsesRequest reads a captured /responses request body so a test can
// assert what the proxy actually sent upstream.
func decodeResponsesRequest(body string) (map[string]any, error) {
	var out map[string]any
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// responsesStream renders a server-sent-event body: the text arriving in
// fragments, then the completed response. This is what the proxy now asks OpenAI
// for, so a stub that returns one JSON object no longer exercises the real path.
func responsesStream(fragments []string, inTokens, outTokens int) string {
	var b strings.Builder
	for _, f := range fragments {
		payload, err := json.Marshal(map[string]any{"type": "response.output_text.delta", "delta": f})
		if err != nil {
			panic(fmt.Sprintf("responsesStream: %v", err))
		}
		b.WriteString("event: response.output_text.delta\n")
		b.WriteString("data: " + string(payload) + "\n\n")
	}
	var full strings.Builder
	for _, f := range fragments {
		full.WriteString(f)
	}
	done, err := json.Marshal(map[string]any{
		"type": "response.completed",
		"response": map[string]any{
			"status": "completed",
			"output": []any{map[string]any{
				"type": "message", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": full.String()}},
			}},
			"usage": map[string]any{"input_tokens": inTokens, "output_tokens": outTokens},
		},
	})
	if err != nil {
		panic(fmt.Sprintf("responsesStream: %v", err))
	}
	b.WriteString("event: response.completed\n")
	b.WriteString("data: " + string(done) + "\n\n")
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}
