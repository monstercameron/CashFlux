package server

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type BlobResponse struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Mime string `json:"mime"`
}

func handlePutBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizedBlobRequest(w, r, cfg, store); !ok {
			return
		}
		hash := r.PathValue("hash")
		if !validBlobHash(hash) {
			http.Error(w, "invalid blob hash", http.StatusBadRequest)
			return
		}
		reader := r.Body
		if cfg.BlobMaxBytes > 0 {
			reader = http.MaxBytesReader(w, r.Body, cfg.BlobMaxBytes)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, "blob is too large", http.StatusRequestEntityTooLarge)
			return
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != hash {
			http.Error(w, "blob hash mismatch", http.StatusBadRequest)
			return
		}
		mime := strings.TrimSpace(r.Header.Get("Content-Type"))
		if mime == "" {
			mime = "application/octet-stream"
		}
		blob, err := store.PutBlob(blobRoot(cfg), data, mime, "", cfg.BlobMaxBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, BlobResponse{Hash: blob.Hash, Size: blob.Size, Mime: blob.Mime})
	}
}

func handleGetBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizedBlobRequest(w, r, cfg, store); !ok {
			return
		}
		blob, data, ok := readHTTPBlob(w, r, cfg, store)
		if !ok {
			return
		}
		writeBlobHeaders(w, blob)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func handleHeadBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authorizedBlobRequest(w, r, cfg, store); !ok {
			return
		}
		blob, _, ok := readHTTPBlob(w, r, cfg, store)
		if !ok {
			return
		}
		writeBlobHeaders(w, blob)
		w.WriteHeader(http.StatusOK)
	}
}

func authorizedBlobRequest(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (AuthUser, bool) {
	if !writeCORS(w, r, cfg) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return AuthUser{}, false
	}
	if store == nil {
		http.Error(w, "store is not configured", http.StatusServiceUnavailable)
		return AuthUser{}, false
	}
	user, ok := httpBearerUser(r, cfg)
	if !ok {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return AuthUser{}, false
	}
	SetLogScope(r.Context(), LogScope{UserID: user.ID})
	return user, true
}

func readHTTPBlob(w http.ResponseWriter, r *http.Request, cfg Config, store *Store) (Blob, []byte, bool) {
	hash := r.PathValue("hash")
	if !validBlobHash(hash) {
		http.Error(w, "invalid blob hash", http.StatusBadRequest)
		return Blob{}, nil, false
	}
	blob, ok, err := store.GetBlob(hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return Blob{}, nil, false
	}
	if !ok {
		http.Error(w, "blob not found", http.StatusNotFound)
		return Blob{}, nil, false
	}
	data, err := store.ReadBlob(blobRoot(cfg), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return Blob{}, nil, false
	}
	return blob, data, true
}

func writeBlobHeaders(w http.ResponseWriter, blob Blob) {
	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+blob.Hash+`"`)
}

func blobRoot(cfg Config) string {
	return filepath.Join(cfg.DataDir, "blobs")
}

func validBlobHash(hash string) bool {
	if len(hash) != sha256.Size*2 {
		return false
	}
	for _, c := range hash {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
