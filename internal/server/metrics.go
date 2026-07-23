// SPDX-License-Identifier: MIT

package server

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics stores the backend's in-process Prometheus counters.
//
// Locking is sharded per metric family: every RPC touches several families
// (the gRPC interceptor, the handler's sync counters, and one DB observation
// per store call), so a single shared mutex serializes the entire request
// path against itself. Scalar counters use atomics and take no lock at all;
// map-backed families each own a small mutex. The scrape path locks each
// family in turn — Prometheus does not need a cross-family-atomic snapshot.
type Metrics struct {
	httpMu      sync.Mutex
	http        map[metricKey]metricValue
	httpBuckets map[metricKey][]int64

	grpcMu          sync.Mutex
	grpc            map[metricKey]metricValue
	streamDurations map[metricKey]metricValue

	streamsActive atomic.Int64

	blobStoredBytes      atomic.Int64
	blobTransferredBytes atomic.Int64
	blobGCSweeps         atomic.Int64
	blobGCDeleted        atomic.Int64

	aiProxyRequests atomic.Int64
	aiProxyTokens   atomic.Int64

	syncMu         sync.Mutex
	syncPulls      map[string]int64
	syncPushes     map[string]int64
	syncLWWRejects atomic.Int64
	watchDropped   atomic.Int64

	dbMu sync.Mutex
	db   map[string]metricValue

	queueMu     sync.Mutex
	queueDepths map[string]int64

	billingMu       sync.Mutex
	billingEvents   map[billingMetricKey]int64
	billingMRRCents int64
}

type metricKey struct {
	Name   string
	Status string
}

type metricValue struct {
	Count        int64
	DurationSecs float64
}

type billingMetricKey struct {
	Event  string
	Plan   string
	Status string
}

var httpDurationBuckets = []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func NewMetrics() *Metrics {
	return &Metrics{
		http:            map[metricKey]metricValue{},
		httpBuckets:     map[metricKey][]int64{},
		grpc:            map[metricKey]metricValue{},
		streamDurations: map[metricKey]metricValue{},
		syncPulls:       map[string]int64{},
		syncPushes:      map[string]int64{},
		db:              map[string]metricValue{},
		queueDepths:     map[string]int64{},
		billingEvents:   map[billingMetricKey]int64{},
	}
}

func (m *Metrics) ObserveHTTP(route string, status int, elapsed time.Duration) {
	if m == nil {
		return
	}
	key := metricKey{Name: route, Status: fmt.Sprintf("%d", status)}
	seconds := elapsed.Seconds()
	m.httpMu.Lock()
	defer m.httpMu.Unlock()
	observeLocked(m.http, key, seconds)
	if m.httpBuckets == nil {
		m.httpBuckets = map[metricKey][]int64{}
	}
	counts := m.httpBuckets[key]
	if len(counts) != len(httpDurationBuckets) {
		counts = make([]int64, len(httpDurationBuckets))
	}
	for i, bucket := range httpDurationBuckets {
		if seconds <= bucket {
			counts[i]++
		}
	}
	m.httpBuckets[key] = counts
}

func (m *Metrics) ObserveGRPC(method, status string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.grpcMu.Lock()
	defer m.grpcMu.Unlock()
	observeLocked(m.grpc, metricKey{Name: method, Status: status}, elapsed.Seconds())
}

func (m *Metrics) IncActiveStream() {
	if m == nil {
		return
	}
	m.streamsActive.Add(1)
}

func (m *Metrics) DecActiveStream() {
	if m == nil {
		return
	}
	if v := m.streamsActive.Add(-1); v < 0 {
		m.streamsActive.Store(0)
	}
}

func (m *Metrics) ObserveStreamDuration(name, status string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.grpcMu.Lock()
	defer m.grpcMu.Unlock()
	observeLocked(m.streamDurations, metricKey{Name: name, Status: status}, elapsed.Seconds())
}

func (m *Metrics) ObserveBlobStored(bytes int64) {
	if m == nil || bytes <= 0 {
		return
	}
	m.blobStoredBytes.Add(bytes)
}

func (m *Metrics) ObserveBlobTransferred(bytes int64) {
	if m == nil || bytes <= 0 {
		return
	}
	m.blobTransferredBytes.Add(bytes)
}

func (m *Metrics) ObserveBlobGC(deleted int) {
	if m == nil {
		return
	}
	m.blobGCSweeps.Add(1)
	if deleted > 0 {
		m.blobGCDeleted.Add(int64(deleted))
	}
}

