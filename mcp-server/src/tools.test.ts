import { describe, it, expect, beforeEach, vi, Mock } from 'vitest';
import { SolvrTools, ToolManifest, ToolResult } from './tools.js';
import { SolvrApiClient } from './api.js';

// Mock the API client
vi.mock('./api.js', () => ({
  SolvrApiClient: vi.fn().mockImplementation(() => ({
    search: vi.fn(),
    getPost: vi.fn(),
    createPost: vi.fn(),
    createAnswer: vi.fn(),
    createApproach: vi.fn(),
    claim: vi.fn(),
  })),
}));

describe('SolvrTools', () => {
  let tools: SolvrTools;
  let mockClient: {
    search: Mock;
    getPost: Mock;
    createPost: Mock;
    createAnswer: Mock;
    createApproach: Mock;
    claim: Mock;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    tools = new SolvrTools('test_key', 'https://api.test.solvr.dev');
    mockClient = (tools as unknown as { client: typeof mockClient }).client;
  });

  describe('getManifest', () => {
    it('returns tool manifest with all tools', () => {
      const manifest = tools.getManifest();

      expect(manifest.tools).toHaveLength(5);
      expect(manifest.tools.map(t => t.name)).toEqual([
        'solvr_search',
        'solvr_get',
        'solvr_post',
        'solvr_answer',
        'solvr_claim',
      ]);
    });

    it('solvr_search tool has correct schema', () => {
      const manifest = tools.getManifest();
      const searchTool = manifest.tools.find(t => t.name === 'solvr_search');

      expect(searchTool).toBeDefined();
      expect(searchTool?.description).toContain('Search Solvr knowledge base');
      expect(searchTool?.inputSchema.properties).toHaveProperty('query');
      expect(searchTool?.inputSchema.properties).toHaveProperty('type');
      expect(searchTool?.inputSchema.properties).toHaveProperty('limit');
      expect(searchTool?.inputSchema.required).toContain('query');
    });

    it('solvr_get tool has correct schema', () => {
      const manifest = tools.getManifest();
      const getTool = manifest.tools.find(t => t.name === 'solvr_get');

      expect(getTool).toBeDefined();
      expect(getTool?.description).toContain('Get full details');
      expect(getTool?.inputSchema.properties).toHaveProperty('id');
      expect(getTool?.inputSchema.properties).toHaveProperty('include');
      expect(getTool?.inputSchema.required).toContain('id');
    });

    it('solvr_post tool has correct schema', () => {
      const manifest = tools.getManifest();
      const postTool = manifest.tools.find(t => t.name === 'solvr_post');

      expect(postTool).toBeDefined();
      expect(postTool?.description).toContain('Create a new');
      expect(postTool?.inputSchema.properties).toHaveProperty('type');
      expect(postTool?.inputSchema.properties).toHaveProperty('title');
      expect(postTool?.inputSchema.properties).toHaveProperty('description');
      expect(postTool?.inputSchema.properties).toHaveProperty('tags');
      expect(postTool?.inputSchema.required).toEqual(['type', 'title', 'description']);
    });

    it('solvr_answer tool has correct schema', () => {
      const manifest = tools.getManifest();
      const answerTool = manifest.tools.find(t => t.name === 'solvr_answer');

      expect(answerTool).toBeDefined();
      expect(answerTool?.description).toContain('answer');
      expect(answerTool?.inputSchema.properties).toHaveProperty('post_id');
      expect(answerTool?.inputSchema.properties).toHaveProperty('content');
      expect(answerTool?.inputSchema.properties).toHaveProperty('approach_angle');
      expect(answerTool?.inputSchema.required).toEqual(['post_id', 'content']);
    });

    it('solvr_claim tool has correct schema', () => {
      const manifest = tools.getManifest();
      const claimTool = manifest.tools.find(t => t.name === 'solvr_claim');

      expect(claimTool).toBeDefined();
      expect(claimTool?.description).toContain('claim token');
      expect(claimTool?.inputSchema.required).toEqual([]);
    });
  });

  describe('executeTool', () => {
    describe('solvr_search', () => {
      it('executes search with query', async () => {
        const mockResults = {
          data: [
            { id: 'post_1', title: 'Test', type: 'problem', score: 0.9 }
          ],
          meta: { total: 1 }
        };
        mockClient.search.mockResolvedValue(mockResults);

        const result = await tools.executeTool('solvr_search', { query: 'test query' });

        expect(mockClient.search).toHaveBeenCalledWith('test query', {});
        expect(result.content[0].type).toBe('text');
        expect(result.content[0].text).toContain('Test');
      });

      it('passes type filter', async () => {
        mockClient.search.mockResolvedValue({ data: [], meta: {} });

        await tools.executeTool('solvr_search', {
          query: 'test',
          type: 'problem'
        });

        expect(mockClient.search).toHaveBeenCalledWith('test', { type: 'problem' });
      });

      it('passes limit', async () => {
        mockClient.search.mockResolvedValue({ data: [], meta: {} });

        await tools.executeTool('solvr_search', {
          query: 'test',
          limit: 5
        });

        expect(mockClient.search).toHaveBeenCalledWith('test', { limit: 5 });
      });

      it('returns error on failure', async () => {
        mockClient.search.mockRejectedValue(new Error('API error'));

        const result = await tools.executeTool('solvr_search', { query: 'test' });

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('Error');
      });
    });

    describe('solvr_get', () => {
      it('executes get with id', async () => {
        const mockPost = {
          data: { id: 'post_123', title: 'Test Post', type: 'question', description: 'Details' }
        };
        mockClient.getPost.mockResolvedValue(mockPost);

        const result = await tools.executeTool('solvr_get', { id: 'post_123' });

        expect(mockClient.getPost).toHaveBeenCalledWith('post_123', {});
        expect(result.content[0].text).toContain('Test Post');
      });

      it('passes include options', async () => {
        mockClient.getPost.mockResolvedValue({ data: {} });

        await tools.executeTool('solvr_get', {
          id: 'post_123',
          include: ['approaches', 'answers']
        });

        expect(mockClient.getPost).toHaveBeenCalledWith('post_123', {
          include: ['approaches', 'answers']
        });
      });

      it('returns error when post not found', async () => {
        mockClient.getPost.mockRejectedValue(new Error('404 Not Found'));

        const result = await tools.executeTool('solvr_get', { id: 'invalid' });

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('Error');
      });
    });

    describe('solvr_post', () => {
      it('creates a new post', async () => {
        const mockResponse = {
          data: { id: 'new_post', title: 'New Question', type: 'question' }
        };
        mockClient.createPost.mockResolvedValue(mockResponse);

        const result = await tools.executeTool('solvr_post', {
          type: 'question',
          title: 'How to test?',
          description: 'I need help',
          tags: ['testing'],
        });

        expect(mockClient.createPost).toHaveBeenCalledWith({
          type: 'question',
          title: 'How to test?',
          description: 'I need help',
          tags: ['testing'],
        });
        expect(result.content[0].text).toContain('new_post');
      });

      it('returns error on validation failure', async () => {
        mockClient.createPost.mockRejectedValue(new Error('400 Bad Request'));

        const result = await tools.executeTool('solvr_post', {
          type: 'question',
          title: '',
          description: 'desc',
        });

        expect(result.isError).toBe(true);
      });
    });

    describe('solvr_answer', () => {
      it('creates answer for question', async () => {
        mockClient.getPost.mockResolvedValue({ data: { type: 'question' } });
        mockClient.createAnswer.mockResolvedValue({
          data: { id: 'answer_123', content: 'The answer' }
        });

        const result = await tools.executeTool('solvr_answer', {
          post_id: 'question_123',
          content: 'The answer',
        });

        expect(mockClient.createAnswer).toHaveBeenCalledWith('question_123', 'The answer');
        expect(result.content[0].text).toContain('answer_123');
      });

      it('creates approach for problem with angle', async () => {
        mockClient.getPost.mockResolvedValue({ data: { type: 'problem' } });
        mockClient.createApproach.mockResolvedValue({
          data: { id: 'approach_123', angle: 'My angle' }
        });

        const result = await tools.executeTool('solvr_answer', {
          post_id: 'problem_123',
          content: 'My approach details',
          approach_angle: 'My angle',
        });

        expect(mockClient.createApproach).toHaveBeenCalledWith('problem_123', {
          angle: 'My angle',
          content: 'My approach details',
        });
        expect(result.content[0].text).toContain('approach_123');
      });

      it('returns error when post not found', async () => {
        mockClient.getPost.mockRejectedValue(new Error('404 Not Found'));

        const result = await tools.executeTool('solvr_answer', {
          post_id: 'invalid',
          content: 'answer',
        });

        expect(result.isError).toBe(true);
      });
    });

    describe('solvr_claim', () => {
      it('executes claim and returns formatted result', async () => {
        const mockClaimResponse = {
          token: 'abc123',
          expires_at: '2026-02-08T22:00:00Z',
          instructions: 'Give this token to your human operator.',
        };
        mockClient.claim.mockResolvedValue(mockClaimResponse);

        const result = await tools.executeTool('solvr_claim', {});

        expect(mockClient.claim).toHaveBeenCalled();
        expect(result.content[0].text).toContain('CLAIM YOUR AGENT');
        expect(result.content[0].text).toContain('https://solvr.dev/settings/agents');
        expect(result.content[0].text).toContain('abc123');
      });

      it('returns error on API failure', async () => {
        mockClient.claim.mockRejectedValue(new Error('401 Unauthorized'));

        const result = await tools.executeTool('solvr_claim', {});

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('Error');
      });
    });

    describe('unknown tool', () => {
      it('returns error for unknown tool name', async () => {
        const result = await tools.executeTool('unknown_tool', {});

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('Unknown tool');
      });
    });
  });
});
