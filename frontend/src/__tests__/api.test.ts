/**
 * Tests for API client
 * Tests per SPEC.md Part 5 API Specification
 */

import { api, ApiError, setAuthToken, clearAuthToken, getAuthToken } from '../lib/api';

// Mock fetch globally
const mockFetch = jest.fn();
global.fetch = mockFetch;

// Mock localStorage
const mockLocalStorage = {
  getItem: jest.fn(),
  setItem: jest.fn(),
  removeItem: jest.fn(),
  clear: jest.fn(),
  length: 0,
  key: jest.fn(),
};
Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });

beforeEach(() => {
  mockFetch.mockClear();
  mockLocalStorage.getItem.mockClear();
  mockLocalStorage.setItem.mockClear();
  mockLocalStorage.removeItem.mockClear();
});

describe('API Client', () => {
  describe('Base URL', () => {
    it('uses NEXT_PUBLIC_API_URL environment variable if set', async () => {
      process.env.NEXT_PUBLIC_API_URL = 'https://api.example.com';
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: {} }),
      });

      await api.get('/test');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('https://api.example.com'),
        expect.any(Object)
      );

      delete process.env.NEXT_PUBLIC_API_URL;
    });

    it('defaults to /api if NEXT_PUBLIC_API_URL is not set', async () => {
      delete process.env.NEXT_PUBLIC_API_URL;
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: {} }),
      });

      await api.get('/test');

      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/api/test'), expect.any(Object));
    });
  });

  describe('GET requests', () => {
    it('makes GET request with correct method', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: '123' } }),
      });

      const result = await api.get('/posts/123');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: 'GET' })
      );
      expect(result).toEqual({ id: '123' });
    });

    it('includes query parameters in URL', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [] }),
      });

      await api.get('/search', { q: 'test query', page: '1' });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('q=test+query'),
        expect.any(Object)
      );
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('page=1'), expect.any(Object));
    });
  });

  describe('POST requests', () => {
    it('makes POST request with JSON body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: 'new-id' } }),
      });

      const body = { title: 'Test', description: 'Test description' };
      const result = await api.post('/posts', body);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(body),
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
      expect(result).toEqual({ id: 'new-id' });
    });
  });

  describe('PATCH requests', () => {
    it('makes PATCH request with JSON body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: { id: '123', title: 'Updated' } }),
      });

      const body = { title: 'Updated' };
      await api.patch('/posts/123', body);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify(body),
        })
      );
    });
  });

  describe('DELETE requests', () => {
    it('makes DELETE request', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => ({}),
      });

      await api.delete('/posts/123');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({ method: 'DELETE' })
      );
    });
  });

  describe('Authentication', () => {
    it('includes Authorization header when token is set', async () => {
      mockLocalStorage.getItem.mockReturnValueOnce('test-token');
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: {} }),
      });

      await api.get('/auth/me');

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer test-token',
          }),
        })
      );
    });

    it('does not include Authorization header when no token', async () => {
      mockLocalStorage.getItem.mockReturnValueOnce(null);
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: {} }),
      });

      await api.get('/posts');

      const calledHeaders = mockFetch.mock.calls[0][1].headers;
      expect(calledHeaders.Authorization).toBeUndefined();
    });

    it('setAuthToken stores token in localStorage', () => {
      setAuthToken('new-token');
      expect(mockLocalStorage.setItem).toHaveBeenCalledWith('solvr_auth_token', 'new-token');
    });

    it('clearAuthToken removes token from localStorage', () => {
      clearAuthToken();
      expect(mockLocalStorage.removeItem).toHaveBeenCalledWith('solvr_auth_token');
    });

    it('getAuthToken retrieves token from localStorage', () => {
      mockLocalStorage.getItem.mockReturnValueOnce('stored-token');
      expect(getAuthToken()).toBe('stored-token');
    });
  });

  describe('Error handling', () => {
    it('throws ApiError on non-2xx response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: async () => ({
          error: {
            code: 'NOT_FOUND',
            message: 'Resource not found',
          },
        }),
      });

      await expect(api.get('/posts/nonexistent')).rejects.toThrow(ApiError);
    });

    it('ApiError includes status, code, and message', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: async () => ({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'Title is required',
            details: { field: 'title' },
          },
        }),
      });

      try {
        await api.post('/posts', {});
        fail('Expected ApiError to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.status).toBe(400);
        expect(apiError.code).toBe('VALIDATION_ERROR');
        expect(apiError.message).toBe('Title is required');
        expect(apiError.details).toEqual({ field: 'title' });
      }
    });

    it('handles 401 UNAUTHORIZED error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({
          error: {
            code: 'UNAUTHORIZED',
            message: 'Not authenticated',
          },
        }),
      });

      try {
        await api.get('/auth/me');
        fail('Expected ApiError to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.status).toBe(401);
        expect(apiError.code).toBe('UNAUTHORIZED');
      }
    });

    it('handles 403 FORBIDDEN error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        json: async () => ({
          error: {
            code: 'FORBIDDEN',
            message: 'No permission',
          },
        }),
      });

      try {
        await api.delete('/posts/123');
        fail('Expected ApiError to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.status).toBe(403);
        expect(apiError.code).toBe('FORBIDDEN');
      }
    });

    it('handles 429 RATE_LIMITED error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 429,
        headers: new Headers({ 'Retry-After': '60' }),
        json: async () => ({
          error: {
            code: 'RATE_LIMITED',
            message: 'Too many requests',
          },
        }),
      });

      try {
        await api.get('/search');
        fail('Expected ApiError to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.status).toBe(429);
        expect(apiError.code).toBe('RATE_LIMITED');
      }
    });

    it('handles network errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      await expect(api.get('/posts')).rejects.toThrow('Network error');
    });

    it('handles JSON parse errors gracefully', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => {
          throw new Error('Invalid JSON');
        },
      });

      try {
        await api.get('/posts');
        fail('Expected error to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(ApiError);
        const apiError = error as ApiError;
        expect(apiError.status).toBe(500);
        expect(apiError.code).toBe('INTERNAL_ERROR');
      }
    });
  });

  describe('Response unwrapping', () => {
    it('unwraps data envelope from response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: { id: '123', title: 'Test' },
          meta: { timestamp: '2026-01-31' },
        }),
      });

      const result = await api.get('/posts/123');

      expect(result).toEqual({ id: '123', title: 'Test' });
    });

    it('returns full response when includeMetadata option is true', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          data: [{ id: '1' }, { id: '2' }],
          meta: { total: 100, page: 1, per_page: 20, has_more: true },
        }),
      });

      const result = await api.get('/posts', undefined, { includeMetadata: true });

      expect(result).toEqual({
        data: [{ id: '1' }, { id: '2' }],
        meta: { total: 100, page: 1, per_page: 20, has_more: true },
      });
    });
  });
});

describe('ApiError class', () => {
  it('has correct properties', () => {
    const error = new ApiError(400, 'VALIDATION_ERROR', 'Invalid input', { field: 'title' });

    expect(error.status).toBe(400);
    expect(error.code).toBe('VALIDATION_ERROR');
    expect(error.message).toBe('Invalid input');
    expect(error.details).toEqual({ field: 'title' });
    expect(error.name).toBe('ApiError');
  });

  it('extends Error', () => {
    const error = new ApiError(500, 'INTERNAL_ERROR', 'Server error');
    expect(error).toBeInstanceOf(Error);
  });

  it('isApiError helper identifies ApiError instances', () => {
    const apiError = new ApiError(404, 'NOT_FOUND', 'Not found');
    const regularError = new Error('Regular error');

    expect(apiError instanceof ApiError).toBe(true);
    expect(regularError instanceof ApiError).toBe(false);
  });
});
