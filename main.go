package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"familyvault/internal/core/drive"
	"familyvault/internal/core/session"
	handlers "familyvault/internal/http/handlers"
)

func main() {
	// Configure simulated drive path (can be overridden via env var)
	drivePath := os.Getenv("FAMILYVAULT_DRIVE_PATH")
	if drivePath == "" {
		drivePath = "/tmp/familyvault-drive"
	}
	drive.SetDrivePath(drivePath)

	// Set drive path for session cleanup
	session.SetDrivePath(drivePath)

	// Start background session cleanup routine
	session.StartCleanupRoutine()
	defer session.StopCleanupRoutine()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	addr := ":8000"
	server := &http.Server{
		Addr:              addr,
		Handler:           logRequestMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("familyvault server starting on %s (drive path: %s)\n", addr, drivePath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Printf("Shutdown signal received, stopping server...")

	// Stop cleanup routine
	session.StopCleanupRoutine()
	log.Printf("Server stopped")
}

func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
