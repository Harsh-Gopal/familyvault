# Build Verification Process

This document explains the automated build verification system implemented to prevent missing module errors in the packaged Electron app.

## Problem Solved

Previously, the packaged Electron app would fail at runtime with "Cannot find module" errors for dependencies like `form-data`, `delayed-stream`, etc. These errors occurred because:

1. Some dependencies are optional or conditionally loaded by packages like `axios`
2. Electron's ASAR packaging can exclude modules that aren't explicitly declared
3. Sub-dependencies might not be properly included in the final package

## Solution Components

### 1. Explicit Dependencies

All required runtime dependencies are now explicitly declared in `package.json`:

```json
{
  "dependencies": {
    "form-data": "^4.0.0",
    "asynckit": "^0.4.0",
    "combined-stream": "^1.0.8",
    "delayed-stream": "^1.0.0",
    "es-errors": "^1.3.0",
    "es-set-tostringtag": "^2.1.0",
    "function-bind": "^1.1.2",
    "get-intrinsic": "^1.3.0",
    "has-tostringtag": "^1.0.2",
    "hasown": "^2.0.2",
    "mime-types": "^3.0.1",
    "mime-db": "^1.54.0"
  }
}
```

### 2. ASAR Unpacking Configuration

Critical modules are unpacked from the ASAR archive to ensure they're available at runtime:

```json
{
  "build": {
    "asarUnpack": [
      "node_modules/form-data/**/*",
      "node_modules/asynckit/**/*",
      "node_modules/combined-stream/**/*",
      "node_modules/delayed-stream/**/*",
      "node_modules/es-errors/**/*",
      "node_modules/es-set-tostringtag/**/*",
      "node_modules/function-bind/**/*",
      "node_modules/get-intrinsic/**/*",
      "node_modules/has-tostringtag/**/*",
      "node_modules/hasown/**/*",
      "node_modules/mime-types/**/*",
      "node_modules/mime-db/**/*"
    ]
  }
}
```

### 3. Runtime Dependency Checker

**Script**: `scripts/check-runtime-deps.js`

This script:
- Scans all compiled Electron files (`.cjs`, `.js`) in `dist/electron/`
- Extracts all `require()` statements
- Checks that each required module exists in `node_modules/`
- Excludes built-in Node.js modules and Electron modules
- Reports any missing dependencies before packaging

**Usage**:
```bash
pnpm check-deps
```

### 4. Packaged App Runtime Tester

**Script**: `scripts/test-packaged-app.js`

This script:
- Finds the packaged `.app` bundle in the release directory
- Launches the app programmatically
- Monitors stdout/stderr for module loading errors
- Runs for 30 seconds to catch startup issues
- Reports success or failure

**Usage**:
```bash
pnpm test-packaged
```

## New Build Scripts

### Safe Build Process

```bash
# Build with dependency verification
pnpm build:safe

# Complete safe build and test pipeline
pnpm electron:build:safe
```

### Individual Commands

```bash
# Check dependencies only
pnpm check-deps

# Test already-packaged app
pnpm test-packaged

# Analyze form-data dependency tree
pnpm analyze-form-data

# Traditional build (no verification)
pnpm build
pnpm electron:build
```

## Build Pipeline Flow

1. **Build**: Compile TypeScript and bundle the app
2. **Dependency Check**: Verify all required modules are available
3. **Package**: Create the Electron app bundle (.dmg, .zip)
4. **Runtime Test**: Launch the packaged app to verify it starts without errors

## Troubleshooting

### Missing Module Errors

If the dependency checker finds missing modules:

1. Add the missing module to `package.json` dependencies
2. Run `pnpm install`
3. If the module needs special handling, add it to `asarUnpack`
4. Re-run the build process

### Runtime Test Failures

If the packaged app test fails:

1. Check the console output for specific error messages
2. Verify the module is in the unpacked directory: `release/mac-arm64/FamilyVault.app/Contents/Resources/app.asar.unpacked/node_modules/`
3. Ensure the module is listed in the `asarUnpack` configuration
4. Rebuild and test again

## Benefits

- **Prevents runtime errors**: Catches missing dependencies before packaging
- **Automated verification**: No manual testing required to verify basic functionality
- **Clear error reporting**: Specific guidance on what's missing and how to fix it
- **CI/CD ready**: Can be integrated into automated build pipelines
- **Maintainable**: Easy to extend for additional checks and validations

## Future Enhancements

- Add checks for native modules and their compatibility
- Verify file permissions and code signing
- Test more complex app functionality beyond just startup
- Integration with CI/CD systems for automated testing