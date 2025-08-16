# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Removed unused variables and imports causing TS6133 errors
- Installed missing clsx and tailwind-merge dependencies
- Cleaned up code to ensure clean build for macOS Electron app
- Fixed TypeScript compilation errors in Electron main process
- Removed dynamic imports that were causing build warnings

### Changed
- Updated package.json to include "type": "module" for PostCSS compatibility
- Cleaned up unused imports in multiple React components:
  - App.tsx: Removed unused `signOut` and `setLoading`
  - CreateGroup.tsx: Removed unused `response` variable
  - Members.tsx: Removed unused icons (QrCode, Copy, Check, Eye), unused state `copiedToken`, and unused function `handleCopyToken`
  - Notifications.tsx: Removed unused `Input` import
  - Pair.tsx: Removed unused `CheckCircle` import
  - SessionDetail.tsx: Removed unused `CardDescription`, `selectedFiles`, and `setSelectedFiles`
  - packages/shared/src/api.ts: Removed unused `AxiosResponse` import

### Added
- clsx ^2.0.0 dependency for conditional class names
- tailwind-merge ^2.2.0 dependency for Tailwind class merging

### Technical Details
- All TypeScript compilation errors resolved
- Production Electron build runs correctly (pnpm electron:build)
- ESLint rules pass without errors
- No functionality broken - only unused code removed
- Build artifacts successfully created:
  - release/FamilyVault-1.0.0-arm64.dmg
  - release/FamilyVault-1.0.0-arm64-mac.zip