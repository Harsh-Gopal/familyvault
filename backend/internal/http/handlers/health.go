package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"familyvault/internal/core/drive"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status         string `json:"status"`
	Uptime         string `json:"uptime"`
	ActiveSessions int    `json:"active_sessions"`
	CPUUsage       string `json:"cpu_usage"`
	MemoryUsage    string `json:"memory_usage"`
	DiskUsage      string `json:"disk_usage"`
	Timestamp      string `json:"timestamp"`
}

var startTime = time.Now()

// HealthHandler handles GET /health
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Calculate uptime
	uptime := time.Since(startTime)
	uptimeStr := formatDuration(uptime)

	// Count active sessions
	activeSessions := countActiveSessions()

	// Get memory stats
	var memStats runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	memoryUsage := fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/(1024*1024))

	// Get CPU usage (simplified - just goroutine count as proxy)
	cpuUsage := fmt.Sprintf("%d goroutines", runtime.NumGoroutine())

	// Get disk usage
	diskUsage := getDiskUsage()

	// Create response
	response := HealthResponse{
		Status:         "ok",
		Uptime:         uptimeStr,
		ActiveSessions: activeSessions,
		CPUUsage:       cpuUsage,
		MemoryUsage:    memoryUsage,
		DiskUsage:      diskUsage,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm%ds", days, hours, minutes, seconds)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

// countActiveSessions counts the number of active sessions
func countActiveSessions() int {
	uploadsPath := filepath.Join(drive.GetDrivePath(), "uploads")

	entries, err := os.ReadDir(uploadsPath)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}

	return count
}

// getDiskUsage gets disk usage information
func getDiskUsage() string {
	basePath := drive.GetDrivePath()

	var totalSize int64
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil {
		return "unknown"
	}

	// Convert to human readable format
	if totalSize < 1024 {
		return fmt.Sprintf("%d B", totalSize)
	} else if totalSize < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(totalSize)/1024)
	} else if totalSize < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(totalSize)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(totalSize)/(1024*1024*1024))
	}
}
