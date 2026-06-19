package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
)

func TestAIServiceGRPCBridgeSetKeyAndChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-grpc-secret" {
			t.Fatalf("authorization = %q", got)
		}
		var body ai.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body.Model != "gpt-4o-mini" || len(body.Messages) != 1 || body.Messages[0].Content != "hello grpc" {
			t.Fatalf("upstream body = %+v", body)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"grpc says hi"}}],"usage":{"total_tokens":13}}`))
	}))
	defer upstream.Close()

	store := openTestStore(t)
	cfg := Config{
		AuthMode:      "token",
		Token:         "dev-token",
		MasterKey:     "0123456789abcdef0123456789abcdef",
		OpenAIBaseURL: upstream.URL,
		AppOrigin:     "*",
	}
	bridge := httptest.NewServer(NewMux(cfg, store))
	defer bridge.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: bridge.URL, Token: "dev-token"})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	var keyResp backendrpc.SetKeyResponse
	if err := conn.Invoke(ctx, backendrpc.MethodAISetKey, backendrpc.SetKeyRequest{Provider: "openai", Key: "sk-grpc-secret"}, &keyResp, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("SetKey invoke: %v", err)
	}
	if !keyResp.Stored || keyResp.Provider != "openai" {
		t.Fatalf("SetKey response = %+v", keyResp)
	}

	var chatResp backendrpc.Completion
	err = conn.Invoke(ctx, backendrpc.MethodAIChat, backendrpc.ChatRequest{
		Model:       "gpt-4o-mini",
		Messages:    []backendrpc.Message{{Role: ai.RoleUser, Content: "hello grpc"}},
		Temperature: 0.2,
	}, &chatResp, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("Chat invoke: %v", err)
	}
	if chatResp.Content != "grpc says hi" || chatResp.Usage.TotalTokens != 13 {
		t.Fatalf("Chat response = %+v", chatResp)
	}
}
