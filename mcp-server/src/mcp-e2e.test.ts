/**
 * E2E tests for MCP Server Tools
 *
 * Per PRD line 590:
 * - E2E: MCP tools
 * - Test solvr_search, solvr_get, solvr_post via MCP
 *
 * These tests verify the full MCP protocol flow:
 * 1. Server initialization
 * 2. Tool listing
 * 3. Tool execution (solvr_search, solvr_get, solvr_post)
 */

import { describe, it, expect, beforeEach, vi, Mock, afterEach } from 'vitest';
import { spawn, ChildProcess } from 'child_process';
import { Readable, Writable } from 'stream';

// Mock environment and API
vi.mock('./config.js', () => ({
  loadConfig: () => ({
    apiKey: 'test_e2e_key',
    apiUrl: 'https://api.test.solvr.dev',
  }),
}));

vi.mock('./api.js', () => ({
  SolvrApiClient: vi.fn().mockImplementation(() => ({
    search: vi.fn(),
    getPost: vi.fn(),
    createPost: vi.fn(),
    createReply: vi.fn(),
    listReplies: vi.fn(),
  })),
}));

import { SolvrTools } from './tools.js';
import { SolvrApiClient } from './api.js';

interface MCPRequest {
  jsonrpc: '2.0';
  id: number | string;
  method: string;
  params?: Record<string, unknown>;
}

interface MCPResponse {
  jsonrpc: '2.0';
  id: number | string;
  result?: unknown;
  error?: {
    code: number;
    message: string;
  };
}

/**
 * Helper class to simulate MCP server message handling
 * Tests the full request/response flow without spawning a subprocess
 */
class MCPTestClient {
  private tools: SolvrTools;
  private requestId: number = 0;

  constructor(tools: SolvrTools) {
    this.tools = tools;
  }

  async sendRequest(method: string, params?: Record<string, unknown>): Promise<MCPResponse> {
    const request: MCPRequest = {
      jsonrpc: '2.0',
      id: ++this.requestId,
      method,
      params,
    };

    return this.handleRequest(request);
  }

  private async handleRequest(request: MCPRequest): Promise<MCPResponse> {
    const { id, method, params } = request;

    switch (method) {
      case 'initialize':
        return {
          jsonrpc: '2.0',
          id,
          result: {
            name: 'solvr',
            version: '1.0.0',
            protocolVersion: '2024-11-05',
            capabilities: { tools: {} },
          },
        };

      case 'tools/list':
        return {
          jsonrpc: '2.0',
          id,
          result: this.tools.getManifest(),
        };

      case 'tools/call': {
        const toolName = params?.name as string;
        const toolArgs = (params?.arguments || {}) as Record<string, unknown>;

        if (!toolName) {
          return {
            jsonrpc: '2.0',
            id,
            error: { code: -32602, message: 'Missing tool name' },
          };
        }

        const result = await this.tools.executeTool(toolName, toolArgs);
        return {
          jsonrpc: '2.0',
          id,
          result,
        };
      }

      default:
        return {
          jsonrpc: '2.0',
          id,
          error: { code: -32601, message: `Method not found: ${method}` },
        };
    }
  }
}

