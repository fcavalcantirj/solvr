import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Solvr } from './client.js';
import { SolvrError } from './types.js';

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('Solvr', () => {
  const apiKey = 'solvr_sk_test_key';

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('constructor', () => {
    it('should create instance with API key', () => {
      const solvr = new Solvr({ apiKey });
      expect(solvr).toBeInstanceOf(Solvr);
    });

    it('should throw if API key is missing', () => {
      expect(() => new Solvr({ apiKey: '' })).toThrow('API key is required');
    });

    it('should use default base URL', () => {
      const solvr = new Solvr({ apiKey });
      expect(solvr['baseUrl']).toBe('https://api.solvr.dev');
    });

    it('should allow custom base URL', () => {
      const solvr = new Solvr({ apiKey, baseUrl: 'https://custom.api.com' });
      expect(solvr['baseUrl']).toBe('https://custom.api.com');
    });
  });

  describe('search', () => {
    it('should search with query only', async () => {
      const mockResponse = {
        data: [
          { id: 'post_1', type: 'problem', title: 'Test Problem', score: 0.95 },
        ],
        meta: { total: 1, page: 1, per_page: 10 },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.search('test query');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/search?q=test+query',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Authorization': `Bearer ${apiKey}`,
          }),
        })
      );
      expect(result.data).toHaveLength(1);
      expect(result.data[0].title).toBe('Test Problem');
    });

    it('should search with options', async () => {
      const mockResponse = {
        data: [],
        meta: { total: 0, page: 1, per_page: 5 },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      await solvr.search('test', { type: 'problem', limit: 5, page: 2 });

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/search?q=test&type=problem&per_page=5&page=2',
        expect.any(Object)
      );
    });

    it('should not include type=all in query', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: { total: 0, page: 1, per_page: 10 } }),
      });

      const solvr = new Solvr({ apiKey });
      await solvr.search('test', { type: 'all' });

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/search?q=test',
        expect.any(Object)
      );
    });
  });

  describe('get', () => {
    it('should get post by ID', async () => {
      const mockPost = {
        id: 'post_123',
        type: 'problem',
        title: 'Test',
        description: 'Test description',
        status: 'open',
        upvotes: 10,
        downvotes: 0,
        view_count: 100,
        created_at: '2024-01-01T00:00:00Z',
        updated_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: mockPost }),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.get('post_123');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/posts/post_123',
        expect.any(Object)
      );
      expect(result.data.id).toBe('post_123');
    });

    it('should get post with includes', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { id: 'post_123' } }),
      });

      const solvr = new Solvr({ apiKey });
      await solvr.get('post_123', { include: ['approaches', 'answers'] });

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/posts/post_123?include=approaches%2Canswers',
        expect.any(Object)
      );
    });
  });

  describe('post', () => {
    it('should create a new post', async () => {
      const input = {
        type: 'problem' as const,
        title: 'New Problem',
        description: 'Problem description',
        tags: ['typescript', 'api'],
      };

      const mockResponse = {
        data: {
          id: 'post_new',
          ...input,
          status: 'open',
          upvotes: 0,
          downvotes: 0,
          view_count: 0,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.post(input);

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/posts',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(input),
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
      expect(result.data.id).toBe('post_new');
    });
  });

  describe('approach', () => {
    it('should add approach to a problem', async () => {
      const mockResponse = {
        data: {
          id: 'approach_1',
          post_id: 'post_123',
          angle: 'Test angle',
          content: 'Test content',
          status: 'proposed',
          upvotes: 0,
          downvotes: 0,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.approach('post_123', {
        angle: 'Test angle',
        content: 'Test content',
      });

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/problems/post_123/approaches',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ angle: 'Test angle', content: 'Test content' }),
        })
      );
      expect(result.data.id).toBe('approach_1');
    });
  });

  describe('answer', () => {
    it('should add answer to a question', async () => {
      const mockResponse = {
        data: {
          id: 'answer_1',
          post_id: 'question_123',
          content: 'Test answer',
          is_accepted: false,
          upvotes: 0,
          downvotes: 0,
          created_at: '2024-01-01T00:00:00Z',
          updated_at: '2024-01-01T00:00:00Z',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.answer('question_123', 'Test answer');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/questions/question_123/answers',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ content: 'Test answer' }),
        })
      );
      expect(result.data.id).toBe('answer_1');
    });
  });

  describe('vote', () => {
    it('should upvote a post', async () => {
      const mockResponse = {
        data: {
          upvotes: 11,
          downvotes: 0,
          user_vote: 'up',
        },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const solvr = new Solvr({ apiKey });
      const result = await solvr.vote('post_123', 'up');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/posts/post_123/vote',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ direction: 'up' }),
        })
      );
      expect(result.data.upvotes).toBe(11);
    });

    it('should downvote a post', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: { upvotes: 10, downvotes: 1, user_vote: 'down' } }),
      });

      const solvr = new Solvr({ apiKey });
      await solvr.vote('post_123', 'down');

      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.solvr.dev/v1/posts/post_123/vote',
        expect.objectContaining({
          body: JSON.stringify({ direction: 'down' }),
        })
      );
    });
  });

  describe('error handling', () => {
    it('should throw SolvrError on API error', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        json: () => Promise.resolve({ error: { message: 'Not found', code: 'NOT_FOUND' } }),
      });

      const solvr = new Solvr({ apiKey, retries: 1 });

      try {
        await solvr.get('invalid_id');
        expect.fail('Should have thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(SolvrError);
        expect((error as SolvrError).status).toBe(404);
        expect((error as SolvrError).code).toBe('NOT_FOUND');
      }
    });

    it('should handle non-JSON error responses', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: () => Promise.reject(new Error('Invalid JSON')),
      });

      const solvr = new Solvr({ apiKey, retries: 1 });

      await expect(solvr.get('post_123')).rejects.toThrow(SolvrError);
    });

    it('should handle network errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'));

      const solvr = new Solvr({ apiKey, retries: 1 });

      await expect(solvr.search('test')).rejects.toThrow('Network error');
    });
  });

  describe('retry logic', () => {
    it('should retry on 5xx errors', async () => {
      // First two calls fail, third succeeds
      mockFetch
        .mockResolvedValueOnce({ ok: false, status: 503, json: () => Promise.resolve({}) })
        .mockResolvedValueOnce({ ok: false, status: 503, json: () => Promise.resolve({}) })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ data: [], meta: { total: 0, page: 1, per_page: 10 } }),
        });

      const solvr = new Solvr({ apiKey, retries: 3 });
      const result = await solvr.search('test');

      expect(mockFetch).toHaveBeenCalledTimes(3);
      expect(result.data).toEqual([]);
    });

    it('should not retry on 4xx errors', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: () => Promise.resolve({ error: { message: 'Unauthorized' } }),
      });

      const solvr = new Solvr({ apiKey, retries: 3 });

      await expect(solvr.search('test')).rejects.toThrow();
      expect(mockFetch).toHaveBeenCalledTimes(1);
    });

    it('should fail after max retries', async () => {
      mockFetch.mockResolvedValue({ ok: false, status: 503, json: () => Promise.resolve({}) });

      const solvr = new Solvr({ apiKey, retries: 2 });

      await expect(solvr.search('test')).rejects.toThrow();
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });
  });
});
