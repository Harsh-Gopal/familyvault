package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"

	"github.com/gorilla/mux"
)

func TestSessionArtifactsHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with artifacts
	sessionID := "test-session-artifacts"
	artifactsPath := filepath.Join(tempDir, "uploads", sessionID, "artifacts")
	err := os.MkdirAll(artifactsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create artifacts directory: %v", err)
	}

	// Create test artifacts with different types and nested directories
	testArtifacts := []struct {
		path    string
		content string
		size    int64
	}{
		{"report.pdf", "PDF content", 11},
		{"screenshots/image1.png", "PNG content", 11},
		{"screenshots/image2.jpg", "JPG content", 11},
		{"data/results.csv", "CSV content", 11},
		{"logs/output.log", "Log content", 11},
		{"archive.zip", "ZIP content", 11},
	}

	for i, artifact := range testArtifacts {
		// Create subdirectories if needed
		artifactPath := filepath.Join(artifactsPath, artifact.path)
		artifactDir := filepath.Dir(artifactPath)
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			t.Fatalf("Failed to create artifact directory %s: %v", artifactDir, err)
		}

		// Create artifact file
		err := os.WriteFile(artifactPath, []byte(artifact.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create artifact file %s: %v", artifact.path, err)
		}

		// Set specific modification time for testing
		modTime := time.Now().Add(-time.Duration(len(testArtifacts)-i) * time.Minute)
		os.Chtimes(artifactPath, modTime, modTime)
	}

	tests := []struct {
		name           string
		sessionID      string
		queryParams    string
		expectedStatus int
		expectedCount  int
		checkResponse  func(*testing.T, *ArtifactsResponse)
	}{
		{
			name:           "basic artifacts retrieval",
			sessionID:      sessionID,
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedCount:  6,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if resp.SessionID != sessionID {
					t.Errorf("Expected session_id %s, got %s", sessionID, resp.SessionID)
				}
				if resp.ArtifactCount != 6 {
					t.Errorf("Expected 6 artifacts, got %d", resp.ArtifactCount)
				}
				if len(resp.Artifacts) != 6 {
					t.Errorf("Expected 6 artifacts in response, got %d", len(resp.Artifacts))
				}
			},
		},
		{
			name:           "limit parameter",
			sessionID:      sessionID,
			queryParams:    "limit=3",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 3 {
					t.Errorf("Expected 3 artifacts with limit=3, got %d", len(resp.Artifacts))
				}
			},
		},
		{
			name:           "offset parameter",
			sessionID:      sessionID,
			queryParams:    "offset=2&limit=2",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 2 {
					t.Errorf("Expected 2 artifacts with offset=2&limit=2, got %d", len(resp.Artifacts))
				}
			},
		},
		{
			name:           "type filter",
			sessionID:      sessionID,
			queryParams:    "type=application/pdf",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 1 {
					t.Errorf("Expected 1 PDF artifact, got %d", len(resp.Artifacts))
				}
				if len(resp.Artifacts) > 0 && resp.Artifacts[0].Name != "report.pdf" {
					t.Errorf("Expected report.pdf, got %s", resp.Artifacts[0].Name)
				}
				if len(resp.Artifacts) > 0 && resp.Artifacts[0].Type != "application/pdf" {
					t.Errorf("Expected application/pdf type, got %s", resp.Artifacts[0].Type)
				}
			},
		},
		{
			name:           "name contains filter",
			sessionID:      sessionID,
			queryParams:    "name_contains=image",
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 2 {
					t.Errorf("Expected 2 image artifacts, got %d", len(resp.Artifacts))
				}
				for _, artifact := range resp.Artifacts {
					if !strings.Contains(strings.ToLower(artifact.Name), "image") {
						t.Errorf("Expected artifact name to contain 'image', got %s", artifact.Name)
					}
				}
			},
		},
		{
			name:           "time range filter",
			sessionID:      sessionID,
			queryParams:    "start_time=2020-01-01T00:00:00Z&end_time=2030-12-31T23:59:59Z",
			expectedStatus: http.StatusOK,
			expectedCount:  6, // All artifacts should be in this broad range
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 6 {
					t.Errorf("Expected 6 artifacts in time range, got %d", len(resp.Artifacts))
				}
			},
		},
		{
			name:           "descending order",
			sessionID:      sessionID,
			queryParams:    "order=desc&limit=3",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) < 2 {
					return
				}
				// Verify descending order
				for i := 1; i < len(resp.Artifacts); i++ {
					time1, _ := time.Parse(time.RFC3339, resp.Artifacts[i-1].LastModified)
					time2, _ := time.Parse(time.RFC3339, resp.Artifacts[i].LastModified)
					if time1.Before(time2) {
						t.Errorf("Artifacts not in descending order: %s before %s",
							resp.Artifacts[i-1].LastModified, resp.Artifacts[i].LastModified)
					}
				}
			},
		},
		{
			name:           "combined filters",
			sessionID:      sessionID,
			queryParams:    "name_contains=screenshots&type=image/png",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
			checkResponse: func(t *testing.T, resp *ArtifactsResponse) {
				if len(resp.Artifacts) != 1 {
					t.Errorf("Expected 1 PNG screenshot, got %d", len(resp.Artifacts))
				}
				if len(resp.Artifacts) > 0 {
					artifact := resp.Artifacts[0]
					if !strings.Contains(artifact.Name, "screenshots") {
						t.Errorf("Expected artifact in screenshots folder, got %s", artifact.Name)
					}
					if artifact.Type != "image/png" {
						t.Errorf("Expected PNG type, got %s", artifact.Type)
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
			url := fmt.Sprintf("/sessions/%s/artifacts", tt.sessionID)
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Session-ID", "test-session")

			// Set up mux router to extract session ID
			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
				return
			}

			if tt.expectedStatus == http.StatusOK {
				var response ArtifactsResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(response.Artifacts) != tt.expectedCount {
					t.Errorf("Expected %d artifacts, got %d", tt.expectedCount, len(response.Artifacts))
				}

				// Verify artifact structure
				for _, artifact := range response.Artifacts {
					if artifact.Name == "" {
						t.Error("Artifact name is empty")
					}
					if artifact.SizeBytes <= 0 {
						t.Error("Artifact size is zero or negative")
					}
					if artifact.LastModified == "" {
						t.Error("Artifact last modified time is empty")
					}
					if artifact.Type == "" {
						t.Error("Artifact type is empty")
					}
				}

				if tt.checkResponse != nil {
					tt.checkResponse(t, &response)
				}
			}
		})
	}
}

