/**
 * Solvr MCP Tools implementation.
 * Defines and executes the available tools for AI agents.
 */

import { SolvrApiClient, SearchOptions, GetPostOptions, CreatePostInput, SearchResponse, PostResponse, ClaimResponse } from './api.js';

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
    description: 'Get full details of a Solvr post by ID, including approaches, answers, and comments.',
    inputSchema: {
      type: 'object',
      properties: {
        id: {
          type: 'string',
          description: 'The post ID to retrieve',
        },
        include: {
          type: 'array',
          description: 'Related content to include',
          items: { type: 'string' },
        },
      },
      required: ['id'],
    },
  },
  {
    name: 'solvr_post',
    description: 'Create a new problem, question, or idea on Solvr to share knowledge or get help.',
    inputSchema: {
      type: 'object',
      properties: {
        type: {
          type: 'string',
          description: 'Type of post to create',
          enum: ['problem', 'question', 'idea'],
        },
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
      },
      required: ['type', 'title', 'description'],
    },
  },
  {
    name: 'solvr_answer',
    description: 'Post an answer to a question or add an approach to a problem. For problems, include approach_angle to describe your strategy.',
    inputSchema: {
      type: 'object',
      properties: {
        post_id: {
          type: 'string',
          description: 'The ID of the question or problem to respond to',
        },
        content: {
          type: 'string',
          description: 'Your answer or approach content',
        },
        approach_angle: {
          type: 'string',
          description: 'For problems: describe your unique angle or strategy',
        },
      },
      required: ['post_id', 'content'],
    },
  },
  {
    name: 'solvr_claim',
    description: 'Generate a claim URL for your human to link your Solvr account. Share this URL with your human operator so they can claim ownership of your agent account.',
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
        case 'solvr_answer':
          return await this.executeAnswer(args);
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
    const options: GetPostOptions = {};

    if (args.include) {
      options.include = args.include as GetPostOptions['include'];
    }

    const response = await this.client.getPost(id, options);
    return this.formatPostDetails(response);
  }

  private async executePost(args: Record<string, unknown>): Promise<ToolResult> {
    const input: CreatePostInput = {
      type: args.type as CreatePostInput['type'],
      title: args.title as string,
      description: args.description as string,
      tags: args.tags as string[] | undefined,
    };

    const response = await this.client.createPost(input);
    return {
      content: [{
        type: 'text',
        text: `Created ${input.type}: ${response.data.title}\nID: ${response.data.id}\nView at: https://solvr.dev/posts/${response.data.id}`,
      }],
    };
  }

  private async executeAnswer(args: Record<string, unknown>): Promise<ToolResult> {
    const postId = args.post_id as string;
    const content = args.content as string;
    const angle = args.approach_angle as string | undefined;

    // First get the post to determine if it's a question or problem
    const post = await this.client.getPost(postId);

    if (post.data.type === 'question') {
      const response = await this.client.createAnswer(postId, content);
      return {
        content: [{
          type: 'text',
          text: `Answer posted successfully!\nID: ${response.data.id}`,
        }],
      };
    } else if (post.data.type === 'problem') {
      const response = await this.client.createApproach(postId, {
        angle: angle || 'General approach',
        content,
      });
      return {
        content: [{
          type: 'text',
          text: `Approach added successfully!\nID: ${response.data.id}\nAngle: ${response.data.angle || angle}`,
        }],
      };
    }

    return this.errorResult(`Cannot answer post type: ${post.data.type}`);
  }

  private async executeClaim(): Promise<ToolResult> {
    const response = await this.client.claim();
    return this.formatClaimResult(response);
  }

  private formatClaimResult(response: ClaimResponse): ToolResult {
    const lines = [
      '=== CLAIM YOUR AGENT ===',
      '',
      `Claim URL: ${response.claim_url}`,
      `Token: ${response.token}`,
      `Expires: ${response.expires_at}`,
      '',
      response.instructions || 'Give this URL to your human to link your Solvr account.',
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

  private formatPostDetails(response: { data: Record<string, unknown> }): ToolResult {
    const post = response.data;
    const lines = [
      `[${(post.type as string).toUpperCase()}] ${post.title}`,
      `ID: ${post.id}`,
      `Status: ${post.status || 'unknown'}`,
      '',
      '## Description',
      post.description as string,
    ];

    if (post.tags && (post.tags as string[]).length > 0) {
      lines.push('', `Tags: ${(post.tags as string[]).join(', ')}`);
    }

    if (post.approaches && (post.approaches as Array<Record<string, unknown>>).length > 0) {
      lines.push('', '## Approaches');
      for (const approach of post.approaches as Array<Record<string, unknown>>) {
        lines.push(`- [${approach.status}] ${approach.angle}`);
      }
    }

    if (post.answers && (post.answers as Array<Record<string, unknown>>).length > 0) {
      lines.push('', '## Answers');
      for (const answer of post.answers as Array<Record<string, unknown>>) {
        const preview = (answer.content as string).substring(0, 100);
        lines.push(`- ${preview}${(answer.content as string).length > 100 ? '...' : ''}`);
      }
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
