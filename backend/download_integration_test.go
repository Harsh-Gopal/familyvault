package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	handlers "familyvault/internal/http/handlers"
)

func TestDownloadIntegration(t *testing.T) {
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
		filename    string
		content     string
		contentType string
		tags        map[string]string
	}{
		{
			filename:    "document.txt",
			content:     "This is a text document for download testing",
			contentType: "text/plain; charset=utf-8",
			tags:        map[string]string{"category": "document", "type": "text"},
		},
		{
			filename:    "image.jpg",
			content:     "This is fake JPEG image content for testing",
			contentType: "image/jpeg",
			tags:        map[string]string{"category": "image", "format": "jpeg"},
		},
		{
			filename:    "report.pdf",
			content:     "This is fake PDF report content for testing",
			contentType: "application/pdf",
			tags:        map[string]string{"category": "document", "type": "report"},
		},
		{
			filename:    "data.json",
			content:     `{"test": "data", "number": 42}`,
			contentType: "application/json",
			tags:        nil,
		},
	}

	var uploadedFiles []handlers.UploadFileResponse

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

		// Add tags field if provided
		if tf.tags != nil {
			tagsJSON, err := json.Marshal(tf.tags)
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
			t.Fatalf("Failed to upload file %s: %v", tf.filename, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Upload failed for %s: status %d, body: %s", tf.filename, resp.StatusCode, string(body))
		}

		// Parse upload response
		var uploadResp handlers.UploadFileResponse
		if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
			t.Fatalf("Failed to decode upload response: %v", err)
		}

		uploadedFiles = append(uploadedFiles, uploadResp)
		t.Logf("Uploaded file: %s", uploadResp.Name)
	}

	// Step 3: Test downloading each uploaded file
	for i, uploadedFile := range uploadedFiles {
		expectedContent := testFiles[i].content
		expectedContentType := testFiles[i].contentType

		t.Run("download_"+uploadedFile.Name, func(t *testing.T) {
			// Create download request
			req, err := http.NewRequest("GET", server.URL+"/download?filename="+uploadedFile.Name, nil)
			if err != nil {
				t.Fatalf("Failed to create download request: %v", err)
			}
			req.Header.Set("X-Session-ID", sessionID)

			// Send download request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to download file: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Download failed: status %d, body: %s", resp.StatusCode, string(body))
			}

			// Verify Content-Type header
			contentType := resp.Header.Get("Content-Type")
			if contentType != expectedContentType {
				t.Errorf("Expected Content-Type %s, got %s", expectedContentType, contentType)
			}

			// Verify Content-Disposition header
			expectedDisposition := `attachment; filename="` + uploadedFile.Name + `"`
			contentDisposition := resp.Header.Get("Content-Disposition")
			if contentDisposition != expectedDisposition {
				t.Errorf("Expected Content-Disposition %s, got %s", expectedDisposition, contentDisposition)
			}

			// Verify content matches original
			downloadedContent, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read downloaded content: %v", err)
			}

			if string(downloadedContent) != expectedContent {
				t.Errorf("Content mismatch for %s. Expected %q, got %q", uploadedFile.Name, expectedContent, string(downloadedContent))
			}

			t.Logf("Successfully downloaded and verified file: %s", uploadedFile.Name)
		})
	}

	// Step 4: Test download with query parameter authentication
	if len(uploadedFiles) > 0 {
		testFile := uploadedFiles[0]
		t.Run("download_with_query_param_auth", func(t *testing.T) {
			// Create download request with session_id in query parameter
			req, err := http.NewRequest("GET", server.URL+"/download?filename="+testFile.Name+"&session_id="+sessionID, nil)
			if err != nil {
				t.Fatalf("Failed to create download request: %v", err)
			}

			// Send download request (no X-Session-ID header)
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to download file: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for query param auth, got %d", resp.StatusCode)
			}

			t.Log("Query parameter authentication works correctly")
		})
	}

	// Step 5: Test error cases
	t.Run("download_nonexistent_file", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/download?filename=nonexistent.txt", nil)
		if err != nil {
			t.Fatalf("Failed to create download request: %v", err)
		}
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send download request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent file, got %d", resp.StatusCode)
		}
	})

	t.Run("download_with_invalid_session", func(t *testing.T) {
		if len(uploadedFiles) > 0 {
			req, err := http.NewRequest("GET", server.URL+"/download?filename="+uploadedFiles[0].Name, nil)
			if err != nil {
				t.Fatalf("Failed to create download request: %v", err)
			}
			req.Header.Set("X-Session-ID", "invalid-session-id")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send download request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected status 401 for invalid session, got %d", resp.StatusCode)
			}
		}
	})

	t.Run("download_with_unsafe_filename", func(t *testing.T) {
		req, err := http.NewRequest("GET", server.URL+"/download?filename=../../../etc/passwd", nil)
		if err != nil {
			t.Fatalf("Failed to create download request: %v", err)
		}
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send download request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400 for unsafe filename, got %d", resp.StatusCode)
		}
	})

	t.Log("Integration test passed: download endpoint works correctly with various scenarios")
}

func TestDownloadIntegrationLargeFile(t *testing.T) {
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

	// Upload a larger file to test streaming
	largeContent := strings.Repeat("This is a line of test data for streaming download test.\n", 1000) // ~57KB

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

	// Upload the large file
	req, err := http.NewRequest("POST", server.URL+"/upload-file", &buf)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Session-ID", sessionID)

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload large file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Large file upload failed: status %d", resp.StatusCode)
	}

	// Download the large file
	req, err = http.NewRequest("GET", server.URL+"/download?filename=large.txt", nil)
	if err != nil {
		t.Fatalf("Failed to create download request: %v", err)
	}
	req.Header.Set("X-Session-ID", sessionID)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to download large file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Large file download failed: status %d", resp.StatusCode)
	}

	// Verify the downloaded content matches
	downloadedContent, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	if string(downloadedContent) != largeContent {
		t.Errorf("Large file content mismatch. Expected length %d, got %d", len(largeContent), len(downloadedContent))
	}

	t.Log("Large file streaming download test passed")
}
