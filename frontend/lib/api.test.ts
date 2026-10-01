// Tests for API client auth event handling
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from './api';
import { APIError } from './api-error';

describe('SolvrAPI Configuration', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('uses configured API base URL for all requests', async () => {
    // Arrange: Mock successful response
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: [], meta: { page: 1, per_page: 20, total: 0 } }),
    });

    // Act: Make an API call
    await api.getPosts();

    // Assert: Verify the URL is using the configured base URL
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const calledUrl = fetchMock.mock.calls[0][0];

    // The baseUrl is set at module load time from NEXT_PUBLIC_API_URL or defaults to production
    // In development with .env.local, should be localhost:8080
    // In production, should be https://api.solvr.dev
    const expectedBaseUrl = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';
    expect(calledUrl).toContain(expectedBaseUrl);
    expect(calledUrl).toContain('/v1/posts');
  });
});

describe('SolvrAPI Auth Event Handling', () => {
  let authHandler: ReturnType<typeof vi.fn>;
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    // Create a mock auth handler
    authHandler = vi.fn();
    api.onAuthError(authHandler as (error: APIError) => void);

    // Mock global fetch
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    // Clean up
    api.offAuthError(authHandler as (error: APIError) => void);
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

describe('SolvrAPI room stream ticket (idx 75 step 2)', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    api.clearAuthToken();
    vi.restoreAllMocks();
  });

  it('POSTs to the ticket route with the credential in the Authorization header, not the URL', async () => {
    api.setAuthToken('jwt-abc');
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ data: { ticket: 'solvr_st_t', expires_at: '2026-09-25T00:00:00Z', ttl_seconds: 60, stream: '/v1/rooms/a b/stream' } }),
    });

    const res = await api.createRoomStreamTicket('a b');

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain('/v1/rooms/a%20b/stream-ticket');
    expect(url).not.toContain('jwt-abc');
    expect(init.method).toBe('POST');
    expect(init.headers['Authorization']).toBe('Bearer jwt-abc');
    expect(res.data.ticket).toBe('solvr_st_t');
  });
});

describe('SolvrAPI claim token transport', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('looks a claim token up with a POST body, never a URL', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ token_valid: false, error: 'invalid or unknown token' }),
    });

    await api.getClaimInfo('claim-secret-value');

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [calledUrl, init] = fetchMock.mock.calls[0];
    expect(String(calledUrl)).toMatch(/\/v1\/agents\/claim\/lookup$/);
    expect(String(calledUrl)).not.toContain('claim-secret-value');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body)).toEqual({ token: 'claim-secret-value' });
  });
});

describe('SolvrAPI replies by author (idx 73 step 3)', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('lists an author\'s replies from GET /v1/replies with the author filter, limit and cursor', async () => {
    fetchMock.mockResolvedValue({ ok: true, json: async () => ({ data: [], meta: { total: 0, has_more: false } }) });

    await api.getRepliesByAuthor('human', 'user 1', { limit: 20 });
    await api.getRepliesByAuthor('agent', 'agent_x', { limit: 20, cursor: 'c/1+' });

    const first = new URL(fetchMock.mock.calls[0][0]);
    expect(first.pathname).toBe('/v1/replies');
    expect(Object.fromEntries(first.searchParams)).toEqual({ author_type: 'human', author_id: 'user 1', limit: '20' });
    const second = new URL(fetchMock.mock.calls[1][0]);
    expect(Object.fromEntries(second.searchParams)).toEqual({
      author_type: 'agent', author_id: 'agent_x', limit: '20', cursor: 'c/1+',
    });
  });
});

describe('SolvrAPI conditional post edit (idx 74 step 5)', () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    global.fetch = fetchMock as unknown as typeof global.fetch;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns the post with the ETag of the read, the version an edit sends back', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      headers: new Headers({ ETag: '"1790000000000001"' }),
      json: async () => ({ data: { id: 'p1', title: 'A post' } }),
    });

    const res = await api.getPost('p1');

    expect(res.data).toEqual({ id: 'p1', title: 'A post' });
    expect(res.etag).toBe('"1790000000000001"');
  });

  it('reads a post without an ETag as no version', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => ({ data: { id: 'p1' } }) });

    const res = await api.getPost('p1');

    expect(res.etag).toBeNull();
  });

  it('sends the version it was given as If-Match on PATCH /v1/posts/{id}', async () => {
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => ({ data: { id: 'p1' } }) });

    await api.updatePost('p1', { title: 'Edited title here' }, '"1790000000000001"');

    const [url, init] = fetchMock.mock.calls[0];
    expect(new URL(url).pathname).toBe('/v1/posts/p1');
    expect(init.method).toBe('PATCH');
    expect(init.headers['If-Match']).toBe('"1790000000000001"');
    expect(JSON.parse(init.body)).toEqual({ title: 'Edited title here' });
  });

  it('surfaces the API message when the post changed since it was read', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 412,
      json: async () => ({ error: { code: 'PRECONDITION_FAILED', message: 'post was modified since you last read it; refetch and retry' } }),
    });

    await expect(api.updatePost('p1', { title: 'Edited title here' }, '"1"')).rejects.toMatchObject({
      statusCode: 412,
      message: 'post was modified since you last read it; refetch and retry',
    });
  });
});
