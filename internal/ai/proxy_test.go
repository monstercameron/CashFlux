// SPDX-License-Identifier: MIT

package ai

import (
	"encoding/json"
	"testing"
)

func TestBuildProxyChatRequest(t *testing.T) {
	raw, err := BuildProxyChatRequest("gpt-4o-mini", []Message{{Role: RoleUser, Content: "hello"}}, 0.2)
	if err != nil {
		t.Fatalf("BuildProxyChatRequest: %v", err)
	}
	var req ChatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("decode proxy chat: %v", err)
	}
	if req.Model != "gpt-4o-mini" || len(req.Messages) != 1 || req.Messages[0].Content != "hello" || req.Temperature != 0.2 {
		t.Fatalf("proxy chat = %+v", req)
	}
}

func TestBuildProxyStructuredVisionRequest(t *testing.T) {
	raw, err := BuildProxyStructuredVisionRequest(" gpt-4o ", "sys", "extract", "data:image/png;base64,AAAA", 0.1, " rows ", []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatalf("BuildProxyStructuredVisionRequest: %v", err)
	}
	var req ProxyVisionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("decode proxy vision: %v", err)
	}
	if req.Model != "gpt-4o" || req.SchemaName != "rows" || req.ImageURL != "data:image/png;base64,AAAA" {
		t.Fatalf("proxy vision = %+v", req)
	}
	if string(req.Schema) != `{"type":"object"}` {
		t.Fatalf("schema = %s", req.Schema)
	}
}
