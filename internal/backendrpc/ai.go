// Package backendrpc defines the browser/server RPC contract used over the
// GoGRPCBridge websocket tunnel.
package backendrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const (
	MethodAISetKey       = "/cashflux.v1.AIService/SetKey"
	MethodAIListModels   = "/cashflux.v1.AIService/ListModels"
	MethodAIChat         = "/cashflux.v1.AIService/Chat"
	MethodAIVision       = "/cashflux.v1.AIService/Vision"
	MethodAIChatStream   = "/cashflux.v1.AIService/ChatStream"
	MethodAIVisionStream = "/cashflux.v1.AIService/VisionStream"

	MethodSyncListWorkspaces  = "/cashflux.v1.SyncService/ListWorkspaces"
	MethodSyncGetWorkspace    = "/cashflux.v1.SyncService/GetWorkspace"
	MethodSyncPutWorkspace    = "/cashflux.v1.SyncService/PutWorkspace"
	MethodSyncDeleteWorkspace = "/cashflux.v1.SyncService/DeleteWorkspace"
	MethodSyncWatchWorkspaces = "/cashflux.v1.SyncService/WatchWorkspaces"
)

type SetKeyRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type SetKeyResponse struct {
	Stored   bool   `json:"stored"`
	Provider string `json:"provider"`
}

type ListModelsRequest struct{}

type ListModelsResponse struct {
	Models []string `json:"models"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type VisionRequest struct {
	Model        string          `json:"model"`
	SystemPrompt string          `json:"systemPrompt"`
	UserText     string          `json:"userText"`
	ImageURL     string          `json:"imageUrl"`
	Temperature  float64         `json:"temperature,omitempty"`
	SchemaName   string          `json:"schemaName,omitempty"`
	Schema       json.RawMessage `json:"schema,omitempty"`
}

type Completion struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

type CompletionChunk struct {
	Content string `json:"content,omitempty"`
	Usage   Usage  `json:"usage,omitempty"`
	Done    bool   `json:"done,omitempty"`
}

type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color,omitempty"`
	Sort      int    `json:"sort,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
	Version   int64  `json:"version,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
}

type ListWorkspacesRequest struct {
	IncludeDeleted bool `json:"includeDeleted,omitempty"`
}

type ListWorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type GetWorkspaceRequest struct {
	ID          string `json:"id"`
	IfNoneMatch string `json:"ifNoneMatch,omitempty"`
}

type GetWorkspaceResponse struct {
	Found       bool      `json:"found"`
	NotModified bool      `json:"notModified,omitempty"`
	ETag        string    `json:"etag,omitempty"`
	Workspace   Workspace `json:"workspace,omitempty"`
	Dataset     []byte    `json:"dataset,omitempty"`
}

type PutWorkspaceRequest struct {
	Workspace       Workspace `json:"workspace"`
	Dataset         []byte    `json:"dataset,omitempty"`
	ClientUpdatedAt string    `json:"clientUpdatedAt,omitempty"`
	Force           bool      `json:"force,omitempty"`
}

type PutWorkspaceResponse struct {
	Accepted  bool      `json:"accepted"`
	Workspace Workspace `json:"workspace"`
	Dataset   []byte    `json:"dataset,omitempty"`
	Version   int64     `json:"version"`
	UpdatedAt string    `json:"updatedAt"`
}

type DeleteWorkspaceRequest struct {
	ID        string `json:"id"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	DeviceID  string `json:"deviceId,omitempty"`
}

type DeleteWorkspaceResponse struct {
	Deleted bool `json:"deleted"`
}

type WatchWorkspacesRequest struct {
	IncludeDeleted bool `json:"includeDeleted,omitempty"`
}

type WatchWorkspacesResponse struct {
	Workspace Workspace `json:"workspace"`
}

type JSONCodec struct{}

func (JSONCodec) Marshal(v any) ([]byte, error) { return json.Marshal(v) }
func (JSONCodec) Unmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("backendrpc: JSON message must contain a single object")
	}
	return nil
}
func (JSONCodec) Name() string { return "json" }

func init() {
	encoding.RegisterCodec(JSONCodec{})
}

func JSONCallOptions() []grpc.CallOption {
	return []grpc.CallOption{grpc.ForceCodec(JSONCodec{})}
}
