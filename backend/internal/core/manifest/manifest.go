package manifest

import (
	"encoding/json"
	"familyvault/internal/core/paths"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry tracks metadata for an uploaded file with group and user context
type Entry struct {
	GroupID    string            `json:"group_id"`
	UserID     string            `json:"user_id"`
	SessionID  string            `json:"session_id"`
	Filename   string            `json:"filename"`
	UploadedAt time.Time         `json:"uploaded_at"`
	Tags       map[string]string `json:"tags,omitempty"`
	Size       int64             `json:"size,omitempty"`
}

// FileRecord is kept for backward compatibility
type FileRecord struct {
	SessionID  string            `json:"session_id"`
	Filename   string            `json:"filename"`
	UploadedAt time.Time         `json:"uploaded_at"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// Manager handles manifest operations for a group
type Manager struct {
	mu      sync.RWMutex
	groupID string
	entries []Entry
}

// NewManager creates a new manifest manager for a group
func NewManager(groupID string) (*Manager, error) {
	m := &Manager{
		groupID: groupID,
		entries: make([]Entry, 0),
	}

	// Load existing entries
	if err := m.load(); err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	return m, nil
}

// Global managers cache
var (
	managersMu sync.RWMutex
	managers   = make(map[string]*Manager)
)

// GetManager returns a manager for the specified group
func GetManager(groupID string) (*Manager, error) {
	managersMu.RLock()
	if manager, exists := managers[groupID]; exists {
		managersMu.RUnlock()
		return manager, nil
	}
	managersMu.RUnlock()

	managersMu.Lock()
	defer managersMu.Unlock()

	// Double-check after acquiring write lock
	if manager, exists := managers[groupID]; exists {
		return manager, nil
	}

	manager, err := NewManager(groupID)
	if err != nil {
		return nil, err
	}

	managers[groupID] = manager
	return manager, nil
}

// Add appends an entry to the manifest
func (m *Manager) Add(entry Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry.GroupID = m.groupID
	m.entries = append(m.entries, entry)
	return m.persist()
}

// List returns a copy of all entries
func (m *Manager) List() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// ListByUser returns entries for a specific user
func (m *Manager) ListByUser(userID string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Entry
	for _, entry := range m.entries {
		if entry.UserID == userID {
			result = append(result, entry)
		}
	}
	return result
}

// ListBySession returns entries for a specific session
func (m *Manager) ListBySession(sessionID string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Entry
	for _, entry := range m.entries {
		if entry.SessionID == sessionID {
			result = append(result, entry)
		}
	}
	return result
}

// ListBySessionAndUser returns entries for a specific session and user
func (m *Manager) ListBySessionAndUser(sessionID, userID string) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Entry
	for _, entry := range m.entries {
		if entry.SessionID == sessionID && entry.UserID == userID {
			result = append(result, entry)
		}
	}
	return result
}

// Remove deletes an entry matching sessionID, userID, and filename
func (m *Manager) Remove(sessionID, userID, filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filtered := m.entries[:0]
	for _, entry := range m.entries {
		if entry.SessionID == sessionID && entry.UserID == userID && entry.Filename == filename {
			continue // Skip this entry (remove it)
		}
		filtered = append(filtered, entry)
	}
	m.entries = filtered
	return m.persist()
}

// RemoveAllForSession removes all entries for a session
func (m *Manager) RemoveAllForSession(sessionID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	filtered := m.entries[:0]
	for _, entry := range m.entries {
		if entry.SessionID == sessionID {
			count++
			continue
		}
		filtered = append(filtered, entry)
	}
	m.entries = filtered
	return count, m.persist()
}

// Exists returns true if an entry exists for the session, user, and filename
func (m *Manager) Exists(sessionID, userID, filename string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.entries {
		if entry.SessionID == sessionID && entry.UserID == userID && entry.Filename == filename {
			return true
		}
	}
	return false
}

// GetEntry returns a specific entry
func (m *Manager) GetEntry(sessionID, userID, filename string) (*Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, entry := range m.entries {
		if entry.SessionID == sessionID && entry.UserID == userID && entry.Filename == filename {
			return &entry, true
		}
	}
	return nil, false
}

// UpdateFileMetadata updates the metadata (tags) for a specific file
func (m *Manager) UpdateFileMetadata(sessionID, userID, filename string, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, entry := range m.entries {
		if entry.SessionID == sessionID && entry.UserID == userID && entry.Filename == filename {
			// Convert metadata to string map for tags
			if entry.Tags == nil {
				entry.Tags = make(map[string]string)
			}

			// Update tags with new metadata
			for key, value := range metadata {
				if strValue, ok := value.(string); ok {
					entry.Tags[key] = strValue
				} else {
					// Convert non-string values to string
					entry.Tags[key] = fmt.Sprintf("%v", value)
				}
			}

			m.entries[i] = entry
			return m.persist()
		}
	}
	return fmt.Errorf("entry not found")
}

// GetUserUsage calculates total bytes used by a user
func (m *Manager) GetUserUsage(userID string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var total int64
	for _, entry := range m.entries {
		if entry.UserID == userID {
			total += entry.Size
		}
	}
	return total
}

// Persistence methods

func (m *Manager) load() error {
	manifestPath := filepath.Join(paths.ManifestsDir(m.groupID), "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return nil // No manifest file yet, start with empty
	}
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	m.entries = entries
	return nil
}

func (m *Manager) persist() error {
	manifestsDir := paths.ManifestsDir(m.groupID)
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return fmt.Errorf("failed to create manifests directory: %w", err)
	}

	manifestPath := filepath.Join(manifestsDir, "manifest.json")
	data, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return os.WriteFile(manifestPath, data, 0644)
}

// SessionMetadata holds session-level metadata
type SessionMetadata struct {
	GroupID   string                 `json:"group_id"`
	SessionID string                 `json:"session_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// UpdateSessionMetadata updates metadata for an entire session
func (m *Manager) UpdateSessionMetadata(sessionID string, metadata map[string]interface{}) error {
	// For now, store session metadata in a separate file
	// In a more sophisticated implementation, this could be in a database
	sessionMetadataPath := filepath.Join(paths.ManifestsDir(m.groupID), fmt.Sprintf("session_%s_metadata.json", sessionID))

	existing := SessionMetadata{
		GroupID:   m.groupID,
		SessionID: sessionID,
		Metadata:  make(map[string]interface{}),
	}

	// Try to load existing metadata
	if data, err := os.ReadFile(sessionMetadataPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	// Merge new metadata with existing
	for key, value := range metadata {
		existing.Metadata[key] = value
	}
	existing.UpdatedAt = time.Now()

	// Persist updated metadata
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session metadata: %w", err)
	}

	return os.WriteFile(sessionMetadataPath, data, 0644)
}

// GetSessionMetadata retrieves metadata for a session
func (m *Manager) GetSessionMetadata(sessionID string) (SessionMetadata, bool) {
	sessionMetadataPath := filepath.Join(paths.ManifestsDir(m.groupID), fmt.Sprintf("session_%s_metadata.json", sessionID))

	data, err := os.ReadFile(sessionMetadataPath)
	if err != nil {
		return SessionMetadata{}, false
	}

	var metadata SessionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return SessionMetadata{}, false
	}

	return metadata, true
}

// ClearSessionMetadata removes session metadata
func (m *Manager) ClearSessionMetadata(sessionID string) error {
	sessionMetadataPath := filepath.Join(paths.ManifestsDir(m.groupID), fmt.Sprintf("session_%s_metadata.json", sessionID))
	err := os.Remove(sessionMetadataPath)
	if os.IsNotExist(err) {
		return nil // Already doesn't exist
	}
	return err
}

// Backward compatibility functions (deprecated)

var (
	legacyMu      sync.RWMutex
	legacyRecords []FileRecord
)

// Add appends a file record to the in-memory manifest (legacy)
// Deprecated: Use Manager.Add instead
func Add(rec FileRecord) {
	legacyMu.Lock()
	legacyRecords = append(legacyRecords, rec)
	legacyMu.Unlock()
}

// List returns a copy of all file records (legacy)
// Deprecated: Use Manager.List instead
func List() []FileRecord {
	legacyMu.RLock()
	defer legacyMu.RUnlock()
	out := make([]FileRecord, len(legacyRecords))
	copy(out, legacyRecords)
	return out
}

// Clear removes all records (legacy)
// Deprecated: Use Manager methods instead
func Clear() {
	legacyMu.Lock()
	legacyRecords = nil
	legacyMu.Unlock()
}

// Remove deletes a record matching sessionID and filename (legacy)
// Deprecated: Use Manager.Remove instead
func Remove(sessionID, filename string) bool {
	legacyMu.Lock()
	defer legacyMu.Unlock()
	removed := false
	filtered := legacyRecords[:0]
	for _, r := range legacyRecords {
		if r.SessionID == sessionID && r.Filename == filename {
			removed = true
			continue
		}
		filtered = append(filtered, r)
	}
	legacyRecords = filtered
	return removed
}

// Belongs returns true if a record exists for the session and filename (legacy)
// Deprecated: Use Manager.Exists instead
func Belongs(sessionID, filename string) bool {
	legacyMu.RLock()
	defer legacyMu.RUnlock()
	for _, r := range legacyRecords {
		if r.SessionID == sessionID && r.Filename == filename {
			return true
		}
	}
	return false
}

// Legacy compatibility functions - these use a default group for backward compatibility

var (
	legacyGroupID = "default"
	legacyManager *Manager
	legacyOnce    sync.Once
)

func getLegacyManager() *Manager {
	legacyOnce.Do(func() {
		var err error
		legacyManager, err = NewManager(legacyGroupID)
		if err != nil {
			// Fallback to in-memory only
			legacyManager = &Manager{
				groupID: legacyGroupID,
				entries: make([]Entry, 0),
			}
		}
	})
	return legacyManager
}

// UpdateFileMetadata updates the metadata (tags) for a specific file in a session (legacy)
// Deprecated: Use Manager.UpdateFileMetadata instead
func UpdateFileMetadata(sessionID, filename string, metadata map[string]interface{}) bool {
	manager := getLegacyManager()
	// For legacy compatibility, assume a default user ID
	err := manager.UpdateFileMetadata(sessionID, "legacy-user", filename, metadata)
	return err == nil
}

// RemoveAllForSession removes all records for a session and returns count removed (legacy)
// Deprecated: Use Manager.RemoveAllForSession instead
func RemoveAllForSession(sessionID string) int {
	manager := getLegacyManager()
	count, _ := manager.RemoveAllForSession(sessionID)
	return count
}

// UpdateSessionMetadata updates metadata for an entire session (legacy)
// Deprecated: Use Manager.UpdateSessionMetadata instead
func UpdateSessionMetadata(sessionID string, metadata map[string]interface{}) {
	manager := getLegacyManager()
	manager.UpdateSessionMetadata(sessionID, metadata)
}

// GetSessionMetadata retrieves metadata for a session (legacy)
// Deprecated: Use Manager.GetSessionMetadata instead
func GetSessionMetadata(sessionID string) (SessionMetadata, bool) {
	manager := getLegacyManager()
	return manager.GetSessionMetadata(sessionID)
}

// ClearSessionMetadata removes session metadata (legacy)
// Deprecated: Use Manager.ClearSessionMetadata instead
func ClearSessionMetadata(sessionID string) {
	manager := getLegacyManager()
	manager.ClearSessionMetadata(sessionID)
}
