// SPDX-License-Identifier: MIT

package loadgen

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/monstercameron/CashFlux/internal/backendrpc"
	"github.com/monstercameron/CashFlux/internal/syncbridge"
	"github.com/monstercameron/GoGRPCBridge/pkg/grpctunnel"
	"google.golang.org/grpc"
)

// Driver executes a Plan against a live CashFlux backend using the real
// client transport (gRPC-over-WS via syncbridge for sync RPCs, plain HTTP for
// blobs), recording every operation into a Recorder.
type Driver struct {
	// ServerURL is the backend base URL (http(s)://host:port).
	ServerURL string
	// Token authenticates every connection (token auth mode).
	Token string
	// WatchEvery opens a WatchWorkspaces stream on every Nth client
	// (0 disables watchers).
	WatchEvery int
	// Logger receives progress and error detail; defaults to slog.Default.
	Logger *slog.Logger
	// HTTPClient serves blob PUT/GET; defaults to a pooled client.
	HTTPClient *http.Client
}

// Run executes the plan and returns the end-of-run report. It respects ctx
// cancellation (clients stop at the next op boundary) and always returns
// whatever was recorded up to that point.
func (d *Driver) Run(ctx context.Context, plan *Plan) (Report, error) {
	if d.ServerURL == "" {
		return Report{}, fmt.Errorf("loadgen driver: server url is required")
	}
	log := d.Logger
	if log == nil {
		log = slog.Default()
	}
	httpc := d.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 30 * time.Second}
	}
	log.Info("loadgen run starting",
		slog.String("scenario", string(plan.Scenario)),
		slog.Int("clients", plan.Clients),
		slog.Duration("duration", plan.Duration),
		slog.Int64("seed", plan.Seed),
		slog.Int("events", plan.TotalEvents()))

	// Preflight: one real RPC before spawning the fleet, so configuration
	// problems fail fast with a useful message instead of N×errors.
	if err := d.preflight(ctx); err != nil {
		return Report{}, err
	}

	start := time.Now()
	rec := NewRecorder(start)
	var wg sync.WaitGroup
	for c := 0; c < plan.Clients; c++ {
		c := c
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.runClient(ctx, log, httpc, rec, plan, c, start)
		}()
	}
	wg.Wait()
	rep := rec.Snapshot(time.Now())
	log.Info("loadgen run finished",
		slog.Int("ops", rep.TotalOps()),
		slog.Int("errors", rep.TotalErrors()),
		slog.Duration("wall", rep.Wall))
	return rep, nil
}

// preflight dials once and issues a ListWorkspaces so misconfiguration is
// reported as one clear error instead of N×errors from the fleet.
func (d *Driver) preflight(ctx context.Context) error {
	conn, err := d.dialConn(ctx)
	if err != nil {
		return fmt.Errorf("loadgen preflight: dial %s: %w", d.ServerURL, err)
	}
	defer func() { _ = conn.Close() }()
	var out backendrpc.ListWorkspacesResponse
	if err := conn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &out, backendrpc.JSONCallOptions()...); err != nil {
		if strings.Contains(err.Error(), "bad handshake") {
			return fmt.Errorf("loadgen preflight: websocket upgrade rejected — the server's Origin check refused %q; make sure the server's CASHFLUX_SERVER_APP_ORIGIN matches the loadgen -server URL (underlying error: %w)", originOf(d.ServerURL), err)
		}
		return fmt.Errorf("loadgen preflight: ListWorkspaces against %s: %w", d.ServerURL, err)
	}
	return nil
}

// dialConn opens a bridge connection presenting the server's own origin in
// the websocket handshake — the same Origin header a same-origin browser
// client sends — so the server's upgrade check passes without special
// load-test configuration.
func (d *Driver) dialConn(ctx context.Context) (*grpc.ClientConn, error) {
	tunnel, err := syncbridge.TunnelConfig(syncbridge.Config{ServerURL: d.ServerURL, Token: d.Token})
	if err != nil {
		return nil, err
	}
	tunnel.Headers = http.Header{"Origin": []string{originOf(d.ServerURL)}}
	return grpctunnel.BuildTunnelConn(ctx, tunnel)
}

// originOf reduces a server URL to its scheme://host[:port] origin.
func originOf(serverURL string) string {
	u, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return serverURL
	}
	return u.Scheme + "://" + u.Host
}

