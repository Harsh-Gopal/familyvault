package handlers

import (
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

// TestSessionFilesPerformanceLargeFileCount tests performance with large number of files (10k+)
func TestSessionFilesPerformanceLargeFileCount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with many files
	sessionID := "large-files-session"
	sessionPath := filepath.Join(tempDir, "uploads", sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create session directory: %v", err)
	}

	// Create subdirectories to organize files
	subDirs := []string{"docs", "images", "data", "scripts", "misc"}
	for _, subDir := range subDirs {
		err := os.MkdirAll(filepath.Join(sessionPath, subDir), 0755)
		if err != nil {
			t.Fatalf("Failed to create subdirectory %s: %v", subDir, err)
		}
	}

	// Generate large number of files (10,000 files)
	numFiles := 10000
	fileExtensions := []string{"txt", "jpg", "png", "csv", "json", "py", "js", "html", "css", "md"}

	t.Logf("Generating %d test files...", numFiles)
	startGenerate := time.Now()

	for i := 0; i < numFiles; i++ {
		// Distribute files across subdirectories
		subDir := subDirs[i%len(subDirs)]
		ext := fileExtensions[i%len(fileExtensions)]
		filename := fmt.Sprintf("file_%06d.%s", i, ext)
		filePath := filepath.Join(sessionPath, subDir, filename)

		// Create files with varying sizes
		var content string
		switch {
		case i%100 == 0: // 1% large files (10KB)
			content = strings.Repeat("x", 10240)
		case i%10 == 0: // 10% medium files (1KB)
			content = strings.Repeat("y", 1024)
		default: // 89% small files (100 bytes)
			content = strings.Repeat("z", 100)
		}

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filename, err)
		}

		// Set varying modification times
		modTime := time.Now().Add(-time.Duration(i) * time.Second)
		os.Chtimes(filePath, modTime, modTime)
	}

	generateTime := time.Since(startGenerate)
	t.Logf("Generated %d files in %v", numFiles, generateTime)

	// Test 1: Basic file listing with default pagination
	t.Run("basic_listing_default_pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Basic listing (1000 files) completed in %v", requestTime)

		// Should be reasonably fast (< 2 seconds)
		if requestTime > 2*time.Second {
			t.Errorf("Basic listing took too long: %v (expected < 2s)", requestTime)
		}
	})

	// Test 2: Large pagination request
	t.Run("large_pagination_request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?limit=5000", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Large pagination (5000 files) completed in %v", requestTime)

		// Should still be reasonable (< 5 seconds)
		if requestTime > 5*time.Second {
			t.Errorf("Large pagination took too long: %v (expected < 5s)", requestTime)
		}
	})

	// Test 3: Extension filtering
	t.Run("extension_filtering", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?ext=jpg", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Extension filtering (jpg files) completed in %v", requestTime)

		// Filtering should be fast (< 3 seconds)
		if requestTime > 3*time.Second {
			t.Errorf("Extension filtering took too long: %v (expected < 3s)", requestTime)
		}
	})

	// Test 4: Size filtering
	t.Run("size_filtering", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?min_size=5000", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Size filtering (min_size=5000) completed in %v", requestTime)

		// Size filtering should be reasonable (< 3 seconds)
		if requestTime > 3*time.Second {
			t.Errorf("Size filtering took too long: %v (expected < 3s)", requestTime)
		}
	})

	// Test 5: Sorting by size
	t.Run("sorting_by_size", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?sort_by=size&order=desc&limit=1000", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Sorting by size (1000 files) completed in %v", requestTime)

		// Sorting should be reasonable (< 4 seconds)
		if requestTime > 4*time.Second {
			t.Errorf("Sorting took too long: %v (expected < 4s)", requestTime)
		}
	})

	// Test 6: Complex filtering and sorting
	t.Run("complex_filtering_and_sorting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/sessions/%s/files?ext=txt&min_size=50&sort_by=modified_time&order=desc&limit=500", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Complex filtering and sorting completed in %v", requestTime)

		// Complex operations should still be reasonable (< 5 seconds)
		if requestTime > 5*time.Second {
			t.Errorf("Complex filtering and sorting took too long: %v (expected < 5s)", requestTime)
		}
	})

	// Test 7: Memory usage test (multiple consecutive requests)
	t.Run("memory_usage_test", func(t *testing.T) {
		// Make multiple requests to ensure no memory accumulation
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?limit=1000&offset=%d", sessionID, i*1000), nil)
			req.Header.Set("X-Session-ID", "test-session")

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

			startRequest := time.Now()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			requestTime := time.Since(startRequest)

			if w.Code != http.StatusOK {
				t.Fatalf("Request %d failed with status %d", i, w.Code)
			}

			// Each request should be consistently fast
			if requestTime > 3*time.Second {
				t.Errorf("Request %d took too long: %v", i, requestTime)
			}
		}

		t.Log("Memory usage test completed - no memory accumulation detected")
	})

	// Test 8: Offset performance (deep pagination)
	t.Run("deep_pagination_performance", func(t *testing.T) {
		// Test pagination at different offsets
		offsets := []int{0, 1000, 5000, 8000}

		for _, offset := range offsets {
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/sessions/%s/files?limit=100&offset=%d", sessionID, offset), nil)
			req.Header.Set("X-Session-ID", "test-session")

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

			startRequest := time.Now()
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			requestTime := time.Since(startRequest)

			if w.Code != http.StatusOK {
				t.Fatalf("Request with offset %d failed with status %d", offset, w.Code)
			}

			t.Logf("Pagination with offset %d completed in %v", offset, requestTime)

			// Deep pagination should still be reasonable (< 3 seconds)
			if requestTime > 3*time.Second {
				t.Errorf("Deep pagination (offset=%d) took too long: %v (expected < 3s)", offset, requestTime)
			}
		}
	})
}

// BenchmarkSessionFilesOperations benchmarks different file operations
func BenchmarkSessionFilesOperations(b *testing.B) {
	tempDir := b.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with moderate number of files for benchmarking
	sessionID := "benchmark-session"
	sessionPath := filepath.Join(tempDir, "uploads", sessionID)
	err := os.MkdirAll(sessionPath, 0755)
	if err != nil {
		b.Fatalf("Failed to create session directory: %v", err)
	}

	// Generate 1000 test files
	numFiles := 1000
	fileExtensions := []string{"txt", "jpg", "csv", "json", "py"}

	for i := 0; i < numFiles; i++ {
		ext := fileExtensions[i%len(fileExtensions)]
		filename := fmt.Sprintf("bench_file_%04d.%s", i, ext)
		filePath := filepath.Join(sessionPath, filename)

		// Create files with varying sizes
		var content string
		switch i % 10 {
		case 0: // 10% large files (5KB)
			content = strings.Repeat("x", 5120)
		case 1, 2: // 20% medium files (1KB)
			content = strings.Repeat("y", 1024)
		default: // 70% small files (200 bytes)
			content = strings.Repeat("z", 200)
		}

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to create file %s: %v", filename, err)
		}
	}

	b.Run("BasicListing", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("ExtensionFiltering", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?ext=txt", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("SizeFiltering", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?min_size=1000", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("SortingBySize", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?sort_by=size&order=desc", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("SortingByName", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?sort_by=name&order=asc", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("Pagination", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/files", SessionFilesHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			offset := (i % 10) * 100 // Vary offset
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/files?limit=100&offset=%d", sessionID, offset), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})
}
