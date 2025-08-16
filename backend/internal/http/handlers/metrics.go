package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"familyvault/internal/core/drive"
)

// MetricsCollector collects and stores metrics
type MetricsCollector struct {
	mu               sync.RWMutex
	requestCount     map[string]int64
	requestLatency   map[string][]time.Duration
	activeSessions   int64
	fileStorageUsage int64
	lastUpdate       time.Time
}

var metricsCollector = &MetricsCollector{
	requestCount:   make(map[string]int64),
	requestLatency: make(map[string][]time.Duration),
	lastUpdate:     time.Now(),
}

// MetricsHandler handles GET /metrics (Prometheus format)
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Update metrics
	metricsCollector.updateMetrics()

	// Generate Prometheus format metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	metricsCollector.mu.RLock()
	defer metricsCollector.mu.RUnlock()

	// Request count metrics
	fmt.Fprintf(w, "# HELP familyvault_requests_total Total number of HTTP requests\n")
	fmt.Fprintf(w, "# TYPE familyvault_requests_total counter\n")
	for endpoint, count := range metricsCollector.requestCount {
		fmt.Fprintf(w, "familyvault_requests_total{endpoint=\"%s\"} %d\n", endpoint, count)
	}

	// Request latency metrics (average)
	fmt.Fprintf(w, "# HELP familyvault_request_duration_seconds Average request duration in seconds\n")
	fmt.Fprintf(w, "# TYPE familyvault_request_duration_seconds gauge\n")
	for endpoint, latencies := range metricsCollector.requestLatency {
		if len(latencies) > 0 {
			var total time.Duration
			for _, latency := range latencies {
				total += latency
			}
			avg := total / time.Duration(len(latencies))
			fmt.Fprintf(w, "familyvault_request_duration_seconds{endpoint=\"%s\"} %.6f\n",
				endpoint, avg.Seconds())
		}
	}

	// Active sessions
	fmt.Fprintf(w, "# HELP familyvault_active_sessions Number of active sessions\n")
	fmt.Fprintf(w, "# TYPE familyvault_active_sessions gauge\n")
	fmt.Fprintf(w, "familyvault_active_sessions %d\n", metricsCollector.activeSessions)

	// File storage usage
	fmt.Fprintf(w, "# HELP familyvault_storage_bytes Total storage usage in bytes\n")
	fmt.Fprintf(w, "# TYPE familyvault_storage_bytes gauge\n")
	fmt.Fprintf(w, "familyvault_storage_bytes %d\n", metricsCollector.fileStorageUsage)

	// Memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Fprintf(w, "# HELP familyvault_memory_bytes Memory usage in bytes\n")
	fmt.Fprintf(w, "# TYPE familyvault_memory_bytes gauge\n")
	fmt.Fprintf(w, "familyvault_memory_bytes{type=\"alloc\"} %d\n", memStats.Alloc)
	fmt.Fprintf(w, "familyvault_memory_bytes{type=\"sys\"} %d\n", memStats.Sys)

	// Goroutines
	fmt.Fprintf(w, "# HELP familyvault_goroutines Number of goroutines\n")
	fmt.Fprintf(w, "# TYPE familyvault_goroutines gauge\n")
	fmt.Fprintf(w, "familyvault_goroutines %d\n", runtime.NumGoroutine())

	// Uptime
	uptime := time.Since(startTime)
	fmt.Fprintf(w, "# HELP familyvault_uptime_seconds Uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE familyvault_uptime_seconds counter\n")
	fmt.Fprintf(w, "familyvault_uptime_seconds %.0f\n", uptime.Seconds())
}

// RecordRequest records a request for metrics
func (mc *MetricsCollector) RecordRequest(endpoint string, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.requestCount[endpoint]++

	// Keep only last 1000 latencies per endpoint to avoid memory growth
	latencies := mc.requestLatency[endpoint]
	if len(latencies) >= 1000 {
		latencies = latencies[1:]
	}
	mc.requestLatency[endpoint] = append(latencies, duration)
}

// updateMetrics updates cached metrics
func (mc *MetricsCollector) updateMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Update only every 30 seconds to avoid excessive I/O
	if time.Since(mc.lastUpdate) < 30*time.Second {
		return
	}

	// Count active sessions
	mc.activeSessions = int64(countActiveSessions())

	// Calculate storage usage
	mc.fileStorageUsage = calculateStorageUsage()

	mc.lastUpdate = time.Now()
}

// calculateStorageUsage calculates total storage usage
func calculateStorageUsage() int64 {
	basePath := drive.GetDrivePath()
	var totalSize int64

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize
}

// MetricsMiddleware wraps handlers to collect metrics
func MetricsMiddleware(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the actual handler
		handler(w, r)

		// Record metrics
		duration := time.Since(start)
		metricsCollector.RecordRequest(endpoint, duration)
	}
}
