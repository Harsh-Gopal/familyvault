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

func TestSessionFilesHandler(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with files
	sessionID := "test-session-files"
	sessionPath := filepath.Join(tempDir, "uploads", sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create test files with different extensions and sizes
	testFiles := []struct {
		name    string
		content string
		size    int64
	}{
		{"document.txt", "Hello world", 11},
		{"image.jpg", strings.Repeat("x", 1024), 1024},
		{"data.csv", "col1,col2\nval1,val2", 18},
		{"script.py", "print('hello')", 14},
		{"large.bin", strings.Repeat("y", 5000), 5000},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(sessionPath, tf.name)
		err := os.WriteFile(filePath, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}

		// Set specific modification time for testing
		modTime := time.Now().Add(-time.Duration(len(tf.name)) * time.Minute)
		os.Chtimes(filePath, modTime, modTime)
	}

	tests := []struct {
		name           string
		sessionID      string
		queryParams    string
		expectedStatus int
		expectedFiles  int
		checkResponse  func(*testing.T, *FilesResponse)
	}{
		{
			name:           "basic file listing",
			sessionID:      sessionID,
			queryParams:    "",
			expectedStatus: http.StatusOK,
			expectedFiles:  5,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if resp.SessionID != sessionID {
					t.Errorf("Expected session_id %s, got %s", sessionID, resp.SessionID)
				}
				if resp.TotalFiles != 5 {
					t.Errorf("Expected 5 total files, got %d", resp.TotalFiles)
				}
				if len(resp.Files) != 5 {
					t.Errorf("Expected 5 files in response, got %d", len(resp.Files))
				}
			},
		},
		{
			name:           "limit parameter",
			sessionID:      sessionID,
			queryParams:    "limit=3",
			expectedStatus: http.StatusOK,
			expectedFiles:  3,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) != 3 {
					t.Errorf("Expected 3 files with limit=3, got %d", len(resp.Files))
				}
				if resp.Limit != 3 {
					t.Errorf("Expected limit=3 in response, got %d", resp.Limit)
				}
			},
		},
		{
			name:           "offset parameter",
			sessionID:      sessionID,
			queryParams:    "offset=2&limit=2",
			expectedStatus: http.StatusOK,
			expectedFiles:  2,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) != 2 {
					t.Errorf("Expected 2 files with offset=2&limit=2, got %d", len(resp.Files))
				}
				if resp.Offset != 2 {
					t.Errorf("Expected offset=2 in response, got %d", resp.Offset)
				}
			},
		},
		{
			name:           "extension filter",
			sessionID:      sessionID,
			queryParams:    "ext=txt",
			expectedStatus: http.StatusOK,
			expectedFiles:  1,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) != 1 {
					t.Errorf("Expected 1 txt file, got %d", len(resp.Files))
				}
				if len(resp.Files) > 0 && resp.Files[0].Name != "document.txt" {
					t.Errorf("Expected document.txt, got %s", resp.Files[0].Name)
				}
			},
		},
		{
			name:           "size filter",
			sessionID:      sessionID,
			queryParams:    "min_size=1000",
			expectedStatus: http.StatusOK,
			expectedFiles:  2, // image.jpg (1024) and large.bin (5000)
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) != 2 {
					t.Errorf("Expected 2 files with min_size=1000, got %d", len(resp.Files))
				}
				for _, file := range resp.Files {
					if file.Size < 1000 {
						t.Errorf("File %s has size %d, expected >= 1000", file.Name, file.Size)
					}
				}
			},
		},
		{
			name:           "size range filter",
			sessionID:      sessionID,
			queryParams:    "min_size=15&max_size=1500",
			expectedStatus: http.StatusOK,
			expectedFiles:  2, // data.csv (18) and image.jpg (1024)
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) != 2 {
					t.Errorf("Expected 2 files in size range, got %d", len(resp.Files))
				}
				for _, file := range resp.Files {
					if file.Size < 15 || file.Size > 1500 {
						t.Errorf("File %s has size %d, expected between 15-1500", file.Name, file.Size)
					}
				}
			},
		},
		{
			name:           "sort by size ascending",
			sessionID:      sessionID,
			queryParams:    "sort_by=size&order=asc",
			expectedStatus: http.StatusOK,
			expectedFiles:  5,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) < 2 {
					return
				}
				for i := 1; i < len(resp.Files); i++ {
					if resp.Files[i-1].Size > resp.Files[i].Size {
						t.Errorf("Files not sorted by size ascending: %d > %d",
							resp.Files[i-1].Size, resp.Files[i].Size)
					}
				}
			},
		},
		{
			name:           "sort by size descending",
			sessionID:      sessionID,
			queryParams:    "sort_by=size&order=desc",
			expectedStatus: http.StatusOK,
			expectedFiles:  5,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) < 2 {
					return
				}
				for i := 1; i < len(resp.Files); i++ {
					if resp.Files[i-1].Size < resp.Files[i].Size {
						t.Errorf("Files not sorted by size descending: %d < %d",
							resp.Files[i-1].Size, resp.Files[i].Size)
					}
				}
			},
		},
		{
			name:           "sort by name",
			sessionID:      sessionID,
			queryParams:    "sort_by=name&order=asc",
			expectedStatus: http.StatusOK,
			expectedFiles:  5,
			checkResponse: func(t *testing.T, resp *FilesResponse) {
				if len(resp.Files) < 2 {
					return
				}
				for i := 1; i < len(resp.Files); i++ {
					if resp.Files[i-1].Name > resp.Files[i].Name {
						t.Errorf("Files not sorted by name ascending: %s > %s",
							resp.Files[i-1].Name, resp.Files[i].Name)
					}
				}
			},
		},
		{
			name:           "invalid session ID",
			sessionID:      "invalid@session",
			queryParams:    "",
			expectedStatus: http.StatusBadRequest,
			expectedFiles:  0,
		},
		{
			name:           "non-existent session",
			sessionID:      "non-existent-session",
			queryParams:    "",
			expectedStatus: http.StatusNotFound,
			expectedFiles:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := fmt.Sprintf("/sessions/%s/files", tt.sessionID)
			if tt.queryParams != "" {
				url += "?" + tt.queryParams
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("X-Session-ID", "test-session")

			// Set up mux router to extract session ID
			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				t.Logf("Response body: %s", w.Body.String())
				return
			}

			if tt.expectedStatus == http.StatusOK {
				var response FilesResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				if len(response.Files) != tt.expectedFiles {
					t.Errorf("Expected %d files, got %d", tt.expectedFiles, len(response.Files))
				}

				// Verify file structure
				for _, file := range response.Files {
					if file.Name == "" {
						t.Error("File name is empty")
					}
					if file.Size < 0 {
						t.Error("File size is negative")
					}
					if file.ModifiedTime == "" {
						t.Error("File modified time is empty")
					}
					if file.Type != "active" && file.Type != "backup" {
						t.Errorf("Invalid file type: %s", file.Type)
					}
				}

				if tt.checkResponse != nil {
					tt.checkResponse(t, &response)
				}
			}
		})
	}
}

