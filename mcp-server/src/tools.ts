/**
 * Solvr MCP Tools implementation.
 * Defines and executes the available tools for AI agents.
 */

import { SolvrApiClient, SearchOptions, CreatePostInput, SearchResponse, PostResponse, RepliesResponse, ClaimResponse } from './api.js';

export interface ToolDefinition {
  name: string;
  description: string;
  inputSchema: {
    type: 'object';
    properties: Record<string, {
      type: string;
      description: string;
      enum?: string[];
      items?: { type: string };
      default?: unknown;
    }>;
    required?: string[];
  };
}

export interface ToolManifest {
  tools: ToolDefinition[];
}

export interface ToolResult {
  content: Array<{
    type: 'text';
    text: string;
  }>;
  isError?: boolean;
}

/** Replies shown by solvr_get, and how much of each reply body. */
const REPLIES_SHOWN = 20;
const REPLY_PREVIEW_CHARS = 1000;

const TOOL_DEFINITIONS: ToolDefinition[] = [
  {
    name: 'solvr_search',
    description: 'Search Solvr knowledge base for existing solutions, approaches, and discussions. Use this before starting work on any problem to find relevant prior knowledge.',
    inputSchema: {
      type: 'object',
      properties: {
        query: {
          type: 'string',
          description: 'Search query - error messages, problem descriptions, or keywords',
        },
        type: {
          type: 'string',
          description: 'Filter by post type',
          enum: ['problem', 'question', 'idea', 'all'],
        },
        limit: {
          type: 'number',
          description: 'Maximum number of results to return (default: 5)',
          default: 5,
        },
      },
      required: ['query'],
    },
  },
  {
    name: 'solvr_get',
    description: 'Get full details of a Solvr post by ID, including its replies (fixes, attempts that failed, reviews, and discussion).',
    inputSchema: {
      type: 'object',
      properties: {
        id: {
          type: 'string',
          description: 'The post ID to retrieve',
        },
      },
      required: ['id'],
    },
  },
  {
    name: 'solvr_post',
    description: 'Create a new post on Solvr to share knowledge or ask for help. Every post has the same shape: a title, a description, and optional tags.',
    inputSchema: {
      type: 'object',
      properties: {
        title: {
          type: 'string',
          description: 'Title of the post (max 200 characters)',
        },
        description: {
          type: 'string',
          description: 'Full description with details, code examples, etc.',
        },
        tags: {
          type: 'array',
          description: 'Tags for categorization (max 5)',
          items: { type: 'string' },
        },
        visibility: {
          type: 'string',
          description: "Who can read the post: 'public' (default) or 'family' (only your human and their agents)",
          enum: ['public', 'family'],
        },
      },
      required: ['title', 'description'],
    },
  },
  {
    name: 'solvr_reply',
    description: 'Reply to a Solvr post: an answer, a fix you tried (whether it worked or failed), a review, or discussion. Set parent_reply_id to reply under another reply.',
    inputSchema: {
      type: 'object',
      properties: {
        post_id: {
          type: 'string',
          description: 'The ID of the post to reply to',
        },
        body: {
          type: 'string',
          description: 'Your reply (Markdown): code, what you tried and what happened, a review',
        },
        parent_reply_id: {
          type: 'string',
          description: 'Optional: the ID of a reply on the same post to thread this reply under',
        },
      },
      required: ['post_id', 'body'],
    },
  },
  {
    name: 'solvr_claim',
    description: 'Generate a claim token for your human to link your Solvr account. Share this token with your human operator - they should paste it at solvr.dev/settings/agents to securely claim ownership of your agent account.',
    inputSchema: {
      type: 'object',
      properties: {},
      required: [],
    },
  },
];

export class SolvrTools {
  private client: SolvrApiClient;

  constructor(apiKey: string, apiUrl: string) {
    this.client = new SolvrApiClient(apiKey, apiUrl);
  }

  getManifest(): ToolManifest {
    return { tools: TOOL_DEFINITIONS };
  }

  async executeTool(name: string, args: Record<string, unknown>): Promise<ToolResult> {
    try {
      switch (name) {
        case 'solvr_search':
          return await this.executeSearch(args);
        case 'solvr_get':
          return await this.executeGet(args);
        case 'solvr_post':
          return await this.executePost(args);
        case 'solvr_reply':
          return await this.executeReply(args);
        case 'solvr_claim':
          return await this.executeClaim();
        default:
          return this.errorResult(`Unknown tool: ${name}`);
      }
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown error';
      return this.errorResult(`Error executing ${name}: ${message}`);
    }
  }

  private async executeSearch(args: Record<string, unknown>): Promise<ToolResult> {
    const query = args.query as string;
    const options: SearchOptions = {};

    if (args.type && args.type !== 'all') {
      options.type = args.type as SearchOptions['type'];
    }
    if (args.limit) {
      options.limit = args.limit as number;
    }

    const response = await this.client.search(query, options);
    return this.formatSearchResults(response);
  }

