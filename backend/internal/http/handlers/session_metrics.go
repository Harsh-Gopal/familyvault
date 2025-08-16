package handlers

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

// MetricEntry represents a single metric data point
type MetricEntry struct {
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
}

// MetricsQueryParams holds query parameters for metrics requests
type MetricsQueryParams struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Type      string `json:"type"`
	Aggregate string `json:"aggregate"` // none, avg, min, max, sum
	Interval  string `json:"interval"`  // 1m, 5m, 1h, etc.
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	Order     string `json:"order"` // asc, desc
}

// MetricsResponse represents the response structure
type MetricsResponse struct {
	SessionID    string        `json:"session_id"`
	TotalMetrics int           `json:"total_metrics"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	Metrics      []MetricEntry `json:"metrics"`
}

// AggregatedMetric represents an aggregated metric over an interval
type AggregatedMetric struct {
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Count     int     `json:"count,omitempty"`
}

// SessionMetricsHandler handles GET /sessions/:id/metrics
func SessionMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL
	vars := mux.Vars(r)
	sessionID := vars["id"]

	// Validate session ID
	if !isValidSessionID(sessionID) {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	// Resolve and validate authenticated session (header or query)
	authSessionID := r.Header.Get("X-Session-ID")
	if authSessionID == "" {
		authSessionID = r.URL.Query().Get("session_id")
	}
	if authSessionID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	params, err := parseMetricsQueryParams(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Get metrics for the session
	metrics, err := getSessionMetrics(sessionID, params)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Session metrics not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create response
	response := MetricsResponse{
		SessionID:    sessionID,
		TotalMetrics: len(metrics),
		Limit:        params.Limit,
		Offset:       params.Offset,
		Metrics:      metrics,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// parseMetricsQueryParams parses and validates query parameters
func parseMetricsQueryParams(r *http.Request) (*MetricsQueryParams, error) {
	params := &MetricsQueryParams{
		Limit:     1000,   // Default limit
		Offset:    0,      // Default offset
		Aggregate: "none", // Default aggregation
		Order:     "asc",  // Default order
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 10000 {
			return nil, fmt.Errorf("limit must be between 1 and 10000")
		}
		params.Limit = limit
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return nil, fmt.Errorf("offset must be non-negative")
		}
		params.Offset = offset
	}

	// Parse start_time
	if startTime := r.URL.Query().Get("start_time"); startTime != "" {
		_, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			return nil, fmt.Errorf("start_time must be in RFC3339 format")
		}
		params.StartTime = startTime
	}

	// Parse end_time
	if endTime := r.URL.Query().Get("end_time"); endTime != "" {
		_, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			return nil, fmt.Errorf("end_time must be in RFC3339 format")
		}
		params.EndTime = endTime
	}

	// Validate time range
	if params.StartTime != "" && params.EndTime != "" {
		startT, _ := time.Parse(time.RFC3339, params.StartTime)
		endT, _ := time.Parse(time.RFC3339, params.EndTime)
		if startT.After(endT) {
			return nil, fmt.Errorf("start_time cannot be after end_time")
		}
	}

	// Parse type filter
	if metricType := r.URL.Query().Get("type"); metricType != "" {
		// Validate metric type format (alphanumeric and underscores)
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, metricType); !matched {
			return nil, fmt.Errorf("invalid metric type format")
		}
		params.Type = metricType
	}

	// Parse aggregate
	if aggregate := r.URL.Query().Get("aggregate"); aggregate != "" {
		switch aggregate {
		case "none", "avg", "min", "max", "sum":
			params.Aggregate = aggregate
		default:
			return nil, fmt.Errorf("aggregate must be one of: none, avg, min, max, sum")
		}
	}

	// Parse interval
	if interval := r.URL.Query().Get("interval"); interval != "" {
		if !isValidInterval(interval) {
			return nil, fmt.Errorf("invalid interval format (use: 1m, 5m, 1h, etc.)")
		}
		params.Interval = interval
	}

	// Parse order
	if order := r.URL.Query().Get("order"); order != "" {
		switch order {
		case "asc", "desc":
			params.Order = order
		default:
			return nil, fmt.Errorf("order must be either 'asc' or 'desc'")
		}
	}

	return params, nil
}

// isValidInterval validates interval format (e.g., 1m, 5m, 1h)
func isValidInterval(interval string) bool {
	matched, _ := regexp.MatchString(`^[0-9]+[smhd]$`, interval)
	return matched
}

// getSessionMetrics retrieves metrics for a session with filtering and aggregation
func getSessionMetrics(sessionID string, params *MetricsQueryParams) ([]MetricEntry, error) {
	var allMetrics []MetricEntry

	// Try to get metrics from active session first
	activeMetrics, activeErr := getMetricsFromDirectory(sessionID, "active", params)
	if activeErr == nil {
		allMetrics = append(allMetrics, activeMetrics...)
	}

	// If no active session or we want to include backup metrics, check backup
	if activeErr != nil {
		backupMetrics, backupErr := getMetricsFromBackup(sessionID, params)
		if backupErr != nil {
			// If both active and backup failed, return the original error
			if activeErr != nil {
				return nil, activeErr
			}
			return nil, backupErr
		}
		allMetrics = append(allMetrics, backupMetrics...)
	}

	// If no metrics found at all
	if len(allMetrics) == 0 {
		return nil, os.ErrNotExist
	}

	// Sort metrics by timestamp
	sortMetrics(allMetrics, params.Order)

	// Apply aggregation if requested
	if params.Aggregate != "none" && params.Interval != "" {
		aggregatedMetrics, err := aggregateMetrics(allMetrics, params)
		if err != nil {
			return nil, err
		}
		allMetrics = aggregatedMetrics
	}

	// Apply pagination
	start := params.Offset
	end := start + params.Limit

	if start >= len(allMetrics) {
		return []MetricEntry{}, nil
	}

	if end > len(allMetrics) {
		end = len(allMetrics)
	}

	return allMetrics[start:end], nil
}

// getMetricsFromDirectory gets metrics from active session directory
func getMetricsFromDirectory(sessionID, metricType string, params *MetricsQueryParams) ([]MetricEntry, error) {
	metricsPath := filepath.Join(drive.GetDrivePath(), "uploads", sessionID, "metrics")

	// Check if metrics directory exists
	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		return nil, err
	}

	var metrics []MetricEntry

	err := filepath.WalkDir(metricsPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .json and .csv files
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".json" && ext != ".csv" {
			return nil
		}

		// Read metrics from file
		fileMetrics, err := readMetricsFromFile(path, params)
		if err != nil {
			// Log error but continue processing other files
			return nil
		}

		metrics = append(metrics, fileMetrics...)
		return nil
	})

	return metrics, err
}

// getMetricsFromBackup gets metrics from backup directory
func getMetricsFromBackup(sessionID string, params *MetricsQueryParams) ([]MetricEntry, error) {
	backupPath, err := findSessionBackupPath(sessionID)
	if err != nil {
		return nil, err
	}

	metricsPath := filepath.Join(backupPath, "metrics")
	if _, err := os.Stat(metricsPath); os.IsNotExist(err) {
		return nil, err
	}

	var metrics []MetricEntry

	err = filepath.WalkDir(metricsPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Only process .json and .csv files
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".json" && ext != ".csv" {
			return nil
		}

		// Read metrics from file
		fileMetrics, err := readMetricsFromFile(path, params)
		if err != nil {
			// Log error but continue processing other files
			return nil
		}

		metrics = append(metrics, fileMetrics...)
		return nil
	})

	return metrics, err
}

// readMetricsFromFile reads metrics from a JSON or CSV file with streaming
func readMetricsFromFile(filePath string, params *MetricsQueryParams) ([]MetricEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".json":
		return readJSONMetrics(file, params)
	case ".csv":
		return readCSVMetrics(file, params)
	default:
		return nil, fmt.Errorf("unsupported file format: %s", ext)
	}
}

// readJSONMetrics reads metrics from a JSON file
func readJSONMetrics(file *os.File, params *MetricsQueryParams) ([]MetricEntry, error) {
	var metrics []MetricEntry
	decoder := json.NewDecoder(file)

	// Handle both array of metrics and line-delimited JSON
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if delim, ok := token.(json.Delim); ok && delim == '[' {
		// Array format
		for decoder.More() {
			var metric MetricEntry
			if err := decoder.Decode(&metric); err != nil {
				continue // Skip invalid entries
			}

			if matchesMetricFilters(metric, params) {
				metrics = append(metrics, metric)
			}
		}
	} else {
		// Line-delimited JSON - reset and read line by line
		file.Seek(0, 0)
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var metric MetricEntry
			if err := json.Unmarshal(scanner.Bytes(), &metric); err != nil {
				continue // Skip invalid lines
			}

			if matchesMetricFilters(metric, params) {
				metrics = append(metrics, metric)
			}
		}
	}

	return metrics, nil
}

// readCSVMetrics reads metrics from a CSV file
func readCSVMetrics(file *os.File, params *MetricsQueryParams) ([]MetricEntry, error) {
	var metrics []MetricEntry
	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Find column indices
	timestampIdx, typeIdx, valueIdx, unitIdx := -1, -1, -1, -1
	for i, col := range header {
		switch strings.ToLower(col) {
		case "timestamp", "time":
			timestampIdx = i
		case "type", "metric_type":
			typeIdx = i
		case "value":
			valueIdx = i
		case "unit":
			unitIdx = i
		}
	}

	if timestampIdx == -1 || typeIdx == -1 || valueIdx == -1 {
		return nil, fmt.Errorf("CSV missing required columns (timestamp, type, value)")
	}

	// Read data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip invalid rows
		}

		if len(record) <= timestampIdx || len(record) <= typeIdx || len(record) <= valueIdx {
			continue // Skip incomplete rows
		}

		value, err := strconv.ParseFloat(record[valueIdx], 64)
		if err != nil {
			continue // Skip invalid values
		}

		unit := ""
		if unitIdx != -1 && len(record) > unitIdx {
			unit = record[unitIdx]
		}

		metric := MetricEntry{
			Timestamp: record[timestampIdx],
			Type:      record[typeIdx],
			Value:     value,
			Unit:      unit,
		}

		if matchesMetricFilters(metric, params) {
			metrics = append(metrics, metric)
		}
	}

	return metrics, nil
}

// matchesMetricFilters checks if a metric matches the specified filters
func matchesMetricFilters(metric MetricEntry, params *MetricsQueryParams) bool {
	// Type filter
	if params.Type != "" && metric.Type != params.Type {
		return false
	}

	// Time range filters
	if params.StartTime != "" || params.EndTime != "" {
		metricTime, err := time.Parse(time.RFC3339, metric.Timestamp)
		if err != nil {
			// Try alternative formats
			if metricTime, err = time.Parse("2006-01-02T15:04:05Z", metric.Timestamp); err != nil {
				if metricTime, err = time.Parse("2006-01-02 15:04:05", metric.Timestamp); err != nil {
					return false // Skip metrics with unparseable timestamps
				}
			}
		}

		if params.StartTime != "" {
			startTime, _ := time.Parse(time.RFC3339, params.StartTime)
			if metricTime.Before(startTime) {
				return false
			}
		}

		if params.EndTime != "" {
			endTime, _ := time.Parse(time.RFC3339, params.EndTime)
			if metricTime.After(endTime) {
				return false
			}
		}
	}

	return true
}

// sortMetrics sorts metrics by timestamp
func sortMetrics(metrics []MetricEntry, order string) {
	sort.Slice(metrics, func(i, j int) bool {
		timeI, errI := time.Parse(time.RFC3339, metrics[i].Timestamp)
		timeJ, errJ := time.Parse(time.RFC3339, metrics[j].Timestamp)

		// Handle parsing errors by falling back to string comparison
		if errI != nil || errJ != nil {
			if order == "desc" {
				return metrics[i].Timestamp > metrics[j].Timestamp
			}
			return metrics[i].Timestamp < metrics[j].Timestamp
		}

		if order == "desc" {
			return timeI.After(timeJ)
		}
		return timeI.Before(timeJ)
	})
}

// aggregateMetrics aggregates metrics over specified intervals
func aggregateMetrics(metrics []MetricEntry, params *MetricsQueryParams) ([]MetricEntry, error) {
	if params.Interval == "" {
		return metrics, nil
	}

	// Parse interval duration
	duration, err := parseInterval(params.Interval)
	if err != nil {
		return nil, err
	}

	// Group metrics by type and interval
	buckets := make(map[string]map[int64][]MetricEntry)

	for _, metric := range metrics {
		metricTime, err := time.Parse(time.RFC3339, metric.Timestamp)
		if err != nil {
			continue // Skip metrics with unparseable timestamps
		}

		// Calculate bucket timestamp (rounded down to interval)
		bucketTime := metricTime.Truncate(duration).Unix()

		if buckets[metric.Type] == nil {
			buckets[metric.Type] = make(map[int64][]MetricEntry)
		}

		buckets[metric.Type][bucketTime] = append(buckets[metric.Type][bucketTime], metric)
	}

	// Aggregate each bucket
	var aggregatedMetrics []MetricEntry

	for metricType, typeBuckets := range buckets {
		for bucketTime, bucketMetrics := range typeBuckets {
			if len(bucketMetrics) == 0 {
				continue
			}

			aggregatedValue := calculateAggregation(bucketMetrics, params.Aggregate)

			aggregatedMetrics = append(aggregatedMetrics, MetricEntry{
				Timestamp: time.Unix(bucketTime, 0).UTC().Format(time.RFC3339),
				Type:      metricType,
				Value:     aggregatedValue,
				Unit:      bucketMetrics[0].Unit, // Use unit from first metric
			})
		}
	}

	// Sort aggregated metrics
	sortMetrics(aggregatedMetrics, params.Order)

	return aggregatedMetrics, nil
}

// parseInterval parses interval string to duration
func parseInterval(interval string) (time.Duration, error) {
	if len(interval) < 2 {
		return 0, fmt.Errorf("invalid interval format")
	}

	numStr := interval[:len(interval)-1]
	unit := interval[len(interval)-1:]

	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval number")
	}

	switch unit {
	case "s":
		return time.Duration(num) * time.Second, nil
	case "m":
		return time.Duration(num) * time.Minute, nil
	case "h":
		return time.Duration(num) * time.Hour, nil
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid interval unit")
	}
}

// calculateAggregation calculates aggregated value based on aggregation type
func calculateAggregation(metrics []MetricEntry, aggregateType string) float64 {
	if len(metrics) == 0 {
		return 0
	}

	switch aggregateType {
	case "avg":
		sum := 0.0
		for _, metric := range metrics {
			sum += metric.Value
		}
		return sum / float64(len(metrics))

	case "min":
		min := metrics[0].Value
		for _, metric := range metrics {
			if metric.Value < min {
				min = metric.Value
			}
		}
		return min

	case "max":
		max := metrics[0].Value
		for _, metric := range metrics {
			if metric.Value > max {
				max = metric.Value
			}
		}
		return max

	case "sum":
		sum := 0.0
		for _, metric := range metrics {
			sum += metric.Value
		}
		return sum

	default: // "none" or invalid
		return metrics[0].Value
	}
}
