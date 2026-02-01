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

    it('includes type filter when provided', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: [], meta: {} }),
      });

      await client.search('test', { type: 'problem' });

      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('type=problem'),
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

    it('includes query params when include option provided', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ data: {} }),
      });

      await client.getPost('post_123', { include: ['approaches', 'answers'] });

      // URL encodes the comma
      expect(fetch).toHaveBeenCalledWith(
        expect.stringContaining('include=approaches%2Canswers'),
        expect.any(Object)
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
    it('calls POST /v1/posts', async () => {
      const mockResponse = {
        data: { id: 'new_post_123', title: 'New Question', type: 'question' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.createPost({
        type: 'question',
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
            type: 'question',
            title: 'How to test MCP servers?',
            description: 'I need help testing MCP servers with vitest',
            tags: ['mcp', 'testing'],
          }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('throws error on validation failure', async () => {
      (fetch as Mock).mockResolvedValueOnce({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
      });

      await expect(client.createPost({
        type: 'question',
        title: '',
        description: 'desc',
      })).rejects.toThrow('API request failed');
    });
  });

  describe('createAnswer', () => {
    it('calls POST /v1/questions/:id/answers', async () => {
      const mockResponse = {
        data: { id: 'answer_123', content: 'Here is the answer' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.createAnswer('question_123', 'Here is the answer');

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/questions/question_123/answers`,
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Authorization': `Bearer ${mockApiKey}`,
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({ content: 'Here is the answer' }),
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('creates approach when post is a problem', async () => {
      const mockResponse = {
        data: { id: 'approach_123', angle: 'Test approach' }
      };
      (fetch as Mock).mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await client.createApproach('problem_123', {
        angle: 'Test approach',
        content: 'My approach details',
      });

      expect(fetch).toHaveBeenCalledWith(
        `${mockApiUrl}/v1/problems/problem_123/approaches`,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ angle: 'Test approach', content: 'My approach details' }),
        })
      );
      expect(result).toEqual(mockResponse);
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
  });
});
