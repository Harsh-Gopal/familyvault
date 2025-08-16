import { ReactNode } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';

interface AuthGuardProps {
  children: ReactNode;
}

export default function AuthGuard({ children }: AuthGuardProps) {
  const { user } = useAuthStore();

  if (!user) {
    return <Navigate to="/welcome" replace />;
  }

  return <>{children}</>;
}