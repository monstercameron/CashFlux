//go:build js && wasm

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/status"
)

const defaultBackendURL = "http://127.0.0.1:8081"

type backendVersionResponse struct {
	APIVersion          string `json:"apiVersion"`
	MinClientAPIVersion string `json:"minClientApiVersion"`
	AuthMode            string `json:"authMode"`
}

func normalizedBackendEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return defaultBackendURL
	}
	return endpoint
}

func testBackendConnection(endpoint, token string, onDone func(string), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	go func() {
		req, err := http.NewRequest(http.MethodGet, endpoint+"/v1/version", nil)
		if err != nil {
			onError("Backend URL is invalid.")
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			onError("Couldn't reach the backend server.")
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			onError(fmt.Sprintf("Backend returned HTTP %d.", resp.StatusCode))
			return
		}
		var version backendVersionResponse
		if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
			onError("Backend version response was invalid.")
			return
		}
		if version.APIVersion != "v1" || version.MinClientAPIVersion != "v1" {
			onError("Backend API version is not compatible.")
			return
		}
		if version.AuthMode == "" {
			version.AuthMode = "token"
		}
		onDone(version.AuthMode)
	}()
}

func uploadOpenAIKeyToBackend(endpoint, token, key string, onDone func(), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
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
