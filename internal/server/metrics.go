package server

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metrics stores the backend's in-process Prometheus counters.
type Metrics struct {
	mu                   sync.Mutex
	http                 map[metricKey]metricValue
	httpBuckets          map[metricKey][]int64
	grpc                 map[metricKey]metricValue
	streamsActive        int64
	streamDurations      map[metricKey]metricValue
	blobStoredBytes      int64
	blobTransferredBytes int64
	blobGCSweeps         int64
	blobGCDeleted        int64
	aiProxyRequests      int64
	aiProxyTokens        int64
	syncPulls            map[string]int64
	syncPushes           map[string]int64
	syncLWWRejects       int64
	db                   map[string]metricValue
	queueDepths          map[string]int64
	billingEvents        map[billingMetricKey]int64
	billingMRRCents      int64
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
	m.observe(m.http, key, elapsed)
	m.observeHTTPBucket(key, elapsed)
}

func (m *Metrics) ObserveGRPC(method, status string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.observe(m.grpc, metricKey{Name: method, Status: status}, elapsed)
}

func (m *Metrics) IncActiveStream() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streamsActive++
}

func (m *Metrics) DecActiveStream() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.streamsActive > 0 {
		m.streamsActive--
	}
}

func (m *Metrics) ObserveStreamDuration(name, status string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.observe(m.streamDurations, metricKey{Name: name, Status: status}, elapsed)
}

func (m *Metrics) ObserveBlobStored(bytes int64) {
	if m == nil || bytes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobStoredBytes += bytes
}

func (m *Metrics) ObserveBlobTransferred(bytes int64) {
	if m == nil || bytes <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobTransferredBytes += bytes
}

func (m *Metrics) ObserveBlobGC(deleted int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobGCSweeps++
	if deleted > 0 {
		m.blobGCDeleted += int64(deleted)
	}
}

func (m *Metrics) ObserveAIProxy(tokens int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.aiProxyRequests++
	if tokens > 0 {
		m.aiProxyTokens += tokens
	}
}

func (m *Metrics) ObserveSyncPull(result string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.syncPulls == nil {
		m.syncPulls = map[string]int64{}
	}
	m.syncPulls[result]++
}

func (m *Metrics) ObserveSyncPush(result string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.syncPushes == nil {
		m.syncPushes = map[string]int64{}
	}
	m.syncPushes[result]++
}

func (m *Metrics) ObserveSyncLWWReject() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncLWWRejects++
}

func (m *Metrics) ObserveDB(operation string, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		m.db = map[string]metricValue{}
	}
	v := m.db[operation]
	v.Count++
	v.DurationSecs += elapsed.Seconds()
	m.db[operation] = v
}

func (m *Metrics) SetQueueDepth(name string, depth int64) {
	if m == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.queueDepths == nil {
		m.queueDepths = map[string]int64{}
	}
	m.queueDepths[name] = depth
}

func (m *Metrics) ObserveBillingEvent(event, plan, status string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.billingEvents == nil {
		m.billingEvents = map[billingMetricKey]int64{}
	}
	m.billingEvents[billingMetricKey{Event: normalizedMetricLabel(event, "unknown"), Plan: normalizedMetricLabel(plan, "unknown"), Status: normalizedMetricLabel(status, "unknown")}]++
}

func (m *Metrics) ObserveBillingMRRDelta(cents int64) {
	if m == nil || cents == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.billingMRRCents += cents
	if m.billingMRRCents < 0 {
		m.billingMRRCents = 0
	}
}

func (m *Metrics) observe(dst map[metricKey]metricValue, key metricKey, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := dst[key]
	v.Count++
	v.DurationSecs += elapsed.Seconds()
	dst[key] = v
}

func (m *Metrics) observeHTTPBucket(key metricKey, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.httpBuckets == nil {
		m.httpBuckets = map[metricKey][]int64{}
	}
	counts := m.httpBuckets[key]
	if len(counts) != len(httpDurationBuckets) {
		counts = make([]int64, len(httpDurationBuckets))
	}
	seconds := elapsed.Seconds()
	for i, bucket := range httpDurationBuckets {
		if seconds <= bucket {
			counts[i]++
		}
	}
	m.httpBuckets[key] = counts
}

