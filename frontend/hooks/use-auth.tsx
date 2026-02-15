"use client";

import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { api } from '@/lib/api';

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

  const setToken = useCallback(async (token: string) => {
    localStorage.setItem(TOKEN_KEY, token);
    api.setAuthToken(token);
    await fetchUser();
  }, [fetchUser]);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    api.clearAuthToken();
    setUser(null);
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
        setToken,
        logout,
        loginWithGitHub,
        loginWithGoogle,
        loginWithEmail,
        register,
      }}
    >
      {children}
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
