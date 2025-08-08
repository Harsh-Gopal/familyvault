package handlers

import "net/http"

// RegisterRoutes wires up all HTTP routes to the provided mux.
func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/status", statusHandler)
	mux.HandleFunc("/session/open", sessionOpenHandler)
	mux.HandleFunc("/session/close", sessionCloseHandler)
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/delete", secureDeleteHandler)
	mux.HandleFunc("/list", listFilesHandler)
	mux.HandleFunc("/files", filesHandler)
	mux.HandleFunc("/files/", deleteFileHandler) // Handle DELETE /files/:filename
	mux.HandleFunc("/download", downloadHandler)
	mux.HandleFunc("/sessions/active", listActiveSessionsHandler)
	mux.HandleFunc("/sessions/", deleteSessionHandler) // Handle DELETE /sessions/{session_id}
}
