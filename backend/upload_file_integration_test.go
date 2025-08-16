package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	handlers "familyvault/internal/http/handlers"
)

func TestUploadFileIntegration(t *testing.T) {
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

	// Step 2: Test various upload scenarios
	uploadTests := []struct {
		name           string
		filename       string
		content        string
		tags           map[string]string
		expectedStatus int
		expectFile     bool
	}{
		{
			name:           "upload PDF with tags",
			filename:       "report.pdf",
			content:        "This is a PDF report content for testing",
			tags:           map[string]string{"category": "document", "priority": "high", "department": "finance"},
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "upload image without tags",
			filename:       "photo.jpg",
			content:        "This is a JPEG image content for testing",
			tags:           nil,
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "upload text file",
			filename:       "notes.txt",
			content:        "These are some important notes for the project",
			tags:           map[string]string{"type": "notes", "project": "familyvault"},
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "upload duplicate filename",
			filename:       "report.pdf", // Same as first test
			content:        "This is another PDF with the same name",
			tags:           map[string]string{"version": "2"},
			expectedStatus: http.StatusCreated,
			expectFile:     true,
		},
		{
			name:           "upload unsupported file type",
			filename:       "malware.exe",
			content:        "This should be rejected",
			tags:           nil,
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
		{
			name:           "upload empty file",
			filename:       "empty.txt",
			content:        "",
			tags:           nil,
			expectedStatus: http.StatusBadRequest,
			expectFile:     false,
		},
	}

	var uploadedFiles []handlers.UploadFileResponse

	for _, ut := range uploadTests {
		t.Run(ut.name, func(t *testing.T) {
			// Create multipart form
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Add file field
			fileWriter, err := writer.CreateFormFile("file", ut.filename)
			if err != nil {
				t.Fatalf("Failed to create form file: %v", err)
			}
			if _, err := fileWriter.Write([]byte(ut.content)); err != nil {
				t.Fatalf("Failed to write file content: %v", err)
			}

			// Add tags field if provided
			if ut.tags != nil {
				tagsJSON, err := json.Marshal(ut.tags)
				if err != nil {
					t.Fatalf("Failed to marshal tags: %v", err)
				}
				if err := writer.WriteField("tags", string(tagsJSON)); err != nil {
					t.Fatalf("Failed to write tags field: %v", err)
				}
			}

			writer.Close()

			// Create upload request
			req, err := http.NewRequest("POST", server.URL+"/upload-file", &buf)
			if err != nil {
				t.Fatalf("Failed to create upload request: %v", err)
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-Session-ID", sessionID)

			// Send upload request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to upload file: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != ut.expectedStatus {
				body := make([]byte, 1024)
				n, _ := resp.Body.Read(body)
				t.Fatalf("Expected status %d, got %d. Response: %s", ut.expectedStatus, resp.StatusCode, string(body[:n]))
			}

			if ut.expectFile && resp.StatusCode == http.StatusCreated {
				// Parse response
				var uploadResp handlers.UploadFileResponse
				if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
					t.Fatalf("Failed to decode upload response: %v", err)
				}

				// Verify response structure
				if uploadResp.Name == "" {
					t.Error("Response missing name")
				}
				if uploadResp.Size <= 0 {
					t.Error("Response missing or invalid size")
				}
				if uploadResp.UploadTime.IsZero() {
					t.Error("Response missing upload time")
				}
				if uploadResp.Type == "" {
					t.Error("Response missing type")
				}

				// Verify tags
				if ut.tags != nil {
					if uploadResp.Tags == nil {
						t.Error("Expected tags in response")
					} else {
						for key, expectedValue := range ut.tags {
							if actualValue, exists := uploadResp.Tags[key]; !exists || actualValue != expectedValue {
								t.Errorf("Tag mismatch: expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
							}
						}
					}
				}

				uploadedFiles = append(uploadedFiles, uploadResp)
				t.Logf("Uploaded file: %s (size: %d, type: %s)", uploadResp.Name, uploadResp.Size, uploadResp.Type)
			}
		})
	}

	// Step 3: Verify files can be searched
	t.Run("search uploaded files", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/search-files", nil)
		if err != nil {
			t.Fatalf("Failed to create search request: %v", err)
		}
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to search files: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Search failed: status %d", resp.StatusCode)
		}

		var searchResults []handlers.FileSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&searchResults); err != nil {
			t.Fatalf("Failed to decode search response: %v", err)
		}

		// Should find the successfully uploaded files
		expectedCount := len(uploadedFiles)
		if len(searchResults) != expectedCount {
			t.Errorf("Expected %d files in search results, got %d", expectedCount, len(searchResults))
		}

		// Verify each uploaded file appears in search results
		searchMap := make(map[string]handlers.FileSearchResult)
		for _, result := range searchResults {
			searchMap[result.Name] = result
		}

		for _, uploaded := range uploadedFiles {
			if searchResult, found := searchMap[uploaded.Name]; !found {
				t.Errorf("Uploaded file %s not found in search results", uploaded.Name)
			} else {
				// Verify metadata matches
				if searchResult.Size != uploaded.Size {
					t.Errorf("Size mismatch for %s: uploaded=%d, search=%d", uploaded.Name, uploaded.Size, searchResult.Size)
				}
				if searchResult.Type != uploaded.Type {
					t.Errorf("Type mismatch for %s: uploaded=%s, search=%s", uploaded.Name, uploaded.Type, searchResult.Type)
				}
			}
		}
	})

	// Step 4: Test file download
	if len(uploadedFiles) > 0 {
		t.Run("download uploaded file", func(t *testing.T) {
			testFile := uploadedFiles[0]

			req, err := http.NewRequest("GET", server.URL+"/download?filename="+testFile.Name, nil)
			if err != nil {
				t.Fatalf("Failed to create download request: %v", err)
			}
			req.Header.Set("X-Session-ID", sessionID)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to download file: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Download failed: status %d", resp.StatusCode)
			}

			// Verify content-disposition header
			expectedDisposition := fmt.Sprintf("attachment; filename=\"%s\"", testFile.Name)
			if resp.Header.Get("Content-Disposition") != expectedDisposition {
				t.Errorf("Expected Content-Disposition %s, got %s", expectedDisposition, resp.Header.Get("Content-Disposition"))
			}

			t.Logf("Successfully downloaded file: %s", testFile.Name)
		})
	}

	t.Log("Integration test passed: upload-file endpoint works correctly with various scenarios")
}

