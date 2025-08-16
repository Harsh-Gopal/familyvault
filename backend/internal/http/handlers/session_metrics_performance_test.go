package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// TestSessionMetricsPerformanceBasic tests basic performance with moderate dataset
func TestSessionMetricsPerformanceBasic(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with metrics
	sessionID := "perf-metrics-session"
	metricsPath := filepath.Join(tempDir, "uploads", sessionID, "metrics")
	err := os.MkdirAll(metricsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create metrics directory: %v", err)
	}

	// Generate 1000 metrics for basic performance test
	numMetrics := 1000
	var metrics []MetricEntry

	for i := 0; i < numMetrics; i++ {
		timestamp := time.Now().Add(-time.Duration(numMetrics-i) * time.Second)
		metrics = append(metrics, MetricEntry{
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Type:      "cpu",
			Value:     float64(10 + i%80),
			Unit:      "%",
		})
	}

	// Write to JSON file
	jsonFile := filepath.Join(metricsPath, "perf_metrics.json")
	jsonData, _ := json.Marshal(metrics)
	err = os.WriteFile(jsonFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to create metrics file: %v", err)
	}

	// Test basic retrieval performance
	t.Run("basic_retrieval", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/metrics", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

		startTime := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		duration := time.Since(startTime)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Basic retrieval of %d metrics completed in %v", numMetrics, duration)

		// Should be fast (< 1 second)
		if duration > 1*time.Second {
			t.Errorf("Basic retrieval took too long: %v (expected < 1s)", duration)
		}

		var response MetricsResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		if len(response.Metrics) != 1000 {
			t.Errorf("Expected 1000 metrics, got %d", len(response.Metrics))
		}
	})

	// Test filtering performance
	t.Run("filtering_performance", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/metrics?type=cpu&limit=500", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

		startTime := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		duration := time.Since(startTime)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Filtering performance completed in %v", duration)

		// Should be fast (< 500ms)
		if duration > 500*time.Millisecond {
			t.Errorf("Filtering took too long: %v (expected < 500ms)", duration)
		}
	})
}

// BenchmarkSessionMetricsBasic benchmarks basic metrics operations
func BenchmarkSessionMetricsBasic(b *testing.B) {
	tempDir := b.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with metrics
	sessionID := "bench-metrics-session"
	metricsPath := filepath.Join(tempDir, "uploads", sessionID, "metrics")
	err := os.MkdirAll(metricsPath, 0755)
	if err != nil {
		b.Fatalf("Failed to create metrics directory: %v", err)
	}

	// Generate 1000 metrics
	var metrics []MetricEntry
	for i := 0; i < 1000; i++ {
		timestamp := time.Now().Add(-time.Duration(1000-i) * time.Second)
		metrics = append(metrics, MetricEntry{
			Timestamp: timestamp.UTC().Format(time.RFC3339),
			Type:      "cpu",
			Value:     float64(10 + i%80),
			Unit:      "%",
		})
	}

	jsonFile := filepath.Join(metricsPath, "bench_metrics.json")
	jsonData, _ := json.Marshal(metrics)
	err = os.WriteFile(jsonFile, jsonData, 0644)
	if err != nil {
		b.Fatalf("Failed to create metrics file: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/metrics", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Request failed with status %d", w.Code)
		}
	}
}
