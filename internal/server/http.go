package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// VersionResponse is returned by /v1/version for client compatibility checks.
type VersionResponse struct {
	APIVersion          string `json:"apiVersion"`
	MinClientAPIVersion string `json:"minClientApiVersion"`
	AuthMode            string `json:"authMode"`
	BillingEnabled      bool   `json:"billingEnabled"`
}

type AIKeyRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type AIKeyResponse struct {
	Stored   bool   `json:"stored"`
	Provider string `json:"provider"`
}

// NewMux returns the backend HTTP surface that exists before gRPC/proto wiring.
func NewMux(cfg Config, stores ...*Store) http.Handler {
	var store *Store
	if len(stores) > 0 {
		store = stores[0]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, VersionResponse{
			APIVersion:          APIVersion,
			MinClientAPIVersion: MinClientAPIVersion,
			AuthMode:            cfg.AuthMode,
			BillingEnabled:      cfg.Billing,
		})
	})
	mux.HandleFunc("OPTIONS /v1/ai/key", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /v1/ai/key", func(w http.ResponseWriter, r *http.Request) {
		if !writeCORS(w, r, cfg) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if store == nil {
			http.Error(w, "store is not configured", http.StatusServiceUnavailable)
			return
		}
		if strings.TrimSpace(cfg.MasterKey) == "" {
			http.Error(w, "master key is not configured", http.StatusServiceUnavailable)
			return
		}
		user, ok := httpBearerUser(r, cfg)
		if !ok {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		var body AIKeyRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		body.Provider = strings.TrimSpace(body.Provider)
		if body.Provider == "" {
			body.Provider = "openai"
		}
		if body.Provider != "openai" || !strings.HasPrefix(strings.TrimSpace(body.Key), "sk-") {
			http.Error(w, "invalid openai key", http.StatusBadRequest)
			return
		}
		if err := store.UpsertUser(User{ID: user.ID, Provider: "token", Subject: user.ID, CreatedAt: time.Now().UTC()}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := store.PutAIKey(user.ID, body.Provider, strings.TrimSpace(body.Key), []byte(cfg.MasterKey)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, AIKeyResponse{Stored: true, Provider: body.Provider})
	})
	mux.HandleFunc("POST /v1/ai/chat", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedAIRequest(w, r, cfg, store)
		if !ok {
			return
		}
		var body AIChatRequest
		if !readJSON(w, r, &body) {
			return
		}
		svc := NewAIService(store, AIServiceConfig{MasterKey: []byte(cfg.MasterKey), BaseURL: cfg.OpenAIBaseURL})
		result, err := svc.Chat(ContextWithAuthUser(r.Context(), user), body)
		if err != nil {
			writeStatusError(w, err)
			return
		}
		writeJSON(w, result)
	})
	mux.HandleFunc("POST /v1/ai/vision", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedAIRequest(w, r, cfg, store)
		if !ok {
			return
		}
		var body AIVisionRequest
		if !readJSON(w, r, &body) {
			return
		}
		svc := NewAIService(store, AIServiceConfig{MasterKey: []byte(cfg.MasterKey), BaseURL: cfg.OpenAIBaseURL})
		result, err := svc.Vision(ContextWithAuthUser(r.Context(), user), body)
		if err != nil {
			writeStatusError(w, err)
			return
		}
		writeJSON(w, result)
	})
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func authorizedAIRequest(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	if !writeCORS(w, r, cfg) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return AuthUser{}, false
	}
	if store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return AuthUser{}, false
	}
	if strings.TrimSpace(cfg.MasterKey) == "" {
		http.Error(w, "master key is not configured", http.StatusServiceUnavailable)
		return AuthUser{}, false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return AuthUser{}, false
	}
	return user, true
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}

func writeStatusError(w http.ResponseWriter, err error) {
	if st, ok := status.FromError(err); ok {
		http.Error(w, st.Message(), grpcHTTPStatus(st.Code()))
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func grpcHTTPStatus(code codes.Code) int {
	switch code {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Canceled:
		return 499
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func httpBearerUser(r *http.Request, cfg Config) (AuthUser, bool) {
	header := r.Header.Get("Authorization")
	fields := strings.Fields(header)
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") || strings.TrimSpace(fields[1]) == "" {
		return AuthUser{}, false
	}
	token := strings.TrimSpace(fields[1])
	expected := strings.TrimSpace(cfg.Token)
	if expected == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
		return AuthUser{}, false
	}
	sum := sha256.Sum256([]byte(token))
	id := "token:" + hex.EncodeToString(sum[:])[:24]
	return AuthUser{ID: id, Token: token}, true
}

func writeCORS(w http.ResponseWriter, r *http.Request, cfg Config) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if !allowedOrigin(origin, cfg.AppOrigin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	return true
}

func allowedOrigin(origin, configured string) bool {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		return origin == configured
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://[::1]:")
}
