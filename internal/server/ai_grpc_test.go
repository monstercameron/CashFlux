// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/ai"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAIServiceGRPCBridgeSetKeyAndChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-grpc-secret" {
			t.Fatalf("authorization = %q", got)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upstream body: %v", err)
		}
		body, err := decodeResponsesRequest(string(raw))
		if err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		input, _ := body["input"].([]any)
		if body["model"] != "gpt-5.4-mini" || len(input) != 1 {
			t.Fatalf("upstream body = %+v", body)
		}
		if first, _ := input[0].(map[string]any); first["content"] != "hello grpc" {
			t.Fatalf("upstream input = %v", input[0])
		}
		_, _ = w.Write([]byte(responsesTextReply("grpc says hi", 6, 7)))
	}))
	defer upstream.Close()

	store := openTestStore(t)
	cfg := Config{
		AuthMode:        "token",
		Token:           "dev-token",
		MasterKey:       "0123456789abcdef0123456789abcdef",
		OpenAIBaseURL:   upstream.URL,
		AIAllowedModels: []string{"gpt-5.4-mini", "gpt-5.5"},
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
	if len(models.Models) != 2 || models.Models[0] != "gpt-5.4-mini" || models.Models[1] != "gpt-5.5" {
		t.Fatalf("ListModels response = %+v", models)
	}

	var chatResp backendrpc.Completion
	err = conn.Invoke(ctx, backendrpc.MethodAIChat, backendrpc.ChatRequest{
		Model:       "gpt-5.4-mini",
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

func TestAIServiceGRPCBridgeChatStream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-stream-secret" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(responsesStream([]string{"stream ", "says ", "hi"}, 10, 11)))
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
	if err := conn.Invoke(ctx, backendrpc.MethodAISetKey, backendrpc.SetKeyRequest{Provider: "openai", Key: "sk-stream-secret"}, &keyResp, backendrpc.JSONCallOptions()...); err != nil {
		t.Fatalf("SetKey invoke: %v", err)
	}
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodAIChatStream, backendrpc.JSONCallOptions()...)
	if err != nil {
		t.Fatalf("ChatStream stream: %v", err)
	}
	if err := stream.SendMsg(&backendrpc.ChatRequest{
		Model:    "gpt-5.4-mini",
		Messages: []backendrpc.Message{{Role: ai.RoleUser, Content: "hello stream"}},
	}); err != nil {
		t.Fatalf("ChatStream send: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("ChatStream close send: %v", err)
	}
	// The answer arrives in fragments and the last chunk carries the accounting —
	// that IS the streaming behaviour, so the test reads it as a stream rather than
	// as one message.
	var text strings.Builder
	var last backendrpc.CompletionChunk
	chunks := 0
	for {
		var chunk backendrpc.CompletionChunk
		if err := stream.RecvMsg(&chunk); err != nil {
			t.Fatalf("ChatStream recv: %v", err)
		}
		chunks++
		text.WriteString(chunk.Content)
		if chunk.Done {
			last = chunk
			break
		}
		if chunks > 10 {
			t.Fatal("ChatStream never finished")
		}
	}
	if chunks < 2 {
		t.Fatalf("the answer arrived in %d chunk(s) — it was not streamed", chunks)
	}
	if text.String() != "stream says hi" {
		t.Fatalf("streamed text = %q", text.String())
	}
	if last.Usage.TotalTokens != 21 {
		t.Fatalf("final chunk usage = %+v", last.Usage)
	}
	if last.Content != "" {
		t.Fatalf("the final chunk repeated the text, which would double it on screen: %q", last.Content)
	}
	var extra backendrpc.CompletionChunk
	if err := stream.RecvMsg(&extra); err != io.EOF {
		t.Fatalf("ChatStream second recv = %v/%+v, want EOF", err, extra)
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

func TestAIServiceGRPCBridgeRejectsOversizedKey(t *testing.T) {
	store := openTestStore(t)
	cfg := Config{
		AuthMode:  "token",
		Token:     "dev-token",
		MasterKey: "0123456789abcdef0123456789abcdef",
		AppOrigin: "*",
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
	err = conn.Invoke(ctx, backendrpc.MethodAISetKey, backendrpc.SetKeyRequest{
		Provider: "openai",
		Key:      "sk-" + strings.Repeat("x", maxAIKeyLength),
	}, &keyResp, backendrpc.JSONCallOptions()...)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("oversized SetKey err = %v code %v, want invalid argument", err, status.Code(err))
	}
	if _, ok, err := store.GetAIKey(authUserFromToken("dev-token").ID, "openai", []byte(cfg.MasterKey)); err != nil || ok {
		t.Fatalf("stored key ok=%v err=%v, want none", ok, err)
	}
}
