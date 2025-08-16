# FamilyVault

A secure, offline-first family file sharing and storage solution with role-based access control.

## Features

- **Multi-tenant Groups**: Create and manage family groups with role-based permissions
- **Offline-First**: No cloud dependencies - all data stored locally
- **Role-Based Access Control**: Admin, Member, and Viewer roles with granular permissions
- **Device Pairing**: QR code and token-based device onboarding with admin approval
- **Secure Storage**: AES-256 encryption for all files
- **Session Management**: Time-limited upload sessions with automatic cleanup
- **Native Desktop App**: macOS Electron app with system integration
- **Local Authentication**: JWT-based auth with Keychain storage

## Architecture

```
familyvault/
├── backend/                 # Go backend server
│   ├── internal/
│   │   ├── core/           # Core business logic
│   │   ├── http/           # HTTP handlers and routing
│   │   └── auth/           # Authentication and middleware
│   └── main.go
├── desktop/                # Desktop application
│   ├── apps/admin-desktop/ # Electron + React app
│   └── packages/           # Shared packages
│       ├── shared/         # API client and types
│       └── ui/             # UI components
└── Makefile
```

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- pnpm
- macOS (for desktop app)

### Development

1. **Clone and setup**:
   ```bash
   git clone <repo>
   cd familyvault
   make install
   ```

2. **Start development server**:
   ```bash
   make dev
   ```

   This will:
   - Build the Go backend for your platform
   - Start the Electron app in development mode
   - Hot reload both frontend and backend changes

3. **Run tests**:
   ```bash
   make test
   ```

### Production Build

1. **Build for macOS (Apple Silicon)**:
   ```bash
   make build
   ```

   This creates:
   - `desktop/apps/admin-desktop/release/FamilyVault-1.0.0-arm64.dmg`
   - `desktop/apps/admin-desktop/release/FamilyVault-1.0.0-arm64-mac.zip`

## Usage

### First Time Setup

1. **Launch FamilyVault**
2. **Create Family Group**:
   - Enter group name (e.g., "Smith Family")
   - Enter your name and contact info
   - You become the admin automatically

3. **Invite Family Members**:
   - Go to Members → Invite Member
   - Enter their email/phone
   - Share the pairing token or QR code
   - Approve their device when they pair

### Daily Workflow

1. **Admin starts a session**:
   ```bash
   Dashboard → Start Session
   ```

2. **Members upload files**:
   - Drag & drop files in Sessions → Active Session
   - Files are encrypted and stored per-user

3. **Download files**:
   - Individual files or Download All (ZIP)
   - Available to all roles based on permissions

4. **Session management**:
   - Sessions auto-expire (default 1 hour)
   - Admin can extend or close early
   - Files remain accessible after session ends

## API Usage

The desktop app communicates with a local HTTP API:

```bash
# Health check
curl http://127.0.0.1:8000/health

# Create group (first time)
curl -X POST http://127.0.0.1:8000/groups \
  -H "Content-Type: application/json" \
  -H "X-Device-Name: MacBook-Pro" \
  -d '{
    "name": "My Family",
    "owner_display_name": "John Doe",
    "email": "john@example.com"
  }'

# Get user info (with JWT)
curl -H "Authorization: Bearer <token>" \
  http://127.0.0.1:8000/me
```

## Configuration

Set environment variables:

```bash
# Required
export FAMILYVAULT_DATA_PATH="$HOME/.familyvault"
export FAMILYVAULT_DRIVE_PATH="$HOME/FamilyVault"

# Optional - Email notifications
export FAMILYVAULT_SMTP_HOST="smtp.gmail.com"
export FAMILYVAULT_SMTP_PORT="587"
export FAMILYVAULT_SMTP_USER="your-email@gmail.com"
export FAMILYVAULT_SMTP_PASS="your-app-password"
export FAMILYVAULT_SMTP_FROM="your-email@gmail.com"
export FAMILYVAULT_SMTP_TLS="true"

# Optional - Token lifetimes
export FAMILYVAULT_JWT_TTL_MINUTES="1440"      # 24 hours
export FAMILYVAULT_PAIRING_TTL_MINUTES="60"    # 1 hour
```

## Security

- **Local-only**: No external servers or cloud dependencies
- **Encryption**: All files encrypted with AES-256
- **JWT Authentication**: Tokens stored securely in macOS Keychain
- **Device Approval**: Admin must approve each device
- **Role-based Access**: Granular permissions (admin/member/viewer)
- **Path Traversal Protection**: Input validation and sanitization

## Permissions Matrix

| Operation | Admin | Member | Viewer |
|-----------|-------|--------|--------|
| Upload files | ✅ | ✅ | ❌ |
| Download files | ✅ | ✅ | ✅ |
| Delete own files | ✅ | ✅ | ❌ |
| Delete any files | ✅ | ❌ | ❌ |
| Start/stop sessions | ✅ | ❌ | ❌ |
| Invite members | ✅ | ❌ | ❌ |
| Approve devices | ✅ | ❌ | ❌ |
| Send notifications | ✅ | ❌ | ❌ |
| View logs/metrics | ✅ | ✅ | ✅ |

## Directory Structure

```
$FAMILYVAULT_DRIVE_PATH/
  groups/<group_id>/
    uploads/<session_id>/<user_id>/<filename.enc>
    backups/
    logs/
    manifests/

$FAMILYVAULT_DATA_PATH/
  groups.json
  users.json
  memberships.json
  devices.json
  tokens.json
  jwt_secret
```

## Development

### Backend Development

```bash
cd backend
go run . --port 8000
```

### Frontend Development

```bash
cd desktop/apps/admin-desktop
pnpm dev
```

### Testing

```bash
# Backend tests
cd backend && go test ./...

# Frontend tests
cd desktop/apps/admin-desktop && pnpm test

# E2E tests
cd desktop/apps/admin-desktop && pnpm e2e
```

## Troubleshooting

### Backend won't start
- Check `~/Library/Logs/FamilyVault/backend.log`
- Ensure ports 8000+ are available
- Verify drive path permissions

### Device pairing fails
- Check token hasn't expired (default 1 hour)
- Ensure admin approves device
- Verify network connectivity to localhost:8000

### Files won't upload
- Check active session exists
- Verify member/admin role
- Check storage quota limits
- Ensure file extensions are allowed

### App won't launch
- Check macOS security settings
- Verify app is signed (for distribution)
- Check Console.app for crash logs

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review logs in `~/Library/Logs/FamilyVault/`
3. Open an issue with system info and logs