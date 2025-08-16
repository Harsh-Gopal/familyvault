# Testing Documentation

## Overview

This document describes how to test the FamilyVault desktop application, including automated tests with Playwright and manual QA procedures.

## Running Tests

### Prerequisites

Ensure you have the application built and dependencies installed:

```bash
cd desktop/apps/admin-desktop
pnpm install
pnpm build
```

### Automated Tests (Playwright)

#### Setup Playwright

```bash
# Install Playwright browsers (first time only)
npx playwright install

# Run all tests
pnpm e2e

# Run tests in headed mode (see browser)
npx playwright test --headed

# Run specific test file
npx playwright test tests/vault.spec.ts

# Run tests with debug mode
npx playwright test --debug
```

#### Test Structure

Tests are organized by feature area:

```
tests/
├── vault.spec.ts           # Vault device selection and upload tests
├── sharing.spec.ts         # Share dialog and target tests  
├── sessions.spec.ts        # Session control tests
├── groups.spec.ts          # Group management tests
└── integration.spec.ts     # End-to-end workflow tests
```

### Unit Tests (Vitest)

```bash
# Run unit tests
pnpm test

# Run tests in watch mode
pnpm test:watch

# Run with coverage
pnpm test -- --coverage
```

## Test Categories

### 1. Vault System Tests

#### Device Detection Tests
```typescript
test('should list available devices', async ({ page }) => {
  // Mock IPC response
  await page.evaluate(() => {
    window.fv.vault.listDevices = () => Promise.resolve({
      ok: true,
      data: [
        {
          id: 'internal-disk',
          name: 'Macintosh HD',
          mountPoint: '/',
          type: 'internal',
          isRemovable: false,
          capacity: 1000000000000,
          free: 500000000000,
          usedPct: 50
        }
      ]
    });
  });

  await page.goto('/vault');
  await expect(page.locator('[data-testid="device-list"]')).toBeVisible();
  await expect(page.locator('text=Macintosh HD')).toBeVisible();
});
```

#### Storage Selection Tests
```typescript
test('should save vault selection', async ({ page }) => {
  let savedConfig = null;
  
  await page.evaluate(() => {
    window.fv.vault.setSelection = (groupId, userId, mountPoint) => {
      savedConfig = { groupId, userId, mountPoint };
      return Promise.resolve({ ok: true, data: '/path/to/vault' });
    };
  });

  await page.goto('/vault');
  await page.click('[data-testid="device-internal-disk"]');
  
  expect(savedConfig).toBeTruthy();
});
```

#### Upload Tests
```typescript
test('should handle file upload with progress', async ({ page }) => {
  const progressEvents = [];
  
  await page.evaluate(() => {
    window.fv.vault.copyFiles = () => Promise.resolve({ ok: true });
    window.fv.vault.onCopyProgress = (callback) => {
      // Simulate progress events
      setTimeout(() => callback({ current: 1, total: 2, fileName: 'test1.jpg' }), 100);
      setTimeout(() => callback({ current: 2, total: 2, fileName: 'test2.jpg' }), 200);
    };
  });

  await page.goto('/vault');
  // Simulate file selection and upload
  await page.click('[data-testid="upload-button"]');
  
  await expect(page.locator('[data-testid="upload-progress"]')).toBeVisible();
});
```

### 2. Sharing System Tests

#### Share Target Detection
```typescript
test('should list available share targets', async ({ page }) => {
  await page.evaluate(() => {
    window.fv.share.listTargets = () => Promise.resolve({
      ok: true,
      data: [
        { id: 'mail', name: 'Mail', icon: '📧', available: true },
        { id: 'copy', name: 'Copy to Clipboard', icon: '📋', available: true }
      ]
    });
  });

  await page.goto('/members');
  await page.click('[data-testid="add-member-button"]');
  // Fill form and submit to trigger share dialog
  await page.fill('[data-testid="contact-input"]', 'test@example.com');
  await page.click('[data-testid="send-invitation"]');
  
  await expect(page.locator('[data-testid="share-dialog"]')).toBeVisible();
  await expect(page.locator('text=Mail')).toBeVisible();
  await expect(page.locator('text=Copy to Clipboard')).toBeVisible();
});
```

