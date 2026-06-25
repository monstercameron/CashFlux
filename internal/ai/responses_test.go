// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"strings"
	"testing"
)

// cannedResponsesReply is a realistic Responses API payload containing:
//   - a web_search_call item (should be ignored by ParseResponsesText)
//   - a message item with one output_text content block
//   - a usage block
const cannedResponsesReply = `{
  "id": "resp_01HXmtest",
  "object": "response",
  "model": "gpt-5.5",
  "output": [
    {
      "type": "web_search_call",
      "id": "ws_01abc",
      "status": "completed"
    },
    {
      "type": "message",
      "id": "msg_01def",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "{\"base\":\"USD\",\"asOf\":\"2026-06-24\",\"rates\":{\"EUR\":1.08,\"GBP\":1.27,\"JPY\":0.0067}}"
        }
      ]
    }
  ],
  "usage": {
    "input_tokens": 512,
    "output_tokens": 64
  }
}`

func TestBuildResponsesWebSearchRequest(t *testing.T) {
	t.Run("marshals model, tools, and input", func(t *testing.T) {
		data, err := BuildResponsesWebSearchRequest("gpt-5.5", "What are today's FX rates?")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("result is not valid JSON: %v", err)
		}

		if got["model"] != "gpt-5.5" {
			t.Errorf("model = %v, want gpt-5.5", got["model"])
		}
		if got["input"] != "What are today's FX rates?" {
			t.Errorf("input = %v, want correct prompt", got["input"])
		}
		tools, ok := got["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("tools should be a single-element array, got %v", got["tools"])
		}
		tool, ok := tools[0].(map[string]any)
		if !ok || tool["type"] != "web_search" {
			t.Errorf("tools[0].type = %v, want web_search", tools[0])
		}
	})

	t.Run("empty input is still valid", func(t *testing.T) {
		_, err := BuildResponsesWebSearchRequest("gpt-5.5", "")
		if err != nil {
			t.Errorf("unexpected error for empty input: %v", err)
		}
	})
}

func TestParseResponsesText(t *testing.T) {
	t.Run("extracts text from message item, skips web_search_call", func(t *testing.T) {
		text, usage, err := ParseResponsesText([]byte(cannedResponsesReply))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(text, `"EUR":1.08`) {
			t.Errorf("expected EUR rate in text, got: %s", text)
		}
		// web_search_call item should contribute nothing to text
		if strings.Contains(text, "web_search_call") {
			t.Error("web_search_call content leaked into text output")
		}
		if usage.PromptTokens != 512 {
			t.Errorf("PromptTokens = %d, want 512", usage.PromptTokens)
		}
		if usage.CompletionTokens != 64 {
			t.Errorf("CompletionTokens = %d, want 64", usage.CompletionTokens)
		}
		if usage.TotalTokens != 576 {
			t.Errorf("TotalTokens = %d, want 576", usage.TotalTokens)
		}
	})

	t.Run("surfaces API error", func(t *testing.T) {
		errPayload := `{"output":[],"error":{"message":"invalid_api_key","type":"auth_error","code":"401"}}`
		_, _, err := ParseResponsesText([]byte(errPayload))
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid_api_key") {
			t.Errorf("error message does not contain API error text: %v", err)
		}
	})

	t.Run("returns error when no text output", func(t *testing.T) {
		emptyOutput := `{"output":[{"type":"web_search_call","id":"ws_1"}],"usage":{"input_tokens":10,"output_tokens":0}}`
		_, _, err := ParseResponsesText([]byte(emptyOutput))
		if err == nil {
			t.Fatal("expected error for empty text output, got nil")
		}
	})

	t.Run("returns error on malformed JSON", func(t *testing.T) {
		_, _, err := ParseResponsesText([]byte("{not json"))
		if err == nil {
			t.Fatal("expected error for malformed JSON, got nil")
		}
	})

	t.Run("concatenates multiple output_text blocks", func(t *testing.T) {
		multi := `{
			"output":[
				{"type":"message","content":[
					{"type":"output_text","text":"Hello "},
					{"type":"output_text","text":"World"}
				]}
			],
			"usage":{"input_tokens":5,"output_tokens":2}
		}`
		text, _, err := ParseResponsesText([]byte(multi))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if text != "Hello World" {
			t.Errorf("concatenation failed, got: %q", text)
		}
	})
}
