import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { useAuth, AuthProvider } from './use-auth';
import { api } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
  api: {
    getMe: vi.fn(),
    setAuthToken: vi.fn(),
    clearAuthToken: vi.fn(),
    login: vi.fn(),
    register: vi.fn(),
  },
}));

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value; }),
    removeItem: vi.fn((key: string) => { delete store[key]; }),
    clear: vi.fn(() => { store = {}; }),
  };
})();

Object.defineProperty(window, 'localStorage', { value: localStorageMock });

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <AuthProvider>{children}</AuthProvider>
);

describe('useAuth', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should have initial unauthenticated state', async () => {
    (api.getMe as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Not authenticated'));

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should load user from stored token', async () => {
    localStorageMock.setItem('auth_token', 'test-token');
    (api.getMe as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Test User',
        email: 'test@example.com',
      }
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(api.setAuthToken).toHaveBeenCalledWith('test-token');
    expect(api.getMe).toHaveBeenCalled();
    expect(result.current.user).toEqual({
      id: 'user-123',
      type: 'human',
      displayName: 'Test User',
      email: 'test@example.com',
    });
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should set token and fetch user on login', async () => {
    (api.getMe as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'user-456',
        type: 'human',
        display_name: 'New User',
        email: 'new@example.com',
      }
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.setToken('new-token');
    });

    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_token', 'new-token');
    expect(api.setAuthToken).toHaveBeenCalledWith('new-token');
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should clear user on logout', async () => {
    localStorageMock.setItem('auth_token', 'test-token');
    (api.getMe as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'user-123',
        type: 'human',
        display_name: 'Test User',
        email: 'test@example.com',
      }
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isAuthenticated).toBe(true);
    });

    act(() => {
      result.current.logout();
    });

    expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token');
    expect(api.clearAuthToken).toHaveBeenCalled();
    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should handle getMe errors gracefully', async () => {
    localStorageMock.setItem('auth_token', 'invalid-token');
    (api.getMe as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Invalid token'));

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.user).toBeNull();
    expect(result.current.isAuthenticated).toBe(false);
    // Token should be cleared on auth failure
    expect(localStorageMock.removeItem).toHaveBeenCalledWith('auth_token');
  });

  it('should loginWithEmail and store JWT on success', async () => {
    (api.login as ReturnType<typeof vi.fn>).mockResolvedValue({
      access_token: 'jwt-token',
      refresh_token: 'refresh-token',
      user: {
        id: 'user-789',
        username: 'testuser',
        display_name: 'Test User',
        email: 'test@example.com',
        role: 'user',
      }
    });
    (api.getMe as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'user-789',
        type: 'human',
        display_name: 'Test User',
        email: 'test@example.com',
      }
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let loginResult;
    await act(async () => {
      loginResult = await result.current.loginWithEmail('test@example.com', 'password123');
    });

    expect(api.login).toHaveBeenCalledWith('test@example.com', 'password123');
    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_token', 'jwt-token');
    expect(api.setAuthToken).toHaveBeenCalledWith('jwt-token');
    expect(loginResult).toEqual({ success: true });
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should return error on loginWithEmail failure', async () => {
    (api.login as ReturnType<typeof vi.fn>).mockRejectedValue({
      code: 'INVALID_CREDENTIALS',
      message: 'Invalid email or password',
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let loginResult;
    await act(async () => {
      loginResult = await result.current.loginWithEmail('test@example.com', 'wrongpassword');
    });

    expect(api.login).toHaveBeenCalledWith('test@example.com', 'wrongpassword');
    expect(loginResult).toEqual({
      success: false,
      error: 'Invalid email or password'
    });
    expect(result.current.isAuthenticated).toBe(false);
  });

  it('should register and store JWT on success', async () => {
    (api.register as ReturnType<typeof vi.fn>).mockResolvedValue({
      access_token: 'new-jwt-token',
      refresh_token: 'new-refresh-token',
      user: {
        id: 'user-999',
        username: 'newuser',
        display_name: 'New User',
        email: 'new@example.com',
        role: 'user',
      }
    });
    (api.getMe as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: {
        id: 'user-999',
        type: 'human',
        display_name: 'New User',
        email: 'new@example.com',
      }
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let registerResult;
    await act(async () => {
      registerResult = await result.current.register(
        'new@example.com',
        'password123',
        'newuser',
        'New User'
      );
    });

    expect(api.register).toHaveBeenCalledWith('new@example.com', 'password123', 'newuser', 'New User');
    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_token', 'new-jwt-token');
    expect(api.setAuthToken).toHaveBeenCalledWith('new-jwt-token');
    expect(registerResult).toEqual({ success: true });
    expect(result.current.isAuthenticated).toBe(true);
  });

  it('should return error on register failure', async () => {
    (api.register as ReturnType<typeof vi.fn>).mockRejectedValue({
      code: 'DUPLICATE_EMAIL',
      message: 'Email already registered',
    });

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let registerResult;
    await act(async () => {
      registerResult = await result.current.register(
        'existing@example.com',
        'password123',
        'testuser',
        'Test User'
      );
    });

    expect(api.register).toHaveBeenCalledWith('existing@example.com', 'password123', 'testuser', 'Test User');
    expect(registerResult).toEqual({
      success: false,
      error: 'Email already registered'
    });
    expect(result.current.isAuthenticated).toBe(false);
  });
});
