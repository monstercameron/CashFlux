// SPDX-License-Identifier: MIT

//go:build js && wasm

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/monstercameron/CashFlux/internal/backendauth"
	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/cryptobox"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/store"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"google.golang.org/grpc/status"
)

const defaultBackendURL = "http://127.0.0.1:8081"

type backendVersionResponse struct {
	APIVersion          string   `json:"apiVersion"`
	MinClientAPIVersion string   `json:"minClientApiVersion"`
	AuthMode            string   `json:"authMode"`
	AuthProviders       []string `json:"authProviders"`
}

type billingSessionResponse struct {
	URL string `json:"url"`
}

func normalizedBackendEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		return defaultBackendURL
	}
	return endpoint
}

func backendOrigin(endpoint string) string {
	u, err := url.Parse(normalizedBackendEndpoint(endpoint))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func testBackendConnection(endpoint, token string, onDone func(backendauth.Discovery), onError func(string)) {
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
		onDone(backendauth.Discovery{AuthMode: version.AuthMode, AuthProviders: version.AuthProviders}.Normalize())
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

// removeOpenAIKeyFromBackend deletes the server-stored OpenAI key by sending an
// empty key to AISetKey — the server treats that as a remove (§7.11). onDone fires
// when the key is cleared (out.Stored == false).
func removeOpenAIKeyFromBackend(endpoint, token string, onDone func(), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if token == "" {
		onError("Sign in before removing the cloud key.")
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
		err = conn.Invoke(ctx, backendrpc.MethodAISetKey, backendrpc.SetKeyRequest{Provider: "openai", Key: ""}, &out, backendrpc.JSONCallOptions()...)
		if err == nil {
			onDone()
			return
		}
		if st, ok := status.FromError(err); ok && strings.TrimSpace(st.Message()) != "" {
			onError(st.Message())
			return
		}
		onError(err.Error())
	}()
}

func startOAuthLogin(endpoint, provider string, onDone func(token, csrf, userID string), onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	provider = strings.TrimSpace(provider)
	origin := backendOrigin(endpoint)
	if provider == "" || origin == "" {
		onError("Backend OAuth configuration is invalid.")
		return
	}
	window := js.Global().Get("window")
	listener := js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		if event.Get("origin").String() != origin {
			return nil
		}
		data := event.Get("data")
		if data.IsUndefined() || data.IsNull() || data.Get("type").String() != "cashflux.oauth" {
			return nil
		}
		token := strings.TrimSpace(data.Get("accessToken").String())
		csrf := strings.TrimSpace(data.Get("csrf").String())
		userID := strings.TrimSpace(data.Get("userId").String())
		if token == "" {
			onError("Backend OAuth response did not include an access token.")
			return nil
		}
		onDone(token, csrf, userID)
		return nil
	})
	window.Call("addEventListener", "message", listener)
	returnTo := js.Global().Get("location").Get("href").String()
	loginURL := endpoint + "/v1/auth/" + url.PathEscape(provider) + "?returnTo=" + url.QueryEscape(returnTo)
	popup := window.Call("open", loginURL, "cashflux-oauth", "popup,width=520,height=720")
	if popup.IsUndefined() || popup.IsNull() {
		onError("The browser blocked the OAuth sign-in window.")
	}
}

func signOutBackendOAuth(endpoint, token, csrf string, onDone func()) {
	endpoint = normalizedBackendEndpoint(endpoint)
	headers := js.Global().Get("Headers").New()
	if strings.TrimSpace(token) != "" {
		headers.Call("set", "Authorization", "Bearer "+strings.TrimSpace(token))
	}
	if strings.TrimSpace(csrf) != "" {
		headers.Call("set", "X-CashFlux-CSRF", strings.TrimSpace(csrf))
	}
	opts := js.Global().Get("Object").New()
	opts.Set("method", "POST")
	opts.Set("credentials", "include")
	opts.Set("headers", headers)
	js.Global().Call("fetch", endpoint+"/v1/auth/logout", opts)
	onDone()
}

func startBillingCheckout(endpoint, token, interval, provider string, onError func(string)) {
	path := "/v1/billing/checkout"
	// Only append the selector for a non-default provider so existing Stripe
	// requests are byte-for-byte unchanged.
	if p := strings.ToLower(strings.TrimSpace(provider)); p != "" && p != "stripe" {
		path += "?provider=" + url.QueryEscape(p)
	}
	createBillingSession(endpoint, token, path, map[string]string{"interval": strings.TrimSpace(interval)}, onError)
}

func openBillingPortal(endpoint, token string, onError func(string)) {
	createBillingSession(endpoint, token, "/v1/billing/portal", map[string]string{}, onError)
}

