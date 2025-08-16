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

func TestSessionMetricsHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with metrics
	sessionID := "test-session-metrics"
	metricsPath := filepath.Join(tempDir, "uploads", sessionID, "metrics")
	err := os.MkdirAll(metricsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create metrics directory: %v", err)
	}

	// Create test JSON metrics file
	jsonMetrics := []MetricEntry{
		{Timestamp: "2025-08-12T10:00:00Z", Type: "cpu", Value: 45.5, Unit: "%"},
		{Timestamp: "2025-08-12T10:01:00Z", Type: "cpu", Value: 50.2, Unit: "%"},
		{Timestamp: "2025-08-12T10:02:00Z", Type: "memory", Value: 1024, Unit: "MB"},
		{Timestamp: "2025-08-12T10:03:00Z", Type: "memory", Value: 1100, Unit: "MB"},
		{Timestamp: "2025-08-12T10:04:00Z", Type: "disk", Value: 85.3, Unit: "%"},
	}

	jsonFile := filepath.Join(metricsPath, "metrics.json")
	jsonData, _ := json.Marshal(jsonMetrics)
	err = os.WriteFile(jsonFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to create JSON metrics file: %v", err)
	}

	// Create test CSV metrics file
	csvContent := `timestamp,type,value,unit
2025-08-12T10:05:00Z,network,125.5,Mbps
2025-08-12T10:06:00Z,network,130.2,Mbps
2025-08-12T10:07:00Z,cpu,42.1,%
2025-08-12T10:08:00Z,disk,87.9,%`

	csvFile := filepath.Join(metricsPath, "metrics.csv")
	err = os.WriteFile(csvFile, []byte(csvContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create CSV metrics file: %v", err)
	}

	tests := []struct {
		name           string
		sessionID      string
		queryParams    string
		expectedStatus int
		expectedCount  int
		checkResponse  func(*testing.T, *MetricsResponse)
	}{
		{
			name:           "basic metrics retrieval",
			sessionID:      sessionID,
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  9, // 5 from JSON + 4 from CSV
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if resp.SessionID != sessionID {
					t.Errorf("Expected session_id %s, got %s", sessionID, resp.SessionID)
				}
				if resp.TotalMetrics != 9 {
					t.Errorf("Expected 9 total metrics, got %d", resp.TotalMetrics)
				}
				if len(resp.Metrics) != 9 {
					t.Errorf("Expected 9 metrics in response, got %d", len(resp.Metrics))
				}
			},
		},
		{
			name:           "limit parameter",
			sessionID:      sessionID,
			queryParams:    "limit=5",
			expectedStatus: http.StatusOK,
			expectedCount:  5,
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) != 5 {
					t.Errorf("Expected 5 metrics with limit=5, got %d", len(resp.Metrics))
				}
				if resp.Limit != 5 {
					t.Errorf("Expected limit=5 in response, got %d", resp.Limit)
				}
			},
		},
		{
			name:           "offset parameter",
			sessionID:      sessionID,
			queryParams:    "offset=3&limit=3",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) != 3 {
					t.Errorf("Expected 3 metrics with offset=3&limit=3, got %d", len(resp.Metrics))
				}
				if resp.Offset != 3 {
					t.Errorf("Expected offset=3 in response, got %d", resp.Offset)
				}
			},
		},
		{
			name:           "type filter",
			sessionID:      sessionID,
			queryParams:    "type=cpu",
			expectedStatus: http.StatusOK,
			expectedCount:  3, // 2 from JSON + 1 from CSV
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) != 3 {
					t.Errorf("Expected 3 cpu metrics, got %d", len(resp.Metrics))
				}
				for _, metric := range resp.Metrics {
					if metric.Type != "cpu" {
						t.Errorf("Expected all metrics to be cpu type, got %s", metric.Type)
					}
				}
			},
		},
		{
			name:           "time range filter",
			sessionID:      sessionID,
			queryParams:    "start_time=2025-08-12T10:02:00Z&end_time=2025-08-12T10:06:00Z",
			expectedStatus: http.StatusOK,
			expectedCount:  5, // metrics between 10:02 and 10:06 (inclusive)
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) != 5 {
					t.Errorf("Expected 5 metrics in time range, got %d", len(resp.Metrics))
				}
				// Verify all metrics are within time range
				startTime, _ := time.Parse(time.RFC3339, "2025-08-12T10:02:00Z")
				endTime, _ := time.Parse(time.RFC3339, "2025-08-12T10:06:00Z")
				for _, metric := range resp.Metrics {
					metricTime, err := time.Parse(time.RFC3339, metric.Timestamp)
					if err != nil {
						t.Errorf("Failed to parse metric timestamp: %v", err)
						continue
					}
					if metricTime.Before(startTime) || metricTime.After(endTime) {
						t.Errorf("Metric timestamp %s is outside range", metric.Timestamp)
					}
				}
			},
		},
		{
			name:           "descending order",
			sessionID:      sessionID,
			queryParams:    "order=desc&limit=3",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) < 2 {
					return
				}
				// Verify descending order
				for i := 1; i < len(resp.Metrics); i++ {
					time1, _ := time.Parse(time.RFC3339, resp.Metrics[i-1].Timestamp)
					time2, _ := time.Parse(time.RFC3339, resp.Metrics[i].Timestamp)
					if time1.Before(time2) {
						t.Errorf("Metrics not in descending order: %s before %s",
							resp.Metrics[i-1].Timestamp, resp.Metrics[i].Timestamp)
					}
				}
			},
		},
		{
			name:           "combined filters",
			sessionID:      sessionID,
			queryParams:    "type=memory&start_time=2025-08-12T10:00:00Z&limit=5",
			expectedStatus: http.StatusOK,
			expectedCount:  2, // 2 memory metrics
			checkResponse: func(t *testing.T, resp *MetricsResponse) {
				if len(resp.Metrics) != 2 {
					t.Errorf("Expected 2 memory metrics, got %d", len(resp.Metrics))
				}
				for _, metric := range resp.Metrics {
					if metric.Type != "memory" {
						t.Errorf("Expected all metrics to be memory type, got %s", metric.Type)
					}
				}
			},
		},
		{
			name:           "invalid session ID",
			sessionID:      "invalid@session",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedCount:  0,
		},
		{
			name:           "non-existent session",
			sessionID:      "non-existent-session",
			queryParams:    "",
			expectedStatus: http.StatusNotFound,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/sessions/%s/metrics", tt.sessionID)
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Session-ID", "test-session")

			// Set up mux router to extract session ID
			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
				return
			}

			if tt.expectedStatus == http.StatusOK {
				var response MetricsResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(response.Metrics) != tt.expectedCount {
					t.Errorf("Expected %d metrics, got %d", tt.expectedCount, len(response.Metrics))
				}

				// Verify metric structure
				for _, metric := range response.Metrics {
					if metric.Timestamp == "" {
						t.Error("Metric timestamp is empty")
					}
					if metric.Type == "" {
						t.Error("Metric type is empty")
					}
					if metric.Value == 0 && metric.Type != "zero_metric" {
						t.Error("Metric value is zero (might be invalid)")
					}
				}

				if tt.checkResponse != nil {
					tt.checkResponse(t, &response)
				}
			}
		})
	}
}

