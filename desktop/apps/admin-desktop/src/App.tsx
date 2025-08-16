import { useEffect } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useAuthStore } from './store/auth';
import { whoAmI } from './lib/auth';
import './styles/globals.css';

const queryClient = new QueryClient();

// Pages
import Welcome from './pages/Welcome';
import CreateGroup from './pages/CreateGroup';
import Pair from './pages/Pair';
import Dashboard from './pages/Dashboard';
import Vault from './pages/Vault';
import Sessions from './pages/Sessions';
import SessionDetail from './pages/SessionDetail';
import Members from './pages/Members';
import Notifications from './pages/Notifications';
import Settings from './pages/Settings';
import Profile from './pages/Profile';

// Components
import LoadingScreen from './components/LoadingScreen';
import AuthGuard from './components/AuthGuard';
import { ToastContainer, useToast } from './components/Toast';

function App() {
  const { user, isLoading, setUser, clearAuth } = useAuthStore();
  const { toasts, removeToast } = useToast();

  useEffect(() => {
    // Initialize auth state
    const initAuth = async () => {
      try {
        const userInfo = await whoAmI();
        setUser(userInfo);
      } catch (error) {
        console.error('Failed to initialize auth:', error);
        setUser(null);
      }
    };

    initAuth();

    // Listen for logout events
    const handleLogout = () => {
      clearAuth();
    };

    window.addEventListener('auth:logout', handleLogout);

    return () => {
      window.removeEventListener('auth:logout', handleLogout);
    };
  }, [setUser, clearAuth]);

  if (isLoading) {
    return <LoadingScreen />;
  }

  return (
    <QueryClientProvider client={queryClient}>
      <div className="min-h-screen bg-background font-sans antialiased">
        {/* Global draggable title bar for macOS */}
        <div 
          className="fixed top-0 left-0 right-0 h-8 bg-transparent z-50 pointer-events-none"
          style={{ WebkitAppRegion: 'drag' } as any}
        />
        <AuthGuard>
          <Routes>
        {/* Public routes */}
        <Route path="/welcome" element={<Welcome />} />
        <Route path="/create-group" element={<CreateGroup />} />
        <Route path="/pair" element={<Pair />} />

        {/* Protected routes */}
        <Route path="/dashboard" element={<Dashboard />} />
        <Route path="/vault" element={<Vault />} />
        <Route path="/sessions" element={<Sessions />} />
        <Route path="/sessions/:sessionId" element={<SessionDetail />} />
        <Route path="/members" element={<Members />} />
        <Route path="/notifications" element={<Notifications />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="/profile" element={<Profile />} />

        {/* Default redirect */}
        <Route path="/" element={
          user ? <Navigate to="/dashboard" replace /> : <Navigate to="/welcome" replace />
        } />
          </Routes>
        </AuthGuard>
        
        {/* Toast Container */}
        <ToastContainer toasts={toasts} onClose={removeToast} />
      </div>
    </QueryClientProvider>
  );
}

export default App;