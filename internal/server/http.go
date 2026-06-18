package server

import (
	"encoding/json"
	"net/http"
)

// VersionResponse is returned by /v1/version for client compatibility checks.
type VersionResponse struct {
	APIVersion          string `json:"apiVersion"`
	MinClientAPIVersion string `json:"minClientApiVersion"`
	AuthMode            string `json:"authMode"`
	BillingEnabled      bool   `json:"billingEnabled"`
}

// NewMux returns the backend HTTP surface that exists before gRPC/proto wiring.
func NewMux(cfg Config) http.Handler {
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
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