func (m *Metrics) ObserveAIProxy(tokens int64) {
	if m == nil {
		return
	}
	m.aiProxyRequests.Add(1)
	if tokens > 0 {
		m.aiProxyTokens.Add(tokens)
	}
}

func (m *Metrics) ObserveSyncPull(result string) {
	if m == nil {
		return
	}
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	if m.syncPulls == nil {
		m.syncPulls = map[string]int64{}
	}
	m.syncPulls[result]++
}

func (m *Metrics) ObserveSyncPush(result string) {
	if m == nil {
		return
	}
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	if m.syncPushes == nil {
		m.syncPushes = map[string]int64{}
	}
	m.syncPushes[result]++
}

func (m *Metrics) ObserveSyncLWWReject() {
	if m == nil {
		return
	}
	m.syncLWWRejects.Add(1)
}

// ObserveWatchDropped counts a workspace-watch event discarded because the
// subscriber's buffer was full — silent staleness made visible.
func (m *Metrics) ObserveWatchDropped() {
	if m == nil {
		return
	}
	m.watchDropped.Add(1)
}

func (m *Metrics) ObserveDB(operation string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	if m.db == nil {
		m.db = map[string]metricValue{}
	}
	observeLocked(m.db, operation, elapsed.Seconds())
}

func (m *Metrics) SetQueueDepth(name string, depth int64) {
	if m == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	m.queueMu.Lock()
	defer m.queueMu.Unlock()
	if m.queueDepths == nil {
		m.queueDepths = map[string]int64{}
	}
	m.queueDepths[name] = depth
}

func (m *Metrics) ObserveBillingEvent(event, plan, status string) {
	if m == nil {
		return
	}
	m.billingMu.Lock()
	defer m.billingMu.Unlock()
	if m.billingEvents == nil {
		m.billingEvents = map[billingMetricKey]int64{}
	}
	m.billingEvents[billingMetricKey{Event: normalizedMetricLabel(event, "unknown"), Plan: normalizedMetricLabel(plan, "unknown"), Status: normalizedMetricLabel(status, "unknown")}]++
}

func (m *Metrics) ObserveBillingMRRDelta(cents int64) {
	if m == nil || cents == 0 {
		return
	}
	m.billingMu.Lock()
	defer m.billingMu.Unlock()
	m.billingMRRCents += cents
	if m.billingMRRCents < 0 {
		m.billingMRRCents = 0
	}
}

// observeLocked accumulates one duration sample into a keyed family. Callers
// hold that family's mutex.
func observeLocked[K comparable](dst map[K]metricValue, key K, seconds float64) {
	v := dst[key]
	v.Count++
	v.DurationSecs += seconds
	dst[key] = v
}