#### Share Invocation Tests
```typescript
test('should invoke share target', async ({ page }) => {
  let invokedTarget = null;
  let sharedContent = null;
  
  await page.evaluate(() => {
    window.fv.share.invoke = (targetId, payload) => {
      invokedTarget = targetId;
      sharedContent = payload;
      return Promise.resolve({ ok: true });
    };
  });

  // Navigate to share dialog and click Mail
  await page.click('[data-testid="share-target-mail"]');
  
  expect(invokedTarget).toBe('mail');
  expect(sharedContent).toContain('FamilyVault');
});
```

### 3. Session Control Tests

#### Session Start/Stop
```typescript
test('should start and stop sessions', async ({ page }) => {
  let sessionState = null;
  
  await page.evaluate(() => {
    window.fv.getAPI = () => ({
      openSession: () => {
        sessionState = 'active';
        return Promise.resolve({ id: 'session-123', created_at: new Date().toISOString() });
      },
      closeSession: () => {
        sessionState = 'stopped';
        return Promise.resolve();
      }
    });
  });

  await page.goto('/dashboard');
  
  // Start session
  await page.click('[data-testid="session-start-button"]');
  await expect(page.locator('text=Active')).toBeVisible();
  
  // Stop session
  await page.click('[data-testid="session-stop-button"]');
  await expect(page.locator('text=Stopped')).toBeVisible();
});
```

### 4. Group Management Tests

#### Create Group
```typescript
test('should create new group', async ({ page }) => {
  let createdGroup = null;
  
  await page.evaluate(() => {
    window.fv.getAPI = () => ({
      createGroup: (data) => {
        createdGroup = data;
        return Promise.resolve({ id: 'group-123', name: data.name });
      }
    });
  });

  await page.goto('/dashboard');
  await page.click('[data-testid="group-dropdown"]');
  await page.click('[data-testid="create-group-button"]');
  await page.fill('[data-testid="group-name-input"]', 'Test Family');
  await page.click('[data-testid="create-group-submit"]');
  
  expect(createdGroup.name).toBe('Test Family');
});
```

#### Join Group
```typescript
test('should join group with token', async ({ page }) => {
  let joinToken = null;
  
  await page.evaluate(() => {
    window.fv.getAPI = () => ({
      joinGroup: (data) => {
        joinToken = data.token;
        return Promise.resolve();
      }
    });
  });

  await page.goto('/dashboard');
  await page.click('[data-testid="group-dropdown"]');
  await page.click('[data-testid="join-group-button"]');
  await page.fill('[data-testid="join-token-input"]', 'ABC123DEF456');
  await page.click('[data-testid="join-group-submit"]');
  
  expect(joinToken).toBe('ABC123DEF456');
});
```

## Manual QA Checklist

### Pre-Test Setup

- [ ] Backend is running and accessible
- [ ] Application is built and installed
- [ ] Test files are available (various formats: jpg, mp4, txt, pdf)
- [ ] External storage devices are connected (USB drive, external SSD)

### Vault Functionality

#### Device Detection
- [ ] Internal drives are listed with correct names and capacities
- [ ] External drives appear when connected
- [ ] Network drives are detected (if available)
- [ ] "Choose folder..." option is present
- [ ] Device icons are appropriate (internal/external/network)
- [ ] Capacity bars show correct usage percentages
- [ ] Available space is calculated correctly

#### Storage Selection
- [ ] Clicking a device creates the vault directory structure
- [ ] Path follows format: `<volume>/FamilyVault/<groupId>/<userId>/`
- [ ] Manifest file is created (`.vault.manifest.json`)
- [ ] Configuration is persisted between app restarts
- [ ] "Change" button allows selecting different storage
- [ ] "Open in Finder" opens the correct directory

#### File Upload
- [ ] File picker opens when clicking "Upload Files"
- [ ] Multiple file selection works
- [ ] Progress bar shows during upload
- [ ] Files are copied to vault directory with UUID names
- [ ] Manifest is updated with file metadata
- [ ] Quota enforcement prevents oversized uploads
- [ ] Upload completes successfully for various file types

### Sharing System

#### App Detection
- [ ] Only installed apps appear in share dialog
- [ ] Mail app is detected (system app)
- [ ] Messages app is detected (system app)
- [ ] Notes app is detected (system app)
- [ ] WhatsApp is detected (if installed)
- [ ] Telegram is detected (if installed)
- [ ] "Copy to Clipboard" is always available

#### Share Actions
- [ ] Mail opens with pre-filled subject and body
- [ ] Messages creates new message with invitation text
- [ ] WhatsApp opens with pre-filled message
- [ ] Telegram opens with pre-filled message
- [ ] Notes creates new note with invitation content
- [ ] Copy to clipboard works and shows "Copied!" confirmation
- [ ] Error handling works for unavailable apps

