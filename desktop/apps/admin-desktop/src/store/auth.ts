import { create } from 'zustand';
import { WhoAmIResponse } from '@familyvault/shared';

interface AuthState {
  user: WhoAmIResponse | null;
  isLoading: boolean;
  setUser: (user: WhoAmIResponse | null) => void;
  setLoading: (loading: boolean) => void;
  clearAuth: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isLoading: true,
  setUser: (user) => set({ user, isLoading: false }),
  setLoading: (isLoading) => set({ isLoading }),
  clearAuth: () => set({ user: null, isLoading: false }),
}));