func TestSessionMetricsHandlerBackup(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create backup session with metrics
	sessionID := "backup-metrics-session"
	backupPath := filepath.Join(tempDir, "backup", "2025-08-12", sessionID, "metrics")
	err := os.MkdirAll(backupPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create backup metrics directory: %v", err)
	}

	// Create backup metrics
	backupMetrics := []MetricEntry{
		{Timestamp: "2025-08-12T09:00:00Z", Type: "cpu", Value: 30.5, Unit: "%"},
		{Timestamp: "2025-08-12T09:01:00Z", Type: "memory", Value: 512, Unit: "MB"},
	}

	jsonFile := filepath.Join(backupPath, "backup_metrics.json")
	jsonData, _ := json.Marshal(backupMetrics)
	err = os.WriteFile(jsonFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to create backup metrics file: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/metrics", sessionID), nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
		return
	}

	var response MetricsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Metrics) != 2 {
		t.Errorf("Expected 2 backup metrics, got %d", len(response.Metrics))
	}
}

func TestParseMetricsQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		expectError bool
		checkParams func(*testing.T, *MetricsQueryParams)
	}{
		{
			name:        "default parameters",
			queryString: "",
			expectError: false,
			checkParams: func(t *testing.T, p *MetricsQueryParams) {
				if p.Limit != 1000 {
					t.Errorf("Expected default limit 1000, got %d", p.Limit)
				}
				if p.Offset != 0 {
					t.Errorf("Expected default offset 0, got %d", p.Offset)
				}
				if p.Aggregate != "none" {
					t.Errorf("Expected default aggregate 'none', got '%s'", p.Aggregate)
				}
				if p.Order != "asc" {
					t.Errorf("Expected default order 'asc', got '%s'", p.Order)
				}
			},
		},
		{
			name:        "custom parameters",
			queryString: "limit=500&offset=100&type=cpu&aggregate=avg&interval=5m&order=desc",
			expectError: false,
			checkParams: func(t *testing.T, p *MetricsQueryParams) {
				if p.Limit != 500 {
					t.Errorf("Expected limit 500, got %d", p.Limit)
				}
				if p.Offset != 100 {
					t.Errorf("Expected offset 100, got %d", p.Offset)
				}
				if p.Type != "cpu" {
					t.Errorf("Expected type 'cpu', got '%s'", p.Type)
				}
				if p.Aggregate != "avg" {
					t.Errorf("Expected aggregate 'avg', got '%s'", p.Aggregate)
				}
				if p.Interval != "5m" {
					t.Errorf("Expected interval '5m', got '%s'", p.Interval)
				}
				if p.Order != "desc" {
					t.Errorf("Expected order 'desc', got '%s'", p.Order)
				}
			},
		},
		{
			name:        "time range parameters",
			queryString: "start_time=2025-08-12T10:00:00Z&end_time=2025-08-12T11:00:00Z",
			expectError: false,
			checkParams: func(t *testing.T, p *MetricsQueryParams) {
				if p.StartTime != "2025-08-12T10:00:00Z" {
					t.Errorf("Expected start_time '2025-08-12T10:00:00Z', got '%s'", p.StartTime)
				}
				if p.EndTime != "2025-08-12T11:00:00Z" {
					t.Errorf("Expected end_time '2025-08-12T11:00:00Z', got '%s'", p.EndTime)
				}
			},
		},
		{
			name:        "invalid limit",
			queryString: "limit=0",
			expectError: true,
		},
		{
			name:        "limit too high",
			queryString: "limit=20000",
			expectError: true,
		},
		{
			name:        "negative offset",
			queryString: "offset=-1",
			expectError: true,
		},
		{
			name:        "invalid start_time",
			queryString: "start_time=invalid-time",
			expectError: true,
		},
		{
			name:        "invalid time range",
			queryString: "start_time=2025-08-12T11:00:00Z&end_time=2025-08-12T10:00:00Z",
			expectError: true,
		},
		{
			name:        "invalid aggregate",
			queryString: "aggregate=invalid",
			expectError: true,
		},
		{
			name:        "invalid interval",
			queryString: "interval=invalid",
			expectError: true,
		},
		{
			name:        "invalid order",
			queryString: "order=invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)

			params, err := parseMetricsQueryParams(req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkParams != nil {
				tt.checkParams(t, params)
			}
		})
	}
}

