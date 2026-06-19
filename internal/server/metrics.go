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
	mu   sync.Mutex
	http map[metricKey]metricValue
	grpc map[metricKey]metricValue
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
		http: map[metricKey]metricValue{},
		grpc: map[metricKey]metricValue{},
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
	httpRows, grpcRows := m.snapshot()
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
}

type metricRow struct {
	Key   metricKey
	Value metricValue
}

func (m *Metrics) snapshot() ([]metricRow, []metricRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	httpRows := metricRows(m.http)
	grpcRows := metricRows(m.grpc)
	return httpRows, grpcRows
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
