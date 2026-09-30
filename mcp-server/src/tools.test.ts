import { describe, it, expect, beforeEach, vi, Mock } from 'vitest';
import { SolvrTools, ToolManifest, ToolResult } from './tools.js';
import { SolvrApiClient } from './api.js';

// Mock the API client
vi.mock('./api.js', () => ({
  SolvrApiClient: vi.fn().mockImplementation(() => ({
    search: vi.fn(),
    getPost: vi.fn(),
    createPost: vi.fn(),
    createReply: vi.fn(),
    listReplies: vi.fn(),
    claim: vi.fn(),
  })),
}));

describe('SolvrTools', () => {
  let tools: SolvrTools;
  let mockClient: {
    search: Mock;
    getPost: Mock;
    createPost: Mock;
    createReply: Mock;
    listReplies: Mock;
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
        'solvr_reply',
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
      expect(getTool?.description).toContain('replies');
      expect(getTool?.inputSchema.properties).toHaveProperty('id');
      expect(getTool?.inputSchema.properties).not.toHaveProperty('include');
      expect(getTool?.inputSchema.required).toContain('id');
    });

    it('solvr_post tool has correct schema', () => {
      const manifest = tools.getManifest();
      const postTool = manifest.tools.find(t => t.name === 'solvr_post');

      expect(postTool).toBeDefined();
      expect(postTool?.description).toContain('Create a new');
      expect(postTool?.inputSchema.properties).not.toHaveProperty('type');
      expect(postTool?.inputSchema.properties).toHaveProperty('title');
      expect(postTool?.inputSchema.properties).toHaveProperty('description');
      expect(postTool?.inputSchema.properties).toHaveProperty('tags');
      expect(postTool?.inputSchema.properties.visibility?.enum).toEqual(['public', 'family']);
      expect(postTool?.inputSchema.required).toEqual(['title', 'description']);
      expect(postTool?.description).not.toMatch(/problem|question|idea/i);
    });

    it('solvr_reply tool has correct schema', () => {
      const manifest = tools.getManifest();
      const replyTool = manifest.tools.find(t => t.name === 'solvr_reply');

      expect(replyTool).toBeDefined();
      expect(replyTool?.description).toContain('Reply to a Solvr post');
      expect(Object.keys(replyTool?.inputSchema.properties ?? {})).toEqual(['post_id', 'body', 'parent_reply_id']);
      expect(replyTool?.inputSchema.required).toEqual(['post_id', 'body']);
    });

    it('offers no legacy typed choice (post type, approach angle, answer tool)', () => {
      const manifest = tools.getManifest();

      expect(manifest.tools.map(t => t.name)).not.toContain('solvr_answer');
      for (const tool of manifest.tools.filter(t => t.name !== 'solvr_search')) {
        expect(tool.inputSchema.properties).not.toHaveProperty('type');
        expect(tool.inputSchema.properties).not.toHaveProperty('approach_angle');
      }
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
        mockClient.listReplies.mockResolvedValue({ data: [], meta: { total: 0, has_more: false } });

        const result = await tools.executeTool('solvr_get', { id: 'post_123' });

        expect(mockClient.getPost).toHaveBeenCalledWith('post_123');
        expect(result.content[0].text).toContain('Test Post');
        expect(result.content[0].text).toContain('No replies yet.');
      });

      it('shows the replies of the post, threaded ones marked with their parent', async () => {
        mockClient.getPost.mockResolvedValue({
          data: { id: 'post_123', title: 'Pool exhaustion', type: 'post', description: 'Details' }
        });
        mockClient.listReplies.mockResolvedValue({
          data: [
            { id: 'reply_1', post_id: 'post_123', author_type: 'agent', author_id: 'bot_a', body: 'Pin the pool size to 10.', score: 3 },
            { id: 'reply_2', post_id: 'post_123', parent_reply_id: 'reply_1', author_type: 'human', author_id: 'u_b', body: 'Confirmed.', score: 0 },
          ],
          meta: { total: 3, has_more: true, next_cursor: 'cur_1' },
        });

        const result = await tools.executeTool('solvr_get', { id: 'post_123' });

        expect(mockClient.listReplies).toHaveBeenCalledWith('post_123', { limit: 20 });
        const text = result.content[0].text;
        expect(text).toContain('## Replies (3)');
        expect(text).toContain('reply_1');
        expect(text).toContain('Pin the pool size to 10.');
        expect(text).toContain('agent bot_a');
        expect(text).toContain('in reply to reply_1');
        expect(text).toContain('Confirmed.');
        expect(text).toContain('Showing 2 of 3 replies');
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
          data: { id: 'new_post', title: 'How to test?', type: 'post' }
        };
        mockClient.createPost.mockResolvedValue(mockResponse);

        const result = await tools.executeTool('solvr_post', {
          title: 'How to test?',
          description: 'I need help',
          tags: ['testing'],
        });

        expect(mockClient.createPost).toHaveBeenCalledWith({
          title: 'How to test?',
          description: 'I need help',
          tags: ['testing'],
        });
        expect(result.content[0].text).toContain('Created post: How to test?');
        expect(result.content[0].text).toContain('new_post');
      });

      it('never sends a type, even when a caller still passes one', async () => {
        mockClient.createPost.mockResolvedValue({ data: { id: 'p1', title: 'T', type: 'post' } });

        await tools.executeTool('solvr_post', {
          type: 'problem',
          title: 'T',
          description: 'D',
          visibility: 'family',
        });

        expect(mockClient.createPost).toHaveBeenCalledWith({
          title: 'T',
          description: 'D',
          visibility: 'family',
        });
      });

      it('returns error on validation failure', async () => {
        mockClient.createPost.mockRejectedValue(new Error('400 Bad Request'));

        const result = await tools.executeTool('solvr_post', {
          title: '',
          description: 'desc',
        });

        expect(result.isError).toBe(true);
      });
    });

    describe('solvr_reply', () => {
      it('replies to a post without looking up its type', async () => {
        mockClient.createReply.mockResolvedValue({
          data: { id: 'reply_123', post_id: 'post_123', body: 'The fix' }
        });

        const result = await tools.executeTool('solvr_reply', {
          post_id: 'post_123',
          body: 'The fix',
        });

        expect(mockClient.getPost).not.toHaveBeenCalled();
        expect(mockClient.createReply).toHaveBeenCalledWith('post_123', 'The fix', undefined);
        expect(result.isError).toBeUndefined();
        expect(result.content[0].text).toContain('Reply posted');
        expect(result.content[0].text).toContain('reply_123');
      });

      it('threads the reply under parent_reply_id', async () => {
        mockClient.createReply.mockResolvedValue({
          data: { id: 'reply_2', post_id: 'post_123', parent_reply_id: 'reply_1', body: 'Confirmed' }
        });

        const result = await tools.executeTool('solvr_reply', {
          post_id: 'post_123',
          body: 'Confirmed',
          parent_reply_id: 'reply_1',
        });

        expect(mockClient.createReply).toHaveBeenCalledWith('post_123', 'Confirmed', 'reply_1');
        expect(result.content[0].text).toContain('in reply to reply_1');
      });

      it('returns the API error when the post is not found', async () => {
        mockClient.createReply.mockRejectedValue(new Error('API request failed: 404 Not Found: NOT_FOUND: post not found'));

        const result = await tools.executeTool('solvr_reply', {
          post_id: 'invalid',
          body: 'reply',
        });

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('post not found');
      });
    });

    describe('solvr_answer', () => {
      it('is no longer a tool (replaced by solvr_reply)', async () => {
        const result = await tools.executeTool('solvr_answer', { post_id: 'p', content: 'c' });

        expect(result.isError).toBe(true);
        expect(result.content[0].text).toContain('Unknown tool: solvr_answer');
        expect(mockClient.createReply).not.toHaveBeenCalled();
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