func TestMatchesMetricFilters(t *testing.T) {
	tests := []struct {
		name     string
		metric   MetricEntry
		params   *MetricsQueryParams
		expected bool
	}{
		{
			name:     "no filters",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:00:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{},
			expected: true,
		},
		{
			name:     "type filter match",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:00:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{Type: "cpu"},
			expected: true,
		},
		{
			name:     "type filter no match",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:00:00Z", Type: "memory", Value: 50.0},
			params:   &MetricsQueryParams{Type: "cpu"},
			expected: false,
		},
		{
			name:     "time range match",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:30:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: true,
		},
		{
			name:     "time range before start",
			metric:   MetricEntry{Timestamp: "2025-08-12T09:30:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: false,
		},
		{
			name:     "time range after end",
			metric:   MetricEntry{Timestamp: "2025-08-12T11:30:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: false,
		},
		{
			name:     "combined filters match",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:30:00Z", Type: "cpu", Value: 50.0},
			params:   &MetricsQueryParams{Type: "cpu", StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: true,
		},
		{
			name:     "combined filters no match",
			metric:   MetricEntry{Timestamp: "2025-08-12T10:30:00Z", Type: "memory", Value: 50.0},
			params:   &MetricsQueryParams{Type: "cpu", StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesMetricFilters(tt.metric, tt.params)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCalculateAggregation(t *testing.T) {
	metrics := []MetricEntry{
		{Value: 10.0},
		{Value: 20.0},
		{Value: 30.0},
		{Value: 40.0},
	}

	tests := []struct {
		name          string
		aggregateType string
		expected      float64
	}{
		{"average", "avg", 25.0},
		{"minimum", "min", 10.0},
		{"maximum", "max", 40.0},
		{"sum", "sum", 100.0},
		{"none", "none", 10.0}, // Should return first value
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAggregation(metrics, tt.aggregateType)
			if result != tt.expected {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		expected time.Duration
		hasError bool
	}{
		{"seconds", "30s", 30 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"days", "1d", 24 * time.Hour, false},
		{"invalid unit", "5x", 0, true},
		{"invalid format", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInterval(tt.interval)

			if tt.hasError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSessionMetricsHandlerAuthentication(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "auth-test-session"

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/metrics", sessionID), nil)
	// No authentication header

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestSessionMetricsHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions/test/metrics", nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/metrics", SessionMetricsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST method, got %d", w.Code)
	}
}
