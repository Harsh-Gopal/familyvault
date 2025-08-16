package handlers

import (
	"net/http"

	"familyvault/internal/auth/localjwt"
	"familyvault/internal/auth/middleware"
	"familyvault/internal/core/groups"
	"familyvault/internal/core/rbac"
	"familyvault/internal/notify"

	"github.com/gorilla/mux"
)

// GroupRouter creates a new router with group-based endpoints
func NewGroupRouter(store *groups.Store, jwtManager *localjwt.JWTManager, notifier *notify.NotificationService) *mux.Router {
	router := mux.NewRouter()

	// Add CORS middleware
	router.Use(CORSMiddleware)

	// Create middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtManager, store)

	// Create handlers
	groupHandlers := NewGroupHandlers(store, jwtManager)
	notificationHandlers := NewNotificationHandlers(store, notifier)

	// Public endpoints (no auth required)
	router.HandleFunc("/pair", groupHandlers.Pair).Methods("POST")
	router.HandleFunc("/health", HealthHandler).Methods("GET")
	router.HandleFunc("/version", VersionHandler).Methods("GET")

	// Group creation (no auth required for initial setup)
	router.HandleFunc("/groups", groupHandlers.CreateGroup).Methods("POST")

	// Authenticated endpoints
	authRouter := router.PathPrefix("").Subrouter()
	authRouter.Use(authMiddleware.WithAuth)

	// User info
	authRouter.HandleFunc("/me", groupHandlers.WhoAmI).Methods("GET")

	// Groups listing (user can see their groups)
	authRouter.HandleFunc("/groups", groupHandlers.ListGroups).Methods("GET")

	// Group-specific routes
	groupRouter := authRouter.PathPrefix("/groups/{group_id}").Subrouter()
	groupRouter.Use(middleware.RequireGroupParam)

	// Group info (any member can view)
	groupRouter.HandleFunc("", groupHandlers.GetGroup).Methods("GET")

	// Member management (admin only)
	adminRouter := groupRouter.PathPrefix("").Subrouter()
	adminRouter.Use(authMiddleware.RequireRole(rbac.RoleAdmin))

	adminRouter.HandleFunc("/members/invite", groupHandlers.InviteMember).Methods("POST")
	adminRouter.HandleFunc("/devices/{device_id}/approve", groupHandlers.ApproveDevice).Methods("POST")
	adminRouter.HandleFunc("/roles/{user_id}", groupHandlers.UpdateMemberRole).Methods("POST")
	adminRouter.HandleFunc("/members/{user_id}", groupHandlers.RemoveMember).Methods("DELETE")
	adminRouter.HandleFunc("/notify", notificationHandlers.NotifyMembers).Methods("POST")

	// Member listing (any member can view)
	groupRouter.HandleFunc("/members", groupHandlers.ListMembers).Methods("GET")

	// Usage information (any member can view)
	groupRouter.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		GroupUsageHandler(w, r, store)
	}).Methods("GET")

	// Session management (admin only for open/close)
	sessionAdminRouter := groupRouter.PathPrefix("/sessions").Subrouter()
	sessionAdminRouter.Use(authMiddleware.RequireRole(rbac.RoleAdmin))

	sessionAdminRouter.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionOpenHandler(w, r, store)
	}).Methods("POST")
	sessionAdminRouter.HandleFunc("/close", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionCloseHandler(w, r, store)
	}).Methods("POST")

	// Session info (any member can view)
	groupRouter.HandleFunc("/sessions/active", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionActiveHandler(w, r, store)
	}).Methods("GET")

	// Session-specific routes
	sessionRouter := groupRouter.PathPrefix("/sessions/{session_id}").Subrouter()

	// Session operations (role-based)
	sessionRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionHandler(w, r, store)
	}).Methods("GET")

	sessionRouter.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionStatusHandler(w, r, store)
	}).Methods("GET")

	// File operations
	memberRouter := sessionRouter.PathPrefix("").Subrouter()
	memberRouter.Use(authMiddleware.RequirePermission(rbac.CanUpload))

	memberRouter.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionFileUploadHandler(w, r, store)
	}).Methods("POST")

	// File listing and viewing (viewer+ can access)
	viewerRouter := sessionRouter.PathPrefix("").Subrouter()
	viewerRouter.Use(authMiddleware.RequirePermission(rbac.CanDownload))

	viewerRouter.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionFilesHandler(w, r, store)
	}).Methods("GET")

	viewerRouter.HandleFunc("/files/{filename}/download", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionFileDownloadHandler(w, r, store)
	}).Methods("GET")

	viewerRouter.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionLogsHandler(w, r, store)
	}).Methods("GET")

	viewerRouter.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionMetricsHandler(w, r, store)
	}).Methods("GET")

	// File deletion (member+ for own files, admin for any)
	sessionRouter.HandleFunc("/files/{filename}", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionFileDeleteHandler(w, r, store)
	}).Methods("DELETE")

	// Admin-only session operations
	sessionAdminSpecificRouter := sessionRouter.PathPrefix("").Subrouter()
	sessionAdminSpecificRouter.Use(authMiddleware.RequireRole(rbac.RoleAdmin))

	sessionAdminSpecificRouter.HandleFunc("/duplicate", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionDuplicateHandler(w, r, store)
	}).Methods("POST")

	sessionAdminSpecificRouter.HandleFunc("/restore", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionRestoreHandler(w, r, store)
	}).Methods("POST")

	sessionAdminSpecificRouter.HandleFunc("", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionDeleteHandler(w, r, store)
	}).Methods("DELETE")

	sessionAdminSpecificRouter.HandleFunc("/download-all", func(w http.ResponseWriter, r *http.Request) {
		GroupSessionDownloadAllHandler(w, r, store)
	}).Methods("GET")

	return router
}

// Session handlers are implemented in group_sessions.go

// Placeholder handlers - these will be implemented in subsequent files
func GroupSessionFileUploadHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionFilesHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionFileDownloadHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionLogsHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionMetricsHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionFileDeleteHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionDuplicateHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionRestoreHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}

func GroupSessionDownloadAllHandler(w http.ResponseWriter, r *http.Request, store *groups.Store) {
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}
