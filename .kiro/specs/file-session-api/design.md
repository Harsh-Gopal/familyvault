# File Session & Metadata Management API Design

## Overview

This design document outlines the technical architecture for a REST API that manages file uploads, sessions, and metadata with robust validation, security, and error handling. The system uses a session-based approach where files are organized within user sessions and metadata can be managed at both file and session levels.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│   HTTP Router   │───▶│   Handlers      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                                               ┌─────────────────┐
                                               │   Middleware    │
                                               │  - Session Auth │
                                               │  - Validation   │
                                               │  - Sanitization │
                                               └─────────────────┘
                                                        │
                       ┌─────────────────┐    ┌─────────────────┐
                       │   File Storage  │◀───│   Core Services │
                       │   - Session Dir │    │  - Manifest Mgmt│
                       │   - File System │    │  - Session Mgmt │
                       └─────────────────┘    └─────────────────┘
```

### Directory Structure

```
sessions/
├── {session-id}/
│   ├── manifest.json          # Session and file metadata
│   ├── files/
│   │   ├── {file-id-1}       # Actual uploaded files
│   │   └── {file-id-2}
│   └── metadata.json         # Session-level metadata
```

## Components and Interfaces

### 1. HTTP Layer

#### Router Configuration
- **Framework**: Standard Go `net/http` with custom routing
- **Middleware Stack**: Session validation, request logging, error handling
- **Content Types**: `multipart/form-data` for uploads, `application/json` for metadata

#### Endpoints
```go
POST   /upload              // File upload with validation
PATCH  /update-metadata     // Metadata updates (file or session)
GET    /files              // List all files in session
GET    /file/:file_id      // Retrieve specific file
DELETE /file/:file_id      // Delete specific file
DELETE /session            // Clear entire session
```

### 2. Session Management

#### Session Validation
```go
type SessionValidator interface {
    ValidateSession(sessionID string) (*Session, error)
    ExtractSessionID(r *http.Request) (string, error)
}

type Session struct {
    ID        string    `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    ExpiresAt time.Time `json:"expires_at"`
    Active    bool      `json:"active"`
}
```

#### Session Directory Management
- Each session gets isolated directory: `sessions/{session-id}/`
- Automatic cleanup of expired sessions
- Concurrent access protection with file locking

### 3. File Management

#### File Storage Interface
```go
type FileStorage interface {
    Store(sessionID string, file multipart.File, header *multipart.FileHeader) (*StoredFile, error)
    Retrieve(sessionID, fileID string) (*StoredFile, error)
    Delete(sessionID, fileID string) error
    List(sessionID string) ([]*StoredFile, error)
}

type StoredFile struct {
    ID         string            `json:"id"`
    Filename   string            `json:"filename"`
    Size       int64             `json:"size"`
    MimeType   string            `json:"mime_type"`
    UploadedAt time.Time         `json:"uploaded_at"`
    Path       string            `json:"-"` // Internal use only
    Metadata   map[string]string `json:"metadata,omitempty"`
}
```

#### File Validation
```go
type FileValidator struct {
    MaxSize      int64    // 100MB default
    AllowedTypes []string // jpg, png, pdf, docx, csv
}

func (v *FileValidator) Validate(file multipart.File, header *multipart.FileHeader) error
```

### 4. Manifest Management

#### Manifest Structure
```go
type Manifest struct {
    SessionID   string                 `json:"session_id"`
    CreatedAt   time.Time             `json:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at"`
    Files       map[string]*FileRecord `json:"files"`
    Metadata    map[string]interface{} `json:"session_metadata,omitempty"`
    mu          sync.RWMutex          `json:"-"`
}

type FileRecord struct {
    SessionID  string            `json:"session_id"`
    Filename   string            `json:"filename"`
    Size       int64             `json:"size"`
    MimeType   string            `json:"mime_type"`
    UploadedAt time.Time         `json:"uploaded_at"`
    UpdatedAt  *time.Time        `json:"updated_at,omitempty"`
    Tags       map[string]string `json:"tags,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

#### Manifest Operations
```go
type ManifestManager interface {
    Load(sessionID string) (*Manifest, error)
    Save(manifest *Manifest) error
    AddFile(sessionID string, file *FileRecord) error
    UpdateFileMetadata(sessionID, fileID string, metadata map[string]interface{}) error
    UpdateSessionMetadata(sessionID string, metadata map[string]interface{}) error
    RemoveFile(sessionID, fileID string) error
    Clear(sessionID string) error
}
```

### 5. Metadata Management

#### Metadata Sanitization
```go
type MetadataSanitizer struct {
    MaxValueLength int    // 255 chars default
    MaxFields      int    // 50 fields default
    AllowedKeys    []string // snake_case validation
}

func (s *MetadataSanitizer) Sanitize(metadata map[string]interface{}) (map[string]interface{}, error)
```

#### Validation Rules
- Keys must be snake_case format: `^[a-z][a-z0-9_]*$`
- Values must be strings, numbers, or booleans
- String values limited to 255 characters
- Maximum 50 metadata fields per update
- HTML/script content stripped from string values

## Data Models

### Core Data Structures

#### Request/Response Models
```go
// Update Metadata Request
type UpdateMetadataRequest struct {
    FileID   string                 `json:"file_id,omitempty"`
    Metadata map[string]interface{} `json:"metadata"`
}

// Standard API Response
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
}

