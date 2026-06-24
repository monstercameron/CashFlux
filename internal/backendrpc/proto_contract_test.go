// SPDX-License-Identifier: MIT

package backendrpc

import (
	"os"
	"strings"
	"testing"

	backendrpcpb "github.com/monstercameron/CashFlux/internal/backendrpc/pb/cashflux/v1"
)

func TestProtoContractCoversBackendRPCMethods(t *testing.T) {
	data, err := os.ReadFile("../../proto/cashflux/v1/cashflux.proto")
	if err != nil {
		t.Fatalf("read backend proto: %v", err)
	}
	proto := string(data)
	for _, want := range []string{
		"service SyncService",
		"rpc ListWorkspaces",
		"rpc GetWorkspace",
		"rpc PutWorkspace",
		"rpc DeleteWorkspace",
		"rpc WatchWorkspaces",
		"service AIService",
		"rpc SetKey",
		"rpc ListModels",
		"rpc Chat",
		"rpc Vision",
		"rpc ChatStream",
		"rpc VisionStream",
	} {
		if !strings.Contains(proto, want) {
			t.Fatalf("backend proto missing %q", want)
		}
	}
}

func TestProtoContractKeepsDatasetOpaque(t *testing.T) {
	data, err := os.ReadFile("../../proto/cashflux/v1/cashflux.proto")
	if err != nil {
		t.Fatalf("read backend proto: %v", err)
	}
	proto := string(data)
	for _, want := range []string{
		"message DatasetEnvelope",
		"bytes gzipped_json = 2;",
		"message BlobRef",
		"bytes dataset = 2;",
	} {
		if !strings.Contains(proto, want) {
			t.Fatalf("backend proto missing %q", want)
		}
	}
	for _, entity := range []string{
		"message Account",
		"message Transaction",
		"message Budget",
		"message Document",
	} {
		if strings.Contains(proto, entity) {
			t.Fatalf("backend proto should not re-model client entity %q", entity)
		}
	}
}

func TestGeneratedProtoMethodNamesMatchBridgeConstants(t *testing.T) {
	for _, tc := range []struct {
		name      string
		manual    string
		generated string
	}{
		{name: "sync list", manual: MethodSyncListWorkspaces, generated: backendrpcpb.SyncService_ListWorkspaces_FullMethodName},
		{name: "sync get", manual: MethodSyncGetWorkspace, generated: backendrpcpb.SyncService_GetWorkspace_FullMethodName},
		{name: "sync put", manual: MethodSyncPutWorkspace, generated: backendrpcpb.SyncService_PutWorkspace_FullMethodName},
		{name: "sync delete", manual: MethodSyncDeleteWorkspace, generated: backendrpcpb.SyncService_DeleteWorkspace_FullMethodName},
		{name: "sync watch", manual: MethodSyncWatchWorkspaces, generated: backendrpcpb.SyncService_WatchWorkspaces_FullMethodName},
		{name: "ai set key", manual: MethodAISetKey, generated: backendrpcpb.AIService_SetKey_FullMethodName},
		{name: "ai list models", manual: MethodAIListModels, generated: backendrpcpb.AIService_ListModels_FullMethodName},
		{name: "ai chat", manual: MethodAIChat, generated: backendrpcpb.AIService_Chat_FullMethodName},
		{name: "ai vision", manual: MethodAIVision, generated: backendrpcpb.AIService_Vision_FullMethodName},
		{name: "ai chat stream", manual: MethodAIChatStream, generated: backendrpcpb.AIService_ChatStream_FullMethodName},
		{name: "ai vision stream", manual: MethodAIVisionStream, generated: backendrpcpb.AIService_VisionStream_FullMethodName},
	} {
		if tc.manual != tc.generated {
			t.Fatalf("%s method = %q, generated %q", tc.name, tc.manual, tc.generated)
		}
	}
}

func TestJSONCodecRejectsUnknownAndTrailingFields(t *testing.T) {
	codec := JSONCodec{}
	var req GetWorkspaceRequest
	if err := codec.Unmarshal([]byte(`{"id":"w1"}`), &req); err != nil {
		t.Fatalf("valid JSON decode: %v", err)
	}
	if req.ID != "w1" {
		t.Fatalf("decoded request = %+v", req)
	}
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{name: "unknown field", raw: `{"id":"w1","extra":true}`},
		{name: "trailing object", raw: `{"id":"w1"} {"id":"w2"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got GetWorkspaceRequest
			if err := codec.Unmarshal([]byte(tc.raw), &got); err == nil {
				t.Fatalf("Unmarshal(%s) succeeded, want error", tc.raw)
			}
		})
	}
}