describe('MCP Server E2E Tests', () => {
  let tools: SolvrTools;
  let client: MCPTestClient;
  let mockApiClient: {
    search: Mock;
    getPost: Mock;
    createPost: Mock;
    createReply: Mock;
    listReplies: Mock;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    tools = new SolvrTools('test_e2e_key', 'https://api.test.solvr.dev');
    mockApiClient = (tools as unknown as { client: typeof mockApiClient }).client;
    client = new MCPTestClient(tools);
  });

  describe('MCP Protocol: Initialize', () => {
    it('server responds to initialize with correct capabilities', async () => {
      const response = await client.sendRequest('initialize', {
        protocolVersion: '2024-11-05',
        clientInfo: { name: 'test-client', version: '1.0.0' },
      });

      expect(response.error).toBeUndefined();
      expect(response.result).toBeDefined();

      const result = response.result as {
        name: string;
        version: string;
        capabilities: { tools: object };
      };
      expect(result.name).toBe('solvr');
      expect(result.version).toBe('1.0.0');
      expect(result.capabilities.tools).toBeDefined();
    });
  });

  describe('MCP Protocol: Tools List', () => {
    it('returns all available tools via tools/list', async () => {
      const response = await client.sendRequest('tools/list');

      expect(response.error).toBeUndefined();
      expect(response.result).toBeDefined();

      const result = response.result as { tools: Array<{ name: string }> };
      expect(result.tools).toHaveLength(14);

      const toolNames = result.tools.map((t) => t.name);
      expect(toolNames).toContain('solvr_search');
      expect(toolNames).toContain('solvr_get');
      expect(toolNames).toContain('solvr_post');
      expect(toolNames).toContain('solvr_reply');
      expect(toolNames).not.toContain('solvr_answer');
      expect(toolNames).toContain('solvr_claim');
      for (const name of ['solvr_replies', 'solvr_get_reply', 'solvr_update_reply', 'solvr_room_create', 'solvr_room_join',
        'solvr_room_read', 'solvr_room_send', 'solvr_room_ticket', 'solvr_room_watch']) {
        expect(toolNames).toContain(name);
      }
    });

    it('each tool has proper schema definition', async () => {
      const response = await client.sendRequest('tools/list');
      const result = response.result as {
        tools: Array<{
          name: string;
          description: string;
          inputSchema: {
            type: string;
            properties: Record<string, unknown>;
            required?: string[];
          };
        }>;
      };

      for (const tool of result.tools) {
        expect(tool.name).toBeDefined();
        expect(tool.description).toBeDefined();
        expect(tool.inputSchema).toBeDefined();
        expect(tool.inputSchema.type).toBe('object');
        expect(tool.inputSchema.properties).toBeDefined();
      }
    });
  });

  describe('MCP Protocol: Tool Call - solvr_search', () => {
    it('executes solvr_search via MCP and returns formatted results', async () => {
      // Setup mock API response
      mockApiClient.search.mockResolvedValue({
        data: [
          {
            id: 'post_e2e_1',
            type: 'problem',
            title: 'E2E Test Problem',
            snippet: 'This is a test problem for E2E testing',
            score: 0.95,
            status: 'open',
            tags: ['testing', 'e2e'],
          },
          {
            id: 'post_e2e_2',
            type: 'question',
            title: 'E2E Test Question',
            snippet: 'This is a test question for E2E testing',
            score: 0.85,
            status: 'answered',
            tags: ['testing'],
          },
        ],
        meta: {
          total: 2,
          page: 1,
          per_page: 20,
          has_more: false,
        },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'e2e test' },
      });

      expect(response.error).toBeUndefined();
      expect(response.result).toBeDefined();

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content).toHaveLength(1);
      expect(result.content[0].type).toBe('text');
      expect(result.content[0].text).toContain('E2E Test Problem');
      expect(result.content[0].text).toContain('post_e2e_1');
      expect(result.content[0].text).toContain('95%'); // Score as percentage
    });

    it('solvr_search with type filter works via MCP', async () => {
      mockApiClient.search.mockResolvedValue({
        data: [
          {
            id: 'problem_1',
            type: 'problem',
            title: 'Filtered Problem',
            score: 0.9,
            status: 'open',
          },
        ],
        meta: { total: 1 },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'test', type: 'problem' },
      });

      expect(response.error).toBeUndefined();
      expect(mockApiClient.search).toHaveBeenCalledWith('test', { type: 'problem' });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('Filtered Problem');
    });

    it('solvr_search with limit works via MCP', async () => {
      mockApiClient.search.mockResolvedValue({ data: [], meta: { total: 0 } });

      await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'limited', limit: 3 },
      });

      expect(mockApiClient.search).toHaveBeenCalledWith('limited', { limit: 3 });
    });

    it('solvr_search returns no results message when empty', async () => {
      mockApiClient.search.mockResolvedValue({ data: [], meta: { total: 0 } });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'nonexistent' },
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('No results found');
    });

    it('solvr_search handles API errors gracefully', async () => {
      mockApiClient.search.mockRejectedValue(new Error('API unavailable'));

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'test' },
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
        isError?: boolean;
      };
      expect(result.isError).toBe(true);
      expect(result.content[0].text).toContain('Error');
    });
  });

  describe('MCP Protocol: Tool Call - solvr_get', () => {
    it('executes solvr_get via MCP and returns post details', async () => {
      mockApiClient.getPost.mockResolvedValue({
        data: {
          id: 'post_get_test',
          type: 'question',
          title: 'How to write E2E tests?',
          description:
            'I need help understanding how to write end-to-end tests for an MCP server.',
          status: 'open',
          tags: ['testing', 'mcp', 'e2e'],
        },
      });
      mockApiClient.listReplies.mockResolvedValue({ data: [], meta: { total: 0, has_more: false } });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_get',
        arguments: { id: 'post_get_test' },
      });

      expect(response.error).toBeUndefined();

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('How to write E2E tests?');
      expect(result.content[0].text).toContain('post_get_test');
      expect(result.content[0].text).toContain('QUESTION');
    });

    it('solvr_get returns the post with its replies', async () => {
      mockApiClient.getPost.mockResolvedValue({
        data: {
          id: 'post_with_replies',
          type: 'post',
          title: 'Post with replies',
          description: 'A post that has been answered.',
          status: 'open',
        },
      });
      mockApiClient.listReplies.mockResolvedValue({
        data: [
          { id: 'rep_1', post_id: 'post_with_replies', author_type: 'agent', author_id: 'a1', body: 'First reply with detailed explanation', score: 2 },
          { id: 'rep_2', post_id: 'post_with_replies', author_type: 'agent', author_id: 'a2', body: 'Second reply with an alternative', score: 0 },
        ],
        meta: { total: 2, has_more: false },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_get',
        arguments: { id: 'post_with_replies' },
      });

      expect(mockApiClient.getPost).toHaveBeenCalledWith('post_with_replies');
      expect(mockApiClient.listReplies).toHaveBeenCalledWith('post_with_replies', { limit: 20 });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('## Replies (2)');
      expect(result.content[0].text).toContain('First reply');
      expect(result.content[0].text).toContain('Second reply');
    });

    it('solvr_get shows migrated approaches and answers as replies of a legacy post', async () => {
      mockApiClient.getPost.mockResolvedValue({
        data: {
          id: 'problem_with_approaches',
          type: 'problem',
          title: 'Complex problem',
          description: 'A problem with multiple approaches.',
          status: 'in_progress',
        },
      });
      mockApiClient.listReplies.mockResolvedValue({
        data: [
          { id: 'rep_a', post_id: 'problem_with_approaches', author_type: 'agent', author_id: 'a1', body: 'Database optimization: add the missing index', legacy_type: 'approach', score: 1 },
          { id: 'rep_b', post_id: 'problem_with_approaches', author_type: 'agent', author_id: 'a2', body: 'Caching layer (failed: stale reads)', legacy_type: 'approach', score: 0 },
        ],
        meta: { total: 2, has_more: false },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_get',
        arguments: { id: 'problem_with_approaches' },
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('[PROBLEM] Complex problem');
      expect(result.content[0].text).toContain('Database optimization');
      expect(result.content[0].text).toContain('Caching layer');
    });

    it('solvr_get handles not found error', async () => {
      mockApiClient.getPost.mockRejectedValue(new Error('404 Not Found'));

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_get',
        arguments: { id: 'nonexistent_post' },
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
        isError?: boolean;
      };
      expect(result.isError).toBe(true);
      expect(result.content[0].text).toContain('Error');
    });
  });

  describe('MCP Protocol: Tool Call - solvr_post', () => {
    it('executes solvr_post via MCP to create a canonical post', async () => {
      mockApiClient.createPost.mockResolvedValue({
        data: {
          id: 'new_post_123',
          type: 'post',
          title: 'How to implement MCP?',
          description: 'Detailed description of my question.',
          status: 'open',
        },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_post',
        arguments: {
          title: 'How to implement MCP?',
          description: 'Detailed description of my question.',
          tags: ['mcp', 'implementation'],
        },
      });

      expect(response.error).toBeUndefined();
      expect(mockApiClient.createPost).toHaveBeenCalledWith({
        title: 'How to implement MCP?',
        description: 'Detailed description of my question.',
        tags: ['mcp', 'implementation'],
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('Created post: How to implement MCP?');
      expect(result.content[0].text).toContain('new_post_123');
    });

    it('solvr_post does not forward a legacy type argument', async () => {
      for (const legacyType of ['problem', 'question', 'idea']) {
        mockApiClient.createPost.mockResolvedValueOnce({
          data: { id: `new_${legacyType}`, type: 'post', title: 'Race condition in async code' },
        });

        const response = await client.sendRequest('tools/call', {
          name: 'solvr_post',
          arguments: {
            type: legacyType,
            title: 'Race condition in async code',
            description: 'Description of the problem.',
          },
        });

        expect(mockApiClient.createPost).toHaveBeenLastCalledWith({
          title: 'Race condition in async code',
          description: 'Description of the problem.',
        });
        const result = response.result as {
          content: Array<{ type: string; text: string }>;
        };
        expect(result.content[0].text).toContain('Created post');
        expect(result.content[0].text).toContain(`new_${legacyType}`);
      }
    });

    it('solvr_post handles validation errors', async () => {
      mockApiClient.createPost.mockRejectedValue(
        new Error('400 Bad Request: Title is required')
      );

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_post',
        arguments: {
          title: '',
          description: 'Missing title',
        },
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
        isError?: boolean;
      };
      expect(result.isError).toBe(true);
      expect(result.content[0].text).toContain('Error');
    });
  });

  describe('MCP Protocol: Tool Call - solvr_reply', () => {
    it('replies to a post via MCP without looking up its type', async () => {
      mockApiClient.createReply.mockResolvedValue({
        data: { id: 'reply_e2e_1', post_id: 'q_123', body: 'Here is my answer to your question.' },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_reply',
        arguments: {
          post_id: 'q_123',
          body: 'Here is my answer to your question.',
        },
      });

      expect(mockApiClient.getPost).not.toHaveBeenCalled();
      expect(mockApiClient.createReply).toHaveBeenCalledWith(
        'q_123',
        'Here is my answer to your question.',
        undefined
      );

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('Reply posted');
      expect(result.content[0].text).toContain('reply_e2e_1');
    });

    it('threads a reply under another reply via MCP', async () => {
      mockApiClient.createReply.mockResolvedValue({
        data: { id: 'reply_e2e_2', post_id: 'p_456', parent_reply_id: 'reply_e2e_1', body: 'Indexing fixed it here too.' },
      });

      const response = await client.sendRequest('tools/call', {
        name: 'solvr_reply',
        arguments: {
          post_id: 'p_456',
          body: 'Indexing fixed it here too.',
          parent_reply_id: 'reply_e2e_1',
        },
      });

      expect(mockApiClient.createReply).toHaveBeenCalledWith('p_456', 'Indexing fixed it here too.', 'reply_e2e_1');

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(result.content[0].text).toContain('in reply to reply_e2e_1');
      expect(result.content[0].text).toContain('reply_e2e_2');
    });
  });

  describe('MCP Protocol: Error Handling', () => {
    it('returns error for unknown tool', async () => {
      const response = await client.sendRequest('tools/call', {
        name: 'unknown_tool',
        arguments: {},
      });

      const result = response.result as {
        content: Array<{ type: string; text: string }>;
        isError?: boolean;
      };
      expect(result.isError).toBe(true);
      expect(result.content[0].text).toContain('Unknown tool');
    });

    it('returns error for missing tool name', async () => {
      const response = await client.sendRequest('tools/call', {
        arguments: {},
      });

      expect(response.error).toBeDefined();
      expect(response.error?.code).toBe(-32602);
      expect(response.error?.message).toContain('Missing tool name');
    });

    it('returns error for unknown method', async () => {
      const response = await client.sendRequest('unknown/method');

      expect(response.error).toBeDefined();
      expect(response.error?.code).toBe(-32601);
      expect(response.error?.message).toContain('Method not found');
    });
  });

  describe('MCP Integration: Full Workflow', () => {
    it('completes full search-before-work workflow', async () => {
      // 1. Initialize
      const initResponse = await client.sendRequest('initialize');
      expect(initResponse.error).toBeUndefined();

      // 2. List tools
      const listResponse = await client.sendRequest('tools/list');
      expect(listResponse.error).toBeUndefined();

      // 3. Search for existing solutions
      mockApiClient.search.mockResolvedValue({
        data: [],
        meta: { total: 0 },
      });
      const searchResponse = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'my specific problem' },
      });
      expect(searchResponse.error).toBeUndefined();

      // 4. No results found, create new post
      mockApiClient.createPost.mockResolvedValue({
        data: {
          id: 'new_post_workflow',
          type: 'post',
          title: 'My specific problem',
        },
      });
      const postResponse = await client.sendRequest('tools/call', {
        name: 'solvr_post',
        arguments: {
          title: 'My specific problem',
          description: 'Detailed description of what I tried...',
          tags: ['workflow', 'test'],
        },
      });
      expect(postResponse.error).toBeUndefined();

      const postResult = postResponse.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(postResult.content[0].text).toContain('new_post_workflow');
    });

    it('completes find-and-use-solution workflow', async () => {
      // 1. Initialize
      await client.sendRequest('initialize');

      // 2. Search and find existing solution
      mockApiClient.search.mockResolvedValue({
        data: [
          {
            id: 'solved_post',
            type: 'problem',
            title: 'Similar problem already solved',
            score: 0.92,
            status: 'solved',
          },
        ],
        meta: { total: 1 },
      });
      const searchResponse = await client.sendRequest('tools/call', {
        name: 'solvr_search',
        arguments: { query: 'my problem' },
      });

      const searchResult = searchResponse.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(searchResult.content[0].text).toContain('solved_post');
      expect(searchResult.content[0].text).toContain('92%');

      // 3. Get full details of the solution
      mockApiClient.getPost.mockResolvedValue({
        data: {
          id: 'solved_post',
          type: 'problem',
          title: 'Similar problem already solved',
          description: 'Original description...',
          status: 'solved',
        },
      });
      mockApiClient.listReplies.mockResolvedValue({
        data: [
          {
            id: 'winning_reply',
            post_id: 'solved_post',
            author_type: 'agent',
            author_id: 'solver',
            body: 'Working solution: pin the pool size. Verified, the approach succeeded.',
            score: 5,
          },
        ],
        meta: { total: 1, has_more: false },
      });
      const getResponse = await client.sendRequest('tools/call', {
        name: 'solvr_get',
        arguments: { id: 'solved_post' },
      });

      const getResult = getResponse.result as {
        content: Array<{ type: string; text: string }>;
      };
      expect(getResult.content[0].text).toContain('Working solution');
      expect(getResult.content[0].text).toContain('succeeded');
    });
  });
});
