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
	grpc                 map[metricKey]metricValue
	streamsActive        int64
	streamDurations      map[metricKey]metricValue
	blobStoredBytes      int64
	blobTransferredBytes int64
	aiProxyRequests      int64
	aiProxyTokens        int64
}

type metricKey struct {
	Name   string
	Status string
}

type metricValue struct {
	Count        int64
	DurationSecs float64
}

func NewMetrics() *Metrics {
	return &Metrics{
		http:            map[metricKey]metricValue{},
		grpc:            map[metricKey]metricValue{},
		streamDurations: map[metricKey]metricValue{},
	}
}

func (m *Metrics) ObserveHTTP(route string, status int, elapsed time.Duration) {
	if m == nil {
		return
	}
	m.observe(m.http, metricKey{Name: route, Status: fmt.Sprintf("%d", status)}, elapsed)
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

func (m *Metrics) observe(dst map[metricKey]metricValue, key metricKey, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := dst[key]
	v.Count++
	v.DurationSecs += elapsed.Seconds()
	dst[key] = v
}

func (m *Metrics) WritePrometheus(w io.Writer) {
	if m == nil {
		m = NewMetrics()
	}
	httpRows, grpcRows, activeStreams, streamRows, blobStoredBytes, blobTransferredBytes, aiProxyRequests, aiProxyTokens := m.snapshot()
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
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_requests_total AI proxy completions served by the backend.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_requests_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_requests_total %d\n", aiProxyRequests)
	_, _ = io.WriteString(w, "# HELP cashflux_ai_proxy_tokens_total AI proxy tokens reported by upstream responses.\n")
	_, _ = io.WriteString(w, "# TYPE cashflux_ai_proxy_tokens_total counter\n")
	_, _ = fmt.Fprintf(w, "cashflux_ai_proxy_tokens_total %d\n", aiProxyTokens)
}

type metricRow struct {
	Key   metricKey
	Value metricValue
}

func (m *Metrics) snapshot() ([]metricRow, []metricRow, int64, []metricRow, int64, int64, int64, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	httpRows := metricRows(m.http)
	grpcRows := metricRows(m.grpc)
	streamRows := metricRows(m.streamDurations)
	return httpRows, grpcRows, m.streamsActive, streamRows, m.blobStoredBytes, m.blobTransferredBytes, m.aiProxyRequests, m.aiProxyTokens
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