// Error Response
type APIError struct {
    Code    string `json:"error_code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}
```

#### File Upload Response
```go
type UploadResponse struct {
    Files []UploadedFile `json:"files"`
}

type UploadedFile struct {
    FileID     string    `json:"file_id"`
    Filename   string    `json:"filename"`
    Size       int64     `json:"size"`
    MimeType   string    `json:"mime_type"`
    UploadedAt time.Time `json:"uploaded_at"`
}
```

### Database Schema (File-based)

#### manifest.json Structure
```json
{
  "session_id": "sess_abc123",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T11:45:00Z",
  "files": {
    "file_001": {
      "session_id": "sess_abc123",
      "filename": "document.pdf",
      "size": 1048576,
      "mime_type": "application/pdf",
      "uploaded_at": "2025-01-15T10:30:00Z",
      "updated_at": "2025-01-15T11:45:00Z",
      "tags": {
        "category": "documents",
        "priority": "high"
      },
      "metadata": {
        "description": "Important contract document",
        "department": "legal",
        "confidential": true
      }
    }
  },
  "session_metadata": {
    "project_name": "Q1 Legal Review",
    "created_by": "user123",
    "department": "legal"
  }
}
```

## Error Handling

### Error Code System
```go
const (
    ErrNoSession        = "ERR_NO_SESSION"
    ErrInvalidFile      = "ERR_INVALID_FILE"
    ErrFileNotFound     = "ERR_FILE_NOT_FOUND"
    ErrInvalidMetadata  = "ERR_INVALID_METADATA"
    ErrInternal         = "ERR_INTERNAL"
)
```

### Error Response Format
All errors return consistent JSON structure:
```json
{
  "success": false,
  "error": {
    "error_code": "ERR_INVALID_FILE",
    "message": "File size exceeds maximum limit of 100MB",
    "details": "Uploaded file size: 150MB, Maximum allowed: 100MB"
  }
}
```

### HTTP Status Code Mapping
- `400 Bad Request`: ERR_INVALID_FILE, ERR_INVALID_METADATA
- `401 Unauthorized`: ERR_NO_SESSION
- `404 Not Found`: ERR_FILE_NOT_FOUND
- `500 Internal Server Error`: ERR_INTERNAL

## Security Implementation

### Input Sanitization
```go
type SecuritySanitizer struct {
    HTMLStripper    *html.Stripper
    PathSanitizer   *path.Sanitizer
    MetadataEscaper *metadata.Escaper
}
```

### Security Measures
1. **File Name Sanitization**: Remove path traversal characters (`../`, `./`, `\`)
2. **MIME Type Validation**: Verify content-based MIME type, not just extension
3. **Metadata Escaping**: HTML entity encoding for all string values
4. **Session Isolation**: Strict session-based access control
5. **Path Protection**: Never expose internal file system paths

### Content Security
- File content scanning for basic malware patterns
- MIME type verification against file headers
- File size limits enforced at multiple layers
- Temporary file cleanup after processing

## Testing Strategy

### Unit Testing
```go
// Manifest operations
func TestManifest_UpdateFileMetadata(t *testing.T)
func TestManifest_UpdateSessionMetadata(t *testing.T)
func TestManifest_ConcurrentAccess(t *testing.T)

// Validation
func TestFileValidator_ValidateSize(t *testing.T)
func TestFileValidator_ValidateMimeType(t *testing.T)
func TestMetadataSanitizer_SanitizeInput(t *testing.T)

// Security
func TestSecurity_PathTraversal(t *testing.T)
func TestSecurity_MetadataEscaping(t *testing.T)
```

### Integration Testing
```go
// End-to-end workflows
func TestE2E_UploadUpdateRetrieve(t *testing.T)
func TestE2E_SessionMetadataFlow(t *testing.T)
func TestE2E_FileDeleteFlow(t *testing.T)
func TestE2E_SessionClearFlow(t *testing.T)

// Error scenarios
func TestE2E_InvalidSession(t *testing.T)
func TestE2E_FileNotFound(t *testing.T)
func TestE2E_InvalidMetadata(t *testing.T)
```

### Manual Testing Scripts
- `upload_manual_test.sh`: File upload scenarios
- `update_metadata_manual_test.sh`: Metadata update scenarios
- `download_manual_test.sh`: File retrieval scenarios
- `delete_manual_test.sh`: File and session deletion scenarios

### Performance Testing
- Concurrent file uploads (100 simultaneous)
- Large file handling (up to 100MB)
- Metadata update performance (1000 updates/second)
- Session cleanup performance

## Implementation Phases

### Phase 1: Core Infrastructure
1. Session management and validation
2. File storage and manifest system
3. Basic HTTP routing and middleware
4. Error handling framework

### Phase 2: File Operations
1. File upload with validation
2. File retrieval and download
3. File deletion
4. File listing

### Phase 3: Metadata Management
1. File-level metadata updates
2. Session-level metadata updates
3. Metadata validation and sanitization
4. Metadata persistence

### Phase 4: Security and Testing
1. Security hardening
2. Comprehensive test suite
3. Performance optimization
4. Documentation and deployment guides

## Performance Considerations

### Concurrency
- Read-write locks for manifest operations
- Goroutine pools for file processing
- Channel-based request queuing
- Connection pooling for high throughput

### Memory Management
- Streaming file uploads to avoid memory spikes
- Efficient JSON marshaling/unmarshaling
- Garbage collection optimization
- Resource cleanup after operations

### Storage Optimization
- Efficient file organization in session directories
- Manifest file size optimization
- Automatic cleanup of expired sessions
- Disk space monitoring and alerts

## Monitoring and Logging

### Logging Strategy
```go
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    Debug(msg string, fields ...Field)
}
```

### Metrics Collection
- Request latency and throughput
- File upload/download rates
- Error rates by endpoint
- Session creation/cleanup rates
- Storage usage metrics

### Health Checks
- `/health`: Basic service health
- `/health/storage`: Storage system health
- `/health/sessions`: Session management health