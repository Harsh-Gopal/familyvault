import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'

// Mock the window.fv API
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

describe('App', () => {
  it('renders without crashing', () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })

    render(
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    )

    // Should show loading initially
    expect(screen.getByText('Loading FamilyVault...')).toBeInTheDocument()
  })
})