func TestSessionArtifactsHandlerBackup(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create backup session with artifacts
	sessionID := "backup-artifacts-session"
	backupPath := filepath.Join(tempDir, "backup", "2025-08-12", sessionID, "artifacts")
	err := os.MkdirAll(backupPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create backup artifacts directory: %v", err)
	}

	// Create backup artifacts
	backupArtifacts := []string{"backup_report.pdf", "backup_data.csv", "backup_image.png"}
	for _, filename := range backupArtifacts {
		filePath := filepath.Join(backupPath, filename)
		err := os.WriteFile(filePath, []byte("backup content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create backup artifact %s: %v", filename, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts", sessionID), nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
		return
	}

	var response ArtifactsResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Artifacts) != 3 {
		t.Errorf("Expected 3 backup artifacts, got %d", len(response.Artifacts))
	}
}

func TestParseArtifactsQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		expectError bool
		checkParams func(*testing.T, *ArtifactsQueryParams)
	}{
		{
			name:        "default parameters",
			queryString: "",
			expectError: false,
			checkParams: func(t *testing.T, p *ArtifactsQueryParams) {
				if p.Limit != 100 {
					t.Errorf("Expected default limit 100, got %d", p.Limit)
				}
				if p.Offset != 0 {
					t.Errorf("Expected default offset 0, got %d", p.Offset)
				}
				if p.Order != "asc" {
					t.Errorf("Expected default order 'asc', got '%s'", p.Order)
				}
			},
		},
		{
			name:        "custom parameters",
			queryString: "limit=50&offset=10&type=image/png&name_contains=test&order=desc",
			expectError: false,
			checkParams: func(t *testing.T, p *ArtifactsQueryParams) {
				if p.Limit != 50 {
					t.Errorf("Expected limit 50, got %d", p.Limit)
				}
				if p.Offset != 10 {
					t.Errorf("Expected offset 10, got %d", p.Offset)
				}
				if p.Type != "image/png" {
					t.Errorf("Expected type 'image/png', got '%s'", p.Type)
				}
				if p.NameContains != "test" {
					t.Errorf("Expected name_contains 'test', got '%s'", p.NameContains)
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
			checkParams: func(t *testing.T, p *ArtifactsQueryParams) {
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
			queryString: "limit=2000",
			expectError: true,
		},
		{
			name:        "negative offset",
			queryString: "offset=-1",
			expectError: true,
		},
		{
			name:        "invalid MIME type",
			queryString: "type=invalid-mime-type",
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
			name:        "invalid order",
			queryString: "order=invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.queryString, nil)

			params, err := parseArtifactsQueryParams(req)

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

func TestMatchesArtifactFilters(t *testing.T) {
	tests := []struct {
		name     string
		artifact ArtifactEntry
		params   *ArtifactsQueryParams
		expected bool
	}{
		{
			name:     "no filters",
			artifact: ArtifactEntry{Name: "test.pdf", Type: "application/pdf", LastModified: "2025-08-12T10:00:00Z"},
			params:   &ArtifactsQueryParams{},
			expected: true,
		},
		{
			name:     "type filter match",
			artifact: ArtifactEntry{Name: "test.pdf", Type: "application/pdf", LastModified: "2025-08-12T10:00:00Z"},
			params:   &ArtifactsQueryParams{Type: "application/pdf"},
			expected: true,
		},
		{
			name:     "type filter no match",
			artifact: ArtifactEntry{Name: "test.png", Type: "image/png", LastModified: "2025-08-12T10:00:00Z"},
			params:   &ArtifactsQueryParams{Type: "application/pdf"},
			expected: false,
		},
		{
			name:     "name contains match",
			artifact: ArtifactEntry{Name: "test_report.pdf", Type: "application/pdf", LastModified: "2025-08-12T10:00:00Z"},
			params:   &ArtifactsQueryParams{NameContains: "report"},
			expected: true,
		},
		{
			name:     "name contains no match",
			artifact: ArtifactEntry{Name: "image.png", Type: "image/png", LastModified: "2025-08-12T10:00:00Z"},
			params:   &ArtifactsQueryParams{NameContains: "report"},
			expected: false,
		},
		{
			name:     "time range match",
			artifact: ArtifactEntry{Name: "test.pdf", Type: "application/pdf", LastModified: "2025-08-12T10:30:00Z"},
			params:   &ArtifactsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: true,
		},
		{
			name:     "time range before start",
			artifact: ArtifactEntry{Name: "test.pdf", Type: "application/pdf", LastModified: "2025-08-12T09:30:00Z"},
			params:   &ArtifactsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: false,
		},
		{
			name:     "time range after end",
			artifact: ArtifactEntry{Name: "test.pdf", Type: "application/pdf", LastModified: "2025-08-12T11:30:00Z"},
			params:   &ArtifactsQueryParams{StartTime: "2025-08-12T10:00:00Z", EndTime: "2025-08-12T11:00:00Z"},
			expected: false,
		},
		{
			name:     "combined filters match",
			artifact: ArtifactEntry{Name: "test_report.pdf", Type: "application/pdf", LastModified: "2025-08-12T10:30:00Z"},
			params:   &ArtifactsQueryParams{Type: "application/pdf", NameContains: "report", StartTime: "2025-08-12T10:00:00Z"},
			expected: true,
		},
		{
			name:     "combined filters no match",
			artifact: ArtifactEntry{Name: "test_image.png", Type: "image/png", LastModified: "2025-08-12T10:30:00Z"},
			params:   &ArtifactsQueryParams{Type: "application/pdf", NameContains: "report", StartTime: "2025-08-12T10:00:00Z"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesArtifactFilters(tt.artifact, tt.params)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"document.pdf", "application/pdf"},
		{"image.png", "image/png"},
		{"image.jpg", "image/jpeg"},
		{"data.csv", "text/csv"}, // Note: may include charset
		{"archive.zip", "application/zip"},
		{"unknown.xyz", "chemical/x-xyz"}, // System-specific MIME type
		{"noextension", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := detectMimeType(tt.filename)
			// For CSV, allow charset parameter
			if tt.filename == "data.csv" && strings.HasPrefix(result, "text/csv") {
				return // Accept text/csv with or without charset
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestSortArtifacts(t *testing.T) {
	artifacts := []ArtifactEntry{
		{Name: "c.pdf", LastModified: "2025-08-12T10:02:00Z"},
		{Name: "a.pdf", LastModified: "2025-08-12T10:00:00Z"},
		{Name: "b.pdf", LastModified: "2025-08-12T10:01:00Z"},
	}

	t.Run("sort ascending", func(t *testing.T) {
		testArtifacts := make([]ArtifactEntry, len(artifacts))
		copy(testArtifacts, artifacts)

		sortArtifacts(testArtifacts, "asc")

		expected := []string{"2025-08-12T10:00:00Z", "2025-08-12T10:01:00Z", "2025-08-12T10:02:00Z"}
		for i, artifact := range testArtifacts {
			if artifact.LastModified != expected[i] {
				t.Errorf("Expected %s at position %d, got %s", expected[i], i, artifact.LastModified)
			}
		}
	})

	t.Run("sort descending", func(t *testing.T) {
		testArtifacts := make([]ArtifactEntry, len(artifacts))
		copy(testArtifacts, artifacts)

		sortArtifacts(testArtifacts, "desc")

		expected := []string{"2025-08-12T10:02:00Z", "2025-08-12T10:01:00Z", "2025-08-12T10:00:00Z"}
		for i, artifact := range testArtifacts {
			if artifact.LastModified != expected[i] {
				t.Errorf("Expected %s at position %d, got %s", expected[i], i, artifact.LastModified)
			}
		}
	})
}

func TestRemoveDuplicateArtifacts(t *testing.T) {
	artifacts := []ArtifactEntry{
		{Name: "file1.pdf", SizeBytes: 100},
		{Name: "file2.png", SizeBytes: 200},
		{Name: "file1.pdf", SizeBytes: 150}, // Duplicate
		{Name: "file3.txt", SizeBytes: 300},
	}

	result := removeDuplicateArtifacts(artifacts)

	if len(result) != 3 {
		t.Errorf("Expected 3 unique artifacts, got %d", len(result))
	}

	// Check that first occurrence is kept
	for _, artifact := range result {
		if artifact.Name == "file1.pdf" && artifact.SizeBytes != 100 {
			t.Errorf("Expected first occurrence of file1.pdf (size 100), got size %d", artifact.SizeBytes)
		}
	}
}

func TestSessionArtifactsHandlerAuthentication(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "auth-test-session"

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts", sessionID), nil)
	// No authentication header

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestSessionArtifactsHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions/test/artifacts", nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST method, got %d", w.Code)
	}
}

func TestSessionArtifactsHandlerPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Test various path traversal attempts
	pathTraversalAttempts := []string{
		"invalid@session",
		"session..invalid",
		"session\\invalid",
		"session-with-dots..",
	}

	for _, sessionID := range pathTraversalAttempts {
		t.Run(fmt.Sprintf("path_traversal_%s", sessionID), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for path traversal attempt %s, got %d", sessionID, w.Code)
			}
		})
	}
}