func (m *Metrics) WritePrometheus(w io.Writer) {
	if m == nil {
		m = NewMetrics()
	}
	snap := m.snapshot()
	_, _ = io.WriteString(w, "# HELP cashflux_server_up Server process health.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_server_up gauge\n")
	_, _ = io.WriteString(w, "cashflux_server_up 1\n")
	_, _ = io.WriteString(w, "# HELP cashflux_http_requests_total HTTP requests by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_requests_total counter\n")
	for _, row := range snap.httpRows {
		_, _ = fmt.Fprintf(w, "cashflux_http_requests_total{route=%q,status=%q} %d\n", row.Key.Name, row.Key.Status, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_http_request_duration_seconds_sum HTTP request duration sum by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_request_duration_seconds_sum counter\n")
	for _, row := range snap.httpRows {
		_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_sum{route=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_http_request_duration_seconds_bucket HTTP request duration histogram buckets by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_request_duration_seconds_bucket histogram\n")
	for _, row := range snap.httpBucketRows {
		for i, bucket := range httpDurationBuckets {
			_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_bucket{route=%q,status=%q,le=%q} %d\n", row.Key.Name, row.Key.Status, fmtFloat(bucket), row.Buckets[i])
		}
		_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_bucket{route=%q,status=%q,le=%q} %d\n", row.Key.Name, row.Key.Status, "+Inf", row.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_requests_total gRPC requests by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_requests_total counter\n")
	for _, row := range snap.grpcRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_requests_total{method=%q,status=%q} %d\n", row.Key.Name, row.Key.Status, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_request_duration_seconds_sum gRPC request duration sum by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_request_duration_seconds_sum counter\n")
	for _, row := range snap.grpcRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_request_duration_seconds_sum{method=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_streams_active Active gRPC server streams.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_streams_active gauge\n")
	_, _ = fmt.Fprintf(w, "cashflux_grpc_streams_active %d\n", snap.activeStreams)
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_stream_duration_seconds_sum gRPC stream duration sum by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_stream_duration_seconds_sum counter\n")
	for _, row := range snap.streamRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_stream_duration_seconds_sum{method=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_blob_stored_bytes_total Blob bytes stored by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_stored_bytes_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_stored_bytes_total %d\n", snap.blobStoredBytes)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_transferred_bytes_total Blob bytes served by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_transferred_bytes_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_transferred_bytes_total %d\n", snap.blobTransferredBytes)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_gc_sweeps_total Blob garbage-collection sweeps run.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_gc_sweeps_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_gc_sweeps_total %d\n", snap.blobGCSweeps)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_gc_deleted_total Unreferenced blobs deleted by garbage collection.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_gc_deleted_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_gc_deleted_total %d\n", snap.blobGCDeleted)
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_requests_total AI proxy completions served by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_requests_total %d\n", snap.aiProxyRequests)
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_tokens_total AI proxy tokens reported by upstream responses.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_tokens_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_tokens_total %d\n", snap.aiProxyTokens)
	_, _ = io.WriteString(w, "# HELP cashflux_sync_pulls_total Sync pull responses by result.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_pulls_total counter\n")
	for _, row := range snap.syncPulls {
		_, _ = fmt.Fprintf(w, "cashflux_sync_pulls_total{result=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_sync_pushes_total Sync push responses by result.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_pushes_total counter\n")
	for _, row := range snap.syncPushes {
		_, _ = fmt.Fprintf(w, "cashflux_sync_pushes_total{result=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_sync_lww_rejects_total Sync last-write-wins rejects.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_lww_rejects_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_sync_lww_rejects_total %d\n", snap.syncLWWRejects)
	_, _ = io.WriteString(w, "# HELP cashflux_sync_watch_dropped_total Workspace watch events dropped because a subscriber buffer was full.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_watch_dropped_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_sync_watch_dropped_total %d\n", snap.watchDropped)
	_, _ = io.WriteString(w, "# HELP cashflux_db_queries_total Store operations by name.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_db_queries_total counter\n")
	for _, row := range snap.dbRows {
		_, _ = fmt.Fprintf(w, "cashflux_db_queries_total{operation=%q} %d\n", row.Name, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_db_query_duration_seconds_sum Store operation duration sum by name.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_db_query_duration_seconds_sum counter\n")
	for _, row := range snap.dbRows {
		_, _ = fmt.Fprintf(w, "cashflux_db_query_duration_seconds_sum{operation=%q} %.6f\n", row.Name, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_queue_depth Buffered backend queue depth by queue.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_queue_depth gauge\n")
	for _, row := range snap.queueDepths {
		_, _ = fmt.Fprintf(w, "cashflux_queue_depth{queue=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_billing_events_total Privacy-safe billing webhook business events by type, plan, and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_billing_events_total counter\n")
	for _, row := range snap.billingRows {
		_, _ = fmt.Fprintf(w, "cashflux_billing_events_total{event=%q,plan=%q,status=%q} %d\n", row.Key.Event, row.Key.Plan, row.Key.Status, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_billing_mrr_cents Estimated active monthly recurring revenue in cents from billing webhooks.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_billing_mrr_cents gauge\n")
	_, _ = fmt.Fprintf(w, "cashflux_billing_mrr_cents %d\n", snap.billingMRRCents)
}

type metricRow struct {
	Key   metricKey
	Value metricValue
}

type labelMetricRow struct {
	Name  string
	Value int64
}

type namedMetricRow struct {
	Name  string
	Value metricValue
}

type bucketMetricRow struct {
	Key     metricKey
	Buckets []int64
	Count   int64
}

type billingMetricRow struct {
	Key   billingMetricKey
	Value int64
}

// metricsSnapshot is a scrape-time copy of every family, taken one family
// lock at a time (Prometheus does not need cross-family atomicity).
type metricsSnapshot struct {
	httpRows             []metricRow
	httpBucketRows       []bucketMetricRow
	grpcRows             []metricRow
	activeStreams        int64
	streamRows           []metricRow
	blobStoredBytes      int64
	blobTransferredBytes int64
	blobGCSweeps         int64
	blobGCDeleted        int64
	aiProxyRequests      int64
	aiProxyTokens        int64
	syncPulls            []labelMetricRow
	syncPushes           []labelMetricRow
	syncLWWRejects       int64
	watchDropped         int64
	dbRows               []namedMetricRow
	queueDepths          []labelMetricRow
	billingRows          []billingMetricRow
	billingMRRCents      int64
}

func (m *Metrics) snapshot() metricsSnapshot {
	var snap metricsSnapshot

	m.httpMu.Lock()
	snap.httpRows = metricRows(m.http)
	snap.httpBucketRows = bucketMetricRows(m.httpBuckets, m.http)
	m.httpMu.Unlock()

	m.grpcMu.Lock()
	snap.grpcRows = metricRows(m.grpc)
	snap.streamRows = metricRows(m.streamDurations)
	m.grpcMu.Unlock()

	snap.activeStreams = m.streamsActive.Load()
	snap.blobStoredBytes = m.blobStoredBytes.Load()
	snap.blobTransferredBytes = m.blobTransferredBytes.Load()
	snap.blobGCSweeps = m.blobGCSweeps.Load()
	snap.blobGCDeleted = m.blobGCDeleted.Load()
	snap.aiProxyRequests = m.aiProxyRequests.Load()
	snap.aiProxyTokens = m.aiProxyTokens.Load()
	snap.syncLWWRejects = m.syncLWWRejects.Load()
	snap.watchDropped = m.watchDropped.Load()

	m.syncMu.Lock()
	snap.syncPulls = labelMetricRows(m.syncPulls)
	snap.syncPushes = labelMetricRows(m.syncPushes)
	m.syncMu.Unlock()

	m.dbMu.Lock()
	snap.dbRows = namedMetricRows(m.db)
	m.dbMu.Unlock()

	m.queueMu.Lock()
	snap.queueDepths = labelMetricRows(m.queueDepths)
	m.queueMu.Unlock()

	m.billingMu.Lock()
	snap.billingRows = billingMetricRows(m.billingEvents)
	snap.billingMRRCents = m.billingMRRCents
	m.billingMu.Unlock()

	return snap
}

func metricRows(src map[metricKey]metricValue) []metricRow {
	rows := make([]metricRow, 0, len(src))
	for key, value := range src {
		rows = append(rows, metricRow{Key: key, Value: value})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Key.Name + "\x00" + rows[i].Key.Status
		right := rows[j].Key.Name + "\x00" + rows[j].Key.Status
		return strings.Compare(left, right) < 0
	})
	return rows
}

func labelMetricRows(src map[string]int64) []labelMetricRow {
	rows := make([]labelMetricRow, 0, len(src))
	for name, value := range src {
		rows = append(rows, labelMetricRow{Name: name, Value: value})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Compare(rows[i].Name, rows[j].Name) < 0
	})
	return rows
}

func billingMetricRows(src map[billingMetricKey]int64) []billingMetricRow {
	rows := make([]billingMetricRow, 0, len(src))
	for key, value := range src {
		rows = append(rows, billingMetricRow{Key: key, Value: value})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Key.Event + "\x00" + rows[i].Key.Plan + "\x00" + rows[i].Key.Status
		right := rows[j].Key.Event + "\x00" + rows[j].Key.Plan + "\x00" + rows[j].Key.Status
		return strings.Compare(left, right) < 0
	})
	return rows
}

func normalizedMetricLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func namedMetricRows(src map[string]metricValue) []namedMetricRow {
	rows := make([]namedMetricRow, 0, len(src))
	for name, value := range src {
		rows = append(rows, namedMetricRow{Name: name, Value: value})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.Compare(rows[i].Name, rows[j].Name) < 0
	})
	return rows
}

func bucketMetricRows(src map[metricKey][]int64, counts map[metricKey]metricValue) []bucketMetricRow {
	rows := make([]bucketMetricRow, 0, len(src))
	for key, buckets := range src {
		copied := append([]int64(nil), buckets...)
		rows = append(rows, bucketMetricRow{Key: key, Buckets: copied, Count: counts[key].Count})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i].Key.Name + "\x00" + rows[i].Key.Status
		right := rows[j].Key.Name + "\x00" + rows[j].Key.Status
		return strings.Compare(left, right) < 0
	})
	return rows
}

func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}