// runClient plays one virtual client's schedule to completion or cancellation.
func (d *Driver) runClient(ctx context.Context, log *slog.Logger, httpc *http.Client, rec *Recorder, plan *Plan, idx int, start time.Time) {
	rng := rand.New(rand.NewSource(plan.Seed ^ int64(idx)*104729))
	deviceID := fmt.Sprintf("loadgen-%03d", idx)
	wsID := plan.WorkspaceIDs[idx]

	conn := d.dial(ctx, rec)
	if conn == nil {
		return
	}
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// A subset of clients also holds a watch stream open, mimicking a second
	// device waiting for changes; events are drained and counted.
	if d.WatchEvery > 0 && idx%d.WatchEvery == 0 {
		d.startWatcher(ctx, log, rec, conn)
	}

	// Bootstrap: a real client always has its workspace before it attaches
	// blobs or pulls, so create it up front (recorded as an ordinary push).
	version := d.doPush(ctx, rec, conn, wsID, deviceID, 0, plan.Profile.DatasetBytes, idx)
	for _, ev := range plan.Schedules[idx] {
		// Sleep until the event's offset (relative to shared run start).
		wait := time.Until(start.Add(ev.At))
		if wait > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
		if ctx.Err() != nil {
			return
		}
		switch ev.Op {
		case OpPush:
			version = d.doPush(ctx, rec, conn, wsID, deviceID, version, plan.Profile.DatasetBytes, idx)
		case OpPull:
			d.doPull(ctx, rec, conn, wsID)
		case OpList:
			d.doList(ctx, rec, conn)
		case OpBlobPut:
			d.doBlobPut(ctx, rec, httpc, wsID, plan.Profile.BlobBytes, rng.Int63())
		case OpBlobGet:
			d.doBlobGet(ctx, rec, httpc, wsID, plan.Profile.BlobBytes, rng.Int63())
		case OpReconnect:
			_ = conn.Close()
			// Jittered backoff before re-dialing, mirroring a well-behaved
			// client after a server restart.
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(rng.Int63n(int64(500 * time.Millisecond)))):
			}
			conn = d.dial(ctx, rec)
			if conn == nil {
				return
			}
		}
	}
}

// dial opens a bridge connection, recording it as a reconnect-op sample.
func (d *Driver) dial(ctx context.Context, rec *Recorder) *grpc.ClientConn {
	t0 := time.Now()
	conn, err := d.dialConn(ctx)
	rec.Record(OpReconnect, time.Since(t0), err)
	if err != nil {
		rec.Count("dial_failed", 1)
		return nil
	}
	return conn
}

// startWatcher opens a WatchWorkspaces stream and drains events until the
// context ends, counting each one.
func (d *Driver) startWatcher(ctx context.Context, log *slog.Logger, rec *Recorder, conn *grpc.ClientConn) {
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, backendrpc.MethodSyncWatchWorkspaces, backendrpc.JSONCallOptions()...)
	if err != nil {
		rec.Count("watch_open_failed", 1)
		return
	}
	if err := stream.SendMsg(&backendrpc.WatchWorkspacesRequest{IncludeDeleted: true}); err != nil {
		rec.Count("watch_open_failed", 1)
		return
	}
	if err := stream.CloseSend(); err != nil {
		rec.Count("watch_open_failed", 1)
		return
	}
	rec.Count("watchers", 1)
	go func() {
		for {
			var event backendrpc.WatchWorkspacesResponse
			if err := stream.RecvMsg(&event); err != nil {
				if ctx.Err() == nil && !strings.Contains(err.Error(), "EOF") {
					log.Debug("watch stream ended", slog.String("err", err.Error()))
				}
				return
			}
			rec.Count("watch_events", 1)
		}
	}()
}