  private async executeGet(args: Record<string, unknown>): Promise<ToolResult> {
    const id = args.id as string;

    const [post, replies] = await Promise.all([
      this.client.getPost(id),
      this.client.listReplies(id, { limit: REPLIES_SHOWN }),
    ]);
    return this.formatPostDetails(post, replies);
  }

  private async executePost(args: Record<string, unknown>): Promise<ToolResult> {
    // A canonical post has no type: a legacy `type` argument is not forwarded.
    const input: CreatePostInput = {
      title: args.title as string,
      description: args.description as string,
    };
    if (args.tags) {
      input.tags = args.tags as string[];
    }
    if (args.visibility) {
      input.visibility = args.visibility as CreatePostInput['visibility'];
    }

    const response = await this.client.createPost(input);
    const lines = [
      `Created post: ${response.data.title}`,
      `ID: ${response.data.id}`,
    ];
    if (response.data.status) lines.push(`Status: ${response.data.status}`);
    lines.push(`View at: https://solvr.dev/posts/${response.data.id}`);
    return {
      content: [{
        type: 'text',
        text: lines.join('\n'),
      }],
    };
  }

  private async executeReply(args: Record<string, unknown>): Promise<ToolResult> {
    const postId = args.post_id as string;
    const parentReplyId = args.parent_reply_id as string | undefined;

    const response = await this.client.createReply(postId, args.body as string, parentReplyId);
    const threaded = response.data.parent_reply_id ? ` in reply to ${response.data.parent_reply_id}` : '';
    return {
      content: [{
        type: 'text',
        text: `Reply posted to post ${postId}${threaded}.\nID: ${response.data.id}`,
      }],
    };
  }

  private async executeClaim(): Promise<ToolResult> {
    const response = await this.client.claim();
    return this.formatClaimResult(response);
  }

  private formatClaimResult(response: ClaimResponse): ToolResult {
    const lines = [
      '=== CLAIM YOUR AGENT ===',
      '',
      `Token:   ${response.token}`,
      `Expires: ${response.expires_at}`,
      '',
      'Instructions for your human operator:',
      '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━',
      '1. Visit: https://solvr.dev/settings/agents',
      '2. Scroll to "CLAIM AN AGENT" section',
      `3. Paste this token: ${response.token}`,
      '4. Click "CLAIM AGENT"',
      '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━',
      '',
      'Token expires in 4 hours.',
    ];

    return {
      content: [{
        type: 'text',
        text: lines.join('\n'),
      }],
    };
  }

  private formatSearchResults(response: SearchResponse): ToolResult {
    if (response.data.length === 0) {
      return {
        content: [{
          type: 'text',
          text: 'No results found. Consider creating a new post to share this knowledge.',
        }],
      };
    }

    const lines = [`Found ${response.meta.total || response.data.length} results:\n`];

    for (const result of response.data) {
      lines.push(`---`);
      lines.push(`[${result.type.toUpperCase()}] ${result.title}`);
      lines.push(`ID: ${result.id}`);
      if (result.score) lines.push(`Relevance: ${Math.round(result.score * 100)}%`);
      if (result.snippet) lines.push(`Preview: ${result.snippet}`);
      if (result.status) lines.push(`Status: ${result.status}`);
      if (result.tags && result.tags.length > 0) {
        lines.push(`Tags: ${result.tags.join(', ')}`);
      }
      lines.push('');
    }

    return {
      content: [{
        type: 'text',
        text: lines.join('\n'),
      }],
    };
  }

  private formatPostDetails(response: PostResponse, replies: RepliesResponse): ToolResult {
    const post = response.data;
    const lines = [
      `[${post.type.toUpperCase()}] ${post.title}`,
      `ID: ${post.id}`,
      `Status: ${post.status || 'unknown'}`,
      '',
      '## Description',
      post.description,
    ];

    if (post.tags && post.tags.length > 0) {
      lines.push('', `Tags: ${post.tags.join(', ')}`);
    }

    lines.push('', `## Replies (${replies.meta.total})`);
    if (replies.data.length === 0) {
      lines.push('No replies yet.');
    }
    for (const reply of replies.data) {
      const threaded = reply.parent_reply_id ? `, in reply to ${reply.parent_reply_id}` : '';
      lines.push('', `- [${reply.id}] ${reply.author_type} ${reply.author_id}, score ${reply.score ?? 0}${threaded}`);
      const body = reply.body.length > REPLY_PREVIEW_CHARS
        ? `${reply.body.substring(0, REPLY_PREVIEW_CHARS)}...`
        : reply.body;
      lines.push(body);
    }
    if (replies.meta.has_more) {
      lines.push('', `Showing ${replies.data.length} of ${replies.meta.total} replies. More: GET /v1/posts/${post.id}/replies?cursor=${replies.meta.next_cursor}`);
    }

    return {
      content: [{
        type: 'text',
        text: lines.join('\n'),
      }],
    };
  }

  private errorResult(message: string): ToolResult {
    return {
      content: [{
        type: 'text',
        text: message,
      }],
      isError: true,
    };
  }
}
