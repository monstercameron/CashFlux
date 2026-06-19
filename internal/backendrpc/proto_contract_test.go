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
