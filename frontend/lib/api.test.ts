// Tests for API client auth event handling
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from './api';
import { APIError } from './api-error';

describe('SolvrAPI Auth Event Handling', () => {
  let authHandler: ReturnType<typeof vi.fn>;
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Create a mock auth handler
    authHandler = vi.fn();
    api.onAuthError(authHandler);

    // Mock global fetch
    fetchMock = vi.fn();
    global.fetch = fetchMock;
  });

  afterEach(() => {
    // Clean up
    api.offAuthError(authHandler);
    vi.restoreAllMocks();
  });

  describe('401 error handling', () => {
    it('should emit auth event when skipAuthEvent is not set', async () => {
      // Arrange: Mock 401 response
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { message: 'Unauthorized' } }),
      });

      // Act: Call an API method without skipAuthEvent
      try {
        await api.voteOnPost('test-post-id', 'up');
      } catch (err) {
        // Expected to throw
      }

      // Assert: Auth event handler should have been called
      expect(authHandler).toHaveBeenCalledTimes(1);
      expect(authHandler).toHaveBeenCalledWith(expect.any(APIError));
      const error = authHandler.mock.calls[0][0] as APIError;
      expect(error.statusCode).toBe(401);
    });

    it('should NOT emit auth event when skipAuthEvent is true', async () => {
      // Arrange: Mock 401 response
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { message: 'Unauthorized' } }),
      });

      // Act: Call getMyVote which should use skipAuthEvent: true
      try {
        await api.getMyVote('test-post-id');
      } catch (err) {
        // Expected to throw
      }

      // Assert: Auth event handler should NOT have been called
      expect(authHandler).not.toHaveBeenCalled();
    });

    it('should still throw the error even when skipAuthEvent is true', async () => {
      // Arrange: Mock 401 response
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { message: 'Unauthorized' } }),
      });

      // Act & Assert: Should still throw error
      await expect(api.getMyVote('test-post-id')).rejects.toThrow(APIError);

      // Need to mock again for second call
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { message: 'Unauthorized' } }),
      });

      await expect(api.getMyVote('test-post-id')).rejects.toThrow('Unauthorized');
    });

    it('should emit auth event for non-optional endpoints even with 401', async () => {
      // Arrange: Mock 401 response
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: { message: 'Unauthorized' } }),
      });

      // Act: Call a method that should trigger auth modal (bookmarking)
      try {
        await api.addBookmark('test-post-id');
      } catch (err) {
        // Expected to throw
      }

      // Assert: Auth event should be emitted for user actions
      expect(authHandler).toHaveBeenCalledTimes(1);
    });
  });

  describe('other error codes', () => {
    it('should not emit auth event for non-401 errors', async () => {
      // Arrange: Mock 404 response
      fetchMock.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: async () => ({ error: { message: 'Not found' } }),
      });

      // Act
      try {
        await api.getPost('non-existent-id');
      } catch (err) {
        // Expected to throw
      }

      // Assert: No auth event for non-401 errors
      expect(authHandler).not.toHaveBeenCalled();
    });
  });
});