func TestSessionFilesHandlerBackup(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create backup session
	sessionID := "backup-session-test"
	backupPath := filepath.Join(tempDir, "backup", "2025-08-12", sessionID)
	err := os.MkdirAll(backupPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create backup directory: %v", err)
	}

	// Create backup files
	backupFiles := []string{"backup1.txt", "backup2.jpg", "backup3.csv"}
	for _, filename := range backupFiles {
		filePath := filepath.Join(backupPath, filename)
		err := os.WriteFile(filePath, []byte("backup content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create backup file %s: %v", filename, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files", sessionID), nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
		return
	}

	var response FilesResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Files) != 3 {
		t.Errorf("Expected 3 backup files, got %d", len(response.Files))
	}

	// Verify all files are marked as backup
	for _, file := range response.Files {
		if file.Type != "backup" {
			t.Errorf("Expected file type 'backup', got '%s'", file.Type)
		}
	}
}

func TestParseFilesQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		expectError bool
		checkParams func(*testing.T, *FilesQueryParams)
	}{
		{
			name:        "default parameters",
			queryString: "",
			expectError: false,
			checkParams: func(t *testing.T, p *FilesQueryParams) {
				if p.Limit != 1000 {
					t.Errorf("Expected default limit 1000, got %d", p.Limit)
				}
				if p.Offset != 0 {
					t.Errorf("Expected default offset 0, got %d", p.Offset)
				}
				if p.SortBy != "name" {
					t.Errorf("Expected default sort_by 'name', got '%s'", p.SortBy)
				}
				if p.Order != "asc" {
					t.Errorf("Expected default order 'asc', got '%s'", p.Order)
				}
			},
		},
		{
			name:        "custom parameters",
			queryString: "limit=500&offset=100&ext=jpg&min_size=1024&max_size=5000&sort_by=size&order=desc",
			expectError: false,
			checkParams: func(t *testing.T, p *FilesQueryParams) {
				if p.Limit != 500 {
					t.Errorf("Expected limit 500, got %d", p.Limit)
				}
				if p.Offset != 100 {
					t.Errorf("Expected offset 100, got %d", p.Offset)
				}
				if p.Ext != "jpg" {
					t.Errorf("Expected ext 'jpg', got '%s'", p.Ext)
				}
				if p.MinSize != 1024 {
					t.Errorf("Expected min_size 1024, got %d", p.MinSize)
				}
				if p.MaxSize != 5000 {
					t.Errorf("Expected max_size 5000, got %d", p.MaxSize)
				}
				if p.SortBy != "size" {
					t.Errorf("Expected sort_by 'size', got '%s'", p.SortBy)
				}
				if p.Order != "desc" {
					t.Errorf("Expected order 'desc', got '%s'", p.Order)
				}
			},
		},
		{
			name:        "extension with dot",
			queryString: "ext=.txt",
			expectError: false,
			checkParams: func(t *testing.T, p *FilesQueryParams) {
				if p.Ext != "txt" {
					t.Errorf("Expected ext 'txt' (dot removed), got '%s'", p.Ext)
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
			name:        "invalid min_size",
			queryString: "min_size=-100",
			expectError: true,
		},
		{
			name:        "invalid size range",
			queryString: "min_size=5000&max_size=1000",
			expectError: true,
		},
		{
			name:        "invalid sort_by",
			queryString: "sort_by=invalid",
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

			params, err := parseFilesQueryParams(req)

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

func TestMatchesFilters(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		size     int64
		params   *FilesQueryParams
		expected bool
	}{
		{
			name:     "no filters",
			filename: "test.txt",
			size:     100,
			params:   &FilesQueryParams{},
			expected: true,
		},
		{
			name:     "extension match",
			filename: "test.txt",
			size:     100,
			params:   &FilesQueryParams{Ext: "txt"},
			expected: true,
		},
		{
			name:     "extension no match",
			filename: "test.jpg",
			size:     100,
			params:   &FilesQueryParams{Ext: "txt"},
			expected: false,
		},
		{
			name:     "size within range",
			filename: "test.txt",
			size:     500,
			params:   &FilesQueryParams{MinSize: 100, MaxSize: 1000},
			expected: true,
		},
		{
			name:     "size below minimum",
			filename: "test.txt",
			size:     50,
			params:   &FilesQueryParams{MinSize: 100},
			expected: false,
		},
		{
			name:     "size above maximum",
			filename: "test.txt",
			size:     1500,
			params:   &FilesQueryParams{MaxSize: 1000},
			expected: false,
		},
		{
			name:     "combined filters match",
			filename: "data.csv",
			size:     800,
			params:   &FilesQueryParams{Ext: "csv", MinSize: 500, MaxSize: 1000},
			expected: true,
		},
		{
			name:     "combined filters no match",
			filename: "data.txt",
			size:     800,
			params:   &FilesQueryParams{Ext: "csv", MinSize: 500, MaxSize: 1000},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesFilters(tt.filename, tt.size, tt.params)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSortFiles(t *testing.T) {
	files := []FileInfo{
		{Name: "c.txt", Size: 300, ModifiedTime: "2025-08-12T10:00:00Z"},
		{Name: "a.txt", Size: 100, ModifiedTime: "2025-08-12T08:00:00Z"},
		{Name: "b.txt", Size: 200, ModifiedTime: "2025-08-12T09:00:00Z"},
	}

	t.Run("sort by name ascending", func(t *testing.T) {
		testFiles := make([]FileInfo, len(files))
		copy(testFiles, files)

		sortFiles(testFiles, "name", "asc")

		expected := []string{"a.txt", "b.txt", "c.txt"}
		for i, file := range testFiles {
			if file.Name != expected[i] {
				t.Errorf("Expected %s at position %d, got %s", expected[i], i, file.Name)
			}
		}
	})

	t.Run("sort by name descending", func(t *testing.T) {
		testFiles := make([]FileInfo, len(files))
		copy(testFiles, files)

		sortFiles(testFiles, "name", "desc")

		expected := []string{"c.txt", "b.txt", "a.txt"}
		for i, file := range testFiles {
			if file.Name != expected[i] {
				t.Errorf("Expected %s at position %d, got %s", expected[i], i, file.Name)
			}
		}
	})

	t.Run("sort by size ascending", func(t *testing.T) {
		testFiles := make([]FileInfo, len(files))
		copy(testFiles, files)

		sortFiles(testFiles, "size", "asc")

		expected := []int64{100, 200, 300}
		for i, file := range testFiles {
			if file.Size != expected[i] {
				t.Errorf("Expected size %d at position %d, got %d", expected[i], i, file.Size)
			}
		}
	})

	t.Run("sort by modified_time ascending", func(t *testing.T) {
		testFiles := make([]FileInfo, len(files))
		copy(testFiles, files)

		sortFiles(testFiles, "modified_time", "asc")

		expected := []string{"a.txt", "b.txt", "c.txt"} // Based on timestamps
		for i, file := range testFiles {
			if file.Name != expected[i] {
				t.Errorf("Expected %s at position %d, got %s", expected[i], i, file.Name)
			}
		}
	})
}

func TestSessionFilesHandlerAuthentication(t *testing.T) {
	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	sessionID := "auth-test-session"

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files", sessionID), nil)
	// No authentication header

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated request, got %d", w.Code)
	}
}

func TestSessionFilesHandlerMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/sessions/test/files", nil)
	req.Header.Set("X-Session-ID", "test-session")

	router := mux.NewRouter()
	router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405 for POST method, got %d", w.Code)
	}
}