func (m *Metrics) WritePrometheus(w io.Writer) {
	if m == nil {
		m = NewMetrics()
	}
	httpRows, httpBucketRows, grpcRows, activeStreams, streamRows, blobStoredBytes, blobTransferredBytes, blobGCSweeps, blobGCDeleted, aiProxyRequests, aiProxyTokens, syncPulls, syncPushes, syncLWWRejects, dbRows, queueDepths, billingRows, billingMRRCents := m.snapshot()
	_, _ = io.WriteString(w, "# HELP cashflux_server_up Server process health.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_server_up gauge\n")
	_, _ = io.WriteString(w, "cashflux_server_up 1\n")
	_, _ = io.WriteString(w, "# HELP cashflux_http_requests_total HTTP requests by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_requests_total counter\n")
	for _, row := range httpRows {
		_, _ = fmt.Fprintf(w, "cashflux_http_requests_total{route=%q,status=%q} %d\n", row.Key.Name, row.Key.Status, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_http_request_duration_seconds_sum HTTP request duration sum by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_request_duration_seconds_sum counter\n")
	for _, row := range httpRows {
		_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_sum{route=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_http_request_duration_seconds_bucket HTTP request duration histogram buckets by route and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_http_request_duration_seconds_bucket histogram\n")
	for _, row := range httpBucketRows {
		for i, bucket := range httpDurationBuckets {
			_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_bucket{route=%q,status=%q,le=%q} %d\n", row.Key.Name, row.Key.Status, fmtFloat(bucket), row.Buckets[i])
		}
		_, _ = fmt.Fprintf(w, "cashflux_http_request_duration_seconds_bucket{route=%q,status=%q,le=%q} %d\n", row.Key.Name, row.Key.Status, "+Inf", row.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_requests_total gRPC requests by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_requests_total counter\n")
	for _, row := range grpcRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_requests_total{method=%q,status=%q} %d\n", row.Key.Name, row.Key.Status, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_request_duration_seconds_sum gRPC request duration sum by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_request_duration_seconds_sum counter\n")
	for _, row := range grpcRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_request_duration_seconds_sum{method=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_streams_active Active gRPC server streams.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_streams_active gauge\n")
	_, _ = fmt.Fprintf(w, "cashflux_grpc_streams_active %d\n", activeStreams)
	_, _ = io.WriteString(w, "# HELP cashflux_grpc_stream_duration_seconds_sum gRPC stream duration sum by method and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_grpc_stream_duration_seconds_sum counter\n")
	for _, row := range streamRows {
		_, _ = fmt.Fprintf(w, "cashflux_grpc_stream_duration_seconds_sum{method=%q,status=%q} %.6f\n", row.Key.Name, row.Key.Status, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_blob_stored_bytes_total Blob bytes stored by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_stored_bytes_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_stored_bytes_total %d\n", blobStoredBytes)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_transferred_bytes_total Blob bytes served by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_transferred_bytes_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_transferred_bytes_total %d\n", blobTransferredBytes)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_gc_sweeps_total Blob garbage-collection sweeps run.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_gc_sweeps_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_gc_sweeps_total %d\n", blobGCSweeps)
	_, _ = io.WriteString(w, "# HELP cashflux_blob_gc_deleted_total Unreferenced blobs deleted by garbage collection.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_blob_gc_deleted_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_blob_gc_deleted_total %d\n", blobGCDeleted)
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_requests_total AI proxy completions served by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_requests_total %d\n", aiProxyRequests)
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_tokens_total AI proxy tokens reported by upstream responses.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_tokens_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_tokens_total %d\n", aiProxyTokens)
	_, _ = io.WriteString(w, "# HELP cashflux_sync_pulls_total Sync pull responses by result.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_pulls_total counter\n")
	for _, row := range syncPulls {
		_, _ = fmt.Fprintf(w, "cashflux_sync_pulls_total{result=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_sync_pushes_total Sync push responses by result.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_pushes_total counter\n")
	for _, row := range syncPushes {
		_, _ = fmt.Fprintf(w, "cashflux_sync_pushes_total{result=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_sync_lww_rejects_total Sync last-write-wins rejects.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_sync_lww_rejects_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_sync_lww_rejects_total %d\n", syncLWWRejects)
	_, _ = io.WriteString(w, "# HELP cashflux_db_queries_total Store operations by name.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_db_queries_total counter\n")
	for _, row := range dbRows {
		_, _ = fmt.Fprintf(w, "cashflux_db_queries_total{operation=%q} %d\n", row.Name, row.Value.Count)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_db_query_duration_seconds_sum Store operation duration sum by name.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_db_query_duration_seconds_sum counter\n")
	for _, row := range dbRows {
		_, _ = fmt.Fprintf(w, "cashflux_db_query_duration_seconds_sum{operation=%q} %.6f\n", row.Name, row.Value.DurationSecs)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_queue_depth Buffered backend queue depth by queue.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_queue_depth gauge\n")
	for _, row := range queueDepths {
		_, _ = fmt.Fprintf(w, "cashflux_queue_depth{queue=%q} %d\n", row.Name, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_billing_events_total Privacy-safe billing webhook business events by type, plan, and status.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_billing_events_total counter\n")
	for _, row := range billingRows {
		_, _ = fmt.Fprintf(w, "cashflux_billing_events_total{event=%q,plan=%q,status=%q} %d\n", row.Key.Event, row.Key.Plan, row.Key.Status, row.Value)
	}
	_, _ = io.WriteString(w, "# HELP cashflux_billing_mrr_cents Estimated active monthly recurring revenue in cents from billing webhooks.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_billing_mrr_cents gauge\n")
	_, _ = fmt.Fprintf(w, "cashflux_billing_mrr_cents %d\n", billingMRRCents)
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

func (m *Metrics) snapshot() ([]metricRow, []bucketMetricRow, []metricRow, int64, []metricRow, int64, int64, int64, int64, int64, int64, []labelMetricRow, []labelMetricRow, int64, []namedMetricRow, []labelMetricRow, []billingMetricRow, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	httpRows := metricRows(m.http)
	httpBucketRows := bucketMetricRows(m.httpBuckets, m.http)
	grpcRows := metricRows(m.grpc)
	streamRows := metricRows(m.streamDurations)
	return httpRows, httpBucketRows, grpcRows, m.streamsActive, streamRows, m.blobStoredBytes, m.blobTransferredBytes, m.blobGCSweeps, m.blobGCDeleted, m.aiProxyRequests, m.aiProxyTokens, labelMetricRows(m.syncPulls), labelMetricRows(m.syncPushes), m.syncLWWRejects, namedMetricRows(m.db), labelMetricRows(m.queueDepths), billingMetricRows(m.billingEvents), m.billingMRRCents
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
