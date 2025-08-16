package main

import (
	"bytes"
	"encoding/json"
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

func TestUpdateMetadataIntegration(t *testing.T) {
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
	testFiles := []struct {
		filename string
		content  string
		tags     map[string]string
	}{
		{
			filename: "document.txt",
			content:  "This is a text document for metadata testing",
			tags:     map[string]string{"category": "document", "type": "text"},
		},
		{
			filename: "image.jpg",
			content:  "This is fake JPEG image content",
			tags:     map[string]string{"category": "image", "format": "jpeg"},
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

		// Add tags field
		tagsJSON, err := json.Marshal(tf.tags)
		if err != nil {
			t.Fatalf("Failed to marshal tags: %v", err)
		}
		if err := writer.WriteField("tags", string(tagsJSON)); err != nil {
			t.Fatalf("Failed to write tags field: %v", err)
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
			t.Fatalf("Upload failed for %s: status %d", tf.filename, resp.StatusCode)
		}

		// Parse upload response
		var uploadResp handlers.UploadFileResponse
		if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
			t.Fatalf("Failed to decode upload response: %v", err)
		}

		uploadedFiles = append(uploadedFiles, uploadResp)
		t.Logf("Uploaded file: %s", uploadResp.Name)
	}

	// Step 3: Test updating file metadata
	if len(uploadedFiles) > 0 {
		testFile := uploadedFiles[0]
		t.Run("update_file_metadata", func(t *testing.T) {
			updateReq := handlers.UpdateMetadataRequest{
				FileID: testFile.Name,
				Metadata: map[string]interface{}{
					"description": "Updated description for the document",
					"author":      "Test Author",
					"version":     "1.1",
					"priority":    "high",
				},
			}

			reqBody, err := json.Marshal(updateReq)
			if err != nil {
				t.Fatalf("Failed to marshal update request: %v", err)
			}

			req, err := http.NewRequest("PATCH", server.URL+"/update-metadata", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to create update request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Session-ID", sessionID)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to update metadata: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Update metadata failed: status %d", resp.StatusCode)
			}

			// Parse response
			var updateResp handlers.UpdateMetadataResponse
			if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
				t.Fatalf("Failed to decode update response: %v", err)
			}

			if !updateResp.Success {
				t.Error("Expected success to be true")
			}

			if updateResp.FileID != testFile.Name {
				t.Errorf("Expected FileID %s, got %s", testFile.Name, updateResp.FileID)
			}

			// Verify metadata was updated
			for key, expectedValue := range updateReq.Metadata {
				if actualValue, exists := updateResp.Metadata[key]; !exists {
					t.Errorf("Expected metadata key %s to exist", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected metadata %s=%v, got %v", key, expectedValue, actualValue)
				}
			}

			t.Log("File metadata updated successfully")
		})
	}

	// Step 4: Test updating session metadata
	t.Run("update_session_metadata", func(t *testing.T) {
		updateReq := handlers.UpdateMetadataRequest{
			Metadata: map[string]interface{}{
				"session_name": "Integration Test Session",
				"purpose":      "Testing metadata update functionality",
				"created_by":   "integration_test",
				"environment":  "test",
			},
		}

		reqBody, err := json.Marshal(updateReq)
		if err != nil {
			t.Fatalf("Failed to marshal update request: %v", err)
		}

		req, err := http.NewRequest("PATCH", server.URL+"/update-metadata", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create update request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to update session metadata: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Update session metadata failed: status %d", resp.StatusCode)
		}

		// Parse response
		var updateResp handlers.UpdateMetadataResponse
		if err := json.NewDecoder(resp.Body).Decode(&updateResp); err != nil {
			t.Fatalf("Failed to decode update response: %v", err)
		}

		if !updateResp.Success {
			t.Error("Expected success to be true")
		}

		if updateResp.FileID != "" {
			t.Error("Expected FileID to be empty for session metadata update")
		}

		// Verify metadata was updated
		for key, expectedValue := range updateReq.Metadata {
			if actualValue, exists := updateResp.Metadata[key]; !exists {
				t.Errorf("Expected session metadata key %s to exist", key)
			} else if actualValue != expectedValue {
				t.Errorf("Expected session metadata %s=%v, got %v", key, expectedValue, actualValue)
			}
		}

		t.Log("Session metadata updated successfully")
	})

	// Step 5: Test updating metadata with query parameter authentication
	if len(uploadedFiles) > 1 {
		testFile := uploadedFiles[1]
		t.Run("update_with_query_param_auth", func(t *testing.T) {
			updateReq := handlers.UpdateMetadataRequest{
				FileID: testFile.Name,
				Metadata: map[string]interface{}{
					"description": "Updated via query param authentication",
					"auth_method": "query_param",
				},
			}

			reqBody, err := json.Marshal(updateReq)
			if err != nil {
				t.Fatalf("Failed to marshal update request: %v", err)
			}

			req, err := http.NewRequest("PATCH", server.URL+"/update-metadata?session_id="+sessionID, bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to create update request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to update metadata: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for query param auth, got %d", resp.StatusCode)
			}

			t.Log("Query parameter authentication works correctly")
		})
	}

	// Step 6: Test error cases
	t.Run("update_nonexistent_file", func(t *testing.T) {
		updateReq := handlers.UpdateMetadataRequest{
			FileID: "nonexistent.txt",
			Metadata: map[string]interface{}{
				"description": "This should fail",
			},
		}

		reqBody, err := json.Marshal(updateReq)
		if err != nil {
			t.Fatalf("Failed to marshal update request: %v", err)
		}

		req, err := http.NewRequest("PATCH", server.URL+"/update-metadata", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create update request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", sessionID)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send update request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent file, got %d", resp.StatusCode)
		}
	})

	t.Run("update_with_invalid_session", func(t *testing.T) {
		updateReq := handlers.UpdateMetadataRequest{
			Metadata: map[string]interface{}{
				"description": "This should fail",
			},
		}

		reqBody, err := json.Marshal(updateReq)
		if err != nil {
			t.Fatalf("Failed to marshal update request: %v", err)
		}

		req, err := http.NewRequest("PATCH", server.URL+"/update-metadata", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create update request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Session-ID", "invalid-session-id")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send update request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for invalid session, got %d", resp.StatusCode)
		}
	})

	t.Log("Integration test passed: update-metadata endpoint works correctly with various scenarios")
}
