package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/manifest"
	"familyvault/internal/core/session"
	"familyvault/internal/core/upload"
)

func TestUpdateMetadataHandler(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Create session directory
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create and upload test files
	testFiles := map[string]string{
		"document.txt": "This is a text document",
		"image.jpg":    "This is fake JPEG content",
	}

	for filename, content := range testFiles {
		// Encrypt and save file
		filePath := filepath.Join(sessionDir, filename)
		if err := upload.EncryptAndSave(newTestFile(content), filePath); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}

		// Add to manifest
		manifest.Add(manifest.FileRecord{
			SessionID:  testSession.ID,
			Filename:   filename,
			UploadedAt: time.Now(),
			Tags:       map[string]string{"original": "true"},
		})
	}

	tests := []struct {
		name           string
		sessionID      string
		requestBody    UpdateMetadataRequest
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name:      "update file metadata",
			sessionID: testSession.ID,
			requestBody: UpdateMetadataRequest{
				FileID: "document.txt",
				Metadata: map[string]interface{}{
					"description": "Updated document description",
					"category":    "documents",
					"priority":    "high",
				},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:      "update session metadata",
			sessionID: testSession.ID,
			requestBody: UpdateMetadataRequest{
				Metadata: map[string]interface{}{
					"session_name": "Test Session",
					"purpose":      "Testing metadata updates",
					"created_by":   "test_user",
				},
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name:      "update nonexistent file",
			sessionID: testSession.ID,
			requestBody: UpdateMetadataRequest{
				FileID: "nonexistent.txt",
				Metadata: map[string]interface{}{
					"description": "This should fail",
				},
			},
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
		},
		{
			name:      "invalid session",
			sessionID: "invalid-session-id",
			requestBody: UpdateMetadataRequest{
				FileID: "document.txt",
				Metadata: map[string]interface{}{
					"description": "This should fail",
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectSuccess:  false,
		},
		{
			name:      "missing metadata",
			sessionID: testSession.ID,
			requestBody: UpdateMetadataRequest{
				FileID: "document.txt",
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name:      "empty metadata",
			sessionID: testSession.ID,
			requestBody: UpdateMetadataRequest{
				FileID:   "document.txt",
				Metadata: map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			requestBody, err := json.Marshal(tt.requestBody)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest("PATCH", "/update-metadata", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.sessionID != "" {
				req.Header.Set("X-Session-ID", tt.sessionID)
			}

			w := httptest.NewRecorder()
			updateMetadataHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectSuccess && w.Code == http.StatusOK {
				// Parse response
				var response UpdateMetadataResponse
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if !response.Success {
					t.Error("Expected success to be true")
				}

				if response.UpdatedAt.IsZero() {
					t.Error("Expected UpdatedAt to be set")
				}

				// Verify metadata was actually updated
				if tt.requestBody.FileID != "" {
					// Check file metadata
					records := manifest.List()
					found := false
					for _, record := range records {
						if record.SessionID == testSession.ID && record.Filename == tt.requestBody.FileID {
							found = true
							for key, expectedValue := range tt.requestBody.Metadata {
								if actualValue, exists := record.Tags[key]; !exists {
									t.Errorf("Expected tag %s to exist", key)
								} else if actualValue != expectedValue {
									t.Errorf("Expected tag %s=%v, got %s", key, expectedValue, actualValue)
								}
							}
							break
						}
					}
					if !found {
						t.Error("File record not found in manifest")
					}
				} else {
					// Check session metadata
					sessionMeta, exists := manifest.GetSessionMetadata(testSession.ID)
					if !exists {
						t.Error("Session metadata not found")
					} else {
						for key, expectedValue := range tt.requestBody.Metadata {
							if actualValue, exists := sessionMeta.Metadata[key]; !exists {
								t.Errorf("Expected session metadata %s to exist", key)
							} else if actualValue != expectedValue {
								t.Errorf("Expected session metadata %s=%v, got %v", key, expectedValue, actualValue)
							}
						}
					}
				}
			}
		})
	}
}

func TestUpdateMetadataHandlerWithQueryParamAuth(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)
	session.SetDrivePath(tempDir)
	manifest.Clear()

	// Create test session
	testSession, err := session.Open(time.Hour)
	if err != nil {
		t.Fatalf("Failed to create test session: %v", err)
	}

	// Create session directory and test file
	sessionDir := filepath.Join(tempDir, "uploads", testSession.ID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	filename := "test.txt"
	content := "Test content"
	filePath := filepath.Join(sessionDir, filename)
	if err := upload.EncryptAndSave(newTestFile(content), filePath); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add to manifest
	manifest.Add(manifest.FileRecord{
		SessionID:  testSession.ID,
		Filename:   filename,
		UploadedAt: time.Now(),
		Tags:       map[string]string{},
	})

	// Test with session_id query parameter
	requestBody := UpdateMetadataRequest{
		FileID: filename,
		Metadata: map[string]interface{}{
			"description": "Updated via query param auth",
		},
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest("PATCH", "/update-metadata?session_id="+testSession.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	updateMetadataHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSanitizeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "basic sanitization",
			input: map[string]interface{}{
				"description": "A normal description",
				"category":    "documents",
			},
			expected: map[string]interface{}{
				"description": "A normal description",
				"category":    "documents",
			},
		},
		{
			name: "HTML escaping",
			input: map[string]interface{}{
				"description": "<script>alert('xss')</script>",
				"title":       "Title with <b>bold</b> text",
			},
			expected: map[string]interface{}{
				"description": "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
				"title":       "Title with &lt;b&gt;bold&lt;/b&gt; text",
			},
		},
		{
			name: "whitespace normalization",
			input: map[string]interface{}{
				"description": "  Multiple   spaces   normalized  ",
				"title":       "\t\nTabs and newlines\r\n",
			},
			expected: map[string]interface{}{
				"description": "Multiple spaces normalized",
				"title":       "Tabs and newlines",
			},
		},
		{
			name: "empty values removed",
			input: map[string]interface{}{
				"description": "Valid description",
				"empty":       "",
				"whitespace":  "   ",
				"category":    "documents",
			},
			expected: map[string]interface{}{
				"description": "Valid description",
				"category":    "documents",
			},
		},
		{
			name: "nested objects",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"author": "John <script>alert('xss')</script> Doe",
					"tags":   []interface{}{"tag1", "<script>", "tag2"},
				},
			},
			expected: map[string]interface{}{
				"metadata": map[string]interface{}{
					"author": "John &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt; Doe",
					"tags":   []interface{}{"tag1", "&lt;script&gt;", "tag2"},
				},
			},
		},
		{
			name: "primitive types preserved",
			input: map[string]interface{}{
				"count":  42,
				"active": true,
				"rating": 4.5,
				"title":  "Document Title",
			},
			expected: map[string]interface{}{
				"count":  42,
				"active": true,
				"rating": 4.5,
				"title":  "Document Title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeMetadata(tt.input)

			// Compare the results
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d fields, got %d", len(tt.expected), len(result))
			}

			for key, expectedValue := range tt.expected {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("Expected key %s to exist", key)
					continue
				}

				// Handle nested maps
				if expectedMap, ok := expectedValue.(map[string]interface{}); ok {
					actualMap, ok := actualValue.(map[string]interface{})
					if !ok {
						t.Errorf("Expected %s to be a map, got %T", key, actualValue)
						continue
					}
					for nestedKey, nestedExpected := range expectedMap {
						if nestedActual, exists := actualMap[nestedKey]; !exists {
							t.Errorf("Expected nested key %s.%s to exist", key, nestedKey)
						} else if !compareValues(nestedActual, nestedExpected) {
							t.Errorf("Expected %s.%s=%v, got %v", key, nestedKey, nestedExpected, nestedActual)
						}
					}
				} else if !compareValues(actualValue, expectedValue) {
					t.Errorf("Expected %s=%v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// compareValues compares two values, handling slices properly
func compareValues(actual, expected interface{}) bool {
	// Handle slice comparison
	if expectedSlice, ok := expected.([]interface{}); ok {
		actualSlice, ok := actual.([]interface{})
		if !ok {
			return false
		}
		if len(actualSlice) != len(expectedSlice) {
			return false
		}
		for i, expectedItem := range expectedSlice {
			if !compareValues(actualSlice[i], expectedItem) {
				return false
			}
		}
		return true
	}
	// For non-slice types, use direct comparison
	return actual == expected
}

func TestSanitizeMetadataString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"  whitespace  ", "whitespace"},
		{"multiple   spaces", "multiple spaces"},
		{"", ""},
		{"   ", ""},
		{"text\x00with\x00nulls", "textwithnulls"},
		{"line\rwith\rcarriage\rreturns", "linewithcarriagereturns"},
		{strings.Repeat("a", 1500), strings.Repeat("a", 1000)}, // Truncation test
	}

	for _, tt := range tests {
		t.Run("sanitize_"+tt.input, func(t *testing.T) {
			result := sanitizeMetadataString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