### Session Control

#### Admin Users
- [ ] Session control chip appears in dashboard header
- [ ] Shows "Stopped" state initially
- [ ] "Start" button initiates session successfully
- [ ] State changes to "Active" with green indicator
- [ ] "Stop" button terminates session
- [ ] State returns to "Stopped"
- [ ] Loading states show during transitions
- [ ] Error messages appear for failed operations

#### Non-Admin Users
- [ ] Session control is hidden for non-admin users
- [ ] Session status is still visible in dashboard cards

### Group Management

#### Create Group
- [ ] Group dropdown shows current group name
- [ ] "Create New Group" option is available
- [ ] Form accepts group name input
- [ ] Group creation succeeds with valid name
- [ ] App reloads/updates to show new group
- [ ] Error handling for duplicate names

#### Join Group
- [ ] "Join Group" option is available
- [ ] Form accepts invitation token input
- [ ] Valid tokens allow joining successfully
- [ ] Invalid tokens show appropriate error
- [ ] App updates to show joined group

#### Switch Group
- [ ] "Switch Group" shows available groups
- [ ] Switching updates vault assignment
- [ ] Vault prompts for new storage if none configured

### UI Polish

#### Visual Design
- [ ] Rounded corners (rounded-2xl) on dialogs and cards
- [ ] Glass effect backgrounds with backdrop blur
- [ ] Proper spacing and padding (12/16px)
- [ ] Minimalist icons from Lucide
- [ ] Colored usage bars for storage
- [ ] Micro-animations on state changes
- [ ] macOS vibrancy in modals

#### Responsive Design
- [ ] Layout adapts to different window sizes
- [ ] Components remain usable at minimum window size
- [ ] Text truncates appropriately
- [ ] Grid layouts reflow correctly

#### Dark Mode
- [ ] All components support dark mode
- [ ] Colors remain readable and accessible
- [ ] Glass effects work in both themes

### Error Handling

#### Network Errors
- [ ] Backend connection failures show appropriate messages
- [ ] Retry mechanisms work correctly
- [ ] Offline state is handled gracefully

#### File System Errors
- [ ] Permission denied errors are handled
- [ ] Disk full scenarios show clear messages
- [ ] Invalid paths are rejected safely

#### User Input Validation
- [ ] Empty fields show validation errors
- [ ] Invalid tokens are rejected
- [ ] File size limits are enforced

## Performance Testing

### Load Testing
- [ ] Upload 100+ files simultaneously
- [ ] Monitor memory usage during large uploads
- [ ] Test with files of various sizes (1KB to 1GB)
- [ ] Verify app remains responsive during operations

### Stress Testing
- [ ] Rapid session start/stop cycles
- [ ] Multiple vault selections in quick succession
- [ ] Concurrent file operations
- [ ] Memory leak detection over extended use

## Accessibility Testing

### Keyboard Navigation
- [ ] All interactive elements are keyboard accessible
- [ ] Tab order is logical and intuitive
- [ ] Focus indicators are visible
- [ ] Escape key closes modals

### Screen Reader Support
- [ ] ARIA labels are present and descriptive
- [ ] Status changes are announced
- [ ] Error messages are accessible
- [ ] Progress updates are communicated

## Browser Compatibility

Since this is an Electron app, browser compatibility is primarily about Chromium versions:

- [ ] Test with latest Electron version
- [ ] Verify ES6+ features work correctly
- [ ] Check for deprecated API usage
- [ ] Validate Node.js integration

## Continuous Integration

### Automated Test Pipeline
```yaml
# Example GitHub Actions workflow
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: pnpm install
      - run: pnpm build
      - run: pnpm test
      - run: pnpm e2e
```

### Test Reporting
- [ ] Test results are collected and reported
- [ ] Screenshots are captured for failed tests
- [ ] Coverage reports are generated
- [ ] Performance metrics are tracked

## Debugging Tests

### Common Issues

1. **IPC Mocking**: Ensure all IPC calls are properly mocked
2. **Timing Issues**: Use proper waits for async operations
3. **State Management**: Reset application state between tests
4. **File System**: Clean up test files and directories

### Debug Tools
```bash
# Run with debug output
DEBUG=pw:api pnpm e2e

# Generate test trace
npx playwright test --trace on

# Open trace viewer
npx playwright show-trace trace.zip
```