// doPush sends a PutWorkspace with a padded dataset and records acceptance.
func (d *Driver) doPush(ctx context.Context, rec *Recorder, conn *grpc.ClientConn, wsID, deviceID string, version int64, datasetBytes, idx int) int64 {
	dataset := paddedDataset(datasetBytes, idx)
	req := backendrpc.PutWorkspaceRequest{
		Workspace: backendrpc.Workspace{
			ID:       wsID,
			Name:     "Loadgen " + wsID,
			Version:  version,
			DeviceID: deviceID,
		},
		Dataset:         dataset,
		ClientUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	var out backendrpc.PutWorkspaceResponse
	t0 := time.Now()
	err := conn.Invoke(ctx, backendrpc.MethodSyncPutWorkspace, req, &out, backendrpc.JSONCallOptions()...)
	rec.Record(OpPush, time.Since(t0), err)
	if err != nil {
		return version
	}
	rec.AddBytes(int64(len(dataset)), 0)
	if out.Accepted {
		rec.Count("push_accepted", 1)
		return out.Workspace.Version
	}
	// A rejected push is the LWW guard doing its job (conflict scenario),
	// not an error - count it separately and adopt the server's version.
	rec.Count("push_rejected", 1)
	return out.Workspace.Version
}

// doPull issues a GetWorkspace and tracks downloaded bytes.
func (d *Driver) doPull(ctx context.Context, rec *Recorder, conn *grpc.ClientConn, wsID string) {
	var out backendrpc.GetWorkspaceResponse
	t0 := time.Now()
	err := conn.Invoke(ctx, backendrpc.MethodSyncGetWorkspace, backendrpc.GetWorkspaceRequest{ID: wsID}, &out, backendrpc.JSONCallOptions()...)
	rec.Record(OpPull, time.Since(t0), err)
	if err == nil {
		rec.AddBytes(0, int64(len(out.Dataset)))
	}
}

// doList issues a ListWorkspaces call.
func (d *Driver) doList(ctx context.Context, rec *Recorder, conn *grpc.ClientConn) {
	var out backendrpc.ListWorkspacesResponse
	t0 := time.Now()
	err := conn.Invoke(ctx, backendrpc.MethodSyncListWorkspaces, backendrpc.ListWorkspacesRequest{}, &out, backendrpc.JSONCallOptions()...)
	rec.Record(OpList, time.Since(t0), err)
}

// doBlobPut uploads a deterministic pseudo-blob via the HTTP blob endpoint.
func (d *Driver) doBlobPut(ctx context.Context, rec *Recorder, httpc *http.Client, wsID string, size int, nonce int64) {
	data := blobPayload(size, nonce)
	hash := sha256.Sum256(data)
	url := fmt.Sprintf("%s/v1/blobs/%s?workspaceId=%s", d.ServerURL, hex.EncodeToString(hash[:]), wsID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		rec.Record(OpBlobPut, 0, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	t0 := time.Now()
	resp, err := httpc.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("blob put status %d", resp.StatusCode)
		}
	}
	rec.Record(OpBlobPut, time.Since(t0), err)
	if err == nil {
		rec.AddBytes(int64(len(data)), 0)
	}
}

// doBlobGet downloads the same deterministic pseudo-blob (uploading first if
// the server has not seen it, so gets never depend on op ordering).
func (d *Driver) doBlobGet(ctx context.Context, rec *Recorder, httpc *http.Client, wsID string, size int, nonce int64) {
	data := blobPayload(size, nonce)
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])
	url := fmt.Sprintf("%s/v1/blobs/%s?workspaceId=%s", d.ServerURL, hashHex, wsID)

	get := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+d.Token)
		return httpc.Do(req)
	}
	t0 := time.Now()
	resp, err := get()
	if err == nil && resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		d.doBlobPut(ctx, rec, httpc, wsID, size, nonce)
		resp, err = get()
	}
	if err == nil {
		n, _ := io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("blob get status %d", resp.StatusCode)
		} else {
			rec.AddBytes(0, n)
		}
	}
	rec.Record(OpBlobGet, time.Since(t0), err)
}

// paddedDataset builds a syntactically-valid dataset payload of roughly the
// target encoded size.
func paddedDataset(target, idx int) []byte {
	head := fmt.Sprintf(`{"schemaVersion":1,"loadgen":%d,"pad":"`, idx)
	tail := `"}`
	pad := target - len(head) - len(tail)
	if pad < 0 {
		pad = 0
	}
	return []byte(head + strings.Repeat("x", pad) + tail)
}

// blobPayload builds a deterministic pseudo-random blob for (size, nonce), so
// the same nonce always yields the same content hash.
func blobPayload(size int, nonce int64) []byte {
	if size <= 0 {
		size = 1
	}
	data := make([]byte, size)
	rng := rand.New(rand.NewSource(nonce))
	rng.Read(data)
	return data
}
