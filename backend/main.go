package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"familyvault/internal/auth/localjwt"
	"familyvault/internal/config"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/groups"
	handlers "familyvault/internal/http/handlers"
	"familyvault/internal/notify"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Configure drive path
	if cfg.DrivePath == "" {
		cfg.DrivePath = "/tmp/familyvault-drive"
	}
	drive.SetDrivePath(cfg.DrivePath)

	// Initialize groups store
	store, err := groups.NewStore(cfg.DataPath)
	if err != nil {
		log.Fatalf("Failed to initialize groups store: %v", err)
	}

	// Initialize JWT manager
	jwtManager, err := localjwt.NewJWTManager(cfg.DataPath)
	if err != nil {
		log.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	// Initialize notification service
	notifier := notify.NewNotificationService(cfg)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create routers
	legacyMux := http.NewServeMux()
	handlers.RegisterRoutes(legacyMux) // Keep legacy routes for backward compatibility

	groupRouter := handlers.NewGroupRouter(store, jwtManager, notifier)

	// Create main mux that handles both legacy and new routes
	mainMux := http.NewServeMux()

	// Mount group-based routes at root
	mainMux.Handle("/", groupRouter)

	// Mount legacy routes with a prefix for backward compatibility
	mainMux.Handle("/legacy/", http.StripPrefix("/legacy", legacyMux))

	addr := ":8000"
	server := &http.Server{
		Addr:              addr,
		Handler:           logRequestMiddleware(mainMux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("FamilyVault server starting on %s", addr)
		log.Printf("Drive path: %s", cfg.DrivePath)
		log.Printf("Data path: %s", cfg.DataPath)
		log.Printf("SMTP configured: %v", cfg.IsSMTPConfigured())
		log.Printf("SMS configured: %v", cfg.IsSMSConfigured())

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Printf("Shutdown signal received, stopping server...")
	log.Printf("Server stopped")
}

func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