func TestUploadFileIntegrationLargeFile(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Set a smaller max file size for testing
	os.Setenv("FAMILYVAULT_MAX_FILE_SIZE_MB", "1")
	defer os.Unsetenv("FAMILYVAULT_MAX_FILE_SIZE_MB")

	// Create HTTP server
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Open a session
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

	// Test uploading a file that exceeds the limit
	t.Run("upload file too large", func(t *testing.T) {
		// Create a file larger than 1MB
		largeContent := strings.Repeat("A", 2*1024*1024) // 2MB

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		fileWriter, err := writer.CreateFormFile("file", "large.txt")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(largeContent)); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", server.URL+"/upload-file", &buf)
		if err != nil {
			t.Fatalf("Failed to create upload request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status 413 for large file, got %d", resp.StatusCode)
		}

		t.Log("Large file correctly rejected")
	})

	// Test uploading a file within the limit
	t.Run("upload file within limit", func(t *testing.T) {
		// Create a file smaller than 1MB
		smallContent := strings.Repeat("B", 500*1024) // 500KB

		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		fileWriter, err := writer.CreateFormFile("file", "small.txt")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(smallContent)); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", server.URL+"/upload-file", &buf)
		if err != nil {
			t.Fatalf("Failed to create upload request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected status 201 for small file, got %d", resp.StatusCode)
		}

		t.Log("Small file correctly accepted")
	})
}

func TestUploadFileIntegrationInvalidSession(t *testing.T) {
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

	// Test with invalid session
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fileWriter, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	if _, err := fileWriter.Write([]byte("test content")); err != nil {
		t.Fatalf("Failed to write file content: %v", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", server.URL+"/upload-file", &buf)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Session-ID", "invalid-session-id")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid session, got %d", resp.StatusCode)
	}

	var errorResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if !strings.Contains(errorResp["error"], "invalid or expired session") {
		t.Errorf("Expected session error message, got: %s", errorResp["error"])
	}

	t.Log("Invalid session correctly rejected")
}
