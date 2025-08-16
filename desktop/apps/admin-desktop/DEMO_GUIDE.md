# FamilyVault Desktop App - Demo Guide

## 🚀 Quick Start

### Option 1: Development Mode (Recommended for Testing)
```bash
cd desktop/apps/admin-desktop
./run-app.sh dev
```

### Option 2: Built Application
```bash
cd desktop/apps/admin-desktop
./run-app.sh built
```

### Option 3: DMG Installation
```bash
cd desktop/apps/admin-desktop
./run-app.sh dmg
```

## 🎯 Demo Walkthrough

### 1. Vault Management (New Feature!)

**What to Demo:**
- Navigate to the new "Vault" tab in the sidebar
- Show real device detection (internal drives, external USB/SSD if connected)
- Demonstrate device selection with capacity bars and usage percentages
- Show auto-creation of vault directory structure
- Upload files and show progress tracking
- Use "Open in Finder" to verify files are stored correctly

**Key Features:**
- ✅ Real device enumeration (replaces fake "Internal/External" radio buttons)
- ✅ Capacity visualization with colored progress bars
- ✅ Auto-directory creation: `<volume>/FamilyVault/<groupId>/<userId>/`
- ✅ File upload with progress tracking
- ✅ Quota enforcement
- ✅ Native "Open in Finder" integration

### 2. Native Sharing (New Feature!)

**What to Demo:**
- Go to Members page and click "Add Member"
- Fill in email/phone and send invitation
- Show the new share dialog with detected apps
- Try different sharing methods (Mail, Messages, WhatsApp if installed)
- Show clipboard fallback

**Key Features:**
- ✅ Detects installed macOS apps (Mail, Messages, Notes, WhatsApp, Telegram)
- ✅ Native integration with each app
- ✅ Modern glass-morphism UI design
- ✅ Clipboard fallback always available
- ✅ No more clipboard-only limitation!

### 3. Session Control (Redesigned!)

**What to Demo:**
- Show the new session control chip in the top-right of Dashboard
- Demonstrate start/stop functionality with visual state indicators
- Show micro-animations on state changes

**Key Features:**
- ✅ Moved to dashboard header (per spec)
- ✅ Compact pill design with status indicator
- ✅ Real-time state updates
- ✅ Smooth animations

### 4. Group Management (Enhanced!)

**What to Demo:**
- Click on the group dropdown in the sidebar
- Show "Create New Group" functionality
- Demonstrate "Join Group" with token input
- Show group switching capabilities

**Key Features:**
- ✅ Create new groups with proper API integration
- ✅ Join groups via invitation tokens
- ✅ Switch between groups (when multiple exist)
- ✅ Vault assignment updates on group switch

### 5. UI Polish (Throughout!)

**What to Demo:**
- Show the modern rounded corners (rounded-2xl) on all dialogs
- Demonstrate glass effects with backdrop blur
- Show responsive design at different window sizes
- Toggle between light and dark modes

**Key Features:**
- ✅ Futuristic glass-morphism design
- ✅ Consistent rounded corners and spacing
- ✅ Minimalist Lucide icons
- ✅ macOS vibrancy effects
- ✅ Smooth micro-animations

## 🧪 Testing Scenarios

### Vault Testing
1. **Device Detection:**
   - Connect/disconnect external USB drive
   - Verify it appears/disappears in device list
   - Check capacity information is accurate

2. **File Upload:**
   - Upload various file types (jpg, mp4, txt, pdf)
   - Upload multiple files simultaneously
   - Test quota enforcement with large files

3. **Storage Management:**
   - Select different storage locations
   - Verify directory structure creation
   - Test "Open in Finder" functionality

### Sharing Testing
1. **App Detection:**
   - Verify only installed apps appear
   - Test with/without WhatsApp, Telegram installed
   - Confirm Mail, Messages, Notes always appear

2. **Share Actions:**
   - Test Mail integration (opens with pre-filled content)
   - Test Messages (creates new message)
   - Test clipboard fallback
   - Verify error handling for failed shares

### Session Testing
1. **Admin Functions:**
   - Start/stop sessions multiple times
   - Verify state persistence
   - Test error handling for backend issues

2. **Non-Admin Users:**
   - Confirm session controls are hidden
   - Verify session status is still visible

### Group Testing
1. **Group Creation:**
   - Create groups with various names
   - Test error handling for invalid names
   - Verify app updates after creation

2. **Group Joining:**
   - Test with valid invitation tokens
   - Test with invalid tokens
   - Verify error messages

## 🐛 Known Issues & Workarounds

### Development Mode Issues
- **Backend Connection:** If backend fails to start automatically, run it manually:
  ```bash
  cd backend && ./familyvault
  ```

### Built App Issues
- **Code Signing:** App is not code-signed, so macOS may show security warnings
  - **Workaround:** Right-click app → "Open" → "Open" in security dialog

### Device Detection Issues
- **Permissions:** Some external drives may not be detected due to permissions
  - **Workaround:** Use "Choose folder..." option to manually select location

## 📊 Performance Notes

### Optimizations Implemented
- ✅ Streaming file copies for large files
- ✅ Non-blocking progress updates
- ✅ Lazy device detection
- ✅ Efficient IPC communication

### Memory Usage
- Expected: ~100-200MB for typical usage
- File uploads: Additional memory proportional to file sizes
- Device scanning: Minimal overhead

## 🔧 Troubleshooting

### App Won't Start
1. Check if backend is running: `curl http://localhost:8000/health`
2. Rebuild the app: `./run-app.sh build`
3. Check console for errors: Open DevTools in development mode

### Vault Issues
1. **No devices shown:** Check permissions, try "Choose folder..."
2. **Upload fails:** Verify disk space and permissions
3. **Directory not created:** Check write permissions on selected volume

### Sharing Issues
1. **No apps detected:** Verify apps are installed in standard locations
2. **Share fails:** Check app permissions and URL scheme support
3. **Clipboard not working:** Restart app, check system permissions

## 📱 Demo Script (30-60 seconds)

```
1. Open app → Navigate to Vault tab (5s)
   "Here's our new real device browser - no more fake radio buttons!"

2. Show device list with capacities (10s)
   "See real mounted drives with actual usage bars and free space"

3. Select device → Show auto-creation (10s)
   "Selecting creates the proper directory structure automatically"

4. Upload files → Show progress (10s)
   "File uploads now show real progress with quota enforcement"

5. Go to Members → Add member → Show share dialog (15s)
   "Sharing now detects your installed apps - Mail, Messages, WhatsApp, etc."

6. Show session control in header (5s)
   "Session controls moved to the header with a clean pill design"

7. Show group dropdown → Create/Join options (5s)
   "Full group management - create, join, and switch between groups"
```

## 🎬 Video Recording Tips

### Screen Recording Setup
- Use QuickTime or similar for high-quality recording
- Record at 1080p or higher resolution
- Ensure good lighting for clear visibility
- Use a clean desktop background

### Demo Flow
1. Start with a clean app state
2. Show each feature clearly with brief explanations
3. Highlight the improvements over old functionality
4. End with a quick recap of all new features

### Audio Tips
- Use a good microphone for clear narration
- Keep explanations concise and enthusiastic
- Mention specific technical improvements
- Highlight user experience improvements

## 🚀 Next Steps

After the demo, consider:
1. **User Feedback:** Gather feedback on the new UI and functionality
2. **Performance Testing:** Test with larger files and more devices
3. **Additional Features:** Plan for cloud storage integration, encryption, etc.
4. **Documentation:** Update user guides and help documentation
5. **Distribution:** Plan for proper code signing and distribution