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
    window.location.href = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/auth/github`;
  }, []);

  const loginWithGoogle = useCallback(() => {
    window.location.href = `${process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev'}/v1/auth/google`;
  }, []);

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
