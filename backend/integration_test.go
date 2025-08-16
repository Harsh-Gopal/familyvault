package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	handlers "familyvault/internal/http/handlers"
)

func TestDownloadAllIntegration(t *testing.T) {
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

	// Step 2: Upload test files
	testFiles := map[string]string{
		"document1.txt": "This is the first document content.",
		"document2.txt": "This is the second document with more content.",
	}

	for filename, content := range testFiles {
		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add file field
		fileWriter, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := fileWriter.Write([]byte(content)); err != nil {
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
			t.Fatalf("Failed to upload file %s: %v", filename, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Upload failed for %s: status %d", filename, resp.StatusCode)
		}
		t.Logf("Uploaded file: %s", filename)
	}

	// Step 3: Download all files as ZIP
	req, err := http.NewRequest("GET", server.URL+"/download-all", nil)
	if err != nil {
		t.Fatalf("Failed to create download-all request: %v", err)
	}
	req.Header.Set("X-Session-ID", sessionID)

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Failed to download-all: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Download-all failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Verify response headers
	if resp.Header.Get("Content-Type") != "application/zip" {
		t.Errorf("Expected Content-Type application/zip, got %s", resp.Header.Get("Content-Type"))
	}

	expectedFilename := fmt.Sprintf("session_%s.zip", sessionID)
	expectedDisposition := fmt.Sprintf("attachment; filename=\"%s\"", expectedFilename)
	if resp.Header.Get("Content-Disposition") != expectedDisposition {
		t.Errorf("Expected Content-Disposition %s, got %s", expectedDisposition, resp.Header.Get("Content-Disposition"))
	}

	// Step 4: Verify ZIP contents
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read ZIP data: %v", err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("Failed to read ZIP: %v", err)
	}

	if len(zipReader.File) != len(testFiles) {
		t.Errorf("Expected %d files in ZIP, got %d", len(testFiles), len(zipReader.File))
	}

	for _, zipFile := range zipReader.File {
		expectedContent, exists := testFiles[zipFile.Name]
		if !exists {
			t.Errorf("Unexpected file in ZIP: %s", zipFile.Name)
			continue
		}

		rc, err := zipFile.Open()
		if err != nil {
			t.Errorf("Failed to open ZIP file %s: %v", zipFile.Name, err)
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Errorf("Failed to read ZIP file %s: %v", zipFile.Name, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("File %s content mismatch. Expected %q, got %q", zipFile.Name, expectedContent, string(content))
		} else {
			t.Logf("Verified file %s: content matches", zipFile.Name)
		}
	}

	t.Log("Integration test passed: download-all successfully created ZIP with correct decrypted content")
}
