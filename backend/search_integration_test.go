package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	handlers "familyvault/internal/http/handlers"
)

func TestSearchFilesIntegration(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create HTTP server
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Step 1: Open a session
	resp, err := http.Post(server.URL+"/session/open", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var sessionResp struct {
		SessionID string    `json:"session_id"`
		Expires   time.Time `json:"expires"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}

	sessionID := sessionResp.SessionID
	t.Logf("Created session: %s", sessionID)

	// Step 2: Upload test files with different types
	testFiles := []struct {
		filename string
		content  string
	}{
		{"report.pdf", "This is a PDF report content"},
		{"photo.jpg", "This is a JPEG image content"},
		{"document.txt", "This is a text document content"},
		{"backup.zip", "This is a ZIP archive content"},
		{"presentation.pptx", "This is a PowerPoint presentation content"},
	}

	for _, tf := range testFiles {
		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add file field
		fileWriter, err := writer.CreateFormFile("file", tf.filename)
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(tf.content)); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}
		writer.Close()

		// Create upload request
		req, err := http.NewRequest("POST", server.URL+"/upload", &buf)
		if err != nil {
			t.Fatalf("Failed to create upload request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Session-ID", sessionID)

		// Send upload request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload file %s: %v", tf.filename, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Upload failed for %s: status %d", tf.filename, resp.StatusCode)
		}
		t.Logf("Uploaded file: %s", tf.filename)
	}

	// Step 3: Test various search queries
	searchTests := []struct {
		name          string
		queryParams   map[string]string
		expectedCount int
		expectedFiles []string
	}{
		{
			name:          "search all files",
			queryParams:   map[string]string{},
			expectedCount: 5,
			expectedFiles: []string{"report.pdf", "photo.jpg", "document.txt", "backup.zip", "presentation.pptx"},
		},
		{
			name:          "search by name substring",
			queryParams:   map[string]string{"name": "report"},
			expectedCount: 1,
			expectedFiles: []string{"report.pdf"},
		},
		{
			name:          "search by file type - pdf",
			queryParams:   map[string]string{"type": "pdf"},
			expectedCount: 1,
			expectedFiles: []string{"report.pdf"},
		},
		{
			name:          "search by file type - jpg",
			queryParams:   map[string]string{"type": "jpg"},
			expectedCount: 1,
			expectedFiles: []string{"photo.jpg"},
		},
		{
			name:          "search by name pattern",
			queryParams:   map[string]string{"name": "photo"},
			expectedCount: 1,
			expectedFiles: []string{"photo.jpg"},
		},
		{
			name:          "search with no matches",
			queryParams:   map[string]string{"name": "nonexistent"},
			expectedCount: 0,
			expectedFiles: []string{},
		},
	}

	for _, st := range searchTests {
		t.Run(st.name, func(t *testing.T) {
			// Build URL with query parameters
			u, _ := url.Parse(server.URL + "/search-files")
			q := u.Query()
			for key, value := range st.queryParams {
				q.Set(key, value)
			}
			u.RawQuery = q.Encode()

			// Create search request
			req, err := http.NewRequest("GET", u.String(), nil)
			if err != nil {
				t.Fatalf("Failed to create search request: %v", err)
			}
			req.Header.Set("X-Session-ID", sessionID)

			// Send search request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to search files: %v", err)
			}
			defer resp.Body.Close()

			if st.expectedCount == 0 {
				// Expect 404 for no matches
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("Expected status 404 for no matches, got %d", resp.StatusCode)
				}
				return
			}

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Search failed: status %d", resp.StatusCode)
			}

			// Parse response
			var results []handlers.FileSearchResult
			if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
				t.Fatalf("Failed to decode search response: %v", err)
			}

			if len(results) != st.expectedCount {
				t.Errorf("Expected %d results, got %d", st.expectedCount, len(results))
			}

			// Verify expected files are present
			foundFiles := make(map[string]bool)
			for _, result := range results {
				foundFiles[result.Name] = true

				// Verify result structure
				if result.Name == "" {
					t.Error("Result missing name")
				}
				if result.Size <= 0 {
					t.Error("Result missing or invalid size")
				}
				if result.UploadTime.IsZero() {
					t.Error("Result missing upload time")
				}
				if result.Type == "" {
					t.Error("Result missing type")
				}

				t.Logf("Found file: %s (size: %d, type: %s, uploaded: %s)",
					result.Name, result.Size, result.Type, result.UploadTime.Format(time.RFC3339))
			}

			for _, expectedFile := range st.expectedFiles {
				if !foundFiles[expectedFile] {
					t.Errorf("Expected file %s not found in results", expectedFile)
				}
			}
		})
	}

	t.Log("Integration test passed: search-files endpoint works correctly with various filters")
}

func TestSearchFilesIntegrationDateFilter(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create HTTP server
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Step 1: Open a session
	resp, err := http.Post(server.URL+"/session/open", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to open session: %v", err)
	}
	defer resp.Body.Close()

	var sessionResp struct {
		SessionID string    `json:"session_id"`
		Expires   time.Time `json:"expires"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		t.Fatalf("Failed to decode session response: %v", err)
	}

	sessionID := sessionResp.SessionID

	// Step 2: Upload files with some delay to test date filtering
	testFiles := []string{"early.txt", "middle.txt", "late.txt"}
	uploadTimes := make([]time.Time, len(testFiles))

	for i, filename := range testFiles {
		// Record upload time
		uploadTimes[i] = time.Now()

		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(fmt.Sprintf("Content of %s", filename))); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}
		writer.Close()

		// Upload file
		req, err := http.NewRequest("POST", server.URL+"/upload", &buf)
		if err != nil {
			t.Fatalf("Failed to create upload request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload file %s: %v", filename, err)
		}
		resp.Body.Close()

		// Small delay between uploads
		time.Sleep(100 * time.Millisecond)
	}

	// Step 3: Test date range filtering - search for all files from a past date
	dateFrom := uploadTimes[0].Add(-1 * time.Hour).Format(time.RFC3339)
	dateTo := time.Now().Add(1 * time.Hour).Format(time.RFC3339)

	u, _ := url.Parse(server.URL + "/search-files")
	q := u.Query()
	q.Set("date_from", dateFrom)
	q.Set("date_to", dateTo)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		t.Fatalf("Failed to create search request: %v", err)
	}
	req.Header.Set("X-Session-ID", sessionID)

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to search files: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Search failed: status %d", resp.StatusCode)
	}

	var results []handlers.FileSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatalf("Failed to decode search response: %v", err)
	}

	// Should find all three files
	if len(results) != 3 {
		t.Errorf("Expected 3 results for broad date range, got %d", len(results))
	}

	t.Log("Date filtering integration test passed")
}