func createBillingSession(endpoint, token, path string, body map[string]string, onError func(string)) {
	endpoint = normalizedBackendEndpoint(endpoint)
	token = strings.TrimSpace(token)
	if token == "" {
		onError("Add a backend token before opening billing.")
		return
	}
	go func() {
		data, err := json.Marshal(body)
		if err != nil {
			onError("Billing request could not be prepared.")
			return
		}
		req, err := http.NewRequest(http.MethodPost, endpoint+path, bytes.NewReader(data))
		if err != nil {
			onError("Backend URL is invalid.")
			return
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
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
		var out billingSessionResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || strings.TrimSpace(out.URL) == "" {
			onError("Backend billing response was invalid.")
			return
		}
		js.Global().Get("location").Call("assign", strings.TrimSpace(out.URL))
	}()
}

func prepareBackendSyncDataset(ctx context.Context, endpoint, token, workspaceID string, data []byte) ([]byte, error) {
	ds, err := store.Import(data)
	if err != nil {
		return nil, err
	}
	changed := false
	for i := range ds.Artifacts {
		if len(ds.Artifacts[i].Bytes) == 0 {
			continue
		}
		ref, err := uploadBackendArtifactBlob(ctx, endpoint, token, workspaceID, ds.Artifacts[i])
		if err != nil {
			return nil, err
		}
		ds.Artifacts[i].BlobRef = &ref
		ds.Artifacts[i].Bytes = nil
		changed = true
	}
	if !changed {
		return data, nil
	}
	return store.Export(ds)
}

func hydrateBackendSyncDataset(ctx context.Context, endpoint, token, workspaceID string, data []byte) ([]byte, error) {
	ds, err := store.Import(data)
	if err != nil {
		return nil, err
	}
	changed := false
	for i := range ds.Artifacts {
		if len(ds.Artifacts[i].Bytes) > 0 || ds.Artifacts[i].BlobRef == nil || strings.TrimSpace(ds.Artifacts[i].BlobRef.Hash) == "" {
			continue
		}
		bytes, err := downloadBackendArtifactBlob(ctx, endpoint, token, workspaceID, ds.Artifacts[i].BlobRef.Hash)
		if err != nil {
			return nil, err
		}
		ds.Artifacts[i].Bytes = bytes
		if ds.Artifacts[i].Size == 0 {
			ds.Artifacts[i].Size = len(bytes)
		}
		changed = true
	}
	if !changed {
		return data, nil
	}
	return store.Export(ds)
}

func uploadBackendArtifactBlob(ctx context.Context, endpoint, token, workspaceID string, art domain.Artifact) (domain.BlobRef, error) {
	// When encryption is active the server stores ciphertext — it never sees the
	// plaintext bytes. The payload is the encrypted envelope; Content-Type is set
	// to application/octet-stream so the real MIME is not leaked. The artifact's
	// MIME is preserved in the dataset record (which is itself encrypted at rest),
	// so the client can still render the blob correctly after decryption.
	payload := art.Bytes
	contentType := strings.TrimSpace(art.MIME)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if datasetEncryptionActive() {
		enc, err := encryptArtifactSync(art.Bytes)
		if err != nil {
			return domain.BlobRef{}, fmt.Errorf("blob upload: encrypt artifact: %w", err)
		}
		payload = enc
		contentType = "application/octet-stream"
	}

	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	blobURL := normalizedBackendEndpoint(endpoint) + "/v1/blobs/" + hash + "?workspaceId=" + url.QueryEscape(workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, blobURL, bytes.NewReader(payload))
	if err != nil {
		return domain.BlobRef{}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return domain.BlobRef{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return domain.BlobRef{}, fmt.Errorf("blob upload returned HTTP %d", resp.StatusCode)
	}
	// Store the real MIME in BlobRef so the client can render the artifact. Size
	// reflects the on-wire payload (ciphertext) for storage accounting; the
	// artifact domain record carries the unencrypted Size separately.
	return domain.BlobRef{Hash: hash, MIME: art.MIME, Size: len(payload)}, nil
}

func downloadBackendArtifactBlob(ctx context.Context, endpoint, token, workspaceID, hash string) ([]byte, error) {
	blobURL := normalizedBackendEndpoint(endpoint) + "/v1/blobs/" + strings.TrimSpace(hash) + "?workspaceId=" + url.QueryEscape(workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("blob download returned HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Transparent decryption: if the server stored an encrypted envelope (EC2),
	// decrypt it now. Legacy plaintext blobs (IsEnvelope → false) pass through
	// unchanged, so old blobs uploaded before encryption was enabled still work.
	if cryptobox.IsEnvelope(raw) {
		plain, err := decryptArtifactSync(raw)
		if err != nil {
			return nil, fmt.Errorf("blob download: decrypt artifact: %w", err)
		}
		return plain, nil
	}
	return raw, nil
}
