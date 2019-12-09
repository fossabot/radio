package cmd

import (
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"
)

// ServerConnStats holds transferred bytes from/to the server
type ServerConnStats struct {
	TotalInputBytes  uint64 `json:"transferred"`
	TotalOutputBytes uint64 `json:"received"`
	Throughput       uint64 `json:"throughput,omitempty"`
	S3InputBytes     uint64 `json:"transferredS3"`
	S3OutputBytes    uint64 `json:"receivedS3"`
}

// ServerHTTPAPIStats holds total number of HTTP operations from/to the server,
// including the average duration the call was spent.
type ServerHTTPAPIStats struct {
	APIStats map[string]int `json:"apiStats"`
}

// ServerHTTPStats holds all type of http operations performed to/from the server
// including their average execution time.
type ServerHTTPStats struct {
	CurrentS3Requests ServerHTTPAPIStats `json:"currentS3Requests"`
	TotalS3Requests   ServerHTTPAPIStats `json:"totalS3Requests"`
	TotalS3Errors     ServerHTTPAPIStats `json:"totalS3Errors"`
}

// ConnStats - Network statistics
// Count total input/output transferred bytes during
// the server's life.
type ConnStats struct {
	totalInputBytes  atomic.Uint64
	totalOutputBytes atomic.Uint64
	s3InputBytes     atomic.Uint64
	s3OutputBytes    atomic.Uint64
}

// Increase total input bytes
func (s *ConnStats) incInputBytes(n int) {
	s.totalInputBytes.Add(uint64(n))
}

// Increase total output bytes
func (s *ConnStats) incOutputBytes(n int) {
	s.totalOutputBytes.Add(uint64(n))
}

// Return total input bytes
func (s *ConnStats) getTotalInputBytes() uint64 {
	return s.totalInputBytes.Load()
}

// Return total output bytes
func (s *ConnStats) getTotalOutputBytes() uint64 {
	return s.totalOutputBytes.Load()
}

// Increase outbound input bytes
func (s *ConnStats) incS3InputBytes(n int) {
	s.s3InputBytes.Add(uint64(n))
}

// Increase outbound output bytes
func (s *ConnStats) incS3OutputBytes(n int) {
	s.s3OutputBytes.Add(uint64(n))
}

// Return outbound input bytes
func (s *ConnStats) getS3InputBytes() uint64 {
	return s.s3InputBytes.Load()
}

// Return outbound output bytes
func (s *ConnStats) getS3OutputBytes() uint64 {
	return s.s3OutputBytes.Load()
}

// Return connection stats (total input/output bytes and total s3 input/output bytes)
func (s *ConnStats) toServerConnStats() ServerConnStats {
	return ServerConnStats{
		TotalInputBytes:  s.getTotalInputBytes(),
		TotalOutputBytes: s.getTotalOutputBytes(),
		S3InputBytes:     s.getS3InputBytes(),
		S3OutputBytes:    s.getS3OutputBytes(),
	}
}

// Prepare new ConnStats structure
func newConnStats() *ConnStats {
	return &ConnStats{}
}

// HTTPAPIStats holds statistics information about
// a given API in the requests.
type HTTPAPIStats struct {
	APIStats map[string]int
	sync.RWMutex
}

// Inc increments the api stats counter.
func (stats *HTTPAPIStats) Inc(api string) {
	stats.Lock()
	defer stats.Unlock()
	if stats == nil {
		return
	}
	if stats.APIStats == nil {
		stats.APIStats = make(map[string]int)
	}
	if _, ok := stats.APIStats[api]; ok {
		stats.APIStats[api]++
		return
	}
	stats.APIStats[api] = 1
}

// Dec increments the api stats counter.
func (stats *HTTPAPIStats) Dec(api string) {
	stats.Lock()
	defer stats.Unlock()
	if stats == nil {
		return
	}
	if val, ok := stats.APIStats[api]; ok && val > 0 {
		stats.APIStats[api]--
	}
}

// Load returns the recorded stats.
func (stats *HTTPAPIStats) Load() map[string]int {
	stats.Lock()
	defer stats.Unlock()
	return stats.APIStats
}

// HTTPStats holds statistics information about
// HTTP requests made by all clients
type HTTPStats struct {
	currentS3Requests HTTPAPIStats
	totalS3Requests   HTTPAPIStats
	totalS3Errors     HTTPAPIStats
}

// Converts http stats into struct to be sent back to the client.
func (st *HTTPStats) toServerHTTPStats() ServerHTTPStats {
	serverStats := ServerHTTPStats{}

	serverStats.CurrentS3Requests = ServerHTTPAPIStats{
		APIStats: st.currentS3Requests.Load(),
	}

	serverStats.TotalS3Requests = ServerHTTPAPIStats{
		APIStats: st.totalS3Requests.Load(),
	}

	serverStats.TotalS3Errors = ServerHTTPAPIStats{
		APIStats: st.totalS3Errors.Load(),
	}
	return serverStats
}

// Update statistics from http request and response data
func (st *HTTPStats) updateStats(api string, r *http.Request, w *recordAPIStats, durationSecs float64) {
	// A successful request has a 2xx response code
	successReq := (w.respStatusCode >= 200 && w.respStatusCode < 300)

	if w.isS3Request && !strings.HasSuffix(r.URL.Path, prometheusMetricsPath) {
		st.totalS3Requests.Inc(api)
		if !successReq && w.respStatusCode != 0 {
			st.totalS3Errors.Inc(api)
		}
	}

	if w.isS3Request && r.Method == "GET" {
		// Increment the prometheus http request response histogram with appropriate label
		httpRequestsDuration.With(prometheus.Labels{"api": api}).Observe(durationSecs)
	}
}

// Prepare new HTTPStats structure
func newHTTPStats() *HTTPStats {
	return &HTTPStats{}
}
