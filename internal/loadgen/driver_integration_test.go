// SPDX-License-Identifier: MIT

package loadgen_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/loadgen"
	"github.com/monstercameron/CashFlux/internal/server"
)

// startTestBackend boots the real server mux (token auth, temp SQLite store,
// blob storage in a temp dir) exactly as production wires it.
func startTestBackend(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := server.OpenStore(filepath.Join(t.TempDir(), "loadgen.db"))
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := server.Config{
		AuthMode:     "token",
		Token:        "loadgen-token",
		AppOrigin:    "*",
		DataDir:      t.TempDir(),
		BlobMaxBytes: 1 << 20,
		Metrics:      server.NewMetrics(),
	}
	srv := httptest.NewServer(server.NewMux(cfg, store))
	t.Cleanup(srv.Close)
	return srv
}

// fastProfile compresses a meaningful op mix into a ~2-second run.
func fastProfile() loadgen.Profile {
	return loadgen.Profile{
		PushPerMin:    300,
		PullPerMin:    300,
		ListPerMin:    60,
		BlobPutPerMin: 60,
		BlobGetPerMin: 60,
		DatasetBytes:  4 << 10,
		BlobBytes:     8 << 10,
	}
}

func TestDriverMixedAgainstRealServer(t *testing.T) {
	srv := startTestBackend(t)

	plan, err := loadgen.BuildPlan(loadgen.ScenarioSteady, 4, 2*time.Second, 11, fastProfile())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	d := &loadgen.Driver{ServerURL: srv.URL, Token: "loadgen-token", WatchEvery: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rep, err := d.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.TotalOps() == 0 {
		t.Fatal("no operations recorded")
	}
	if rep.TotalErrors() != 0 {
		t.Fatalf("errors against in-process server: %d\n%s", rep.TotalErrors(), rep.String())
	}
	if rep.Counters["push_accepted"] == 0 {
		t.Fatalf("no pushes accepted:\n%s", rep.String())
	}
	if rep.Counters["watchers"] == 0 {
		t.Fatalf("no watchers opened:\n%s", rep.String())
	}
	if rep.BytesUp == 0 || rep.BytesDown == 0 {
		t.Fatalf("byte accounting empty: up=%d down=%d", rep.BytesUp, rep.BytesDown)
	}
}

func TestDriverConflictScenarioCountsRejections(t *testing.T) {
	srv := startTestBackend(t)

	profile := fastProfile()
	profile.BlobPutPerMin, profile.BlobGetPerMin = 0, 0
	plan, err := loadgen.BuildPlan(loadgen.ScenarioConflict, 4, 1500*time.Millisecond, 23, profile)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	d := &loadgen.Driver{ServerURL: srv.URL, Token: "loadgen-token"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rep, err := d.Run(ctx, plan)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var pushes int
	for _, op := range rep.Ops {
		if op.Op == loadgen.OpPush {
			pushes = op.Count
		}
	}
	if pushes == 0 {
		t.Fatal("conflict run performed no pushes")
	}
	// Every non-errored push is either accepted or rejected by the LWW
	// guard — the two counters must fully account for them.
	total := rep.Counters["push_accepted"] + rep.Counters["push_rejected"]
	if total != int64(pushes-func() int {
		for _, op := range rep.Ops {
			if op.Op == loadgen.OpPush {
				return op.Errors
			}
		}
		return 0
	}()) {
		t.Fatalf("accepted(%d)+rejected(%d) != pushes(%d)-errors:\n%s",
			rep.Counters["push_accepted"], rep.Counters["push_rejected"], pushes, rep.String())
	}
}
