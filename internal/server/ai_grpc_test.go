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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		AuthMode:        "token",
		Token:           "dev-token",
		MasterKey:       "0123456789abcdef0123456789abcdef",
		OpenAIBaseURL:   upstream.URL,
		AIAllowedModels: []string{"gpt-4.1-mini", "gpt-4o-mini"},
		AppOrigin:       "*",
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

	var models backendrpc.ListModelsResponse
	if err := conn.Invoke(ctx, backendrpc.MethodAIListModels, backendrpc.ListModelsRequest{}, &models, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("ListModels invoke: %v", err)
	}
	if len(models.Models) != 2 || models.Models[0] != "gpt-4.1-mini" || models.Models[1] != "gpt-4o-mini" {
		t.Fatalf("ListModels response = %+v", models)
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

func TestAIServiceGRPCBridgeDisabled(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{
		AuthMode:        "token",
		Token:           "dev-token",
		MasterKey:       "0123456789abcdef0123456789abcdef",
		AIProxyDisabled: true,
		AppOrigin:       "*",
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

	var models backendrpc.ListModelsResponse
	err = conn.Invoke(ctx, backendrpc.MethodAIListModels, backendrpc.ListModelsRequest{}, &models, backendrpc.JSONCallOptions()...)
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("disabled ListModels err = %v, want failed precondition", err)
	}
}
