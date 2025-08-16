package handlers

import (
	"net/http"
	"strings"
)

// RegisterRoutes wires up all HTTP routes to the provided mux.
func RegisterRoutes(mux *http.ServeMux) {
	// Legacy endpoints
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/session/open", sessionOpenHandler)
	mux.HandleFunc("/session/close", sessionCloseHandler)
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/upload-file", uploadFileHandler)
	mux.HandleFunc("/delete", secureDeleteHandler)
	mux.HandleFunc("/list", listFilesHandler)
	mux.HandleFunc("/files", filesHandler)
	mux.HandleFunc("/files/", deleteFileHandler) // Handle DELETE /files/:filename
	mux.HandleFunc("/download", downloadHandler)
	mux.HandleFunc("/download-all", downloadAllHandler)
	mux.HandleFunc("/search-files", searchFilesHandler)
	mux.HandleFunc("/update-metadata", updateMetadataHandler)
	mux.HandleFunc("/sessions/active", listActiveSessionsHandler)

	// System endpoints
	mux.HandleFunc("/health", MetricsMiddleware("health", HealthHandler))
	mux.HandleFunc("/metrics", MetricsHandler)

	// Session management endpoints
	mux.HandleFunc("/sessions", MetricsMiddleware("sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			SessionsHandler(w, r)
		case http.MethodPost:
			CreateSessionHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	// Session-specific endpoints
	mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Handle different session endpoints
		if r.Method == http.MethodGet && strings.Contains(path, "/files/") && strings.HasSuffix(path, "/download") {
			SessionFileDownloadHandler(w, r)
		} else if r.Method == http.MethodDelete && strings.Contains(path, "/files/") {
			SessionFileDeleteHandler(w, r)
		} else if r.Method == http.MethodPost && strings.HasSuffix(path, "/files/upload") {
			SessionFileUploadHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/logs/search") {
			SessionLogsSearchHandler(w, r)
		} else if r.Method == http.MethodDelete {
			DeleteSessionHandler(w, r)
		} else if r.Method == http.MethodPatch {
			UpdateSessionHandler(w, r)
		} else if r.Method == http.MethodPost && strings.HasSuffix(path, "/duplicate") {
			duplicateSessionHandler(w, r)
		} else if r.Method == http.MethodPost && strings.HasSuffix(path, "/restore") {
			restoreSessionHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/status") {
			sessionStatusHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/logs") {
			sessionLogsHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/files") {
			SessionFilesHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/metrics") {
			SessionMetricsHandler(w, r)
		} else if r.Method == http.MethodGet && strings.HasSuffix(path, "/artifacts") {
			SessionArtifactsHandler(w, r)
		} else if r.Method == http.MethodGet && !strings.Contains(path, "/") {
			SessionHandler(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
