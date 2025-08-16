# Vault System Documentation

## Overview

The Vault system provides device detection, storage management, and file upload capabilities for the FamilyVault desktop application. It replaces the simple "Internal/External" radio buttons with a comprehensive device browser that shows real mounted volumes and drives.

## Architecture

### Main Process (vault.ts)

The vault functionality is implemented in the main Electron process to access system-level APIs for device detection and file operations.

#### Key Components:

1. **Device Detection**: Uses `drivelist` and `diskutil` (macOS) to enumerate physical drives
2. **Storage Management**: Creates and manages per-group/per-user directory structures
3. **File Operations**: Handles file copying with progress tracking and quota enforcement
4. **Configuration**: Persists vault assignments in `vault-config.json`

### Device Detection Process

#### macOS Implementation:
1. **Primary Method**: Uses `diskutil list -plist` and `diskutil info -plist <volume>` for detailed device information
2. **Fallback Method**: Uses `drivelist` package with `df -k` for capacity information
3. **Information Gathered**:
   - Device name and mount point
   - Type (internal/external/network)
   - Removable status
   - Capacity, free space, and usage percentage

#### Cross-Platform Support:
- **macOS**: Full `diskutil` integration with plist parsing
- **Windows/Linux**: Falls back to `drivelist` + `df` equivalent commands

### Storage Structure

When a user selects a storage location, the system creates:

```
<selectedVolume>/FamilyVault/<groupId>/<userId>/
├── .vault.manifest.json    # File metadata and tracking
└── <fileId>                # Actual uploaded files (UUID names)
```

#### Manifest Format:
```json
{
  "groupId": "uuid",
  "userId": "uuid", 
  "createdAt": "ISO8601",
  "files": [
    {
      "id": "uuid",
      "name": "original-filename.jpg",
      "size": 1234567,
      "hash": "sha256-hash",
      "createdAt": "ISO8601"
    }
  ]
}
```

### Configuration Schema

The vault configuration is stored in `<userData>/vault-config.json`:

```json
{
  "currentGroupId": "uuid",
  "userId": "uuid",
  "storage": {
    "<groupId>": {
      "<userId>": {
        "mountPoint": "/Volumes/SanDisk 128",
        "absolutePath": "/Volumes/SanDisk 128/FamilyVault/<groupId>/<userId>",
        "quotaBytes": 10737418240
      }
    }
  }
}
```

## IPC API

### vault:listDevices
Returns array of available storage devices.

**Response:**
```typescript
{
  ok: boolean;
  data?: DeviceInfo[];
  error?: string;
}

interface DeviceInfo {
  id: string;
  name: string;
  mountPoint: string;
  type: 'internal' | 'external' | 'network';
  isRemovable: boolean;
  capacity: number;
  free: number;
  usedPct: number;
}
```

### vault:chooseFolder
Opens native folder selection dialog.

**Response:**
```typescript
{
  ok: boolean;
  data?: string; // Selected folder path
  error?: string;
}
```

### vault:setSelection
Sets the vault location for a group/user combination.

**Parameters:**
- `groupId: string`
- `userId: string` 
- `mountPoint: string`

**Response:**
```typescript
{
  ok: boolean;
  data?: string; // Created vault path
  error?: string;
}
```

### vault:getAssignment
Gets current vault assignment for a group/user.

**Parameters:**
- `groupId: string`
- `userId: string`

**Response:**
```typescript
{
  ok: boolean;
  data?: {
    mountPoint: string;
    absolutePath: string;
    quotaBytes?: number;
    currentSize: number;
    freeSpace: number;
  };
  error?: string;
}
```

### vault:copyFiles
Copies files to the vault with progress tracking.

**Parameters:**
- `groupId: string`
- `userId: string`
- `filePaths: string[]`

**Progress Events:**
Sends `vault:copyProgress` events to renderer:
```typescript
{
  current: number;
  total: number;
  fileName: string;
  fileId: string;
}
```

### vault:openInFinder
Opens the vault folder in Finder/Explorer.

**Parameters:**
- `groupId: string`
- `userId: string`

## Security Considerations

1. **Path Traversal Prevention**: All paths are validated and sanitized
2. **Quota Enforcement**: File uploads are blocked if they exceed the configured quota
3. **File Integrity**: SHA-256 hashes are calculated and stored for all files
4. **Sandboxing**: If app sandboxing is enabled, appropriate entitlements must be configured

## Error Handling

All IPC calls return a consistent `{ok, data?, error?}` format. Common error scenarios:

- **Device enumeration fails**: Falls back to basic directory selection
- **Insufficient permissions**: Shows appropriate error messages
- **Quota exceeded**: Blocks upload with clear messaging
- **Disk full**: Detected during copy operations

## Performance Optimizations

1. **Streaming Copies**: Uses Node.js streams for large file operations
2. **Progress Tracking**: Non-blocking progress updates to renderer
3. **Lazy Loading**: Device information is fetched on-demand
4. **Caching**: Device list is cached briefly to avoid repeated system calls

## Future Enhancements

1. **Network Storage**: Enhanced support for SMB/NFS mounts
2. **Cloud Integration**: Direct upload to cloud storage providers
3. **Encryption**: Optional file-level encryption for sensitive data
4. **Compression**: Automatic compression for certain file types
5. **Deduplication**: Avoid storing duplicate files across users