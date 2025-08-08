package manifest

import (
	"sync"
	"time"
)

// FileRecord tracks metadata for an uploaded file.
type FileRecord struct {
	SessionID  string            `json:"session_id"`
	Filename   string            `json:"filename"`
	UploadedAt time.Time         `json:"uploaded_at"`
	Tags       map[string]string `json:"tags,omitempty"`
}

var (
	mu      sync.RWMutex
	records []FileRecord
)

// Add appends a file record to the in-memory manifest.
func Add(rec FileRecord) {
	mu.Lock()
	records = append(records, rec)
	mu.Unlock()
}

// List returns a copy of all file records.
func List() []FileRecord {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]FileRecord, len(records))
	copy(out, records)
	return out
}

// Clear removes all records (useful for tests).
func Clear() {
	mu.Lock()
	records = nil
	mu.Unlock()
}

// Remove deletes a record matching sessionID and filename. Returns true if removed.
func Remove(sessionID, filename string) bool {
	mu.Lock()
	defer mu.Unlock()
	removed := false
	filtered := records[:0]
	for _, r := range records {
		if r.SessionID == sessionID && r.Filename == filename {
			removed = true
			continue
		}
		filtered = append(filtered, r)
	}
	records = filtered
	return removed
}

// RemoveAllForSession removes all records for a session and returns count removed.
func RemoveAllForSession(sessionID string) int {
	mu.Lock()
	defer mu.Unlock()
	count := 0
	filtered := records[:0]
	for _, r := range records {
		if r.SessionID == sessionID {
			count++
			continue
		}
		filtered = append(filtered, r)
	}
	records = filtered
	return count
}

// Belongs returns true if a record exists for the session and filename.
func Belongs(sessionID, filename string) bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range records {
		if r.SessionID == sessionID && r.Filename == filename {
			return true
		}
	}
	return false
}
