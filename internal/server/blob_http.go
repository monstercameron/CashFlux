package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

const blobWorkspaceHeader = "X-CashFlux-Workspace-ID"

type BlobResponse struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Mime string `json:"mime"`
}

func handlePutBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedBlobRequest(w, r, cfg, store)
		if !ok {
			return
		}
		hash := r.PathValue("hash")
		if !validBlobHash(hash) {
			http.Error(w, "invalid blob hash", http.StatusBadRequest)
			return
		}
		workspaceID, ok := authorizedBlobWorkspace(w, r, store, user, hash, false)
		if !ok {
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
		mime, ok := safeBlobMIME(w, r.Header.Get("Content-Type"), data)
		if !ok {
			return
		}
		if !withinStorageQuota(w, store, user.ID, hash, int64(len(data)), cfg.StorageMaxBytes) {
			return
		}
		ctx, cancel := blobIOContext(r.Context(), cfg)
		defer cancel()
		blob, err := store.PutBlobContext(ctx, blobRoot(cfg), data, mime, "", cfg.BlobMaxBytes)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := store.LinkWorkspaceBlob(workspaceID, blob.Hash); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if cfg.Metrics != nil {
			cfg.Metrics.ObserveBlobStored(blob.Size)
		}
		auditFromRequest(r, store, user, "blob.put", "blob", blob.Hash)
		writeJSON(w, BlobResponse{Hash: blob.Hash, Size: blob.Size, Mime: blob.Mime})
	}
}

func withinStorageQuota(w http.ResponseWriter, store *Store, userID, hash string, size, maxBytes int64) bool {
	if maxBytes <= 0 {
		return true
	}
	linked, err := store.UserBlobLinked(userID, hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	if linked {
		return true
	}
	current, err := store.UserBlobBytes(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false
	}
	if current+size > maxBytes {
		http.Error(w, "storage quota exceeded", http.StatusInsufficientStorage)
		return false
	}
	return true
}

func handleGetBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedBlobRequest(w, r, cfg, store)
		if !ok {
			return
		}
		if _, ok := authorizedBlobWorkspace(w, r, store, user, r.PathValue("hash"), true); !ok {
			return
		}
		blob, data, ok := readHTTPBlob(w, r, cfg, store)
		if !ok {
			return
		}
		writeBlobHeaders(w, blob)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		if cfg.Metrics != nil {
			cfg.Metrics.ObserveBlobTransferred(blob.Size)
		}
	}
}

func handleHeadBlob(cfg Config, store *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := authorizedBlobRequest(w, r, cfg, store)
		if !ok {
			return
		}
		if _, ok := authorizedBlobWorkspace(w, r, store, user, r.PathValue("hash"), true); !ok {
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

func authorizedBlobWorkspace(w http.ResponseWriter, r *http.Request, store *Store, user AuthUser, hash string, requireLink bool) (string, bool) {
	workspaceID := strings.TrimSpace(r.URL.Query().Get("workspaceId"))
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(r.Header.Get(blobWorkspaceHeader))
	}
	if workspaceID == "" {
		http.Error(w, "workspace id is required", http.StatusBadRequest)
		return "", false
	}
	if !validBlobHash(hash) {
		http.Error(w, "invalid blob hash", http.StatusBadRequest)
		return "", false
	}
	SetLogScope(r.Context(), LogScope{WorkspaceID: workspaceID})
	if _, ok, err := store.GetWorkspace(user.ID, workspaceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return "", false
	} else if !ok {
		http.Error(w, "workspace not found", http.StatusNotFound)
		return "", false
	}
	if requireLink {
		linked, err := store.UserWorkspaceBlob(user.ID, workspaceID, hash)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return "", false
		}
		if !linked {
			http.Error(w, "blob not found", http.StatusNotFound)
			return "", false
		}
	}
	return workspaceID, true
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
	ctx, cancel := blobIOContext(r.Context(), cfg)
	defer cancel()
	data, err := store.ReadBlobContext(ctx, blobRoot(cfg), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return Blob{}, nil, false
	}
	return blob, data, true
}

func blobIOContext(parent context.Context, cfg Config) (context.Context, context.CancelFunc) {
	if cfg.BlobIOTimeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, cfg.BlobIOTimeout)
}

func writeBlobHeaders(w http.ResponseWriter, blob Blob) {
	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Disposition", "attachment")
	w.Header().Set("ETag", `"`+blob.Hash+`"`)
}

func safeBlobMIME(w http.ResponseWriter, declared string, data []byte) (string, bool) {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	if declared != "" && forbiddenBlobMIME(declared) {
		http.Error(w, "blob content type is not allowed", http.StatusUnsupportedMediaType)
		return "", false
	}
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if forbiddenBlobMIME(sniffed) {
		http.Error(w, "blob content type is not allowed", http.StatusUnsupportedMediaType)
		return "", false
	}
	if declared != "" && declared != "application/octet-stream" {
		return declared, true
	}
	if sniffed == "" {
		return "application/octet-stream", true
	}
	return sniffed, true
}

func forbiddenBlobMIME(mime string) bool {
	switch mime {
	case "text/html", "application/xhtml+xml", "image/svg+xml":
		return true
	default:
		return false
	}
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
