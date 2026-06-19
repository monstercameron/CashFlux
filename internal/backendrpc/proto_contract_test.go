package backendrpc

import (
	"os"
	"strings"
	"testing"
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
