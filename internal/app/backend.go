//go:build js && wasm

package app

import (
	"context"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/status"
)

const defaultBackendURL = "http://127.0.0.1:8081"

func uploadOpenAIKeyToBackend(endpoint, token, key string, onDone func(), onError func(string)) {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = defaultBackendURL
	}
	token = strings.TrimSpace(token)
	key = strings.TrimSpace(key)
	if token == "" {
		onError("Add a backend token before uploading the key.")
		return
	}
	if key == "" {
		onError("Add your OpenAI key before uploading it.")
		return
	}
	go func() {
		ctx := context.Background()
		conn, err := syncbridge.Dial(ctx, syncbridge.Config{ServerURL: endpoint, Token: token})
		if err != nil {
			onError("Couldn't reach the backend server.")
			return
		}
		defer conn.Close()
		var out backendrpc.SetKeyResponse
		err = conn.Invoke(ctx, backendrpc.MethodAISetKey, backendrpc.SetKeyRequest{Provider: "openai", Key: key}, &out, backendrpc.JSONCallOptions()...)
		if err == nil && out.Stored {
			onDone()
			return
		}
		if err == nil {
			onError("Backend rejected the key upload.")
			return
		}
		if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
			onError(st.Message())
			return
		}
		onError(err.Error())
	}()
}
