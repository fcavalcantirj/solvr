import { describe, it, expect, beforeEach, vi, Mock } from 'vitest';
import { SolvrApiClient } from './api.js';

// Mock global fetch
global.fetch = vi.fn();

describe('SolvrApiClient', () => {
  const mockApiKey = 'solvr_test_key_123';
  const mockApiUrl = 'https://api.test.solvr.dev';
  let client: SolvrApiClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new SolvrApiClient(mockApiKey, mockApiUrl);
  });

  describe('constructor', () => {
    it('stores API key and URL', () => {
      expect(client).toBeDefined();
    });
  });

  describe('search', () => {
    it('calls /v1/search with query parameter', async () => {
      const mockResponse = {
        data: [
          { id: 'post_1', title: 'Test Post', type: 'problem', score: 0.95 }
        ],
        meta: { total: 1, page: 1, per_page: 20 }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.search('test query');

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/search?q=test+query`,
        {
          headers: {
            'Authorization': `Bearer ${mockApiKey}`,
          },
        }
      );
      expect(result).toEqual(mockResponse);
    });

    it('sends no type filter, even when a JavaScript caller still passes one (2.0.0)', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: {} }),
      });

      await client.search('test', { type: 'problem', limit: 5 } as Parameters<SolvrApiClient['search']>[1]);

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/search?q=test&per_page=5`,
        expect.any(Object)
      );
    });

    it('includes limit when provided', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: {} }),
      });

      await client.search('test', { limit: 10 });

      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('per_page=10'),
        expect.any(Object)
      );
    });

    it('throws error on API failure', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
      });

      await expect(client.search('test')).rejects.toThrow('API request failed');
    });
  });

  describe('getPost', () => {
    it('calls /v1/posts/:id', async () => {
      const mockPost = {
        data: { id: 'post_123', title: 'Test', type: 'question' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPost),
      });

      const result = await client.getPost('post_123');

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts/post_123`,
        {
          headers: {
            'Authorization': `Bearer ${mockApiKey}`,
          },
        }
      );
      expect(result).toEqual(mockPost);
    });

    it('requests the post alone (its contributions are read with listReplies)', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: {} }),
      });

      // An old caller may still pass an include list; nothing is appended to the URL.
      await (client.getPost as (id: string, options?: unknown) => Promise<unknown>)(
        'post_123',
        { include: ['approaches', 'answers'] }
      );

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts/post_123`,
        { headers: { 'Authorization': `Bearer ${mockApiKey}` } }
      );
    });

    it('throws error on 404', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 404,
        statusText: 'Not Found',
      });

      await expect(client.getPost('invalid_id')).rejects.toThrow('API request failed');
    });
  });

  describe('createPost', () => {
    it('calls POST /v1/posts with a canonical post (no type)', async () => {
      const mockResponse = {
        data: { id: 'new_post_123', title: 'How to test MCP servers?', type: 'post' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.createPost({
        title: 'How to test MCP servers?',
        description: 'I need help testing MCP servers with vitest',
        tags: ['mcp', 'testing'],
      });

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts`,
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Authorization': `Bearer ${mockApiKey}`,
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({
            title: 'How to test MCP servers?',
            description: 'I need help testing MCP servers with vitest',
            tags: ['mcp', 'testing'],
          }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('sends visibility when given', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { id: 'p1', type: 'post' } }),
      });

      await client.createPost({ title: 'Private notes', description: 'For my human', visibility: 'family' });

      const [, init] = (fetch as Mock).mock.calls[0];
      expect(JSON.parse(init.body)).toEqual({
        title: 'Private notes',
        description: 'For my human',
        visibility: 'family',
      });
    });

    it('throws error on validation failure', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
      });

      await expect(client.createPost({
        title: '',
        description: 'desc',
      })).rejects.toThrow('API request failed');
    });
  });

  describe('createReply', () => {
    it('calls POST /v1/posts/:id/replies with the body', async () => {
      const mockResponse = {
        data: { id: 'reply_123', post_id: 'post_123', body: 'Pin the pool size.' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.createReply('post_123', 'Pin the pool size.');

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts/post_123/replies`,
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Authorization': `Bearer ${mockApiKey}`,
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({ body: 'Pin the pool size.' }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('threads a reply under a parent reply', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { id: 'reply_2', parent_reply_id: 'reply_1' } }),
      });

      await client.createReply('post_123', 'Confirmed on Go 1.23.', 'reply_1');

      const [url, init] = (fetch as Mock).mock.calls[0];
      expect(url).toBe(`${mockApiUrl}/v1/posts/post_123/replies`);
      expect(JSON.parse(init.body)).toEqual({ body: 'Confirmed on Go 1.23.', parent_reply_id: 'reply_1' });
    });
  });

  describe('listReplies', () => {
    it('calls GET /v1/posts/:id/replies', async () => {
      const mockResponse = {
        data: [{ id: 'reply_1', body: 'First' }],
        meta: { total: 1, has_more: false },
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.listReplies('post_123');

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts/post_123/replies`,
        { headers: { 'Authorization': `Bearer ${mockApiKey}` } }
      );
      expect(result).toEqual(mockResponse);
    });

    it('pages with a cursor and a limit', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0, has_more: false } }),
      });

      await client.listReplies('post_123', { cursor: 'abc', limit: 10 });

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/posts/post_123/replies?cursor=abc&limit=10`,
        expect.any(Object)
      );
    });
  });

  describe('legacy contribution routes', () => {
    it('has no method that calls a retired answer or approach route', () => {
      const methods = client as unknown as Record<string, unknown>;
      expect(methods.createAnswer).toBeUndefined();
      expect(methods.createApproach).toBeUndefined();
    });
  });

  describe('error handling', () => {
    it('handles network errors', async () => {
      (fetch as Mock).mockRejectedValueOnce(new Error('Network error'));

      await expect(client.search('test')).rejects.toThrow('Network error');
    });

    it('handles 401 unauthorized', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
      });

      await expect(client.search('test')).rejects.toThrow('API request failed: 401');
    });

    it('handles 429 rate limit', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 429,
        statusText: 'Too Many Requests',
      });

      await expect(client.search('test')).rejects.toThrow('API request failed: 429');
    });

    it('includes the API error code and message from the body', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 410,
        statusText: 'Gone',
        json: () => Promise.resolve({
          error: {
            code: 'ENDPOINT_RETIRED',
            message: 'POST /v1/questions/{id}/answers was retired with the canonical knowledge model; use POST /v1/posts/{id}/replies instead.',
          },
        }),
      });

      await expect(client.createReply('post_123', 'x')).rejects.toThrow(
        'API request failed: 410 Gone: ENDPOINT_RETIRED: POST /v1/questions/{id}/answers was retired with the canonical knowledge model; use POST /v1/posts/{id}/replies instead.'
      );
    });

    it('keeps the status line when the error body is not JSON', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 502,
        statusText: 'Bad Gateway',
        json: () => Promise.reject(new SyntaxError('Unexpected token <')),
      });

      await expect(client.search('test')).rejects.toThrow(/^API request failed: 502 Bad Gateway$/);
    });
  });
});
