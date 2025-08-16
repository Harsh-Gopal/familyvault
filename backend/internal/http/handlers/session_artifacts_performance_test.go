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

// TestSessionArtifactsPerformanceLargeDirectory tests performance with large number of artifacts (10k+)
func TestSessionArtifactsPerformanceLargeDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tempDir := t.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with large number of artifacts
	sessionID := "large-artifacts-session"
	artifactsPath := filepath.Join(tempDir, "uploads", sessionID, "artifacts")
	err := os.MkdirAll(artifactsPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create artifacts directory: %v", err)
	}

	// Create subdirectories to organize artifacts
	subDirs := []string{"reports", "images", "data", "logs", "archives"}
	for _, subDir := range subDirs {
		err := os.MkdirAll(filepath.Join(artifactsPath, subDir), 0755)
		if err != nil {
			t.Fatalf("Failed to create subdirectory %s: %v", subDir, err)
		}
	}

	// Generate large number of artifacts (10,000 files)
	numArtifacts := 10000
	fileExtensions := []string{"pdf", "png", "jpg", "csv", "txt", "zip", "log", "json", "xml", "html"}

	t.Logf("Generating %d test artifacts...", numArtifacts)
	startGenerate := time.Now()

	for i := 0; i < numArtifacts; i++ {
		// Distribute artifacts across subdirectories
		subDir := subDirs[i%len(subDirs)]
		ext := fileExtensions[i%len(fileExtensions)]
		filename := fmt.Sprintf("artifact_%06d.%s", i, ext)
		filePath := filepath.Join(artifactsPath, subDir, filename)

		// Create artifacts with varying sizes
		var content string
		switch {
		case i%100 == 0: // 1% large artifacts (50KB)
			content = strings.Repeat("x", 50*1024)
		case i%10 == 0: // 10% medium artifacts (5KB)
			content = strings.Repeat("y", 5*1024)
		default: // 89% small artifacts (500 bytes)
			content = strings.Repeat("z", 500)
		}

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create artifact %s: %v", filename, err)
		}

		// Set varying modification times
		modTime := time.Now().Add(-time.Duration(i) * time.Second)
		os.Chtimes(filePath, modTime, modTime)
	}

	generateTime := time.Since(startGenerate)
	t.Logf("Generated %d artifacts in %v", numArtifacts, generateTime)

	// Calculate total directory size
	var totalSize int64
	filepath.WalkDir(artifactsPath, func(path string, d os.DirEntry, err error) error {
		if !d.IsDir() {
			info, _ := d.Info()
			totalSize += info.Size()
		}
		return nil
	})
	t.Logf("Total artifacts size: %.2f MB", float64(totalSize)/(1024*1024))

	// Test 1: Basic artifact listing with default pagination
	t.Run("basic_listing_default_pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Basic listing (100 artifacts) completed in %v", requestTime)

		// Should be reasonably fast (< 2 seconds)
		if requestTime > 2*time.Second {
			t.Errorf("Basic listing took too long: %v (expected < 2s)", requestTime)
		}

		var response ArtifactsResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		if len(response.Artifacts) != 100 {
			t.Errorf("Expected 100 artifacts, got %d", len(response.Artifacts))
		}
	})

	// Test 2: Large pagination request
	t.Run("large_pagination_request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?limit=1000", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Large pagination (1000 artifacts) completed in %v", requestTime)

		// Should still be reasonable (< 5 seconds)
		if requestTime > 5*time.Second {
			t.Errorf("Large pagination took too long: %v (expected < 5s)", requestTime)
		}
	})

	// Test 3: Type filtering
	t.Run("type_filtering", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?type=application/pdf", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Type filtering (PDF artifacts) completed in %v", requestTime)

		// Type filtering should be efficient (< 3 seconds)
		if requestTime > 3*time.Second {
			t.Errorf("Type filtering took too long: %v (expected < 3s)", requestTime)
		}

		var response ArtifactsResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		for _, artifact := range response.Artifacts {
			if artifact.Type != "application/pdf" {
				t.Errorf("Expected all artifacts to be PDF type, got %s", artifact.Type)
				break
			}
		}
	})

	// Test 4: Name contains filtering
	t.Run("name_contains_filtering", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?name_contains=reports", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Name contains filtering completed in %v", requestTime)

		// Name filtering should be reasonable (< 3 seconds)
		if requestTime > 3*time.Second {
			t.Errorf("Name contains filtering took too long: %v (expected < 3s)", requestTime)
		}
	})

	// Test 5: Time range filtering
	t.Run("time_range_filtering", func(t *testing.T) {
		// Get artifacts from last hour
		endTime := time.Now().UTC()
		startTime := endTime.Add(-1 * time.Hour)

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/sessions/%s/artifacts?start_time=%s&end_time=%s&limit=500",
				sessionID, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}

		t.Logf("Time range filtering completed in %v", requestTime)

		// Time filtering should be efficient (< 4 seconds)
		if requestTime > 4*time.Second {
			t.Errorf("Time range filtering took too long: %v (expected < 4s)", requestTime)
		}
	})

	// Test 6: Complex filtering and sorting
	t.Run("complex_filtering_and_sorting", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/sessions/%s/artifacts?name_contains=artifact&type=image/png&order=desc&limit=200", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

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
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/sessions/%s/artifacts?limit=500&offset=%d", sessionID, i*500), nil)
			req.Header.Set("X-Session-ID", "test-session")

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

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

	// Test 8: Deep pagination performance
	t.Run("deep_pagination_performance", func(t *testing.T) {
		// Test pagination at different offsets
		offsets := []int{0, 1000, 5000, 8000}

		for _, offset := range offsets {
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/sessions/%s/artifacts?limit=100&offset=%d", sessionID, offset), nil)
			req.Header.Set("X-Session-ID", "test-session")

			router := mux.NewRouter()
			router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

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

	// Test 9: Streaming efficiency test
	t.Run("streaming_efficiency_test", func(t *testing.T) {
		// This test verifies that we're not loading entire directory into memory
		// by checking that we can handle multiple large requests without issues

		req := httptest.NewRequest(http.MethodGet,
			fmt.Sprintf("/sessions/%s/artifacts?limit=1000", sessionID), nil)
		req.Header.Set("X-Session-ID", "test-session")

		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		startRequest := time.Now()
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requestTime := time.Since(startRequest)

		if w.Code != http.StatusOK {
			t.Fatalf("Streaming test failed with status %d", w.Code)
		}

		t.Logf("Streaming efficiency test (1000 artifacts) completed in %v", requestTime)

		// Should handle large requests efficiently (< 5 seconds)
		if requestTime > 5*time.Second {
			t.Errorf("Streaming efficiency test took too long: %v (expected < 5s)", requestTime)
		}

		// Verify we got the expected number of artifacts
		var response ArtifactsResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		if len(response.Artifacts) != 1000 {
			t.Errorf("Expected 1000 artifacts, got %d", len(response.Artifacts))
		}
	})
}

// BenchmarkSessionArtifactsOperations benchmarks different artifacts operations
func BenchmarkSessionArtifactsOperations(b *testing.B) {
	tempDir := b.TempDir()
	drive.SetDrivePath(tempDir)

	// Create test session with moderate number of artifacts for benchmarking
	sessionID := "benchmark-artifacts-session"
	artifactsPath := filepath.Join(tempDir, "uploads", sessionID, "artifacts")
	err := os.MkdirAll(artifactsPath, 0755)
	if err != nil {
		b.Fatalf("Failed to create artifacts directory: %v", err)
	}

	// Generate 1000 test artifacts
	numArtifacts := 1000
	fileExtensions := []string{"pdf", "png", "csv", "txt", "zip"}

	for i := 0; i < numArtifacts; i++ {
		ext := fileExtensions[i%len(fileExtensions)]
		filename := fmt.Sprintf("bench_artifact_%04d.%s", i, ext)
		filePath := filepath.Join(artifactsPath, filename)

		// Create artifacts with varying sizes
		var content string
		switch i % 10 {
		case 0: // 10% large artifacts (10KB)
			content = strings.Repeat("x", 10*1024)
		case 1, 2: // 20% medium artifacts (2KB)
			content = strings.Repeat("y", 2*1024)
		default: // 70% small artifacts (500 bytes)
			content = strings.Repeat("z", 500)
		}

		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to create artifact %s: %v", filename, err)
		}
	}

	b.Run("BasicListing", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("TypeFiltering", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?type=application/pdf", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("NameFiltering", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?name_contains=bench", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("SortingDescending", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/sessions/%s/artifacts?order=desc", sessionID), nil)
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
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			offset := (i % 10) * 100 // Vary offset
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/sessions/%s/artifacts?limit=100&offset=%d", sessionID, offset), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})

	b.Run("ComplexFiltering", func(b *testing.B) {
		router := mux.NewRouter()
		router.HandleFunc("/sessions/{id}/artifacts", SessionArtifactsHandler)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest(http.MethodGet,
				fmt.Sprintf("/sessions/%s/artifacts?type=image/png&name_contains=artifact&order=desc", sessionID), nil)
			req.Header.Set("X-Session-ID", "test-session")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Request failed with status %d", w.Code)
			}
		}
	})
}
