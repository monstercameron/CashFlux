// SPDX-License-Identifier: MIT

// Package backendrpc defines the browser/server RPC contract used over the
// GoGRPCBridge websocket tunnel.
package backendrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// FunctionDef describes a tool the caller can run on the model's behalf: its name,
// the plain-English description the model uses to decide when to call it, and a
// JSON Schema for its arguments.
type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// Tool wraps a function definition in the tools envelope.
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionCall is the function name and raw JSON arguments the model wants run.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall is one tool invocation the model requested in an assistant turn.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// Message is one chat turn on the wire. ToolCalls carry an assistant turn's
// requested calls and ToolCallID keys a tool result back to its call — without
// them a tool conversation could not survive the trip through the proxy, which is
// why the server path used to answer without tools at all.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string     `json:"toolCallId,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Temperature     float64   `json:"temperature,omitempty"`
	Tools           []Tool    `json:"tools,omitempty"`
	ReasoningEffort string    `json:"reasoningEffort,omitempty"`
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
	Content      string     `json:"content"`
	Usage        Usage      `json:"usage"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason,omitempty"`
}

type CompletionChunk struct {
	Content      string     `json:"content,omitempty"`
	Usage        Usage      `json:"usage,omitempty"`
	ToolCalls    []ToolCall `json:"toolCalls,omitempty"`
	FinishReason string     `json:"finishReason,omitempty"`
	Done         bool       `json:"done,omitempty"`
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
