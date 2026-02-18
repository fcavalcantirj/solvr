"use client";

import React, { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react';
import { api } from '@/lib/api';
import { AuthRequiredModal } from '@/components/ui/auth-required-modal';

export interface User {
  id: string;
  type: 'human' | 'agent';
  displayName: string;
  email?: string;
}

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  showAuthModal: boolean;
  authModalMessage: string;
  setShowAuthModal: (show: boolean) => void;
  setToken: (token: string) => Promise<void>;
  logout: () => void;
  loginWithGitHub: () => void;
  loginWithGoogle: () => void;
  loginWithEmail: (email: string, password: string) => Promise<{ success: boolean; error?: string }>;
  register: (email: string, password: string, username: string, displayName: string) => Promise<{ success: boolean; error?: string }>;
}

const AuthContext = createContext<AuthContextType | null>(null);

const TOKEN_KEY = 'auth_token';

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [authModalMessage, setAuthModalMessage] = useState('Login required to continue');
  const userRef = useRef<User | null>(null);
  const isLoadingRef = useRef(true);

  // Keep refs in sync with state
  useEffect(() => {
    userRef.current = user;
  }, [user]);

  useEffect(() => {
    isLoadingRef.current = isLoading;
  }, [isLoading]);

  const fetchUser = useCallback(async () => {
    try {
      const response = await api.getMe();
      setUser({
        id: response.data.id,
        type: response.data.type === 'agent' ? 'agent' : 'human',
        displayName: response.data.display_name,
        email: response.data.email,
      });
    } catch {
      // Auth failed - clear token
      localStorage.removeItem(TOKEN_KEY);
      api.clearAuthToken();
      setUser(null);
    }
  }, []);

  // Initialize auth state from stored token
  useEffect(() => {
    const initAuth = async () => {
      const token = localStorage.getItem(TOKEN_KEY);
      if (token) {
        api.setAuthToken(token);
        await fetchUser();
      }
      setIsLoading(false);
    };
    initAuth();
  }, [fetchUser]);

  // Listen for auth errors from API client
  useEffect(() => {
    const handler = () => {
      // Don't show modal during initialization (stale token 401s)
      if (isLoadingRef.current) return;
      // Don't show on pages that already have login UI
      const path = window.location.pathname;
      if (path.startsWith('/login') || path.startsWith('/join') || path.startsWith('/auth')) return;
      // Only show modal if user is not authenticated (use ref for current value)
      if (!userRef.current) {
        setAuthModalMessage('Login required to continue');
        setShowAuthModal(true);
      }
    };

    api.onAuthError(handler);
    return () => api.offAuthError(handler);
  }, []); // Empty dependency array since we use refs

  const setToken = useCallback(async (token: string) => {
    localStorage.setItem(TOKEN_KEY, token);
    api.setAuthToken(token);
    await fetchUser();
  }, [fetchUser]);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    api.clearAuthToken();
    setUser(null);
    // Reload page to clear all state
    window.location.href = '/';
  }, []);

  const loginWithGitHub = useCallback(() => {
    // Check for next parameter in URL query first
    const searchParams = new URLSearchParams(window.location.search);
    const nextUrl = searchParams.get('next');

    if (nextUrl) {
      // Store the next URL from query parameter
      localStorage.setItem('auth_return_url', nextUrl);
    } else {
      // Fallback to current path (if not on auth pages)
      const currentPath = window.location.pathname;
      if (!currentPath.startsWith('/login') && !currentPath.startsWith('/auth') && !currentPath.startsWith('/join')) {
        localStorage.setItem('auth_return_url', currentPath);
      }
    }
    window.location.href = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/auth/github`;
  }, []);

  const loginWithGoogle = useCallback(() => {
    // Check for next parameter in URL query first
    const searchParams = new URLSearchParams(window.location.search);
    const nextUrl = searchParams.get('next');

    if (nextUrl) {
      // Store the next URL from query parameter
      localStorage.setItem('auth_return_url', nextUrl);
    } else {
      // Fallback to current path (if not on auth pages)
      const currentPath = window.location.pathname;
      if (!currentPath.startsWith('/login') && !currentPath.startsWith('/auth') && !currentPath.startsWith('/join')) {
        localStorage.setItem('auth_return_url', currentPath);
      }
    }
    window.location.href = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/auth/google`;
  }, []);

  const loginWithEmail = useCallback(async (email: string, password: string): Promise<{ success: boolean; error?: string }> => {
    try {
      const response = await api.login(email, password);
      await setToken(response.access_token);
      return { success: true };
    } catch (error: unknown) {
      const err = error as { code?: string; message?: string };
      return {
        success: false,
        error: err.message || 'Login failed. Please try again.',
      };
    }
  }, [setToken]);

  const register = useCallback(async (
    email: string,
    password: string,
    username: string,
    displayName: string
  ): Promise<{ success: boolean; error?: string }> => {
    try {
      const response = await api.register(email, password, username, displayName);
      await setToken(response.access_token);
      return { success: true };
    } catch (error: unknown) {
      const err = error as { code?: string; message?: string };
      return {
        success: false,
        error: err.message || 'Registration failed. Please try again.',
      };
    }
  }, [setToken]);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: user !== null,
        isLoading,
        showAuthModal,
        authModalMessage,
        setShowAuthModal,
        setToken,
        logout,
        loginWithGitHub,
        loginWithGoogle,
        loginWithEmail,
        register,
      }}
    >
      {children}
      <AuthRequiredModal
        isOpen={showAuthModal}
        onClose={() => setShowAuthModal(false)}
        message={authModalMessage}
      />
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
