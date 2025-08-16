import '@testing-library/jest-dom'

// Mock window.fv API for tests
Object.defineProperty(window, 'fv', {
  value: {
    getVersion: () => Promise.resolve('1.0.0'),
    spawnBackendIfNeeded: () => Promise.resolve(),
    getBaseURL: () => Promise.resolve('http://127.0.0.1:8000'),
    setToken: () => Promise.resolve(),
    getToken: () => Promise.resolve(null),
    clearToken: () => Promise.resolve(),
    openFileDialog: () => Promise.resolve([]),
    showItemInFolder: () => {},
    copyToClipboard: () => {},
    onDeepLink: () => {},
    removeAllListeners: () => {},
  },
  writable: true,
})